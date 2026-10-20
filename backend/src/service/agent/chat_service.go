package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"gorm.io/datatypes"
)

var ErrChatNotFound = errors.New("agent: chat not found")

// ErrTurnInFlight is returned when a second turn is requested for a chat that
// already has one running. The handler maps it to 409.
var ErrTurnInFlight = errors.New("agent: a turn is already running for this chat")

// turnTimeout bounds one detached turn. It matches the server's WriteTimeout,
// the longest a live client can wait for the blocking response.
const turnTimeout = 5 * time.Minute

// turnKind is what a chat message turns out to be, once the chat's pending
// interrupt is taken into account. It is the single answer to "where does this
// message go?" — see ChatService.resolveTurn.
type turnKind int

const (
	freshTurn             turnKind = iota // no pause -> start a new turn
	answerQuestions                       // paused on ask_user -> this message IS the answer
	sideQuestionDuringArm                 // paused on await_arm -> new turn, arm pause stays alive
	cancelTurn                            // paused + explicit cancellation
)

// turnIntent is the resolved message: its kind, plus the interrupt it answers
// when it answers one (so the pending marker is read exactly once).
type turnIntent struct {
	kind        turnKind
	interruptID string
}

// FreshTurnIntent is the intent for a message that starts a new turn. Exported
// so a caller outside this package (a test, a tool) can drive a turn without
// going through ChatService's session resolution.
func FreshTurnIntent() turnIntent { return turnIntent{kind: freshTurn} }

// ChatService manages chat threads, user messages, and supervisor replies.
// It decides what each incoming message is (resolveTurn) and persists the
// user message and the supervisor reply, then delegates the whole turn to
// Orchestrator — pause and resume are handled by Eino's checkpoint store
// inside Orchestrator.Send, and every word the user reads is written by the
// model, never here: this type builds no LLM prompt.
type ChatService struct {
	Chats           *repository.AgentChatRepository
	ChatMessages    *repository.AgentChatMessageRepository
	Orchestrator    *Orchestrator
	CheckPointStore ExtendedCheckPointStore
	// Turns is the per-chat single-turn guard. Nil in tests that build the
	// service directly; tryStartTurn treats nil as "no guard".
	Turns *TurnRegistry
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
		return model.AgentChat{}, nil, ErrChatNotFound
	}
	if !strings.EqualFold(chat.OwnerWallet, wallet) {
		return model.AgentChat{}, nil, ErrWalletMismatch
	}
	messages, err := svc.ChatMessages.FindByChatID(ctx, chatID)
	if err != nil {
		return model.AgentChat{}, nil, err
	}
	return chat, messages, nil
}

func (svc *ChatService) GetChatMessages(ctx context.Context, chatID uuid.UUID, wallet string) ([]model.AgentChatMessage, error) {
	_, messages, err := svc.GetChat(ctx, chatID, wallet)
	if errors.Is(err, ErrChatNotFound) {
		// A chat with no row yet has no messages — QuasarPanel's "+ new chat"
		// opens one before the first message is sent, so reading it must be an
		// empty list, not an error. Ownership is unchanged: a chat that exists
		// and belongs to someone else still returns ErrWalletMismatch.
		return []model.AgentChatMessage{}, nil
	}
	return messages, err
}

// resolveTurn is the single decision point: what is this message, given the
// chat's pending interrupt? It reads the pending marker exactly once; Send
// then switches on the result instead of asking again.
func (svc *ChatService) resolveTurn(ctx context.Context, chatID uuid.UUID, message string) turnIntent {
	pending, paused := svc.interrupts().pending(ctx, chatID)
	if !paused {
		return turnIntent{kind: freshTurn}
	}
	if isCancellationAction(message) {
		return turnIntent{kind: cancelTurn}
	}
	if pending.kind == interruptQuestions {
		return turnIntent{kind: answerQuestions, interruptID: pending.id}
	}
	return turnIntent{kind: sideQuestionDuringArm}
}

// HandleChatMessage takes the chat's single turn slot, then runs the turn on a
// context that does not belong to the caller's connection: a refresh or a
// closed tab must not cancel work already committed to. hidden marks a message
// as real chat history (still persisted, still fed to the LLM as context,
// still returned by every read) that must never render as a bubble — the
// compiled answer a clarifying-questions card sends, not something the user
// typed by hand.
func (svc *ChatService) HandleChatMessage(ctx context.Context, chatID uuid.UUID, wallet, message string, hidden bool) (card WorkflowCard, err error) {
	release, ok := svc.tryStartTurn(chatID)
	if !ok {
		return WorkflowCard{}, ErrTurnInFlight
	}
	defer release()

	svc.announceTurn(chatID, "running")
	defer func() { svc.announceTurn(chatID, turnStatus(err)) }()

	turnCtx, cancel := turnContext(ctx)
	defer cancel()
	return svc.handleChatMessage(turnCtx, chatID, wallet, message, hidden)
}

func (svc *ChatService) handleChatMessage(ctx context.Context, chatID uuid.UUID, wallet, message string, hidden bool) (WorkflowCard, error) {
	chat, err := svc.Chats.FindOrCreate(ctx, chatID, strings.ToLower(wallet), truncateRunes(message, 50))
	if err != nil {
		return WorkflowCard{}, err
	}
	if !strings.EqualFold(chat.OwnerWallet, wallet) {
		return WorkflowCard{}, ErrWalletMismatch
	}

	intent := svc.resolveTurn(ctx, chatID, message)
	if intent.kind == cancelTurn {
		return svc.handleCancellation(ctx, chatID, message)
	}

	existingMessages, err := svc.ChatMessages.FindByChatID(ctx, chatID)
	if err != nil {
		return WorkflowCard{}, err
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
		return WorkflowCard{}, fmt.Errorf("agent: persist user message: %w", err)
	}

	return svc.runForMessage(ctx, chatID, userMessage, append(existingMessages, userMessage), strings.ToLower(wallet), intent)
}

