package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
)

// UserInfo is the normalised profile we extract from any provider.
type UserInfo struct {
	ProviderID string
	Email      string
	Name       string
	AvatarURL  string
}

// FetchUserInfo retrieves and normalises the user profile from a provider
// using the exchanged OAuth2 token.
func FetchUserInfo(ctx context.Context, provider Provider, token *oauth2.Token) (*UserInfo, error) {
	switch provider {
	case ProviderGoogle:
		return fetchGoogle(ctx, token)
	case ProviderGitHub:
		return fetchGitHub(ctx, token)
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}
func fetchGoogle(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, fmt.Errorf("fetchGoogle: %w", err)
	}
	defer resp.Body.Close()

	var g struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		return nil, fmt.Errorf("fetchGoogle decode: %w", err)
	}
	return &UserInfo{
		ProviderID: g.Sub,
		Email:      g.Email,
		Name:       g.Name,
		AvatarURL:  g.Picture,
	}, nil
}

func fetchGitHub(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))

	// GitHub primary user info
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("fetchGitHub user: %w", err)
	}
	defer resp.Body.Close()

	var g struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		Email     string `json:"email"` // may be empty if private
	}
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		return nil, fmt.Errorf("fetchGitHub decode: %w", err)
	}
	// GitHub can have private emails — fetch the primary one separately
	email := g.Email
	if email == "" {
		email, err = fetchGitHubPrimaryEmail(ctx, client)
		if err != nil {
			return nil, err
		}
	}

	name := g.Name
	if name == "" {
		name = g.Login // fallback to username
	}

	return &UserInfo{
		ProviderID: fmt.Sprintf("%d", g.ID),
		Email:      email,
		Name:       name,
		AvatarURL:  g.AvatarURL,
	}, nil
}

func fetchGitHubPrimaryEmail(ctx context.Context, client *http.Client) (string, error) {
	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return "", fmt.Errorf("fetchGitHubPrimaryEmail: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var emails []struct {
		Email   string `json:"email"`
		Primary bool   `json:"primary"`
	}
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", fmt.Errorf("fetchGitHubPrimaryEmail decode: %w", err)
	}
	for _, e := range emails {
		if e.Primary {
			return e.Email, nil
		}
	}
	return "", fmt.Errorf("no primary email found on GitHub account")
}
