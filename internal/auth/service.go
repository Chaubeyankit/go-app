package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/ankit.chaubey/myapp/config"
	"github.com/ankit.chaubey/myapp/internal/notification"
	"github.com/ankit.chaubey/myapp/pkg/apperrors"
	"github.com/ankit.chaubey/myapp/pkg/jwt"
	"github.com/ankit.chaubey/myapp/pkg/logger"
	"github.com/ankit.chaubey/myapp/pkg/queue"
)

type Service interface {
	Signup(ctx context.Context, req *SignupRequest) (*AuthResponse, error)
	Login(ctx context.Context, req *LoginRequest, ip, ua string) (*AuthResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error

	ForgotPassword(ctx context.Context, req *ForgotPasswordRequest) error
	ResetPassword(ctx context.Context, req *ResetPasswordRequest) error
	ChangePassword(ctx context.Context, userID string, req *ChangePasswordRequest) error
}

type service struct {
	repo        Repository
	tokenStore  TokenStore
	jwtCfg      config.JWTConfig
	producer    *queue.Producer
	mfaService  MFAService
	securityCfg config.SecurityConfig
}

func NewService(
	repo Repository,
	tokenStore TokenStore,
	producer *queue.Producer,
	jwtCfg config.JWTConfig,
	mfamfaService MFAService,
	securityCfg config.SecurityConfig,
) Service {
	return &service{
		repo:        repo,
		tokenStore:  tokenStore,
		producer:    producer,
		jwtCfg:      jwtCfg,
		mfaService:  mfamfaService,
		securityCfg: securityCfg,
	}
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
		// Check for duplicate key violation (race condition)
		if IsDuplicateKeyError(err) {
			return nil, apperrors.Conflict("email already registered")
		}
		return nil, apperrors.InternalError(fmt.Errorf("Signup.Create: %w", err))
	}

	logger.WithContext(ctx).Info("user registered", zap.String("user_id", user.ID.String()))

	// Send welcome email
	_, _ = s.producer.Publish(ctx, queue.StreamEmails, queue.EventWelcomeEmail,
		notification.WelcomeEmailPayload{
			UserID: user.ID.String(),
			Email:  user.Email,
			Name:   user.Name,
		})

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

	// Check if account is locked
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		remainingTime := time.Until(*user.LockedUntil)
		return nil, apperrors.TooManyRequests(fmt.Sprintf("account locked. try again in %d minutes", int(remainingTime.Minutes()+1)))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		// Increment failed login attempts
		_ = s.repo.IncrementFailedLoginAttempts(ctx, user.ID)

		_ = s.repo.CreateAuditLog(ctx, &AuditLog{
			UserID:    user.ID,
			Action:    "login_failed",
			IPAddress: ip,
			UserAgent: ua,
		})
		return nil, apperrors.Unauthorized("invalid credentials")
	}

	// Reset failed login attempts on successful login
	_ = s.repo.ResetFailedLoginAttempts(ctx, user.ID)

	// MFA is enabled — don't issue full tokens yet
	// Return a challenge token instead, client must POST /auth/mfa/challenge
	if user.MFAEnabled {
		challengeToken, err := s.mfaService.IssueMFAChallenge(ctx, user.ID.String())
		if err != nil {
			return nil, err
		}
		// Return 200 with a special shape — frontend detects mfa_required
		return &AuthResponse{
			MFARequired:       true,
			MFAChallengeToken: challengeToken,
		}, nil
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

		// Send login notification email
		_, _ = s.producer.Publish(context.Background(), queue.StreamEmails, queue.EventLoginNotification,
			notification.LoginNotificationPayload{
				UserID:    user.ID.String(),
				Email:     user.Email,
				Name:      user.Name,
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

func (s *service) ForgotPassword(ctx context.Context, req *ForgotPasswordRequest) error {
	user, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		// Always return success — never reveal whether an email exists.
		// This prevents user enumeration via the forgot-password endpoint.
		logger.WithContext(ctx).Debug("forgot password: email not found (silent)")
		return nil
	}

	// Invalidate any outstanding reset tokens for this user first
	_ = s.repo.InvalidateOldResets(ctx, user.ID)

	// Generate a cryptographically secure random token
	rawToken, tokenHash, err := generateSecureToken()
	if err != nil {
		return apperrors.InternalError(fmt.Errorf("ForgotPassword generateToken: %w", err))
	}

	expiresAt := time.Now().UTC().Add(30 * time.Minute)

	if err := s.repo.CreatePasswordReset(ctx, &PasswordReset{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	}); err != nil {
		return apperrors.InternalError(fmt.Errorf("ForgotPassword CreateReset: %w", err))
	}

	// Publish email job to the queue — non-blocking
	_, err = s.producer.Publish(ctx, queue.StreamEmails, queue.EventPasswordResetEmail,
		notification.PasswordResetPayload{
			UserID:    user.ID.String(),
			Email:     user.Email,
			Name:      user.Name,
			RawToken:  rawToken,
			ExpiresIn: "30 minutes",
		},
	)
	if err != nil {
		// Log but don't fail the request — the token is persisted,
		// and the email can be retried separately
		logger.WithContext(ctx).Error("failed to publish reset email job", zap.Error(err))
	}

	return nil
}

func (s *service) ResetPassword(ctx context.Context, req *ResetPasswordRequest) error {
	tokenHash := hashToken(req.Token)

	// First, find the token to get the user ID
	pr, err := s.repo.FindPasswordReset(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.BadRequest("invalid or expired reset token")
		}
		return apperrors.InternalError(fmt.Errorf("ResetPassword FindReset: %w", err))
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return apperrors.InternalError(fmt.Errorf("ResetPassword bcrypt: %w", err))
	}

	// Update password and mark reset token as used atomically
	if err := s.repo.ConsumePasswordReset(ctx, tokenHash, pr.UserID, string(newHash)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.BadRequest("invalid or expired reset token (may have been already used)")
		}
		return apperrors.InternalError(fmt.Errorf("ResetPassword Consume: %w", err))
	}

	// Revoke all active refresh tokens — forces re-login on all devices
	_ = s.tokenStore.RevokeAll(ctx, pr.UserID.String())

	// Find user to get email/name for the confirmation email
	user, err := s.repo.FindByID(ctx, pr.UserID)
	if err == nil {
		changedAt := time.Now().UTC().Format("2006-01-02 15:04:05 MST")
		_, _ = s.producer.Publish(ctx, queue.StreamEmails, queue.EventPasswordChanged,
			notification.PasswordChangedPayload{
				UserID:    user.ID.String(),
				Email:     user.Email,
				Name:      user.Name,
				ChangedAt: changedAt,
			},
		)
	}

	logger.WithContext(ctx).Info("password reset successful",
		zap.String("user_id", pr.UserID.String()),
	)
	return nil
}

