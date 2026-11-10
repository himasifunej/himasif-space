package appui

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

type memberManagementState struct {
	actingCustom          bool
	actingCaps            []string
	division              id.ID
	loaded, loading, busy bool
	seq                   uint64
	err, notice, email    string
	access                api.Access
	members               []api.Member
	invitations           []api.Invitation
	roles                 []api.CustomRole
	// editing is the member whose access drawer is open (stage A2), with
	// the drawer's unsaved choices: role, groups, roleIDs, on45 (and
	// replace45 when it replaces the current 45), actingCustom/actingCaps.
	editing               id.ID
	drawerOpen            bool
	on45, replace45       bool
	replaceAsk, removeAsk bool
	discardAsk            bool
	role                  string
	groups                []string
	roleIDs               []id.ID
	roleEditing           id.ID
	roleName              string
	roleCaps              []string
	confirmAction         string
	confirmID             id.ID
	inviteOpen            bool
	// query, filter and sort are the table's toolbar (stage A1).
	query, filter, sort string
	// pinned keeps the rows where they were after a save, so a row whose
	// role changed does not jump (stage A3); the refresh button or another
	// sort order lets the table sort again.
	pinned []id.ID
	// removeTasks is the server's count of the active tasks of the member
	// in the removal dialog (removeTasksFor), across the division's Spaces;
	// nil while loading or when the server cannot tell (removeTasksLocal:
	// the dialog falls back to the open Space's tasks).
	removeTasks        *api.MemberActiveTasks
	removeTasksFor     id.ID
	removeTasksLoading bool
	removeTasksLocal   bool
}

func (a *App) memberDivision() id.ID {
	if s := a.selectedSpace(); s != nil {
		for _, r := range s.Roles {
			if a.spacePermissionsForDivision(r.DivisionID, api.CapMembersRead) {
				return r.DivisionID
			}
		}
	}
	return id.Nil
}
func (a *App) loadMembers(d id.ID) {
	svc, ok := a.workspaceService()
	if !ok || !a.spacePermissionsForDivision(d, api.CapMembersRead) {
		return
	}
	old := a.access.members
	a.access.members = memberManagementState{division: d, loading: true, seq: old.seq + 1}
	if old.division == d && old.loaded {
		// Reloading the same division keeps the table, its toolbar and its
		// row order on screen until the new data arrives.
		n := &a.access.members
		n.loaded, n.access, n.members, n.roles, n.invitations = true, old.access, old.members, old.roles, old.invitations
		n.query, n.filter, n.sort, n.pinned = old.query, old.filter, old.sort, old.pinned
	}
	seq, gen, p, sid := a.access.members.seq, a.gen, a.periodID, a.selectedSpaceID()
	a.async(func() func() {
		ac, err := svc.DivisionAccess(context.Background(), p, d)
		var members []api.Member
		var roles []api.CustomRole
		var invites []api.Invitation
		if err == nil {
			members, err = svc.Members(context.Background(), p, d)
		}
		if err == nil {
			roles, err = svc.Roles(context.Background(), p, d)
		}
		if err == nil && slices.Contains(ac.Capabilities, api.CapInvitationsManage) {
			invites, err = svc.Invitations(context.Background(), p, d)
		}
		return func() {
			if gen != a.gen || p != a.periodID || sid != a.selectedSpaceID() || seq != a.access.members.seq || d != a.access.members.division {
				return
			}
			m := &a.access.members
			m.loading = false
			if err != nil {
				m.err = a.accessFailure(err)
				return
			}
			m.loaded = true
			m.access = ac
			m.members = members
			m.roles = roles
			m.invitations = invites
		}
	})
}
func (a *App) memberCan(cap string) bool {
	m := &a.access.members
	return m.loaded && !m.access.ReadOnly && !(slices.Contains(m.access.Roles, api.Role45) && administrativeCapability(cap)) && slices.Contains(m.access.Capabilities, cap) && a.spacePermissionsForDivision(m.division, cap)
}
func (a *App) memberWrite(cap, label string, work func(WorkspaceAPI, id.ID, id.ID) error) {
	m := &a.access.members
	if m.busy || !a.memberCan(cap) {
		return
	}
	svc, ok := a.workspaceService()
	if !ok {
		return
	}
	m.busy = true
	m.err = ""
	m.notice = ""
	gen, p, sid, d, seq := a.gen, a.periodID, a.selectedSpaceID(), m.division, m.seq
	a.async(func() func() {
		err := work(svc, p, d)
		return func() {
			if gen != a.gen || p != a.periodID || sid != a.selectedSpaceID() || d != a.access.members.division || seq != a.access.members.seq {
				return
			}
			a.access.members.busy = false
			// A save that did part of its work reloads what was saved and
			// says what failed.
			var step *memberStepError
			if errors.As(err, &step) {
				msg := step.done + " " + a.accessFailure(step.err)
				a.pinMemberOrder()
				a.loadMembers(d)
				a.access.members.err = msg
				a.loadSpaces()
				return
			}
			// A removal the server refused for active tasks: the dialog shows
			// the server's count and where the tasks are.
			if tasks, ok := memberTasksFromError(err); ok {
				m := &a.access.members
				m.removeTasks, m.removeTasksFor, m.removeTasksLocal, m.removeTasksLoading = &tasks, m.editing, false, false
				m.removeAsk = m.editing != id.Nil
				return
			}
			if err != nil {
				a.access.members.err = a.accessFailure(err)
				return
			}
			a.pinMemberOrder()
			a.loadMembers(d)
			a.access.members.notice = label
			if strings.HasPrefix(label, "Undangan dikirim ke ") {
				a.access.members.email = "" // ready for the next NIM
			}
			a.loadSpaces()
		}
	})
}

