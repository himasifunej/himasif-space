// Package config reads the server's configuration from the environment.
//
// Secrets (the database password, SMTP credentials) come only from the
// environment or a file outside the repository given with -env-file; they
// are never compiled in or committed (PRD NFR-09).
package config

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Config is the server's configuration.
type Config struct {
	RunMode            string
	AttachmentDir      string
	AttachmentMaxBytes int64
	WABridgeURL        string
	WABridgeToken      string
	Env                string // "development" or "production"
	HTTPAddr           string
	PublicURL          string // where people open Space, for links in emails
	DatabaseURL        string
	WebDir             string // the built web app to serve at /, "" for none
	// GuidesDir holds the in-app guide as Markdown files
	// (SPACE_GUIDES_DIR); "" serves the guide built in.
	GuidesDir string

	SessionTTL time.Duration

	Mail Mail

	// AdminEmail and AdminPassword seed the organization admin account
	// when it does not exist yet.
	AdminEmail    string
	AdminPassword string
}

// Mail configures outgoing email.
type Mail struct {
	Driver   string // "log" writes emails to the log; "smtp" sends them
	From     string
	Host     string
	Port     int
	Username string
	Password string
	// TLS is "starttls" (port 587), "tls" (port 465) or "none" (a local
	// catcher such as Mailpit).
	TLS string
	// PerMinute bounds the emails the server sends in one minute, all
	// addresses together (SPACE_MAIL_PER_MINUTE, default 20). Amazon SES
	// allows 1 per second in its sandbox and 14 after it.
	PerMinute int
	// SNSTopicARN is the Amazon SNS topic SES publishes bounces and
	// complaints to (SPACE_SES_SNS_TOPIC_ARN). Empty turns the endpoint
	// /api/mail/ses-events off.
	SNSTopicARN string
}

// Development reports whether the server runs for development.
func (c Config) Development() bool { return c.Env == "development" }

