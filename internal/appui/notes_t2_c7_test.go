package appui

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// t2API is the notes fake with the shared timer (T2) and the WhatsApp
// summary (C7).
type t2API struct {
	*c2NoteAPI
	meeting  *api.Meeting
	timerReq []api.MeetingTimerRequest
	timerErr error
	sumReq   []api.NoteWASummaryRequest
}

func (f *t2API) MeetingTimer(_ context.Context, mid id.ID, req api.MeetingTimerRequest) (api.Meeting, error) {
	f.timerReq = append(f.timerReq, req)
	if f.timerErr != nil {
		return api.Meeting{}, f.timerErr
	}
	next, err := api.ApplyTimer(f.meeting.Timer, req.Action, time.Now())
	if err != nil {
		return api.Meeting{}, err
	}
	if next != nil {
		next.ServerNow = time.Now()
		next.UpdatedBy = api.PersonRef{DisplayName: "Sinta Notulis"}
	}
	m := *f.meeting
	m.Timer, m.TimerVersion = next, m.TimerVersion+1
	return m, nil
}

func (f *t2API) NoteWASummary(_ context.Context, _ id.ID, req api.NoteWASummaryRequest) (api.NoteWASummary, error) {
	f.sumReq = append(f.sumReq, req)
	return api.NoteWASummary{Text: "*Ringkasan rapat: Rapat mingguan*\n\n*Keputusan*\n1. Oprec 15 Nov", GroupName: "Grup Mediatek", Decisions: 1, Actions: 0, Version: 3, Queued: req.Send}, nil
}

func t2Open(t *testing.T, editing bool) (*harness, *t2API) {
	t.Helper()
	h := gayaCOpen(t, editing, 1440, 900)
	f := &t2API{c2NoteAPI: h.app.api.(*c2NoteAPI), meeting: &h.app.meetings.items[0]}
	h.app.api = f
	h.tt.Frame()
	return h, f
}

// T2: a meeting's note shows the meeting's timer from the server. Who may
// not steer it sees it run, with who set it; who may, steers it through
// the server with the version seen, and a refusal shows and reloads.
func TestNoteSharedTimer(t *testing.T) {
	h, f := t2Open(t, false)
	h.want("Timer rapat belum dimulai. Notulis atau pengelola rapat yang memulainya.")
	h.dontWant("Mulai timer")

	now := time.Now()
	f.meeting.Timer = &api.MeetingTimer{StartedAt: now.Add(-3 * time.Minute), UpdatedBy: api.PersonRef{DisplayName: "Kadiv Mediatek"}, ServerNow: now, DoneMs: []int64{}}
	f.meeting.TimerVersion = 1
	h.tt.Frame()
	h.want("Timer rapat")
	h.want("Diatur oleh Kadiv Mediatek.")
	h.want("Timer bersama: semua yang membuka rapat ini melihat waktu yang sama.")
	h.dontWant("Jeda timer")
	h.dontWant("Hentikan timer")

	f.meeting.CanRunTimer = true
	h.tt.Frame()
	h.click("Jeda timer")
	drainNotes(t, h)
	h.tt.Frame()
	if len(f.timerReq) != 1 || f.timerReq[0] != (api.MeetingTimerRequest{Action: api.TimerPause, ExpectedVersion: 1}) {
		t.Fatalf("requests %+v", f.timerReq)
	}
	if m := h.app.meetings.items[0]; m.Timer == nil || m.Timer.PausedAt == nil || m.TimerVersion != 2 {
		t.Fatalf("meeting after pause %+v", m.Timer)
	}
	h.want("dijeda")
	h.want("Lanjutkan timer")
	if h.app.notes.dirty {
		t.Fatal("the timer must not change the note")
	}

	// Someone else moved it first: the refusal shows; nothing changes here.
	f.timerErr = errors.New("Timer baru saja diubah oleh Kadiv Mediatek.")
	h.click("Lanjutkan timer")
	drainNotes(t, h)
	h.tt.Frame()
	if h.app.notes.timerErr == "" || h.app.notes.timerBusy {
		t.Fatalf("refusal not shown: %q busy=%v", h.app.notes.timerErr, h.app.notes.timerBusy)
	}

	// Hiding the pill is this device's choice; the timer runs on.
	f.timerErr = nil
	h.app.notes.timerErr = ""
	h.click("Sembunyikan timer")
	h.tt.Frame()
	h.want("Tampilkan pil timer")
	if len(f.timerReq) != 2 {
		t.Fatalf("hiding sent a request: %+v", f.timerReq)
	}
}

// T1 stays for a note without a meeting.
func TestNoteTimerLocalWithoutMeeting(t *testing.T) {
	h, f := t2Open(t, false)
	h.app.notes.note.MeetingID = nil
	h.tt.Frame()
	h.want("Timer hanya di perangkat ini.")
	h.click("Mulai timer")
	h.tt.Frame()
	h.want("Timer rapat")
	if len(f.timerReq) != 0 {
		t.Fatal("a local timer called the server")
	}
}

// C7: the editor previews the summary, then sends it with the version the
// preview showed.
func TestNoteWASummaryDialog(t *testing.T) {
	h, f := t2Open(t, true)
	h.want("Kirim ringkasan ke WhatsApp")
	h.click("Kirim ringkasan ke WhatsApp")
	drainNotes(t, h)
	h.tt.Frame()
	h.want("Kirim ringkasan ke WhatsApp?")
	h.want("Ke grup Grup Mediatek · 1 keputusan, 0 action item")
	if len(f.sumReq) != 1 || f.sumReq[0].Send {
		t.Fatalf("preview %+v", f.sumReq)
	}
	h.click("Kirim ke Grup Mediatek")
	drainNotes(t, h)
	h.tt.Frame()
	if len(f.sumReq) != 2 || !f.sumReq[1].Send || f.sumReq[1].ExpectedVersion != 3 {
		t.Fatalf("send %+v", f.sumReq)
	}
	h.dontWant("Kirim ringkasan ke WhatsApp?")
	h.want("Ringkasan dikirim ke grup Grup Mediatek.")
}

// A4: the row menu ⋯ (Lihat tugas, Keluarkan…) and the bottom panel with
// the pending invitations beside a short "Kewenangan role".
func TestMemberRowMenuAndBottomPanel(t *testing.T) {
	h, _, _, _ := memberAccessHarness(t, 1440, 1500)
	h.want("Kewenangan role")
	h.want("Semua, termasuk role, undangan, 45 dan bot.")
	h.want("Undangan tertunda")
	h.click("Lainnya untuk Nara Tekno")
	h.tt.Frame()
	h.want("Lihat tugas")
	h.click("Keluarkan…")
	h.tt.Frame()
	h.want("Keluarkan Nara Tekno dari Media dan Teknologi?")
}
