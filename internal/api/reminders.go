package api

import (
	"time"

	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// How a reminder is due. A DueDate reminder is a calendar date with no time
// of day; a DueInstant reminder is a precise moment. The two are never
// mixed, and a date is never given a made-up hour (PRD MED-04, MED-05).
const (
	DueDate    = "date"
	DueInstant = "instant"
)

// The caller's relation to a reminder: its owner, who may do anything, or a
// recipient, who may only read it (PRI-07).
const (
	ReminderOwner     = "owner"
	ReminderRecipient = "recipient"
)

// Reminder is a personal reminder as the owner or a recipient sees it. It
// is private by default and never a division's task (PRI-01, PRI-07).
//
// Exactly one of DueOn and DueAt is set, as DueKind says: a date-only
// reminder carries DueOn, an instant reminder DueAt (RFC3339 UTC). TimeZone
// is the zone the owner set the reminder in, for showing it.
//
// Relation tells whether the caller owns the reminder or only reads it as a
// recipient. Shared and Recipients are filled only for the owner; a
// recipient never sees who else a reminder is shared with.
type Reminder struct {
	ID          id.ID           `json:"id"`
	OwnerID     id.ID           `json:"ownerId"`
	Title       string          `json:"title"`
	Notes       string          `json:"notes"`
	DueKind     string          `json:"dueKind"`
	DueOn       *timeutil.Date  `json:"dueOn,omitempty"`
	DueAt       *time.Time      `json:"dueAt,omitempty"`
	TimeZone    string          `json:"timeZone"`
	Completed   bool            `json:"completed"`
	CompletedAt *time.Time      `json:"completedAt,omitempty"`
	Shared      bool            `json:"shared"`
	Recipients  []ReminderShare `json:"recipients,omitempty"`
	Relation    string          `json:"relation"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// ReminderShare is one read-only recipient of a reminder, as the owner sees
// it (PRI-05). RecipientID is the account chosen when sharing, not a
// position: changing who holds a role changes nothing here (PRI-03, PRI-04).
type ReminderShare struct {
	RecipientID id.ID     `json:"recipientId"`
	DisplayName string    `json:"displayName"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ReminderRequest creates or replaces a reminder. DueKind picks the field
// read: "date" reads DueOn, "instant" reads DueAt. TimeZone defaults to
// Asia/Jakarta. On create, Shared defaults to the owner's preference and is
// ignored here; sharing is toggled with the share endpoints.
type ReminderRequest struct {
	Title    string         `json:"title"`
	Notes    string         `json:"notes"`
	DueKind  string         `json:"dueKind"`
	DueOn    *timeutil.Date `json:"dueOn,omitempty"`
	DueAt    *time.Time     `json:"dueAt,omitempty"`
	TimeZone string         `json:"timeZone"`
}

// ReminderCompleteRequest marks a reminder done or not done. Only the owner
// may send it (PRI-07).
type ReminderCompleteRequest struct {
	Completed bool `json:"completed"`
}

// ReminderShareRequest shares a reminder with another account, read-only.
// RecipientID is a personal account; it is the account itself, not a
// position (PRI-03).
type ReminderShareRequest struct {
	RecipientID id.ID `json:"recipientId"`
}

// ReminderSharingRequest turns a reminder's sharing toggle on or off
// without naming recipients (PRI-02). Turning it off keeps the recipient
// list but hides the reminder from them until it is turned on again.
type ReminderSharingRequest struct {
	Shared bool `json:"shared"`
}

// ReminderPreferences is the owner's default for new reminders (PRI-02).
// Changing it never reopens an existing reminder (PRI-05).
type ReminderPreferences struct {
	DefaultShare bool `json:"defaultShare"`
}

// ReminderAuditEntry is one recorded change to a reminder, readable by its
// owner (PRI-05, AC-09). Before and After are the raw audit values.
type ReminderAuditEntry struct {
	Action      string    `json:"action"`
	ActorUserID *id.ID    `json:"actorUserId,omitempty"`
	OccurredAt  time.Time `json:"occurredAt"`
	Before      any       `json:"before,omitempty"`
	After       any       `json:"after,omitempty"`
}
