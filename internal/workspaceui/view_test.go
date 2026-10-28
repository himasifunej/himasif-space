package workspaceui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/id"
)

func fixtureTasks() []Task {
	return []Task{
		{ID: id.New(), Title: "Konsep Mingper 11", Description: "Siapkan konsep dan caption untuk posting pekan ini.", Category: "Konten", Assignee: "Naura Yesha", Deadline: "24 Okt", Posting: "26 Oktober 2026", Space: "Medhum · Humas", Lane: Todo},
		{ID: id.New(), Title: "Desain Lesson Learned", Description: "Buat desain berdasarkan konsep Humas.", Category: "Desain", Assignee: "Tiyo Ahimsa", Deadline: "23 Okt", Posting: "25 Oktober 2026", Space: "Medhum · Mediatek", ExpectedOutput: "Tautan Drive berisi desain siap posting.", Lane: Todo},
		{ID: id.New(), Title: "Publikasi Sempro", Category: "Publikasi", Assignee: "Oktavia", Deadline: "21 Okt", Posting: "23 Oktober 2026", Space: "Medhum", Evidence: "Tautan Drive dikirim oleh PJ; kecocokan tugas belum ditinjau.", Lane: Review},
	}
}

func boardTester(t *testing.T, state *State, width int) *ui.Tester {
	t.Helper()
	return ui.NewTester(func(c *ui.Context) {
		theme := *ui.DarkTheme()
		theme.Background = ui.RGB(22, 21, 17)
		theme.Surface = ui.RGB(34, 33, 28)
		theme.Accent = ui.RGB(212, 198, 162)
		c.SetTheme(&theme)
		ui.Column(c).Fill().Background(theme.Background).TextColor(theme.Text).Children(func() { state.View(c) })
	}, width, 740)
}

func TestSnapshotValidationAndEmptyState(t *testing.T) {
	s := New()
	if err := s.SetTasks(nil); err != nil {
		t.Fatal(err)
	}
	tt := boardTester(t, s, 1100)
	if !tt.HasText("Belum ada tugas") {
		t.Fatal("missing empty state")
	}
	tasks := fixtureTasks()
	if err := s.SetTasks(tasks); err != nil {
		t.Fatal(err)
	}
	s.Open(tasks[0].ID)
	if err := s.SetTasks(append(tasks, tasks[0])); err == nil {
		t.Fatal("duplicate accepted")
	}
	if s.SelectedID() != tasks[0].ID || len(s.tasks) != 3 {
		t.Fatal("invalid snapshot changed state")
	}
	if err := s.SetTasks([]Task{{Title: "Missing identity"}}); err == nil {
		t.Fatal("zero ID accepted")
	}
	if err := s.SetTasks([]Task{{ID: id.New(), Lane: Lane(99)}}); err == nil {
		t.Fatal("unknown lane accepted")
	}
	tasks[0].Title = "Caller changed original slice"
	if s.tasks[0].Title == tasks[0].Title {
		t.Fatal("snapshot retained caller slice")
	}
	if err := s.SetTasks(nil); err != nil {
		t.Fatal(err)
	}
	if s.SelectedID() != id.Nil {
		t.Fatal("removed selection retained")
	}
	tt.Frame()
	if !tt.HasText("Belum ada tugas") {
		t.Fatal("empty snapshot not rendered")
	}
}

