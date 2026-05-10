package auth

import (
	"github.com/ankit.chaubey/myapp/config"
	"github.com/ankit.chaubey/myapp/internal/middleware"
	"github.com/ankit.chaubey/myapp/pkg/apperrors"
	"github.com/ankit.chaubey/myapp/pkg/response"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

type MFAHandler struct {
	svc MFAService
	rdb *redis.Client
}

func NewMFAHandler(svc MFAService, rdb *redis.Client) *MFAHandler {
	return &MFAHandler{svc: svc, rdb: rdb}
}

func (h *MFAHandler) RegisterRoutes(app *fiber.App, jwtCfg config.JWTConfig, securityCfg config.SecurityConfig) {
	// Challenge endpoint is public with aggressive rate limiting
	// 5 attempts per 5 minutes per IP to prevent TOTP brute force
	mfaChallengeRateLimiter := middleware.RateLimiter(
		h.rdb,
		securityCfg.MFARateLimitAttempts,
		securityCfg.MFARateLimitWindow,
		middleware.ByIP,
	)
	app.Post("/api/v1/auth/mfa/challenge", mfaChallengeRateLimiter, h.Challenge)

	// All MFA management routes require a valid JWT
	g := app.Group("/api/v1/auth/mfa", middleware.JWTAuth(jwtCfg))
	g.Post("/setup", h.Setup)
	g.Post("/verify", h.Enable) // confirm setup by submitting first code
	g.Post("/disable", h.Disable)
}

func (h *MFAHandler) Setup(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	resp, err := h.svc.Setup(c.UserContext(), userID)
	if err != nil {
		return err
	}
	return c.JSON(response.OK(resp))
}

func (h *MFAHandler) Enable(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)

	var req struct {
		Code string `json:"code" validate:"required,len=6"`
	}
	if err := c.BodyParser(&req); err != nil {
		return apperrors.BadRequest("invalid request body")
	}

	if err := h.svc.Enable(c.UserContext(), userID, req.Code); err != nil {
		return err
	}
	return c.JSON(response.OK(fiber.Map{"message": "MFA enabled successfully"}))
}

func (h *MFAHandler) Disable(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)

	var req struct {
		Password string `json:"password" validate:"required"`
	}
	if err := c.BodyParser(&req); err != nil {
		return apperrors.BadRequest("invalid request body")
	}

	if err := h.svc.Disable(c.UserContext(), userID, req.Password); err != nil {
		return err
	}
	return c.JSON(response.OK(fiber.Map{"message": "MFA disabled"}))
}

func (h *MFAHandler) Challenge(c *fiber.Ctx) error {
	var req struct {
		ChallengeToken string `json:"challengeToken" validate:"required"`
		Code           string `json:"code"            validate:"required,len=6"`
	}
	if err := c.BodyParser(&req); err != nil {
		return apperrors.BadRequest("invalid request body")
	}

	resp, err := h.svc.CompleteMFAChallenge(c.UserContext(), req.ChallengeToken, req.Code)
	if err != nil {
		return err
	}
	return c.JSON(response.OK(resp))
}
