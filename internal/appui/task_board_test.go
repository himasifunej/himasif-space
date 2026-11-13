package appui

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
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

// boardMentionFake answers the board's comment box: whom to tag, and the
// comments sent.
type boardMentionFake struct {
	*fakeAPI
	TasksAPI
	app      *App
	people   []api.TaskMentionCandidate
	comments []api.TaskCommentRequest
}

func (f *boardMentionFake) TaskMentionCandidates(context.Context, id.ID) ([]api.TaskMentionCandidate, error) {
	return f.people, nil
}
func (f *boardMentionFake) AddTaskComment(_ context.Context, _ id.ID, req api.TaskCommentRequest) (api.TaskComment, error) {
	f.comments = append(f.comments, req)
	return api.TaskComment{ID: id.New(), Body: req.Body}, nil
}
func (f *boardMentionFake) SpaceTasks(context.Context, id.ID, api.TaskFilter) ([]api.Task, error) {
	return slices.Clone(f.app.tasks.items), nil
}

// The owner's rc.6 report: in the board's task detail (#T9, Mediatek ·
// Board tugas) typing @ in "Komentar dan aktivitas" showed nothing. The
// box there is the same as Untukku's now: @ lists the task's people, a
// name goes in, and the comment is sent with the person tagged.
func TestTaskDetailCommentMention(t *testing.T) {
	for _, size := range [][2]float32{{1440, 900}, {900, 760}} {
		h := newChromeHarness(t, nil)
		h.tt.SetSize(int(size[0]), int(size[1]))
		testAuthorizedSpace(h.app, h.api.divisions[0])
		h.app.chrome.tab = "Board tugas"
		h.app.router.Push(PathSpaces)
		nara := id.New()
		task := api.Task{ID: id.New(), Title: "Redesain Page Vote", Progress: api.TaskTodo, MatchStatus: api.MatchNone, Assignee: api.PersonRef{ID: h.app.user.ID, DisplayName: h.app.user.DisplayName},
			Comments:    []api.TaskComment{{ID: id.New(), Author: api.PersonRef{DisplayName: "Aruna Desain"}, Body: "Cek ya @Nara Tekno", Mentions: []api.PersonRef{{ID: nara, DisplayName: "Nara Tekno"}}, CreatedAt: time.Date(2026, 11, 5, 8, 0, 0, 0, time.UTC)}},
			Permissions: api.TaskPermissions{Progress: true, Comment: true}}
		h.app.tasks = tasksUIState{scope: h.app.selectedSpaceID(), loaded: true, items: []api.Task{task}}
		f := &boardMentionFake{fakeAPI: h.api, app: h.app, people: []api.TaskMentionCandidate{
			{ID: id.New(), DisplayName: "Bima Ketua", Role: api.RoleKadiv, Division: "BPH"},
			{ID: nara, DisplayName: "Nara Tekno", Role: api.RoleMember, Division: "Mediatek", OnTask: true},
		}}
		h.app.api = f
		h.tt.Advance(400 * time.Millisecond)
		h.click("Tugas " + task.Title)
		h.want("Komentar dan aktivitas")
		h.click("Tulis komentar")
		h.tt.Type("Sudah @")
		h.tt.Frame()
		h.settle()
		h.want("Tandai orang")
		h.want("Tandai Nara Tekno")
		h.want("Tandai Bima Ketua")
		// The list lies inside the window, not clipped away.
		r, _ := h.tt.Find("Tandai Nara Tekno")
		if r.W == 0 || r.H == 0 || r.X+r.W > size[0] || r.Y+r.H > size[1] {
			t.Fatalf("%v: @ row at %+v", size, r)
		}
		// The owner's rc.7 report: the rows had no hover. Pointing at a row
		// highlights it, as ↑/↓ does, and only one row is highlighted.
		first, second := r, chromeRect(t, h.tt, "Tandai Bima Ketua")
		px := func(b ui.Rect) any { return h.tt.Image().At(int(b.X+b.W-6), int(b.Y+b.H/2)) }
		firstOn, secondOff := px(first), px(second)
		if firstOn == secondOff || h.app.untuk.mention.active != 0 {
			t.Fatalf("%v: the first row should start highlighted", size)
		}
		h.tt.Move(second.X+second.W-6, second.Y+second.H/2)
		h.tt.Frame()
		h.tt.Frame()
		if st := &h.app.untuk.mention; st.active != 1 {
			t.Fatalf("%v: pointing at the 2nd row left active at %d", size, st.active)
		}
		if px(second) != firstOn || px(first) != secondOff {
			t.Fatalf("%v: hovered row not highlighted alone", size)
		}
		// ↑ with the mouse resting still moves the highlight.
		h.tt.Key(0, ui.KeyUp)
		h.tt.Frame()
		h.tt.Frame()
		if st := &h.app.untuk.mention; st.active != 0 {
			t.Fatalf("%v: ↑ under a resting mouse left active at %d", size, st.active)
		}
		h.tt.Move(1, 1)
		h.tt.Type("nar")
		h.tt.Frame()
		h.dontWant("Tandai Bima Ketua")
		h.tt.Key(0, ui.KeyEnter)
		h.tt.Frame()
		if h.app.tasks.comment != "Sudah @Nara Tekno " {
			t.Fatalf("%v: comment %q", size, h.app.tasks.comment)
		}
		h.tt.Type("revisi")
		h.tt.Key(0, ui.KeyEnter)
		h.settle()
		if len(f.comments) != 1 || len(f.comments[0].Mentions) != 1 || f.comments[0].Mentions[0] != nara {
			t.Fatalf("%v: sent %+v", size, f.comments)
		}
	}
}
