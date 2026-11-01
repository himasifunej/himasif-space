package api

import (
	"github.com/himasifunej/himasif-space/internal/id"
	"time"
)

type MedhumPolicy struct {
	SpaceID      id.ID      `json:"spaceId"`
	PostingClock *string    `json:"postingClock,omitempty"`
	DesignClock  *string    `json:"designClock,omitempty"`
	TimeZone     string     `json:"timeZone"`
	Version      int64      `json:"version"`
	UpdatedAt    *time.Time `json:"updatedAt,omitempty"`
}
type MedhumPolicyRequest struct {
	PostingClock    *string `json:"postingClock,omitempty"`
	DesignClock     *string `json:"designClock,omitempty"`
	ExpectedVersion int64   `json:"expectedVersion"`
	Reason          string  `json:"reason"`
}
type ConnectorEvent struct {
	ID         id.ID     `json:"id"`
	ActorID    *id.ID    `json:"actorId,omitempty"`
	Action     string    `json:"action"`
	Before     any       `json:"before,omitempty"`
	After      any       `json:"after,omitempty"`
	OccurredAt time.Time `json:"occurredAt"`
}
