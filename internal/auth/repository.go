package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, user *User) error
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error
	CreateAuditLog(ctx context.Context, log *AuditLog) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, user *User) error {
	result := r.db.WithContext(ctx).Create(user)
	if result.Error != nil {
		return fmt.Errorf("auth.repo.Create: %w", result.Error)
	}
	return nil
}

func (r *repository) FindByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	result := r.db.WithContext(ctx).Where("email = ? AND is_active = true", email).First(&user)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, gorm.ErrRecordNotFound
	}
	if result.Error != nil {
		return nil, fmt.Errorf("auth.repo.FindByEmail: %w", result.Error)
	}
	return &user, nil
}

func (r *repository) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var user User
	result := r.db.WithContext(ctx).First(&user, "id = ?", id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, gorm.ErrRecordNotFound
	}
	if result.Error != nil {
		return nil, fmt.Errorf("auth.repo.FindByID: %w", result.Error)
	}
	return &user, nil
}

func (r *repository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&User{}).Where("email = ?", email).Count(&count)
	if result.Error != nil {
		return false, fmt.Errorf("auth.repo.ExistsByEmail: %w", result.Error)
	}
	return count > 0, nil
}

func (r *repository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).
		Update("last_login_at", gorm.Expr("NOW()"))
	return result.Error
}

func (r *repository) CreateAuditLog(ctx context.Context, log *AuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}
