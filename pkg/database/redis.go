package database

import (
	"context"

	"github.com/redis/go-redis/v9"

	"github.com/ankit.chaubey/myapp/config"
	"github.com/ankit.chaubey/myapp/pkg/logger"
	"go.uber.org/zap"
)

func NewRedis(cfg config.RedisConfig) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		logger.Fatal("redis connection failed", zap.Error(err))
	}

	logger.Info("redis connected", zap.String("addr", cfg.Addr))
	return rdb
}
