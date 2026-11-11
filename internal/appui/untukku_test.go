package appui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// myMeetingsFake answers MyMeetings.
type myMeetingsFake struct {
	*fakeAPI
	meetings []api.Meeting
	calls    int
}

func (f *myMeetingsFake) MyMeetings(context.Context, id.ID) ([]api.Meeting, error) {
	f.calls++
	return f.meetings, nil
}

// untukFixture is a member of Mediatek and Medhum with a week of tasks,
// meetings and reminders around now.
func untukFixture(t *testing.T) (*harness, *myMeetingsFake, time.Time) {
	t.Helper()
	h := newChromeHarness(t, nil)
	h.tt.SetSize(1440, 900)
	h.app.period().TimeZone = noonZone(h.tt.Now())
	sp := testAuthorizedSpace(h.app, h.api.divisions[0])
	shared := api.Space{ID: id.New(), PeriodID: h.app.periodID, Name: "Medhum", Kind: api.SpaceShared, Divisions: h.api.divisions[:2]}
	h.app.access.spaces = append(h.app.access.spaces, shared)
	now := h.tt.Now()
	loc := h.app.homeZone()
	today := now.In(loc)
	at := func(days, hour int) time.Time {
		return time.Date(today.Year(), today.Month(), today.Day()+days, hour, 0, 0, 0, loc)
	}
	late, soon, tomorrow := at(-2, 9), now.Add(90*time.Minute), at(1, 10)
	h.app.tasks = tasksUIState{scope: h.app.periodID, personal: true, loaded: true, items: []api.Task{
		{ID: id.New(), Title: "Laporan proker", Progress: api.TaskDoing, DeadlineAt: &late, SpaceIDs: []id.ID{sp.ID}},
		{ID: id.New(), Title: "Caption Dies", Progress: api.TaskTodo, DeadlineAt: &tomorrow, SpaceIDs: []id.ID{shared.ID}},
		{ID: id.New(), Title: "Sudah selesai", Progress: api.TaskDone, DeadlineAt: &tomorrow, SpaceIDs: []id.ID{sp.ID}},
		{ID: id.New(), Title: "Tanpa tenggat", Progress: api.TaskTodo, SpaceIDs: []id.ID{sp.ID}},
	}}
	f := &myMeetingsFake{fakeAPI: h.api, meetings: []api.Meeting{
		{ID: id.New(), SpaceID: sp.ID, Title: "Rapat Divisi", StartsAt: soon, Location: "Sekret", Participants: []api.MeetingParticipant{{}, {}}},
		{ID: id.New(), SpaceID: shared.ID, Title: "Rapat batal", StartsAt: soon, Cancelled: true},
	}}
	h.app.api = f
	h.app.untuk = untukState{} // read again from f
	due := at(1, 8)
	h.app.reminders = remindersUIState{loaded: true, items: []api.Reminder{{ID: id.New(), Title: "Bayar kas", DueAt: &due, OwnerID: h.app.user.ID, Relation: api.ReminderOwner}}}
	h.app.router.Push(PathUntukku)
	h.tt.Advance(500 * time.Millisecond)
	h.settle()
	h.settle() // the meetings, asked for by the first frame
	h.tt.Frame()
	return h, f, now
}

// Untukku is the first page and Pribadi's one item; it shows the week by
// day, late first, each row with its Space and action, the next thing on
// top, and leaves out what is done, cancelled or without a deadline.
func TestUntukkuTimeline(t *testing.T) {
	h, f, _ := untukFixture(t)
	if f.calls == 0 {
		t.Fatal("meetings not read")
	}
	for _, want := range []string{"Untukku", "Lewat tenggat", "Hari ini", "Besok", "Laporan proker", "Rapat Divisi", "Caption Dies", "Bayar kas", "Berikutnya", "Selesai", "Semua tugasku", "Semua pengingat"} {
		h.want(want)
	}
	for _, gone := range []string{"Sudah selesai", "Tanpa tenggat", "Rapat batal", "Jobdesk saya", "Reminder pribadi"} {
		h.dontWant(gone)
	}
	texts := strings.Join(h.tt.Texts(), "|")
	if !strings.Contains(texts, "dalam 1 j 2") {
		t.Errorf("no countdown to the meeting: %s", texts)
	}
	if i, j := strings.Index(texts, "Lewat tenggat"), strings.Index(texts, "Hari ini"); i < 0 || j < 0 || i > j {
		t.Fatalf("late not first: %s", texts)
	}
	h.click("Rapat")
	h.tt.Frame()
	h.dontWant("Laporan proker")
	h.want("Rapat Divisi")
}

