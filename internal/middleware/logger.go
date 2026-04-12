package middleware

import (
	"time"

	"github.com/ankit.chaubey/myapp/pkg/logger"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func RequestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		status := c.Response().StatusCode()
		lvl := zap.InfoLevel
		if status >= 500 {
			lvl = zap.ErrorLevel
		} else if status >= 400 {
			lvl = zap.WarnLevel
		}

		logger.WithContext(c.UserContext()).Log(lvl, "request",
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.Int("status", status),
			zap.Duration("latency", time.Since(start)),
			zap.String("ip", c.IP()),
			zap.Int("bytes_out", len(c.Response().Body())),
		)

		return err
	}
}
