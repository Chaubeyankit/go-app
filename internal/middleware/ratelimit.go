package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/ankit.chaubey/myapp/pkg/apperrors"
	"github.com/ankit.chaubey/myapp/pkg/response"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

// RateLimiter implements a sliding window counter using Redis.
// limit: max requests, window: time window, keyFn: how to derive the key from a request.
func RateLimiter(rdb *redis.Client, limit int, window time.Duration, keyFn func(*fiber.Ctx) string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := "rl:" + keyFn(c)
		ctx := context.Background()

		pipe := rdb.Pipeline()
		incr := pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, window)
		if _, err := pipe.Exec(ctx); err != nil {
			// Fail open — don't block requests if Redis is down
			return c.Next()
		}

		count := incr.Val()
		remaining := int64(limit) - count
		if remaining < 0 {
			remaining = 0
		}

		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(window).Unix()))

		if count > int64(limit) {
			return c.Status(fiber.StatusTooManyRequests).JSON(
				response.Err(apperrors.TooManyRequests()),
			)
		}

		return c.Next()
	}
}

// ByIP is the standard key function for IP-based rate limiting.
func ByIP(c *fiber.Ctx) string { return c.IP() }

// ByUser keys on the authenticated user ID, for per-user limits on protected routes.
func ByUser(c *fiber.Ctx) string {
	if uid, ok := c.Locals("user_id").(string); ok && uid != "" {
		return "user:" + uid
	}
	return c.IP()
}
