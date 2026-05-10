package apikey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/ankit.chaubey/myapp/pkg/apperrors"
	"github.com/ankit.chaubey/myapp/pkg/cache"
	"github.com/ankit.chaubey/myapp/pkg/logger"
	"go.uber.org/zap"
)

const (
	keyPrefix         = "sk_live_"
	apiKeyCacheTTL    = 5 * time.Minute
	apiKeyCachePrefix = "apikey:"
)

type CreateRequest struct {
	Name      string     `json:"name"       validate:"required,min=1,max=100"`
	Scopes    []string   `json:"scopes"     validate:"required,min=1"`
	ExpiresAt *time.Time `json:"expiresAt"` // optional
}

type CreateResponse struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	RawKey    string     `json:"key"`        // returned ONCE — never stored
	KeyPrefix string     `json:"keyPrefix"` // for display in UI
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

type KeyInfo struct {
	KeyID  uuid.UUID
	UserID uuid.UUID
	Scopes []string
}

type Service interface {
	Create(ctx context.Context, userID string, req *CreateRequest) (*CreateResponse, error)
	Validate(ctx context.Context, rawKey string) (*KeyInfo, error)
	List(ctx context.Context, userID string) ([]*APIKey, error)
	Revoke(ctx context.Context, keyID, userID string) error
}

type service struct {
	repo  Repository
	cache *cache.Store
}

func NewService(repo Repository, cache *cache.Store) Service {
	return &service{repo: repo, cache: cache}
}

func (s *service) Create(ctx context.Context, userID string, req *CreateRequest) (*CreateResponse, error) {

	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, apperrors.BadRequest("invalid user id")
	}

	// Generate a cryptographically secure random key
	rawKey, hash, prefix, err := generateAPIKey()
	if err != nil {
		return nil, apperrors.InternalError(fmt.Errorf("apikey.Create generate: %w", err))
	}

	key := &APIKey{
		UserID:    uid,
		Name:      req.Name,
		KeyHash:   hash,
		KeyPrefix: prefix,
		Scopes:    req.Scopes,
		ExpiresAt: req.ExpiresAt,
	}

	if err := s.repo.Create(ctx, key); err != nil {
		return nil, apperrors.InternalError(fmt.Errorf("apikey.Create: %w", err))
	}

	logger.WithContext(ctx).Info("API key created",
		zap.String("user_id", userID),
		zap.String("key_id", key.ID.String()),
		zap.Strings("scopes", req.Scopes),
	)

	return &CreateResponse{
		ID:        key.ID.String(),
		Name:      key.Name,
		RawKey:    rawKey, // shown ONCE — user must copy it now
		KeyPrefix: prefix,
		Scopes:    req.Scopes,
		ExpiresAt: req.ExpiresAt,
		CreatedAt: key.CreatedAt,
	}, nil
}

// Validate authenticates a raw API key. It uses a Redis cache so the DB
// is not hit on every request. Cache TTL is 5 minutes.
func (s *service) Validate(ctx context.Context, rawKey string) (*KeyInfo, error) {
	hash := hashKey(rawKey)
	cacheKey := apiKeyCachePrefix + hash

	// L1: check cache
	var info KeyInfo
	if err := s.cache.Get(ctx, cacheKey, &info); err == nil {
		// Async touch — don't slow down the request
		go func() { _ = s.repo.TouchLastUsed(context.Background(), info.KeyID) }()
		return &info, nil
	}

	// L2: check DB
	key, err := s.repo.FindByHash(ctx, hash)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperrors.Unauthorized("invalid API key")
		}
		return nil, apperrors.InternalError(fmt.Errorf("apikey.Validate: %w", err))
	}

	info = KeyInfo{
		KeyID:  key.ID,
		UserID: key.UserID,
		Scopes: key.Scopes,
	}

	// Populate cache
	go func() {
		_ = s.cache.Set(context.Background(), cacheKey, info, apiKeyCacheTTL)
		_ = s.repo.TouchLastUsed(context.Background(), key.ID)
	}()

	return &info, nil
}

func (s *service) List(ctx context.Context, userID string) ([]*APIKey, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, apperrors.BadRequest("invalid user id")
	}
	keys, err := s.repo.ListByUser(ctx, uid)
	if err != nil {
		return nil, apperrors.InternalError(fmt.Errorf("apikey.List: %w", err))
	}
	return keys, nil
}

func (s *service) Revoke(ctx context.Context, keyID, userID string) error {
	kid, err := uuid.Parse(keyID)
	if err != nil {
		return apperrors.BadRequest("invalid key id")
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return apperrors.BadRequest("invalid user id")
	}
	if err := s.repo.Revoke(ctx, kid, uid); err != nil {
		if err == gorm.ErrRecordNotFound {
			return apperrors.NotFound("api key")
		}
		return apperrors.InternalError(err)
	}
	return nil
}

// generateAPIKey returns: rawKey, sha256Hash, displayPrefix, error
func generateAPIKey() (string, string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", "", err
	}
	raw := keyPrefix + hex.EncodeToString(b) // "sk_live_<64 hex chars>"
	hash := hashKey(raw)
	prefix := raw[:len(keyPrefix)+8] // "sk_live_ab12cd34" — safe to display
	return raw, hash, prefix, nil
}

func hashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
