package appui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// The Space's Anggota tab as on canvas A (#view-anggota): a table of
// members with their role, subdivision and active workload, Ping and
// "Ubah role" on each row, pending invitations and what each role may do.
// On a phone each member is a compact card. The older actions (Atur, Hapus
// anggota, Tunjuk 45) open from "Ubah role", as permissions allow.

// memberRoleBadge is the single role the canvas shows for a member.
func memberRoleBadge(m api.Member) string {
	roles := append([]string{m.Role}, m.Roles...)
	for _, r := range []struct{ code, label string }{{api.RoleKadiv, "Kadiv"}, {api.Role45, "45"}, {api.RoleCompanion, "Pendamping"}, {api.RoleMember, "Anggota"}} {
		if slices.Contains(roles, r.code) {
			return r.label
		}
	}
	if label := memberRoleLabels(m.Roles); label != "" {
		return label
	}
	return "Anggota"
}

// memberLoad counts a member's unfinished tasks among the Space's loaded
// tasks. It reports false while the tasks are not loaded.
func (a *App) memberLoad(uid id.ID) (int, bool) {
	if !a.tasks.loaded || a.tasks.personal {
		return 0, false
	}
	n := 0
	for _, t := range a.tasks.items {
		if t.Assignee.ID == uid && t.Progress != api.TaskDone && t.Progress != api.TaskCancelled {
			n++
		}
	}
	return n, true
}

// memberCanEdit reports whether "Ubah role" has anything to offer for a
// member: base role and removal, or appointing 45.
func (a *App) memberCanEdit(member api.Member) bool {
	if member.Role == api.RoleKadiv {
		return false
	}
	m := &a.access.members
	return a.memberCan(api.CapMembersManage) || (a.memberCan(api.Cap45Manage) && slices.Contains(m.access.Roles, api.RoleKadiv))
}

// pingMember opens the ping composer addressed to one member.
func (a *App) pingMember(uid id.ID) {
	sp := a.selectedSpace()
	if !canPing(sp) {
		return
	}
	st := &a.notificationSettingsState
	if st.period != sp.PeriodID || st.space != sp.ID {
		*st = notificationSettingsState{period: sp.PeriodID, space: sp.ID, division: st.division, device: true, pingKind: "general"}
	}
	st.recipients = []id.ID{uid}
	a.chrome.pingOpen = true
}

func (a *App) membersPage(c *ui.Context) {
	t := c.Theme()
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
	if d != id.Nil {
		a.ensureTasks(false)
	}
	m := &a.access.members
	phone := isMobile(c)
	ui.Scroll(c).Fill().Children(func() {
		body := ui.Column(c).FillWidth().MinWidth(0).Padding(24).Gap(16)
		if phone {
			body.Padding(16).Gap(12)
		}
		body.Children(func() {
			head := ui.Column(c).FillWidth().MinWidth(0).Gap(12)
			wide := head.Bounds().W >= 640
			if wide {
				head.Row().AlignItems(ui.Center)
			}
			head.Children(func() {
				titles := ui.Column(c).MinWidth(0).Gap(4)
				if wide {
					titles.Grow(1)
				}
				titles.Children(func() {
					ui.Text(c, "Anggota dan role").FontSize(20).Bold()
					ui.Text(c, "Semua anggota bisa melihat beban kerja rekan. Hanya kadiv asli yang mengatur role, undangan, dan 45.").FontSize(13).TextColor(t.TextMuted)
				})
				ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
					if d == id.Nil {
						return
					}
					if spaceIconButton(c, "Muat ulang anggota", "refresh").Disabled(m.busy || m.loading).Clicked() {
						a.loadMembers(d)
					}
					if a.archiveDivisionAllowed(d) {
						if spaceButton(c, "Kelola izin arsip", "archive", false).Border(1, t.Border).Clicked() {
							a.selectArchiveDivision(d)
							a.navigate(PathArchiveAccess)
						}
					}
					if a.memberCan(api.CapInvitationsManage) {
						if primarySpaceButton(c, "Undang anggota", "plus").Disabled(m.busy).Clicked() {
							m.inviteOpen = !m.inviteOpen
						}
					}
				})
			})
			if d == id.Nil {
				pageNotice(c, "lock", "Direktori tidak tersedia", "Akses ruang ini tidak memberikan akses ke direktori divisi.")
				return
			}
			readable := 0
			for _, div := range s.Divisions {
				if a.spacePermissionsForDivision(div.ID, api.CapMembersRead) {
					readable++
				}
			}
			if readable > 1 {
				ui.Row(c).FillWidth().Gap(8).Wrap().Children(func() {
					for _, div := range s.Divisions {
						if a.spacePermissionsForDivision(div.ID, api.CapMembersRead) {
							if spaceButton(c.Key(div.ID), div.Name, "users", d == div.ID).Clicked() {
								a.loadMembers(div.ID)
							}
						}
					}
				})
			}
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
			if m.inviteOpen && a.memberCan(api.CapInvitationsManage) {
				a.memberInviteForm(c)
			}
			a.memberTable(c, phone)
			if a.memberCan(api.CapInvitationsManage) {
				a.memberInvitations(c)
			}
			if a.memberCan(api.CapRolesManage) {
				a.memberRoles(c)
			}
			ui.Column(c).FillWidth().MinWidth(0).Padding(14, 16).Gap(6).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
				ui.Text(c, "Kewenangan role").FontSize(13).Bold()
				ui.Text(c, "Kadiv: semua, termasuk role, undangan, 45, dan bot. · 45: operasional kadiv dan bot, tanpa role/akses. · Pendamping: delegasi, tenggat, ping, tinjauan. · Anggota: tugas sendiri, komentar, ajukan bantuan.").FontSize(13).TextColor(t.TextMuted).LineHeight(1.6)
			})
		})
	})
}

