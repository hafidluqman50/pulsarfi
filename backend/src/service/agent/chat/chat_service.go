package chat

import (
	"context"
	"errors"
	"fmt"
	helper "github.com/horizonlabs/pulsarfi-backend/src/app/helper"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/orchestrator"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	agentcore "github.com/horizonlabs/pulsarfi-backend/src/app/core/agent"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"gorm.io/datatypes"
)

// turnTimeout bounds one detached turn. It matches the server's WriteTimeout,
// the longest a live client can wait for the blocking response.
const turnTimeout = 5 * time.Minute

// ChatService manages chat threads, user messages, and supervisor replies.
// It decides what each incoming message is (resolveTurn) and persists the
// user message and the supervisor reply, then delegates the whole turn to
// Orchestrator — pause and resume are handled by Eino's checkpoint store
// inside Orchestrator.Send, and every word the user reads is written by the
// model, never here: this type builds no LLM prompt.
type ChatService struct {
	Chats           *repository.AgentChatRepository
	ChatMessages    *repository.AgentChatMessageRepository
	Orchestrator    *orchestrator.Orchestrator
	CheckPointStore agentcore.ExtendedCheckPointStore
	// Turns is the per-chat single-turn guard. Nil in tests that build the
	// service directly; tryStartTurn treats nil as "no guard".
	Turns *agentcore.TurnRegistry
}

func (svc *ChatService) ListChats(ctx context.Context, wallet string) ([]model.AgentChat, error) {
	return svc.Chats.FindByOwnerWallet(ctx, strings.ToLower(wallet))
}

func (svc *ChatService) GetChat(ctx context.Context, chatID uuid.UUID, wallet string) (model.AgentChat, []model.AgentChatMessage, error) {
	chat, found, err := svc.Chats.FindByID(ctx, chatID)
	if err != nil {
		return model.AgentChat{}, nil, err
	}
	if !found {
		return model.AgentChat{}, nil, agentcore.ErrChatNotFound
	}
	if !strings.EqualFold(chat.OwnerWallet, wallet) {
		return model.AgentChat{}, nil, agentcore.ErrWalletMismatch
	}
	messages, err := svc.ChatMessages.FindByChatID(ctx, chatID)
	if err != nil {
		return model.AgentChat{}, nil, err
	}
	return chat, messages, nil
}

func (svc *ChatService) GetChatMessages(ctx context.Context, chatID uuid.UUID, wallet string) ([]model.AgentChatMessage, error) {
	_, messages, err := svc.GetChat(ctx, chatID, wallet)
	if errors.Is(err, agentcore.ErrChatNotFound) {
		// A chat with no row yet has no messages — QuasarPanel's "+ new chat"
		// opens one before the first message is sent, so reading it must be an
		// empty list, not an error. Ownership is unchanged: a chat that exists
		// and belongs to someone else still returns agentcore.ErrWalletMismatch.
		return []model.AgentChatMessage{}, nil
	}
	return messages, err
}

// resolveTurn is the single decision point: what is this message, given the
// chat's pending interrupt? It reads the pending marker exactly once; Send
// then switches on the result instead of asking again.
func (svc *ChatService) resolveTurn(ctx context.Context, chatID uuid.UUID, message string) agentcore.TurnIntent {
	pending, paused := svc.interrupts().Pending(ctx, chatID)
	if !paused {
		return agentcore.TurnIntent{Kind: agentcore.FreshTurn}
	}
	if isCancellationAction(message) {
		return agentcore.TurnIntent{Kind: agentcore.CancelTurn}
	}
	if pending.Kind == agentcore.InterruptQuestions {
		return agentcore.TurnIntent{Kind: agentcore.AnswerQuestions, InterruptID: pending.ID}
	}
	return agentcore.TurnIntent{Kind: agentcore.SideQuestionDuringArm}
}

// HandleChatMessage takes the chat's single turn slot, then runs the turn on a
// context that does not belong to the caller's connection: a refresh or a
// closed tab must not cancel work already committed to. hidden marks a message
// as real chat history (still persisted, still fed to the LLM as context,
// still returned by every read) that must never render as a bubble — the
// compiled answer a clarifying-questions card sends, not something the user
// typed by hand.
func (svc *ChatService) HandleChatMessage(ctx context.Context, chatID uuid.UUID, wallet, message string, hidden bool) (card agentcore.WorkflowCard, err error) {
	release, ok := svc.tryStartTurn(chatID)
	if !ok {
		return agentcore.WorkflowCard{}, agentcore.ErrTurnInFlight
	}
	defer release()

	svc.announceTurn(chatID, "running")
	defer func() { svc.announceTurn(chatID, turnStatus(err)) }()

	turnCtx, cancel := turnContext(ctx)
	defer cancel()
	return svc.handleChatMessage(turnCtx, chatID, wallet, message, hidden)
}

