package api

import (
	"encoding/json"
	"github.com/himasifunej/himasif-space/internal/id"
	"time"
)

type WAParserPolicy struct {
	SpaceID           id.ID `json:"spaceId"`
	BroadcastEnabled  bool  `json:"broadcastEnabled"`
	DriveEnabled      bool  `json:"driveEnabled"`
	TemplateConfirmed bool  `json:"templateConfirmed"`
	Version           int64 `json:"version"`
}
type WAContentItem struct {
	ID            id.ID           `json:"id"`
	InboundID     id.ID           `json:"inboundId"`
	SpaceID       id.ID           `json:"spaceId"`
	Kind          string          `json:"kind"`
	Position      int             `json:"position"`
	ParserVersion string          `json:"parserVersion"`
	Parsed        json.RawMessage `json:"parsed"`
	Candidates    []id.ID         `json:"candidates"`
	ContentID     *id.ID          `json:"contentId,omitempty"`
	EvidenceID    *id.ID          `json:"evidenceId,omitempty"`
	SenderUserID  *id.ID          `json:"senderUserId,omitempty"`
	SenderName    string          `json:"senderName"`
	State         string          `json:"state"`
	Issue         string          `json:"issue"`
	Body          string          `json:"body"`
	SourceAt      time.Time       `json:"sourceAt"`
	ReceivedAt    time.Time       `json:"receivedAt"`
	MatchedAt     *time.Time      `json:"matchedAt,omitempty"`
	ReviewedAt    *time.Time      `json:"reviewedAt,omitempty"`
	ReviewedBy    *id.ID          `json:"reviewedBy,omitempty"`
	Version       int64           `json:"version"`
}
type WAContentDecision struct {
	Action          string `json:"action"`
	ContentID       *id.ID `json:"contentId,omitempty"`
	ExpectedVersion int64  `json:"expectedVersion"`
	ConfirmedAccess bool   `json:"confirmedAccess"`
	Reason          string `json:"reason"`
}
type WAContentReview struct {
	ID         id.ID           `json:"id"`
	ActorName  string          `json:"actorName"`
	Action     string          `json:"action"`
	Reason     string          `json:"reason"`
	Before     json.RawMessage `json:"before"`
	After      json.RawMessage `json:"after"`
	OccurredAt time.Time       `json:"occurredAt"`
}