func (s *service) ChangePassword(ctx context.Context, userID string, req *ChangePasswordRequest) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return apperrors.BadRequest("invalid user id")
	}

	user, err := s.repo.FindByID(ctx, uid)
	if err != nil {
		return apperrors.NotFound("user")
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		return apperrors.Unauthorized("current password is incorrect")
	}

	// Hash new password
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return apperrors.InternalError(fmt.Errorf("ChangePassword bcrypt: %w", err))
	}

	// Update password
	if err := s.repo.UpdatePassword(ctx, user.ID, string(newHash)); err != nil {
		return apperrors.InternalError(fmt.Errorf("ChangePassword UpdatePassword: %w", err))
	}

	// Revoke all active refresh tokens — forces re-login on all devices
	_ = s.tokenStore.RevokeAll(ctx, user.ID.String())

	// Send password change notification email
	changedAt := time.Now().UTC().Format("2006-01-02 15:04:05 MST")
	_, _ = s.producer.Publish(ctx, queue.StreamEmails, queue.EventPasswordChanged,
		notification.PasswordChangedPayload{
			UserID:    user.ID.String(),
			Email:     user.Email,
			Name:      user.Name,
			ChangedAt: changedAt,
		},
	)

	logger.WithContext(ctx).Info("password changed",
		zap.String("user_id", user.ID.String()),
	)
	return nil
}

// --- Token helpers ---

// generateSecureToken creates a URL-safe random token and its SHA-256 hash.
// The raw token goes in the email URL. Only the hash is stored in the DB.
func generateSecureToken() (rawToken, tokenHash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	rawToken = base64.URLEncoding.EncodeToString(b)
	tokenHash = hashToken(rawToken)
	return rawToken, tokenHash, nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func toUserResponse(u *User) UserResponse {
	return UserResponse{
		ID:    u.ID.String(),
		Email: u.Email,
		Name:  u.Name,
		Role:  string(u.Role),
	}
}

// IsDuplicateKeyError checks if the error is a PostgreSQL duplicate key violation
func IsDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}

	// Check for PostgreSQL duplicate key error code (23505)
	errStr := err.Error()
	return strings.Contains(errStr, "duplicate key") ||
		strings.Contains(errStr, "SQLSTATE 23505") ||
		strings.Contains(errStr, "users_email_key")
}
