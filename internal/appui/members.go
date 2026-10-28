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
func (a *App) membersPage(c *ui.Context) {
	pageBody(c, "Anggota space", func() {
		s := a.selectedSpace()
		if s == nil {
			return
		}
		d := a.access.members.division
		if d == id.Nil {
			d = a.memberDivision()
			if d != id.Nil {
				a.loadMembers(d)
			}
		}
		if d == id.Nil {
			pageNotice(c, "lock", "Direktori tidak tersedia", "Akses ruang ini tidak memberikan akses ke direktori divisi.")
			return
		}
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			for _, div := range s.Divisions {
				if a.spacePermissionsForDivision(div.ID, api.CapMembersRead) {
					if spaceButton(c.Key(div.ID), div.Name, "users", d == div.ID).Clicked() {
						a.loadMembers(div.ID)
					}
				}
			}
		})
		if a.archiveDivisionAllowed(d) {
			if spaceButton(c, "Kelola izin arsip", "archive", false).Clicked() {
				a.selectArchiveDivision(d)
				a.navigate(PathArchiveAccess)
			}
		}
		m := &a.access.members
		if m.loading {
			muted(c, "Memuat anggota dan kewenangan…")
			return
		}
		if m.err != "" {
			pageNotice(c, "alert", "Permintaan belum berhasil", m.err)
		}
		if m.notice != "" {
			pageNotice(c, "check", "Perubahan tersimpan", m.notice)
		}
		if spaceButton(c, "Muat ulang anggota", "refresh", false).Disabled(m.busy).Clicked() {
			a.loadMembers(d)
		}
		if !m.loaded {
			return
		}
		if m.access.ReadOnly {
			pageNotice(c, "lock", "Hanya baca", "Keanggotaan periode yang ditutup tidak dapat diubah.")
		}
		if slices.Contains(m.access.Roles, api.Role45) {
			pageNotice(c, "shield", "Kewenangan 45", "Kamu dapat mengelola pekerjaan dan WhatsApp sesuai izin. Pengaturan anggota, role, dan akses tetap dipegang kadiv asli.")
		}
		if m.confirmAction != "" {
			a.memberConfirmation(c)
		}
		for _, member := range m.members {
			pageSection(c.Key(member.UserID), member.DisplayName, memberRoleLabels(member.Roles), func() {
				if len(member.Groups) > 0 {
					muted(c, "Kelompok: "+strings.Join(member.Groups, ", "))
				}
				if len(member.CustomRoleIDs) > 0 {
					names := []string{}
					for _, r := range m.roles {
						if slices.Contains(member.CustomRoleIDs, r.ID) {
							names = append(names, r.Name)
						}
					}
					muted(c, "Role kustom: "+strings.Join(names, ", "))
				}
				if a.memberCan(api.CapMembersManage) && member.Role != api.RoleKadiv {
					if spaceButton(c, "Atur "+member.DisplayName, "settings", m.editing == member.UserID).Disabled(m.busy).Clicked() {
						m.editing = member.UserID
						m.role = member.Role
						m.groups = slices.Clone(member.Groups)
						m.roleIDs = slices.Clone(member.CustomRoleIDs)
					}
					if m.editing == member.UserID {
						a.memberEditor(c, member)
					}
					if !slices.Contains(member.Roles, api.Role45) && spaceButton(c, "Hapus anggota "+member.DisplayName, "close", false).Disabled(m.busy).Clicked() {
						m.confirmAction = "remove-member"
						m.confirmID = member.UserID
					}
				}
				if a.memberCan(api.Cap45Manage) && slices.Contains(m.access.Roles, api.RoleKadiv) && member.Role != api.RoleKadiv {
					label := "Tunjuk 45 " + member.DisplayName
					action := "appoint45"
					if slices.Contains(member.Roles, api.Role45) {
						label = "Copot 45 " + member.DisplayName
						action = "remove45"
					}
					if spaceButton(c, label, "shield", false).Disabled(m.busy).Clicked() {
						m.confirmAction = action
						m.confirmID = member.UserID
						m.actingCustom = false
						m.actingCaps = []string{api.CapTasksManage, api.CapResultsReview, api.CapPingSend, api.CapWhatsAppManage}
					}
				}
			})
		}
		if len(m.members) == 0 {
			pageEmpty(c, "users", "Belum ada anggota", "Belum ada keanggotaan aktif dalam divisi ini.")
		}
		if a.memberCan(api.CapInvitationsManage) {
			a.memberInvitations(c)
		}
		if a.memberCan(api.CapRolesManage) {
			a.memberRoles(c)
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
func (a *App) memberInvitations(c *ui.Context) {
	m := &a.access.members
	pageSection(c, "Undangan anggota", "Dikirim ke email anggota. Akun baru memverifikasi email sebelum bergabung.", func() {
		ui.TextInput(c.Key("invite-email"), &m.email).Label("Email undangan").Placeholder("nama@example.com").FillWidth().Disabled(m.busy)
		if spaceButton(c, "Kirim undangan", "send", true).Disabled(m.busy || strings.TrimSpace(m.email) == "").Clicked() {
			req := api.InviteRequest{Email: strings.TrimSpace(m.email)}
			a.memberWrite(api.CapInvitationsManage, "Undangan dikirim ke email.", func(s WorkspaceAPI, p, d id.ID) error {
				_, err := s.Invite(context.Background(), p, d, req)
				return err
			})
		}
		for _, inv := range m.invitations {
			ui.Column(c.Key(inv.ID)).FillWidth().Gap(6).Padding(10).Children(func() {
				ui.Text(c, inv.Email).Bold()
				muted(c, inv.Status+" · berlaku hingga "+inv.ExpiresAt.Format("02 Jan 2006 15:04"))
				if inv.Status == api.InvitationPending || inv.Status == api.InvitationExpired {
					if spaceButton(c, "Kirim ulang "+inv.Email, "refresh", false).Disabled(m.busy).Clicked() {
						a.memberWrite(api.CapInvitationsManage, "Undangan dikirim ulang. Tautan sebelumnya tidak berlaku.", func(s WorkspaceAPI, p, d id.ID) error {
							_, err := s.ResendInvitation(context.Background(), p, d, inv.ID)
							return err
						})
					}
					if spaceButton(c, "Cabut undangan "+inv.Email, "close", false).Disabled(m.busy).Clicked() {
						m.confirmAction = "revoke-invite"
						m.confirmID = inv.ID
					}
				}
			})
		}
	})
}

var operationalRoleChoices = []struct{ cap, label string }{{api.CapTasksManage, "Kelola tugas"}, {api.CapResultsReview, "Tinjau hasil"}, {api.CapPingSend, "Kirim ping"}, {api.CapWhatsAppManage, "Kelola WhatsApp"}, {api.CapNotesManage, "Kelola catatan"}}

func (a *App) memberRoles(c *ui.Context) {
	m := &a.access.members
	pageSection(c, "Role kustom", "Role tambahan hanya memberi kewenangan operasional. Role 45 tidak dapat mengatur akses.", func() {
		for _, r := range m.roles {
			ui.Row(c.Key(r.ID)).FillWidth().Gap(6).Children(func() {
				ui.Text(c, r.Name).Grow(1)
				if spaceButton(c, "Edit role "+r.Name, "settings", false).Disabled(m.busy).Clicked() {
					m.roleEditing = r.ID
					m.roleName = r.Name
					m.roleCaps = slices.Clone(r.Capabilities)
				}
				if spaceButton(c, "Hapus role "+r.Name, "close", false).Disabled(m.busy).Clicked() {
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