// RetryLastMessage takes the same turn slot: a retry issued while the original
// turn is still running would otherwise start a second turn concurrently.
func (svc *ChatService) RetryLastMessage(ctx context.Context, chatID uuid.UUID, wallet string) (card WorkflowCard, err error) {
	release, ok := svc.tryStartTurn(chatID)
	if !ok {
		return WorkflowCard{}, ErrTurnInFlight
	}
	defer release()

	svc.announceTurn(chatID, "running")
	defer func() { svc.announceTurn(chatID, turnStatus(err)) }()

	turnCtx, cancel := turnContext(ctx)
	defer cancel()
	return svc.retryLastMessage(turnCtx, chatID, wallet)
}

func (svc *ChatService) retryLastMessage(ctx context.Context, chatID uuid.UUID, wallet string) (WorkflowCard, error) {
	chat, found, err := svc.Chats.FindByID(ctx, chatID)
	if err != nil {
		return WorkflowCard{}, err
	}
	if !found {
		return WorkflowCard{}, ErrNothingToRetry
	}
	if !strings.EqualFold(chat.OwnerWallet, wallet) {
		return WorkflowCard{}, ErrWalletMismatch
	}

	existingMessages, err := svc.ChatMessages.FindByChatID(ctx, chatID)
	if err != nil {
		return WorkflowCard{}, err
	}
	lastMessage, ok := lastUserMessage(existingMessages)
	if !ok {
		return WorkflowCard{}, ErrNothingToRetry
	}

	return svc.runForMessage(ctx, chatID, lastMessage, existingMessages, strings.ToLower(wallet), svc.resolveTurn(ctx, chatID, lastMessage.Content))
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
	PublishChatEvent(chatID, "turn", map[string]any{"status": status})
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
		return false, time.Time{}, ErrWalletMismatch
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

func (svc *ChatService) handleCancellation(ctx context.Context, chatID uuid.UUID, message string) (WorkflowCard, error) {
	svc.interrupts().clear(ctx, chatID)

	if _, err := svc.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
		ChatID: chatID, Sender: "user", ContentType: "text", Content: message,
	}); err != nil {
		slog.ErrorContext(ctx, "agent: persist user cancel message failed", "error", err)
	}

	existingMessages, _ := svc.ChatMessages.FindByChatID(ctx, chatID)
	cancelReply := svc.Orchestrator.CancellationReply(ctx, "the asset", existingMessages)

	if _, err := svc.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
		ChatID:      chatID,
		Sender:      "supervisor",
		ContentType: "text",
		Content:     cancelReply,
	}); err != nil {
		slog.ErrorContext(ctx, "agent: persist supervisor cancel reply failed", "error", err)
	}

	PublishChatEvent(chatID, "reply_delta", map[string]any{"delta": cancelReply})
	return WorkflowCard{Reply: cancelReply, ContentType: "text"}, nil
}

// runForMessage delegates the whole turn to Orchestrator.Send and publishes
// the terminal chat event itself — "error" on a failed turn, "final" once the
// reply is persisted — so the HTTP handler only maps errors to status codes.
// Validation errors (wallet mismatch, nothing to retry) return before this,
// so they never publish.
func (svc *ChatService) runForMessage(ctx context.Context, chatID uuid.UUID, userMessage model.AgentChatMessage, allMessages []model.AgentChatMessage, wallet string, intent turnIntent) (WorkflowCard, error) {
	sourceMessageID := userMessage.ID

	result, err := svc.Orchestrator.Send(ctx, OrchestratorInput{
		ChatID:          chatID,
		Wallet:          wallet,
		RawPrompt:       userMessage.Content,
		Messages:        chatHistory(allMessages),
		SourceMessageID: &sourceMessageID,
	}, intent)
	if err != nil {
		// No failure bubble is persisted. The HTTP 500 (and this event) already
		// carry the failure, and leaving the user's own message as the chat's
		// last row keeps RetryLastMessage — and the frontend's Retry button —
		// working for exactly this chat id.
		PublishChatEvent(chatID, "error", map[string]any{"message": "Failed to process chat message"})
		return WorkflowCard{}, fmt.Errorf("agent: orchestrator run failed: %w", err)
	}

	card, err := svc.persistSupervisorReply(ctx, chatID, result)
	if err == nil {
		PublishChatEvent(chatID, "final", card)
	}
	return card, err
}

// chatHistory turns stored chat messages into the conversation the model
// reads: the user's messages as user turns, everything else as assistant.
func chatHistory(messages []model.AgentChatMessage) []*schema.Message {
	history := make([]*schema.Message, len(messages))
	for i, message := range messages {
		if message.Sender == "user" {
			history[i] = schema.UserMessage(message.Content)
		} else {
			history[i] = schema.AssistantMessage(message.Content, nil)
		}
	}
	return history
}

func (svc *ChatService) persistSupervisorReply(ctx context.Context, chatID uuid.UUID, result OrchestratorResult) (WorkflowCard, error) {
	card := WorkflowCard{TaskID: result.TaskID, Reply: result.Reply, ContentType: result.ContentType, UIProps: result.UIProps}
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
	if _, err := svc.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
		ChatID:      chatID,
		Sender:      "supervisor",
		ContentType: card.ContentType,
		Content:     card.Reply,
		UIComponent: card.UIComponent,
		UIProps:     uiProps,
		UIRefTaskID: refTaskID,
	}); err != nil {
		slog.ErrorContext(ctx, "agent: persist supervisor reply failed", "error", err)
	}

	return card, nil
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
