package api

import (
	"github.com/himasifunej/himasif-space/internal/id"
	"time"
)

type WAGroup struct {
	ID              id.ID  `json:"id"`
	SpaceID         id.ID  `json:"spaceId"`
	JID             string `json:"jid"`
	Name            string `json:"name"`
	CommandsEnabled bool   `json:"commandsEnabled"`
}
type WAConnection struct {
	ID          id.ID             `json:"id"`
	PeriodID    id.ID             `json:"periodId"`
	DivisionID  id.ID             `json:"divisionId"`
	DisplayName string            `json:"displayName"`
	Language    string            `json:"language"`
	Greeting    string            `json:"greeting"`
	Templates   map[string]string `json:"templates"`
	Enabled     bool              `json:"enabled"`
	Status      string            `json:"status"`
	AccountJID  string            `json:"accountJid"`
	Groups      []WAGroup         `json:"groups"`
	LastEventAt *time.Time        `json:"lastEventAt,omitempty"`
	LastError   string            `json:"lastError"`
	Version     int64             `json:"version"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}
type WAConnectionRequest struct {
	DisplayName     string            `json:"displayName"`
	Language        string            `json:"language"`
	Greeting        string            `json:"greeting"`
	Templates       map[string]string `json:"templates"`
	ExpectedVersion int64             `json:"expectedVersion"`
}
type WAActionRequest struct {
	Action          string `json:"action"`
	ExpectedVersion int64  `json:"expectedVersion"`
	Confirmed       bool   `json:"confirmed"`
}
type WAGroupRequest struct {
	SpaceID         id.ID  `json:"spaceId"`
	JID             string `json:"jid"`
	CommandsEnabled bool   `json:"commandsEnabled"`
	ExpectedVersion int64  `json:"expectedVersion"`
}
type WALinkChallenge struct {
	Code         string    `json:"code"`
	ExpiresAt    time.Time `json:"expiresAt"`
	Instructions string    `json:"instructions"`
	// DeepLink opens a chat with the division's bot with the code typed in
	// (https://wa.me/<bot>?text=<code>): the member only taps Send. Empty
	// while the division has no connected bot; then only !link in a group
	// works.
	DeepLink string `json:"deepLink,omitempty"`
}

// WALinkStatus is the caller's own WhatsApp link in a period.
type WALinkStatus struct {
	Linked bool `json:"linked"`
	// Phone is the linked number with its middle hidden (+62 812-••••-4471);
	// empty when WhatsApp only shared a private ID (LID).
	Phone    string     `json:"phone,omitempty"`
	LinkedAt *time.Time `json:"linkedAt,omitempty"`
	// BotReady tells whether "Tautkan WhatsApp" can open a chat with the
	// division's bot.
	BotReady bool `json:"botReady"`
}
type WASessionState struct {
	Status      string     `json:"status"`
	AccountJID  string     `json:"accountJid"`
	QR          string     `json:"qr"`
	QRExpiresAt *time.Time `json:"qrExpiresAt,omitempty"`
	LastError   string     `json:"lastError"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}
type WAAvailableGroup struct {
	JID  string `json:"jid"`
	Name string `json:"name"`
}
type WACustomCommand struct {
	ID        id.ID   `json:"id"`
	Trigger   string  `json:"trigger"`
	Response  string  `json:"response"`
	AssetID   string  `json:"assetId"`
	MediaType string  `json:"mediaType"`
	GroupIDs  []id.ID `json:"groupIds"`
	Version   int64   `json:"version"`
}
type WACustomCommandRequest struct {
	Trigger         string  `json:"trigger"`
	Response        string  `json:"response"`
	AssetID         string  `json:"assetId"`
	MediaType       string  `json:"mediaType"`
	GroupIDs        []id.ID `json:"groupIds"`
	ExpectedVersion int64   `json:"expectedVersion"`
}
type WAOutboxItem struct {
	SuppressedAt      *time.Time `json:"suppressedAt,omitempty"`
	SuppressionReason string     `json:"suppressionReason,omitempty"`
	ID                id.ID      `json:"id"`
	GroupName         string     `json:"groupName"`
	Status            string     `json:"status"`
	Attempts          int        `json:"attempts"`
	MessageID         string     `json:"messageId"`
	LastError         string     `json:"lastError"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

// WAPreviewRequest is a draft of the bot's texts to preview (Tahap 7d).
// Kind: reminder, overdue, ping, reply, weekly or followup.
type WAPreviewRequest struct {
	Kind        string            `json:"kind"`
	DisplayName string            `json:"displayName"`
	Language    string            `json:"language"`
	Greeting    string            `json:"greeting"`
	Templates   map[string]string `json:"templates"`
}

// WAPreview is the text the bot would send for the draft, made from
// example data (Example is always true).
type WAPreview struct {
	Kind    string `json:"kind"`
	Text    string `json:"text"`
	Example bool   `json:"example"`
}
