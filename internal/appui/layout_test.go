package appui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/workspaceui"
)

// Side navigation stays fixed and every session control remains reachable at
// the window's minimum supported size, including a selected division.
func TestShellFitsMinimumWindow(t *testing.T) {
	h := newHarness(t, newFake(api.AccountPerson), "")
	h.app.Start()
	h.settle()
	h.login(h.api.user.Email, h.api.password)
	h.app.chrome.divisionID = h.api.divisions[0].ID
	h.app.router.Push(PathSpaces)
	for _, size := range [][2]int{{1440, 900}, {1100, 720}, {880, 640}} {
		h.tt.SetSize(size[0], size[1])
		h.tt.Frame()
		for _, label := range []string{"Menu profil", "Penampilan", "Ciutkan sidebar", "Tersambung", "Notifikasi", "Cari di Space"} {
			r, ok := h.tt.Find(label)
			if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > float32(size[0])+1 || r.Y+r.H > float32(size[1])+1 {
				t.Errorf("%dx%d: %s out of window: %+v, exists=%v", size[0], size[1], label, r, ok)
			}
		}
	}
}

// SPACE_UI_CAPTURE_DIR opts into PNG captures from the actual native renderer.
// It writes no credentials or production data: the account is a test fixture.
func TestNativeCapture(t *testing.T) {
	dir := os.Getenv("SPACE_UI_CAPTURE_DIR")
	if dir == "" {
		t.Skip("set SPACE_UI_CAPTURE_DIR for native screenshots")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, newFake(api.AccountPerson), "")
	h.app.Start()
	h.settle()
	capture := func(name string) {
		t.Helper()
		h.tt.Advance(400 * time.Millisecond)
		h.tt.Frame()
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, h.tt.Image())
		closeErr := f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	h.tt.SetSize(1440, 900)
	capture("login")
	h.login(h.api.user.Email, h.api.password)
	h.app.chrome.divisionID = h.api.divisions[0].ID
	h.app.router.Push(PathSpaces)
	capture("overview-dark")
	h.click("Notifikasi")
	capture("notifications")
	h.click("Notifikasi")
	h.click("Penampilan")
	capture("appearance")
	h.click("Terang")
	capture("overview-light")
	h.click("Tutup penampilan")
	h.app.chrome.tab = "Board tugas"
	taskID := id.New()
	if err := h.app.workspace.SetTasks([]workspaceui.Task{
		{ID: taskID, Title: "Desain Lesson Learned", Description: "Sesuaikan desain dengan konsep Humas.", Category: "Desain", Assignee: "Ani Wulandari", Deadline: "23 Oktober 2026", Posting: "25 Oktober 2026", Space: "Medhum", ExpectedOutput: "Link Drive berisi desain siap posting.", Lane: workspaceui.Doing},
		{ID: id.New(), Title: "Konsep Mingper 11", Category: "Konten", Assignee: "PJ Humas", Deadline: "24 Oktober 2026", Lane: workspaceui.Todo},
	}); err != nil {
		t.Fatal(err)
	}
	capture("board")
	h.click("Tugas Desain Lesson Learned")
	capture("board-inspector")
	h.app.workspace.Close()
	h.app.chrome.tab = "Pengaturan"
	capture("whatsapp")
	h.tt.SetSize(880, 640)
	capture("minimum-window")
	h.app.chrome.preferences.SidebarCollapsed = true
	capture("collapsed")
}

func TestInspectorsShareRightSideAtMinimumWidth(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(880, 640)
	h.app.chrome.divisionID = h.api.divisions[0].ID
	h.app.chrome.tab = "Board tugas"
	h.app.router.Push(PathSpaces)
	task := workspaceui.Task{ID: id.New(), Title: "Tugas untuk uji panel", Lane: workspaceui.Todo}
	if err := h.app.workspace.SetTasks([]workspaceui.Task{task}); err != nil {
		t.Fatal(err)
	}
	h.tt.Advance(400 * time.Millisecond)
	h.click("Tugas " + task.Title)
	h.want("Detail tugas")
	h.click("Penampilan")
	h.want("Tutup penampilan")
	h.dontWant("Detail tugas")
	if h.app.workspace.SelectedID() != id.Nil {
		t.Fatal("appearance retained the task inspector")
	}
	h.click("Tugas " + task.Title)
	h.want("Detail tugas")
	h.dontWant("Tutup penampilan")
	if h.app.chrome.appearance {
		t.Fatal("task inspector left appearance open")
	}
	board := chromeRect(t, h.tt, "Kolom board")
	if board.W < 200 {
		t.Fatalf("board became unusably narrow: %+v", board)
	}
}
