package appui

import (
	"context"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// The owner's rc.6 report: "Ubah rapat" slid in as a view of its own,
// confusing to use. Tugas baru, Ubah tugas, Rapat baru / Ubah rapat and
// Ubah akses share one sheet now (canvas 3c8b4b7134bc): over the page from
// the right, 560 px, its sections headed, its foot with Batal and the one
// primary action in view however long the form.
func TestFormSheetsShareOnePattern(t *testing.T) {
	h, draft := drawerHarness(t, nil)
	draft.title, draft.output = "Poster", "Poster di Drive"
	h.app.taskDraftKeep()
	h.tt.Frame()
	sheet := chromeRect(t, h.tt, "Tugas baru")
	if sheet.W < 540 || sheet.W > 561 || sheet.X+sheet.W < 1439 {
		t.Fatalf("task sheet at %+v, want 560 px at the right edge", sheet)
	}
	for _, s := range []string{"TUGAS", "PENANGGUNG JAWAB", "WAKTU", "DETAIL LAINNYA", "Batal", "Esc"} {
		h.want(s)
	}
	captureNativeUI(t, h, "sheet-task")
	foot := chromeRect(t, h.tt, "Batal")
	if foot.Y+foot.H > 900 || foot.Y < 800 {
		t.Fatalf("foot at %+v, not kept at the bottom", foot)
	}

	// Rapat: the same sheet, sectioned; Batalkan rapat at the end.
	h.app.tasks.draft = nil
	h.app.chrome.tab = "Rapat"
	h.app.meetings = meetingsState{space: h.app.selectedSpaceID(), loaded: true, members: []api.PersonRef{{ID: id.New(), DisplayName: "Aruna Desain"}, {ID: id.New(), DisplayName: "Nara Tekno"}}}
	h.app.beginMeeting(&api.Meeting{ID: id.New(), Title: "Rapat koordinasi", StartsAt: time.Now().Add(24 * time.Hour), TimeZone: "Asia/Jakarta", Version: 2})
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	for _, s := range []string{"Ubah rapat", "RAPAT", "WAKTU", "TEMPAT", "PESERTA · 0 DARI 2", "NOTULENSI", "Batalkan rapat", "Simpan rapat"} {
		h.want(s)
	}
	if r := chromeRect(t, h.tt, "Ubah rapat"); r.W < 540 || r.W > 561 {
		t.Fatalf("meeting sheet %+v", r)
	}
	captureNativeUI(t, h, "sheet-meeting")
	// Nothing changed: Esc closes at once.
	h.tt.Key(0, ui.KeyEscape)
	h.tt.Frame()
	if h.app.meetings.draft != nil {
		t.Fatal("Esc kept an unchanged form")
	}
	h.tt.Advance(time.Second)
	h.tt.Frame()
	// Changed: Esc asks first; Lanjut mengisi keeps it; Batal drops it.
	h.app.beginMeeting(&api.Meeting{ID: id.New(), Title: "Rapat koordinasi", StartsAt: time.Now().Add(24 * time.Hour), TimeZone: "Asia/Jakarta", Version: 2})
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	h.click("Judul rapat *")
	h.tt.Type(" showcase")
	h.tt.Frame()
	if !h.app.meetingDirty() {
		t.Fatal("typing did not change the form")
	}
	h.tt.Key(0, ui.KeyEscape)
	h.tt.Frame()
	h.want("Buang perubahan rapat yang belum disimpan?")
	if h.app.meetings.draft == nil {
		t.Fatal("Esc dropped a changed form without asking")
	}
	h.click("Lanjut mengisi")
	h.tt.Frame()
	if d := h.app.meetings.draft; d == nil || d.title != "Rapat koordinasi showcase" {
		t.Fatal("Lanjut mengisi lost the change")
	}
	h.click("Batal")
	h.tt.Frame()
	if h.app.meetings.draft != nil {
		t.Fatal("Batal kept the form")
	}
}

// Ctrl+Enter saves from anywhere in the sheet.
func TestFormSheetCtrlEnterSaves(t *testing.T) {
	h, _ := drawerHarness(t, nil)
	h.app.tasks.draft = nil
	h.app.chrome.tab = "Rapat"
	saved := make(chan api.MeetingRequest, 1)
	h.app.api = &meetingSaveFake{fakeAPI: h.api, saved: saved}
	h.app.meetings = meetingsState{space: h.app.selectedSpaceID(), loaded: true}
	m := api.Meeting{ID: id.New(), Title: "Rapat koordinasi", StartsAt: time.Now().Add(24 * time.Hour), TimeZone: "Asia/Jakarta", Version: 2}
	h.app.beginMeeting(&m)
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	h.click("Judul rapat *")
	h.tt.Type(" (revisi)")
	h.tt.Key(ui.Ctrl, ui.KeyEnter)
	h.settle()
	select {
	case r := <-saved:
		if r.Title != m.Title+" (revisi)" {
			t.Fatalf("saved %q", r.Title)
		}
	case <-time.After(time.Second):
		t.Fatal("Ctrl+Enter did not save")
	}
}

type meetingSaveFake struct {
	*fakeAPI
	MeetingsAPI
	saved chan api.MeetingRequest
}

func (f *meetingSaveFake) UpdateMeeting(_ context.Context, mid id.ID, r api.MeetingRequest) (api.Meeting, error) {
	f.saved <- r
	return api.Meeting{ID: mid, Title: r.Title, StartsAt: r.StartsAt, TimeZone: r.TimeZone}, nil
}
func (f *meetingSaveFake) SpaceMeetings(context.Context, id.ID) ([]api.Meeting, error) {
	return nil, nil
}
func (f *meetingSaveFake) MeetingCandidates(context.Context, id.ID) ([]api.PersonRef, error) {
	return nil, nil
}
func (f *meetingSaveFake) SpaceNotes(context.Context, id.ID) ([]api.Note, error) { return nil, nil }

// On a phone the sheet fills the screen, paired fields stack without
// running into what follows, and the foot stays in view.
func TestFormSheetOnPhone(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(390, 844)
	testAuthorizedSpace(h.app, h.api.divisions[0])
	h.app.router.Push(PathSpaces)
	h.app.chrome.tab = "Rapat"
	h.app.meetings = meetingsState{space: h.app.selectedSpaceID(), loaded: true}
	h.app.beginMeeting(&api.Meeting{ID: id.New(), Title: "Rapat koordinasi", StartsAt: time.Now().Add(24 * time.Hour), TimeZone: "Asia/Jakarta", Version: 2})
	h.tt.Advance(time.Second)
	h.tt.Frame()
	if r := chromeRect(t, h.tt, "Ubah rapat"); r.X != 0 || r.W != 390 {
		t.Fatalf("sheet %+v, want the whole width", r)
	}
	if save := chromeRect(t, h.tt, "Simpan rapat"); save.Y+save.H > 844 {
		t.Fatalf("Simpan rapat below the screen: %+v", save)
	}
	step, reminders := chromeRect(t, h.tt, "+15"), chromeRect(t, h.tt, "Pengingat")
	if reminders.Y < step.Y+step.H {
		t.Fatalf("Pengingat %+v runs into Jam selesai's steps %+v", reminders, step)
	}
}
