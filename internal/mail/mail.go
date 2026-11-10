// Package mail sends Space's emails: invitations, verification and account
// recovery (Sesi 1B). The provider sits behind Mailer, so development uses
// a local catcher (Mailpit) or the log, and production any SMTP provider,
// such as Zoho Mail or Amazon SES, without changing the callers.
package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/config"
)

// Message is one email: plain text, and optionally the same content as
// HTML (sent as multipart/alternative).
type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

// RejectedError is a permanent refusal by the mail server (an SMTP 5xx
// reply), such as an address that does not exist or, in the Amazon SES
// sandbox, one that is not verified. Sending again does not help.
type RejectedError struct {
	Code int
	Msg  string
}

func (e *RejectedError) Error() string { return fmt.Sprintf("mail: rejected (%d): %s", e.Code, e.Msg) }

// rejected turns a permanent SMTP reply into a RejectedError.
func rejected(err error) error {
	var tp *textproto.Error
	if errors.As(err, &tp) && tp.Code >= 500 && tp.Code < 600 {
		return &RejectedError{Code: tp.Code, Msg: tp.Msg}
	}
	return err
}

// Mailer sends messages.
type Mailer interface {
	Send(ctx context.Context, m Message) error
}

// New returns the mailer the configuration chooses.
func New(c config.Mail, log *slog.Logger) (Mailer, error) {
	from, err := mail.ParseAddress(c.From)
	if err != nil {
		return nil, fmt.Errorf("mail: SPACE_MAIL_FROM: %w", err)
	}
	switch c.Driver {
	case "disabled":
		return Disabled(), nil
	case "log":
		return logMailer{log: log}, nil
	case "smtp":
		return Limited(&smtpMailer{cfg: c, from: from}, c.PerMinute), nil
	}
	return nil, fmt.Errorf("mail: unknown driver %q", c.Driver)
}

// logMailer writes emails to the log instead of sending them. Only the
// recipient and subject are logged, not the body, which may hold links that
// act as credentials.
type logMailer struct{ log *slog.Logger }

func (l logMailer) Send(_ context.Context, m Message) error {
	if err := m.validate(); err != nil {
		return err
	}
	l.log.Info("email not sent (SPACE_MAIL_DRIVER=log)", "to", m.To, "subject", m.Subject)
	return nil
}

type smtpMailer struct {
	cfg  config.Mail
	from *mail.Address
}

func (s *smtpMailer) Send(ctx context.Context, m Message) error {
	if err := m.validate(); err != nil {
		return err
	}
	to, _ := mail.ParseAddress(m.To)
	body, err := s.compose(to, m)
	if err != nil {
		return err
	}
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	var conn net.Conn
	if s.cfg.TLS == "tls" {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("mail: connect %s: %w", addr, err)
	}
	if dl, ok := ctx.Deadline(); ok {
		conn.SetDeadline(dl)
	} else {
		conn.SetDeadline(time.Now().Add(time.Minute))
	}
	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("mail: %w", err)
	}
	defer c.Close()
	if s.cfg.TLS == "starttls" {
		if err := c.StartTLS(&tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("mail: STARTTLS: %w", err)
		}
	}
	if s.cfg.Username != "" {
		// PlainAuth refuses to send credentials without TLS unless the
		// server is localhost, which is what a development catcher is.
		if err := c.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)); err != nil {
			return fmt.Errorf("mail: auth: %w", err)
		}
	}
	if err := c.Mail(s.from.Address); err != nil {
		return rejected(fmt.Errorf("mail: MAIL FROM: %w", err))
	}
	if err := c.Rcpt(to.Address); err != nil {
		return rejected(fmt.Errorf("mail: RCPT TO: %w", err))
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("mail: DATA: %w", err)
	}
	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("mail: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return rejected(fmt.Errorf("mail: send: %w", err))
	}
	return c.Quit()
}

func (s *smtpMailer) compose(to *mail.Address, m Message) ([]byte, error) {
	var b bytes.Buffer
	domain := s.from.Address[strings.LastIndexByte(s.from.Address, '@')+1:]
	idBytes := make([]byte, 12)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, err
	}
	h := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, v) }
	h("From", s.from.String())
	h("To", to.String())
	h("Subject", mime.QEncoding.Encode("utf-8", m.Subject))
	h("Date", time.Now().Format(time.RFC1123Z))
	h("Message-ID", fmt.Sprintf("<%s@%s>", hex.EncodeToString(idBytes), domain))
	h("MIME-Version", "1.0")
	if m.HTML == "" {
		h("Content-Type", `text/plain; charset="utf-8"`)
		h("Content-Transfer-Encoding", "quoted-printable")
		b.WriteString("\r\n")
		if err := writeQP(&b, m.Text); err != nil {
			return nil, err
		}
		return b.Bytes(), nil
	}
	boundary := "space-" + hex.EncodeToString(idBytes)
	h("Content-Type", `multipart/alternative; boundary="`+boundary+`"`)
	b.WriteString("\r\n")
	for _, part := range []struct{ kind, body string }{{"text/plain", m.Text}, {"text/html", m.HTML}} {
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString(`Content-Type: ` + part.kind + `; charset="utf-8"` + "\r\n")
		b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		if err := writeQP(&b, part.body); err != nil {
			return nil, err
		}
		b.WriteString("\r\n")
	}
	b.WriteString("--" + boundary + "--\r\n")
	return b.Bytes(), nil
}

func writeQP(b *bytes.Buffer, s string) error {
	qp := quotedprintable.NewWriter(b)
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n")
	if _, err := qp.Write([]byte(s)); err != nil {
		return err
	}
	return qp.Close()
}

func (m Message) validate() error {
	if _, err := mail.ParseAddress(m.To); err != nil {
		return fmt.Errorf("mail: recipient: %w", err)
	}
	if strings.ContainsAny(m.Subject, "\r\n") {
		return errors.New("mail: subject must be one line")
	}
	if m.Subject == "" || m.Text == "" {
		return errors.New("mail: subject and text are required")
	}
	return nil
}
