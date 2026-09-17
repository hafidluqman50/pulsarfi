package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	dbmodel "github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"gorm.io/datatypes"
)

type TaskScheduler struct {
	Tasks        *repository.AgentTaskRepository
	ChatMessages *repository.AgentChatMessageRepository
	Chats        *repository.AgentChatRepository
	TaskService  *TaskService
	Model        model.BaseChatModel
	interval     time.Duration
	stopCh       chan struct{}
}

func NewTaskScheduler(tasks *repository.AgentTaskRepository, chatMessages *repository.AgentChatMessageRepository, chats *repository.AgentChatRepository, taskService *TaskService, m model.BaseChatModel) *TaskScheduler {
	return &TaskScheduler{
		Tasks:        tasks,
		ChatMessages: chatMessages,
		Chats:        chats,
		TaskService:  taskService,
		Model:        m,
		interval:     60 * time.Second,
		stopCh:       make(chan struct{}),
	}
}

func (s *TaskScheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	go func() {
		slog.InfoContext(ctx, "agent: task scheduler background worker started", "interval", s.interval)
		for {
			select {
			case <-s.stopCh:
				ticker.Stop()
				slog.InfoContext(ctx, "agent: task scheduler background worker stopped")
				return
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				s.runCycle(ctx)
			}
		}
	}()
}

func (s *TaskScheduler) Stop() {
	close(s.stopCh)
}

func (s *TaskScheduler) runCycle(ctx context.Context) {
	s.processDueRecurringTasks(ctx)
	s.processHorizonNotices(ctx)
}

func (s *TaskScheduler) processDueRecurringTasks(ctx context.Context) {
	if s.Tasks == nil || s.TaskService == nil {
		return
	}
	tasks, err := s.Tasks.FindDueRecurringTasks(ctx, 10)
	if err != nil {
		slog.ErrorContext(ctx, "scheduler: query due recurring tasks failed", "error", err)
		return
	}
	for _, t := range tasks {
		slog.InfoContext(ctx, "scheduler: triggering due recurring task tranche", "task_id", t.ID)
		res, err := s.TaskService.ExecuteTask(ctx, t.ID, t.WalletAddress)
		if err != nil {
			slog.ErrorContext(ctx, "scheduler: execute recurring task failed", "task_id", t.ID, "error", err)
			continue
		}
		slog.InfoContext(ctx, "scheduler: recurring task tranche executed", "task_id", t.ID, "status", res.Status)
	}
}

func (s *TaskScheduler) processHorizonNotices(ctx context.Context) {
	if s.Tasks == nil || s.ChatMessages == nil || s.Chats == nil {
		return
	}
	// Check tasks within 24 hours of horizon expiry
	tasks, err := s.Tasks.FindPendingHorizonTasks(ctx, 24*time.Hour, 10)
	if err != nil {
		slog.ErrorContext(ctx, "scheduler: query pending horizon tasks failed", "error", err)
		return
	}
	for _, t := range tasks {
		s.dispatchHorizonNotice(ctx, t)
	}
}

type HorizonNoticeCardLabels struct {
	Badge             string `json:"badge"`
	TimeRemainingLabel string `json:"time_remaining_label"`
	Headline          string `json:"headline"`
	CustodialNotice   string `json:"custodial_notice"`
	CloseButton       string `json:"close_button"`
	LeaveButton       string `json:"leave_button"`
	ProcessingButton  string `json:"processing_button"`
	CloseSettledText  string `json:"close_settled_text"`
	LeaveSettledText  string `json:"leave_settled_text"`
	CloseToastTitle   string `json:"close_toast_title"`
	CloseToastDesc    string `json:"close_toast_desc"`
	LeaveToastTitle   string `json:"leave_toast_title"`
	LeaveToastDesc    string `json:"leave_toast_desc"`
	ClosePrompt       string `json:"close_prompt"`
}

type HorizonNoticeLLMResponse struct {
	Content string                   `json:"content"`
	Card    HorizonNoticeCardLabels `json:"card"`
}

