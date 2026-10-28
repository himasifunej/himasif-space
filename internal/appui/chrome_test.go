package appui

import (
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/workspaceui"
)

type fakePreferences struct {
	value            Preferences
	loadErr, saveErr error
	loads, saves     int
}

func (s *fakePreferences) Load() (Preferences, error) {
	s.loads++
	return s.value, s.loadErr
}

func (s *fakePreferences) Save(p Preferences) error {
	s.saves++
	if s.saveErr != nil {
		return s.saveErr
	}
	s.value = p
	return nil
}

// Set the preference store before the first rendered frame, as main does.
func newChromeHarness(t *testing.T, preferences PreferencesStore) *harness {
	t.Helper()
	return newChromeHarnessAfterView(t, preferences, nil)
}

func newChromeHarnessAfterView(t *testing.T, preferences PreferencesStore, after func(*ui.Context)) *harness {
	t.Helper()
	f := newFake(api.AccountPerson)
	f.valid["tok"] = true
	h := &harness{t: t, api: f, tokens: &memTokens{token: "tok"}, queue: make(chan func(), 64)}
	h.app = New(f, h.tokens, func(fn func()) { h.queue <- fn })
	h.app.PreferencesStore = preferences
	h.app.ServerURL = "https://api.contoh.test"
	h.app.Version = "0.1.0-test"
	h.tt = ui.NewTester(func(c *ui.Context) {
		h.app.View(c)
		if after != nil {
			after(c)
		}
	}, 1300, 900)
	h.app.Start()
	h.settle()
	return h
}

func TestChromeSearchShortcutAndEscape(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.Key(ui.Ctrl, ui.KeyK)
	if !h.app.chrome.searchOpen || !h.tt.Focused("Cari di Space") {
		t.Fatalf("Ctrl K did not open and focus search: %+v", h.app.chrome)
	}
	h.want("Navigasi cepat")
	h.want("Tugas pada tampilan aktif; pencarian notes dan anggota belum tersedia.")
	h.tt.Type("halaman-yang-tidak-ada")
	h.want("Tidak ada halaman yang cocok.")
	if h.app.chrome.search != "halaman-yang-tidak-ada" {
		t.Fatalf("search did not receive keyboard input: %q", h.app.chrome.search)
	}
	h.tt.Key(0, ui.KeyEscape)
	if h.app.chrome.searchOpen {
		t.Fatal("Escape did not close the search dropdown")
	}
	h.dontWant("Navigasi cepat")
}

func TestChromeNotificationDropdownDismissal(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.click("Notifikasi")
	if !h.app.chrome.notificationsOpen {
		t.Fatal("notification dropdown did not open")
	}
	h.want("Layanan notifikasi belum tersedia.")
	h.click("Space · 0.1.0-test") // outside the anchored panel
	if h.app.chrome.notificationsOpen {
		t.Fatal("outside click did not close notifications")
	}
	h.dontWant("Layanan notifikasi belum tersedia.")
	h.click("Notifikasi")
	h.tt.Key(0, ui.KeyEscape)
	if h.app.chrome.notificationsOpen {
		t.Fatal("Escape did not close notifications")
	}
	h.dontWant("Layanan notifikasi belum tersedia.")

	h.tt.Key(ui.Ctrl, ui.KeyK)
	h.click("Notifikasi")
	if h.app.chrome.searchOpen || !h.app.chrome.notificationsOpen {
		t.Fatal("opening notifications did not replace the search dropdown")
	}
	h.click("Menu profil")
	if h.app.chrome.notificationsOpen || !h.app.chrome.profileOpen {
		t.Fatal("opening profile did not replace the notification dropdown")
	}
}

func TestChromeProfileSettingsAndLogout(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.click("Menu profil")
	h.want("Keluar akun")
	h.click("Pengaturan")
	if h.app.router.Path() != PathPengaturan || h.app.chrome.profileOpen {
		t.Fatal("profile settings action did not navigate and close the menu")
	}
	h.want("WhatsApp divisi")
	h.click("Akun")
	h.want("Akun dan aplikasi")
	h.want("ani@himasif.id")
	h.click("Menu profil")
	h.click("Keluar akun")
	h.settle()
	if h.app.screen != screenLogin || h.tokens.get() != "" || h.api.logouts != 1 || h.api.token != "" {
		t.Fatalf("profile logout did not end the real session: screen=%d token=%q logouts=%d clientToken=%q", h.app.screen, h.tokens.get(), h.api.logouts, h.api.token)
	}
	h.want("Masuk")
}

func TestChromeLoadsAppearanceOnce(t *testing.T) {
	want := Preferences{Theme: "light", Accent: "blue", Compact: true, SidebarCollapsed: true}
	store := &fakePreferences{value: want}
	h := newChromeHarness(t, store)
	h.tt.Frame()
	if store.loads != 1 || h.app.chrome.preferences != want {
		t.Fatalf("appearance loaded %d times with %+v; want once with %+v", store.loads, h.app.chrome.preferences, want)
	}
	h.want("Buka sidebar")
}