func TestSnapshotReadinessRequiresValidSnapshot(t *testing.T) {
	var absent *State
	if absent.HasSnapshot() {
		t.Fatal("nil state has a snapshot")
	}
	s := New()
	if s.HasSnapshot() {
		t.Fatal("new state has a snapshot")
	}
	if err := s.SetTasks([]Task{{Title: "Missing identity"}}); err == nil {
		t.Fatal("invalid snapshot accepted")
	}
	if s.HasSnapshot() {
		t.Fatal("invalid snapshot marked unloaded state ready")
	}
	if err := s.SetTasks(nil); err != nil {
		t.Fatal(err)
	}
	if !s.HasSnapshot() {
		t.Fatal("confirmed empty snapshot is not ready")
	}
	if err := s.SetTasks([]Task{{ID: id.New(), Lane: Lane(99)}}); err == nil {
		t.Fatal("invalid lane accepted")
	}
	if !s.HasSnapshot() || len(s.tasks) != 0 {
		t.Fatal("invalid snapshot changed confirmed empty state")
	}
	tasks := fixtureTasks()
	if err := s.SetTasks(tasks); err != nil {
		t.Fatal(err)
	}
	if !s.HasSnapshot() {
		t.Fatal("populated snapshot is not ready")
	}
	if err := s.SetTasks(append(tasks, tasks[0])); err == nil {
		t.Fatal("duplicate accepted")
	}
	if !s.HasSnapshot() || len(s.tasks) != len(tasks) {
		t.Fatal("invalid snapshot changed populated readiness")
	}
}

func rect(t *testing.T, tt *ui.Tester, label string) ui.Rect {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("missing element %q; texts: %v", label, tt.Texts())
	}
	return r
}
func pointerDrag(tt *ui.Tester, from, to ui.Rect, bottom bool) {
	x, y := from.X+35, from.Y+45
	dx, dy := to.X+40, to.Y+8
	if bottom {
		dy = to.Y + to.H - 8
	}
	tt.Press(x, y)
	for step := 1; step <= 4; step++ {
		f := float32(step) / 4
		tt.Move(x+(dx-x)*f, y+(dy-y)*f)
	}
	tt.Release(dx, dy)
}

func TestPointerMoveAndReorderAreIntents(t *testing.T) {
	s := New()
	tasks := fixtureTasks()
	if err := s.SetTasks(tasks); err != nil {
		t.Fatal(err)
	}
	var moves []MoveRequest
	s.OnMove = func(m MoveRequest) { moves = append(moves, m) }
	tt := boardTester(t, s, 1240)
	pointerDrag(tt, rect(t, tt, "Tugas "+tasks[0].Title), rect(t, tt, "Akhir "+Doing.String()), false)
	if len(moves) != 1 || moves[0] != (MoveRequest{TaskID: tasks[0].ID, Destination: Doing}) {
		t.Fatalf("cross-lane move: %+v", moves)
	}
	if s.tasks[0].Lane != Todo {
		t.Fatal("move pretended snapshot was saved")
	}
	if s.SelectedID() != id.Nil {
		t.Fatal("drag opened inspector")
	}
	pointerDrag(tt, rect(t, tt, "Tugas "+tasks[1].Title), rect(t, tt, "Tugas "+tasks[0].Title), false)
	if len(moves) != 2 || moves[1] != (MoveRequest{TaskID: tasks[1].ID, Destination: Todo, BeforeID: tasks[0].ID}) {
		t.Fatalf("same-lane reorder: %+v", moves)
	}
	// Cancelled drags cannot submit a request.
	source := rect(t, tt, "Tugas "+tasks[0].Title)
	target := rect(t, tt, "Akhir "+Done.String())
	tt.Press(source.X+30, source.Y+45)
	tt.Move(target.X+30, target.Y+20)
	tt.Key(0, ui.KeyEscape)
	tt.Release(target.X+30, target.Y+20)
	if len(moves) != 2 {
		t.Fatal("cancelled drag emitted intent")
	}
}

func TestInspectorShrinksBoardAndReadOnlyDisablesMoves(t *testing.T) {
	s := New()
	tasks := fixtureTasks()
	if err := s.SetTasks(tasks); err != nil {
		t.Fatal(err)
	}
	tt := boardTester(t, s, 1240)
	before := rect(t, tt, "Kolom board")
	if err := tt.Click("Tugas " + tasks[2].Title); err != nil {
		t.Fatal(err)
	}
	if s.SelectedID() != tasks[2].ID {
		t.Fatal("card did not open inspector")
	}
	panel := rect(t, tt, "Detail tugas")
	after := rect(t, tt, "Kolom board")
	if !(after.W < before.W && after.X+after.W <= panel.X+1) {
		t.Fatalf("inspector overlays rather than shrinks board: before=%+v after=%+v panel=%+v", before, after, panel)
	}
	if !tt.HasText(tasks[2].Evidence) {
		t.Fatal("evidence not distinct in inspector")
	}
	if _, ok := tt.Find("Pindahkan " + tasks[0].Title); ok {
		t.Fatal("read-only board exposed move control")
	}
	if err := tt.Click("Tutup detail tugas"); err != nil {
		t.Fatal(err)
	}
	if s.SelectedID() != id.Nil {
		t.Fatal("close did not clear selection")
	}
	s.Open(tasks[1].ID)
	if err := s.SetTasks(tasks[:1]); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := tt.Find("Detail tugas"); ok {
		t.Fatal("removed task kept stale inspector")
	}
}