// memberColumns are the table's column widths after Nama, which grows.
var memberColumns = []struct {
	label string
	width float32
}{{"Role", 110}, {"Subdivisi", 100}, {"Beban aktif", 100}, {"WhatsApp", 90}, {"", 170}}

func (a *App) memberTable(c *ui.Context, phone bool) {
	t := c.Theme()
	m := &a.access.members
	if len(m.members) == 0 {
		pageEmpty(c, "users", "Belum ada anggota", "Belum ada keanggotaan aktif dalam divisi ini.")
		return
	}
	panel := ui.Column(c).FillWidth().MinWidth(0).Radius(12).Background(t.Surface).Border(1, t.Border)
	table := !phone && panel.Bounds().W >= 720
	panel.Children(func() {
		if table {
			ui.Row(c).FillWidth().Padding(8, 12).Gap(12).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				ui.Text(c, "NAMA").FontSize(11).TextColor(faint(t)).Grow(1).MinWidth(0)
				for _, col := range memberColumns {
					ui.Text(c.Key(col.label), strings.ToUpper(col.label)).FontSize(11).TextColor(faint(t)).Width(col.width).Shrink(0)
				}
			})
		}
		for i, member := range m.members {
			row := ui.Column(c.Key(member.UserID)).FillWidth().MinWidth(0)
			if i < len(m.members)-1 || m.editing == member.UserID {
				row.BorderWidth(0, 0, 1, 0).BorderColor(t.Border)
			}
			row.Children(func() {
				if table {
					a.memberRow(c, member)
				} else {
					a.memberCard(c, member)
				}
				if m.editing == member.UserID && a.memberCanEdit(member) {
					a.memberRowEditor(c, member)
				}
			})
		}
	})
}

func memberBadge(c *ui.Context, label string) ui.Element {
	t := c.Theme()
	b := ui.Text(c, label).FontSize(11).TextColor(t.TextMuted).Padding(2, 8).Radius(6).Border(1, t.Border).MaxLines(1)
	if label == "Kadiv" || label == "45" {
		b.BorderColor(t.Accent).TextColor(t.Text)
	}
	return b
}

func memberGroups(member api.Member) string {
	if len(member.Groups) == 0 {
		return "—"
	}
	return strings.Join(member.Groups, ", ")
}

func (a *App) memberLoadText(member api.Member) string {
	if n, ok := a.memberLoad(member.UserID); ok {
		return fmt.Sprintf("%d tugas", n)
	}
	return "—"
}

