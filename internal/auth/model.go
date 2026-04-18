package auth

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
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
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       gorm.DeletedAt `gorm:"index"` // soft delete
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
