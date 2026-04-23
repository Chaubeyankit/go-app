package user

import (
    "time"

    "github.com/google/uuid"
    "gorm.io/gorm"
)

// User is the read model — mirrors the auth.User table but owned by this module.
// In a microservice split, this module would have its own DB table. For now
// it reads from the shared users table via GORM.
type User struct {
    ID              uuid.UUID      `gorm:"type:uuid;primaryKey"`
    Email           string         `gorm:"uniqueIndex;not null"`
    Name            string         `gorm:"not null"`
    Role            string         `gorm:"type:varchar(20)"`
    IsActive        bool
    IsEmailVerified bool
    LastLoginAt     *time.Time
    CreatedAt       time.Time
    UpdatedAt       time.Time
    DeletedAt       gorm.DeletedAt `gorm:"index"`
    Profile         *Profile       `gorm:"foreignKey:UserID"`
}

func (User) TableName() string { return "users" }

type Profile struct {
    ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    UserID    uuid.UUID `gorm:"type:uuid;uniqueIndex;not null"`
    Bio       string
    AvatarURL string
    Location  string
    Website   string
    CreatedAt time.Time
    UpdatedAt time.Time
}

func (Profile) TableName() string { return "user_profiles" }