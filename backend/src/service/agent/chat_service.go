package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"gorm.io/datatypes"
)

var ErrChatNotFound = errors.New("agent: chat not found")

// ChatService manages chat threads, user messages, and supervisor replies.
// It delegates the entire route/analyze/execute/reply turn to Orchestrator —
// pause and resume are handled natively by Eino's compose.CheckPointStore,
// never by any hand-rolled state of ChatService's own
// (docs/plans/quasar-clean-routing-scalp-refactor.md Finding #8). This
// package never builds LLM prompts or invokes an agent directly; that stays
// inside Orchestrator.
type ChatService struct {
	Chats           *repository.AgentChatRepository
	ChatMessages    *repository.AgentChatMessageRepository
	Orchestrator    *Orchestrator
	CheckPointStore compose.CheckPointStore
}

func (s *ChatService) ListChats(ctx context.Context, wallet string) ([]model.AgentChat, error) {
	return s.Chats.FindByOwnerWallet(ctx, strings.ToLower(wallet))
}

func (s *ChatService) GetChat(ctx context.Context, chatID uuid.UUID, wallet string) (model.AgentChat, []model.AgentChatMessage, error) {
	chat, found, err := s.Chats.FindByID(ctx, chatID)
	if err != nil {
		return model.AgentChat{}, nil, err
	}
	if !found {
		return model.AgentChat{}, nil, ErrChatNotFound
	}
	if !strings.EqualFold(chat.OwnerWallet, wallet) {
		return model.AgentChat{}, nil, ErrWalletMismatch
	}
	messages, err := s.ChatMessages.FindByChatID(ctx, chatID)
	if err != nil {
		return model.AgentChat{}, nil, err
	}
	return chat, messages, nil
}

func (s *ChatService) GetChatMessages(ctx context.Context, chatID uuid.UUID, wallet string) ([]model.AgentChatMessage, error) {
	_, messages, err := s.GetChat(ctx, chatID, wallet)
	return messages, err
}

// HandleChatMessage persists the user's message and either resumes a paused
// graph run or starts a fresh one, depending on whether an active checkpoint
// exists for this chat. hidden marks a message as real chat history (still
// persisted, still fed to the LLM as context, still returned by every read)
// that must never render as a bubble in the UI — the compiled answer a
// clarifying-questions card sends, not something the user typed by hand.
func (s *ChatService) HandleChatMessage(ctx context.Context, chatID uuid.UUID, wallet, message string, hidden bool, events AgentEventCallbacks) (WorkflowCard, error) {
	chat, err := s.Chats.FindOrCreate(ctx, chatID, strings.ToLower(wallet), truncateRunes(message, 50))
	if err != nil {
		return WorkflowCard{}, err
	}
	if !strings.EqualFold(chat.OwnerWallet, wallet) {
		return WorkflowCard{}, ErrWalletMismatch
	}

	hasCheckpoint := s.hasActiveCheckpoint(ctx, chatID)

	if hasCheckpoint && isCancellationAction(message) {
		return s.handleCancellation(ctx, chatID, message, events)
	}

	existingMessages, err := s.ChatMessages.FindByChatID(ctx, chatID)
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
	userMessage, err := s.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
		ChatID: chatID, Sender: "user", ContentType: "text", Content: message, UIProps: uiProps,
	})
	if err != nil {
		return WorkflowCard{}, fmt.Errorf("agent: persist user message: %w", err)
	}

	return s.runForMessage(ctx, chatID, userMessage, append(existingMessages, userMessage), strings.ToLower(wallet), events)
}

func (s *ChatService) RetryLastMessage(ctx context.Context, chatID uuid.UUID, wallet string, events AgentEventCallbacks) (WorkflowCard, error) {
	chat, found, err := s.Chats.FindByID(ctx, chatID)
	if err != nil {
		return WorkflowCard{}, err
	}
	if !found {
		return WorkflowCard{}, ErrNothingToRetry
	}
	if !strings.EqualFold(chat.OwnerWallet, wallet) {
		return WorkflowCard{}, ErrWalletMismatch
	}

	existingMessages, err := s.ChatMessages.FindByChatID(ctx, chatID)
	if err != nil {
		return WorkflowCard{}, err
	}
	if len(existingMessages) == 0 {
		return WorkflowCard{}, ErrNothingToRetry
	}
	lastMessage := existingMessages[len(existingMessages)-1]
	if lastMessage.Sender != "user" {
		return WorkflowCard{}, ErrNothingToRetry
	}

	return s.runForMessage(ctx, chatID, lastMessage, existingMessages, strings.ToLower(wallet), events)
}

func (s *ChatService) hasActiveCheckpoint(ctx context.Context, chatID uuid.UUID) bool {
	cpStore, ok := s.CheckPointStore.(ExtendedCheckPointStore)
	if !ok {
		return false
	}
	has, _ := cpStore.Has(ctx, chatID.String())
	return has
}

