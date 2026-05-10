package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OAuthRepository manages oauth_identities rows.
type OAuthRepository interface {
	FindIdentity(ctx context.Context, provider, providerID string) (*OAuthIdentity, error)
	CreateIdentity(ctx context.Context, identity *OAuthIdentity) error
	LinkIdentity(ctx context.Context, userID uuid.UUID, identity *OAuthIdentity) error
}

type oauthRepository struct{ db *gorm.DB }

func NewOAuthRepository(db *gorm.DB) OAuthRepository {
	return &oauthRepository{db: db}
}

func (r *oauthRepository) FindIdentity(ctx context.Context, provider, providerID string) (*OAuthIdentity, error) {
	var id OAuthIdentity
	result := r.db.WithContext(ctx).
		Where("provider = ? AND provider_id = ?", provider, providerID).
		First(&id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, gorm.ErrRecordNotFound
	}
	if result.Error != nil {
		return nil, fmt.Errorf("oauthRepo.FindIdentity: %w", result.Error)
	}
	return &id, nil
}

func (r *oauthRepository) CreateIdentity(ctx context.Context, identity *OAuthIdentity) error {
	if err := r.db.WithContext(ctx).Create(identity).Error; err != nil {
		return fmt.Errorf("oauthRepo.CreateIdentity: %w", err)
	}
	return nil
}

func (r *oauthRepository) LinkIdentity(ctx context.Context, userID uuid.UUID, identity *OAuthIdentity) error {
	identity.UserID = userID
	return r.CreateIdentity(ctx, identity)
}
