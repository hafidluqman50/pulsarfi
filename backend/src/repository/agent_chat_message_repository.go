package repository

import (
	"context"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type AgentChatMessageRepository struct {
	DB *gorm.DB
}

func (r *AgentChatMessageRepository) FindByChatID(ctx context.Context, chatID int64) ([]model.AgentChatMessage, error) {
	var messages []model.AgentChatMessage
	err := r.DB.WithContext(ctx).
		Where("chat_id = ?", chatID).
		Order("created_at ASC").
		Find(&messages).Error
	return messages, err
}

type AgentChatMessageCreateInput struct {
	ChatID      int64
	Sender      string
	ContentType string
	Content     string
	UIComponent *string
	UIProps     datatypes.JSON
	UIRefTaskID *int64
}

func (r *AgentChatMessageRepository) Create(ctx context.Context, input AgentChatMessageCreateInput) (model.AgentChatMessage, error) {
	message := model.AgentChatMessage{
		ChatID:      input.ChatID,
		Sender:      input.Sender,
		ContentType: input.ContentType,
		Content:     input.Content,
		UIComponent: input.UIComponent,
		UIProps:     input.UIProps,
		UIRefTaskID: input.UIRefTaskID,
	}
	return message, r.DB.WithContext(ctx).Create(&message).Error
}
