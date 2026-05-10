package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenStore manages refresh token storage and revocation in Redis.
type TokenStore interface {
	Save(ctx context.Context, tokenID, userID string, ttl time.Duration) error
	Exists(ctx context.Context, tokenID string) (bool, error)
	Revoke(ctx context.Context, tokenID string) error
	RevokeAll(ctx context.Context, userID string) error

	// Raw key/value operations — used by MFA challenge tokens
	SetRaw(ctx context.Context, key, value string, ttl time.Duration) error
	GetRaw(ctx context.Context, key string) (string, error)
	DeleteRaw(ctx context.Context, key string) error
}

type redisTokenStore struct {
	rdb *redis.Client
}

func NewTokenStore(rdb *redis.Client) TokenStore {
	return &redisTokenStore{rdb: rdb}
}

func tokenKey(tokenID string) string     { return "refresh:" + tokenID }
func userTokensKey(userID string) string { return "user_tokens:" + userID }

func (s *redisTokenStore) Save(ctx context.Context, tokenID, userID string, ttl time.Duration) error {
	pipe := s.rdb.Pipeline()
	// Store tokenID → userID for validation
	pipe.Set(ctx, tokenKey(tokenID), userID, ttl)
	// Track all token IDs per user for bulk revocation (logout all devices)
	pipe.SAdd(ctx, userTokensKey(userID), tokenID)
	pipe.Expire(ctx, userTokensKey(userID), ttl+time.Hour)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("tokenStore.Save: %w", err)
	}
	return nil
}

func (s *redisTokenStore) Exists(ctx context.Context, tokenID string) (bool, error) {
	n, err := s.rdb.Exists(ctx, tokenKey(tokenID)).Result()
	return n > 0, err
}

func (s *redisTokenStore) Revoke(ctx context.Context, tokenID string) error {
	return s.rdb.Del(ctx, tokenKey(tokenID)).Err()
}

func (s *redisTokenStore) RevokeAll(ctx context.Context, userID string) error {
	tokenIDs, err := s.rdb.SMembers(ctx, userTokensKey(userID)).Result()
	if err != nil {
		return err
	}
	if len(tokenIDs) == 0 {
		return nil
	}
	keys := make([]string, 0, len(tokenIDs)+1)
	for _, id := range tokenIDs {
		keys = append(keys, tokenKey(id))
	}
	keys = append(keys, userTokensKey(userID))
	return s.rdb.Del(ctx, keys...).Err()
}

func (s *redisTokenStore) SetRaw(ctx context.Context, key, value string, ttl time.Duration) error {
	if err := s.rdb.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("tokenStore.SetRaw %q: %w", key, err)
	}
	return nil
}

func (s *redisTokenStore) GetRaw(ctx context.Context, key string) (string, error) {
	val, err := s.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil // not found — caller checks for empty string
	}
	if err != nil {
		return "", fmt.Errorf("tokenStore.GetRaw %q: %w", key, err)
	}
	return val, nil
}

func (s *redisTokenStore) DeleteRaw(ctx context.Context, key string) error {
	return s.rdb.Del(ctx, key).Err()
}