var operationalRoleChoices = []struct{ cap, label string }{{api.CapTasksManage, "Kelola tugas"}, {api.CapResultsReview, "Tinjau hasil"}, {api.CapPingSend, "Kirim ping"}, {api.CapWhatsAppManage, "Kelola WhatsApp"}, {api.CapNotesManage, "Kelola catatan"}, {api.CapSOPRead, "Baca evaluasi SOP"}, {api.CapSOPReview, "Tinjau temuan SOP"}}

func (a *App) memberRoles(c *ui.Context) {
	m := &a.access.members
	pageSection(c, "Role kustom", "Role tambahan hanya memberi kewenangan operasional. Role 45 tidak dapat mengatur akses.", func() {
		for _, r := range m.roles {
			// Icon buttons (the name stays their label and tooltip) so the
			// row fits a phone instead of running past its edge.
			ui.Row(c.Key(r.ID)).FillWidth().MinWidth(0).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, r.Name).Grow(1).MinWidth(0).MaxLines(2)
				if spaceIconButton(c, "Edit role "+r.Name, "settings").Disabled(m.busy).Clicked() {
					m.roleEditing = r.ID
					m.roleName = r.Name
					m.roleCaps = slices.Clone(r.Capabilities)
				}
				if spaceIconButton(c, "Hapus role "+r.Name, "close").Disabled(m.busy).Clicked() {
					m.confirmAction = "delete-role"
					m.confirmID = r.ID
				}
			})
		}
		ui.TextInput(c.Key("role-name"), &m.roleName).Label("Nama role kustom").Placeholder("Nama role").FillWidth().Disabled(m.busy)
		for _, r := range operationalRoleChoices {
			if spaceButton(c, r.label, "check", slices.Contains(m.roleCaps, r.cap)).Disabled(m.busy).Clicked() {
				m.roleCaps = toggleString(m.roleCaps, r.cap)
			}
		}
		label := "Buat role"
		if m.roleEditing != id.Nil {
			label = "Simpan role"
		}
		if spaceButton(c, label, "plus", true).Disabled(m.busy || strings.TrimSpace(m.roleName) == "").Clicked() {
			req := api.RoleRequest{Name: strings.TrimSpace(m.roleName), Capabilities: append([]string{}, m.roleCaps...)}
			rid := m.roleEditing
			a.memberWrite(api.CapRolesManage, "Role disimpan.", func(s WorkspaceAPI, p, d id.ID) error {
				if rid == id.Nil {
					_, err := s.CreateRole(context.Background(), p, d, req)
					return err
				}
				_, err := s.UpdateRole(context.Background(), p, d, rid, req)
				return err
			})
		}
		if m.roleEditing != id.Nil && spaceButton(c, "Batal edit role", "close", false).Clicked() {
			m.roleEditing = id.Nil
			m.roleName = ""
			m.roleCaps = nil
		}
	})
}
func (a *App) memberConfirmation(c *ui.Context) {
	m := &a.access.members
	action, target := m.confirmAction, m.confirmID
	// Member access, removal and the 45 are confirmed in the access drawer.
	text := map[string]string{"revoke-invite": "Cabut undangan ini? Tautan email sebelumnya tidak dapat digunakan.", "delete-role": "Hapus role ini beserta penugasannya?"}[action]
	pageSection(c, "Konfirmasi perubahan", text, func() {
		if spaceButton(c, "Batal perubahan", "close", false).Disabled(m.busy).Clicked() {
			m.confirmAction = ""
		}
		if spaceButton(c, "Konfirmasi perubahan", "check", true).Disabled(m.busy).Clicked() {
			switch action {
			case "revoke-invite":
				a.memberWrite(api.CapInvitationsManage, "Undangan dicabut.", func(s WorkspaceAPI, p, d id.ID) error { return s.RevokeInvitation(context.Background(), p, d, target) })
			case "delete-role":
				a.memberWrite(api.CapRolesManage, "Role dihapus.", func(s WorkspaceAPI, p, d id.ID) error { return s.DeleteRole(context.Background(), p, d, target) })
			}
		}
	})
}
func toggleString(values []string, value string) []string {
	if i := slices.Index(values, value); i >= 0 {
		return slices.Delete(values, i, i+1)
	}
	return append(values, value)
}

func memberRoleLabels(roles []string) string {
	labels := make([]string, 0, len(roles))
	for _, role := range roles {
		switch role {
		case api.RoleMember:
			labels = append(labels, "Anggota")
		case api.RoleCompanion:
			labels = append(labels, "Pendamping kadiv")
		case api.RoleKadiv:
			labels = append(labels, "Kadiv")
		case api.Role45:
			labels = append(labels, "45")
		case api.RoleOrganizationAdmin:
			labels = append(labels, "Admin organisasi")
		default:
			labels = append(labels, role)
		}
	}
	return strings.Join(labels, " · ")
}
