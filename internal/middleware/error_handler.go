package middleware

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/ankit.chaubey/myapp/pkg/apperrors"
	"github.com/ankit.chaubey/myapp/pkg/logger"
	"github.com/ankit.chaubey/myapp/pkg/response"
	"go.uber.org/zap"
)

func ErrorHandler(c *fiber.Ctx, err error) error {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		if appErr.StatusCode == 500 {
			logger.WithContext(c.UserContext()).Error("internal server error",
				zap.Error(appErr.Unwrap()),
				zap.String("path", c.Path()),
				zap.String("method", c.Method()),
			)
		}
		return c.Status(appErr.StatusCode).JSON(response.Err(appErr))
	}

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return c.Status(fiberErr.Code).JSON(response.Err(
			&apperrors.AppError{Code: "HTTP_ERROR", Message: fiberErr.Message},
		))
	}

	logger.WithContext(c.UserContext()).Error("unhandled error", zap.Error(err))
	return c.Status(500).JSON(response.Err(apperrors.InternalError(err)))
}
