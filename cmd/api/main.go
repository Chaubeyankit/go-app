package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"github.com/ankit.chaubey/myapp/config"
	"github.com/ankit.chaubey/myapp/internal/auth"
	"github.com/ankit.chaubey/myapp/internal/middleware"
	"github.com/ankit.chaubey/myapp/internal/user"

	"github.com/ankit.chaubey/myapp/pkg/cache"
	"github.com/ankit.chaubey/myapp/pkg/database"
	"github.com/ankit.chaubey/myapp/pkg/logger"

	"github.com/ankit.chaubey/myapp/internal/notification"
	"github.com/ankit.chaubey/myapp/pkg/email"
	"github.com/ankit.chaubey/myapp/pkg/location"
	"github.com/ankit.chaubey/myapp/pkg/queue"

	"github.com/ankit.chaubey/myapp/internal/apikey"
	"github.com/ankit.chaubey/myapp/internal/auth/oauth"
)

func main() {
	// --- Production-ready setup ---
	ctx := context.Background()

	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found or error loading: %v", err)
	}

	cfg := config.Load()

	// Initialize logger with production settings
	logger.Init(cfg.App.Env)
	defer logger.Sync()

	// Validate critical security configuration
	if cfg.Security.EncryptionKey == "" {
		logger.Fatal("ENCRYPTION_KEY environment variable must be set (base64-encoded 32 bytes for AES-256-GCM)")
	}
	if cfg.JWT.AccessSecret == "" || cfg.JWT.RefreshSecret == "" {
		logger.Fatal("JWT_ACCESS_SECRET and JWT_REFRESH_SECRET environment variables must be set")
	}

	// Initialize database with connection pooling
	db, err := database.NewPostgres(cfg.Database)
	if err != nil {
		logger.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()

	// Redis Client with health check
	rdb := database.NewRedis(cfg.Redis)
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatal("failed to connect to redis", zap.Error(err))
	}

	// --- Queue infrastructure ---
	producer := queue.NewProducer(rdb)

	// --- Auth module (now takes producer) ---
	authRepo := auth.NewRepository(db)
	authTokenStore := auth.NewTokenStore(rdb)

	// Decode base64-encoded encryption key (must be 32 bytes when decoded)
	encryptionKey, err := base64.StdEncoding.DecodeString(cfg.Security.EncryptionKey)
	if err != nil {
		logger.Fatal("ENCRYPTION_KEY must be valid base64", zap.Error(err))
	}
	if len(encryptionKey) != 32 {
		logger.Fatal("ENCRYPTION_KEY must decode to 32 bytes for AES-256",
			zap.Int("got", len(encryptionKey)),
			zap.Int("want", 32))
	}

	// MFA module (must be created before auth service)
	mfaService := auth.NewMFAService(authRepo, authTokenStore, cfg.JWT, cfg.App.Name, encryptionKey)
	mfaHandler := auth.NewMFAHandler(mfaService, rdb)

	authService := auth.NewService(authRepo, authTokenStore, producer, cfg.JWT, mfaService, cfg.Security)
	authHandler := auth.NewHandler(authService, cfg.JWT)

	// --- User module ---
	userRepo := user.NewRepository(db)
	cacheStore := cache.New(rdb)
	userService := user.NewService(userRepo, cacheStore)
	userHandler := user.NewHandler(userService)

	apiKeyRepo := apikey.NewRepository(db)
	apiKeyService := apikey.NewService(apiKeyRepo, cacheStore)
	apiKeyHandler := apikey.NewHandler(apiKeyService)

	// OAuth module
	oauthProviders := oauth.Providers(cfg.OAuth)
	oauthRepo := auth.NewOAuthRepository(db)
	oauthService := auth.NewOAuthService(oauthProviders, authRepo, oauthRepo, authTokenStore, cfg.JWT, encryptionKey)
	oauthHandler := auth.NewOAuthHandler(oauthService)

	// --- Fiber app with production optimizations ---
	app := fiber.New(fiber.Config{
		ErrorHandler:          middleware.ErrorHandler,
		ReadTimeout:           30 * time.Second,
		WriteTimeout:          30 * time.Second,
		IdleTimeout:           60 * time.Second,
		DisableStartupMessage: true,
		CaseSensitive:         false,
		StrictRouting:         false,
		BodyLimit:             10 * 1024 * 1024,
	})

	// Global middleware — order matters
	app.Use(middleware.RequestID())
	app.Use(middleware.SecurityHeaders())
	app.Use(middleware.RequestLogger())
	app.Use(middleware.Recovery())
	app.Use(middleware.CORS(cfg.App.AllowedOrigins))

	// Enhanced health check with metrics
	app.Get("/health", func(c *fiber.Ctx) error {
		healthCheck := fiber.Map{
			"status":    "ok",
			"timestamp": time.Now().UTC(),
			"version":   "1.0.0",
			"env":       cfg.App.Env,
		}

		// Database health check
		sqlDB, _ := db.DB()
		if err := sqlDB.Ping(); err != nil {
			healthCheck["dependencies"] = fiber.Map{"postgres": "degraded"}
			healthCheck["status"] = "degraded"
		} else {
			healthCheck["dependencies"] = fiber.Map{"postgres": "ok"}
		}

		// Redis health check
		if err := rdb.Ping(ctx).Err(); err != nil {
			if deps, ok := healthCheck["dependencies"].(fiber.Map); ok {
				deps["redis"] = "degraded"
			}
			healthCheck["status"] = "degraded"
		} else {
			if deps, ok := healthCheck["dependencies"].(fiber.Map); ok {
				deps["redis"] = "ok"
			}
		}

		return c.JSON(healthCheck)
	})

	// Ready endpoint for Kubernetes probes
	app.Get("/ready", func(c *fiber.Ctx) error {
		sqlDB, _ := db.DB()
		if err := sqlDB.Ping(); err != nil {
			return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{
				"status": "not ready",
				"error":  err.Error(),
			})
		}
		return c.JSON(fiber.Map{"status": "ready"})
	})

	// Auth endpoint rate limiting (configurable via env)
	authRL := middleware.RateLimiter(rdb, cfg.Security.AuthRateLimitRequests, cfg.Security.AuthRateLimitWindow, middleware.ByIP)
	jwtAuth := middleware.JWTAuth(cfg.JWT)

	// Register routes
	authHandler.RegisterRoutes(app, authRL)
	userHandler.RegisterRoutes(app, cfg.JWT)
	oauthHandler.RegisterRoutes(app)
	mfaHandler.RegisterRoutes(app, cfg.JWT, cfg.Security)
	apiKeyHandler.RegisterRoutes(app, jwtAuth)

	// --- Email sender with production configuration ---
	var emailSender email.Sender
	if cfg.App.Env == "production" {
		emailSender = email.NewSMTPSender(cfg.SMTP)
		logger.Info("using production SMTP sender")
	} else {
		emailSender = email.NewSMTPSender(cfg.SMTP) // use real SMTP for testing
		logger.Info("using development SMTP sender")
	}

	// --- Worker pool (email stream) with configurable size ---
	workerCount := 3
	if cfg.App.Env == "production" {
		workerCount = 5 // Increase workers in production
	}

	// Create location detector (can be nil if no API key provided)
	var locationDetector *location.Detector
	if cfg.Location.APIKey != "" {
		locationDetector = location.NewDetector(cfg.Location.APIKey, cfg.Location.APIURL)
		logger.Info("location detector initialized")
	}

	notifHandlers := notification.NewHandlers(emailSender, cfg.App.Name, cfg.App.URL, locationDetector)
	emailPool := &queue.WorkerPool{}

	// Build pool manually so each consumer gets its handlers registered
	for i := 1; i <= workerCount; i++ {
		name := fmt.Sprintf("email-worker-%d", i)
		c := queue.NewConsumer(rdb, queue.StreamEmails, "email-workers", name)
		notifHandlers.Register(c)
		emailPool.Add(c)
	}

	logger.Info("email worker pool initialized", zap.Int("workers", workerCount))

	// --- Graceful shutdown setup ---
	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start email workers
	emailErrChan := make(chan error, 1)
	go func() {
		if err := emailPool.Start(shutdownCtx); err != nil {
			emailErrChan <- fmt.Errorf("failed to start email workers: %w", err)
		}
	}()

	// Start server
	serverErrChan := make(chan error, 1)
	go func() {
		logger.Info("server starting",
			zap.String("addr", cfg.App.Addr),
			zap.String("env", cfg.App.Env),
			zap.String("pid", fmt.Sprintf("%d", os.Getpid())),
		)
		if err := app.Listen(cfg.App.Addr); err != nil {
			serverErrChan <- fmt.Errorf("server listen error: %w", err)
		}
	}()

	// Wait for interrupt signal or error
	select {
	case <-shutdownCtx.Done():
		logger.Info("shutdown signal received")
	case err := <-emailErrChan:
		logger.Fatal("email worker error", zap.Error(err))
	case err := <-serverErrChan:
		logger.Fatal("server error", zap.Error(err))
	}

	// Graceful shutdown with timeout
	shutdownTimeout := 30 * time.Second
	if cfg.App.Env == "production" {
		shutdownTimeout = 60 * time.Second
	}

	gracefulCtx, cancel := context.WithTimeout(shutdownCtx, shutdownTimeout)
	defer cancel()

	// Stop accepting new requests
	if err := app.ShutdownWithContext(gracefulCtx); err != nil {
		logger.Error("graceful shutdown error", zap.Error(err))
	}

	// Wait for email workers to finish
	emailPool.Wait()
	logger.Info("email workers stopped")

	// Additional cleanup if needed
	select {
	case err := <-emailErrChan:
		logger.Error("email worker final error", zap.Error(err))
	default:
	}

	logger.Info("server stopped cleanly")
}