func TestCustomMoveMenuAndHorizontalEdgeScroll(t *testing.T) {
	s := New()
	tasks := fixtureTasks()
	if err := s.SetTasks(tasks); err != nil {
		t.Fatal(err)
	}
	var moves []MoveRequest
	s.OnMove = func(m MoveRequest) { moves = append(moves, m) }
	tt := boardTester(t, s, 700)
	if err := tt.Click("Pindahkan " + tasks[0].Title); err != nil {
		t.Fatal(err)
	}
	// The custom popover takes focus and supports arrows + Enter.
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	if len(moves) != 1 || moves[0].Destination != Doing {
		t.Fatalf("custom menu move: %+v", moves)
	}
	if len(tt.Menu()) != 0 {
		t.Fatal("native OS menu used")
	}
	source := rect(t, tt, "Tugas "+tasks[0].Title)
	viewport := rect(t, tt, "Kolom board")
	tt.Press(source.X+30, source.Y+45)
	tt.Move(viewport.X+viewport.W-12, source.Y+50)
	start := s.boardScroll.X
	tt.Advance(160 * time.Millisecond)
	if s.boardScroll.X <= start {
		t.Fatalf("horizontal edge did not scroll: %v => %v (max %v)", start, s.boardScroll.X, s.boardScroll.MaxX)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Release(viewport.X+viewport.W-12, source.Y+50)
	if len(moves) != 1 {
		t.Fatal("edge-scroll cancellation emitted move")
	}
}

// Set SPACE_UI_SCREENSHOT_DIR to capture the real native renderer for review.
func TestNativeBoardCapture(t *testing.T) {
	dir := os.Getenv("SPACE_UI_SCREENSHOT_DIR")
	if dir == "" {
		t.Skip("set SPACE_UI_SCREENSHOT_DIR for native captures")
	}
	s := New()
	if err := s.SetTasks(fixtureTasks()); err != nil {
		t.Fatal(err)
	}
	s.OnMove = func(MoveRequest) {}
	tt := boardTester(t, s, 1480)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"board", "inspector"} {
		if name == "inspector" {
			s.Open(s.tasks[2].ID)
			tt.Frame()
		}
		file, err := os.Create(filepath.Join(dir, "native-"+name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(file, tt.Image())
		closeErr := file.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
}

func TestMixedTaskPermissionsAndCancelledInspector(t *testing.T) {
	tasks := fixtureTasks()
	tasks[0].ReadOnly = true
	tasks = append(tasks, Task{ID: id.New(), Title: "Tugas dibatalkan", Lane: Cancelled})
	s := New()
	if err := s.SetTasks(tasks); err != nil {
		t.Fatal(err)
	}
	moves := 0
	s.OnMove = func(MoveRequest) { moves++ }
	s.requestMove(tasks[0].ID, Doing, id.Nil)
	if moves != 0 {
		t.Fatal("read-only card emitted move in a mixed board")
	}
	s.requestMove(tasks[1].ID, Doing, id.Nil)
	if moves != 1 {
		t.Fatal("own card cannot move in a mixed board")
	}
	s.Open(tasks[3].ID)
	tt := boardTester(t, s, 1440)
	if !tt.HasText("Dibatalkan") || !tt.HasText("Tugas dibatalkan") {
		t.Fatal("cancelled task cannot be inspected")
	}
}
