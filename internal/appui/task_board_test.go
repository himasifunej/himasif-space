package appui

import (
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

func TestTaskLabelsRefAndShortDate(t *testing.T) {
	if got := taskLabels(" Desain, Urgent,,Desain "); len(got) != 2 || got[0] != "Desain" || got[1] != "Urgent" {
		t.Fatalf("labels %v", got)
	}
	if got := taskLabels(""); len(got) != 0 {
		t.Fatalf("empty category gave %v", got)
	}
	tid := id.New()
	if ref := taskRef(tid); len(ref) != 5 || ref[0] != '#' || taskRef(tid) != ref {
		t.Fatalf("ref %q", ref)
	}
	d := timeutil.Date{Year: 2026, Month: 11, Day: 5}
	if got := taskShortDate(api.Task{DeadlineDate: &d}); got != "5 Nov" {
		t.Fatalf("date-only deadline %q", got)
	}
	at := time.Date(2026, 11, 5, 23, 30, 0, 0, time.UTC) // 6 Nov in Jakarta
	if got := taskShortDate(api.Task{DeadlineAt: &at, TimeZone: "Asia/Jakarta"}); got != "6 Nov" {
		t.Fatalf("clocked deadline %q", got)
	}
}

// "Saya saja" keeps the tasks the user leads or helps on.
func TestBoardMineOnly(t *testing.T) {
	h, f := taskHarness(t)
	other := api.PersonRef{ID: id.New(), DisplayName: "Nara"}
	late := timeutil.Date{Year: 2020, Month: 1, Day: 1}
	f.items = append(f.items,
		api.Task{ID: id.New(), Title: "Bukan milikku", PeriodID: h.app.periodID, Version: 1, Progress: api.TaskTodo, Assignee: other, DeadlineDate: &late, Category: "Tekno, Konten",
			Checklist: []api.TaskChecklistItem{{ID: id.New(), Done: true}, {ID: id.New()}}, Evidence: []api.TaskEvidence{{ID: id.New()}}},
		api.Task{ID: id.New(), Title: "Dibantu", PeriodID: h.app.periodID, Version: 1, Progress: api.TaskDoing, Assignee: other, Participants: []api.PersonRef{{ID: f.user.ID}}},
	)
	h.app.tasks.loaded = false
	h.app.loadTasks()
	h.settle()
	h.app.projectTasks()
	for _, tk := range f.items {
		if tk.Title == "Bukan milikku" {
			h.app.workspace.Open(tk.ID)
		}
	}
	if h.app.workspace.SelectedID() == id.Nil {
		t.Fatal("other task missing from the board")
	}
	h.app.tasks.mineOnly = true
	h.app.projectTasks()
	if h.app.workspace.SelectedID() != id.Nil {
		t.Fatal("Saya saja kept someone else's task")
	}
	for _, title := range []string{"Poster", "Dibantu"} {
		found := false
		for _, tk := range f.items {
			if tk.Title == title {
				h.app.workspace.Open(tk.ID)
				found = h.app.workspace.SelectedID() == tk.ID
			}
		}
		if !found {
			t.Fatalf("Saya saja dropped %q", title)
		}
	}
	if !taskLate(f.items[1], time.Now()) {
		t.Fatal("past deadline not late")
	}
}

// The detail modal follows canvas A: status pill and code in the header,
// actions only as permissions allow, the Lewat badge, and one timeline of
// comments and activity on the right.
func TestTaskDetailModalFollowsCanvas(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(1440, 900)
	testAuthorizedSpace(h.app, h.api.divisions[0])
	h.app.chrome.tab = "Board tugas"
	h.app.router.Push(PathSpaces)
	past := timeutil.Date{Year: 2020, Month: 1, Day: 1}
	pj := api.PersonRef{ID: id.New(), DisplayName: "Aruna Desain"}
	task := api.Task{ID: id.New(), Title: "Siapkan poster showcase", Progress: api.TaskTodo, MatchStatus: api.MatchNone, Category: "Desain, Urgent", Assignee: pj, DeadlineDate: &past, ExpectedOutput: "Poster 1080×1350 di Drive",
		CreatedBy: api.PersonRef{DisplayName: "Demo Mediatek"}, CreatedAt: time.Date(2026, 11, 4, 8, 10, 0, 0, time.UTC),
		Comments:    []api.TaskComment{{ID: id.New(), Author: pj, Body: "Draf pertama sudah di Drive", CreatedAt: time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)}},
		Permissions: api.TaskPermissions{Progress: true, Comment: true}}
	h.app.tasks = tasksUIState{scope: h.app.selectedSpaceID(), loaded: true, items: []api.Task{task}}
	h.tt.Advance(400 * time.Millisecond)
	h.click("Tugas " + task.Title)
	for _, want := range []string{"Status: Belum mulai", taskRef(task.ID), "Serahkan hasil", "Lampiran", "Desain", "Urgent", "Lewat", "Hasil yang diminta", "Deskripsi", "Komentar dan aktivitas", "Draf pertama sudah di Drive", "membuat tugas dan menetapkan Aruna Desain sebagai PJ"} {
		h.want(want)
	}
	// Without manage rights there is no Label, PJ, Ingatkan or task menu.
	for _, absent := range []string{"Ingatkan Aruna", "Ajukan bantuan", "Menu tugas", "Simpan label"} {
		h.dontWant(absent)
	}
	if _, ok := h.tt.Find("Label"); ok {
		if _, action := h.tt.Find("Ganti PJ utama"); action {
			t.Fatal("PJ popover shown without manage rights")
		}
	}
	h.click("Serahkan hasil")
	h.want("Tautan hasil (https://…)")
}
