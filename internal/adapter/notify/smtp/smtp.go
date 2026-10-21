// Package smtp sends e-mail through the server's SMTP settings
// (DENAR_SMTP_*): notifications to e-mail channels and invitation links
// (FR-540, FR-570).
package smtp

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"git.eiac.dev/tools/denar/internal/usecase"
)

// TLS modes.
const (
	StartTLS = "starttls" // plain connection upgraded with STARTTLS (port 587)
	TLS      = "tls"      // TLS from the start (port 465)
	None     = "none"     // no TLS; only for a relay on a trusted network
)

// Config is how to reach the SMTP server.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string // "Denar <denar@example.com>" or an address
	TLS      string // StartTLS (default), TLS or None
	// TLSConfig replaces the default (server name = Host); for tests.
	TLSConfig *tls.Config
	// Timeout bounds one message; 0 means 10 s.
	Timeout time.Duration
	// Now gives the Date header; nil means time.Now.
	Now func() time.Time
}

var (
	_ usecase.Notifier = (*Mailer)(nil)
	_ usecase.Mailer   = (*Mailer)(nil)
)

// Mailer sends plain-text UTF-8 e-mail.
type Mailer struct {
	cfg  Config
	from *mail.Address
}

// New checks the configuration.
func New(cfg Config) (*Mailer, error) {
	if cfg.Host == "" {
		return nil, errors.New("smtp: host is required")
	}
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("smtp: from address %q is not an e-mail address", cfg.From)
	}
	switch cfg.TLS {
	case "":
		cfg.TLS = StartTLS
	case StartTLS, TLS, None:
	default:
		return nil, fmt.Errorf("smtp: TLS mode must be starttls, tls or none")
	}
	if cfg.Port == 0 {
		cfg.Port = 587
		if cfg.TLS == TLS {
			cfg.Port = 465
		}
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Mailer{cfg: cfg, from: from}, nil
}

// Send mails a notification to the channel's address.
func (m *Mailer) Send(ctx context.Context, target usecase.ChannelTarget, msg usecase.OutgoingMessage) error {
	body := msg.Body
	if msg.URL != "" {
		body += "\n\n" + msg.URL
	}
	if msg.HouseholdName != "" {
		body += "\n\nGospodarstwo: " + msg.HouseholdName
	}
	body += "\n\n-- \nDenar · ustawienia powiadomień zmienisz w aplikacji.\n"
	return m.SendMail(ctx, target.Config["address"], msg.Title, body)
}

// SendMail sends one message.
func (m *Mailer) SendMail(ctx context.Context, to, subject, body string) error {
	rcpt, err := mail.ParseAddress(to)
	if err != nil {
		return errors.New("niepoprawny adres odbiorcy")
	}
	raw, err := m.message(rcpt, subject, body)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
	defer cancel()
	conn, err := m.dial(ctx)
	if err != nil {
		return fmt.Errorf("SMTP: połączenie nieudane: %w", unwrapNet(err))
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("SMTP: %w", err)
	}
	defer c.Close()
	if m.cfg.TLS == StartTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errors.New("SMTP: serwer nie obsługuje STARTTLS")
		}
		if err := c.StartTLS(m.tlsConfig()); err != nil {
			return fmt.Errorf("SMTP: STARTTLS: %w", err)
		}
	}
	if m.cfg.Username != "" {
		if ok, _ := c.Extension("AUTH"); !ok {
			return errors.New("SMTP: serwer nie obsługuje AUTH")
		}
		if err := c.Auth(smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)); err != nil {
			return fmt.Errorf("SMTP: logowanie nieudane: %w", err)
		}
	}
	if err := c.Mail(m.from.Address); err != nil {
		return fmt.Errorf("SMTP: MAIL FROM: %w", err)
	}
	if err := c.Rcpt(rcpt.Address); err != nil {
		return fmt.Errorf("SMTP: RCPT TO: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("SMTP: DATA: %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		return fmt.Errorf("SMTP: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("SMTP: %w", err)
	}
	return c.Quit()
}

func (m *Mailer) tlsConfig() *tls.Config {
	if m.cfg.TLSConfig != nil {
		return m.cfg.TLSConfig.Clone()
	}
	return &tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12}
}

func (m *Mailer) dial(ctx context.Context) (net.Conn, error) {
	addr := net.JoinHostPort(m.cfg.Host, strconv.Itoa(m.cfg.Port))
	if m.cfg.TLS == TLS {
		d := tls.Dialer{Config: m.tlsConfig()}
		return d.DialContext(ctx, "tcp", addr)
	}
	var d net.Dialer
	return d.DialContext(ctx, "tcp", addr)
}

func unwrapNet(err error) error {
	var op *net.OpError
	if errors.As(err, &op) && op.Err != nil {
		return op.Err
	}
	return err
}

// message builds the RFC 5322 message: UTF-8 text, quoted-printable.
func (m *Mailer) message(to *mail.Address, subject, body string) ([]byte, error) {
	if strings.ContainsAny(subject, "\r\n") {
		subject = strings.Join(strings.Fields(subject), " ")
	}
	id := make([]byte, 12)
	if _, err := rand.Read(id); err != nil {
		return nil, err
	}
	domain := m.from.Address[strings.LastIndex(m.from.Address, "@")+1:]
	var b bytes.Buffer
	header := func(k, v string) { b.WriteString(k + ": " + v + "\r\n") }
	header("From", m.from.String())
	header("To", to.String())
	header("Subject", mime.QEncoding.Encode("utf-8", subject))
	header("Date", m.cfg.Now().UTC().Format(time.RFC1123Z))
	header("Message-ID", "<"+hex.EncodeToString(id)+"@"+domain+">")
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=utf-8")
	header("Content-Transfer-Encoding", "quoted-printable")
	header("Auto-Submitted", "auto-generated")
	b.WriteString("\r\n")
	qp := quotedprintable.NewWriter(&b)
	if _, err := qp.Write([]byte(strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\n", "\r\n"))); err != nil {
		return nil, err
	}
	if err := qp.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
