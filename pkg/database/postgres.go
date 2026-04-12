package database

import (
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/ankit.chaubey/myapp/config"
	"github.com/ankit.chaubey/myapp/pkg/logger"
)

func NewPostgres(cfg config.DatabaseConfig) (*gorm.DB, error) {
	gormCfg := &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent), // we handle logging ourselves
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		PrepareStmt: true, // cache prepared statements — big perf win
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN), gormCfg)
	if err != nil {
		return nil, fmt.Errorf("gorm.Open: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("db.DB(): %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.MaxLifetime)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}

	logger.Info("postgres connected",
		zap.Int("max_open_conns", cfg.MaxOpenConns),
		zap.Int("max_idle_conns", cfg.MaxIdleConns),
	)

	return db, nil
}
