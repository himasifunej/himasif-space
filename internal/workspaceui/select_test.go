package workspaceui

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Select mode: "Pilih" or Ctrl/Shift+click, checkboxes on the cards the
// user may cancel only, "Batalkan (N)" in board order, Esc leaves.
func TestSelectModeAndBulkCancel(t *testing.T) {
	s := New()
	tasks := fixtureTasks()
	tasks[0].Selectable, tasks[2].Selectable = true, true // tasks[1] is not the user's to cancel
	if err := s.SetTasks(tasks); err != nil {
		t.Fatal(err)
	}
	s.OnMove = func(MoveRequest) {}
	tt := boardTester(t, s, 1240)
	if _, ok := tt.Find("Pilih"); ok {
		t.Fatal("Pilih shown without OnBulkCancel")
	}
	var got [][]id.ID
	s.OnBulkCancel = func(ids []id.ID) { got = append(got, ids) }
	tt.Frame()

	// A plain click still opens the task; Ctrl+click starts select mode.
	if err := tt.Click("Tugas " + tasks[0].Title); err != nil {
		t.Fatal(err)
	}
	if s.Selecting() || s.SelectedID() != tasks[0].ID {
		t.Fatal("plain click did not open the task")
	}
	s.Close()
	tt.Frame()
	if err := tt.ClickWith(ui.Ctrl, "Tugas "+tasks[2].Title); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !s.Selecting() || !slices.Equal(s.Selected(), []id.ID{tasks[2].ID}) || s.SelectedID() != id.Nil {
		t.Fatalf("ctrl+click: selecting %v, %v", s.Selecting(), s.Selected())
	}
	if !tt.HasText("1 dipilih") {
		t.Fatalf("no count in %q", tt.Texts())
	}
	if _, ok := tt.Find("Pindahkan " + tasks[0].Title); ok {
		t.Fatal("card menu offered in select mode")
	}
	// A click picks; a card the user may not cancel stays unpicked.
	for _, title := range []string{tasks[0].Title, tasks[1].Title + ", tidak dapat kamu batalkan"} {
		label := "Pilih tugas " + title
		if title != tasks[0].Title {
			label = title
		}
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(s.Selected(), []id.ID{tasks[0].ID, tasks[2].ID}) {
		t.Fatalf("selected %v", s.Selected())
	}
	s.Select([]id.ID{tasks[1].ID, tasks[0].ID, tasks[2].ID})
	if !slices.Equal(s.Selected(), []id.ID{tasks[0].ID, tasks[2].ID}) {
		t.Fatalf("Select picked an unselectable task: %v", s.Selected())
	}
	tt.Frame()
	if err := tt.Click("Batalkan (2)"); err != nil {
		t.Fatalf("%v in %q", err, tt.Texts())
	}
	if len(got) != 1 || !slices.Equal(got[0], []id.ID{tasks[0].ID, tasks[2].ID}) {
		t.Fatalf("bulk cancel got %v", got)
	}
	// Unpick by clicking again.
	if err := tt.Click("Pilih tugas " + tasks[0].Title); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(s.Selected(), []id.ID{tasks[2].ID}) {
		t.Fatalf("unpick: %v", s.Selected())
	}

	// A new snapshot drops what is gone or no longer cancellable.
	tasks[2].Selectable = false
	if err := s.SetTasks(tasks); err != nil {
		t.Fatal(err)
	}
	if len(s.Selected()) != 0 || !s.Selecting() {
		t.Fatalf("snapshot kept %v", s.Selected())
	}
	tt.Frame()
	if !tt.HasText("Belum ada yang dipilih") {
		t.Fatalf("no empty count in %q", tt.Texts())
	}
	if err := tt.Click("Batalkan (0)"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("Batalkan (0) sent %v", got)
	}

	// Esc leaves select mode and clears it.
	s.Select([]id.ID{tasks[0].ID})
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if s.Selecting() || len(s.Selected()) != 0 {
		t.Fatal("Esc did not leave select mode")
	}
	if tt.HasText("Klik kartu untuk memilih · Esc untuk keluar") {
		t.Fatal("selection bar still shown")
	}
	// "Pilih" enters it, "Selesai memilih" leaves it.
	if err := tt.Click("Pilih"); err != nil {
		t.Fatal(err)
	}
	if !s.Selecting() {
		t.Fatal("Pilih did not enter select mode")
	}
	if err := tt.Click("Selesai memilih"); err != nil {
		t.Fatal(err)
	}
	if s.Selecting() {
		t.Fatal("Selesai memilih did not leave")
	}
	// Without the caller's bulk action, select mode closes.
	s.Select([]id.ID{tasks[0].ID})
	s.OnBulkCancel = nil
	tt.Frame()
	if s.Selecting() {
		t.Fatal("select mode kept without OnBulkCancel")
	}
}
