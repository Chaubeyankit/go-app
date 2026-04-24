package email

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"strings"

	"github.com/ankit.chaubey/myapp/config"
)

// Sender is the interface every email backend must satisfy.
// Swap SMTP for SendGrid/SES by implementing this interface — no other code changes.
type Sender interface {
	Send(msg *Message) error
}

// Message represents an outgoing email.
type Message struct {
	From    string
	To      string
	Subject string
	HTML    string
	Text    string // plain-text fallback
}

// SMTPSender sends via a plain SMTP server (works with Mailhog locally,
// and any SMTP relay in production).
type SMTPSender struct {
	cfg config.SMTPConfig
}

func NewSMTPSender(cfg config.SMTPConfig) Sender {
	return &SMTPSender{cfg: cfg}
}

func (s *SMTPSender) Send(msg *Message) error {
	// Set the From field if not provided
	if msg.From == "" {
		msg.From = s.cfg.From
	}

	body := buildMIME(msg)
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	// Create authentication
	var auth smtp.Auth
	if s.cfg.Username != "" && s.cfg.Password != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	// For SendGrid and Office 365, use standard smtp.SendMail with explicit auth
	if s.cfg.Host == "smtp.sendgrid.net" || s.cfg.Host == "smtp.office365.com" {
		// These providers require explicit authentication
		if auth == nil {
			auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
		}
		return smtp.SendMail(addr, auth, msg.From, []string{msg.To}, []byte(body))
	}

	// Regular SMTP without TLS
	return smtp.SendMail(addr, auth, msg.From, []string{msg.To}, []byte(body))
}


func buildMIME(msg *Message) string {
	var b strings.Builder
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString(fmt.Sprintf("From: %s\r\n", msg.From))
	b.WriteString(fmt.Sprintf("To: %s\r\n", msg.To))
	b.WriteString(fmt.Sprintf("Subject: %s\r\n", msg.Subject))
	b.WriteString("Content-Type: multipart/alternative; boundary=\"boundary42\"\r\n\r\n")
	b.WriteString("--boundary42\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString(msg.Text + "\r\n")
	b.WriteString("--boundary42\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	b.WriteString(msg.HTML + "\r\n")
	b.WriteString("--boundary42--\r\n")
	return b.String()
}

// NoopSender discards all emails — use in tests and development.
type NoopSender struct{}

func (n *NoopSender) Send(msg *Message) error { return nil }

// RenderTemplate renders an HTML template from a string with the given data.
func RenderTemplate(tmpl string, data interface{}) (string, error) {
	t, err := template.New("email").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("template.Parse: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template.Execute: %w", err)
	}
	return buf.String(), nil
}
