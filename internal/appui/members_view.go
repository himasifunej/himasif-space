package appui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// The Space's Anggota tab as on canvas A (#view-anggota): a table of
// members with their role, subdivision and active workload, Ping and
// "Ubah role" on each row, pending invitations and what each role may do.
// On a phone each member is a compact card. "Ubah akses" opens the access
// drawer (stage A2): base role, 45, groups, custom roles and removal.

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

// memberCanEdit reports whether "Ubah akses" has anything to offer for a
// member: base role, groups, custom roles and removal, or the 45.
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
					ui.Text(c, "Anggota").FontSize(20).Bold()
					sub := "Beban kerja terlihat oleh semua anggota. Hanya kadiv yang mengatur role, undangan, dan 45."
					if m.loaded && len(m.members) > 0 {
						sub = fmt.Sprintf("%d anggota · %s", len(m.members), sub)
					}
					ui.Text(c, sub).FontSize(13).TextColor(t.TextMuted)
				})
				ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
					if d == id.Nil {
						return
					}
					if spaceIconButton(c, "Muat ulang anggota", "refresh").Disabled(m.busy || m.loading).Clicked() {
						m.pinned = nil // a reload sorts the table again
						a.loadMembers(d)
					}
					if a.archiveDivisionAllowed(d) {
						if outlineButton(c.Key("member-archive"), "Izin arsip", "", false).Tooltip("Kelola izin arsip").Clicked() {
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
			if m.loading && !m.loaded {
				muted(c, "Memuat anggota dan kewenangan…")
				return
			}
			if m.err != "" {
				pageNotice(c, "alert", "Permintaan belum berhasil", m.err)
			}
			if m.notice != "" {
				// A toast, so the table below does not move.
				c.AddToast(ui.Toast{Title: "Perubahan tersimpan", Description: m.notice, Type: "success", Timeout: 5 * time.Second})
				m.notice = ""
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
			a.memberAccessPanel(c)
			a.memberBottomPanel(c)
			if a.memberCan(api.CapRolesManage) {
				a.memberRoles(c)
			}
		})
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
		if memberAction(c, "Ubah akses", "Ubah akses "+member.DisplayName, m.editing == member.UserID).Disabled(m.busy).Clicked() {
			a.openMemberAccess(member)
		}
	}
	a.memberRowMenu(c, member)
}

// memberRowMenu is the row's ⋯ (stage A4): Lihat tugas, and Keluarkan…
// for who may remove members (the drawer's removal dialog, with its check
// of active tasks).
func (a *App) memberRowMenu(c *ui.Context, member api.Member) {
	m := &a.access.members
	canRemove := a.memberCan(api.CapMembersManage) && member.Role != api.RoleKadiv && !m.access.ReadOnly
	if a.selectedSpace() == nil && !canRemove {
		return
	}
	b := spaceIconButton(c.Key("row-more"), "Lainnya untuk "+member.DisplayName, "more")
	if b.Clicked() {
		if m.rowMenu == member.UserID {
			m.rowMenu = id.Nil
		} else {
			m.rowMenu = member.UserID
		}
	}
	open := m.rowMenu == member.UserID
	popover(c, b, &open, 220, func() {
		t := c.Theme()
		item := func(key, label, icon string, danger bool) bool {
			e := ui.ButtonBase(c.Key(key)).Label(label).Row().FillWidth().Height(32).Padding(0, 8).Gap(8).AlignItems(ui.Center).Radius(7).TextColor(t.Text)
			if danger {
				e.TextColor(t.Danger)
			}
			if e.Hovered() {
				e.Background(t.SurfaceHover)
			}
			clicked := e.Clicked()
			e.FocusRing(e.FocusVisible()).Children(func() {
				ui.Icon(c, spaceIcon(icon)).Size(14, 14).Shrink(0)
				ui.Text(c, label).FontSize(13).MaxLines(1)
			})
			return clicked
		}
		if a.selectedSpace() != nil && item("menu-tasks", "Lihat tugas", "board", false) {
			m.rowMenu = id.Nil
			a.showMemberTasks(member, id.Nil)
		}
		if canRemove && item("menu-remove", "Keluarkan…", "close", true) {
			m.rowMenu = id.Nil
			a.openMemberAccess(member)
			m.removeAsk = true
			a.loadMemberRemoveTasks(member)
		}
	})
	if !open && m.rowMenu == member.UserID {
		m.rowMenu = id.Nil
	}
}

// memberRoleSummary is the short "Kewenangan role" of the bottom panel.
var memberRoleSummary = []struct{ role, text string }{
	{"Kadiv", "Semua, termasuk role, undangan, 45 dan bot."},
	{"45", "Operasional kadiv dan bot, tanpa role dan akses."},
	{"Pendamping", "Delegasi, tenggat, ping, tinjauan."},
	{"Anggota", "Tugas sendiri, komentar, ajukan bantuan."},
}

