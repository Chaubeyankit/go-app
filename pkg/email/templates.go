package email

// WelcomeData is the data injected into the welcome email template.
type WelcomeData struct {
	Name    string
	AppName string
	AppURL  string
}

// PasswordResetData is the data injected into the reset email template.
type PasswordResetData struct {
	Name      string
	ResetURL  string
	ExpiresIn string
	AppName   string
}

// PasswordChangedData is the data for the confirmation email.
type PasswordChangedData struct {
	Name      string
	AppName   string
	ChangedAt string
	AppURL    string
}

// LoginNotificationData is the data for the login notification email template.
type LoginNotificationData struct {
	Name      string
	AppName   string
	IPAddress string
	UserAgent string
	Location  string
	Browser   string
	OS        string
	LoginTime string
	AppURL    string
}

const WelcomeTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
  <title>Welcome to {{.AppName}}</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f4f5;font-family:'Segoe UI',Arial,sans-serif;">
  <table width="100%" cellpadding="0" cellspacing="0" style="padding:40px 16px;">
    <tr>
      <td align="center">
        <table width="560" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:12px;overflow:hidden;">

          <!-- Header -->
          <tr>
            <td style="background:#4F46E5;padding:32px 40px;text-align:center;">
              <h1 style="margin:0;color:#ffffff;font-size:22px;font-weight:600;letter-spacing:-0.3px;">
                {{.AppName}}
              </h1>
            </td>
          </tr>

          <!-- Body -->
          <tr>
            <td style="padding:36px 40px;">
              <h2 style="margin:0 0 10px;font-size:21px;font-weight:600;color:#111827;">
                Welcome aboard, {{.Name}}!
              </h2>
              <p style="margin:0 0 28px;font-size:15px;line-height:1.7;color:#4B5563;">
                Your account has been successfully created. We're thrilled to have
                you with us — you can log in anytime and get started right away.
              </p>

              <!-- CTA Button -->
              <table cellpadding="0" cellspacing="0" style="margin:0 0 28px;">
                <tr>
                  <td style="background:#4F46E5;border-radius:8px;">
                    <a href="{{.AppURL}}"
                       style="display:inline-block;padding:11px 26px;color:#ffffff;font-size:15px;font-weight:600;text-decoration:none;">
                      Go to {{.AppName}} →
                    </a>
                  </td>
                </tr>
              </table>

              <!-- Info Note -->
              <table width="100%" cellpadding="0" cellspacing="0">
                <tr>
                  <td style="background:#F5F3FF;border-left:3px solid #4F46E5;border-radius:0 6px 6px 0;padding:12px 16px;">
                    <p style="margin:0;font-size:13px;color:#4338CA;line-height:1.6;">
                      If you didn't create this account, you can safely ignore this email.
                    </p>
                  </td>
                </tr>
              </table>
            </td>
          </tr>

          <!-- Footer -->
          <tr>
            <td style="padding:16px 40px;border-top:1px solid #F3F4F6;text-align:center;">
              <p style="margin:0;font-size:11px;color:#9CA3AF;">
                © {{.AppName}} &middot; You're receiving this because an account was created with your email.
              </p>
            </td>
          </tr>

        </table>
      </td>
    </tr>
  </table>
</body>
</html>`

const WelcomeText = `Welcome to {{.AppName}}, {{.Name}}!

Your account has been successfully created. We're thrilled to have you with us.

Log in anytime at: {{.AppURL}}

If you didn't create this account, you can safely ignore this email.

— The {{.AppName}} Team`

const PasswordResetTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
  <title>Reset your password</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f4f5;font-family:'Segoe UI',Arial,sans-serif;">
  <table width="100%" cellpadding="0" cellspacing="0" style="padding:40px 16px;">
    <tr>
      <td align="center">
        <table width="560" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:12px;overflow:hidden;">

          <!-- Header -->
          <tr>
            <td style="background:#4F46E5;padding:32px 40px;text-align:center;">
              <h1 style="margin:0;color:#ffffff;font-size:22px;font-weight:600;letter-spacing:-0.3px;">
                {{.AppName}}
              </h1>
            </td>
          </tr>

          <!-- Body -->
          <tr>
            <td style="padding:36px 40px;">
              <h2 style="margin:0 0 6px;font-size:21px;font-weight:600;color:#111827;">
                Password reset request
              </h2>
              <p style="margin:0 0 24px;font-size:15px;line-height:1.7;color:#4B5563;">
                Hi <strong style="color:#111827;">{{.Name}}</strong>,<br/>
                We received a request to reset the password for your account.
                Click the button below to choose a new password.
                This link will expire in <strong style="color:#111827;">{{.ExpiresIn}}</strong>.
              </p>

              <!-- CTA Button -->
              <table cellpadding="0" cellspacing="0" style="margin:0 0 28px;">
                <tr>
                  <td style="background:#4F46E5;border-radius:8px;">
                    <a href="{{.ResetURL}}"
                       style="display:inline-block;padding:11px 26px;color:#ffffff;font-size:15px;font-weight:600;text-decoration:none;">
                      Reset my password →
                    </a>
                  </td>
                </tr>
              </table>

              <!-- Warning Note -->
              <table width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 20px;">
                <tr>
                  <td style="background:#FFFBEB;border-left:3px solid #F59E0B;border-radius:0 6px 6px 0;padding:12px 16px;">
                    <p style="margin:0;font-size:13px;color:#92400E;line-height:1.6;">
                      If you didn't request a password reset, you can safely ignore this email.
                      Your password will remain unchanged.
                    </p>
                  </td>
                </tr>
              </table>

              <!-- Expiry hint -->
              <p style="margin:0;font-size:12px;color:#9CA3AF;">
                This link expires in {{.ExpiresIn}} for your security.
              </p>
            </td>
          </tr>

          <!-- Footer -->
          <tr>
            <td style="padding:16px 40px;border-top:1px solid #F3F4F6;text-align:center;">
              <p style="margin:0;font-size:11px;color:#9CA3AF;">
                © {{.AppName}} &middot; This is an automated security email.
              </p>
            </td>
          </tr>

        </table>
      </td>
    </tr>
  </table>
</body>
</html>`