func (a *App) memberRow(c *ui.Context, member api.Member) {
	t := c.Theme()
	ui.Row(c).FillWidth().MinWidth(0).MinHeight(46).Padding(8, 12).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Grow(1).MinWidth(0).Gap(10).AlignItems(ui.Center).Children(func() {
			avatar(c, member.DisplayName, 22)
			ui.Text(c, member.DisplayName).FontSize(13).MaxLines(1).MinWidth(0)
		})
		ui.Box(c).Width(memberColumns[0].width).Shrink(0).Children(func() { memberBadge(c, memberRoleBadge(member)) })
		ui.Text(c, memberGroups(member)).FontSize(13).TextColor(t.TextMuted).Width(memberColumns[1].width).Shrink(0).MaxLines(1)
		ui.Text(c, a.memberLoadText(member)).Font(FontMono).FontSize(12).Width(memberColumns[2].width).Shrink(0)
		wa := ui.Text(c, memberWhatsApp(member)).FontSize(12).Width(memberColumns[3].width).Shrink(0).MaxLines(1)
		switch {
		case member.WhatsAppLinked == nil:
			wa.TextColor(faint(t)).Tooltip("Status tautan WhatsApp belum tersedia dari server")
		case *member.WhatsAppLinked:
			wa.TextColor(t.Success)
		default:
			wa.TextColor(t.TextMuted)
		}
		ui.Row(c).Width(memberColumns[4].width).Shrink(0).Gap(4).Justify(ui.End).Children(func() { a.memberActions(c, member) })
	})
}

func (a *App) memberCard(c *ui.Context, member api.Member) {
	t := c.Theme()
	ui.Column(c).FillWidth().MinWidth(0).Padding(12, 14).Gap(8).Children(func() {
		ui.Row(c).FillWidth().MinWidth(0).Gap(10).AlignItems(ui.Center).Children(func() {
			avatar(c, member.DisplayName, 28)
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, member.DisplayName).FontSize(14).FontWeight(600).MaxLines(1)
				line := memberGroups(member) + " · " + a.memberLoadText(member)
				if member.WhatsAppLinked != nil {
					line += " · WA " + strings.ToLower(memberWhatsApp(member))
				}
				ui.Text(c, line).FontSize(12).TextColor(t.TextMuted).MaxLines(1)
			})
			memberBadge(c, memberRoleBadge(member)).Shrink(0)
		})
		ui.Row(c).FillWidth().Gap(4).Justify(ui.End).Children(func() { a.memberActions(c, member) })
	})
}

// memberAction is a quiet text button of a row, as the canvas' "Ping".
func memberAction(c *ui.Context, text, label string, on bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(label)).Label(label).Tooltip(label).Height(30).Padding(0, 10).Radius(7).Center().TextColor(t.TextMuted)
	if on {
		b.Background(accentSoft(t)).TextColor(t.Text)
	} else if b.Hovered() {
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.FocusRing(b.FocusVisible()).Children(func() { ui.Text(c, text).FontSize(12) })
	return b
}

func (a *App) memberActions(c *ui.Context, member api.Member) {
	m := &a.access.members
	if canPing(a.selectedSpace()) && member.UserID != a.user.ID {
		if memberAction(c, "Ping", "Ping "+member.DisplayName, false).Clicked() {
			a.pingMember(member.UserID)
		}
	}
	if a.memberCanEdit(member) {
		if memberAction(c, "Ubah role", "Ubah role "+member.DisplayName, m.editing == member.UserID).Disabled(m.busy).Clicked() {
			if m.editing == member.UserID {
				m.editing = id.Nil
			} else {
				m.editing = member.UserID
				m.role = member.Role
				m.groups = slices.Clone(member.Groups)
				m.roleIDs = slices.Clone(member.CustomRoleIDs)
			}
		}
	}
}

