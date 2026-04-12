package middleware

import (
	"fmt"
	"runtime/debug"

	"github.com/ankit.chaubey/myapp/pkg/apperrors"
	"github.com/ankit.chaubey/myapp/pkg/logger"
	"github.com/ankit.chaubey/myapp/pkg/response"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func Recovery() fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.WithContext(c.UserContext()).Error("panic recovered",
					zap.Any("panic", r),
					zap.ByteString("stack", debug.Stack()),
				)
				err = c.Status(fiber.StatusInternalServerError).JSON(
					response.Err(apperrors.InternalError(fmt.Errorf("%v", r))),
				)
			}
		}()
		return c.Next()
	}
}
