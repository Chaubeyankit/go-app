package apikey

import (
	"fmt"

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

func (h *Handler) RegisterRoutes(app *fiber.App, jwtAuth fiber.Handler) {
	g := app.Group("/api/v1/api-keys", jwtAuth)
	g.Post("/", h.Create)
	g.Get("/", h.List)
	g.Delete("/:id", h.Revoke)
}

func (h *Handler) Create(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return apperrors.BadRequest("invalid request body")
	}
	if fields := validator.Validate(req); fields != nil {
		return c.Status(422).JSON(
			response.ErrWithFields(apperrors.UnprocessableEntity("validation failed"), fields),
		)
	}

	// Validate scopes against allowed list
	if err := validateScopes(req.Scopes); err != nil {
		return err
	}

	resp, err := h.svc.Create(c.UserContext(), userID, &req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(response.OK(resp))
}

func (h *Handler) List(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	keys, err := h.svc.List(c.UserContext(), userID)
	if err != nil {
		return err
	}

	// Never return the hash — only safe display fields
	type safeKey struct {
		ID         string   `json:"id"`
		Name       string   `json:"name"`
		KeyPrefix  string   `json:"keyPrefix"`
		Scopes     []string `json:"scopes"`
		LastUsedAt *string  `json:"lastUsedAt,omitempty"`
		ExpiresAt  *string  `json:"expiresAt,omitempty"`
		CreatedAt  string   `json:"createdAt"`
	}

	result := make([]safeKey, 0, len(keys))
	for _, k := range keys {
		sk := safeKey{
			ID:        k.ID.String(),
			Name:      k.Name,
			KeyPrefix: k.KeyPrefix,
			Scopes:    k.Scopes,
			CreatedAt: k.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
		if k.LastUsedAt != nil {
			s := k.LastUsedAt.Format("2006-01-02T15:04:05Z")
			sk.LastUsedAt = &s
		}
		if k.ExpiresAt != nil {
			s := k.ExpiresAt.Format("2006-01-02T15:04:05Z")
			sk.ExpiresAt = &s
		}
		result = append(result, sk)
	}

	return c.JSON(response.OK(result))
}

func (h *Handler) Revoke(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	keyID := c.Params("id")

	if err := h.svc.Revoke(c.UserContext(), keyID, userID); err != nil {
		return err
	}
	return c.JSON(response.OK(fiber.Map{"message": "API key revoked"}))
}

var allowedScopes = map[string]struct{}{
	ScopeReadUsers:  {},
	ScopeWriteUsers: {},
	ScopeReadData:   {},
	ScopeWriteData:  {},
	ScopeAdmin:      {},
}

func validateScopes(scopes []string) error {
	for _, s := range scopes {
		if _, ok := allowedScopes[s]; !ok {
			return apperrors.BadRequest(fmt.Sprintf("invalid scope: %s", s))
		}
	}
	return nil
}
