package appui

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/authz"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

// memberAccessFixture records the writes of the access drawer.
type memberAccessFixture struct {
	*fakeAPI
	WorkspaceAPI
	mu         sync.Mutex
	access     api.Access
	members    []api.Member
	roles      []api.CustomRole
	calls      []string
	update     api.UpdateMemberRequest
	appoint    api.Appoint45Request
	appointErr error
	spaces     []api.Space
}

func (f *memberAccessFixture) record(call string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
}
func (f *memberAccessFixture) Spaces(context.Context, id.ID) ([]api.Space, error) {
	return slices.Clone(f.spaces), nil
}
func (f *memberAccessFixture) DivisionAccess(context.Context, id.ID, id.ID) (api.Access, error) {
	return f.access, nil
}
func (f *memberAccessFixture) Members(context.Context, id.ID, id.ID) ([]api.Member, error) {
	return slices.Clone(f.members), nil
}
func (f *memberAccessFixture) Roles(context.Context, id.ID, id.ID) ([]api.CustomRole, error) {
	return slices.Clone(f.roles), nil
}
func (f *memberAccessFixture) Invitations(context.Context, id.ID, id.ID) ([]api.Invitation, error) {
	return nil, nil
}
func (f *memberAccessFixture) UpdateMember(_ context.Context, _, _, uid id.ID, req api.UpdateMemberRequest) (api.Member, error) {
	f.record("update")
	f.mu.Lock()
	f.update = req
	f.mu.Unlock()
	return api.Member{UserID: uid}, nil
}
func (f *memberAccessFixture) Remove45(context.Context, id.ID, id.ID) error {
	f.record("remove45")
	return nil
}
func (f *memberAccessFixture) Appoint45(_ context.Context, _, _ id.ID, req api.Appoint45Request) (api.Appointment45, error) {
	f.record("appoint45")
	f.mu.Lock()
	f.appoint = req
	f.mu.Unlock()
	return api.Appointment45{UserID: req.UserID}, f.appointErr
}
func (f *memberAccessFixture) RemoveMember(_ context.Context, _, _, uid id.ID) error {
	f.record("remove:" + uid.String())
	return nil
}

var kadivCaps = []string{api.CapSpaceRead, api.CapMembersRead, api.CapMembersManage, api.CapInvitationsManage, api.CapRolesManage, api.Cap45Manage, api.CapArchivesManage}

// memberAccessHarness opens the Anggota tab of Mediatek as its kadiv,
// with Aruna (the 45), Nara and a custom role.
func memberAccessHarness(t *testing.T, w, h int) (*harness, *memberAccessFixture, api.Member, api.Member) {
	t.Helper()
	f := &memberAccessFixture{fakeAPI: newFake(api.AccountPerson)}
	hs := newHarness(t, f.fakeAPI, "")
	hs.app.api, hs.app.user, hs.app.screen, hs.app.conn = f, f.user, screenShell, connOnline
	hs.app.periodID, hs.app.periods = f.periods[1].ID, f.periods
	hs.tt.SetSize(w, h)
	d := f.divisions[0]
	for _, div := range f.divisions {
		if div.Slug == "mediatek" {
			d = div
		}
	}
	f.access = api.Access{DivisionID: d.ID, Roles: []string{api.RoleKadiv}, Capabilities: kadivCaps, PeriodStatus: api.PeriodActive}
	editor := api.CustomRole{ID: id.New(), Name: "Editor web", Capabilities: []string{api.CapNotesManage, api.CapSOPRead}}
	aruna := api.Member{UserID: id.New(), DisplayName: "Aruna Desain", Role: api.RoleMember, Roles: []string{api.Role45}, Groups: []string{api.GroupDesain}, Capabilities: append(authz.RoleCaps(api.RoleMember), authz.Default45...)}
	nara := api.Member{UserID: id.New(), DisplayName: "Nara Tekno", Role: api.RoleMember, Groups: []string{api.GroupTekno}, Capabilities: authz.RoleCaps(api.RoleMember)}
	me := api.Member{UserID: hs.app.user.ID, DisplayName: "Demo Mediatek", Role: api.RoleKadiv}
	f.members, f.roles = []api.Member{me, aruna, nara}, []api.CustomRole{editor}
	sp := api.Space{ID: id.New(), PeriodID: hs.app.periodID, Name: d.Name, Kind: api.SpaceDivision, Divisions: []api.Division{d}, Roles: []api.Access{f.access}, Capabilities: kadivCaps}
	hs.app.access.spaces = []api.Space{sp}
	f.spaces = []api.Space{sp}
	hs.app.selectSpace(sp.ID)
	hs.app.access.members = memberManagementState{division: d.ID, loaded: true, access: f.access, members: slices.Clone(f.members), roles: slices.Clone(f.roles)}
	hs.app.router.Push(PathSpaces)
	hs.app.chrome.tab = "Anggota"
	hs.tt.Frame()
	return hs, f, aruna, nara
}

