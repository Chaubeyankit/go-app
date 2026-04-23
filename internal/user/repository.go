package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)
	FindAll(ctx context.Context, req *ListUsersRequest) ([]*User, int64, error)
	Update(ctx context.Context, id uuid.UUID, updates map[string]interface{}) (*User, error)
	UpsertProfile(ctx context.Context, profile *Profile) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var u User
	result := r.db.WithContext(ctx).
		Preload("Profile").
		First(&u, "id = ? AND is_active = true", id)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, gorm.ErrRecordNotFound
	}
	if result.Error != nil {
		return nil, fmt.Errorf("user.repo.FindByID: %w", result.Error)
	}
	return &u, nil
}

func (r *repository) FindAll(ctx context.Context, req *ListUsersRequest) ([]*User, int64, error) {
	var users []*User
	var total int64

	q := r.db.WithContext(ctx).Model(&User{}).Where("deleted_at IS NULL AND is_active = true")

	if req.Search != "" {
		like := "%" + req.Search + "%"
		q = q.Where("name ILIKE ? OR email ILIKE ?", like, like)
	}
	if req.Role != "" {
		q = q.Where("role = ?", req.Role)
	}

	// Count total before applying pagination
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("user.repo.FindAll count: %w", err)
	}

	offset := (req.Page - 1) * req.Limit
	result := q.
		Preload("Profile").
		Order("created_at DESC").
		Limit(req.Limit).
		Offset(offset).
		Find(&users)

	if result.Error != nil {
		return nil, 0, fmt.Errorf("user.repo.FindAll: %w", result.Error)
	}
	return users, total, nil
}

func (r *repository) Update(ctx context.Context, id uuid.UUID, updates map[string]interface{}) (*User, error) {
	result := r.db.WithContext(ctx).
		Model(&User{}).
		Where("id = ?", id).
		Updates(updates)

	if result.Error != nil {
		return nil, fmt.Errorf("user.repo.Update: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return r.FindByID(ctx, id)
}

// UpsertProfile creates a profile row or updates it if one already exists.
// Uses Postgres ON CONFLICT DO UPDATE via GORM's clause.OnConflict.
func (r *repository) UpsertProfile(ctx context.Context, profile *Profile) error {
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"bio", "avatar_url", "location", "website", "updated_at"}),
		}).
		Create(profile)

	if result.Error != nil {
		return fmt.Errorf("user.repo.UpsertProfile: %w", result.Error)
	}
	return nil
}

func (r *repository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	// GORM soft-delete: sets deleted_at and is_active=false
	result := r.db.WithContext(ctx).
		Model(&User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_active":  false,
			"deleted_at": gorm.Expr("NOW()"),
		})
	if result.Error != nil {
		return fmt.Errorf("user.repo.SoftDelete: %w", result.Error)
	}
	return nil
}