// memberBottomPanel is the canvas' bottom of the Anggota page (stage A4):
// "Undangan tertunda" (for who may invite) beside a short "Kewenangan
// role"; one above the other on a narrow screen.
func (a *App) memberBottomPanel(c *ui.Context) {
	t := c.Theme()
	invites := a.memberCan(api.CapInvitationsManage)
	row := ui.Column(c.Key("member-bottom")).FillWidth().MinWidth(0).Gap(12)
	wide := row.Bounds().W >= 760
	if wide {
		row.Row().AlignItems(ui.Start)
	}
	row.Children(func() {
		if invites {
			ui.Column(c.Key("bottom-invites")).Grow(1).FillWidth().MinWidth(0).Children(func() { a.memberInvitations(c) })
		}
		card := ui.Column(c.Key("bottom-roles")).FillWidth().MinWidth(0).Padding(14, 16).Gap(8).Radius(12).Background(t.Surface).Border(1, t.Border)
		if wide && invites {
			card.Width(340).Shrink(0)
		}
		card.Children(func() {
			ui.Text(c, "Kewenangan role").FontSize(13).Bold()
			for _, r := range memberRoleSummary {
				ui.Row(c.Key(r.role)).FillWidth().MinWidth(0).Gap(10).AlignItems(ui.Start).Children(func() {
					ui.Text(c, r.role).FontSize(12).FontWeight(600).Width(84).Shrink(0)
					ui.Text(c, r.text).FontSize(12).TextColor(t.TextMuted).LineHeight(1.5).Grow(1).MinWidth(0)
				})
			}
		})
	})
}

func (a *App) memberInviteForm(c *ui.Context) {
	t := c.Theme()
	m := &a.access.members
	ui.Column(c).FillWidth().MinWidth(0).Padding(14, 16).Gap(10).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, "Undang anggota").FontSize(13).Bold()
		muted(c, "Ketik NIM mahasiswa; undangan dikirim ke email kampusnya. Akun baru memverifikasi email sebelum bergabung.")
		target := inviteTarget(m.email)
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.End).Children(func() {
			ui.TextInput(c.Key("invite-email"), &m.email).Label("NIM atau email").Placeholder("232410101090").Grow(1).MinWidth(0).Disabled(m.busy)
			if primarySpaceButton(c, "Kirim undangan", "send").Disabled(m.busy || target.req == nil).Clicked() {
				req := *target.req
				a.memberWrite(api.CapInvitationsManage, "Undangan dikirim ke "+target.email+".", func(s WorkspaceAPI, p, d id.ID) error {
					_, err := s.Invite(context.Background(), p, d, req)
					return err
				})
			}
		})
		switch {
		case target.problem != "":
			ui.Text(c, target.problem).FontSize(12).TextColor(t.Danger)
		case target.email != "":
			ui.Row(c).Gap(6).AlignItems(ui.Center).MinWidth(0).Children(func() {
				ui.Icon(c, spaceIcon("send")).Size(14, 14).TextColor(t.TextMuted).Shrink(0)
				muted(c, "Dikirim ke").FontSize(12).Shrink(0)
				ui.Text(c, target.email).FontSize(12).FontWeight(600).MaxLines(1).MinWidth(0).Selectable()
			})
			if target.warning != "" {
				ui.Text(c, target.warning).FontSize(12).TextColor(t.Warning)
			}
		}
	})
}

// inviteTargetInfo is what the invite form would send for its input.
type inviteTargetInfo struct {
	req              *api.InviteRequest
	email            string // the address shown before sending
	warning, problem string
}

// inviteTarget reads the invite field: a NIM becomes the student address
// <NIM>@mail.unej.ac.id; a full email is sent as typed, with a warning when
// it is not a student address (lecturers and alumni have other emails).
func inviteTarget(input string) inviteTargetInfo {
	s := strings.TrimSpace(input)
	switch {
	case s == "":
		return inviteTargetInfo{}
	case !strings.Contains(s, "@"):
		if !api.ValidNIM(s) {
			return inviteTargetInfo{problem: "NIM hanya berisi angka, 8 sampai 20 digit. Untuk alamat lain, ketik email lengkap."}
		}
		return inviteTargetInfo{req: &api.InviteRequest{NIM: s}, email: api.StudentEmail(s)}
	}
	email := strings.ToLower(s)
	at := strings.LastIndexByte(email, '@')
	if at < 1 || at == len(email)-1 || !strings.Contains(email[at:], ".") || strings.ContainsAny(email, " ,;<>") {
		return inviteTargetInfo{problem: "Alamat email belum lengkap."}
	}
	out := inviteTargetInfo{req: &api.InviteRequest{Email: email}, email: email}
	if !api.IsStudentEmail(email) {
		out.warning = "Bukan email mahasiswa UNEJ (@" + api.StudentEmailDomain + "). Tetap bisa dikirim, misalnya untuk dosen atau alumni."
	}
	return out
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
				if inv.BouncedAt != nil {
					// Amazon SES reported the email did not arrive: the NIM or
					// address is probably wrong. Sending again clears it.
					reason := inv.BounceReason
					if reason == "" {
						reason = "Email tidak sampai."
					}
					ui.Text(c, "Tidak terkirim · "+reason).FontSize(12).TextColor(t.Danger).MinWidth(0).MaxLines(2).Tooltip("Periksa NIM atau alamatnya. Bila salah, cabut lalu undang ulang; bila benar, kirim ulang.")
				} else {
					ui.Text(c, status).FontSize(12).TextColor(t.TextMuted).Shrink(0)
				}
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
