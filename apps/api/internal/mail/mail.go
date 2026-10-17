// Package mail sends the few transactional messages the API needs (member
// invitations) over SMTP. Without SMTP settings nothing is sent and callers
// fall back to handing the link over by other means.
package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Message is one email. HTML is optional; Text is always sent.
type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

// Sender delivers a Message.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// SMTPConfig is where and how to submit mail. Port 465 speaks TLS from the
// start; any other port starts plain and upgrades with STARTTLS when the
// server offers it.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	// Timeout bounds the whole submission.
	Timeout time.Duration
}

// Configured reports whether there is enough to send mail at all.
func (c SMTPConfig) Configured() bool {
	return c.Host != "" && c.From != ""
}

type SMTPSender struct {
	cfg SMTPConfig
}

// NewSMTPSender returns nil when the configuration is incomplete, so a nil
// Sender means "not configured".
func NewSMTPSender(cfg SMTPConfig) *SMTPSender {
	if !cfg.Configured() {
		return nil
	}
	if cfg.Port == 0 {
		cfg.Port = 587
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 15 * time.Second
	}
	return &SMTPSender{cfg: cfg}
}

func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	if s == nil {
		return errors.New("mail: not configured")
	}
	to, err := mail.ParseAddress(msg.To)
	if err != nil {
		return fmt.Errorf("mail: recipient: %w", err)
	}
	from, err := mail.ParseAddress(s.cfg.From)
	if err != nil {
		return fmt.Errorf("mail: sender: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("mail: connect %s: %w", addr, err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if s.cfg.Port == 465 {
		conn = tls.Client(conn, &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12})
	}
	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("mail: smtp handshake: %w", err)
	}
	defer func() { _ = client.Close() }()

	if s.cfg.Port != 465 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
				return fmt.Errorf("mail: starttls: %w", err)
			}
		}
	}
	if s.cfg.Username != "" {
		if ok, _ := client.Extension("AUTH"); !ok {
			return errors.New("mail: server offers no AUTH but credentials are set")
		}
		if err := client.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)); err != nil {
			return fmt.Errorf("mail: auth: %w", err)
		}
	}
	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("mail: from: %w", err)
	}
	if err := client.Rcpt(to.Address); err != nil {
		return fmt.Errorf("mail: rcpt: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("mail: data: %w", err)
	}
	if _, err := w.Write(Build(from, to, msg)); err != nil {
		return fmt.Errorf("mail: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mail: send: %w", err)
	}
	return client.Quit()
}

// Build renders the RFC 5322 message: text only, or multipart/alternative
// with a text and an HTML part. Header values are folded and encoded so a
// subject or name cannot inject headers.
func Build(from, to *mail.Address, msg Message) []byte {
	var b strings.Builder
	b.WriteString("From: " + from.String() + "\r\n")
	b.WriteString("To: " + to.String() + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", sanitizeHeader(msg.Subject)) + "\r\n")
	b.WriteString("Date: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	if msg.HTML == "" {
		b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		b.WriteString(crlf(msg.Text))
		return []byte(b.String())
	}
	boundary := "prism-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n")
	b.WriteString("--" + boundary + "\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(crlf(msg.Text) + "\r\n")
	b.WriteString("--" + boundary + "\r\nContent-Type: text/html; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(crlf(msg.HTML) + "\r\n")
	b.WriteString("--" + boundary + "--\r\n")
	return []byte(b.String())
}

func sanitizeHeader(s string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(s)
}

func crlf(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}