// A busy day shows four rows and "+N lainnya", which opens the rest.
func TestUntukkuBusyDay(t *testing.T) {
	h, f, now := untukFixture(t)
	for i := range 6 {
		f.meetings = append(f.meetings, api.Meeting{ID: id.New(), SpaceID: h.app.access.spaces[0].ID, Title: fmt.Sprintf("Rapat %d", i), StartsAt: now.Add(time.Duration(10+i) * time.Minute)})
	}
	h.app.untuk.meetings = f.meetings
	h.tt.Frame()
	h.want("+3 lainnya")
	h.dontWant("Rapat 5")
	h.click("Tampilkan 3 lainnya")
	h.tt.Frame()
	h.want("Rapat 5")
}

// A quiet week says so, with one action.
func TestUntukkuEmpty(t *testing.T) {
	h, f, _ := untukFixture(t)
	h.app.tasks.items, f.meetings, h.app.untuk.meetings = nil, nil, nil
	h.app.reminders.items = nil
	h.tt.Frame()
	h.want("Minggu ini lapang")
	h.want("+ Pengingat pribadi")
	h.dontWant("Berikutnya")
}

// The picker is a table: role, next meeting, new activity from unseen
// device notices, the user's late tasks; it searches, and highlights the
// Space last opened. The logo comes here; the breadcrumb goes back.
func TestSpacePickerTableAndLogo(t *testing.T) {
	h, _, _ := untukFixture(t)
	sp := h.app.access.spaces[0]
	h.app.access.spaces[0].Roles[0].Roles = []string{api.RoleKadiv}
	h.app.deliveries.items = []api.Delivery{{ID: id.New(), SpaceID: &sp.ID, Channel: "device", Status: "queued"}, {ID: id.New(), SpaceID: &sp.ID, Channel: "whatsapp", Status: "queued"}}
	h.app.selectSpace(sp.ID)
	h.app.navigate(PathSpaces)
	h.tt.Advance(500 * time.Millisecond)
	h.tt.Frame()
	h.want("›")
	h.click("Kembali ke Space saya")
	h.tt.Advance(500 * time.Millisecond)
	h.tt.Frame()
	if h.app.selectedSpace() != nil || h.app.router.Path() != PathSpaces || h.app.access.lastSpace != sp.ID {
		t.Fatalf("breadcrumb: selected %v path %q last %v", h.app.selectedSpace(), h.app.router.Path(), h.app.access.lastSpace)
	}
	// The picker counts the user's own late tasks (Jobdesk's list); the
	// fake has none of its own, so they are put back.
	h.app.tasks = tasksUIState{scope: h.app.periodID, personal: true, loaded: true, items: []api.Task{{ID: id.New(), Title: "Laporan proker", Progress: api.TaskDoing, DeadlineAt: ptrTime(h.tt.Now().Add(-48 * time.Hour)), SpaceIDs: []id.ID{sp.ID}}}}
	rows := h.app.spacePickerRows(h.tt.Now(), "")
	if len(rows) != 2 {
		t.Fatalf("rows %+v", rows)
	}
	r := rows[0]
	if r.role != "Kadiv" || !strings.Contains(r.next, "Rapat Divisi") || r.unseen != 1 || r.late != 1 || !r.current {
		t.Fatalf("Mediatek row %+v", r)
	}
	if rows[1].role != "Bersama" || rows[1].late != 0 || rows[1].current {
		t.Fatalf("Medhum row %+v", rows[1])
	}
	for _, want := range []string{"Space saya", "PERANMU", "RAPAT BERIKUTNYA", "KABAR BARU", "TUGASMU LEWAT", "Kadiv", "Bersama"} {
		h.want(want)
	}
	if rows := h.app.spacePickerRows(h.tt.Now(), "medh"); len(rows) != 1 || rows[0].space.Name != "Medhum" {
		t.Fatalf("search %+v", rows)
	}
	// Open a Space, then the logo leads back to the picker.
	h.click("Buka Medhum")
	h.tt.Advance(500 * time.Millisecond)
	h.tt.Frame()
	if h.app.selectedSpace() == nil || h.app.selectedSpace().Name != "Medhum" {
		t.Fatal("row did not open its Space")
	}
	h.click("Ke daftar Space")
	if h.app.selectedSpace() != nil || h.app.router.Path() != PathSpaces || h.app.access.lastSpace == id.Nil {
		t.Fatal("logo did not lead to Space saya")
	}
}

func TestUntukCountdown(t *testing.T) {
	for d, want := range map[time.Duration]string{30 * time.Second: "sebentar lagi", 5 * time.Minute: "dalam 5 m", 134 * time.Minute: "dalam 2 j 14 m", 50 * time.Hour: "dalam 2 hari"} {
		if got := untukCountdown(d); got != want {
			t.Errorf("%v: %q, want %q", d, got, want)
		}
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

// noonZone is a zone where now falls at midday, so that "in 90 minutes"
// stays today and "tomorrow" stays tomorrow whatever hour the tests run.
func noonZone(now time.Time) string {
	switch off := 12 - now.UTC().Hour(); {
	case off > 0:
		return fmt.Sprintf("Etc/GMT-%d", off)
	case off < 0:
		return fmt.Sprintf("Etc/GMT+%d", -off)
	}
	return "Etc/GMT"
}
