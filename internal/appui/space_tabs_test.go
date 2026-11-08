package appui

import (
	"slices"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func spaceTabTitles(tabs []spaceTabDef) []string {
	var out []string
	for _, tab := range tabs {
		out = append(out, tab.title)
	}
	return out
}

// newSpaceTabsHarness opens a division Space whose account may open every
// tab of canvas A.
func newSpaceTabsHarness(t *testing.T, store *fakePreferences) (*harness, api.Space) {
	t.Helper()
	h := newChromeHarness(t, store)
	h.tt.SetSize(1700, 900)
	d := h.api.divisions[0]
	caps := []string{api.CapSpaceRead, api.CapMembersRead, api.CapTasksOwn, api.CapResultsReview, api.CapPingSend}
	sp := api.Space{ID: id.New(), PeriodID: h.app.periodID, Name: d.Name, Kind: api.SpaceDivision, Divisions: []api.Division{d}, Roles: []api.Access{{DivisionID: d.ID, Roles: []string{api.RoleMember}, Capabilities: caps}}, Capabilities: caps}
	h.app.access.spaces = []api.Space{sp}
	h.app.selectSpace(sp.ID)
	h.app.chrome.tab = "Beranda"
	h.app.router.Push(PathSpaces)
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	return h, sp
}

func TestSpaceTabsFollowCanvasOrder(t *testing.T) {
	h, sp := newSpaceTabsHarness(t, &fakePreferences{})
	want := []string{"Beranda", "Board tugas", "Agenda", "Catatan", "Rapat", "Tinjauan", "Ping", "Anggota", "Bot WhatsApp"}
	if got := spaceTabTitles(h.app.spaceTabsVisible(&sp)); !slices.Equal(got, want) {
		t.Fatalf("tabs %q, want %q", got, want)
	}
	if _, ok := h.tt.Find("Atur tab"); !ok {
		t.Fatal("no Atur tab button")
	}

	// An account without review, ping or bot rights does not see those tabs.
	limited := sp
	limited.Capabilities = []string{api.CapSpaceRead, api.CapMembersRead}
	limited.Roles = []api.Access{{DivisionID: sp.Divisions[0].ID, Capabilities: limited.Capabilities}}
	if got := spaceTabTitles(h.app.spaceTabsVisible(&limited)); !slices.Equal(got, []string{"Beranda", "Board tugas", "Agenda", "Catatan", "Rapat", "Anggota"}) {
		t.Fatalf("limited tabs %q", got)
	}
}

func TestSpaceTabClickSwitchesTab(t *testing.T) {
	h, _ := newSpaceTabsHarness(t, &fakePreferences{})
	h.click("Tinjauan")
	h.tt.Frame()
	if h.app.chrome.tab != "Tinjauan" {
		t.Fatalf("tab %q after clicking Tinjauan", h.app.chrome.tab)
	}
	h.want("Tinjauan hasil")
	h.click("Bot WhatsApp")
	h.tt.Frame()
	if h.app.chrome.tab != "Bot WhatsApp" {
		t.Fatalf("tab %q after clicking Bot WhatsApp", h.app.chrome.tab)
	}
	h.want("WhatsApp divisi")
	h.app.chrome.tab = "Ping"
	h.tt.Frame()
	h.want("Riwayat ping belum tersedia")
	h.dontWant("Urutan tab disimpan")
}

func TestSpaceTabLegacyTitles(t *testing.T) {
	h, _ := newSpaceTabsHarness(t, &fakePreferences{})
	h.app.chrome.tab = "Notes"
	h.tt.Frame()
	if h.app.chrome.tab != "Catatan" {
		t.Fatalf("Notes opened %q", h.app.chrome.tab)
	}
	h.app.chrome.tab = "Pengaturan"
	h.tt.Frame()
	if h.app.chrome.tab != "Bot WhatsApp" {
		t.Fatalf("Pengaturan opened %q", h.app.chrome.tab)
	}
}

func TestSpaceTabDragReordersAndPersists(t *testing.T) {
	store := &fakePreferences{}
	h, sp := newSpaceTabsHarness(t, store)
	from, ok := h.tt.Find("Rapat")
	if !ok {
		t.Fatal("no Rapat tab")
	}
	to, ok := h.tt.Find("Tinjauan")
	if !ok {
		t.Fatal("no Tinjauan tab")
	}
	y := from.Y + from.H/2
	x0, x1 := from.X+from.W/2, to.X+to.W/2
	h.tt.Press(x0, y)
	h.tt.Frame()
	for i := 1; i <= 8; i++ {
		h.tt.Move(x0+(x1-x0)*float32(i)/8, y)
		h.tt.Frame()
	}
	h.tt.Release(x1, y)
	h.tt.Advance(100 * time.Millisecond)
	h.tt.Frame()
	if h.app.chrome.tab != "Beranda" {
		t.Fatalf("drag switched the tab to %q", h.app.chrome.tab)
	}
	want := []string{"Beranda", "Board tugas", "Agenda", "Catatan", "Tinjauan", "Rapat", "Ping", "Anggota", "Bot WhatsApp"}
	if got := spaceTabTitles(h.app.spaceTabsVisible(&sp)); !slices.Equal(got, want) {
		t.Fatalf("tabs after drag %q, want %q", got, want)
	}
	h.want("Urutan tab disimpan")
	saved := (*store.value.SpaceTabs)[h.app.user.ID.String()+"/"+sp.ID.String()]
	if store.saves == 0 || saved.Order[5] != "rapat" {
		t.Fatalf("order not saved: saves=%d %+v", store.saves, saved)
	}

	// A new session on this device reads the order back.
	again, sp2 := newSpaceTabsHarness(t, &fakePreferences{value: store.value})
	again.app.user.ID = h.app.user.ID
	sp2.ID = sp.ID
	if got := spaceTabTitles(again.app.spaceTabsVisible(&sp2)); !slices.Equal(got, want) {
		t.Fatalf("tabs after reload %q, want %q", got, want)
	}
}

func TestSpaceTabMenuHidesAndShowsTabs(t *testing.T) {
	store := &fakePreferences{}
	h, sp := newSpaceTabsHarness(t, store)
	h.click("Atur tab")
	h.want("Tampilkan tab · geser tab untuk mengubah urutan")
	h.click("Tampilkan Ping")
	h.tt.Frame()
	key := h.app.user.ID.String() + "/" + sp.ID.String()
	if !slices.Contains((*store.value.SpaceTabs)[key].Hidden, "ping") {
		t.Fatalf("Ping not saved hidden: %+v", store.value.SpaceTabs)
	}
	if slices.Contains(spaceTabTitles(h.app.spaceTabsVisible(&sp)), "Ping") {
		t.Fatal("hidden Ping still in the bar")
	}
	// Beranda cannot be hidden.
	h.click("Tampilkan Beranda")
	h.tt.Frame()
	if slices.Contains((*store.value.SpaceTabs)[key].Hidden, "beranda") || !slices.Contains(spaceTabTitles(h.app.spaceTabsVisible(&sp)), "Beranda") {
		t.Fatal("Beranda was hidden")
	}
	h.click("Tampilkan Ping")
	h.tt.Frame()
	if slices.Contains((*store.value.SpaceTabs)[key].Hidden, "ping") || !slices.Contains(spaceTabTitles(h.app.spaceTabsVisible(&sp)), "Ping") {
		t.Fatal("Ping not shown again")
	}
}

func TestMoveSpaceTabKey(t *testing.T) {
	order := []string{"a", "b", "c", "d"}
	for _, tc := range []struct {
		from, to string
		want     []string
	}{
		{"a", "c", []string{"b", "c", "a", "d"}},
		{"d", "b", []string{"a", "d", "b", "c"}},
		{"a", "d", []string{"b", "c", "d", "a"}},
		{"b", "b", order},
		{"x", "b", order},
	} {
		if got := moveSpaceTabKey(order, tc.from, tc.to); !slices.Equal(got, tc.want) {
			t.Errorf("move %s onto %s: %q, want %q", tc.from, tc.to, got, tc.want)
		}
	}
	tabs := []spaceTabDef{{key: "a"}, {key: "b"}, {key: "c"}, {key: "d"}}
	got := orderSpaceTabs(tabs, []string{"c", "a", "gone"})
	var keys []string
	for _, tab := range got {
		keys = append(keys, tab.key)
	}
	if !slices.Equal(keys, []string{"c", "d", "a", "b"}) {
		t.Fatalf("ordered %q", keys)
	}
}
