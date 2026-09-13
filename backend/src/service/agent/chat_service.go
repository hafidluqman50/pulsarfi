package agent

import (
	"context"
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

// ChatService manages chat threads, message histories, and orchestrates
// the chat-facing AI turn. Extracted from task_service.go to enforce
// Single Responsibility: conversation domain stays here, financial task
// and on-chain execution domain stays in TaskService.
type ChatService struct {
	Chats           *repository.AgentChatRepository
	ChatMessages    *repository.AgentChatMessageRepository
	Orchestrator    *Orchestrator
	CheckPointStore *PostgresCheckPointStore
}

func (s *ChatService) ListChats(ctx context.Context, walletAddress string) ([]model.AgentChat, error) {
	return s.Chats.FindByOwnerWallet(ctx, strings.ToLower(walletAddress))
}

func (s *ChatService) GetChatMessages(ctx context.Context, chatID uuid.UUID, walletAddress string) ([]model.AgentChatMessage, error) {
	chat, found, err := s.Chats.FindByID(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if !found {
		return []model.AgentChatMessage{}, nil
	}
	if !strings.EqualFold(chat.OwnerWallet, walletAddress) {
		return nil, ErrWalletMismatch
	}
	return s.ChatMessages.FindByChatID(ctx, chatID)
}

func (s *ChatService) HandleChatMessage(ctx context.Context, chatID uuid.UUID, wallet, message string, events AgentEventCallbacks) (WorkflowCard, error) {
	chat, err := s.Chats.FindOrCreate(ctx, chatID, strings.ToLower(wallet), truncateRunes(message, 50))
	if err != nil {
		return WorkflowCard{}, err
	}
	if !strings.EqualFold(chat.OwnerWallet, wallet) {
		return WorkflowCard{}, ErrWalletMismatch
	}

	// 1. Check for active pending trade checkpoint (HITL)
	var pendingCP *PendingTradeCheckpoint
	var hasCheckpoint bool
	if s.CheckPointStore != nil {
		pendingCP, hasCheckpoint, _ = s.CheckPointStore.GetPendingTrade(ctx, chatID.String())
	}

	// 2. Explicit cancellation intent via structured button action when a trade confirmation is pending
	if hasCheckpoint && isCancellationAction(message) {
		_ = s.CheckPointStore.DeletePendingTrade(ctx, chatID.String())

		// Persist user's cancellation message
		if _, err := s.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
			ChatID: chatID, Sender: "user", ContentType: "text", Content: message,
		}); err != nil {
			slog.ErrorContext(ctx, "agent: persist user cancel message failed", "error", err)
		}

		cancelReply := "Trade plan cancelled. No orders were executed or recorded on-chain."
		ticker := ""
		if pendingCP != nil {
			if pendingCP.ResolvedTicker != "" {
				ticker = pendingCP.ResolvedTicker
			} else if pendingCP.MentionedTicker != "" {
				ticker = pendingCP.MentionedTicker
			}
		}
		if ticker != "" {
			cancelReply = fmt.Sprintf("Trade plan for %s cancelled. No orders were executed or recorded on-chain.", ticker)
		}

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

	existingMessages, err := s.ChatMessages.FindByChatID(ctx, chatID)
	if err != nil {
		return WorkflowCard{}, err
	}

	userMessage, err := s.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
		ChatID: chatID, Sender: "user", ContentType: "text", Content: message,
	})
	if err != nil {
		return WorkflowCard{}, fmt.Errorf("agent: persist user message: %w", err)
	}

	return s.runForMessage(ctx, chatID, userMessage, append(existingMessages, userMessage), strings.ToLower(wallet), events, pendingCP)
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

	var pendingCP *PendingTradeCheckpoint
	if s.CheckPointStore != nil {
		pendingCP, _, _ = s.CheckPointStore.GetPendingTrade(ctx, chatID.String())
	}

	return s.runForMessage(ctx, chatID, lastMessage, existingMessages, strings.ToLower(wallet), events, pendingCP)
}

