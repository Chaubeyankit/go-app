package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/ankit.chaubey/myapp/pkg/email"
	"github.com/ankit.chaubey/myapp/pkg/location"
	"github.com/ankit.chaubey/myapp/pkg/logger"
	"github.com/ankit.chaubey/myapp/pkg/queue"
)

// Handlers holds all email job handlers. Inject the email.Sender so
// we can swap SMTP for a mock in tests without touching handler logic.
type Handlers struct {
	sender    email.Sender
	appName   string
	appURL    string
	location  *location.Detector
}

func NewHandlers(sender email.Sender, appName, appURL string, location *location.Detector) *Handlers {
	return &Handlers{sender: sender, appName: appName, appURL: appURL, location: location}
}

// Register wires all handlers onto the given consumer.
func (h *Handlers) Register(c *queue.Consumer) {
	c.Register(queue.EventWelcomeEmail, h.HandleWelcome)
	c.Register(queue.EventPasswordResetEmail, h.HandlePasswordReset)
	c.Register(queue.EventPasswordChanged, h.HandlePasswordChanged)
	c.Register(queue.EventLoginNotification, h.HandleLoginNotification)
}

func (h *Handlers) HandleWelcome(ctx context.Context, msg queue.Message) error {
	var p WelcomeEmailPayload
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return fmt.Errorf("HandleWelcome unmarshal: %w", err)
	}

	html, err := email.RenderTemplate(email.WelcomeTmpl, email.WelcomeData{
		Name: p.Name, AppName: h.appName, AppURL: h.appURL,
	})
	if err != nil {
		return fmt.Errorf("HandleWelcome render: %w", err)
	}
	text, _ := email.RenderTemplate(email.WelcomeText, email.WelcomeData{
		Name: p.Name, AppName: h.appName, AppURL: h.appURL,
	})

	if err := h.sender.Send(&email.Message{
		To:      p.Email,
		Subject: fmt.Sprintf("Welcome to %s!", h.appName),
		HTML:    html,
		Text:    text,
	}); err != nil {
		logger.WithContext(ctx).Error("failed to send welcome email",
			zap.String("user_id", p.UserID),
			zap.Error(err),
		)
		return err // will be retried
	}

	logger.WithContext(ctx).Info("welcome email sent", zap.String("user_id", p.UserID))
	return nil
}

func (h *Handlers) HandlePasswordReset(ctx context.Context, msg queue.Message) error {
	var p PasswordResetPayload
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return fmt.Errorf("HandlePasswordReset unmarshal: %w", err)
	}

	resetURL := fmt.Sprintf("%s/reset-password?token=%s", h.appURL, p.RawToken)

	html, err := email.RenderTemplate(email.PasswordResetTmpl, email.PasswordResetData{
		Name: p.Name, ResetURL: resetURL,
		ExpiresIn: p.ExpiresIn, AppName: h.appName,
	})
	if err != nil {
		return fmt.Errorf("HandlePasswordReset render: %w", err)
	}
	text, _ := email.RenderTemplate(email.PasswordResetText, email.PasswordResetData{
		Name: p.Name, ResetURL: resetURL,
		ExpiresIn: p.ExpiresIn, AppName: h.appName,
	})

	if err := h.sender.Send(&email.Message{
		To:      p.Email,
		Subject: fmt.Sprintf("Reset your %s password", h.appName),
		HTML:    html,
		Text:    text,
	}); err != nil {
		logger.WithContext(ctx).Error("failed to send reset email",
			zap.String("user_id", p.UserID),
			zap.Error(err),
		)
		return err
	}

	logger.WithContext(ctx).Info("password reset email sent", zap.String("user_id", p.UserID))
	return nil
}

func (h *Handlers) HandlePasswordChanged(ctx context.Context, msg queue.Message) error {
	var p PasswordChangedPayload
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return fmt.Errorf("HandlePasswordChanged unmarshal: %w", err)
	}

	html, err := email.RenderTemplate(email.PasswordChangedTmpl, email.PasswordChangedData{
		Name: p.Name, AppName: h.appName, AppURL: h.appURL,
	})
	if err != nil {
		return fmt.Errorf("HandlePasswordChanged render: %w", err)
	}

	text, err := email.RenderTemplate(email.PasswordChangedText, email.PasswordChangedData{
		Name: p.Name, AppName: h.appName, AppURL: h.appURL,
	})
	if err != nil {
		return fmt.Errorf("HandlePasswordChanged text render: %w", err)
	}

	if err := h.sender.Send(&email.Message{
		To:      p.Email,
		Subject: fmt.Sprintf("Your %s password was changed", h.appName),
		HTML:    html,
		Text:    text,
	}); err != nil {
		return err
	}

	logger.WithContext(ctx).Info("password-changed email sent", zap.String("user_id", p.UserID))
	return nil
}

func (h *Handlers) HandleLoginNotification(ctx context.Context, msg queue.Message) error {
	var p LoginNotificationPayload
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return fmt.Errorf("HandleLoginNotification unmarshal: %w", err)
	}

	// Get location info if not already provided
	if p.Location == "" && h.location != nil {
		location, err := h.location.DetectLocation(ctx, p.IPAddress)
		if err != nil {
			logger.WithContext(ctx).Error("failed to detect location", zap.Error(err))
		} else {
			p.Location = location.Country
		}
	}

	// Detect browser and OS if not already provided
	if p.Browser == "" {
		p.Browser = location.DetectBrowser(p.UserAgent)
	}

	if p.OS == "" {
		p.OS = location.DetectOS(p.UserAgent)
	}

	// Set current time if not provided
	if p.LoginTime == "" {
		p.LoginTime = time.Now().Format("2006-01-02 15:04:05 MST")
	}

	html, err := email.RenderTemplate(email.LoginNotificationTmpl, email.LoginNotificationData{
		Name:      p.Name,
		AppName:   h.appName,
		IPAddress: p.IPAddress,
		UserAgent: p.UserAgent,
		Location:  p.Location,
		Browser:   p.Browser,
		OS:        p.OS,
		LoginTime: p.LoginTime,
		AppURL:    h.appURL,
	})
	if err != nil {
		return fmt.Errorf("HandleLoginNotification render: %w", err)
	}

	text, _ := email.RenderTemplate(email.LoginNotificationText, email.LoginNotificationData{
		Name:      p.Name,
		AppName:   h.appName,
		IPAddress: p.IPAddress,
		UserAgent: p.UserAgent,
		Location:  p.Location,
		Browser:   p.Browser,
		OS:        p.OS,
		LoginTime: p.LoginTime,
		AppURL:    h.appURL,
	})

	if err := h.sender.Send(&email.Message{
		To:      p.Email,
		Subject: fmt.Sprintf("New Login to %s", h.appName),
		HTML:    html,
		Text:    text,
	}); err != nil {
		logger.WithContext(ctx).Error("failed to send login notification email",
			zap.String("user_id", p.UserID),
			zap.Error(err),
		)
		return err
	}

	logger.WithContext(ctx).Info("login notification email sent", zap.String("user_id", p.UserID))
	return nil
}
