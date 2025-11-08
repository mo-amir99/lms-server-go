package email

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"time"
)

// Client handles email sending operations.
type Client struct {
	host     string
	port     string
	username string
	password string
	from     string
	secure   bool
}

// NewClient creates a new email client.
func NewClient(host, port, username, password, from string, secure bool) *Client {
	return &Client{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
		secure:   secure,
	}
}

// EmailOptions represents the options for sending an email.
type EmailOptions struct {
	To      string
	Subject string
	HTML    string
	Text    string
}

// SendEmail sends an email with HTML content.
func (c *Client) SendEmail(opts EmailOptions) error {
	// Wrap HTML in template
	wrappedHTML := c.wrapHTMLTemplate(opts.HTML)

	// Build message
	message := c.buildMessage(opts.To, opts.Subject, wrappedHTML, opts.Text)

	// Connect and send
	auth := smtp.PlainAuth("", c.username, c.password, c.host)
	addr := fmt.Sprintf("%s:%s", c.host, c.port)

	err := smtp.SendMail(addr, auth, c.from, []string{opts.To}, []byte(message))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// wrapHTMLTemplate wraps the HTML content in a nice template.
func (c *Client) wrapHTMLTemplate(content string) string {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin: 0; padding: 0; font-family: Arial, sans-serif; background: #f9f9f9;">
    <div style="padding: 32px;">
        <div style="max-width: 600px; margin: auto; background: #fff; border-radius: 8px; box-shadow: 0 2px 8px #eee; padding: 32px;">
            <div style="text-align: center; margin-bottom: 24px;">
                <h2 style="color: #2a7ae2; margin: 0;">Elites Academy Notification</h2>
            </div>
            <div style="font-size: 16px; color: #333;">
                {{.Content}}
            </div>
            <div style="margin-top: 32px; text-align: center; color: #aaa; font-size: 12px;">
                &copy; {{.Year}} Elites Academy. All rights reserved.
            </div>
        </div>
    </div>
</body>
</html>
`

	t := template.Must(template.New("email").Parse(tmpl))
	var buf bytes.Buffer
	data := map[string]interface{}{
		"Content": template.HTML(content),
		"Year":    time.Now().Year(),
	}

	if err := t.Execute(&buf, data); err != nil {
		// Fallback to plain content if template fails
		return content
	}

	return buf.String()
}

// buildMessage constructs the email message with headers.
func (c *Client) buildMessage(to, subject, html, text string) string {
	from := c.from
	if from == "" {
		from = "noreply@example.com"
	}

	msg := fmt.Sprintf("From: %s\r\n", from)
	msg += fmt.Sprintf("To: %s\r\n", to)
	msg += fmt.Sprintf("Subject: %s\r\n", subject)
	msg += "MIME-Version: 1.0\r\n"
	msg += "Content-Type: multipart/alternative; boundary=\"boundary42\"\r\n"
	msg += "\r\n"

	// Plain text part
	if text != "" {
		msg += "--boundary42\r\n"
		msg += "Content-Type: text/plain; charset=\"UTF-8\"\r\n"
		msg += "\r\n"
		msg += text + "\r\n"
	}

	// HTML part
	msg += "--boundary42\r\n"
	msg += "Content-Type: text/html; charset=\"UTF-8\"\r\n"
	msg += "\r\n"
	msg += html + "\r\n"
	msg += "--boundary42--\r\n"

	return msg
}

// SendPasswordReset sends a password reset email with a token.
func (c *Client) SendPasswordReset(to, resetToken, resetURL string) error {
	html := fmt.Sprintf(`
		<p>Hello,</p>
		<p>You requested to reset your password. Click the link below to reset your password:</p>
		<p style="text-align: center; margin: 24px 0;">
			<a href="%s?token=%s" style="background: #2a7ae2; color: #fff; padding: 12px 24px; text-decoration: none; border-radius: 4px; display: inline-block;">
				Reset Password
			</a>
		</p>
		<p>If you did not request this, please ignore this email.</p>
		<p>This link will expire in 1 hour.</p>
	`, resetURL, resetToken)

	return c.SendEmail(EmailOptions{
		To:      to,
		Subject: "Password Reset Request",
		HTML:    html,
		Text:    fmt.Sprintf("Reset your password: %s?token=%s", resetURL, resetToken),
	})
}

// SendEmailVerification sends an email verification link.
func (c *Client) SendEmailVerification(to, verificationToken, verificationURL string) error {
	html := fmt.Sprintf(`
		<p>Hello,</p>
		<p>Welcome! Please verify your email address by clicking the link below:</p>
		<p style="text-align: center; margin: 24px 0;">
			<a href="%s?token=%s" style="background: #2a7ae2; color: #fff; padding: 12px 24px; text-decoration: none; border-radius: 4px; display: inline-block;">
				Verify Email
			</a>
		</p>
		<p>If you did not create this account, please ignore this email.</p>
	`, verificationURL, verificationToken)

	return c.SendEmail(EmailOptions{
		To:      to,
		Subject: "Verify Your Email Address",
		HTML:    html,
		Text:    fmt.Sprintf("Verify your email: %s?token=%s", verificationURL, verificationToken),
	})
}

// SendWelcome sends a welcome email to a new user.
func (c *Client) SendWelcome(to, userName string) error {
	html := fmt.Sprintf(`
		<p>Hello %s,</p>
		<p>Welcome to Elites Academy! We're excited to have you on board.</p>
		<p>Get started by adding your first course.</p>
		<p>If you have any questions, feel free to reach out to our support team.</p>
		<p>Happy teaching!</p>
	`, userName)

	return c.SendEmail(EmailOptions{
		To:      to,
		Subject: "Welcome to Elites Academy!",
		HTML:    html,
		Text:    fmt.Sprintf("Hello %s, Welcome to Elites Academy!", userName),
	})
}

// SendNotification sends a general notification email.
func (c *Client) SendNotification(to, title, message string) error {
	html := fmt.Sprintf(`
		<h3 style="color: #2a7ae2;">%s</h3>
		<p>%s</p>
	`, title, message)

	return c.SendEmail(EmailOptions{
		To:      to,
		Subject: title,
		HTML:    html,
		Text:    message,
	})
}
