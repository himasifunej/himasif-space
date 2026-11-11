package appui

import (
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
)

// Release review 0.4.5-rc2: a meeting going on past midnight stays under
// Hari ini until it ends.
func TestMeetingPastMidnightStaysToday(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	start := time.Date(2026, 11, 11, 23, 0, 0, 0, loc)
	end := start.Add(2 * time.Hour)
	m := meetingAt("Rapat malam", start, &end)
	m.TimeZone = "Asia/Jakarta"
	if g := meetingGroupOf(m, start.Add(90*time.Minute)); g != meetingToday {
		t.Fatalf("at 00.30 the meeting is %v, want today", g)
	}
	if g := meetingGroupOf(m, end.Add(time.Minute)); g != meetingPast {
		t.Fatalf("after its end the meeting is %v, want past", g)
	}
}

// "Buat notulensi di Catatan" in a meeting's Notulen tab links the new
// notes to that meeting, not to the next meeting without notes.
func TestMeetingNotulensiLinksThatMeeting(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(1440, 900)
	sp := testAuthorizedSpace(h.app, h.api.divisions[0])
	past := meetingAt("Rapat lalu", time.Now().Add(-72*time.Hour), nil)
	next := meetingAt("Rapat besok", time.Now().Add(20*time.Hour), nil)
	past.CanManage, past.OrganizerID = true, h.app.user.ID
	h.app.access.spaces[0].Roles[0].Capabilities = append(h.app.access.spaces[0].Roles[0].Capabilities, api.CapNotesManage, api.CapNotesRead)
	h.app.meetings = meetingsState{space: sp.ID, loaded: true, items: []api.Meeting{next, past}, selected: past.ID, tab: 2}
	h.app.notes.space, h.app.notes.loaded = sp.ID, true
	h.app.router.Push(PathSpaces)
	h.app.chrome.tab = "Rapat"
	h.tt.Frame()
	h.app.notes.templates = []api.NoteTemplate{{Kind: api.TemplateMeeting}}
	if err := h.tt.Click("Buat notulensi di Catatan"); err != nil {
		t.Fatalf("%v; %q", err, h.tt.Texts())
	}
	h.tt.Frame()
	if h.app.notes.share.meetingID != past.ID {
		t.Fatalf("notes will link %v, want the meeting on screen %v", h.app.notes.share.meetingID, past.ID)
	}
}
