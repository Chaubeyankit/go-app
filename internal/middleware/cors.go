package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"strings"
)

func CORS(allowedOrigins []string) fiber.Handler {
	origins := strings.Join(allowedOrigins, ",")
	if origins == "" {
		origins = "*"
	}
	return cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Request-ID",
		AllowMethods:     "GET, POST, PUT, PATCH, DELETE, OPTIONS",
		AllowCredentials: true,
		MaxAge:           86400,
	})
}