// runForMessage delegates the whole route/analyze/execute/reply turn to the
// Orchestrator (orchestrator_service.go) — build history, call the graph, persist the
// reply. The Orchestrator owns Task creation, hash-chain recording, and the
// on-chain createTask/RecordSubTasks calls itself.
func (s *ChatService) runForMessage(ctx context.Context, chatID uuid.UUID, userMessage model.AgentChatMessage, allMessages []model.AgentChatMessage, wallet string, events AgentEventCallbacks, pendingCP *PendingTradeCheckpoint) (WorkflowCard, error) {
	sourceMessageID := userMessage.ID

	history := make([]*schema.Message, len(allMessages))
	for i, m := range allMessages {
		if m.Sender == "user" {
			history[i] = schema.UserMessage(m.Content)
		} else {
			history[i] = schema.AssistantMessage(m.Content, nil)
		}
	}

	result, err := s.Orchestrator.Run(ctx, OrchestratorInput{
		ChatID:              chatID,
		Wallet:              wallet,
		RawPrompt:           userMessage.Content,
		Messages:            history,
		SourceMessageID:     &sourceMessageID,
		AgentEventCallbacks: events,
	})
	if err != nil {
		// Zero fallback (docs/plans/agent-orchestration-graph-rebuild.md v2.5):
		// the failure itself is never hidden or faked, but the user must
		// still hear about it from Quasar, in the chat, not just a generic
		// system string surfaced by the transport layer. Persisted like any
		// other supervisor reply — visible on reload, not just a one-time
		// toast — deliberately a static string, never a second LLM call,
		// since the thing that just failed might be the LLM/API itself.
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

	// Handle Checkpoint lifecycle
	if s.CheckPointStore != nil {
		if result.ContentType == "workflow_card" {
			// Confirmation gate active: save or update checkpoint
			newCP := PendingTradeCheckpoint{
				ChatID:           chatID,
				MentionedTicker:  result.Decision.MentionedTicker,
				ResolvedTicker:   result.ResolvedTicker,
				Shape:            result.Decision.Shape,
				Side:             result.Decision.Side,
				Summary:          result.Decision.Summary,
				PendingQuestions: result.PendingQuestions,
				CreatedAt:        time.Now(),
			}
			_ = s.CheckPointStore.SetPendingTrade(ctx, chatID.String(), newCP)
		} else if result.TaskID != 0 {
			// Trade confirmed and executed: delete checkpoint
			_ = s.CheckPointStore.DeletePendingTrade(ctx, chatID.String())
		} else if pendingCP != nil {
			// Side-question or informational turn: retain checkpoint (keep-alive)
			// and append footnote reminder about the pending trade.
			ticker := pendingCP.ResolvedTicker
			if ticker == "" {
				ticker = pendingCP.MentionedTicker
			}
			reminder := "\n\n*Catatan: Rencana trading Anda di atas masih menunggu konfirmasi. Anda dapat melanjutkan kapan saja melalui kartu konfirmasi atau chat.*"
			if ticker != "" {
				reminder = fmt.Sprintf("\n\n*Catatan: Rencana trading %s Anda masih menunggu konfirmasi. Anda dapat melanjutkan kapan saja melalui kartu konfirmasi atau chat.*", ticker)
			}
			result.Reply += reminder
			if events.OnTextDelta != nil {
				events.OnTextDelta(reminder)
			}
		}
	}

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

// isCancellationAction detects whether a user message signals intent to abort
// an open trade confirmation via explicit button payload.
func isCancellationAction(msg string) bool {
	lower := strings.ToLower(strings.TrimSpace(msg))
	if lower == "" {
		return false
	}
	return lower == "cancel" ||
		lower == "cancel_trade" ||
		strings.Contains(lower, `"action":"cancel"`) ||
		strings.Contains(lower, `"action":"cancel_trade"`) ||
		strings.Contains(lower, `"action": "cancel"`) ||
		strings.Contains(lower, `"action": "cancel_trade"`)
}
