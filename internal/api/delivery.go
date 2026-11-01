package api

import (
	"github.com/himasifunej/himasif-space/internal/id"
	"time"
)

type NotificationPolicy struct {
	TaskDays    []int `json:"taskDays"`
	LocalHour   int   `json:"localHour"`
	LocalMinute int   `json:"localMinute"`
	OverdueDays int   `json:"overdueDays"`
}

func DefaultNotificationPolicy() NotificationPolicy {
	return NotificationPolicy{TaskDays: []int{2, 1, 0}, LocalHour: 6, OverdueDays: 1}
}

type Delivery struct {
	ID           id.ID      `json:"id"`
	EventID      id.ID      `json:"eventId"`
	SourceKind   string     `json:"sourceKind"`
	SourceID     id.ID      `json:"sourceId"`
	RelatedKind  string     `json:"relatedKind,omitempty"`
	RelatedID    *id.ID     `json:"relatedId,omitempty"`
	SpaceID      *id.ID     `json:"spaceId,omitempty"`
	Title        string     `json:"title"`
	Body         string     `json:"body"`
	TimeZone     string     `json:"timeZone"`
	ScheduledAt  time.Time  `json:"scheduledAt"`
	AvailableAt  time.Time  `json:"availableAt"`
	Channel      string     `json:"channel"`
	Status       string     `json:"status"`
	SeenAt       *time.Time `json:"seenAt,omitempty"`
	SnoozedUntil *time.Time `json:"snoozedUntil,omitempty"`
	LastError    string     `json:"lastError"`
}

type DeliveryResponse struct {
	Action        string `json:"action"`
	SnoozeMinutes int    `json:"snoozeMinutes,omitempty"`
}

type PingRequest struct {
	SourceKind string   `json:"sourceKind"`
	SourceID   *id.ID   `json:"sourceId,omitempty"`
	Message    string   `json:"message"`
	Recipients []id.ID  `json:"recipients"`
	Channels   []string `json:"channels"`
}