func (svc *ChatService) handleChatMessage(ctx context.Context, chatID uuid.UUID, wallet, message string, hidden bool) (agentcore.WorkflowCard, error) {
	chat, err := svc.Chats.FindOrCreate(ctx, chatID, strings.ToLower(wallet), helper.TruncateRunes(message, 50))
	if err != nil {
		return agentcore.WorkflowCard{}, err
	}
	if !strings.EqualFold(chat.OwnerWallet, wallet) {
		return agentcore.WorkflowCard{}, agentcore.ErrWalletMismatch
	}

	intent := svc.resolveTurn(ctx, chatID, message)
	if intent.Kind == agentcore.CancelTurn {
		return svc.handleCancellation(ctx, chatID, message)
	}

	existingMessages, err := svc.ChatMessages.FindByChatID(ctx, chatID)
	if err != nil {
		return agentcore.WorkflowCard{}, err
	}

	// content_type is a strict DB enum (agent_chat_messages_content_type_check)
	// that does not include a "hidden" variant, and extending it needs a
	// migration — ui_props is a plain, unconstrained JSON column already used
	// for arbitrary per-card data, so the hidden marker lives there instead.
	var uiProps datatypes.JSON
	if hidden {
		uiProps = datatypes.JSON(`{"hidden":true}`)
	}
	userMessage, err := svc.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
		ChatID: chatID, Sender: "user", ContentType: "text", Content: message, UIProps: uiProps,
	})
	if err != nil {
		return agentcore.WorkflowCard{}, fmt.Errorf("agent: persist user message: %w", err)
	}

	return svc.runForMessage(ctx, chatID, userMessage, append(existingMessages, userMessage), strings.ToLower(wallet), intent)
}

// RetryLastMessage takes the same turn slot: a retry issued while the original
// turn is still running would otherwise start a second turn concurrently.
func (svc *ChatService) RetryLastMessage(ctx context.Context, chatID uuid.UUID, wallet string) (card agentcore.WorkflowCard, err error) {
	release, ok := svc.tryStartTurn(chatID)
	if !ok {
		return agentcore.WorkflowCard{}, agentcore.ErrTurnInFlight
	}
	defer release()

	svc.announceTurn(chatID, "running")
	defer func() { svc.announceTurn(chatID, turnStatus(err)) }()

	turnCtx, cancel := turnContext(ctx)
	defer cancel()
	return svc.retryLastMessage(turnCtx, chatID, wallet)
}

func (svc *ChatService) retryLastMessage(ctx context.Context, chatID uuid.UUID, wallet string) (agentcore.WorkflowCard, error) {
	chat, found, err := svc.Chats.FindByID(ctx, chatID)
	if err != nil {
		return agentcore.WorkflowCard{}, err
	}
	if !found {
		return agentcore.WorkflowCard{}, agentcore.ErrNothingToRetry
	}
	if !strings.EqualFold(chat.OwnerWallet, wallet) {
		return agentcore.WorkflowCard{}, agentcore.ErrWalletMismatch
	}

	existingMessages, err := svc.ChatMessages.FindByChatID(ctx, chatID)
	if err != nil {
		return agentcore.WorkflowCard{}, err
	}
	lastMessage, ok := lastUserMessage(existingMessages)
	if !ok {
		return agentcore.WorkflowCard{}, agentcore.ErrNothingToRetry
	}

	return svc.runForMessage(ctx, chatID, lastMessage, existingMessages, strings.ToLower(wallet), svc.resolveTurn(ctx, chatID, lastMessage.Content))
}

func (svc *ChatService) interrupts() *agentcore.InterruptStore {
	return agentcore.NewInterruptStore(svc.CheckPointStore)
}

// tryStartTurn claims the chat's single turn slot. A nil registry (unit tests
// that build ChatService directly) means "no guard", not a panic.
func (svc *ChatService) tryStartTurn(chatID uuid.UUID) (func(), bool) {
	if svc.Turns == nil {
		return func() {}, true
	}
	return svc.Turns.TryStart(chatID)
}

func turnStatus(err error) string {
	if err != nil {
		return "failed"
	}
	return "done"
}

func (svc *ChatService) announceTurn(chatID uuid.UUID, status string) {
	orchestrator.PublishChatEvent(chatID, "turn", map[string]any{"status": status})
}

// turnContext detaches the turn from the caller's connection while keeping its
// values, and bounds it with its own timeout.
func turnContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), turnTimeout)
}

// TurnStatus is the authoritative "is a turn running for this chat" answer for
// a client that has just mounted. A chat with no row yet is simply idle.
func (svc *ChatService) TurnStatus(ctx context.Context, chatID uuid.UUID, wallet string) (running bool, startedAt time.Time, err error) {
	chat, found, err := svc.Chats.FindByID(ctx, chatID)
	if err != nil {
		return false, time.Time{}, err
	}
	if !found {
		return false, time.Time{}, nil
	}
	if !strings.EqualFold(chat.OwnerWallet, wallet) {
		return false, time.Time{}, agentcore.ErrWalletMismatch
	}
	if svc.Turns == nil {
		return false, time.Time{}, nil
	}
	running, startedAt = svc.Turns.Status(chatID)
	return running, startedAt, nil
}

