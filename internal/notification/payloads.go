package notification

// All job payload types live here — they are serialised into the stream.

type WelcomeEmailPayload struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

type PasswordResetPayload struct {
	UserID    string `json:"userId"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	RawToken  string `json:"rawToken"`  // the unhashed token to embed in the URL
	ExpiresIn string `json:"expiresIn"` // human-readable, e.g. "30 minutes"
}

type PasswordChangedPayload struct {
	UserID    string `json:"userId"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	ChangedAt string `json:"changedAt"`
}

type LoginNotificationPayload struct {
	UserID    string `json:"userId"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	IPAddress string `json:"ipAddress"`
	UserAgent string `json:"userAgent"`
	Location  string `json:"location"`
	Browser   string `json:"browser"`
	OS        string `json:"os"`
	LoginTime string `json:"loginTime"`
}