func TestMemberAccessSummary(t *testing.T) {
	editor := api.CustomRole{Name: "Editor web", Capabilities: []string{api.CapSOPRead, api.CapPingSend}}
	can, cannot := memberAccessSummary(api.RoleMember, true, authz.Default45, []api.CustomRole{editor})
	from := map[string]string{}
	for _, s := range can {
		from[s.label] = s.from
	}
	for label, want := range map[string]string{"Mengerjakan tugas sendiri": "", "Kelola catatan": "", "Kelola tugas": "dari 45", "Kirim ping": "dari 45", "Kelola WhatsApp": "dari 45", "Baca evaluasi SOP": "dari Editor web"} {
		if got, ok := from[label]; !ok || got != want {
			t.Errorf("%s: from %q (listed %v), want %q", label, got, ok, want)
		}
	}
	if !slices.Contains(cannot, "Tinjau temuan SOP") {
		t.Errorf("cannot = %v", cannot)
	}
	// A companion gets delegation from the base role, not from a 45.
	can, cannot = memberAccessSummary(api.RoleCompanion, false, nil, nil)
	for _, s := range can {
		if s.label == "Kelola tugas" && s.from != "" {
			t.Errorf("companion's Kelola tugas from %q", s.from)
		}
	}
	if !slices.Contains(cannot, "Kelola WhatsApp") {
		t.Errorf("companion manages WhatsApp: %v", cannot)
	}
}

// "Ubah akses" opens the drawer in the canvas' order, with no inline panel
// under the row; nothing changed means nothing to save.
func TestMemberAccessDrawerOpensInOrder(t *testing.T) {
	h, _, _, _ := memberAccessHarness(t, 1440, 1500)
	h.dontWant("Tunjuk 45 Nara Tekno")
	h.click("Ubah akses Nara Tekno")
	h.tt.Frame()
	texts := h.tt.Texts()
	order := []string{"Ubah akses · Nara Tekno", "Role dasar", "Role dasar Anggota", "Role dasar Pendamping kadiv", "Wakil operasional (45)", "Kelompok", "Kelompok Desain", "Role kustom", "Yang bisa dilakukan Nara", "Keluarkan dari Media dan Teknologi", "Belum ada perubahan", "Simpan akses"}
	at := -1
	for _, s := range order {
		i := slices.Index(texts[at+1:], s)
		if i < 0 {
			t.Fatalf("%q missing or out of order; texts %q", s, texts)
		}
		at += i + 1
	}
	for _, old := range []string{"Hapus anggota Nara Tekno", "Tunjuk 45 Nara Tekno", "Konfirmasi perubahan"} {
		h.dontWant(old)
	}
	h.click("Tutup ubah akses")
	h.tt.Frame()
	h.dontWant("Simpan akses")
	if h.app.access.members.editing != id.Nil {
		t.Fatal("closing kept the drawer's member")
	}
}

// Changing the role and groups and making Nara the 45 in place of Aruna
// asks first, counts three changes, then saves member, removes the old 45
// and appoints the new one, in that order.
func TestMemberAccessDrawerSavesAndReplaces45(t *testing.T) {
	h, f, _, nara := memberAccessHarness(t, 1440, 1500)
	h.click("Ubah akses Nara Tekno")
	h.click("Role dasar Pendamping kadiv")
	h.click("Kelompok Desain")
	h.want("2 perubahan belum disimpan")
	h.click("Wakil operasional (45) Nara Tekno")
	h.tt.Frame()
	h.want("Ganti 45 dari Aruna Desain ke Nara Tekno?")
	h.click("Batal ganti 45")
	h.tt.Frame()
	if h.app.access.members.on45 {
		t.Fatal("cancelling the replacement turned the switch on")
	}
	h.click("Wakil operasional (45) Nara Tekno")
	h.click("Ganti 45")
	h.tt.Frame()
	h.want("3 perubahan belum disimpan")
	h.want("dari 45") // the summary shows what the 45 adds
	h.click("Simpan akses")
	h.settle()
	f.mu.Lock()
	calls, update, appoint := slices.Clone(f.calls), f.update, f.appoint
	f.mu.Unlock()
	if strings.Join(calls, ",") != "update,remove45,appoint45" {
		t.Fatalf("calls = %v", calls)
	}
	if update.Role == nil || *update.Role != api.RoleCompanion || !sameSet(update.Groups, []string{api.GroupTekno, api.GroupDesain}) || appoint.UserID != nara.UserID || appoint.Capabilities != nil {
		t.Fatalf("update %+v appoint %+v", update, appoint)
	}
	if h.app.access.members.editing != id.Nil || !strings.Contains(h.app.access.members.notice, "Akses Nara Tekno disimpan") {
		t.Fatalf("after save: editing %v notice %q", h.app.access.members.editing, h.app.access.members.notice)
	}
}

