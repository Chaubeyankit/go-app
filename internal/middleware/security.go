package middleware

import (
	"github.com/gofiber/fiber/v2"
)

// SecurityHeaders adds security headers to all responses
func SecurityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Content Security Policy - restrict resources that can be loaded
		// For development, we allow localhost; tighten for production
		c.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self'")

		// Prevent clickjacking attacks
		c.Set("X-Frame-Options", "DENY")

		// Prevent MIME type sniffing
		c.Set("X-Content-Type-Options", "nosniff")

		// Enable XSS filter (browser-side)
		c.Set("X-XSS-Protection", "1; mode=block")

		// Referrer Policy - control how much referrer info is sent
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// HSTS - enforce HTTPS (only in production)
		// Skip for development and HTTP
		if c.Protocol() == "https" {
			c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		// Permissions Policy - restrict browser features
		c.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(self)")

		return c.Next()
	}
}
