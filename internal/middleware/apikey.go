package middleware

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/ankit.chaubey/myapp/internal/apikey"
	"github.com/ankit.chaubey/myapp/pkg/apperrors"
	"github.com/ankit.chaubey/myapp/pkg/logger"
	"github.com/ankit.chaubey/myapp/pkg/response"
)

// APIKeyAuth authenticates requests using the X-API-Key header.
// On success it injects user_id, key_id, and key_scopes into Fiber locals,
// using the same keys as JWTAuth so downstream handlers are unaware
// of which auth method was used.
func APIKeyAuth(svc apikey.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		raw := c.Get("X-API-Key")
		if raw == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(
				response.Err(apperrors.Unauthorized("missing X-API-Key header")),
			)
		}

		info, err := svc.Validate(c.UserContext(), raw)
		if err != nil {
			logger.WithContext(c.UserContext()).Warn("invalid API key",
				zap.String("ip", c.IP()),
			)
			return c.Status(fiber.StatusUnauthorized).JSON(
				response.Err(apperrors.Unauthorized("invalid API key")),
			)
		}

		c.Locals("user_id", info.UserID.String())
		c.Locals("key_id", info.KeyID.String())
		c.Locals("key_scopes", info.Scopes)
		c.Locals("auth_method", "api_key")

		ctx := logger.InjectLogger(c.UserContext(),
			zap.String("user_id", info.UserID.String()),
			zap.String("key_id", info.KeyID.String()),
		)
		c.SetUserContext(ctx)

		return c.Next()
	}
}

// RequireScope enforces that the authenticated API key has the given scope.
// Use after APIKeyAuth middleware.
func RequireScope(scope string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		scopes, _ := c.Locals("key_scopes").([]string)
		for _, s := range scopes {
			if s == scope || s == apikey.ScopeAdmin {
				return c.Next()
			}
		}
		return c.Status(fiber.StatusForbidden).JSON(
			response.Err(apperrors.Forbidden(
				fmt.Sprintf("API key missing required scope: %s", scope),
			)),
		)
	}
}

// AnyAuth accepts either a valid JWT or a valid API key.
// Use on routes that should be accessible to both humans and machines.
func AnyAuth(jwtMiddleware, apiKeyMiddleware fiber.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Prefer JWT if Authorization header is present
		if strings.HasPrefix(c.Get("Authorization"), "Bearer ") {
			return jwtMiddleware(c)
		}
		// Fall back to API key
		if c.Get("X-API-Key") != "" {
			return apiKeyMiddleware(c)
		}
		return c.Status(fiber.StatusUnauthorized).JSON(
			response.Err(apperrors.Unauthorized("authentication required")),
		)
	}
}
