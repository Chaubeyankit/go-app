package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"github.com/ankit.chaubey/myapp/config"
	"github.com/ankit.chaubey/myapp/internal/auth/oauth"
	"github.com/ankit.chaubey/myapp/pkg/apperrors"
	"github.com/ankit.chaubey/myapp/pkg/crypto"
	"github.com/ankit.chaubey/myapp/pkg/jwt"
	"github.com/ankit.chaubey/myapp/pkg/logger"
	"go.uber.org/zap"
)

type OAuthService interface {
	GetAuthURL(provider oauth.Provider) (url, state string, err error)
	HandleCallback(ctx context.Context, provider oauth.Provider, code, state, savedState string) (*AuthResponse, error)
}

type oauthService struct {
	providers  map[oauth.Provider]*oauth2.Config
	repo       Repository
	oauthRepo  OAuthRepository
	tokenStore TokenStore
	jwtCfg     config.JWTConfig
	encryptionKey []byte
}

func NewOAuthService(
	providers map[oauth.Provider]*oauth2.Config,
	repo Repository,
	oauthRepo OAuthRepository,
	tokenStore TokenStore,
	jwtCfg config.JWTConfig,
	encryptionKey []byte,
) OAuthService {
	return &oauthService{
		providers:  providers,
		repo:       repo,
		oauthRepo:  oauthRepo,
		tokenStore: tokenStore,
		jwtCfg:     jwtCfg,
		encryptionKey: encryptionKey,
	}
}

// GetAuthURL generates a provider redirect URL with a CSRF state token.
// The caller must store `state` in a short-lived cookie to verify on callback.
func (s *oauthService) GetAuthURL(provider oauth.Provider) (url, state string, err error) {
	cfg, ok := s.providers[provider]
	if !ok {
		return "", "", apperrors.BadRequest(fmt.Sprintf("unsupported provider: %s", provider))
	}

	state, err = generateState()
	if err != nil {
		return "", "", apperrors.InternalError(fmt.Errorf("GetAuthURL generateState: %w", err))
	}

	// AccessTypeOffline requests a refresh token from Google
	url = cfg.AuthCodeURL(state, oauth2.AccessTypeOffline)
	return url, state, nil
}

func (s *oauthService) HandleCallback(ctx context.Context, provider oauth.Provider, code, state, savedState string) (*AuthResponse, error) {
	// CSRF check — state must match what we generated in GetAuthURL
	if state != savedState {
		return nil, apperrors.Unauthorized("invalid OAuth state — possible CSRF attack")
	}

	cfg, ok := s.providers[provider]
	if !ok {
		return nil, apperrors.BadRequest("unsupported provider")
	}

	// Exchange code → token
	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, apperrors.Unauthorized(fmt.Sprintf("OAuth code exchange failed: %v", err))
	}

	// Fetch normalised user info from provider
	info, err := oauth.FetchUserInfo(ctx, provider, token)
	if err != nil {
		return nil, apperrors.InternalError(fmt.Errorf("HandleCallback FetchUserInfo: %w", err))
	}
	// Check if this OAuth identity already exists
	identity, err := s.oauthRepo.FindIdentity(ctx, string(provider), info.ProviderID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperrors.InternalError(fmt.Errorf("HandleCallback FindIdentity: %w", err))
	}

	var user *User

	if identity != nil {
		// Returning OAuth user — look them up
		uid, _ := uuid.Parse(identity.UserID.String())
		user, err = s.repo.FindByID(ctx, uid)
		if err != nil {
			return nil, apperrors.InternalError(fmt.Errorf("HandleCallback FindByID: %w", err))
		}
	} else {
		// New OAuth user — check if the email already has a local account
		user, err = s.repo.FindByEmail(ctx, info.Email)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.InternalError(fmt.Errorf("HandleCallback FindByEmail: %w", err))
		}

		if user == nil {
			// Brand new user — create account (no password)
			user = &User{
				Email:           info.Email,
				Name:            info.Name,
				Role:            RoleUser,
				IsEmailVerified: true, // OAuth emails are pre-verified
				PasswordHash:    "",   // no password for OAuth-only accounts
			}
			if err := s.repo.Create(ctx, user); err != nil {
				return nil, apperrors.InternalError(fmt.Errorf("HandleCallback Create: %w", err))
			}
			logger.WithContext(ctx).Info("new user via OAuth",
				zap.String("provider", string(provider)),
				zap.String("user_id", user.ID.String()),
			)
		}

		// Link this OAuth identity to the user (new or existing)
		// Encrypt the access token before storing
		encryptedToken, err := crypto.EncryptString(token.AccessToken, s.encryptionKey)
		if err != nil {
			return nil, apperrors.InternalError(fmt.Errorf("HandleCallback encrypt: %w", err))
		}

		newIdentity := &OAuthIdentity{
			UserID:      user.ID,
			Provider:    string(provider),
			ProviderID:  info.ProviderID,
			Email:       info.Email,
			Name:        info.Name,
			AvatarURL:   info.AvatarURL,
			AccessToken: encryptedToken,
		}
		if err := s.oauthRepo.CreateIdentity(ctx, newIdentity); err != nil {
			return nil, apperrors.InternalError(fmt.Errorf("HandleCallback CreateIdentity: %w", err))
		}
	}

	tokens, err := s.issueTokensForUser(ctx, user)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{User: toUserResponse(user), Tokens: *tokens}, nil
}

// issueTokensForUser is shared between OAuthService and the main Service.
// Kept separate to avoid a circular dependency.
func (s *oauthService) issueTokensForUser(ctx context.Context, user *User) (*TokenPair, error) {
	accessToken, err := jwt.IssueAccess(s.jwtCfg, user.ID.String(), user.Email, string(user.Role))
	if err != nil {
		return nil, apperrors.InternalError(fmt.Errorf("issueTokensForUser access: %w", err))
	}
	refreshToken, tokenID, err := jwt.IssueRefresh(s.jwtCfg, user.ID.String())
	if err != nil {
		return nil, apperrors.InternalError(fmt.Errorf("issueTokensForUser refresh: %w", err))
	}
	if err := s.tokenStore.Save(ctx, tokenID, user.ID.String(), s.jwtCfg.RefreshTTL); err != nil {
		return nil, apperrors.InternalError(fmt.Errorf("issueTokensForUser save: %w", err))
	}
	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.jwtCfg.AccessTTL.Seconds()),
	}, nil
}

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}
