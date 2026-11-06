package appui

import (
	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSpaceCalendarAndOptionalClock(t *testing.T) {
	a := New(nil, nil, func(fn func()) { fn() })
	a.chrome.datePickers = map[string]*datePickerState{"deadline": {month: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}}
	date, clock := "2026-11-05", "15:30"
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(spaceTheme(true, "sand", false))
		ui.Column(c).Width(360).Padding(16).Gap(12).Children(func() {
			a.dateField(c, "deadline", "Tenggat tugas", &date, true, true)
			clockField(c, "deadline-time", "Jam tenggat", &clock, false)
		})
	}, 400, 650)
	if err := tt.Click("Tenggat tugas"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Tanggal 2026-11-07"); err != nil {
		t.Fatal(err)
	}
	if date != "2026-11-07" {
		t.Fatalf("calendar did not select date: %s", date)
	}
	if err := tt.Click("Lewati Jam tenggat"); err != nil {
		t.Fatal(err)
	}
	if clock != "" || date != "2026-11-07" {
		t.Fatalf("skipping hour changed date: %s %s", date, clock)
	}
}

func TestFeedbackPaletteRetainsCustomAccentAndSidebarShortcut(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.app.chrome.preferences.Accent = "#123456"
	h.click("Penampilan")
	h.click("Palet Midnight")
	if p := h.app.chrome.preferences; p.Palette != "midnight" || p.Accent != "#123456" {
		t.Fatalf("palette replaced accent: %+v", p)
	}
	if before := h.app.chrome.preferences.SidebarCollapsed; before {
		t.Fatal("unexpected initial collapse")
	}
	h.tt.Key(ui.Ctrl, ui.KeyB)
	h.tt.Advance(300 * time.Millisecond)
	if !h.app.chrome.preferences.SidebarCollapsed {
		t.Fatal("Ctrl+B did not collapse sidebar")
	}
}

// Capture the real shared native UI, not HTML artifacts. Optional output is
// for visual review; interaction and geometry assertions still run normally.
func TestDesktopFeedbackNativeCapture(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(1440, 1000)
	sp := testAuthorizedSpace(h.app, h.api.divisions[0])
	h.app.router.Push(PathSpaces)
	h.app.chrome.tab = "Board tugas"
	people := []api.PersonRef{{ID: id.New(), DisplayName: "Naura Yesha"}, {ID: id.New(), DisplayName: "Tiyo Ahimsa"}, {ID: id.New(), DisplayName: "Oktavia"}}
	h.app.chrome.members = headerMembersState{space: sp.ID, loaded: true, items: people}
	h.app.tasks = tasksUIState{scope: sp.ID, loaded: true, items: []api.Task{{ID: id.New(), Title: "Desain kegiatan", Category: "Desain", Description: "Siapkan desain sesuai konsep rapat.", Progress: api.TaskTodo, Assignee: people[0]}}}
	h.app.chrome.preferences.LaneColors = [5]string{"#dca471", "#a2bbd2", "#b6a8d0", "#a8c2ac"}
	h.app.chrome.preferences.ColorCardsByTag = true
	capture := func(name string) {
		h.tt.Frame()
		h.tt.Advance(350 * time.Millisecond)
		if dir := os.Getenv("SPACE_FEEDBACK_CAPTURE_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			f, err := os.Create(filepath.Join(dir, name+".png"))
			if err != nil {
				t.Fatal(err)
			}
			if err = png.Encode(f, h.tt.Image()); err != nil {
				t.Fatal(err)
			}
			if err = f.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
	capture("board")
	h.click("Anggota Space")
	capture("members-dropdown")
	trigger := chromeRect(t, h.tt, "Anggota Space")
	person := chromeRect(t, h.tt, "Naura Yesha")
	if person.Y < trigger.Y+trigger.H || person.Y > trigger.Y+trigger.H+110 {
		t.Fatalf("members not immediately below trigger: %+v %+v", trigger, person)
	}
	h.tt.Key(0, ui.KeyEscape)
	h.click("Penampilan")
	capture("appearance")
	scroll := chromeRect(t, h.tt, "Scroll penampilan")
	h.tt.Scroll(scroll.X+scroll.W/2, scroll.Y+scroll.H/2, 0, 10000)
	capture("appearance-board-colors")
	h.click("Tutup penampilan")
	h.app.tasks.draft = &taskDraft{owner: sp.Divisions[0].ID, title: "Poster kegiatan", output: "Desain siap posting", plannedStart: "2026-11-02", deadline: "2026-11-05", shared: map[id.ID]bool{}}
	capture("task-drawer")
	h.click("Jadwal tugas")
	capture("task-range-calendar")
	h.click("Selesai")
	h.app.tasks.draft = nil
	h.app.chrome.tab = "Agenda"
	h.app.meetings = meetingsState{space: sp.ID, loaded: true, draft: &meetingDraft{title: "Rapat koordinasi", date: "2026-11-05", start: "19:00", zone: "Asia/Jakarta", minutes: "60,15"}}
	capture("meeting-drawer")
	if !h.tt.HasText("Mulai rapat *") || h.tt.HasText("Jam selesai *") {
		t.Fatal("meeting required labels incorrect")
	}
	h.click("Mulai rapat")
	capture("meeting-calendar")
	h.app.chrome.preferences.Theme = "light"
	capture("meeting-calendar-light")
}

func TestFeedbackRangeSelectionKeepsPostingAndSkipKeepsDates(t *testing.T) {
	a := New(nil, nil, func(fn func()) { fn() })
	d := &taskDraft{posting: "2026-11-12"}
	a.chrome.datePickers = map[string]*datePickerState{"task-range": {month: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), selectStart: true}}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(spaceTheme(true, "sand", false))
		ui.Column(c).Width(380).Padding(16).Children(func() { a.taskSchedule(c, d) })
	}, 400, 750)
	for _, label := range []string{"Pilih mulai pengerjaan", "Tanggal 2026-11-02", "Tanggal 2026-11-05", "+15 menit task-start-time", "Lewati Jam mulai"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if d.plannedStart != "2026-11-02" || d.deadline != "2026-11-05" || d.plannedStartTime != "" || d.posting != "2026-11-12" {
		t.Fatalf("range or skip conflated dates: %+v", d)
	}
}
