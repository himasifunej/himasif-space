package appui

import (
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

func dateIn(now time.Time, days int) *timeutil.Date {
	d := timeutil.DateOf(now.AddDate(0, 0, days))
	return &d
}

func TestJobdeskGroupsFollowCanvas(t *testing.T) {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	sp := id.New()
	mk := func(title, progress string, days int) api.Task {
		return api.Task{ID: id.New(), Title: title, Progress: progress, TimeZone: "UTC", DeadlineDate: dateIn(now, days), SpaceIDs: []id.ID{sp}}
	}
	tasks := []api.Task{
		mk("lewat", api.TaskTodo, -1),
		mk("pekan-b", api.TaskDoing, 4),
		mk("pekan-a", api.TaskTodo, 2),
		mk("nanti", api.TaskTodo, 14),
		mk("serah", api.TaskSubmitted, -3),
		mk("selesai", api.TaskDone, 1),
		mk("batal", api.TaskCancelled, 1),
		{ID: id.New(), Title: "tanpa tenggat", Progress: api.TaskTodo, SpaceIDs: []id.ID{sp}},
	}
	g := jobdeskGroups(tasks, now, id.Nil)
	names := func(ts []api.Task) []string {
		var out []string
		for _, t := range ts {
			out = append(out, t.Title)
		}
		return out
	}
	want := [4][]string{{"lewat"}, {"pekan-a", "pekan-b"}, {"nanti", "tanpa tenggat"}, {"serah"}}
	for i := range g {
		if got := names(g[i]); len(got) != len(want[i]) || (len(got) > 0 && (got[0] != want[i][0] || got[len(got)-1] != want[i][len(want[i])-1])) {
			t.Errorf("group %v = %v; want %v", jobdeskGroup(i), got, want[i])
		}
	}
	if g := jobdeskGroups(tasks, now, id.New()); len(g[0])+len(g[1])+len(g[2])+len(g[3]) != 0 {
		t.Error("Space filter kept tasks of other Spaces")
	}
}

// The Jobdesk page is a list, not the board; its steps move a task the way
// the board does, and Serahkan opens the detail for evidence instead.
func TestJobdeskListStepsUseExistingMoves(t *testing.T) {
	h, f := taskHarness(t)
	h.tt.SetSize(1440, 900)
	now := time.Now()
	f.items[0].DeadlineDate = dateIn(now, 2)
	f.items[0].TimeZone = "UTC"
	h.app.tasks.items[0] = f.items[0]
	h.app.router.Push(PathJobdesk)
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	h.want("MINGGU INI")
	h.want("Belum")
	h.want("Kerjakan")
	h.want("Serahkan")
	h.want("Buka")
	h.dontWant("Board tugas")
	h.click("Kerjakan: Poster")
	select {
	case r := <-f.moves:
		if r.Progress != api.TaskDoing {
			t.Fatalf("Kerjakan moved to %q", r.Progress)
		}
	case <-time.After(time.Second):
		t.Fatal("Kerjakan did not move the task")
	}
	h.settle()
	h.click("Serahkan: Poster")
	select {
	case r := <-f.moves:
		t.Fatalf("Serahkan moved the task without evidence: %+v", r)
	default:
	}
	if h.app.workspace.SelectedID() != f.items[0].ID {
		t.Fatal("Serahkan did not open the task's detail")
	}
	h.app.workspace.Close()
	h.tt.Frame()
	h.click("Buka Poster")
	if h.app.workspace.SelectedID() != f.items[0].ID {
		t.Fatal("Buka did not open the task's detail")
	}
}

func TestSubmissionsCountFromEvidenceTime(t *testing.T) {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	ev := func(at time.Time, status string) []api.TaskEvidence {
		return []api.TaskEvidence{{ID: id.New(), SubmittedAt: at, MatchStatus: status}}
	}
	due := dateIn(now, -2)
	tasks := []api.Task{
		{Title: "tepat", Progress: api.TaskDone, TimeZone: "UTC", DeadlineDate: due, Evidence: ev(now.AddDate(0, 0, -3), api.MatchVerified)},
		{Title: "telat", Progress: api.TaskSubmitted, TimeZone: "UTC", DeadlineDate: due, Evidence: ev(now.AddDate(0, 0, -1), api.MatchPending)},
		{Title: "kurang", Progress: api.TaskDoing, TimeZone: "UTC", DeadlineDate: due, Evidence: ev(now.AddDate(0, 0, -4), api.MatchRejected)},
		{Title: "lama", Progress: api.TaskDone, TimeZone: "UTC", DeadlineDate: due, Evidence: ev(now.AddDate(0, 0, -40), api.MatchVerified)},
	}
	counts, total := submissionCounts(tasks, now.AddDate(0, 0, -28), now)
	if total != 3 || counts != [3]int{1, 1, 1} {
		t.Fatalf("counts %v of %d; want one of each of 3", counts, total)
	}
	s := computeHomeStats(tasks, now.AddDate(0, 0, -7), now)
	if s.subTotal != 3 || s.review != 1 || s.active != 2 {
		t.Fatalf("stats %+v", s)
	}
	for i, series := range s.series {
		if len(series) < 2 {
			t.Errorf("sparkline %d has %d points", i, len(series))
		}
	}
}

func TestBerandaDateLineAndDeadlineStrip(t *testing.T) {
	loc := time.UTC
	p := &api.Period{StartsOn: timeutil.Date{Year: 2026, Month: 9, Day: 1}, EndsOn: timeutil.Date{Year: 2027, Month: 7, Day: 31}}
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, loc)
	if got, want := homeDateLine(p, now, loc), "Jumat, 6 November 2026 · minggu ke-10 periode"; got != want {
		t.Fatalf("date line %q; want %q", got, want)
	}
	days := deadlineDays([]api.Task{
		{Title: "hari ini", Progress: api.TaskTodo, DeadlineDate: dateIn(now, 0)},
		{Title: "lusa", Progress: api.TaskDoing, DeadlineDate: dateIn(now, 2)},
		{Title: "jauh", Progress: api.TaskTodo, DeadlineDate: dateIn(now, 20)},
		{Title: "selesai", Progress: api.TaskDone, DeadlineDate: dateIn(now, 1)},
	}, now, loc)
	if len(days[0]) != 1 || len(days[2]) != 1 || len(days[1]) != 0 {
		t.Fatalf("strip %v", days)
	}
}

func TestBerandaFollowsCanvas(t *testing.T) {
	h, f := taskHarness(t)
	h.tt.SetSize(1440, 900)
	sp := testAuthorizedSpace(h.app, h.api.divisions[0])
	now := time.Now()
	f.items[0].SpaceIDs = []id.ID{sp.ID}
	f.items[0].DeadlineDate = dateIn(now, 3)
	f.items[0].TimeZone = "UTC"
	f.items[0].CreatedAt = now.Add(-time.Hour)
	f.items[0].CreatedBy = f.items[0].Assignee
	h.app.router.Push(PathSpaces)
	h.app.chrome.tab = "Beranda"
	h.tt.Frame()
	h.settle()
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	for _, s := range []string{"Beranda " + sp.Name, "Minggu ini", "Bulan ini", "Periode", "Tugas aktif", "Lewat tenggat", "Perlu tinjauan", "Tepat waktu", "Beban kerja anggota", "Tenggat 14 hari ke depan", "Aktivitas terbaru"} {
		h.want(s)
	}
	h.dontWant("Ruang " + sp.Name + " kita.")
	h.click("Bulan ini")
	if h.app.home.rng != homeMonth {
		t.Fatal("range did not change")
	}
}
