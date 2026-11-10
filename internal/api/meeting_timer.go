package api

import (
	"errors"
	"time"
)

// Stage T2: the meeting timer lives on the server, on the meeting, so the
// notulis on a laptop and the kadiv on a phone see the same one. Its state
// is a few stamps from the server's clock; a client works the time out
// from them (correcting for its own clock with ServerNow) and changes it
// only through the timer actions, with the TimerVersion it saw.

// Timer actions.
const (
	TimerStart  = "start"
	TimerPause  = "pause"
	TimerResume = "resume"
	TimerNext   = "next"
	TimerExtend = "extend"
	TimerStop   = "stop"
)

// TimerExtendStep is what one "+5 mnt" adds to the current agenda item.
const TimerExtendStep = 5 * time.Minute

// MeetingTimer is the shared timer. Durations are milliseconds of running
// time (pauses left out): AgendaAtMs is when the current agenda item began,
// DoneMs how long each finished item took, ExtraMs the "+5 mnt" given to
// the current one. Agenda counts past the last item once all are done.
type MeetingTimer struct {
	StartedAt  time.Time  `json:"startedAt"`
	PausedAt   *time.Time `json:"pausedAt,omitempty"`
	PausedMs   int64      `json:"pausedMs"`
	Agenda     int        `json:"agenda"`
	AgendaAtMs int64      `json:"agendaAtMs"`
	ExtraMs    int64      `json:"extraMs"`
	DoneMs     []int64    `json:"doneMs"`
	UpdatedBy  PersonRef  `json:"updatedBy"`
	// ServerNow is the server's clock when it sent the timer; not stored.
	ServerNow time.Time `json:"serverNow,omitzero"`
}

// MeetingTimerRequest is one timer action on a meeting.
type MeetingTimerRequest struct {
	Action          string `json:"action"`
	ExpectedVersion int64  `json:"expectedVersion"`
}

// ErrTimerAction: the action does not fit the timer's state (start while it
// runs, resume while it runs, …) or is unknown.
var ErrTimerAction = errors.New("timer action does not fit")

// Running is the timer's running time at now, pauses left out.
func (t MeetingTimer) Running(now time.Time) time.Duration {
	end := now
	if t.PausedAt != nil {
		end = *t.PausedAt
	}
	return max(0, end.Sub(t.StartedAt)-time.Duration(t.PausedMs)*time.Millisecond)
}

// ApplyTimer applies an action at now to t (nil: no timer) and returns the
// new timer, nil after stop. t is not changed.
func ApplyTimer(t *MeetingTimer, action string, now time.Time) (*MeetingTimer, error) {
	if action == TimerStart {
		if t != nil {
			return nil, ErrTimerAction
		}
		return &MeetingTimer{StartedAt: now, DoneMs: []int64{}}, nil
	}
	if t == nil {
		return nil, ErrTimerAction
	}
	n := *t
	n.DoneMs = append([]int64{}, t.DoneMs...)
	n.ServerNow = time.Time{}
	switch action {
	case TimerPause:
		if n.PausedAt != nil {
			return nil, ErrTimerAction
		}
		n.PausedAt = &now
	case TimerResume:
		if n.PausedAt == nil {
			return nil, ErrTimerAction
		}
		n.PausedMs += max(0, now.Sub(*n.PausedAt)).Milliseconds()
		n.PausedAt = nil
	case TimerNext:
		at := n.Running(now).Milliseconds()
		n.DoneMs = append(n.DoneMs, max(0, at-n.AgendaAtMs))
		n.Agenda++
		n.AgendaAtMs, n.ExtraMs = at, 0
	case TimerExtend:
		n.ExtraMs += TimerExtendStep.Milliseconds()
	case TimerStop:
		return nil, nil
	default:
		return nil, ErrTimerAction
	}
	return &n, nil
}
