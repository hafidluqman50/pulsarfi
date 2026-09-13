package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"gorm.io/gorm"
)

type AgentChatRepository struct {
	DB *gorm.DB
}

func (r *AgentChatRepository) FindByID(ctx context.Context, id uuid.UUID) (model.AgentChat, bool, error) {
	var chat model.AgentChat
	err := r.DB.WithContext(ctx).First(&chat, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.AgentChat{}, false, nil
	}
	return chat, err == nil, err
}

func (r *AgentChatRepository) FindByOwnerWallet(ctx context.Context, ownerWallet string) ([]model.AgentChat, error) {
	var chats []model.AgentChat
	err := r.DB.WithContext(ctx).
		Where("owner_wallet = ?", ownerWallet).
		Order("created_at DESC").
		Find(&chats).Error
	return chats, err
}

func (r *AgentChatRepository) FindOrCreate(ctx context.Context, id uuid.UUID, ownerWallet, description string) (model.AgentChat, error) {
	var chat model.AgentChat
	err := r.DB.WithContext(ctx).First(&chat, "id = ?", id).Error
	if err == nil {
		return chat, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.AgentChat{}, err
	}
	chat = model.AgentChat{ID: id, OwnerWallet: ownerWallet, Description: &description}
	return chat, r.DB.WithContext(ctx).Create(&chat).Error
}
