package user

import (
	"github.com/ankit.chaubey/myapp/config"
	"github.com/ankit.chaubey/myapp/internal/middleware"
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

func (h *Handler) RegisterRoutes(app *fiber.App, jwtCfg config.JWTConfig) {
	g := app.Group("/api/v1/users", middleware.JWTAuth(jwtCfg))

	// Any authenticated user
	g.Get("/me", h.GetMe)
	g.Patch("/me", h.UpdateMe)

	// Admin only
	admin := g.Group("", middleware.RequireRole("admin"))
	admin.Get("/", h.ListUsers)
	admin.Delete("/:id", h.DeleteUser)
	admin.Get("/:id", h.GetUser)
}

func (h *Handler) GetMe(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	user, err := h.svc.GetByID(c.UserContext(), userID)
	if err != nil {
		return err
	}
	return c.JSON(response.OK(user))
}

func (h *Handler) UpdateMe(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)

	var req UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return apperrors.BadRequest("invalid request body")
	}
	if fields := validator.Validate(req); fields != nil {
		return c.Status(422).JSON(
			response.ErrWithFields(apperrors.UnprocessableEntity("validation failed"), fields),
		)
	}
	user, err := h.svc.UpdateProfile(c.UserContext(), userID, &req)
	if err != nil {
		return err
	}
	return c.JSON(response.OK(user))
}

func (h *Handler) GetUser(c *fiber.Ctx) error {
	id := c.Params("id")
	user, err := h.svc.GetByID(c.UserContext(), id)
	if err != nil {
		return err
	}
	return c.JSON(response.OK(user))
}

func (h *Handler) ListUsers(c *fiber.Ctx) error {
	var req ListUsersRequest
	if err := c.QueryParser(&req); err != nil {
		return apperrors.BadRequest("invalid query parameters")
	}
	if fields := validator.Validate(req); fields != nil {
		return c.Status(422).JSON(
			response.ErrWithFields(apperrors.UnprocessableEntity("validation failed"), fields),
		)
	}

	users, meta, err := h.svc.ListUsers(c.UserContext(), &req)
	if err != nil {
		return err
	}
	return c.JSON(response.Paginated(users, meta))
}

func (h *Handler) DeleteUser(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.svc.DeleteUser(c.UserContext(), id); err != nil {
		return err
	}
	return c.JSON(response.OK(fiber.Map{"message": "user deleted"}))
}