// lastUserMessage is the message a retry re-runs: the chat's most recent row,
// but only when it is the user's own. A failed turn deliberately persists no
// supervisor bubble (see runForMessage), so this stays true after a failure.
func lastUserMessage(messages []model.AgentChatMessage) (model.AgentChatMessage, bool) {
	if len(messages) == 0 {
		return model.AgentChatMessage{}, false
	}
	last := messages[len(messages)-1]
	if last.Sender != "user" {
		return model.AgentChatMessage{}, false
	}
	return last, true
}

func (svc *ChatService) handleCancellation(ctx context.Context, chatID uuid.UUID, message string) (agentcore.WorkflowCard, error) {
	svc.interrupts().Clear(ctx, chatID)

	if _, err := svc.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
		ChatID: chatID, Sender: "user", ContentType: "text", Content: message,
	}); err != nil {
		slog.ErrorContext(ctx, "agent: persist user cancel message failed", "error", err)
	}

	existingMessages, _ := svc.ChatMessages.FindByChatID(ctx, chatID)
	cancelReply := svc.Orchestrator.CancellationReply(ctx, "the asset", existingMessages)

	replyRow, err := svc.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
		ChatID:      chatID,
		Sender:      "supervisor",
		ContentType: "text",
		Content:     cancelReply,
	})
	if err != nil {
		slog.ErrorContext(ctx, "agent: persist supervisor cancel reply failed", "error", err)
	}

	card := agentcore.WorkflowCard{Reply: cancelReply, ContentType: "text", Messages: existingMessages}
	if replyRow.ID != 0 {
		card.Messages = append(card.Messages, replyRow)
	}

	orchestrator.PublishChatEvent(chatID, "reply_delta", map[string]any{"delta": cancelReply})
	return card, nil
}

// runForMessage delegates the whole turn to Orchestrator.Send and publishes
// the terminal chat event itself — "error" on a failed turn, "final" once the
// reply is persisted — so the HTTP handler only maps errors to status codes.
// Validation errors (wallet mismatch, nothing to retry) return before this,
// so they never publish.
func (svc *ChatService) runForMessage(ctx context.Context, chatID uuid.UUID, userMessage model.AgentChatMessage, allMessages []model.AgentChatMessage, wallet string, intent agentcore.TurnIntent) (agentcore.WorkflowCard, error) {
	sourceMessageID := userMessage.ID

	result, err := svc.Orchestrator.Send(ctx, orchestrator.OrchestratorInput{
		ChatID:          chatID,
		Wallet:          wallet,
		RawPrompt:       userMessage.Content,
		Messages:        agentcore.ChatHistory(allMessages),
		SourceMessageID: &sourceMessageID,
	}, intent)
	if err != nil {
		// No failure bubble is persisted. The HTTP 500 (and this event) already
		// carry the failure, and leaving the user's own message as the chat's
		// last row keeps RetryLastMessage — and the frontend's Retry button —
		// working for exactly this chat id.
		orchestrator.PublishChatEvent(chatID, "error", map[string]any{"message": "Failed to process chat message"})
		return agentcore.WorkflowCard{}, fmt.Errorf("agent: orchestrator run failed: %w", err)
	}

	replyRow, card := svc.persistSupervisorReply(ctx, chatID, result)
	// The response is the chat's state: the history this turn read plus the
	// row it just wrote. The client replaces its list with it.
	card.Messages = allMessages
	if replyRow.ID != 0 {
		card.Messages = append(card.Messages, replyRow)
	}
	orchestrator.PublishChatEvent(chatID, "final", card)
	return card, nil
}

func (svc *ChatService) persistSupervisorReply(ctx context.Context, chatID uuid.UUID, result orchestrator.OrchestratorResult) (model.AgentChatMessage, agentcore.WorkflowCard) {
	card := agentcore.WorkflowCard{TaskID: result.TaskID, Reply: result.Reply, ContentType: result.ContentType, UIProps: result.UIProps}
	if result.UIComponent != "" {
		component := result.UIComponent
		card.UIComponent = &component
	}

	var uiProps datatypes.JSON
	if len(card.UIProps) > 0 {
		uiProps = datatypes.JSON(card.UIProps)
	}
	var refTaskID *int64
	if result.TaskID != 0 {
		refTaskID = &result.TaskID
	}
	row, err := svc.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
		ChatID:      chatID,
		Sender:      "supervisor",
		ContentType: card.ContentType,
		Content:     card.Reply,
		UIComponent: card.UIComponent,
		UIProps:     uiProps,
		UIRefTaskID: refTaskID,
	})
	if err != nil {
		slog.ErrorContext(ctx, "agent: persist supervisor reply failed", "error", err)
	}

	return row, card
}

// isCancellationAction detects whether a user message signals intent to
// abort an open trade confirmation, via an explicit button payload.
func isCancellationAction(msg string) bool {
	lower := strings.ToLower(strings.TrimSpace(msg))
	if lower == "" {
		return false
	}
	return lower == "cancel" ||
		lower == "cancel_trade" ||
		lower == "batal" ||
		lower == "batalkan" ||
		strings.Contains(lower, `"action":"cancel"`) ||
		strings.Contains(lower, `"action":"cancel_trade"`) ||
		strings.Contains(lower, `"action": "cancel"`) ||
		strings.Contains(lower, `"action": "cancel_trade"`)
}
