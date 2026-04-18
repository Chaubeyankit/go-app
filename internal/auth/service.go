package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/ankit.chaubey/myapp/config"
	"github.com/ankit.chaubey/myapp/pkg/apperrors"
	"github.com/ankit.chaubey/myapp/pkg/jwt"
	"github.com/ankit.chaubey/myapp/pkg/logger"
	"go.uber.org/zap"
)

type Service interface {
	Signup(ctx context.Context, req *SignupRequest) (*AuthResponse, error)
	Login(ctx context.Context, req *LoginRequest, ip, ua string) (*AuthResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
}

type service struct {
	repo       Repository
	tokenStore TokenStore
	jwtCfg     config.JWTConfig
}

func NewService(repo Repository, tokenStore TokenStore, jwtCfg config.JWTConfig) Service {
	return &service{repo: repo, tokenStore: tokenStore, jwtCfg: jwtCfg}
}

func (s *service) Signup(ctx context.Context, req *SignupRequest) (*AuthResponse, error) {
	exists, err := s.repo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		return nil, apperrors.InternalError(fmt.Errorf("Signup.ExistsByEmail: %w", err))
	}
	if exists {
		return nil, apperrors.Conflict("email already registered")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperrors.InternalError(fmt.Errorf("Signup.bcrypt: %w", err))
	}

	user := &User{
		Email:        req.Email,
		PasswordHash: string(hash),
		Name:         req.Name,
		Role:         RoleUser,
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, apperrors.InternalError(fmt.Errorf("Signup.Create: %w", err))
	}

	logger.WithContext(ctx).Info("user registered", zap.String("user_id", user.ID.String()))

	tokens, err := s.issueTokens(ctx, user)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		User:   toUserResponse(user),
		Tokens: *tokens,
	}, nil
}

func (s *service) Login(ctx context.Context, req *LoginRequest, ip, ua string) (*AuthResponse, error) {
	user, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Constant-time comparison to prevent timing attacks
			_ = bcrypt.CompareHashAndPassword([]byte("$2a$12$dummy"), []byte(req.Password))
			return nil, apperrors.Unauthorized("invalid credentials")
		}
		return nil, apperrors.InternalError(fmt.Errorf("Login.FindByEmail: %w", err))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		_ = s.repo.CreateAuditLog(ctx, &AuditLog{
			UserID:    user.ID,
			Action:    "login_failed",
			IPAddress: ip,
			UserAgent: ua,
		})
		return nil, apperrors.Unauthorized("invalid credentials")
	}

	// Record successful login (fire-and-forget style — don't block on it)
	go func() {
		_ = s.repo.UpdateLastLogin(context.Background(), user.ID)
		_ = s.repo.CreateAuditLog(context.Background(), &AuditLog{
			UserID:    user.ID,
			Action:    "login_success",
			IPAddress: ip,
			UserAgent: ua,
		})
	}()

	tokens, err := s.issueTokens(ctx, user)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		User:   toUserResponse(user),
		Tokens: *tokens,
	}, nil
}

func (s *service) RefreshToken(ctx context.Context, rawToken string) (*TokenPair, error) {
	claims, err := jwt.ParseRefresh(s.jwtCfg, rawToken)
	if err != nil {
		return nil, apperrors.Unauthorized("invalid refresh token")
	}

	exists, err := s.tokenStore.Exists(ctx, claims.TokenID)
	if err != nil || !exists {
		return nil, apperrors.Unauthorized("refresh token revoked or expired")
	}

	_ = s.tokenStore.Revoke(ctx, claims.TokenID)

	uid, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, apperrors.Unauthorized("invalid user id in token")
	}

	user, err := s.repo.FindByID(ctx, uid)
	if err != nil {
		return nil, apperrors.Unauthorized("user not found")
	}

	return s.issueTokens(ctx, user)
}

func (s *service) Logout(ctx context.Context, rawToken string) error {
	claims, err := jwt.ParseRefresh(s.jwtCfg, rawToken)
	if err != nil {
		return nil // already invalid — treat as success
	}
	return s.tokenStore.Revoke(ctx, claims.TokenID)
}

func (s *service) issueTokens(ctx context.Context, user *User) (*TokenPair, error) {
	accessToken, err := jwt.IssueAccess(s.jwtCfg, user.ID.String(), user.Email, string(user.Role))
	if err != nil {
		return nil, apperrors.InternalError(fmt.Errorf("issueTokens.access: %w", err))
	}

	refreshToken, tokenID, err := jwt.IssueRefresh(s.jwtCfg, user.ID.String())
	if err != nil {
		return nil, apperrors.InternalError(fmt.Errorf("issueTokens.refresh: %w", err))
	}

	if err := s.tokenStore.Save(ctx, tokenID, user.ID.String(), s.jwtCfg.RefreshTTL); err != nil {
		return nil, apperrors.InternalError(fmt.Errorf("issueTokens.save: %w", err))
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.jwtCfg.AccessTTL.Seconds()),
	}, nil
}

func toUserResponse(u *User) UserResponse {
	return UserResponse{
		ID:    u.ID.String(),
		Email: u.Email,
		Name:  u.Name,
		Role:  string(u.Role),
	}
}
