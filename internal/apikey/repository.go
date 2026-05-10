package apikey

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, key *APIKey) error
	FindByHash(ctx context.Context, hash string) (*APIKey, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*APIKey, error)
	Revoke(ctx context.Context, keyID, userID uuid.UUID) error
	TouchLastUsed(ctx context.Context, keyID uuid.UUID) error
}

type repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, key *APIKey) error {
	if err := r.db.WithContext(ctx).Create(key).Error; err != nil {
		return fmt.Errorf("apikey.repo.Create: %w", err)
	}
	return nil
}

func (r *repository) FindByHash(ctx context.Context, hash string) (*APIKey, error) {
	var key APIKey
	result := r.db.WithContext(ctx).
		Where("key_hash = ? AND is_active = true AND (expires_at IS NULL OR expires_at > NOW())", hash).
		First(&key)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, gorm.ErrRecordNotFound
	}
	if result.Error != nil {
		return nil, fmt.Errorf("apikey.repo.FindByHash: %w", result.Error)
	}
	return &key, nil
}

func (r *repository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*APIKey, error) {
	var keys []*APIKey
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND is_active = true", userID).
		Order("created_at DESC").
		Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("apikey.repo.ListByUser: %w", err)
	}
	return keys, nil
}

func (r *repository) Revoke(ctx context.Context, keyID, userID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&APIKey{}).
		Where("id = ? AND user_id = ?", keyID, userID).
		Update("is_active", false)
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return result.Error
}

// TouchLastUsed updates last_used_at asynchronously — called in a goroutine
// so it never adds latency to the authenticated request.
func (r *repository) TouchLastUsed(ctx context.Context, keyID uuid.UUID) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).
		Model(&APIKey{}).
		Where("id = ?", keyID).
		Update("last_used_at", now).Error
}
