package appui

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Pilot 0.4.4: the tab row hides the engine's scroll bar, which showed
// over the tabs while the pointer was on them; a mouse wheel scrolls it
// sideways, and a thin bar shows only while it scrolls.
func TestSpaceTabRowScrollsWithoutStandingBar(t *testing.T) {
	h, _ := newSpaceTabsHarness(t, &fakePreferences{})
	h.tt.SetSize(700, 800)
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	st := &h.app.spaceTabs
	if st.scroll.MaxX <= 0 {
		t.Fatalf("tabs fit in 700 DIPs: %+v", st.scroll)
	}
	r, ok := h.tt.Find("Board tugas")
	if !ok {
		t.Fatal("no Board tugas tab")
	}
	h.tt.Move(r.X+r.W/2, r.Y+r.H/2)
	h.tt.Advance(100 * time.Millisecond)
	if !st.scrolledAt.IsZero() {
		t.Fatal("bar shown without scrolling")
	}
	h.tt.Wheel(r.X+r.W/2, r.Y+r.H/2, 0, 120)
	h.tt.Advance(100 * time.Millisecond)
	if st.scroll.X <= 0 || st.scrolledAt.IsZero() {
		t.Fatalf("wheel did not scroll the tab row: %+v", st.scroll)
	}
	h.tt.Advance(2 * time.Second)
	if h.tt.Now().Sub(st.scrolledAt) < spaceTabShowBar {
		t.Fatal("bar still showing")
	}
}

// openMemberAccessMenu opens a member's access drawer as a user does now:
// the row's ⋯, then Ubah role & akses.
func openMemberAccessMenu(h *harness, name string) {
	h.t.Helper()
	h.click("Lainnya untuk " + name)
	h.tt.Frame()
	h.click("Ubah akses " + name)
}

// The Anggota rows keep Ping and ⋯ inside the table, with Ubah role & akses
// in the ⋯ menu instead of beside them.
func TestMemberRowActionsFitAndMenu(t *testing.T) {
	for _, w := range []int{1280, 1440} {
		h, _, _, _ := memberAccessHarness(t, w, 900)
		more, ok := h.tt.Find("Lainnya untuk Nara Tekno")
		if !ok {
			t.Fatal("no ⋯ on Nara's row")
		}
		head, ok := h.tt.Find("WhatsApp")
		if !ok {
			t.Fatal("no WhatsApp column")
		}
		// The last column starts 16 DIPs after WhatsApp (112 wide); the
		// row's padding (16) is left after it.
		if right := more.X + more.W; right > head.X+112+16+104+0.5 {
			t.Fatalf("width %d: ⋯ ends at %v, past its column", w, right)
		}
		h.dontWant("Ubah akses Nara Tekno")
		h.click("Lainnya untuk Nara Tekno")
		h.tt.Frame()
		for _, s := range []string{"Ubah role & akses", "Lihat tugas", "Keluarkan…"} {
			h.want(s)
		}
		h.click("Ubah akses Nara Tekno")
		h.tt.Frame()
		h.want("Ubah akses · Nara Tekno")
	}
}

// roleFixture adds the custom role writes to the access drawer's fixture.
type roleFixture struct {
	*memberAccessFixture
	created []api.RoleRequest
	deleted []id.ID
}

func (f *roleFixture) CreateRole(_ context.Context, _, d id.ID, req api.RoleRequest) (api.CustomRole, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := api.CustomRole{ID: id.New(), DivisionID: d, Name: req.Name, Capabilities: req.Capabilities}
	f.created = append(f.created, req)
	f.roles = append(f.roles, r)
	return r, nil
}

func (f *roleFixture) DeleteRole(_ context.Context, _, _, rid id.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, rid)
	f.roles = slices.DeleteFunc(f.roles, func(r api.CustomRole) bool { return r.ID == rid })
	return nil
}

