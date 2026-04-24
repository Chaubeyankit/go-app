package auth

import (
	"github.com/ankit.chaubey/myapp/pkg/apperrors"
	"github.com/ankit.chaubey/myapp/pkg/response"
	"github.com/ankit.chaubey/myapp/pkg/validator"
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(app *fiber.App, rateLimiter fiber.Handler) {
	g := app.Group("/api/v1/auth")
	g.Use(rateLimiter) // apply stricter rate limit to all auth routes
	g.Post("/signup", h.Signup)
	g.Post("/login", h.Login)
	g.Post("/refresh", h.Refresh)
	g.Post("/logout", h.Logout)

	g.Post("/forgot-password", h.ForgotPassword)
	g.Post("/reset-password", h.ResetPassword)
}

func (h *Handler) Signup(c *fiber.Ctx) error {
	var req SignupRequest
	if err := c.BodyParser(&req); err != nil {
		return apperrors.BadRequest("invalid request body")
	}
	if fields := validator.Validate(req); fields != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(
			response.ErrWithFields(apperrors.UnprocessableEntity("validation failed"), fields),
		)
	}

	resp, err := h.svc.Signup(c.UserContext(), &req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(response.OK(resp))
}

func (h *Handler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return apperrors.BadRequest("invalid request body")
	}
	if fields := validator.Validate(req); fields != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(
			response.ErrWithFields(apperrors.UnprocessableEntity("validation failed"), fields),
		)
	}

	resp, err := h.svc.Login(c.UserContext(), &req, c.IP(), c.Get("User-Agent"))
	if err != nil {
		return err
	}
	return c.JSON(response.OK(resp))
}

func (h *Handler) Refresh(c *fiber.Ctx) error {
	var req RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return apperrors.BadRequest("invalid request body")
	}

	tokens, err := h.svc.RefreshToken(c.UserContext(), req.RefreshToken)
	if err != nil {
		return err
	}
	return c.JSON(response.OK(tokens))
}

func (h *Handler) Logout(c *fiber.Ctx) error {
	var req LogoutRequest
	if err := c.BodyParser(&req); err != nil {
		return apperrors.BadRequest("invalid request body")
	}

	if err := h.svc.Logout(c.UserContext(), req.RefreshToken); err != nil {
		return err
	}
	return c.JSON(response.OK(fiber.Map{"message": "logged out successfully"}))
}

func (h *Handler) ForgotPassword(c *fiber.Ctx) error {
	var req ForgotPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return apperrors.BadRequest("invalid request body")
	}
	if fields := validator.Validate(req); fields != nil {
		return c.Status(422).JSON(
			response.ErrWithFields(apperrors.UnprocessableEntity("validation failed"), fields),
		)
	}

	// Always return 200 regardless of whether the email exists
	_ = h.svc.ForgotPassword(c.UserContext(), &req)
	return c.JSON(response.OK(fiber.Map{
		"message": "if that email is registered, a reset link has been sent",
	}))
}

func (h *Handler) ResetPassword(c *fiber.Ctx) error {
	var req ResetPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return apperrors.BadRequest("invalid request body")
	}
	if fields := validator.Validate(req); fields != nil {
		return c.Status(422).JSON(
			response.ErrWithFields(apperrors.UnprocessableEntity("validation failed"), fields),
		)
	}

	if err := h.svc.ResetPassword(c.UserContext(), &req); err != nil {
		return err
	}
	return c.JSON(response.OK(fiber.Map{
		"message": "password reset successful, please log in again",
	}))
}