const PasswordResetText = `Password reset — {{.AppName}}

Hi {{.Name}},

We received a request to reset your password.
Use the link below to set a new one (expires in {{.ExpiresIn}}):

{{.ResetURL}}

If you didn't request this, ignore this email. Your password won't change.

— The {{.AppName}} Team`

const PasswordChangedTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
  <title>Password changed</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f4f5;font-family:'Segoe UI',Arial,sans-serif;">
  <table width="100%" cellpadding="0" cellspacing="0" style="padding:40px 16px;">
    <tr>
      <td align="center">
        <table width="560" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:12px;overflow:hidden;">

          <!-- Header -->
          <tr>
            <td style="background:#4F46E5;padding:32px 40px;text-align:center;">
              <h1 style="margin:0;color:#ffffff;font-size:22px;font-weight:600;letter-spacing:-0.3px;">
                {{.AppName}}
              </h1>
            </td>
          </tr>

          <!-- Body -->
          <tr>
            <td style="padding:36px 40px;">

              <!-- Success Badge -->
              <table cellpadding="0" cellspacing="0" style="margin:0 0 20px;">
                <tr>
                  <td style="background:#ECFDF5;border:1px solid #6EE7B7;border-radius:20px;padding:5px 14px;">
                    <p style="margin:0;font-size:12px;font-weight:600;color:#065F46;">
                      &#10003; Password changed successfully
                    </p>
                  </td>
                </tr>
              </table>

              <h2 style="margin:0 0 6px;font-size:21px;font-weight:600;color:#111827;">
                Your password was updated
              </h2>
              <p style="margin:0 0 24px;font-size:15px;line-height:1.7;color:#4B5563;">
                Hi <strong style="color:#111827;">{{.Name}}</strong>,<br/>
                Your <strong style="color:#111827;">{{.AppName}}</strong> account password was changed
                successfully on <strong style="color:#111827;">{{.ChangedAt}}</strong>.
                You can now log in with your new password.
              </p>

              <!-- Danger Alert -->
              <table width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 28px;">
                <tr>
                  <td style="background:#FEF2F2;border-left:3px solid #EF4444;border-radius:0 6px 6px 0;padding:14px 16px;">
                    <p style="margin:0 0 4px;font-size:13px;font-weight:600;color:#991B1B;">
                      Wasn't you?
                    </p>
                    <p style="margin:0;font-size:13px;color:#B91C1C;line-height:1.6;">
                      If you did not make this change, your account may be compromised.
                      Contact our support team immediately and secure your account.
                    </p>
                  </td>
                </tr>
              </table>

              <!-- CTA Button -->
              <table cellpadding="0" cellspacing="0">
                <tr>
                  <td style="background:#4F46E5;border-radius:8px;">
                    <a href="{{.AppURL}}"
                       style="display:inline-block;padding:11px 26px;color:#ffffff;font-size:15px;font-weight:600;text-decoration:none;">
                      Log in to {{.AppName}} →
                    </a>
                  </td>
                </tr>
              </table>

            </td>
          </tr>

          <!-- Footer -->
          <tr>
            <td style="padding:16px 40px;border-top:1px solid #F3F4F6;text-align:center;">
              <p style="margin:0;font-size:11px;color:#9CA3AF;">
                © {{.AppName}} &middot; This is an automated security email.
              </p>
            </td>
          </tr>

        </table>
      </td>
    </tr>
  </table>
</body>
</html>`

const PasswordChangedText = `Password changed — {{.AppName}}

Hi {{.Name}},

Your {{.AppName}} password was successfully changed on {{.ChangedAt}}.

If you made this change, no action is needed.

Wasn't you? Contact our support team immediately and secure your account.