// Custom roles moved off the Anggota page into their own dialog: an
// example fills the name and the permissions, which are cards with icons;
// deleting asks first, inside the dialog.
func TestCustomRoleDialog(t *testing.T) {
	h, base, _, _ := memberAccessHarness(t, 1440, 900)
	f := &roleFixture{memberAccessFixture: base}
	h.app.api = f
	h.dontWant("Nama role kustom")
	h.dontWant("Buat role")
	h.want("Editor web") // in the Kewenangan role card
	h.click("Role kustom")
	h.tt.Frame()
	h.want("Dialog role kustom")
	h.want("Baca evaluasi SOP")
	h.click("Buat role baru")
	h.tt.Frame()
	h.want("Role baru")
	h.want("Beri nama role dulu")
	h.click("Contoh Notulis")
	h.tt.Frame()
	h.want("1 izin dipilih")
	h.want("Menulis dan merapikan catatan rapat.")
	h.click("Kirim ping")
	h.tt.Frame()
	h.want("2 izin dipilih")
	h.click("Buat role")
	h.settle()
	h.settle()
	if len(f.created) != 1 || f.created[0].Name != "Notulis" || !slices.Equal(f.created[0].Capabilities, []string{api.CapNotesManage, api.CapPingSend}) {
		t.Fatalf("created %+v", f.created)
	}
	// Back on the list, the dialog still open, with the new role.
	if !h.app.access.members.roleDialog.open || h.app.access.members.roleDialog.editing {
		t.Fatalf("dialog %+v", h.app.access.members.roleDialog)
	}
	h.want("Edit role Notulis")
	h.click("Hapus role Editor web")
	h.tt.Frame()
	h.want("Hapus role Editor web?")
	h.click("Hapus role")
	h.settle()
	h.settle()
	if len(f.deleted) != 1 {
		t.Fatalf("deleted %v", f.deleted)
	}
	h.dontWant("Edit role Editor web")
}

// The access drawer opens the dialog over itself and keeps its unsaved
// choices while a role is made.
func TestCustomRoleFromAccessDrawer(t *testing.T) {
	h, base, _, nara := memberAccessHarness(t, 1440, 900)
	f := &roleFixture{memberAccessFixture: base}
	h.app.api = f
	openMemberAccessMenu(h, "Nara Tekno")
	h.click("Role dasar Pendamping kadiv")
	h.click("Atur role kustom")
	h.tt.Frame()
	h.click("Buat role baru")
	h.click("Contoh Admin WhatsApp")
	h.click("Buat role")
	h.settle()
	h.settle()
	m := &h.app.access.members
	if len(f.created) != 1 || m.editing != nara.UserID || !m.drawerOpen || m.role != api.RoleCompanion {
		t.Fatalf("created %d, drawer %v %v role %q", len(f.created), m.editing, m.drawerOpen, m.role)
	}
}

// The PJ chip of an action item without a PJ shows only while its row is
// hovered; its picker lies below the row. Going over to the picker kept
// the picker only while the row was hovered, so it vanished on the way.
func TestNotePJPickerStaysWhilePointerMovesToIt(t *testing.T) {
	aruna := id.New()
	e := c3Editable(api.NoteBlock{Type: api.BlockChecklist, Items: []api.NoteListItem{{Spans: []api.Span{{Text: "Revisi poster"}}}, {Spans: []api.Span{{Text: "Storyboard"}}}}})
	e.Members = func() []noteMember { return []noteMember{{aruna, "Aruna Desain"}, {id.New(), "Nara Tekno"}} }
	e.MemberName = func(uid id.ID) string { return map[id.ID]string{aruna: "Aruna Desain"}[uid] }
	tt := c3Tester(e, 900, 700)
	row, ok := tt.Find("Butir 1 blok 1")
	if !ok {
		t.Fatal("no item")
	}
	tt.Move(row.X+4, row.Y+row.H/2)
	c3Frames(tt)
	c3Click(t, tt, "PJ butir 1 blok 1")
	pick, ok := tt.Find("Pilih PJ Aruna Desain")
	if !ok {
		t.Fatalf("no picker: %q", tt.Texts())
	}
	// Over to the picker, off the row.
	tt.Move(pick.X+pick.W/2, pick.Y+pick.H/2)
	c3Frames(tt)
	tt.Advance(time.Second)
	c3Frames(tt)
	if !tt.HasText("Penanggung jawab") {
		t.Fatal("the picker went when the pointer left the row")
	}
	c3Click(t, tt, "Pilih PJ Aruna Desain")
	if it := e.Content()[0].Items[0]; it.AssigneeID == nil || *it.AssigneeID != aruna {
		t.Fatalf("PJ not set: %+v", it)
	}
	// Due date the same way.
	tt.Move(row.X+4, row.Y+row.H/2)
	c3Frames(tt)
	c3Click(t, tt, "Tenggat butir 1 blok 1")
	day, ok := tt.Find("Pakai tanggal")
	if !ok {
		t.Fatalf("no due picker: %q", tt.Texts())
	}
	tt.Move(day.X+day.W/2, day.Y+day.H/2)
	c3Frames(tt)
	if !tt.HasText("Tanggal tenggat") {
		t.Fatal("the due picker went when the pointer left the row")
	}
}
