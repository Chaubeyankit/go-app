package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	// User queries
	Create(ctx context.Context, user *User) error
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error

	// Account lockout
	IncrementFailedLoginAttempts(ctx context.Context, id uuid.UUID) error
	ResetFailedLoginAttempts(ctx context.Context, id uuid.UUID) error
	LockAccount(ctx context.Context, id uuid.UUID, duration int) error

	// Audit
	CreateAuditLog(ctx context.Context, log *AuditLog) error

	// Password reset
	CreatePasswordReset(ctx context.Context, pr *PasswordReset) error
	FindPasswordReset(ctx context.Context, tokenHash string) (*PasswordReset, error)
	MarkPasswordResetUsed(ctx context.Context, id uuid.UUID) error
	ConsumePasswordReset(ctx context.Context, tokenHash string, userID uuid.UUID, newHash string) error
	UpdatePassword(ctx context.Context, userID uuid.UUID, newHash string) error
	InvalidateOldResets(ctx context.Context, userID uuid.UUID) error

	// MFA
	StoreMFASecret(ctx context.Context, userID uuid.UUID, secret string) error
	EnableMFA(ctx context.Context, userID uuid.UUID) error
	DisableMFA(ctx context.Context, userID uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreatePasswordReset(ctx context.Context, pr *PasswordReset) error {
	return r.db.WithContext(ctx).Create(pr).Error
}

func (r *repository) FindPasswordReset(ctx context.Context, tokenHash string) (*PasswordReset, error) {
	var pr PasswordReset
	result := r.db.WithContext(ctx).
		Where("token_hash = ? AND used_at IS NULL AND expires_at > NOW()", tokenHash).
		First(&pr)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, gorm.ErrRecordNotFound
	}
	return &pr, result.Error
}

func (r *repository) MarkPasswordResetUsed(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&PasswordReset{}).
		Where("id = ?", id).
		Update("used_at", gorm.Expr("NOW()")).Error
}

func (r *repository) UpdatePassword(ctx context.Context, userID uuid.UUID, newHash string) error {
	return r.db.WithContext(ctx).
		Model(&User{}).
		Where("id = ?", userID).
		Update("password_hash", newHash).Error
}

// ConsumePasswordReset atomically checks if the token is valid, marks it as used, and updates the password
// This ensures the token can only be used once, preventing race conditions
func (r *repository) ConsumePasswordReset(ctx context.Context, tokenHash string, userID uuid.UUID, newHash string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Find the password reset token
		var pr PasswordReset
		result := tx.Where("token_hash = ? AND used_at IS NULL AND expires_at > NOW()", tokenHash).First(&pr)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return gorm.ErrRecordNotFound
			}
			return result.Error
		}

		// Verify the user ID matches
		if pr.UserID != userID {
			return gorm.ErrRecordNotFound
		}

		// Mark token as used
		if err := tx.Model(&PasswordReset{}).Where("id = ?", pr.ID).Update("used_at", gorm.Expr("NOW()")).Error; err != nil {
			return err
		}

		// Update password
		if err := tx.Model(&User{}).Where("id = ?", userID).Update("password_hash", newHash).Error; err != nil {
			return err
		}

		return nil
	})
}

func (r *repository) InvalidateOldResets(ctx context.Context, userID uuid.UUID) error {
	// Mark all existing unused resets for this user as used before issuing a new one
	return r.db.WithContext(ctx).
		Model(&PasswordReset{}).
		Where("user_id = ? AND used_at IS NULL", userID).
		Update("used_at", gorm.Expr("NOW()")).Error
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

// IncrementFailedLoginAttempts increments the failed login counter and locks the account if threshold reached
func (r *repository) IncrementFailedLoginAttempts(ctx context.Context, id uuid.UUID) error {
	// Use a transaction to ensure atomicity
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.First(&user, "id = ?", id).Error; err != nil {
			return err
		}

		// Increment failed attempts
		newAttempts := user.FailedLoginAttempts + 1

		updates := map[string]interface{}{
			"failed_login_attempts": newAttempts,
		}

		// Lock account if threshold reached (default 5 attempts)
		// Lock for 15 minutes
		if newAttempts >= 5 {
			updates["locked_until"] = gorm.Expr("NOW() + INTERVAL '15 minutes'")
		}

		return tx.Model(&User{}).Where("id = ?", id).Updates(updates).Error
	})
}

// ResetFailedLoginAttempts resets the failed login counter and unlocks the account
func (r *repository) ResetFailedLoginAttempts(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"failed_login_attempts": 0,
			"locked_until":          nil,
		})
	return result.Error
}

// LockAccount manually locks an account for a specified duration in minutes
func (r *repository) LockAccount(ctx context.Context, id uuid.UUID, duration int) error {
	result := r.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).
		Update("locked_until", gorm.Expr("NOW() + INTERVAL '? minutes'", duration))
	return result.Error
}

func (r *repository) CreateAuditLog(ctx context.Context, log *AuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *repository) StoreMFASecret(ctx context.Context, userID uuid.UUID, secret string) error {
	result := r.db.WithContext(ctx).
		Model(&User{}).
		Where("id = ?", userID).
		Update("mfa_secret_enc", secret)
	if result.Error != nil {
		return fmt.Errorf("repo.StoreMFASecret: %w", result.Error)
	}
	return nil
}

func (r *repository) EnableMFA(ctx context.Context, userID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"mfa_enabled":    true,
			"mfa_enabled_at": gorm.Expr("NOW()"),
		})
	if result.Error != nil {
		return fmt.Errorf("repo.EnableMFA: %w", result.Error)
	}
	return nil
}

func (r *repository) DisableMFA(ctx context.Context, userID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"mfa_enabled":    false,
			"mfa_secret_enc": "",
			"mfa_enabled_at": nil,
		})
	if result.Error != nil {
		return fmt.Errorf("repo.DisableMFA: %w", result.Error)
	}
	return nil
}
