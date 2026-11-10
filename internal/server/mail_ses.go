package server

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
)

// Amazon SES reports bounces and complaints to an SNS topic, which posts
// them here (SPACE_SES_SNS_TOPIC_ARN). A pending invitation whose email
// bounced for good, or was marked as spam, shows "Tidak terkirim" until it
// is sent again: the kadiv then checks the NIM or address. Only messages
// signed by SNS for the configured topic are accepted.

// snsHost is the host of SNS certificate and subscription URLs.
var snsHost = regexp.MustCompile(`^sns\.[a-z0-9-]+\.amazonaws\.com$`)

type snsMessage struct {
	Type             string
	MessageID        string `json:"MessageId"`
	Token            string
	TopicArn         string
	Subject          string
	Message          string
	SubscribeURL     string
	Timestamp        string
	SignatureVersion string
	Signature        string
	SigningCertURL   string
}

// snsCerts caches SNS signing certificates by URL; fetch is replaceable in
// tests, as is confirm, which opens a subscription's confirmation URL.
type snsClient struct {
	mu      sync.Mutex
	certs   map[string]*x509.Certificate
	fetch   func(ctx context.Context, url string) ([]byte, error)
	confirm func(ctx context.Context, url string) error
}

func newSNSClient() *snsClient {
	c := &http.Client{Timeout: 10 * time.Second}
	get := func(ctx context.Context, u string) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		res, err := c.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("sns: GET %s: %s", u, res.Status)
		}
		return io.ReadAll(io.LimitReader(res.Body, 64<<10))
	}
	return &snsClient{certs: map[string]*x509.Certificate{}, fetch: get,
		confirm: func(ctx context.Context, u string) error { _, err := get(ctx, u); return err }}
}

func snsURL(raw string, pem bool) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || !snsHost.MatchString(u.Host) || (pem && !strings.HasSuffix(u.Path, ".pem")) {
		return errors.New("sns: URL is not an SNS endpoint")
	}
	return nil
}

func (c *snsClient) cert(ctx context.Context, raw string) (*x509.Certificate, error) {
	if err := snsURL(raw, true); err != nil {
		return nil, err
	}
	c.mu.Lock()
	cert := c.certs[raw]
	c.mu.Unlock()
	if cert != nil {
		return cert, nil
	}
	body, err := c.fetch(ctx, raw)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(body)
	if block == nil {
		return nil, errors.New("sns: certificate is not PEM")
	}
	cert, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	if len(c.certs) > 20 {
		c.certs = map[string]*x509.Certificate{}
	}
	c.certs[raw] = cert
	c.mu.Unlock()
	return cert, nil
}

// signedText is the text SNS signs, per message type.
func (m snsMessage) signedText() string {
	var b strings.Builder
	add := func(k, v string) { b.WriteString(k + "\n" + v + "\n") }
	add("Message", m.Message)
	add("MessageId", m.MessageID)
	if m.Type == "Notification" {
		if m.Subject != "" {
			add("Subject", m.Subject)
		}
	} else {
		add("SubscribeURL", m.SubscribeURL)
	}
	add("Timestamp", m.Timestamp)
	if m.Type != "Notification" {
		add("Token", m.Token)
	}
	add("TopicArn", m.TopicArn)
	add("Type", m.Type)
	return b.String()
}

func (c *snsClient) verify(ctx context.Context, m snsMessage) error {
	sig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil {
		return err
	}
	cert, err := c.cert(ctx, m.SigningCertURL)
	if err != nil {
		return err
	}
	key, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return errors.New("sns: certificate key is not RSA")
	}
	text := []byte(m.signedText())
	switch m.SignatureVersion {
	case "1":
		sum := sha1.Sum(text)
		return rsa.VerifyPKCS1v15(key, crypto.SHA1, sum[:], sig)
	case "2":
		sum := sha256.Sum256(text)
		return rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig)
	}
	return errors.New("sns: unknown signature version")
}

