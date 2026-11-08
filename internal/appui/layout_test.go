package appui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Side navigation stays fixed and every session control remains reachable at
// the window's minimum supported size, including a selected division.
func TestShellFitsMinimumWindow(t *testing.T) {
	h := newHarness(t, newFake(api.AccountPerson), "")
	h.app.Start()
	h.settle()
	h.login(h.api.user.Email, h.api.password)
	testAuthorizedSpace(h.app, h.api.divisions[0])
	h.app.router.Push(PathSpaces)
	for _, size := range [][2]int{{1440, 900}, {1100, 720}, {880, 640}} {
		h.tt.SetSize(size[0], size[1])
		h.tt.Frame()
		sidebarToggle := "Ciutkan"
		if size[0] < tabletMaxWidth {
			// Tablets show the icon rail, whose toggle opens the sidebar.
			sidebarToggle = "Buka sidebar"
		}
		for _, label := range []string{"Menu profil", "Tampilan", sidebarToggle, "Tersambung", "Notifikasi", "Cari di Space"} {
			r, ok := h.tt.Find(label)
			if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > float32(size[0])+1 || r.Y+r.H > float32(size[1])+1 {
				t.Errorf("%dx%d: %s out of window: %+v, exists=%v", size[0], size[1], label, r, ok)
			}
		}
	}
}

func TestShellFitsWithCustomTitleBar(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(880, 640)
	h.tt.SetTitleBar(ui.TitleBar{Height: 32, Right: 138})
	h.tt.Frame()
	for _, label := range []string{"Menu profil", "Tersambung", "Notifikasi"} {
		r, ok := h.tt.Find(label)
		if !ok || r.Y < 32 || r.Y+r.H > 640 {
			t.Fatalf("%s outside content area with title bar: %+v exists=%v", label, r, ok)
		}
	}
}

func TestSettingsTabsReachableAtDesktopWidths(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.app.router.Replace(PathPengaturan)
	h.app.DeviceSettings = func(c *ui.Context) { ui.Text(c, "Perangkat, notifikasi, dan Ace") }
	for _, size := range [][2]int{{1440, 900}, {880, 1000}, {390, 1000}} {
		h.tt.SetSize(size[0], size[1])
		for _, tab := range []struct{ name, label, heading string }{
			{"Sinkronisasi", "Offline dan sinkronisasi", "Sinkronisasi perangkat"}, {"Darurat", "Spreadsheet darurat", "Salinan darurat dan rekonsiliasi"},
			{"Status layanan", "Status layanan", "Status layanan"}, {"Perangkat", "Notifikasi, suara, dan Ace", "Perangkat, notifikasi, dan Ace"},
			{"WhatsApp", "Tautan dan bot WhatsApp", "WhatsApp divisi"}, {"Pengingat", "Pengingat dan ping", "Pengingat dan ping"},
			{"Command", "Command bot", "Command bot"}, {"Aktivitas", "Aktivitas WhatsApp", "Aktivitas WhatsApp"}, {"Akun", "Profil dan keamanan", "Akun dan aplikasi"},
		} {
			h.tt.Frame()
			if size[0] < 900 && h.app.chrome.settingsDetail {
				// Narrow windows list the settings and open one at a time.
				if err := h.tt.Click("Semua pengaturan"); err != nil {
					t.Fatalf("%dx%d: back: %v", size[0], size[1], err)
				}
				h.tt.Frame()
			}
			if err := h.tt.Click(tab.label); err != nil {
				t.Fatalf("%dx%d: tab %s: %v", size[0], size[1], tab.name, err)
			}
			h.tt.Frame()
			if h.app.chrome.settingsTab != tab.name || !h.tt.HasText(tab.heading) {
				t.Fatalf("%dx%d: tab %s did not open %s", size[0], size[1], tab.name, tab.heading)
			}
			if size[0] == 1440 {
				h.tt.Advance(500 * time.Millisecond)
				captureNativeUI(t, h, "settings-"+tab.name)
			}
		}
	}
}

