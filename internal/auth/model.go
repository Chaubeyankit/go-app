package auth

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"

	"github.com/ankit.chaubey/myapp/internal/user"
)

type UserRole string

const (
	RoleUser  UserRole = "user"
	RoleAdmin UserRole = "admin"
)

type User struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Email           string    `gorm:"uniqueIndex;not null"`
	PasswordHash    string    `gorm:"not null"`
	Name            string    `gorm:"not null"`
	Role            UserRole  `gorm:"type:varchar(20);default:'user'"`
	IsActive        bool      `gorm:"default:true"`
	IsEmailVerified bool      `gorm:"default:false"`
	LastLoginAt     *time.Time
	// MFA fields — added in migration 000004
	MFAEnabled   bool       `gorm:"column:mfa_enabled;default:false"`
	MFASecretEnc string     `gorm:"column:mfa_secret_enc"` // TOTP secret (encrypted at rest)
	MFAEnabledAt *time.Time `gorm:"column:mfa_enabled_at"`
	// Account lockout fields — added in migration 000006
	FailedLoginAttempts  int        `gorm:"column:failed_login_attempts;default:0"`
	LockedUntil          *time.Time `gorm:"column:locked_until;index"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"` // soft delete

	Profile *user.Profile `gorm:"foreignKey:UserID"`
}

type AuditLog struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID    uuid.UUID `gorm:"type:uuid;index"`
	Action    string    `gorm:"not null"`
	IPAddress string
	UserAgent string
	Metadata  []byte `gorm:"type:jsonb"`
	CreatedAt time.Time
}

// Add to existing model.go

type PasswordReset struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	TokenHash string    `gorm:"not null;uniqueIndex"`
	ExpiresAt time.Time `gorm:"not null"`
	UsedAt    *time.Time
	CreatedAt time.Time
}
