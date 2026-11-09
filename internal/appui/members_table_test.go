package appui

import (
	"os"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Stage A1: the workload counts late tasks, which the bar draws in red.
func TestMemberWorkCountsLateTasks(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	past, future := now.Add(-2*time.Hour), now.Add(48*time.Hour)
	uid := id.New()
	a := &App{}
	if _, _, ok := a.memberWork(uid, now); ok {
		t.Fatal("work reported before tasks are loaded")
	}
	a.tasks.loaded = true
	a.tasks.items = []api.Task{
		{Assignee: api.PersonRef{ID: uid}, Progress: api.TaskTodo, DeadlineAt: &past},
		{Assignee: api.PersonRef{ID: uid}, Progress: api.TaskDoing, DeadlineAt: &future},
		{Assignee: api.PersonRef{ID: uid}, Progress: api.TaskDoing},
		{Assignee: api.PersonRef{ID: uid}, Progress: api.TaskDone, DeadlineAt: &past},
		{Assignee: api.PersonRef{ID: id.New()}, Progress: api.TaskTodo, DeadlineAt: &past},
	}
	if active, late, ok := a.memberWork(uid, now); !ok || active != 3 || late != 1 {
		t.Fatalf("work = %d active, %d late, %v; want 3, 1", active, late, ok)
	}
	for _, tc := range []struct {
		active, late int
		want         string
	}{{0, 0, "Kosong"}, {4, 0, "4 aktif"}, {5, 1, "5 aktif · 1 lewat"}} {
		if got := memberLoadLabel(tc.active, tc.late); got != tc.want {
			t.Errorf("label(%d,%d) = %q; want %q", tc.active, tc.late, got, tc.want)
		}
	}
}

// Search, the filter chips and the sort orders of the toolbar.
func TestVisibleMembersFiltersAndSorts(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	yes, no := true, false
	kadiv := api.Member{UserID: id.New(), DisplayName: "Demo Mediatek", Role: api.RoleKadiv, WhatsAppLinked: &yes}
	wakil := api.Member{UserID: id.New(), DisplayName: "Aruna Desain", Role: api.RoleMember, Roles: []string{api.Role45}, Groups: []string{api.GroupDesain}, WhatsAppLinked: &yes}
	nara := api.Member{UserID: id.New(), DisplayName: "Nara Tekno", Role: api.RoleMember, Groups: []string{api.GroupTekno}, WhatsAppLinked: &no}
	bima := api.Member{UserID: id.New(), DisplayName: "Bima Konten", Role: api.RoleMember, Groups: []string{api.GroupDesain}}
	a := &App{}
	a.access.members.members = []api.Member{nara, bima, wakil, kadiv}
	a.tasks.loaded = true
	a.tasks.items = []api.Task{
		{Assignee: api.PersonRef{ID: bima.UserID}, Progress: api.TaskTodo, DeadlineAt: &past},
		{Assignee: api.PersonRef{ID: nara.UserID}, Progress: api.TaskTodo},
		{Assignee: api.PersonRef{ID: nara.UserID}, Progress: api.TaskTodo},
	}
	names := func() []string {
		out := []string{}
		for _, m := range a.visibleMembers(now) {
			out = append(out, m.DisplayName)
		}
		return out
	}
	check := func(what string, want ...string) {
		t.Helper()
		got := names()
		if len(got) != len(want) {
			t.Fatalf("%s: %v; want %v", what, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s: %v; want %v", what, got, want)
			}
		}
	}
	check("role order", "Demo Mediatek", "Aruna Desain", "Bima Konten", "Nara Tekno")
	a.access.members.sort = "beban"
	check("workload order", "Bima Konten", "Nara Tekno", "Aruna Desain", "Demo Mediatek")
	a.access.members.sort = "nama"
	check("name order", "Aruna Desain", "Bima Konten", "Demo Mediatek", "Nara Tekno")
	a.access.members.filter = memberFilterPengurus
	check("pengurus", "Aruna Desain", "Demo Mediatek")
	a.access.members.filter = memberFilterAnggota
	check("anggota", "Bima Konten", "Nara Tekno")
	a.access.members.filter = memberFilterNoWA
	check("not linked", "Nara Tekno") // unknown status (nil) is not "belum"
	a.access.members.filter = memberFilterLate
	check("late", "Bima Konten")
	a.access.members.filter = memberFilterAll
	a.access.members.query = "desain"
	check("search by group", "Aruna Desain", "Bima Konten")
	if n := a.memberFilterCount(memberFilterPengurus, now); n != 2 {
		t.Fatalf("pengurus count = %d; want 2", n)
	}
	if nextMemberSort("nama") != "peran" || nextMemberSort("") != "beban" {
		t.Fatal("sort cycle")
	}
}

// SPACE_UI_CAPTURE_DIR opts into PNGs of the Anggota table (stage A1) on a
// desktop and a phone, in the light theme, with dummy members.
func TestMembersTableNativeCapture(t *testing.T) {
	if os.Getenv("SPACE_UI_CAPTURE_DIR") == "" {
		t.Skip("set SPACE_UI_CAPTURE_DIR for native screenshots")
	}
	h := newChromeHarness(t, nil)
	h.app.chrome.preferences.Theme = "light"
	h.app.chrome.preferences.Accent = "rose"
	h.tt.SetSize(1440, 900)
	d := h.api.divisions[0]
	sp := testAuthorizedSpace(h.app, d)
	caps := []string{api.CapSpaceRead, api.CapMembersRead, api.CapMembersManage, api.CapInvitationsManage, api.Cap45Manage, api.CapPingSend, api.CapArchivesManage}
	roles := []string{api.RoleKadiv}
	h.app.access.spaces[0].Roles = []api.Access{{DivisionID: d.ID, Roles: roles, Capabilities: caps}}
	yes, no := true, false
	editor := id.New()
	members := []api.Member{
		{UserID: h.app.user.ID, DisplayName: "Demo Mediatek", Role: api.RoleKadiv, Groups: []string{api.GroupTekno}, WhatsAppLinked: &yes},
		{UserID: id.New(), DisplayName: "Aruna Desain", Role: api.RoleMember, Roles: []string{api.Role45}, Groups: []string{api.GroupDesain}, WhatsAppLinked: &yes},
		{UserID: id.New(), DisplayName: "Sinta Pendamping", Role: api.RoleCompanion, WhatsAppLinked: &yes},
		{UserID: id.New(), DisplayName: "Nara Tekno", Role: api.RoleMember, Groups: []string{api.GroupTekno}, CustomRoleIDs: []id.ID{editor}, WhatsAppLinked: &no},
		{UserID: id.New(), DisplayName: "Raka Video", Role: api.RoleMember, Groups: []string{api.GroupVideo}, WhatsAppLinked: &no},
		{UserID: id.New(), DisplayName: "Bima Konten", Role: api.RoleMember, Groups: []string{api.GroupDesain}, WhatsAppLinked: &yes},
		{UserID: id.New(), DisplayName: "Laras Foto", Role: api.RoleMember, Groups: []string{api.GroupVideo}, WhatsAppLinked: &yes},
	}
	h.app.access.members = memberManagementState{division: d.ID, loaded: true, access: api.Access{DivisionID: d.ID, Roles: roles, Capabilities: caps}, members: members,
		roles:       []api.CustomRole{{ID: editor, Name: "Editor web"}},
		invitations: []api.Invitation{{ID: id.New(), Email: "calon.anggota@contoh.ac.id", Status: api.InvitationPending, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(7 * 24 * time.Hour)}}}
	past := time.Now().Add(-26 * time.Hour)
	tasks := []api.Task{}
	add := func(m api.Member, n, late int) {
		for i := 0; i < n; i++ {
			tk := api.Task{ID: id.New(), Title: "Tugas contoh", Assignee: api.PersonRef{ID: m.UserID, DisplayName: m.DisplayName}, Progress: api.TaskDoing}
			if i < late {
				tk.DeadlineAt = &past
			}
			tasks = append(tasks, tk)
		}
	}
	add(members[0], 3, 0)
	add(members[1], 5, 1)
	add(members[2], 2, 0)
	add(members[3], 4, 0)
	add(members[4], 6, 2)
	add(members[5], 1, 0)
	h.app.tasks = tasksUIState{scope: sp.ID, loaded: true, items: tasks}
	h.app.router.Push(PathSpaces)
	h.app.chrome.tab = "Anggota"
	shot := func(name string) {
		h.tt.Frame()
		h.tt.Advance(400 * time.Millisecond)
		h.tt.Frame()
		captureNativeUI(t, h, name)
	}
	shot("anggota-a1-desktop")
	h.app.access.members.filter = memberFilterLate
	shot("anggota-a1-desktop-filter-lewat")
	h.app.access.members.filter = memberFilterAll
	h.app.access.members.query = "zaki"
	shot("anggota-a1-desktop-kosong")
	h.app.access.members.query = ""
	h.tt.SetSize(390, 844)
	shot("anggota-a1-hp")
}

// The page shows the new head, chips and an empty state for a search that
// finds nobody, with a way back.
func TestMembersPageSearchEmptyState(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(1440, 900)
	d := h.api.divisions[0]
	testAuthorizedSpace(h.app, d)
	caps := []string{api.CapSpaceRead, api.CapMembersRead}
	roles := []string{api.RoleMember}
	h.app.access.spaces[0].Roles = []api.Access{{DivisionID: d.ID, Roles: roles, Capabilities: caps}}
	h.app.access.members = memberManagementState{division: d.ID, loaded: true, access: api.Access{DivisionID: d.ID, Roles: roles, Capabilities: caps}, members: []api.Member{
		{UserID: id.New(), DisplayName: "Aruna Desain", Role: api.RoleMember, Roles: []string{api.Role45}},
		{UserID: id.New(), DisplayName: "Nara Tekno", Role: api.RoleMember},
	}}
	h.app.router.Push(PathSpaces)
	h.app.chrome.tab = "Anggota"
	h.tt.Frame()
	for _, s := range []string{"Anggota", "45 · Anggota", "Semua", "Pengurus", "Urutkan: Peran"} {
		h.want(s)
	}
	h.app.access.members.query = "zaki"
	h.tt.Frame()
	h.want("Tidak ada anggota bernama “zaki”")
	h.dontWant("Nara Tekno")
	h.click("Hapus pencarian dan filter")
	h.tt.Frame()
	h.want("Nara Tekno")
}
