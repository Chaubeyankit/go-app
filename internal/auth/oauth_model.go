package auth

import (
	"github.com/google/uuid"
	"time"
)

type OAuthIdentity struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index"`
	Provider    string    `gorm:"not null"`
	ProviderID  string    `gorm:"not null"`
	Email       string    `gorm:"not null"`
	Name        string
	AvatarURL   string
	AccessToken string // encrypted at rest using AES-256-GCM
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (OAuthIdentity) TableName() string {
	return "oauth_identities"
}
