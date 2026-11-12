package appui

import (
	"fmt"
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

// Jobdesk saya (canvas 3747dbbac0d0 design 6) groups by progress:
// Dikerjakan, Belum mulai, Diserahkan, Selesai; by deadline inside, those
// without one last; cancelled ones leave; the Space filter keeps its own.
func TestJobdeskGroupsFollowCanvas(t *testing.T) {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	sp := id.New()
	mk := func(title, progress string, days int) api.Task {
		return api.Task{ID: id.New(), Title: title, Progress: progress, TimeZone: "UTC", DeadlineDate: dateIn(now, days), SpaceIDs: []id.ID{sp}}
	}
	tasks := []api.Task{
		{ID: id.New(), Title: "tanpa tenggat", Progress: api.TaskTodo, SpaceIDs: []id.ID{sp}},
		mk("belum-b", api.TaskTodo, 4),
		mk("belum-a", api.TaskTodo, -1),
		mk("kerja", api.TaskDoing, 14),
		mk("serah", api.TaskSubmitted, -3),
		mk("selesai", api.TaskDone, 1),
		mk("batal", api.TaskCancelled, 1),
	}
	g := jobdeskGroups(tasks, id.Nil)
	var got [][]string
	for _, ts := range g {
		var names []string
		for _, t := range ts {
			names = append(names, t.Title)
		}
		got = append(got, names)
	}
	want := [][]string{{"kerja"}, {"belum-a", "belum-b", "tanpa tenggat"}, {"serah"}, {"selesai"}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("groups %v, want %v", got, want)
	}
	for _, ts := range jobdeskGroups(tasks, id.New()) {
		if len(ts) != 0 {
			t.Fatal("Space filter kept tasks of other Spaces")
		}
	}
}

// A row opens Detail tugas, whose back link returns to Jobdesk saya.
func TestJobdeskRowOpensDetail(t *testing.T) {
	h, f := taskHarness(t)
	h.tt.SetSize(1440, 900)
	f.items[0].DeadlineDate = dateIn(time.Now(), 2)
	f.items[0].TimeZone = "UTC"
	h.app.tasks.items[0] = f.items[0]
	h.app.router.Push(PathJobdesk)
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	h.want("Jobdesk saya")
	h.want("Belum mulai")
	h.dontWant("Board tugas")
	h.click("Buka Poster")
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	if h.app.router.Path() != PathUntukTask || h.app.untuk.task != f.items[0].ID {
		t.Fatalf("row opened %s %v", h.app.router.Path(), h.app.untuk.task)
	}
	h.want("Deskripsi")
	h.want("Bukti pengerjaan")
	h.click("Kembali ke Jobdesk saya")
	if h.app.router.Path() != PathJobdesk {
		t.Fatalf("back led to %s", h.app.router.Path())
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
