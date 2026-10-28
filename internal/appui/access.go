package appui

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/workspaceui"
)

// WorkspaceAPI is optional so an older service cannot accidentally confer membership.
type WorkspaceAPI interface {
	Spaces(context.Context, id.ID) ([]api.Space, error)
	DivisionAccess(context.Context, id.ID, id.ID) (api.Access, error)
	Members(context.Context, id.ID, id.ID) ([]api.Member, error)
	Invitations(context.Context, id.ID, id.ID) ([]api.Invitation, error)
	Invite(context.Context, id.ID, id.ID, api.InviteRequest) (api.Invitation, error)
	ResendInvitation(context.Context, id.ID, id.ID, id.ID) (api.Invitation, error)
	RevokeInvitation(context.Context, id.ID, id.ID, id.ID) error
	UpdateMember(context.Context, id.ID, id.ID, id.ID, api.UpdateMemberRequest) (api.Member, error)
	RemoveMember(context.Context, id.ID, id.ID, id.ID) error
	Roles(context.Context, id.ID, id.ID) ([]api.CustomRole, error)
	CreateRole(context.Context, id.ID, id.ID, api.RoleRequest) (api.CustomRole, error)
	UpdateRole(context.Context, id.ID, id.ID, id.ID, api.RoleRequest) (api.CustomRole, error)
	DeleteRole(context.Context, id.ID, id.ID, id.ID) error
	Appoint45(context.Context, id.ID, id.ID, api.Appoint45Request) (api.Appointment45, error)
	Remove45(context.Context, id.ID, id.ID) error
}

var _ WorkspaceAPI = (*client.Client)(nil)

type workspaceAccessState struct {
	spaces   []api.Space
	selected id.ID
	loading  bool
	err      string
	notice   string
	seq      uint64
	members  memberManagementState
}

// workspaceService binds queued operations to the bearer that authorized them.
func (a *App) workspaceService() (WorkspaceAPI, bool) {
	service, ok := a.api.(WorkspaceAPI)
	if live, yes := a.api.(*client.Client); yes {
		service = live.AccountSession()
	}
	return service, ok
}
func (a *App) selectedSpace() *api.Space {
	for i := range a.access.spaces {
		if a.access.spaces[i].ID == a.access.selected {
			return &a.access.spaces[i]
		}
	}
	return nil
}
func (a *App) selectedSpaceID() id.ID {
	if s := a.selectedSpace(); s != nil {
		return s.ID
	}
	return id.Nil
}
func (a *App) selectSpace(sid id.ID) {
	if a.access.selected == sid && a.selectedSpace() != nil {
		return
	}
	valid := sid == id.Nil
	for _, s := range a.access.spaces {
		if s.ID == sid {
			valid = true
			break
		}
	}
	if !valid {
		return
	}
	a.access.selected = sid
	a.access.notice = ""
	a.access.members = memberManagementState{seq: a.access.members.seq + 1}
	a.chrome.tab = "Beranda"
	a.chrome.divisionID = id.Nil
	if d := a.selectedDivision(); d != nil {
		a.chrome.divisionID = d.ID
	}
	a.workspace = workspaceui.New()
	a.tasksContextChanged()
}
func (a *App) spacePermissionsForDivision(d id.ID, capability string) bool {
	s := a.selectedSpace()
	if s == nil {
		return false
	}
	readCapability := capability == api.CapSpaceRead || capability == api.CapMembersRead || capability == api.CapNotesRead
	if (s.ReadOnly || s.PeriodStatus == api.PeriodClosed) && !readCapability {
		return false
	}
	for _, r := range s.Roles {
		if r.DivisionID == d {
			if slices.Contains(r.Roles, api.Role45) && administrativeCapability(capability) {
				return false
			}
			if (r.ReadOnly || r.PeriodStatus == api.PeriodClosed) && !readCapability {
				return false
			}
			for _, cap := range r.Capabilities {
				if cap == capability {
					return true
				}
			}
		}
	}
	return false
}
func (a *App) changePeriod(p id.ID) {
	if a.periodID == p {
		return
	}
	a.periodID = p
	tab := a.organization.tab
	a.resetOrganization()
	if tab != "" {
		a.organization.tab = tab
	}
	a.resetAccess()
	a.workspace = workspaceui.New()
	a.tasksContextChanged()
	a.loadSpaces()
}
func (a *App) resetAccess() {
	a.resetArchiveAdmin()
	seq := a.access.seq + 1
	memberSeq := a.access.members.seq + 1
	a.access = workspaceAccessState{seq: seq, members: memberManagementState{seq: memberSeq}}
	a.chrome.divisionID = id.Nil
}
func (a *App) loadSpaces() {
	service, ok := a.workspaceService()
	if !ok {
		a.access.err = "Layanan akses Space belum tersedia."
		return
	}
	if a.periodID == id.Nil {
		return
	}
	a.access.seq++
	seq, gen, p := a.access.seq, a.gen, a.periodID
	a.access.loading = true
	a.access.err = ""
	a.async(func() func() {
		spaces, err := service.Spaces(context.Background(), p)
		return func() {
			if gen != a.gen || p != a.periodID || seq != a.access.seq {
				return
			}
			a.access.loading = false
			if err != nil {
				a.access.spaces = nil
				a.access.selected = id.Nil
				a.access.members = memberManagementState{seq: a.access.members.seq + 1}
				a.workspace = workspaceui.New()
				a.tasksContextChanged()
				a.access.err = a.failed(err)
				return
			}
			a.conn = connOnline
			previousSelection := a.access.selected
			a.access.spaces = spaces
			if a.access.members.division != id.Nil && !a.spacePermissionsForDivision(a.access.members.division, api.CapMembersRead) {
				a.access.members = memberManagementState{seq: a.access.members.seq + 1}
			}
			if a.selectedSpace() == nil {
				if previousSelection != id.Nil {
					a.access.notice = "Akses ruang yang dipilih telah berubah. Pilih kembali ruang yang tersedia."
				}
				a.access.selected = id.Nil
				a.access.members = memberManagementState{seq: a.access.members.seq + 1}
				a.chrome.divisionID = id.Nil
				a.workspace = workspaceui.New()
			}
			a.tasksContextChanged()
		}
	})
}
func (a *App) accessFailure(err error) string {
	var e *client.Error
	if errors.As(err, &e) && e.Status == http.StatusForbidden {
		a.resetAccess()
		a.workspace = workspaceui.New()
		a.tasksContextChanged()
		a.loadSpaces()
		a.access.notice = "Akses telah berubah. Daftar ruang dimuat ulang dari server."
	}
	return a.failed(err)
}

func administrativeCapability(cap string) bool {
	switch cap {
	case api.CapInvitationsManage, api.CapMembersManage, api.CapRolesManage, api.CapArchivesManage, api.Cap45Manage, api.CapPeriodsManage, api.CapKadivManage:
		return true
	}
	return false
}