func TestChromeAppearanceLoadFailureIsVisible(t *testing.T) {
	store := &fakePreferences{value: DefaultPreferences(), loadErr: errors.New("unreadable config")}
	h := newChromeHarness(t, store)
	if h.app.chrome.preferencesErr == "" || h.app.chrome.preferences != DefaultPreferences() {
		t.Fatal("failed appearance load did not retain defaults and a notice")
	}
	h.click("Penampilan")
	h.want("Preferensi perangkat tidak dapat dibaca.")
}

func TestChromeAppearancePersistsChanges(t *testing.T) {
	store := &fakePreferences{value: DefaultPreferences()}
	h := newChromeHarness(t, store)
	h.click("Penampilan")
	h.click("Terang")
	if h.app.chrome.preferences.Theme != "light" || store.value.Theme != "light" || store.saves != 1 || h.app.chrome.preferencesErr != "" {
		t.Fatalf("theme was not persisted: preferences=%+v store=%+v saves=%d error=%q", h.app.chrome.preferences, store.value, store.saves, h.app.chrome.preferencesErr)
	}
	h.click("Aksen sage")
	if store.value.Accent != "sage" || store.saves != 2 {
		t.Fatalf("accent was not persisted: %+v; saves %d", store.value, store.saves)
	}
	h.click("Warna aksen khusus")
	h.tt.Type("#123456")
	h.click("Terapkan warna")
	if store.value.Accent != "#123456" || store.saves != 3 {
		t.Fatalf("custom accent was not persisted: %+v; saves %d", store.value, store.saves)
	}
	h.click("Tampilan ringkas")
	if !store.value.Compact || store.saves != 4 {
		t.Fatalf("density was not persisted: %+v; saves %d", store.value, store.saves)
	}
	h.click("Ciutkan sidebar")
	if !store.value.SidebarCollapsed || store.saves != 5 {
		t.Fatalf("sidebar collapse was not persisted: %+v; saves %d", store.value, store.saves)
	}
}

func TestChromeAppearanceSaveFailureAndRecovery(t *testing.T) {
	store := &fakePreferences{value: DefaultPreferences(), saveErr: errors.New("disk full")}
	h := newChromeHarness(t, store)
	h.click("Penampilan")
	h.click("Terang")
	if h.app.chrome.preferences.Theme != "light" || store.value.Theme != "dark" || h.app.chrome.preferencesErr == "" {
		t.Fatalf("failure did not distinguish applied and saved appearance: local=%+v stored=%+v error=%q", h.app.chrome.preferences, store.value, h.app.chrome.preferencesErr)
	}
	h.want("Preferensi belum tersimpan")
	h.want("Tampilan berubah untuk sesi ini, tetapi belum tersimpan di perangkat.")
	store.saveErr = nil
	h.click("Gelap")
	if store.saves != 2 || store.value.Theme != "dark" || h.app.chrome.preferencesErr != "" {
		t.Fatalf("appearance save did not recover: saves=%d stored=%+v error=%q", store.saves, store.value, h.app.chrome.preferencesErr)
	}
	h.tt.Advance(5 * time.Second)
	h.dontWant("Preferensi belum tersimpan")
}

func TestChromeSessionStateCleared(t *testing.T) {
	for _, ending := range []string{"logout", "expired"} {
		t.Run(ending, func(t *testing.T) {
			preferences := Preferences{Theme: "light", Accent: "rose", Compact: true}
			h := newChromeHarness(t, &fakePreferences{value: preferences})
			h.app.chrome.tab = "Notes"
			h.app.chrome.settingsTab = "Akun"
			h.app.chrome.search = "private search from previous session"
			h.app.chrome.customAccent = "#123456"
			h.app.chrome.divisionID = h.api.divisions[0].ID
			h.click("Penampilan")
			oldWorkspace := h.app.workspace
			taskID := id.New()
			if err := oldWorkspace.SetTasks([]workspaceui.Task{{ID: taskID, Title: "Private previous session task"}}); err != nil {
				t.Fatal(err)
			}
			oldWorkspace.Open(taskID)
			oldWorkspace.OnMove = func(workspaceui.MoveRequest) {}
			if ending == "logout" {
				h.click("Menu profil")
				h.click("Keluar akun")
			} else {
				h.api.mu.Lock()
				delete(h.api.valid, "tok")
				h.api.mu.Unlock()
				h.app.loadDivisions()
			}
			h.settle()
			state := h.app.chrome
			if state.search != "" || state.customAccent != "" || state.divisionID != id.Nil || state.appearance || state.periodOpen || state.profileOpen || state.notificationsOpen || state.searchOpen || state.tab != "Beranda" || state.settingsTab != "WhatsApp" {
				t.Fatalf("per-session chrome state survived %s: %+v", ending, state)
			}
			if state.preferences != preferences {
				t.Fatalf("device preferences lost on %s: %+v", ending, state.preferences)
			}
			if h.app.workspace == oldWorkspace || h.app.workspace.SelectedID() != id.Nil || h.app.workspace.OnMove != nil {
				t.Fatalf("previous session's workspace survived %s", ending)
			}
			if h.app.screen != screenLogin || h.tokens.get() != "" {
				t.Fatalf("session not cleared: screen=%d token=%q", h.app.screen, h.tokens.get())
			}
			h.login("ani@himasif.id", "rahasia")
			if h.app.screen != screenShell || h.app.router.Path() != PathJobdesk {
				t.Fatal("next session did not start at its own default page")
			}
			h.dontWant("Private previous session task")
		})
	}
}

