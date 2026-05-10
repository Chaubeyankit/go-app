package auth

import (
	"fmt"
	"net/url"
	"time"

	"github.com/ankit.chaubey/myapp/internal/auth/oauth"
	"github.com/ankit.chaubey/myapp/pkg/apperrors"
	"github.com/gofiber/fiber/v2"
)

type OAuthHandler struct {
	svc OAuthService
}

func NewOAuthHandler(svc OAuthService) *OAuthHandler {
	return &OAuthHandler{svc: svc}
}

func (h *OAuthHandler) RegisterRoutes(app *fiber.App) {
	g := app.Group("/api/v1/auth/oauth")
	g.Get("/:provider", h.Redirect)
	g.Get("/callback/:provider", h.Callback)
}

// Redirect sends the browser to the OAuth provider's consent page.
func (h *OAuthHandler) Redirect(c *fiber.Ctx) error {
	provider := oauth.Provider(c.Params("provider"))

	authURL, state, err := h.svc.GetAuthURL(provider)
	if err != nil {
		return err
	}

	// Store frontend URL from query parameter, Origin or Referer header for callback redirect
	frontendURL := c.Query("redirect_uri")
	if frontendURL == "" {
		frontendURL = c.Get("Origin")
	}
	if frontendURL == "" {
		frontendURL = c.Get("Referer")
	}
	if frontendURL == "" {
		return apperrors.BadRequest("cannot determine frontend URL - please provide redirect_uri parameter, Origin or Referer header")
	}

	// Store state in a short-lived, HttpOnly cookie for CSRF verification
	c.Cookie(&fiber.Cookie{
		Name:     "oauth_state",
		Value:    state,
		MaxAge:   int(10 * time.Minute.Seconds()),
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Lax",
	})

	// Store frontend URL for callback redirect
	c.Cookie(&fiber.Cookie{
		Name:     "oauth_frontend_url",
		Value:    frontendURL,
		MaxAge:   int(10 * time.Minute.Seconds()),
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Lax",
	})

	return c.Redirect(authURL, fiber.StatusTemporaryRedirect)
}

// Callback receives the provider's redirect, verifies state, exchanges code,
// and redirects back to frontend with tokens in URL hash
func (h *OAuthHandler) Callback(c *fiber.Ctx) error {
	provider := oauth.Provider(c.Params("provider"))
	code := c.Query("code")
	state := c.Query("state")
	savedState := c.Cookies("oauth_state")
	frontendURL := c.Cookies("oauth_frontend_url")

	if code == "" {
		return apperrors.BadRequest("missing OAuth code")
	}

	// Clear the state and frontend URL cookies immediately
	c.Cookie(&fiber.Cookie{
		Name:   "oauth_state",
		Value:  "",
		MaxAge: -1,
	})
	c.Cookie(&fiber.Cookie{
		Name:   "oauth_frontend_url",
		Value:  "",
		MaxAge: -1,
	})

	// Validate frontend URL
	if frontendURL == "" {
		return apperrors.InternalError(fmt.Errorf("cannot determine frontend URL - please ensure your client sends Origin header"))
	}

	resp, err := h.svc.HandleCallback(c.UserContext(), provider, code, state, savedState)
	if err != nil {
		return err
	}

	// Construct callback URL with tokens
	frontendCallback := fmt.Sprintf("%s/oauth/callback#access_token=%s&refresh_token=%s",
		frontendURL,
		url.QueryEscape(resp.Tokens.AccessToken),
		url.QueryEscape(resp.Tokens.RefreshToken),
	)

	return c.Redirect(frontendCallback)
}
