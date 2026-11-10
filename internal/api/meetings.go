package api

import (
	"github.com/himasifunej/himasif-space/internal/id"
	"time"
)

type MeetingParticipant struct {
	UserID           id.ID      `json:"userId"`
	DisplayName      string     `json:"displayName"`
	ArrivedAt        *time.Time `json:"arrivedAt,omitempty"`
	AttendanceStatus string     `json:"attendanceStatus"`
	AttendanceNote   string     `json:"attendanceNote"`
	ConfirmedBy      *id.ID     `json:"confirmedBy,omitempty"`
}

type Meeting struct {
	ID              id.ID                `json:"id"`
	PeriodID        id.ID                `json:"periodId"`
	SpaceID         id.ID                `json:"spaceId"`
	OwnerDivisionID id.ID                `json:"ownerDivisionId"`
	OrganizerID     id.ID                `json:"organizerId"`
	Title           string               `json:"title"`
	Description     string               `json:"description"`
	Location        string               `json:"location"`
	JoinURL         string               `json:"joinUrl"`
	StartsAt        time.Time            `json:"startsAt"`
	EndsAt          *time.Time           `json:"endsAt"`
	TimeZone        string               `json:"timeZone"`
	NoteID          *id.ID               `json:"noteId,omitempty"`
	Cancelled       bool                 `json:"cancelled"`
	ReminderMinutes []int                `json:"reminderMinutes"`
	Participants    []MeetingParticipant `json:"participants"`
	Version         int64                `json:"version"`
	CreatedAt       time.Time            `json:"createdAt"`
	UpdatedAt       time.Time            `json:"updatedAt"`
	CanManage       bool                 `json:"canManage"`
	// Timer is the shared meeting timer (stage T2), nil while none runs.
	// TimerVersion counts its changes; an action carries it as
	// ExpectedVersion. It is apart from Version, so running the timer
	// never conflicts with editing the meeting.
	Timer        *MeetingTimer `json:"timer,omitempty"`
	TimerVersion int64         `json:"timerVersion"`
	// CanRunTimer: the caller may start and steer the timer (the meeting's
	// manager, or who may edit its note).
	CanRunTimer bool `json:"canRunTimer"`
}

type MeetingRequest struct {
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	Location        string     `json:"location"`
	JoinURL         string     `json:"joinUrl"`
	StartsAt        time.Time  `json:"startsAt"`
	EndsAt          *time.Time `json:"endsAt"`
	TimeZone        string     `json:"timeZone"`
	NoteID          *id.ID     `json:"noteId,omitempty"`
	Participants    []id.ID    `json:"participants"`
	ReminderMinutes []int      `json:"reminderMinutes"`
	Cancelled       bool       `json:"cancelled"`
	ExpectedVersion int64      `json:"expectedVersion"`
}

type AttendanceRequest struct {
	Status          string     `json:"status"`
	ArrivedAt       *time.Time `json:"arrivedAt,omitempty"`
	Note            string     `json:"note"`
	ExpectedVersion int64      `json:"expectedVersion"`
}
