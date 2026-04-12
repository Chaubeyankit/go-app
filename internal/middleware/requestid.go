package middleware

import (
	"github.com/ankit.chaubey/myapp/pkg/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Get("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		c.Set("X-Request-ID", id)
		// Inject into context so all downstream logs carry this ID
		ctx := logger.InjectLogger(c.Context(), zap.String("request_id", id))
		c.SetUserContext(ctx)
		return c.Next()
	}
}