// memberRowEditor opens under a row from "Ubah role": the base role,
// groups and custom roles, removing the member and appointing 45, each as
// the viewer's permissions allow.
func (a *App) memberRowEditor(c *ui.Context, member api.Member) {
	t := c.Theme()
	m := &a.access.members
	ui.Column(c).FillWidth().MinWidth(0).Padding(4, 16, 16, 16).Gap(10).Background(spaceMix(t.Surface, t.Background, .5)).Children(func() {
		if len(member.CustomRoleIDs) > 0 {
			names := []string{}
			for _, r := range m.roles {
				if slices.Contains(member.CustomRoleIDs, r.ID) {
					names = append(names, r.Name)
				}
			}
			muted(c, "Role kustom: "+strings.Join(names, ", "))
		}
		if a.memberCan(api.CapMembersManage) {
			a.memberEditor(c, member)
		}
		ui.Row(c).FillWidth().Gap(8).Wrap().Children(func() {
			if a.memberCan(api.CapMembersManage) && !slices.Contains(member.Roles, api.Role45) {
				if spaceButton(c, "Hapus anggota "+member.DisplayName, "close", false).TextColor(t.Danger).Disabled(m.busy).Clicked() {
					m.confirmAction = "remove-member"
					m.confirmID = member.UserID
				}
			}
			if a.memberCan(api.Cap45Manage) && slices.Contains(m.access.Roles, api.RoleKadiv) {
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
	})
}

func (a *App) memberInviteForm(c *ui.Context) {
	t := c.Theme()
	m := &a.access.members
	ui.Column(c).FillWidth().MinWidth(0).Padding(14, 16).Gap(10).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, "Undang anggota").FontSize(13).Bold()
		muted(c, "Dikirim ke email anggota. Akun baru memverifikasi email sebelum bergabung.")
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.End).Children(func() {
			ui.TextInput(c.Key("invite-email"), &m.email).Label("Email undangan").Placeholder("nama@example.com").Grow(1).MinWidth(0).Disabled(m.busy)
			if primarySpaceButton(c, "Kirim undangan", "send").Disabled(m.busy || strings.TrimSpace(m.email) == "").Clicked() {
				req := api.InviteRequest{Email: strings.TrimSpace(m.email)}
				a.memberWrite(api.CapInvitationsManage, "Undangan dikirim ke email.", func(s WorkspaceAPI, p, d id.ID) error {
					_, err := s.Invite(context.Background(), p, d, req)
					return err
				})
			}
		})
	})
}

// memberInvitations is the canvas' "Undangan tertunda" card: invitations
// still pending or expired, with resend and revoke.
func (a *App) memberInvitations(c *ui.Context) {
	t := c.Theme()
	m := &a.access.members
	var open []api.Invitation
	for _, inv := range m.invitations {
		if inv.Status == api.InvitationPending || inv.Status == api.InvitationExpired {
			open = append(open, inv)
		}
	}
	ui.Column(c).FillWidth().MinWidth(0).Padding(14, 16).Gap(8).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Undangan tertunda").FontSize(13).Bold().Grow(1)
			ui.Text(c, fmt.Sprintf("%d undangan", len(open))).FontSize(11).TextColor(faint(t))
		})
		if len(open) == 0 {
			muted(c, "Tidak ada undangan yang menunggu.")
		}
		for _, inv := range open {
			row := ui.Column(c.Key(inv.ID)).FillWidth().MinWidth(0).Gap(6).Padding(6, 0)
			wide := row.Bounds().W >= 560
			if wide {
				row.Row().AlignItems(ui.Center).Gap(12)
			}
			row.Children(func() {
				ui.Text(c, inv.Email).FontSize(13).MaxLines(1).Grow(1).MinWidth(0)
				status := "dikirim " + fmt.Sprintf("%d %s", inv.CreatedAt.Day(), monthName(inv.CreatedAt.Month())[:3]) + " · berlaku hingga " + inv.ExpiresAt.Format("02 Jan 15:04")
				if inv.Status == api.InvitationExpired {
					status = "kedaluwarsa " + inv.ExpiresAt.Format("02 Jan 2006")
				}
				ui.Text(c, status).FontSize(12).TextColor(t.TextMuted).Shrink(0)
				ui.Row(c).Gap(4).Shrink(0).Children(func() {
					if memberAction(c, "Kirim ulang", "Kirim ulang "+inv.Email, false).Disabled(m.busy).Clicked() {
						a.memberWrite(api.CapInvitationsManage, "Undangan dikirim ulang. Tautan sebelumnya tidak berlaku.", func(s WorkspaceAPI, p, d id.ID) error {
							_, err := s.ResendInvitation(context.Background(), p, d, inv.ID)
							return err
						})
					}
					if memberAction(c, "Cabut", "Cabut undangan "+inv.Email, false).Disabled(m.busy).Clicked() {
						m.confirmAction = "revoke-invite"
						m.confirmID = inv.ID
					}
				})
			})
		}
	})
}

// memberWhatsApp is the table's WhatsApp cell: "Tertaut" or "Belum", or
// "—" while the server does not tell.
func memberWhatsApp(member api.Member) string {
	switch {
	case member.WhatsAppLinked == nil:
		return "—"
	case *member.WhatsAppLinked:
		return "Tertaut"
	}
	return "Belum"
}