func TestChromeBoardRouteDistinguishesUnloadedAndConfirmedSnapshots(t *testing.T) {
	h := newChromeHarness(t, nil)
	testAuthorizedSpace(h.app, h.api.divisions[0])
	h.app.router.Push(PathSpaces)
	h.app.chrome.tab = "Board tugas"
	h.tt.Frame()
	h.want("Layanan tugas belum tersedia.")
	h.dontWant("Board kosong")
	h.app.tasks = tasksUIState{scope: h.app.selectedSpaceID(), loaded: true, items: []api.Task{}}
	h.tt.Frame()
	h.want("Board kosong")
	h.want("0 tugas")
	h.dontWant("Tugas belum dapat dimuat")
	task := api.Task{ID: id.New(), Title: "Desain dari snapshot terkonfirmasi", Progress: api.TaskTodo, Assignee: api.PersonRef{DisplayName: "Ani Wulandari"}, Description: "Tautan hasil masih memerlukan tinjauan."}
	h.app.tasks.items = []api.Task{task}
	h.tt.Frame()
	h.want(task.Title)
	h.want("1 tugas")
	h.dontWant("Board kosong")
	before := chromeRect(t, h.tt, "Kolom board")
	h.click("Tugas " + task.Title)
	if h.app.workspace.SelectedID() != task.ID {
		t.Fatal("task did not open its inspector")
	}
	h.want(task.Description)
	panel := chromeRect(t, h.tt, "Detail tugas")
	after := chromeRect(t, h.tt, "Kolom board")
	if !(after.W < before.W && after.X+after.W <= panel.X+1 && panel.X > after.X) {
		t.Fatalf("inspector did not shrink board: before=%+v after=%+v inspector=%+v", before, after, panel)
	}
}

func TestChromePeriodChangeClearsBoardSnapshotAndAuthority(t *testing.T) {
	h := newChromeHarness(t, nil)
	testAuthorizedSpace(h.app, h.api.divisions[0])
	h.app.router.Push(PathSpaces)
	h.app.chrome.tab = "Board tugas"
	task := api.Task{ID: id.New(), Title: "Tugas privat periode sebelumnya", Progress: api.TaskTodo}
	h.app.tasks = tasksUIState{scope: h.app.selectedSpaceID(), loaded: true, items: []api.Task{task}}
	h.tt.Frame()
	h.click("Tugas " + task.Title)
	previous := h.app.workspace
	h.click("2025/2026 (aktif)")
	h.click("2024/2025 (ditutup)")
	h.settle()
	if period := h.app.period(); period == nil || period.ID != h.api.periods[0].ID {
		t.Fatalf("period selection failed: %+v", period)
	}
	current := h.app.workspace
	if current == nil || current == previous || current.HasSnapshot() || current.SelectedID() != id.Nil || current.OnMove != nil {
		t.Fatal("period retained task snapshot, inspector or authority")
	}
	if h.app.selectedSpace() != nil || len(h.app.access.spaces) != 0 || len(h.app.tasks.items) != 0 {
		t.Fatal("period retained old Space or canonical tasks")
	}
	h.dontWant(task.Title)
	h.dontWant("Detail tugas")
	h.dontWant("Board kosong")
	h.want("Layanan akses Space belum tersedia.")
}

func TestChromeSystemThemeAndExplicitOverrides(t *testing.T) {
	for _, mode := range []string{"system", "dark", "light"} {
		t.Run(mode, func(t *testing.T) {
			// Capture the effective theme after App.View without retaining Context
			// or relying on a pixel whose component styling might change later.
			var effectiveDark bool
			h := newChromeHarnessAfterView(t, &fakePreferences{value: Preferences{Theme: mode, Accent: "sand"}}, func(c *ui.Context) {
				effectiveDark = c.Theme().Dark
			})
			for _, systemDark := range []bool{false, true, false} {
				h.tt.SetDark(systemDark)
				wantDark := mode == "dark" || mode == "system" && systemDark
				if effectiveDark != wantDark {
					t.Fatalf("theme=%s systemDark=%v produced dark=%v; want %v", mode, systemDark, effectiveDark, wantDark)
				}
			}
		})
	}
}

func chromeRect(t *testing.T, tester *ui.Tester, label string) ui.Rect {
	t.Helper()
	bounds, ok := tester.Find(label)
	if !ok {
		t.Fatalf("missing element %q; texts %q", label, tester.Texts())
	}
	return bounds
}