// sesNotification is the part of an SES bounce or complaint that matters
// here, in both the notification and the event publishing formats.
type sesNotification struct {
	NotificationType string `json:"notificationType"`
	EventType        string `json:"eventType"`
	Bounce           *struct {
		BounceType        string `json:"bounceType"`
		BounceSubType     string `json:"bounceSubType"`
		BouncedRecipients []struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"bouncedRecipients"`
	} `json:"bounce"`
	Complaint *struct {
		ComplainedRecipients []struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"complainedRecipients"`
	} `json:"complaint"`
}

// failures returns the addresses that should not get mail again, with why.
func (n sesNotification) failures() map[string]string {
	out := map[string]string{}
	kind := n.NotificationType
	if kind == "" {
		kind = n.EventType
	}
	switch {
	case kind == "Bounce" && n.Bounce != nil && n.Bounce.BounceType == "Permanent":
		reason := "Email tidak sampai: alamat tidak ada atau menolak email."
		if n.Bounce.BounceSubType == "Suppressed" || n.Bounce.BounceSubType == "OnAccountSuppressionList" {
			reason = "Email tidak dikirim: alamat ini pernah gagal menerima email."
		}
		for _, r := range n.Bounce.BouncedRecipients {
			out[store.NormalizeEmail(r.EmailAddress)] = reason
		}
	case kind == "Complaint" && n.Complaint != nil:
		for _, r := range n.Complaint.ComplainedRecipients {
			out[store.NormalizeEmail(r.EmailAddress)] = "Penerima menandai email sebagai spam."
		}
	}
	return out
}

// sesEvents receives SNS posts from Amazon SES.
func (s *Server) sesEvents(w http.ResponseWriter, r *http.Request) {
	topic := s.cfg.Mail.SNSTopicARN
	if topic == "" {
		http.NotFound(w, r)
		return
	}
	var m snsMessage
	if err := json.NewDecoder(io.LimitReader(r.Body, 256<<10)).Decode(&m); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if m.TopicArn != topic {
		http.Error(w, "unknown topic", http.StatusForbidden)
		return
	}
	ctx := r.Context()
	if err := s.sns.verify(ctx, m); err != nil {
		s.log.Warn("ses: SNS signature rejected", "type", m.Type, "err", err)
		http.Error(w, "invalid signature", http.StatusForbidden)
		return
	}
	switch m.Type {
	case "SubscriptionConfirmation":
		if err := snsURL(m.SubscribeURL, false); err != nil {
			http.Error(w, "bad subscribe URL", http.StatusBadRequest)
			return
		}
		if err := s.sns.confirm(ctx, m.SubscribeURL); err != nil {
			s.log.Error("ses: confirm SNS subscription", "err", err)
			http.Error(w, "confirmation failed", http.StatusBadGateway)
			return
		}
		s.log.Info("ses: SNS subscription confirmed", "topic", topic)
	case "Notification":
		var n sesNotification
		if err := json.Unmarshal([]byte(m.Message), &n); err != nil {
			http.Error(w, "bad notification", http.StatusBadRequest)
			return
		}
		if err := s.markBounces(r, n.failures()); err != nil {
			s.log.Error("ses: record bounce", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

// markBounces marks the pending invitations of each address.
func (s *Server) markBounces(r *http.Request, failed map[string]string) error {
	if len(failed) == 0 {
		return nil
	}
	return s.store.Tx(r.Context(), func(q store.Querier) error {
		now := s.now()
		for email, reason := range failed {
			rows, err := q.Query(r.Context(), `update invitations set bounced_at = $2, bounce_reason = $3
				where email = $1 and accepted_at is null and revoked_at is null and bounced_at is null
				returning id, period_id, division_id`, email, now, reason)
			if err != nil {
				return err
			}
			type hit struct{ id, period, division id.ID }
			var hits []hit
			for rows.Next() {
				var h hit
				if err := rows.Scan(&h.id, &h.period, &h.division); err != nil {
					rows.Close()
					return err
				}
				hits = append(hits, h)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
			for _, h := range hits {
				s.log.Info("ses: invitation bounced", "invitation", h.id)
				if err := s.record(r, q, nil, store.AuditEvent{ActorKind: "system", Source: "system", Action: "invitation.bounce",
					PeriodID: &h.period, ScopeType: "division", ScopeID: &h.division, TargetType: "invitation", TargetID: &h.id,
					After: map[string]any{"reason": reason}}); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