// A save that fails half way says what was saved and what was not.
func TestMemberAccessDrawerReportsPartialSave(t *testing.T) {
	h, f, _, _ := memberAccessHarness(t, 1440, 1500)
	f.appointErr = &client.Error{Status: 409, Message: "45 sudah ada."}
	h.click("Ubah akses Nara Tekno")
	h.click("Role dasar Pendamping kadiv")
	h.click("Wakil operasional (45) Nara Tekno")
	h.click("Ganti 45")
	h.click("Simpan akses")
	h.settle()
	got := h.app.access.members.err
	if !strings.HasPrefix(got, "Role, kelompok dan role kustom tersimpan, tetapi menunjuk 45 gagal:") || !strings.Contains(got, "45 sudah ada.") {
		t.Fatalf("error = %q", got)
	}
	var step *memberStepError
	if !errors.As(&memberStepError{done: "x", err: errors.New("y")}, &step) {
		t.Fatal("step error does not unwrap")
	}
}

// Removing asks in the drawer; a 45 is removed from the 45 first.
func TestMemberAccessDrawerRemoval(t *testing.T) {
	h, f, aruna, nara := memberAccessHarness(t, 1440, 1500)
	h.click("Ubah akses Aruna Desain")
	h.want("Aruna masih 45. Matikan sakelar 45 dan simpan dulu, lalu keluarkan.")
	h.dontWant("Keluarkan Aruna…")
	h.click("Tutup ubah akses")
	h.click("Ubah akses Nara Tekno")
	h.click("Keluarkan Nara…")
	h.tt.Frame()
	h.want("Keluarkan Nara Tekno dari Media dan Teknologi?")
	h.click("Keluarkan anggota")
	h.settle()
	f.mu.Lock()
	calls := slices.Clone(f.calls)
	f.mu.Unlock()
	if len(calls) != 1 || calls[0] != "remove:"+nara.UserID.String() {
		t.Fatalf("calls = %v (aruna %v)", calls, aruna.UserID)
	}
}

// On a phone the drawer is a bottom sheet with the same content and the
// row action has a 36 px target.
func TestMemberAccessSheetOnPhone(t *testing.T) {
	h, _, _, _ := memberAccessHarness(t, 390, 844)
	h.click("Ubah akses Nara Tekno")
	h.tt.Frame()
	for _, s := range []string{"Ubah akses · Nara Tekno", "Role dasar Anggota", "Simpan akses", "Batal"} {
		h.want(s)
	}
	r, ok := h.tt.Find("Simpan akses")
	if !ok || r.Y+r.H > 844 || r.Y < 844*0.08 {
		t.Fatalf("sheet footer at %+v", r)
	}
}

// Someone who may not manage members sees no "Ubah akses".
func TestMemberAccessHiddenWithoutRights(t *testing.T) {
	h, _, _, _ := memberAccessHarness(t, 1440, 1500)
	member := api.Access{DivisionID: h.app.access.members.division, Roles: []string{api.RoleMember}, Capabilities: []string{api.CapSpaceRead, api.CapMembersRead}}
	h.app.access.spaces[0].Roles = []api.Access{member}
	h.app.access.members.access = member
	h.tt.Frame()
	h.dontWant("Ubah akses Nara Tekno")
}

// Kelompok is a column only when someone has a group.
func TestMemberColumnsHideEmptyGroups(t *testing.T) {
	if memberHasGroups([]api.Member{{DisplayName: "A"}}) || !memberHasGroups([]api.Member{{Groups: []string{api.GroupVideo}}}) {
		t.Fatal("memberHasGroups")
	}
	without, with := memberColumns(false), memberColumns(true)
	if len(with) != len(without)+1 || slices.ContainsFunc(without, func(c memberColumn) bool { return c.label == "Kelompok" }) {
		t.Fatalf("columns %v / %v", without, with)
	}
	sum := float32(0)
	for _, c := range with {
		sum += c.width
	}
	// At 1280 px the page is 984 px wide: less row padding, gaps and the
	// avatar, a name keeps at least 200 px (it had about 70 in 0.3.2).
	if 984-32-sum-16*float32(len(with))-44 < 200 {
		t.Fatalf("fixed columns take %v px; names get too little", sum)
	}
}

// Leaving with unsaved changes asks first; "Batal" is an explicit discard.
func TestMemberAccessDrawerAsksBeforeDiscarding(t *testing.T) {
	h, f, _, _ := memberAccessHarness(t, 1440, 1500)
	h.click("Ubah akses Nara Tekno")
	h.click("Role dasar Pendamping kadiv")
	h.click("Tutup ubah akses")
	h.tt.Frame()
	h.want("Buang 1 perubahan yang belum disimpan (role dasar)?")
	h.click("Lanjut mengubah")
	h.tt.Frame()
	if h.app.access.members.role != api.RoleCompanion || h.app.access.members.editing == id.Nil {
		t.Fatal("continuing lost the change")
	}
	h.want("1 perubahan belum disimpan")
	h.click("Tutup ubah akses")
	h.click("Buang perubahan")
	h.tt.Frame()
	if h.app.access.members.editing != id.Nil {
		t.Fatal("discarding kept the drawer open")
	}
	h.click("Ubah akses Nara Tekno")
	if h.app.access.members.role != api.RoleMember {
		t.Fatal("a discarded change came back")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != 0 {
		t.Fatalf("calls = %v", f.calls)
	}
}