— The {{.AppName}} Team`

const LoginNotificationTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
  <title>New login notification</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f4f5;font-family:'Segoe UI',Arial,sans-serif;">
  <table width="100%" cellpadding="0" cellspacing="0" style="padding:40px 16px;">
    <tr>
      <td align="center">
        <table width="560" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:12px;overflow:hidden;">

          <!-- Header -->
          <tr>
            <td style="background:#4F46E5;padding:32px 40px;text-align:center;">
              <h1 style="margin:0;color:#ffffff;font-size:22px;font-weight:600;letter-spacing:-0.3px;">
                {{.AppName}}
              </h1>
            </td>
          </tr>

          <!-- Body -->
          <tr>
            <td style="padding:36px 40px;">

              <!-- Info Badge -->
              <table cellpadding="0" cellspacing="0" style="margin:0 0 20px;">
                <tr>
                  <td style="background:#EFF6FF;border:1px solid #BFDBFE;border-radius:20px;padding:5px 14px;">
                    <p style="margin:0;font-size:12px;font-weight:600;color:#1D4ED8;">
                      &#9432; New login detected
                    </p>
                  </td>
                </tr>
              </table>

              <h2 style="margin:0 0 6px;font-size:21px;font-weight:600;color:#111827;">
                New login to your account
              </h2>
              <p style="margin:0 0 24px;font-size:15px;line-height:1.7;color:#4B5563;">
                Hi <strong style="color:#111827;">{{.Name}}</strong>,<br/>
                A successful login was detected on your <strong style="color:#111827;">{{.AppName}}</strong>
                account. If this was you, no action is needed.
              </p>

              <!-- Login Details Card -->
              <table width="100%" cellpadding="0" cellspacing="0" style="background:#F9FAFB;border-radius:8px;margin:0 0 24px;">
                <tr>
                  <td style="padding:16px 20px;">
                    <p style="margin:0 0 12px;font-size:11px;font-weight:600;color:#6B7280;letter-spacing:0.6px;">
                      LOGIN DETAILS
                    </p>
                    <table width="100%" cellpadding="0" cellspacing="0" style="font-size:13px;">
                      <tr>
                        <td style="padding:5px 0;color:#6B7280;width:40%;">Location</td>
                        <td style="padding:5px 0;color:#111827;font-weight:500;">{{.Location}}</td>
                      </tr>
                      <tr>
                        <td style="padding:5px 0;color:#6B7280;">IP Address</td>
                        <td style="padding:5px 0;color:#111827;font-weight:500;">{{.IPAddress}}</td>
                      </tr>
                      <tr>
                        <td style="padding:5px 0;color:#6B7280;">Browser</td>
                        <td style="padding:5px 0;color:#111827;font-weight:500;">{{.Browser}}</td>
                      </tr>
                      <tr>
                        <td style="padding:5px 0;color:#6B7280;">Operating System</td>
                        <td style="padding:5px 0;color:#111827;font-weight:500;">{{.OS}}</td>
                      </tr>
                      <tr>
                        <td style="padding:5px 0;color:#6B7280;">Login Time</td>
                        <td style="padding:5px 0;color:#111827;font-weight:500;">{{.LoginTime}}</td>
                      </tr>
                    </table>
                  </td>
                </tr>
              </table>

              <!-- Danger Alert -->
              <table width="100%" cellpadding="0" cellspacing="0" style="margin:0 0 28px;">
                <tr>
                  <td style="background:#FEF2F2;border-left:3px solid #EF4444;border-radius:0 6px 6px 0;padding:14px 16px;">
                    <p style="margin:0 0 4px;font-size:13px;font-weight:600;color:#991B1B;">
                      Wasn't you?
                    </p>
                    <p style="margin:0;font-size:13px;color:#B91C1C;line-height:1.6;">
                      Change your password immediately and enable two-factor authentication
                      to secure your account.
                    </p>
                  </td>
                </tr>
              </table>

              <!-- CTA Button -->
              <table cellpadding="0" cellspacing="0">
                <tr>
                  <td style="background:#4F46E5;border-radius:8px;">
                    <a href="{{.AppURL}}"
                       style="display:inline-block;padding:11px 26px;color:#ffffff;font-size:15px;font-weight:600;text-decoration:none;">
                      Manage account →
                    </a>
                  </td>
                </tr>
              </table>

            </td>
          </tr>

          <!-- Footer -->
          <tr>
            <td style="padding:16px 40px;border-top:1px solid #F3F4F6;text-align:center;">
              <p style="margin:0;font-size:11px;color:#9CA3AF;">
                © {{.AppName}} &middot; This is an automated security notification.
                If you believe this is suspicious, contact support immediately.
              </p>
            </td>
          </tr>

        </table>
      </td>
    </tr>
  </table>
</body>
</html>`

const LoginNotificationText = `New login notification — {{.AppName}}

Hi {{.Name}},

A successful login was detected on your {{.AppName}} account.

Login Details:
  Location         : {{.Location}}
  IP Address       : {{.IPAddress}}
  Browser          : {{.Browser}}
  Operating System : {{.OS}}
  Login Time       : {{.LoginTime}}

If this was you, no action is needed.

Wasn't you? Change your password immediately and enable two-factor
authentication to secure your account.

Manage your account: {{.AppURL}}

— The {{.AppName}} Security Team`
