package appui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// openFullAppearance opens the full appearance panel through the sidebar's
// popover, as a user does.
func openFullAppearance(h *harness) {
	h.t.Helper()
	h.click("Tampilan")
	h.click("Semua pengaturan tampilan…")
	if !h.app.chrome.appearance {
		h.t.Fatal("full appearance panel did not open")
	}
}

func TestPersonInitialsSkipDemoTags(t *testing.T) {
	for name, want := range map[string]string{"[DUMMY] Aruna Desain": "AD", "[DUMMY] Nara": "N", "Demo Mediatek": "DM", "": ""} {
		if got := personInitials(name); got != want {
			t.Errorf("personInitials(%q) = %q; want %q", name, got, want)
		}
	}
}

func TestSpaceInitialsFollowCanvasTiles(t *testing.T) {
	med := api.Division{ID: id.New(), Name: "Mediatek"}
	hum := api.Division{ID: id.New(), Name: "Humas"}
	for _, tc := range []struct {
		s    api.Space
		want string
	}{
		{api.Space{Name: "Mediatek", Kind: api.SpaceDivision, Divisions: []api.Division{med}}, "M"},
		{api.Space{Name: "Humas", Kind: api.SpaceDivision, Divisions: []api.Division{hum}}, "H"},
		{api.Space{Name: "Medhum", Kind: api.SpaceShared, Divisions: []api.Division{hum, med}}, "MH"},
		{api.Space{Name: "[uji] ruang", Kind: api.SpaceDivision}, "U"},
	} {
		if got := spaceInitials(tc.s); got != tc.want {
			t.Errorf("spaceInitials(%q) = %q; want %q", tc.s.Name, got, tc.want)
		}
	}
}

// The sidebar is canvas A's: a flat Space list with tiles and counts, no
// repeated Space tabs, ORGANISASI, and a footer with Pengaturan, Tampilan
// and Ciutkan. The user card names the role, not the email.
func TestSidebarMatchesCanvas(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(1440, 900)
	sp := testAuthorizedSpace(h.app, h.api.divisions[0])
	h.app.access.spaces[0].Roles[0].Roles = []string{api.RoleKadiv}
	shared := api.Space{ID: id.New(), PeriodID: h.app.periodID, Name: "Medhum", Kind: api.SpaceShared, Divisions: h.api.divisions[:2]}
	h.app.access.spaces = append(h.app.access.spaces, shared)
	h.app.tasks = tasksUIState{scope: sp.ID, loaded: true, items: []api.Task{
		{ID: id.New(), Title: "A", Progress: api.TaskTodo}, {ID: id.New(), Title: "B", Progress: api.TaskDoing}, {ID: id.New(), Title: "C", Progress: api.TaskDone},
	}}
	h.app.router.Push(PathSpaces)
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	for _, s := range []string{"Space", "Ruang Kerja HIMASIF", "PRIBADI", "SPACE", "ORGANISASI", "Evaluasi SOP", "Arsip", "Pengaturan", "Tampilan", "Ciutkan", "Ctrl B", "bersama", "Kadiv · " + sp.Name} {
		h.want(s)
	}
	if !h.tt.HasText("2") {
		t.Errorf("active task count missing: %q", h.tt.Texts())
	}
	for _, s := range []string{"space.", "Space saya", "Arsip periode", "Penampilan", h.api.user.Email, "Tersinkron · terakhir", "Sinkronisasi", "Esc · Tutup menu"} {
		if _, ok := h.tt.Find(s); ok {
			t.Errorf("old chrome %q still shown", s)
		}
	}
	h.click("Pengaturan")
	if h.app.router.Path() != PathPengaturan {
		t.Fatal("Pengaturan did not open the settings page")
	}
	h.click("Medhum")
	if h.app.selectedSpaceID() != shared.ID || h.app.router.Path() != PathSpaces {
		t.Fatal("a Space row did not open its Space")
	}
	h.tt.Key(ui.Ctrl, ui.KeyB)
	h.tt.Advance(300 * time.Millisecond)
	if !h.app.chrome.preferences.SidebarCollapsed {
		t.Fatal("Ctrl+B did not collapse the sidebar")
	}
	h.want("Buka sidebar")
	h.click("Buka sidebar")
	if h.app.chrome.preferences.SidebarCollapsed {
		t.Fatal("the collapsed rail did not open again")
	}
}

// The topbar holds the breadcrumb, the sync pill and "N anggota"; the
// avatars skip the "[DUMMY]" tag of demo names.
func TestTopbarMatchesCanvas(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(1440, 900)
	sp := testAuthorizedSpace(h.app, h.api.divisions[0])
	people := []api.PersonRef{{ID: id.New(), DisplayName: "[DUMMY] Aruna Desain"}, {ID: id.New(), DisplayName: "[DUMMY] Nara Tekno"}, {ID: id.New(), DisplayName: "Demo Mediatek"}}
	h.app.chrome.members = headerMembersState{space: sp.ID, loaded: true, items: people}
	h.app.router.Push(PathSpaces)
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	p := h.app.period()
	for _, s := range []string{"Space · Periode " + p.Label, "3 anggota", "AD", "NT", "DM", "Tersinkron"} {
		h.want(s)
	}
	for _, s := range []string{"[A", "[N"} {
		if h.tt.HasText(s) {
			t.Errorf("initials still show %q", s)
		}
	}
	h.app.router.Push(PathJobdesk)
	h.tt.Frame()
	if _, ok := h.tt.Find("Periode " + p.Label); !ok {
		t.Errorf("personal page breadcrumb missing: %q", h.tt.Texts())
	}
}

// Escape closes the appearance popover and the full panel.
func TestEscapeClosesAppearance(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.click("Tampilan")
	h.want("Kepadatan")
	h.tt.Key(0, ui.KeyEscape)
	h.tt.Advance(300 * time.Millisecond)
	if h.app.chrome.appearanceQuick {
		t.Fatal("Escape did not close the appearance popover")
	}
	openFullAppearance(h)
	h.tt.Key(0, ui.KeyEscape)
	h.tt.Advance(300 * time.Millisecond)
	if h.app.chrome.appearance {
		t.Fatal("Escape did not close the appearance panel")
	}
}

// Pengaturan › Perangkat ini › Tampilan holds the full appearance panel.
func TestSettingsHoldAppearance(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(1440, 900)
	h.app.router.Replace(PathPengaturan)
	h.tt.Frame()
	h.click("Tampilan dan tema")
	if h.app.chrome.settingsTab != "Tampilan" {
		t.Fatalf("settings tab %q", h.app.chrome.settingsTab)
	}
	h.want("Palet tema")
	h.want("Warna board")
}
