package middleware

import (
	"strings"

	"github.com/ankit.chaubey/myapp/config"
	"github.com/ankit.chaubey/myapp/pkg/apperrors"
	"github.com/ankit.chaubey/myapp/pkg/jwt"
	"github.com/ankit.chaubey/myapp/pkg/logger"
	"github.com/ankit.chaubey/myapp/pkg/response"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func JWTAuth(cfg config.JWTConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(
				response.Err(apperrors.Unauthorized("missing authorization header")),
			)
		}

		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims, err := jwt.ParseAccess(cfg, tokenStr)

		if err != nil {
			logger.WithContext(c.UserContext()).Warn("invalid JWT",
				zap.String("error", err.Error()),
				zap.String("ip", c.IP()),
			)
			return c.Status(fiber.StatusUnauthorized).JSON(
				response.Err(apperrors.Unauthorized(err.Error())),
			)
		}

		// Inject claims into context and locals for downstream use
		c.Locals("user_id", claims.UserID)
		c.Locals("user_email", claims.Email)
		c.Locals("user_role", claims.Role)

		ctx := logger.InjectLogger(c.UserContext(), zap.String("user_id", claims.UserID))
		c.SetUserContext(ctx)

		return c.Next()
	}
}

// RequireRole returns 403 if the authenticated user doesn't have one of the given roles.
func RequireRole(roles ...string) fiber.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[strings.ToLower(r)] = struct{}{}
	}
	return func(c *fiber.Ctx) error {
		role, ok := c.Locals("user_role").(string)
		if !ok {
			// role missing or wrong type
			return c.Status(fiber.StatusUnauthorized).JSON(
				response.Err(apperrors.Unauthorized("invalid or missing role")),
			)
		}
		if _, ok := allowed[strings.ToLower(role)]; !ok {
			return c.Status(fiber.StatusForbidden).JSON(
				response.Err(apperrors.Forbidden("insufficient permissions")),
			)
		}

		return c.Next()
	}
}
