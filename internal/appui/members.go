package appui

import (
	"context"
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
	editing               id.ID
	role                  string
	groups                []string
	roleIDs               []id.ID
	roleEditing           id.ID
	roleName              string
	roleCaps              []string
	confirmAction         string
	confirmID             id.ID
	inviteOpen            bool
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
	oldseq := a.access.members.seq
	a.access.members = memberManagementState{division: d, loading: true, seq: oldseq + 1}
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
			if err != nil {
				a.access.members.err = a.accessFailure(err)
				return
			}
			a.loadMembers(d)
			a.access.members.notice = label
			a.loadSpaces()
		}
	})
}
func (a *App) memberEditor(c *ui.Context, member api.Member) {
	m := &a.access.members
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		ui.Text(c, "Role dasar").Bold()
		for _, r := range []struct{ code, label string }{{api.RoleMember, "Anggota"}, {api.RoleCompanion, "Pendamping kadiv"}} {
			if spaceButton(c, r.label, "user", m.role == r.code).Disabled(m.busy).Clicked() {
				m.role = r.code
			}
		}
		isMediatek := false
		if s := a.selectedSpace(); s != nil {
			for _, d := range s.Divisions {
				if d.ID == m.division && d.Slug == "mediatek" {
					isMediatek = true
				}
			}
		}
		if isMediatek {
			ui.Text(c, "Kelompok (tanpa tambahan kewenangan)").Bold()
			for _, group := range []string{api.GroupTekno, api.GroupDesain, api.GroupVideo} {
				if spaceButton(c, group, "check", slices.Contains(m.groups, group)).Disabled(m.busy).Clicked() {
					m.groups = toggleString(m.groups, group)
				}
			}
		}
		ui.Text(c, "Role kustom").Bold()
		for _, r := range m.roles {
			if spaceButton(c.Key(r.ID), r.Name, "shield", slices.Contains(m.roleIDs, r.ID)).Disabled(m.busy).Clicked() {
				if i := slices.Index(m.roleIDs, r.ID); i >= 0 {
					m.roleIDs = slices.Delete(m.roleIDs, i, i+1)
				} else {
					m.roleIDs = append(m.roleIDs, r.ID)
				}
			}
		}
		if spaceButton(c, "Simpan akses "+member.DisplayName, "check", true).Disabled(m.busy).Clicked() {
			role := m.role
			req := api.UpdateMemberRequest{Role: &role, Groups: append([]string{}, m.groups...), CustomRoleIDs: append([]id.ID{}, m.roleIDs...)}
			a.memberWrite(api.CapMembersManage, "Akses anggota diperbarui.", func(s WorkspaceAPI, p, d id.ID) error {
				_, err := s.UpdateMember(context.Background(), p, d, member.UserID, req)
				return err
			})
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
	text := map[string]string{"remove-member": "Hapus keanggotaan ini? Akses divisi akan dicabut.", "revoke-invite": "Cabut undangan ini? Tautan email sebelumnya tidak dapat digunakan.", "delete-role": "Hapus role ini beserta penugasannya?", "appoint45": "Tunjuk sebagai 45? Ia mendapat kewenangan operasional kadiv, termasuk WhatsApp, sampai dicopot; tanpa pengaturan role atau akses.", "remove45": "Copot 45? Kewenangan tambahan langsung dicabut."}[action]
	pageSection(c, "Konfirmasi perubahan", text, func() {
		if action == "appoint45" {
			if spaceButton(c, "Sesuaikan kewenangan 45", "settings", m.actingCustom).Disabled(m.busy).Clicked() {
				m.actingCustom = !m.actingCustom
			}
			if m.actingCustom {
				for _, r := range operationalRoleChoices {
					if !slices.Contains([]string{api.CapTasksManage, api.CapResultsReview, api.CapPingSend, api.CapWhatsAppManage}, r.cap) {
						continue
					}
					if spaceButton(c, r.label, "check", slices.Contains(m.actingCaps, r.cap)).Disabled(m.busy).Clicked() {
						m.actingCaps = toggleString(m.actingCaps, r.cap)
					}
				}
			}
		}

		if spaceButton(c, "Batal perubahan", "close", false).Disabled(m.busy).Clicked() {
			m.confirmAction = ""
		}
		if action == "appoint45" && m.actingCustom && len(m.actingCaps) == 0 {
			muted(c, "Pilih minimal satu kewenangan 45, atau gunakan kewenangan default.")
		}
		if spaceButton(c, "Konfirmasi perubahan", "check", true).Disabled(m.busy || (action == "appoint45" && m.actingCustom && len(m.actingCaps) == 0)).Clicked() {
			switch action {
			case "remove-member":
				a.memberWrite(api.CapMembersManage, "Keanggotaan dicabut.", func(s WorkspaceAPI, p, d id.ID) error { return s.RemoveMember(context.Background(), p, d, target) })
			case "revoke-invite":
				a.memberWrite(api.CapInvitationsManage, "Undangan dicabut.", func(s WorkspaceAPI, p, d id.ID) error { return s.RevokeInvitation(context.Background(), p, d, target) })
			case "delete-role":
				a.memberWrite(api.CapRolesManage, "Role dihapus.", func(s WorkspaceAPI, p, d id.ID) error { return s.DeleteRole(context.Background(), p, d, target) })
			case "appoint45":
				req := api.Appoint45Request{UserID: target}
				if m.actingCustom {
					req.Capabilities = append([]string{}, m.actingCaps...)
				}
				a.memberWrite(api.Cap45Manage, "45 ditunjuk sampai dicopot kadiv asli.", func(s WorkspaceAPI, p, d id.ID) error {
					_, err := s.Appoint45(context.Background(), p, d, req)
					return err
				})
			case "remove45":
				a.memberWrite(api.Cap45Manage, "Kewenangan 45 dicabut.", func(s WorkspaceAPI, p, d id.ID) error { return s.Remove45(context.Background(), p, d) })
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