func (s *ChatService) handleCancellation(ctx context.Context, chatID uuid.UUID, message string, events AgentEventCallbacks) (WorkflowCard, error) {
	if cpStore, ok := s.CheckPointStore.(ExtendedCheckPointStore); ok {
		_ = cpStore.Delete(ctx, chatID.String())
	}

	if _, err := s.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
		ChatID: chatID, Sender: "user", ContentType: "text", Content: message,
	}); err != nil {
		slog.ErrorContext(ctx, "agent: persist user cancel message failed", "error", err)
	}

	existingMessages, _ := s.ChatMessages.FindByChatID(ctx, chatID)
	cancelReply := s.generateCancellationReply(ctx, "the asset", existingMessages)

	if _, err := s.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
		ChatID:      chatID,
		Sender:      "supervisor",
		ContentType: "text",
		Content:     cancelReply,
	}); err != nil {
		slog.ErrorContext(ctx, "agent: persist supervisor cancel reply failed", "error", err)
	}

	if events.OnTextDelta != nil {
		events.OnTextDelta(cancelReply)
	}
	return WorkflowCard{Reply: cancelReply, ContentType: "text"}, nil
}

// runForMessage delegates the whole turn to Orchestrator. If an active pause
// exists in CheckPointStore, it resumes via Orchestrator.ResumeRoute (feeding
// the new message in as the answer to whatever was pending) rather than
// starting a fresh, stateless decision from the full message history.
func (s *ChatService) runForMessage(ctx context.Context, chatID uuid.UUID, userMessage model.AgentChatMessage, allMessages []model.AgentChatMessage, wallet string, events AgentEventCallbacks) (WorkflowCard, error) {
	sourceMessageID := userMessage.ID

	result, err := s.resumeOrRun(ctx, chatID, userMessage, allMessages, wallet, sourceMessageID, events)
	if err != nil {
		if _, persistErr := s.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
			ChatID:      chatID,
			Sender:      "supervisor",
			ContentType: "text",
			Content:     "Failed to process request. Please try again.",
		}); persistErr != nil {
			slog.ErrorContext(ctx, "agent: persist error message failed", "error", persistErr)
		}
		return WorkflowCard{}, fmt.Errorf("agent: orchestrator run failed: %w", err)
	}

	return s.persistSupervisorReply(ctx, chatID, result)
}

func (s *ChatService) resumeOrRun(ctx context.Context, chatID uuid.UUID, userMessage model.AgentChatMessage, allMessages []model.AgentChatMessage, wallet string, sourceMessageID int64, events AgentEventCallbacks) (OrchestratorResult, error) {
	if cpStore, ok := s.CheckPointStore.(ExtendedCheckPointStore); ok {
		if has, hErr := cpStore.Has(ctx, chatID.String()); hErr == nil && has {
			if interruptID, found, _ := cpStore.GetInterruptID(ctx, chatID.String()); found && interruptID != "" {
				return s.Orchestrator.ResumeRoute(ctx, chatID, interruptID, userMessage.Content, wallet, &sourceMessageID, events)
			}
		}
	}

	history := make([]*schema.Message, len(allMessages))
	for i, m := range allMessages {
		if m.Sender == "user" {
			history[i] = schema.UserMessage(m.Content)
		} else {
			history[i] = schema.AssistantMessage(m.Content, nil)
		}
	}

	return s.Orchestrator.Run(ctx, OrchestratorInput{
		ChatID:              chatID,
		Wallet:              wallet,
		RawPrompt:           userMessage.Content,
		Messages:            history,
		SourceMessageID:     &sourceMessageID,
		AgentEventCallbacks: events,
	})
}

func (s *ChatService) persistSupervisorReply(ctx context.Context, chatID uuid.UUID, result OrchestratorResult) (WorkflowCard, error) {
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
	if _, err := s.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
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

func (s *ChatService) generateCancellationReply(ctx context.Context, ticker string, history []model.AgentChatMessage) string {
	if s.Orchestrator == nil || s.Orchestrator.ReplyModel == nil {
		return ""
	}

	var chatHistory []*schema.Message
	limit := 10
	if len(history) < limit {
		limit = len(history)
	}
	for _, m := range history[len(history)-limit:] {
		if m.Sender == "user" {
			chatHistory = append(chatHistory, schema.UserMessage(m.Content))
		} else {
			chatHistory = append(chatHistory, schema.AssistantMessage(m.Content, nil))
		}
	}

	target := ticker
	if target == "" {
		target = "the asset"
	}

	systemPrompt := fmt.Sprintf(`You are Quasar, PulsarFi's AI trading assistant.
The user has explicitly cancelled the pending trade confirmation for %s.

CRITICAL USER-DRIVEN LANGUAGE POLICY:
Detect and mirror 100%% the user's active language from the chat history (Indonesian, English, Javanese, Turkish, Japanese, etc.).
NEVER hardcode or assume any single language. Do NOT use em dashes.
Produce a single friendly, professional confirmation sentence stating that the trade plan for %s has been cancelled and no orders or on-chain transactions were executed.
Return ONLY the confirmation sentence, with no markdown fences, greetings, or extra commentary.`, target, target)

	msgs := append([]*schema.Message{schema.SystemMessage(systemPrompt)}, chatHistory...)
	resp, err := s.Orchestrator.ReplyModel.Generate(ctx, msgs)
	if err == nil && resp != nil && strings.TrimSpace(resp.Content) != "" {
		return strings.TrimSpace(resp.Content)
	}
	return ""
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
