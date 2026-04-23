package user

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/ankit.chaubey/myapp/pkg/apperrors"
	"github.com/ankit.chaubey/myapp/pkg/cache"
	"github.com/ankit.chaubey/myapp/pkg/logger"
	"github.com/ankit.chaubey/myapp/pkg/response"
	"go.uber.org/zap"
)

const (
	userCacheTTL = 15 * time.Minute
	userCacheKey = "user:" // user:{uuid}
	// userListKey  = "user_list:" // user_list:{page}:{per_page}:{search}:{role}
	// userListTTL  = 2 * time.Minute
)

type Service interface {
	GetByID(ctx context.Context, id string) (*UserResponse, error)
	UpdateProfile(ctx context.Context, id string, req *UpdateProfileRequest) (*UserResponse, error)
	ListUsers(ctx context.Context, req *ListUsersRequest) ([]*UserResponse, response.Meta, error)
	DeleteUser(ctx context.Context, id string) error
}

type service struct {
	repo  Repository
	cache *cache.Store
}

func NewService(repo Repository, cache *cache.Store) Service {
	return &service{repo: repo, cache: cache}
}

// GetByID implements cache-aside: check Redis → miss → query DB → populate cache.
func (s *service) GetByID(ctx context.Context, id string) (*UserResponse, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, apperrors.BadRequest("invalid user id format")
	}

	cacheKey := userCacheKey + id

	// L1: Try cache first
	var cached UserResponse
	if err := s.cache.Get(ctx, cacheKey, &cached); err == nil {
		logger.WithContext(ctx).Debug("user cache hit", zap.String("user_id", id))
		return &cached, nil
	}

	// L2: Cache miss — go to DB
	u, err := s.repo.FindByID(ctx, uid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NotFound("user")
		}
		return nil, apperrors.InternalError(fmt.Errorf("user.service.GetByID: %w", err))
	}

	resp := toResponse(u)

	// Populate cache asynchronously — don't block the response
	go func() {
		if err := s.cache.Set(context.Background(), cacheKey, resp, userCacheTTL); err != nil {
			logger.WithContext(ctx).Warn("failed to cache user",
				zap.String("user_id", id),
				zap.Error(err),
			)
		}
	}()

	return &resp, nil
}

func (s *service) UpdateProfile(ctx context.Context, id string, req *UpdateProfileRequest) (*UserResponse, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, apperrors.BadRequest("invalid user id format")
	}

	// Build the updates map — only include non-zero fields to avoid overwriting with blanks
	userUpdates := make(map[string]interface{})
	if req.Name != "" {
		userUpdates["name"] = req.Name
	}

	if len(userUpdates) > 0 {
		if _, err := s.repo.Update(ctx, uid, userUpdates); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, apperrors.NotFound("user")
			}
			return nil, apperrors.InternalError(fmt.Errorf("user.service.UpdateProfile update: %w", err))
		}
	}

	// Upsert the profile sub-table
	profile := &Profile{
		UserID:    uid,
		Bio:       req.Bio,
		AvatarURL: req.AvatarURL,
		Location:  req.Location,
		Website:   req.Website,
	}

	if err := s.repo.UpsertProfile(ctx, profile); err != nil {
		return nil, apperrors.InternalError(fmt.Errorf("user.service.UpsertProfile: %w", err))
	}

	// Invalidate cache — the next GetByID will repopulate from DB
	_ = s.cache.Delete(ctx, userCacheKey+id)

	logger.WithContext(ctx).Info("user profile updated", zap.String("user_id", id))

	return s.GetByID(ctx, id)
}

func (s *service) ListUsers(ctx context.Context, req *ListUsersRequest) ([]*UserResponse, response.Meta, error) {
	req.Normalize()

	users, total, err := s.repo.FindAll(ctx, req)
	if err != nil {
		return nil, response.Meta{}, apperrors.InternalError(
			fmt.Errorf("user.service.ListUsers: %w", err),
		)
	}

	totalPages := int(math.Ceil(float64(total) / float64(req.Limit)))
	meta := response.Meta{
		Page:       req.Page,
		Limit:      req.Limit,
		TotalItems: total,
		TotalPages: totalPages,
	}

	results := make([]*UserResponse, 0, len(users))
	for _, u := range users {
		r := toResponse(u)
		results = append(results, &r)
	}

	return results, meta, nil
}

func (s *service) DeleteUser(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return apperrors.BadRequest("invalid user id format")
	}

	if err := s.repo.SoftDelete(ctx, uid); err != nil {
		return apperrors.InternalError(fmt.Errorf("user.service.DeleteUser: %w", err))
	}

	_ = s.cache.Delete(ctx, userCacheKey+id)
	logger.WithContext(ctx).Info("user soft-deleted", zap.String("user_id", id))
	return nil
}