// Load reads the configuration from the environment.
func Load() (Config, error) {
	c := Config{
		RunMode:       get("SPACE_RUN_MODE", RunLive),
		AttachmentDir: get("SPACE_ATTACHMENT_DIR", "space-data/attachments"),
		WABridgeURL:   os.Getenv("SPACE_WA_BRIDGE_URL"),
		WABridgeToken: os.Getenv("SPACE_WA_BRIDGE_TOKEN"),
		Env:           get("SPACE_ENV", "development"),
		HTTPAddr:      get("SPACE_HTTP_ADDR", "127.0.0.1:8787"),
		PublicURL:     strings.TrimRight(get("SPACE_PUBLIC_URL", "http://localhost:8787"), "/"),
		DatabaseURL:   os.Getenv("SPACE_DATABASE_URL"),
		WebDir:        os.Getenv("SPACE_WEB_DIR"),
		GuidesDir:     os.Getenv("SPACE_GUIDES_DIR"),
		AdminEmail:    os.Getenv("SPACE_ADMIN_EMAIL"),
		AdminPassword: os.Getenv("SPACE_ADMIN_PASSWORD"),
		Mail: Mail{
			Driver:   get("SPACE_MAIL_DRIVER", "log"),
			From:     get("SPACE_MAIL_FROM", "Space HIMASIF <noreply@himasif.id>"),
			Host:     os.Getenv("SPACE_SMTP_HOST"),
			Username: os.Getenv("SPACE_SMTP_USERNAME"),
			Password: os.Getenv("SPACE_SMTP_PASSWORD"),
			TLS:      get("SPACE_SMTP_TLS", "starttls"),

			SNSTopicARN: os.Getenv("SPACE_SES_SNS_TOPIC_ARN"),
		},
	}
	var errs []error
	switch c.RunMode {
	case RunLive:
	case RunIsolatedRestore, RunMaintenance:
		c.Mail.Driver = "disabled"
		c.WABridgeURL = ""
		c.WABridgeToken = ""
	default:
		errs = append(errs, errors.New("SPACE_RUN_MODE must be live, isolated_restore or maintenance"))
	}
	if c.RunMode == RunIsolatedRestore {
		public, err := url.Parse(c.PublicURL)
		if err != nil || !runtimeLoopback(public.Hostname()) {
			errs = append(errs, errors.New("isolated restore requires a loopback SPACE_PUBLIC_URL; use localhost forwarding for previews"))
		}
		host, _, err := net.SplitHostPort(c.HTTPAddr)
		if err != nil || !runtimeLoopback(host) {
			errs = append(errs, errors.New("isolated restore must bind SPACE_HTTP_ADDR to loopback"))
		}
	}

	maxAttachment, parseAttachmentErr := strconv.ParseInt(get("SPACE_ATTACHMENT_MAX_BYTES", "20971520"), 10, 64)
	if parseAttachmentErr != nil || maxAttachment < 1 || maxAttachment > 100<<20 {
		errs = append(errs, errors.New("SPACE_ATTACHMENT_MAX_BYTES must be between 1 and 104857600"))
	}
	c.AttachmentMaxBytes = maxAttachment
	attachmentRoot, attachmentErr := filepath.Abs(c.AttachmentDir)
	if attachmentErr != nil {
		errs = append(errs, attachmentErr)
	} else {
		c.AttachmentDir = attachmentRoot
	}
	if c.WebDir != "" && attachmentErr == nil {
		webRoot, err := filepath.Abs(c.WebDir)
		if err != nil {
			errs = append(errs, err)
		} else if rel, err := filepath.Rel(webRoot, attachmentRoot); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			errs = append(errs, errors.New("SPACE_ATTACHMENT_DIR must be outside SPACE_WEB_DIR"))
		}
	}

	if c.Env != "development" && c.Env != "production" {
		errs = append(errs, fmt.Errorf("SPACE_ENV must be development or production, not %q", c.Env))
	}
	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("SPACE_DATABASE_URL is required"))
	}
	ttl, err := time.ParseDuration(get("SPACE_SESSION_TTL", "720h"))
	if err != nil || ttl <= 0 {
		errs = append(errs, fmt.Errorf("SPACE_SESSION_TTL: %q is not a positive duration", os.Getenv("SPACE_SESSION_TTL")))
	}
	c.SessionTTL = ttl
	port, err := strconv.Atoi(get("SPACE_SMTP_PORT", "587"))
	if err != nil {
		errs = append(errs, fmt.Errorf("SPACE_SMTP_PORT: %w", err))
	}
	c.Mail.Port = port
	perMinute, err := strconv.Atoi(get("SPACE_MAIL_PER_MINUTE", "20"))
	if err != nil || perMinute < 1 || perMinute > 600 {
		errs = append(errs, errors.New("SPACE_MAIL_PER_MINUTE must be between 1 and 600"))
	}
	c.Mail.PerMinute = perMinute
	if c.Mail.SNSTopicARN != "" && !snsTopicARN.MatchString(c.Mail.SNSTopicARN) {
		errs = append(errs, fmt.Errorf("SPACE_SES_SNS_TOPIC_ARN is not an SNS topic ARN: %q", c.Mail.SNSTopicARN))
	}
	switch c.Mail.Driver {
	case "log", "disabled":
	case "smtp":
		if c.Mail.Host == "" {
			errs = append(errs, errors.New("SPACE_SMTP_HOST is required with SPACE_MAIL_DRIVER=smtp"))
		}
		// The verified sending identity of HIMASIF (Amazon SES) is the
		// domain himasif.id; production mail comes from it.
		if !c.Development() {
			if a, err := mail.ParseAddress(c.Mail.From); err != nil || !strings.HasSuffix(strings.ToLower(a.Address), "@himasif.id") {
				errs = append(errs, fmt.Errorf("SPACE_MAIL_FROM must be an address @himasif.id in production, not %q", c.Mail.From))
			}
		}
		switch c.Mail.TLS {
		case "starttls", "tls":
		case "none":
			if !c.Development() {
				errs = append(errs, errors.New("SPACE_SMTP_TLS=none is only allowed in development"))
			}
		default:
			errs = append(errs, fmt.Errorf("SPACE_SMTP_TLS must be starttls, tls or none, not %q", c.Mail.TLS))
		}
	default:
		errs = append(errs, fmt.Errorf("SPACE_MAIL_DRIVER must be disabled, log or smtp, not %q", c.Mail.Driver))
	}
	return c, errors.Join(errs...)
}

var snsTopicARN = regexp.MustCompile(`^arn:aws:sns:[a-z0-9-]+:[0-9]{12}:[A-Za-z0-9_-]{1,256}$`)

func get(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// LoadEnvFile sets the variables of a file of KEY=VALUE lines that are not
// set already. Blank lines and lines starting with # are skipped; a value
// may be wrapped in single or double quotes. Values are taken literally:
// nothing is expanded, so secrets may hold any character.
func LoadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for n := 1; s.Scan(); n++ {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(strings.TrimPrefix(key, "export "))
		if !ok || key == "" {
			return fmt.Errorf("%s:%d: expected KEY=VALUE", path, n)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		if _, set := os.LookupEnv(key); !set {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}
	return s.Err()
}

func runtimeLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
