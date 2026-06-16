// Package notify wraps outbound notifications (email today, SMS/push later)
// behind a small interface. The same Emailer is used by every service that
// wants to send a transactional message; the concrete implementation is
// chosen at boot based on config (SMTP for prod, log-only for dev).
package notify

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"go.uber.org/zap"
)

// Emailer is the minimal interface every concrete sender must satisfy.
// `text` is required, `html` is optional — if both are present the message
// uses multipart/alternative.
type Emailer interface {
	Send(to []string, subject, text, html string) error
	Enabled() bool // false → no-op fallback (dev mode without SMTP)
}

// ── Log-only (dev fallback) ───────────────────────────────────────────────

type LogEmailer struct{}

func (LogEmailer) Enabled() bool { return false }

func (LogEmailer) Send(to []string, subject, text, _ string) error {
	zap.L().Info("email (log-only)",
		zap.Strings("to", to),
		zap.String("subject", subject),
		zap.String("body_preview", preview(text, 200)))
	return nil
}

// ── SMTP ──────────────────────────────────────────────────────────────────

type SMTPEmailer struct {
	host     string
	port     string
	username string
	password string
	from     string // formatted "Name <addr>" if name provided, else bare addr
}

func NewSMTPEmailer(host, port, username, password, fromEmail, fromName string) *SMTPEmailer {
	from := fromEmail
	if fromName != "" {
		from = fmt.Sprintf("%s <%s>", fromName, fromEmail)
	}
	return &SMTPEmailer{
		host: host, port: port, username: username, password: password, from: from,
	}
}

func (s *SMTPEmailer) Enabled() bool { return s != nil && s.host != "" }

func (s *SMTPEmailer) Send(to []string, subject, text, html string) error {
	if !s.Enabled() {
		return fmt.Errorf("smtp emailer not configured")
	}
	if len(to) == 0 {
		return fmt.Errorf("no recipients")
	}

	body := buildMessage(s.from, to, subject, text, html)
	addr := s.host + ":" + s.port

	// Use STARTTLS via smtp.SendMail. If smtp server requires TLS-from-the-
	// start (port 465 / SMTPS), wrap a TLS conn manually.
	if s.port == "465" {
		return s.sendSMTPS(addr, to, body)
	}
	var auth smtp.Auth
	if s.username != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}
	return smtp.SendMail(addr, auth, extractAddr(s.from), to, body)
}

// sendSMTPS handles servers that require implicit TLS (port 465). We open
// the TLS connection ourselves then drive the SMTP exchange.
func (s *SMTPEmailer) sendSMTPS(addr string, to []string, body []byte) error {
	tlsConfig := &tls.Config{ServerName: s.host}
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return err
	}
	defer c.Close()

	if s.username != "" {
		if err := c.Auth(smtp.PlainAuth("", s.username, s.password, s.host)); err != nil {
			return err
		}
	}
	if err := c.Mail(extractAddr(s.from)); err != nil {
		return err
	}
	for _, r := range to {
		if err := c.Rcpt(r); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(body); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// ── helpers ───────────────────────────────────────────────────────────────

// buildMessage assembles a basic RFC 5322 message. If html is non-empty
// we emit multipart/alternative so clients can choose; otherwise plain
// text only.
func buildMessage(from string, to []string, subject, text, html string) []byte {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")

	if html == "" {
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
		b.WriteString(text)
		return []byte(b.String())
	}
	boundary := "ims-mp-boundary"
	b.WriteString("Content-Type: multipart/alternative; boundary=" + boundary + "\r\n\r\n")
	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString(text + "\r\n\r\n")
	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	b.WriteString(html + "\r\n\r\n")
	b.WriteString("--" + boundary + "--\r\n")
	return []byte(b.String())
}

// extractAddr pulls "addr" out of "Name <addr>" or returns input unchanged.
func extractAddr(formatted string) string {
	if i := strings.LastIndex(formatted, "<"); i >= 0 {
		if j := strings.LastIndex(formatted, ">"); j > i {
			return formatted[i+1 : j]
		}
	}
	return formatted
}

func preview(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// MustNew picks SMTP if SMTP_HOST is set, otherwise falls back to log-only.
func MustNew(host, port, user, pass, fromEmail, fromName string) Emailer {
	if host == "" {
		zap.L().Warn("SMTP not configured — emails will be logged only")
		return LogEmailer{}
	}
	if port == "" {
		port = "587"
	}
	zap.L().Info("SMTP emailer configured",
		zap.String("host", host), zap.String("port", port),
		zap.String("from", fromEmail))
	return NewSMTPEmailer(host, port, user, pass, fromEmail, fromName)
}
