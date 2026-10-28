package appui

import (
	"context"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

type accessFixture struct {
	WorkspaceAPI
	*fakeAPI
	list func(context.Context, id.ID) ([]api.Space, error)
}

func (f *accessFixture) Spaces(c context.Context, p id.ID) ([]api.Space, error) { return f.list(c, p) }
func testAuthorizedSpace(a *App, d api.Division) api.Space {
	s := api.Space{ID: id.New(), PeriodID: a.periodID, Name: d.Name, Kind: api.SpaceDivision, Divisions: []api.Division{d}, Roles: []api.Access{{DivisionID: d.ID, Roles: []string{api.RoleMember}, Capabilities: []string{api.CapSpaceRead, api.CapMembersRead}}}, Capabilities: []string{api.CapSpaceRead, api.CapMembersRead}}
	a.access.spaces = []api.Space{s}
	a.selectSpace(s.ID)
	return s
}
func TestLegacyAPIConfirmsNoMembership(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.click("Space saya")
	h.want("Layanan akses Space belum tersedia.")
	h.dontWant("Medhum")
	h.dontWant("Media dan Teknologi")
	if h.app.selectedSpace() != nil || len(h.app.access.spaces) != 0 {
		t.Fatal("catalog conferred membership")
	}
}
func TestSpacesUseAuthorizedIDsAndBindings(t *testing.T) {
	h := newChromeHarness(t, nil)
	p := h.app.periodID
	med, hum := h.api.divisions[0], h.api.divisions[1]
	own := api.Space{ID: id.New(), PeriodID: p, Name: "Mediatek", Kind: api.SpaceDivision, Divisions: []api.Division{med}, Roles: []api.Access{{DivisionID: med.ID, Capabilities: []string{api.CapSpaceRead, api.CapMembersRead, api.CapTasksManage}}}}
	shared := api.Space{ID: id.New(), PeriodID: p, Name: "Medhum", Kind: api.SpaceShared, Divisions: []api.Division{hum, med}, Roles: own.Roles}
	h.app.api = &accessFixture{fakeAPI: h.api, list: func(context.Context, id.ID) ([]api.Space, error) { return []api.Space{own, shared}, nil }}
	h.app.loadSpaces()
	h.settle()
	h.app.selectSpace(shared.ID)
	if h.app.selectedDivision() != nil {
		t.Fatal("shared identity became a division binding")
	}
	if !h.app.spacePermissionsForDivision(med.ID, api.CapTasksManage) || h.app.spacePermissionsForDivision(hum.ID, api.CapTasksManage) {
		t.Fatal("shared Space broadened owner-division permission")
	}
	h.app.selectSpace(id.New())
	if h.app.selectedSpaceID() != shared.ID {
		t.Fatal("unknown Space selected")
	}
	h.app.router.Push(PathSpaces)
	h.tt.Frame()
	h.want("Ruang Medhum kita.")
	h.app.selectSpace(own.ID)
	if h.app.selectedDivision() == nil || h.app.selectedDivision().ID != med.ID {
		t.Fatal("division identity did not follow authorized Space")
	}
}
func TestSpaceResponsesCannotCrossPeriodOrAccount(t *testing.T) {
	for _, reset := range []string{"period", "account"} {
		t.Run(reset, func(t *testing.T) {
			h := newChromeHarness(t, nil)
			p := h.app.periodID
			started, release := make(chan struct{}), make(chan struct{})
			h.app.api = &accessFixture{fakeAPI: h.api, list: func(_ context.Context, period id.ID) ([]api.Space, error) {
				if period == p {
					close(started)
					<-release
					return []api.Space{{ID: id.New(), PeriodID: p, Name: "Private old Space"}}, nil
				}
				return nil, nil
			}}
			h.app.loadSpaces()
			<-started
			if reset == "period" {
				h.app.changePeriod(h.api.periods[0].ID)
			} else {
				h.app.toLogin("")
			}
			close(release)
			h.settle()
			if len(h.app.access.spaces) != 0 || h.app.selectedSpace() != nil {
				t.Fatal("stale authorized snapshot survived context change")
			}
			h.dontWant("Private old Space")
		})
	}
}
func TestRevokedSpaceClearsMemberAndTaskStateWithoutRetryLoop(t *testing.T) {
	h := newChromeHarness(t, nil)
	s := testAuthorizedSpace(h.app, h.api.divisions[0])
	h.app.access.members = memberManagementState{division: s.Divisions[0].ID, loaded: true, members: []api.Member{{UserID: id.New(), DisplayName: "Private member"}}}
	calls := 0
	h.app.api = &accessFixture{fakeAPI: h.api, list: func(context.Context, id.ID) ([]api.Space, error) {
		calls++
		return nil, &client.Error{Status: 403, Code: api.CodeForbidden, Message: "Akses dicabut"}
	}}
	h.app.loadSpaces()
	h.settle()
	if calls != 1 || h.app.selectedSpace() != nil || len(h.app.access.members.members) != 0 || h.app.workspace.HasSnapshot() {
		t.Fatalf("revocation retained authority or retried: calls=%d state=%+v", calls, h.app.access)
	}
}
func Test45AndReadOnlyNeverExposeAccessForms(t *testing.T) {
	for _, mode := range []string{"45", "closed"} {
		t.Run(mode, func(t *testing.T) {
			h := newChromeHarness(t, nil)
			d := h.api.divisions[0]
			s := testAuthorizedSpace(h.app, d)
			caps := []string{api.CapSpaceRead, api.CapMembersRead, api.CapTasksManage, api.CapWhatsAppManage}
			roles := []string{api.Role45}
			if mode == "closed" {
				roles = []string{api.RoleKadiv}
				caps = append(caps, api.CapMembersManage, api.CapInvitationsManage, api.CapRolesManage, api.Cap45Manage)
			}
			readOnly := mode == "closed"
			h.app.access.spaces[0].ReadOnly = readOnly
			h.app.access.spaces[0].Roles = []api.Access{{DivisionID: d.ID, Roles: roles, Capabilities: caps, ReadOnly: readOnly}}
			h.app.access.members = memberManagementState{division: d.ID, loaded: true, access: api.Access{DivisionID: d.ID, Roles: roles, Capabilities: caps, ReadOnly: readOnly}, members: []api.Member{{UserID: id.New(), DisplayName: "Anggota biasa", Role: api.RoleMember}}}
			h.app.router.Push(PathSpaces)
			h.app.chrome.tab = "Anggota"
			h.tt.Frame()
			h.want("Anggota biasa")
			h.dontWant("Kirim undangan")
			h.dontWant("Nama role kustom")
			h.dontWant("Atur Anggota biasa")
			h.dontWant("Tunjuk 45 Anggota biasa")
			if h.app.selectedSpaceID() != s.ID {
				t.Fatal("read-only lost readable selection")
			}
		})
	}
}
