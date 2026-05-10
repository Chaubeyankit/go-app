package auth

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"time"

	"crypto/rand"
	"encoding/base64"

	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
	"golang.org/x/crypto/bcrypt"

	"github.com/ankit.chaubey/myapp/config"
	"github.com/ankit.chaubey/myapp/pkg/apperrors"
	"github.com/ankit.chaubey/myapp/pkg/crypto"
	"github.com/ankit.chaubey/myapp/pkg/jwt"
	"github.com/ankit.chaubey/myapp/pkg/logger"
	"go.uber.org/zap"
)

// MFASetupResponse is returned by /auth/mfa/setup.
// The QR code PNG is base64-encoded — the frontend renders it as an <img>.
type MFASetupResponse struct {
	Secret     string `json:"secret"`      // show to user as backup
	QRCodeB64  string `json:"qr_code_b64"` // data:image/png;base64,...
	OTPAuthURL string `json:"otpAuthUrl"`
}

type MFAService interface {
	Setup(ctx context.Context, userID string) (*MFASetupResponse, error)
	Enable(ctx context.Context, userID, code string) error
	Disable(ctx context.Context, userID, password string) error
	ValidateCode(ctx context.Context, userID, code string) error
	// IssueMFAChallenge is called by Login when the user has MFA enabled.
	// It issues a short-lived "partial" token that can only be used to
	// call /auth/mfa/challenge — not for any other API endpoint.
	IssueMFAChallenge(ctx context.Context, userID string) (challengeToken string, err error)
	// CompleteMFAChallenge verifies the TOTP code and issues the full JWT pair.
	CompleteMFAChallenge(ctx context.Context, challengeToken, totpCode string) (*AuthResponse, error)
}

type mfaService struct {
	repo         Repository
	tokenStore   TokenStore
	jwtCfg       config.JWTConfig
	appName      string
	encryptionKey []byte
}

func NewMFAService(
	repo Repository,
	tokenStore TokenStore,
	jwtCfg config.JWTConfig,
	appName string,
	encryptionKey []byte,
) MFAService {
	return &mfaService{
		repo:         repo,
		tokenStore:   tokenStore,
		jwtCfg:       jwtCfg,
		appName:      appName,
		encryptionKey: encryptionKey,
	}
}

func (s *mfaService) Setup(ctx context.Context, userID string) (*MFASetupResponse, error) {

	user, err := s.findUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.MFAEnabled {
		return nil, apperrors.Conflict("MFA is already enabled for this account")
	}

	// Generate a new TOTP secret
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.appName,
		AccountName: user.Email,
		Period:      30,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return nil, apperrors.InternalError(fmt.Errorf("mfa.Setup generate: %w", err))
	}

	// Encrypt the secret before storing
	encryptedSecret, err := crypto.EncryptString(key.Secret(), s.encryptionKey)
	if err != nil {
		return nil, apperrors.InternalError(fmt.Errorf("mfa.Setup encrypt: %w", err))
	}

	// Store the encrypted secret
	// We store BEFORE confirming — user must verify with a valid code
	// via /auth/mfa/verify before we set mfa_enabled = true
	if err := s.repo.StoreMFASecret(ctx, user.ID, encryptedSecret); err != nil {
		return nil, apperrors.InternalError(fmt.Errorf("mfa.Setup store: %w", err))
	}

	// Generate QR code PNG
	qrPNG, err := generateQRCode(key.URL())
	if err != nil {
		return nil, apperrors.InternalError(fmt.Errorf("mfa.Setup QR: %w", err))
	}

	return &MFASetupResponse{
		Secret:     key.Secret(),
		QRCodeB64:  "data:image/png;base64," + base64Encode(qrPNG),
		OTPAuthURL: key.URL(),
	}, nil
}

func (s *mfaService) Enable(ctx context.Context, userID, code string) error {

	user, err := s.findUser(ctx, userID)
	if err != nil {
		return err
	}
	if user.MFAEnabled {
		return apperrors.Conflict("MFA already enabled")
	}
	if user.MFASecretEnc == "" {
		return apperrors.BadRequest("call /auth/mfa/setup first")
	}

	// Decrypt the secret before validation
	secret, err := crypto.DecryptString(user.MFASecretEnc, s.encryptionKey)
	if err != nil {
		return apperrors.InternalError(fmt.Errorf("mfa.Enable decrypt: %w", err))
	}

	// Validate the code against the stored secret
	valid := totp.Validate(code, secret)
	if !valid {
		return apperrors.Unauthorized("invalid TOTP code")
	}

	if err := s.repo.EnableMFA(ctx, user.ID); err != nil {
		return apperrors.InternalError(fmt.Errorf("mfa.Enable: %w", err))
	}

	logger.WithContext(ctx).Info("MFA enabled", zap.String("user_id", userID))
	return nil
}