func TestNotificationSettingsFieldsHaveVisibleLabels(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(1440, 900)
	s := testAuthorizedSpace(h.app, h.api.divisions[0])
	s.Roles[0].Capabilities = append(s.Roles[0].Capabilities, api.CapTasksManage, api.CapPingSend)
	s.Capabilities = append(s.Capabilities, api.CapTasksManage, api.CapPingSend)
	h.app.access.spaces[0] = s
	h.app.chrome.settingsTab = "Pengingat"
	h.app.router.Replace(PathPengaturan)
	h.tt.Advance(500 * time.Millisecond)
	h.tt.Frame()
	for _, label := range []string{"Hari sebelum tenggat · pisahkan koma", "Jam lokal · 0–23", "Menit · 0–59", "Overdue setiap berapa hari · 1–30"} {
		count := 0
		for _, text := range h.tt.Texts() {
			if text == label {
				count++
			}
		}
		if count < 2 {
			t.Fatalf("field %q should have a visible label and an accessible input: %v", label, h.tt.Texts())
		}
	}
	captureNativeUI(t, h, "settings-pengingat-labels")
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
	testAuthorizedSpace(h.app, h.api.divisions[0])
	h.app.router.Push(PathSpaces)
	capture("overview-dark")
	h.click("Notifikasi")
	capture("notifications")
	h.click("Notifikasi")
	h.click("Tampilan")
	capture("appearance")
	h.click("Terang")
	capture("overview-light")
	h.click("Tutup tampilan")
	h.app.chrome.tab = "Board tugas"
	taskID := id.New()
	h.app.tasks = tasksUIState{scope: h.app.selectedSpaceID(), loaded: true, items: []api.Task{
		{ID: taskID, Title: "Desain Lesson Learned", Description: "Sesuaikan desain dengan konsep Humas.", Category: "Desain", Assignee: api.PersonRef{DisplayName: "Ani Wulandari"}, ExpectedOutput: "Link Drive berisi desain siap posting.", Progress: api.TaskDoing},
		{ID: id.New(), Title: "Konsep Mingper 11", Category: "Konten", Assignee: api.PersonRef{DisplayName: "PJ Humas"}, Progress: api.TaskTodo},
	}}

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

func TestTaskModalAndAppearanceAreExclusiveAtMinimumWidth(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(880, 640)
	testAuthorizedSpace(h.app, h.api.divisions[0])
	h.app.chrome.tab = "Board tugas"
	h.app.router.Push(PathSpaces)
	task := api.Task{ID: id.New(), Title: "Tugas untuk uji panel", Progress: api.TaskTodo}
	h.app.tasks = tasksUIState{scope: h.app.selectedSpaceID(), loaded: true, items: []api.Task{task}}

	h.tt.Advance(400 * time.Millisecond)
	h.click("Tugas " + task.Title)
	h.want("Detail tugas")
	h.click("Tutup detail tugas")
	h.dontWant("Detail tugas")
	if h.app.workspace.SelectedID() != id.Nil {
		t.Fatal("closing the modal kept the task selected")
	}
	openFullAppearance(h)
	h.want("Tutup panel tampilan")
	h.click("Tugas " + task.Title)
	h.want("Detail tugas")
	if h.app.chrome.appearance {
		t.Fatal("task modal left appearance open")
	}
	board := chromeRect(t, h.tt, "Kolom board")
	if board.W < 200 {
		t.Fatalf("board became unusably narrow: %+v", board)
	}
}

func TestCreateTaskPanelResizesBoard(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(1440, 900)
	testAuthorizedSpace(h.app, h.api.divisions[0])
	h.app.chrome.tab = "Board tugas"
	h.app.router.Push(PathSpaces)
	h.app.tasks = tasksUIState{scope: h.app.selectedSpaceID(), loaded: true}
	h.tt.Advance(500 * time.Millisecond)
	before := chromeRect(t, h.tt, "Kolom board")
	laneBefore := chromeRect(t, h.tt, "Kolom Belum mulai")
	h.app.tasks.draft = &taskDraft{}
	h.tt.Frame()
	h.tt.Advance(500 * time.Millisecond)
	board := chromeRect(t, h.tt, "Kolom board")
	laneAfter := chromeRect(t, h.tt, "Kolom Belum mulai")
	if laneAfter.W >= laneBefore.W-10 {
		t.Fatalf("lane should shrink with board: before=%+v after=%+v", laneBefore, laneAfter)
	}
	panel := chromeRect(t, h.tt, "Tutup tugas baru")
	if board.W >= before.W || board.X+board.W > panel.X || board.W < 200 {
		t.Fatalf("board should shrink beside the creation panel: before=%+v board=%+v panel=%+v", before, board, panel)
	}
	done := chromeRect(t, h.tt, "Kolom Selesai")
	if done.X+done.W > board.X+board.W+1 {
		t.Fatalf("all four columns should fit when the board has enough room: board=%+v done=%+v", board, done)
	}
}

func TestPhoneShellUsesBottomTabsAndFullScreenPanels(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(390, 844)
	testAuthorizedSpace(h.app, h.api.divisions[0])
	h.app.router.Push(PathJobdesk)
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	if h.tt.HasText("Ciutkan") || h.tt.HasText("Cari di Space") {
		t.Fatal("phone layout still shows the desktop sidebar or header search")
	}
	for _, label := range []string{"Beranda", "Jobdesk", "Board", "Medhum", "Lainnya", "Notifikasi", "Menu profil"} {
		r, ok := h.tt.Find(label)
		if !ok || r.X < 0 || r.X+r.W > 391 || r.Y+r.H > 845 {
			t.Fatalf("%s missing or outside the phone: %+v exists=%v", label, r, ok)
		}
	}
	h.click("Lainnya")
	h.tt.Frame()
	h.want("Reminder pribadi")
	h.want("Pengaturan")
	h.click("Board")
	h.tt.Frame()
	if h.app.chrome.tab != "Board tugas" || h.app.router.Path() != PathSpaces {
		t.Fatalf("Board tab opened %q on %s", h.app.chrome.tab, h.app.router.Path())
	}
}
