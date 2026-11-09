package workspaceui

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Cards follow canvas A: several labels, the PJ's first name, a red
// "lewat" deadline, a short code with checklist and attachment counts, no
// description; lanes end in a drop slot and "Tambah tugas".
func TestCardFollowsCanvas(t *testing.T) {
	s := New()
	s.OnMove = func(MoveRequest) {}
	var added []Lane
	s.OnAdd = func(l Lane) { added = append(added, l) }
	task := Task{ID: id.New(), Title: "Siapkan poster showcase", Description: "Deskripsi panjang yang tidak tampil di kartu", Assignee: "[DUMMY] Aruna Desain", Due: "5 Nov", Late: true, Ref: "#3F2A", ChecklistDone: 1, ChecklistTotal: 3, Attachments: 2, Color: ui.Hex("#a9c4dd"),
		Labels: []Label{{Name: "Desain", Color: ui.Hex("#a9c4dd")}, {Name: "Urgent", Color: ui.Hex("#e2806f")}}, Lane: Todo}
	if err := s.SetTasks([]Task{task}); err != nil {
		t.Fatal(err)
	}
	tt := boardTester(t, s, 1280)
	for _, want := range []string{"Desain", "Urgent", "Aruna", "5 Nov · lewat", "#3F2A", "1/3", "2", "Lepas kartu di sini", "Tarik kartu ke kolom ini"} {
		if !tt.HasText(want) {
			t.Fatalf("card or lane misses %q", want)
		}
	}
	if tt.HasText(task.Description) {
		t.Fatal("card shows the description")
	}
	if err := tt.Click("Tambah tugas di Dikerjakan"); err != nil {
		t.Fatal(err)
	}
	if len(added) != 1 || added[0] != Doing {
		t.Fatalf("Tambah tugas reported %v", added)
	}
}

// A toolbar replaces the task count, and lanes without OnAdd have no
// "Tambah tugas".
func TestBoardToolbarAndNoAddWithoutHandler(t *testing.T) {
	s := New()
	s.Subtitle = "Geser kartu antar kolom."
	s.Toolbar = func(c *ui.Context) { ui.Text(c, "Saya saja") }
	if err := s.SetTasks(fixtureTasks()); err != nil {
		t.Fatal(err)
	}
	tt := boardTester(t, s, 1280)
	if !tt.HasText("Saya saja") || !tt.HasText("Geser kartu antar kolom.") || tt.HasText("3 tugas") {
		t.Fatal("toolbar or subtitle missing, or count still shown")
	}
	if tt.HasText("Tambah tugas") {
		t.Fatal("Tambah tugas shown without a handler")
	}
}

func TestFirstNameSkipsTags(t *testing.T) {
	for in, want := range map[string]string{"[DUMMY] Aruna Desain": "Aruna", "Nara": "Nara", "": ""} {
		if got := firstName(in); got != want {
			t.Fatalf("firstName(%q) = %q, want %q", in, got, want)
		}
	}
}