func (s *mfaService) Disable(ctx context.Context, userID, password string) error {

	user, err := s.findUser(ctx, userID)
	if err != nil {
		return err
	}

	// Require password confirmation to disable MFA
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return apperrors.Unauthorized("incorrect password")
	}

	if err := s.repo.DisableMFA(ctx, user.ID); err != nil {
		return apperrors.InternalError(fmt.Errorf("mfa.Disable: %w", err))
	}

	logger.WithContext(ctx).Info("MFA disabled", zap.String("user_id", userID))
	return nil
}

func (s *mfaService) ValidateCode(ctx context.Context, userID, code string) error {
	user, err := s.findUser(ctx, userID)
	if err != nil {
		return err
	}
	if !user.MFAEnabled || user.MFASecretEnc == "" {
		return apperrors.BadRequest("MFA not enabled for this account")
	}

	// Decrypt the secret before validation
	secret, err := crypto.DecryptString(user.MFASecretEnc, s.encryptionKey)
	if err != nil {
		return apperrors.InternalError(fmt.Errorf("mfa.ValidateCode decrypt: %w", err))
	}

	// totp.ValidateCustom allows checking the previous and next window
	// to account for clock skew between user device and server
	valid, err := totp.ValidateCustom(code, secret, time.Now().UTC(),
		totp.ValidateOpts{
			Period:    30,
			Skew:      1, // accept codes ±30 seconds
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		},
	)
	if err != nil || !valid {
		return apperrors.Unauthorized("invalid or expired TOTP code")
	}
	return nil
}

// IssueMFAChallenge stores a short-lived challenge token in Redis.
// The token encodes only the userID and expires in 5 minutes.
// It cannot be used as a Bearer token for protected endpoints.
func (s *mfaService) IssueMFAChallenge(ctx context.Context, userID string) (string, error) {
	challengeToken, err := generateSecureToken32()
	if err != nil {
		return "", apperrors.InternalError(err)
	}

	key := "mfa_challenge:" + challengeToken
	if err := s.tokenStore.SetRaw(ctx, key, userID, 5*time.Minute); err != nil {
		return "", apperrors.InternalError(fmt.Errorf("IssueMFAChallenge store: %w", err))
	}
	return challengeToken, nil

}

// CompleteMFAChallenge validates the challenge token + TOTP code,
// then issues the full JWT pair.
func (s *mfaService) CompleteMFAChallenge(ctx context.Context, challengeToken, totpCode string) (*AuthResponse, error) {

	key := "mfa_challenge:" + challengeToken
	userID, err := s.tokenStore.GetRaw(ctx, key)
	if err != nil || userID == "" {
		return nil, apperrors.Unauthorized("MFA challenge expired or invalid")
	}

	// Single use — delete immediately after reading
	_ = s.tokenStore.DeleteRaw(ctx, key)

	if err := s.ValidateCode(ctx, userID, totpCode); err != nil {
		return nil, err
	}

	user, err := s.findUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Issue full JWT pair
	accessToken, err := jwt.IssueAccess(s.jwtCfg, user.ID.String(), user.Email, string(user.Role))
	if err != nil {
		return nil, apperrors.InternalError(err)
	}
	refreshToken, tokenID, err := jwt.IssueRefresh(s.jwtCfg, user.ID.String())
	if err != nil {
		return nil, apperrors.InternalError(err)
	}
	if err := s.tokenStore.Save(ctx, tokenID, user.ID.String(), s.jwtCfg.RefreshTTL); err != nil {
		return nil, apperrors.InternalError(err)
	}

	return &AuthResponse{
		User: toUserResponse(user),
		Tokens: TokenPair{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    int64(s.jwtCfg.AccessTTL.Seconds()),
		},
	}, nil
}

func (s *mfaService) findUser(ctx context.Context, userID string) (*User, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, apperrors.BadRequest("invalid user id")
	}
	user, err := s.repo.FindByID(ctx, uid)
	if err != nil {
		return nil, apperrors.NotFound("user")
	}
	return user, nil
}

func generateQRCode(content string) ([]byte, error) {
	q, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return nil, err
	}
	img := q.Image(256)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func generateSecureToken32() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
