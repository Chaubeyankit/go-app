package logger

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log *zap.Logger

type contextKey string

const loggerKey contextKey = "logger"

func Init(env string) {
	var cfg zap.Config

	if env == "production" {
		cfg = zap.NewProductionConfig()
		cfg.EncoderConfig.TimeKey = "timestamp"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	var err error
	log, err = cfg.Build(zap.AddCallerSkip(1))
	if err != nil {
		panic("failed to init logger: " + err.Error())
	}
}

func WithContext(ctx context.Context) *zap.Logger {
	if l, ok := ctx.Value(loggerKey).(*zap.Logger); ok {
		return l
	}
	return log
}

// InjectLogger returns a new context with a child logger containing the given fields.
// Use this at the start of request handling to attach request_id, user_id, etc.
func InjectLogger(ctx context.Context, fields ...zap.Field) context.Context {
	child := log.With(fields...)
	return context.WithValue(ctx, loggerKey, child)
}

func Info(msg string, fields ...zap.Field)  { log.Info(msg, fields...) }
func Error(msg string, fields ...zap.Field) { log.Error(msg, fields...) }
func Warn(msg string, fields ...zap.Field)  { log.Warn(msg, fields...) }
func Debug(msg string, fields ...zap.Field) { log.Debug(msg, fields...) }
func Fatal(msg string, fields ...zap.Field) { log.Fatal(msg, fields...) }

func Sync() { _ = log.Sync() }
