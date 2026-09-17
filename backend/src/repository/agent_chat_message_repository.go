package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type AgentChatMessageRepository struct {
	DB *gorm.DB
}

func (r *AgentChatMessageRepository) FindByID(ctx context.Context, id int64) (model.AgentChatMessage, bool, error) {
	var message model.AgentChatMessage
	err := r.DB.WithContext(ctx).First(&message, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.AgentChatMessage{}, false, nil
	}
	return message, err == nil, err
}

func (r *AgentChatMessageRepository) FindByChatID(ctx context.Context, chatID uuid.UUID) ([]model.AgentChatMessage, error) {
	var messages []model.AgentChatMessage
	err := r.DB.WithContext(ctx).
		Where("chat_id = ?", chatID).
		Order("created_at ASC").
		Find(&messages).Error
	return messages, err
}

func (r *AgentChatMessageRepository) FindByUIRefTaskID(ctx context.Context, taskID int64) (model.AgentChatMessage, bool, error) {
	var message model.AgentChatMessage
	err := r.DB.WithContext(ctx).First(&message, "ui_ref_task_id = ?", taskID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.AgentChatMessage{}, false, nil
	}
	return message, err == nil, err
}

type AgentChatMessageCreateInput struct {
	ChatID      uuid.UUID
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