func (s *TaskScheduler) dispatchHorizonNotice(ctx context.Context, t dbmodel.AgentTask) {
	var chatID uuid.UUID
	if t.SourceMessageID != nil {
		if msg, found, err := s.ChatMessages.FindByID(ctx, *t.SourceMessageID); err == nil && found {
			chatID = msg.ChatID
		}
	}
	if chatID == uuid.Nil {
		if chats, err := s.Chats.FindByOwnerWallet(ctx, t.WalletAddress); err == nil && len(chats) > 0 {
			chatID = chats[0].ID
		}
	}
	if chatID == uuid.Nil {
		slog.WarnContext(ctx, "scheduler: no chat found for horizon notice", "task_id", t.ID, "wallet", t.WalletAddress)
		return
	}

	ticker := "ASET"
	side := "buy"
	shape := ""
	if t.TriggerDescription != nil {
		var td map[string]any
		if err := json.Unmarshal([]byte(*t.TriggerDescription), &td); err == nil {
			if rTicker, ok := td["resolved_ticker"].(string); ok && rTicker != "" {
				ticker = rTicker
			}
			if s, ok := td["side"].(string); ok && s != "" {
				side = s
			}
			if sh, ok := td["shape"].(string); ok {
				shape = sh
			}
		}
	}

	// This notice is written entirely around "your swing position is
	// nearing its holding horizon" — scalp's own horizon_expires_at is a
	// same-day trade-permission validity window, not a position anyone is
	// holding, so it has nothing to prompt an exit/keep decision about.
	// Still mark it notified so the scheduler stops re-checking it forever.
	if shape != "swing" && shape != "investment" {
		if err := s.Tasks.MarkHorizonNotified(ctx, t.ID); err != nil {
			slog.ErrorContext(ctx, "scheduler: mark non-swing horizon task notified failed", "task_id", t.ID, "error", err)
		}
		return
	}

	hoursLeft := 24
	if t.HorizonExpiresAt != nil {
		diff := time.Until(*t.HorizonExpiresAt)
		if diff > 0 {
			hoursLeft = int(diff.Hours())
		}
	}

	content := fmt.Sprintf("Horizon alert for %s: approximately %d hours remaining before target horizon.", ticker, hoursLeft)
	var cardLabels HorizonNoticeCardLabels

	// Dynamically generate notice content and card labels matching the user's active conversation language
	if s.Model != nil {
		var chatHistory []*schema.Message
		if chatMsgs, err := s.ChatMessages.FindByChatID(ctx, chatID); err == nil && len(chatMsgs) > 0 {
			limit := 10
			if len(chatMsgs) < limit {
				limit = len(chatMsgs)
			}
			for _, m := range chatMsgs[len(chatMsgs)-limit:] {
				if m.Sender == "user" {
					chatHistory = append(chatHistory, schema.UserMessage(m.Content))
				} else {
					chatHistory = append(chatHistory, schema.AssistantMessage(m.Content, nil))
				}
			}
		}

		prompt := fmt.Sprintf(`You are Quasar, PulsarFi's AI trading assistant.
A swing trading position for stock ticker %s is approaching its horizon expiration (%d hours remaining).

CRITICAL USER-DRIVEN LANGUAGE POLICY:
Detect and mirror 100%% the exact language or dialect the user used in the chat history (Indonesian, English, Turkish, Javanese, Banjar, etc.).
NEVER hardcode or assume any single language. Do NOT use em dash characters.

Respond with ONLY a single JSON object, no prose, no markdown fences:
{
  "content": "Single friendly, concise chat message in the user's active language notifying them that their swing position in %s is nearing its horizon deadline, reminding that PulsarFi is self-custodial so tokens never leave without signature, and asking whether they want to exit or keep it.",
  "card": {
    "badge": "Short badge title in the user's active language (e.g. 'SWING HORIZON ALERT · H-1')",
    "time_remaining_label": "Localized phrase for %d hours remaining in the user's active language",
    "headline": "Headline stating that the %s swing position is nearing expiration in the user's active language",
    "custodial_notice": "Self-custodial notice explaining that assets never move without explicit user approval in the user's active language",
    "close_button": "Button label to exit position (sell to IDRX) in the user's active language",
    "leave_button": "Button label to keep position in portfolio in the user's active language",
    "processing_button": "Button progress label while processing in the user's active language",
    "close_settled_text": "Confirmation statement that sell order was submitted in the user's active language",
    "leave_settled_text": "Confirmation statement that position was retained in the user's active language",
    "close_toast_title": "Toast title for exit in the user's active language",
    "close_toast_desc": "Toast description for exit in the user's active language",
    "leave_toast_title": "Toast title for keep in the user's active language",
    "leave_toast_desc": "Toast description for keep in the user's active language",
    "close_prompt": "Command prompt text to send in chat to execute exit in the user's active language"
  }
}`, ticker, hoursLeft, ticker, hoursLeft, ticker)

		llmMsgs := append([]*schema.Message{schema.SystemMessage(prompt)}, chatHistory...)
		if resp, err := s.Model.Generate(ctx, llmMsgs); err == nil && resp != nil {
			cleanJSON := strings.TrimSpace(resp.Content)
			cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
			cleanJSON = strings.TrimPrefix(cleanJSON, "```")
			cleanJSON = strings.TrimSuffix(cleanJSON, "```")
			cleanJSON = strings.TrimSpace(cleanJSON)
			var parsed HorizonNoticeLLMResponse
			if err := json.Unmarshal([]byte(cleanJSON), &parsed); err == nil && parsed.Content != "" {
				content = parsed.Content
				cardLabels = parsed.Card
			}
		}
	}

	uiPropsBytes, _ := json.Marshal(map[string]any{
		"task_id":        t.ID,
		"ticker":         ticker,
		"side":           side,
		"time_remaining": fmt.Sprintf("%d hours", hoursLeft),
		"expires_at":     t.HorizonExpiresAt,
		"card":           cardLabels,
	})

	uiComp := "HorizonNoticeCard"
	_, err := s.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
		ChatID:      chatID,
		Sender:      "supervisor",
		ContentType: "horizon_notice",
		Content:     content,
		UIComponent: &uiComp,
		UIProps:     datatypes.JSON(uiPropsBytes),
		UIRefTaskID: &t.ID,
	})
	if err != nil {
		slog.ErrorContext(ctx, "scheduler: create horizon notice chat message failed", "task_id", t.ID, "error", err)
		return
	}

	if err := s.Tasks.MarkHorizonNotified(ctx, t.ID); err != nil {
		slog.ErrorContext(ctx, "scheduler: mark horizon notified failed", "task_id", t.ID, "error", err)
	}
}
