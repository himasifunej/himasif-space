package appui

import (
	"slices"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// The member chip and the Anggota table count the same people: in a
// division Space whose table is loaded the chip shows the table's count;
// before anything has loaded it shows no number rather than "0 anggota".
func TestHeaderMemberChipMatchesTable(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(1440, 900)
	d := h.api.divisions[0]
	sp := testAuthorizedSpace(h.app, d)
	h.app.router.Push(PathSpaces)
	texts := func() []string { h.tt.Frame(); return h.tt.Texts() }

	// The fake has no meeting candidates: the count is unknown.
	if got := texts(); slices.Contains(got, "0 anggota") || !slices.Contains(got, "Anggota") {
		t.Fatalf("unknown count shown as a number: %q", got)
	}
	if _, known := h.app.headerMemberCount(&sp); known {
		t.Fatal("count known before anything loaded")
	}

	// Candidates loaded (the real client), table not yet.
	h.app.chrome.members.loaded, h.app.chrome.members.err = true, ""
	h.app.chrome.members.items = []api.PersonRef{{ID: id.New(), DisplayName: "A"}, {ID: id.New(), DisplayName: "B"}, {ID: id.New(), DisplayName: "C"}}
	if got := texts(); !slices.Contains(got, "3 anggota") {
		t.Fatalf("candidate count missing: %q", got)
	}

	// The table counts every membership, disabled accounts too; the chip
	// follows it once loaded.
	members := make([]api.Member, 7)
	for i := range members {
		members[i] = api.Member{UserID: id.New(), DisplayName: "Anggota", Role: api.RoleMember}
	}
	h.app.access.members = memberManagementState{division: d.ID, loaded: true, members: members}
	if got := texts(); !slices.Contains(got, "7 anggota") || slices.Contains(got, "3 anggota") {
		t.Fatalf("chip does not follow the table: %q", got)
	}

	// Another division's table never counts for this Space.
	h.app.access.members.division = id.New()
	if n, _ := h.app.headerMemberCount(&sp); n != 3 {
		t.Fatalf("count from another division's table: %d", n)
	}
}

// "Izin arsip" in the Anggota header follows the right to manage archive
// grants: a kadiv (archives.manage) sees it; a member, a 45 or a role
// without archives.manage does not.
func TestMembersHeaderArchiveButton(t *testing.T) {
	for _, tc := range []struct {
		name  string
		roles []string
		caps  []string
		want  bool
	}{
		{"kadiv", []string{api.RoleKadiv}, []string{api.CapSpaceRead, api.CapMembersRead, api.CapMembersManage, api.CapInvitationsManage, api.CapArchivesManage}, true},
		{"kadiv without archives.manage", []string{api.RoleKadiv}, []string{api.CapSpaceRead, api.CapMembersRead, api.CapMembersManage}, false},
		{"45", []string{api.RoleMember, api.Role45}, []string{api.CapSpaceRead, api.CapMembersRead, api.CapArchivesManage}, false},
		{"member", []string{api.RoleMember}, []string{api.CapSpaceRead, api.CapMembersRead}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newChromeHarness(t, nil)
			h.tt.SetSize(1440, 900)
			d := h.api.divisions[0]
			testAuthorizedSpace(h.app, d)
			h.app.access.spaces[0].Roles = []api.Access{{DivisionID: d.ID, Roles: tc.roles, Capabilities: tc.caps}}
			h.app.access.members = memberManagementState{division: d.ID, loaded: true, access: api.Access{DivisionID: d.ID, Roles: tc.roles, Capabilities: tc.caps},
				members: []api.Member{{UserID: h.app.user.ID, DisplayName: "Saya", Role: api.RoleKadiv}}}
			h.app.router.Push(PathSpaces)
			h.app.chrome.tab = "Anggota"
			h.tt.Frame()
			if got := slices.Contains(h.tt.Texts(), "Izin arsip"); got != tc.want {
				t.Fatalf("Izin arsip shown = %v, want %v; texts %q", got, tc.want, h.tt.Texts())
			}
		})
	}
}
