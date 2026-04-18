package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrCacheMiss = errors.New("cache miss")

type Store struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

func (s *Store) Get(ctx context.Context, key string, dest interface{}) error {
	val, err := s.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return ErrCacheMiss
	}
	if err != nil {
		return fmt.Errorf("cache.Get %q: %w", key, err)
	}
	if err := json.Unmarshal(val, dest); err != nil {
		return fmt.Errorf("cache.Unmarshal %q: %w", key, err)
	}
	return nil
}

func (s *Store) Set(ctx context.Context, key string, val interface{}, ttl time.Duration) error {
	b, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("cache.Set marshal %q: %w", key, err)
	}
	if err := s.rdb.Set(ctx, key, b, ttl).Err(); err != nil {
		return fmt.Errorf("cache.Set %q: %w", key, err)
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, keys ...string) error {
	if err := s.rdb.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("cache.Delete: %w", err)
	}
	return nil
}

// SetNX sets key only if it doesn't exist — useful for distributed locks.
// func (s *Store) SetNX(ctx context.Context, key string, val interface{}, ttl time.Duration) (bool, error) {
// 	b, err := json.Marshal(val)
// 	if err != nil {
// 		return false, fmt.Errorf("cache.SetNX marshal: %w", err)
// 	}
// 	ok, err := s.rdb.SetNX(ctx, key, b, ttl).Result()
// 	return ok, err
// }
