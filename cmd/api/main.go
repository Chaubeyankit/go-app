package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"github.com/ankit.chaubey/myapp/config"
	"github.com/ankit.chaubey/myapp/internal/auth"
	"github.com/ankit.chaubey/myapp/internal/middleware"
	"github.com/ankit.chaubey/myapp/pkg/database"
	"github.com/ankit.chaubey/myapp/pkg/logger"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found or error loading: %v", err)
	}

	cfg := config.Load()
	logger.Init(cfg.App.Env)

	db, err := database.NewPostgres(cfg.Database)
	if err != nil {
		logger.Fatal("failed to connect to postgres", zap.Error(err))
	}

	// Redis Client
	rdb := database.NewRedis(cfg.Redis)

	//Server
	app := fiber.New(fiber.Config{
		ErrorHandler:          middleware.ErrorHandler,
		ReadTimeout:           10 * time.Second,
		WriteTimeout:          10 * time.Second,
		IdleTimeout:           120 * time.Second,
		DisableStartupMessage: true,
		AppName:               cfg.App.Name,
	})

	// Global middleware — order matters
	app.Use(middleware.RequestID())
	app.Use(middleware.RequestLogger())
	app.Use(middleware.Recovery())
	app.Use(middleware.CORS(cfg.App.AllowedOrigins))

	// Health check (no auth, no rate limit)
	app.Get("/health", func(c *fiber.Ctx) error {
		sqlDB, _ := db.DB()
		dbStatus := "ok"
		if err := sqlDB.Ping(); err != nil {
			dbStatus = "degraded"
		}
		redisStatus := "ok"
		if err := rdb.Ping(c.Context()).Err(); err != nil {
			redisStatus = "degraded"
		}
		return c.JSON(fiber.Map{
			"status": "ok",
			"env":    cfg.App.Env,
			"dependencies": fiber.Map{
				"postgres": dbStatus,
				"redis":    redisStatus,
			},
		})
	})

	// Auth module
	authRateLimiter := middleware.RateLimiter(rdb, 5, time.Minute, middleware.ByIP)
	authRepo := auth.NewRepository(db)
	authTokenStore := auth.NewTokenStore(rdb)
	authService := auth.NewService(authRepo, authTokenStore, cfg.JWT)
	authHandler := auth.NewHandler(authService)
	authHandler.RegisterRoutes(app, authRateLimiter)

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("server starting",
			zap.String("addr", cfg.App.Addr),
			zap.String("env", cfg.App.Env),
		)
		if err := app.Listen(cfg.App.Addr); err != nil {
			logger.Fatal("server listen error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}

	logger.Info("server stopped cleanly")
}
