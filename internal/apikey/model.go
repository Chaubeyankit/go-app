package apikey

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type APIKey struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID     uuid.UUID      `gorm:"type:uuid;not null;index"`
	Name       string         `gorm:"not null"`
	KeyHash    string         `gorm:"not null;uniqueIndex"`
	KeyPrefix  string         `gorm:"not null"` // "sk_live_ab12" for display
	Scopes     pq.StringArray `gorm:"type:text[]"`
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	IsActive   bool `gorm:"default:true"`
	CreatedAt  time.Time
}

func (APIKey) TableName() string { return "api_keys" }

// Scope constants — extend as needed
const (
	ScopeReadUsers  = "read:users"
	ScopeWriteUsers = "write:users"
	ScopeReadData   = "read:data"
	ScopeWriteData  = "write:data"
	ScopeAdmin      = "admin"
)
