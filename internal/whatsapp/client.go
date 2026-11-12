// Package whatsapp is the authenticated, bounded protocol to Space's separate
// Baileys service. It never exposes its bearer or session credentials to apps.
package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/himasifunej/himasif-space/internal/id"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	disabledReason string
	BaseURL, Token string
	HTTP           *http.Client
}
type Error struct {
	Status    int
	Message   string
	Attempted *bool
}

func (e *Error) Error() string { return e.Message }
func New(base, token string) *Client {
	return &Client{BaseURL: strings.TrimRight(base, "/"), Token: token, HTTP: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("bridge redirect disabled") }}}
}
func Disabled(reason string) *Client { return &Client{disabledReason: reason} }
func (c *Client) Available() bool {
	return c != nil && c.disabledReason == "" && c.BaseURL != "" && len(c.Token) >= 32
}
func (c *Client) call(ctx context.Context, method, path string, body, out any) error {
	if c != nil && c.disabledReason != "" {
		attempted := false
		return &Error{Status: 503, Message: c.disabledReason, Attempted: &attempted}
	}
	if !c.Available() {
		return &Error{Status: 503, Message: "Bridge WhatsApp belum dikonfigurasi."}
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return &Error{Status: 503, Message: "Alamat bridge belum valid."}
	}
	var data []byte
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bytes.NewReader(data))
	if err != nil {
		return errors.New("permintaan bridge tidak valid")
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return &Error{Status: 503, Message: "Bridge tidak merespons; status tindakan belum dapat dipastikan."}
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
	if err != nil || len(payload) > 8<<20 {
		return errors.New("respons bridge terputus atau terlalu besar")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var failure struct {
			Error     string `json:"error"`
			Attempted *bool  `json:"attempted"`
		}
		_ = json.Unmarshal(payload, &failure)
		message := "Bridge menolak operasi."
		if failure.Error != "" {
			message = failure.Error
		}
		return &Error{Status: resp.StatusCode, Message: message, Attempted: failure.Attempted}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("format respons bridge tidak valid")
	}
	return nil
}

type State struct {
	ID          id.ID      `json:"id"`
	Status      string     `json:"status"`
	AccountJID  string     `json:"accountJid"`
	QR          string     `json:"qr"`
	QRExpiresAt *time.Time `json:"qrExpiresAt,omitempty"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	LastError   string     `json:"lastError"`
	ReconnectAt *time.Time `json:"reconnectAt,omitempty"`
}
type Group struct {
	JID  string `json:"jid"`
	Name string `json:"name"`
}
type Media struct {
	Available bool   `json:"available"`
	Type      string `json:"type"`
	MIME      string `json:"mime"`
	Size      int64  `json:"size"`
}
type Event struct {
	Sequence        int64      `json:"sequence"`
	Type            string     `json:"type"`
	Status          string     `json:"status"`
	AccountJID      string     `json:"accountJid"`
	GroupJID        string     `json:"groupJid"`
	SenderJID       string     `json:"senderJid"`
	SenderAltJID    string     `json:"senderAltJid"`
	MessageID       string     `json:"messageId"`
	EnvelopeID      string     `json:"envelopeId"`
	Revision        string     `json:"revision"`
	OccurredAt      *time.Time `json:"occurredAt"`
	ReceivedAt      time.Time  `json:"receivedAt"`
	Text            string     `json:"text"`
	QuotedMessageID string     `json:"quotedMessageId"`
	MentionedJIDs   []string   `json:"mentionedJids"`
	Media           *Media     `json:"media"`
}
type SendRequest struct {
	ID       id.ID  `json:"id"`
	GroupJID string `json:"groupJid"`
	// ToJID is set, without GroupJID, only for the reply to a link code a
	// member sent the bot in a direct message.
	ToJID     string   `json:"toJid,omitempty"`
	Text      string   `json:"text"`
	Mentions  []string `json:"mentions"`
	AssetID   string   `json:"assetId,omitempty"`
	MediaType string   `json:"mediaType,omitempty"`
}
type Job struct {
	AcceptedAt *time.Time `json:"acceptedAt,omitempty"`
	ID         id.ID      `json:"id"`
	MessageID  string     `json:"messageId"`
	Status     string     `json:"status"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}
type Asset struct {
	AssetID string `json:"assetId"`
	MIME    string `json:"mime"`
	Size    int64  `json:"size"`
	// DurationMS is the length of a GIF (a gif-playback mp4); 0 for a sticker.
	DurationMS int64 `json:"durationMs,omitempty"`
}

func path(cid id.ID) string { return "/connections/" + cid.String() }
func (c *Client) State(ctx context.Context, cid id.ID) (State, error) {
	var out State
	err := c.call(ctx, "GET", path(cid), nil, &out)
	return out, err
}
func (c *Client) Configure(ctx context.Context, cid id.ID, groups []string) (State, error) {
	if groups == nil {
		groups = []string{}
	}
	var out State
	err := c.call(ctx, "PUT", path(cid), map[string]any{"allowedGroups": groups}, &out)
	return out, err
}
func (c *Client) Action(ctx context.Context, cid id.ID, action string) (State, error) {
	if action != "connect" && action != "disconnect" && action != "logout" {
		return State{}, errors.New("aksi bridge tidak valid")
	}
	var out State
	err := c.call(ctx, "POST", path(cid)+"/"+action, nil, &out)
	return out, err
}
func (c *Client) Groups(ctx context.Context, cid id.ID) ([]Group, error) {
	var out []Group
	err := c.call(ctx, "GET", path(cid)+"/groups", nil, &out)
	return out, err
}
func (c *Client) Events(ctx context.Context, cid id.ID, after int64) ([]Event, error) {
	var out []Event
	err := c.call(ctx, "GET", fmt.Sprintf("%s/events?after=%d", path(cid), after), nil, &out)
	return out, err
}
func (c *Client) Acknowledge(ctx context.Context, cid id.ID, through int64) error {
	return c.call(ctx, "POST", path(cid)+"/ack", map[string]any{"through": through}, nil)
}
func (c *Client) Send(ctx context.Context, cid id.ID, request SendRequest) (Job, error) {
	var out Job
	err := c.call(ctx, "POST", path(cid)+"/send", request, &out)
	return out, err
}
func (c *Client) Job(ctx context.Context, cid, jid id.ID) (*Job, error) {
	var out *Job
	err := c.call(ctx, "GET", path(cid)+"/jobs/"+jid.String(), nil, &out)
	return out, err
}
func (c *Client) Sticker(ctx context.Context, cid id.ID, group, message string) (Asset, error) {
	var out Asset
	err := c.call(ctx, "POST", path(cid)+"/sticker", map[string]any{"groupJid": group, "messageId": message}, &out)
	return out, err
}

// GIF converts a stored or quoted group message (a GIF, video or image) into
// a short silent mp4 the bridge sends with gifPlayback (mediaType "gif").
func (c *Client) GIF(ctx context.Context, cid id.ID, group, message string) (Asset, error) {
	var out Asset
	err := c.call(ctx, "POST", path(cid)+"/gif", map[string]any{"groupJid": group, "messageId": message}, &out)
	return out, err
}
