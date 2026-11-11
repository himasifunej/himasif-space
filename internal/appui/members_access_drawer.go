package appui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/authz"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Stage A2 (canvas Anggota A, 0b2425c385f0): "Ubah akses" opens a drawer
// from the right on a desktop and a bottom sheet on a phone. Top to
// bottom: the base role as radio cards, the 45 switch (replacing another
// 45 asks first), groups, custom roles, what the member will be able to
// do, removal in a red box, and a footer counting unsaved changes.

// memberBaseRoles are the base roles a kadiv gives, with what they mean.
var memberBaseRoles = []struct{ code, label, desc string }{
	{api.RoleMember, "Anggota", "Mengerjakan tugas sendiri, berkomentar, mengajukan bantuan."},
	{api.RoleCompanion, "Pendamping kadiv", "Ditambah: mendelegasikan tugas, mengubah tenggat, ping, tinjauan."},
}

// memberSummaryCaps are the capabilities the summary lists, in order.
var memberSummaryCaps = []struct{ cap, label string }{
	{api.CapTasksOwn, "Mengerjakan tugas sendiri"},
	{api.CapCommentsWrite, "Berkomentar dan mengajukan bantuan"},
	{api.CapNotesManage, "Kelola catatan"},
	{api.CapTasksManage, "Kelola tugas"},
	{api.CapResultsReview, "Tinjau hasil"},
	{api.CapPingSend, "Kirim ping"},
	{api.CapWhatsAppManage, "Kelola WhatsApp"},
	{api.CapSOPRead, "Baca evaluasi SOP"},
	{api.CapSOPReview, "Tinjau temuan SOP"},
}

// memberCapSource is one line of the summary: a capability and where it
// comes from ("" for the base role).
type memberCapSource struct{ label, from string }

// memberAccessSummary is what a member will be able to do with a base
// role, the 45 (with its capabilities) and custom roles, and what not.
func memberAccessSummary(role string, acting45 bool, caps45 []string, custom []api.CustomRole) (can []memberCapSource, cannot []string) {
	base := authz.RoleCaps(role)
	for _, s := range memberSummaryCaps {
		switch {
		case slices.Contains(base, s.cap):
			can = append(can, memberCapSource{s.label, ""})
		case acting45 && slices.Contains(caps45, s.cap):
			can = append(can, memberCapSource{s.label, "dari 45"})
		default:
			from := ""
			for _, r := range custom {
				if slices.Contains(r.Capabilities, s.cap) {
					from = "dari " + r.Name
					break
				}
			}
			if from != "" {
				can = append(can, memberCapSource{s.label, from})
			} else {
				cannot = append(cannot, s.label)
			}
		}
	}
	return can, cannot
}

func (a *App) memberByID(uid id.ID) *api.Member {
	m := &a.access.members
	for i := range m.members {
		if m.members[i].UserID == uid {
			return &m.members[i]
		}
	}
	return nil
}

// member45 is the division's current 45, if any.
func (a *App) member45() *api.Member {
	m := &a.access.members
	for i := range m.members {
		if slices.Contains(m.members[i].Roles, api.Role45) {
			return &m.members[i]
		}
	}
	return nil
}

// memberCan45 reports whether the viewer may appoint or remove the 45:
// only the original kadiv.
func (a *App) memberCan45() bool {
	return a.memberCan(api.Cap45Manage) && slices.Contains(a.access.members.access.Roles, api.RoleKadiv)
}

// openMemberAccess opens the drawer for a member, with the member's
// current access as the starting point.
func (a *App) openMemberAccess(member api.Member) {
	m := &a.access.members
	m.editing = member.UserID
	m.role = member.Role
	m.groups = slices.Clone(member.Groups)
	m.roleIDs = slices.Clone(member.CustomRoleIDs)
	m.on45 = slices.Contains(member.Roles, api.Role45)
	m.replace45, m.replaceAsk, m.removeAsk, m.discardAsk = false, false, false, false
	m.actingCustom = false
	m.actingCaps = slices.Clone(authz.Default45)
	m.err, m.notice = "", ""
	m.drawerOpen = true
}

// closeMemberAccess closes the drawer; unsaved changes are dropped.
func (a *App) closeMemberAccess() {
	m := &a.access.members
	m.editing = id.Nil
	m.drawerOpen, m.replaceAsk, m.removeAsk, m.discardAsk = false, false, false, false
}

// leaveMemberAccess closes the drawer from its X, the backdrop or Escape:
// with unsaved changes it asks first instead of dropping them.
func (a *App) leaveMemberAccess(member api.Member) {
	m := &a.access.members
	if len(a.memberAccessChanges(member)) > 0 && !m.busy {
		m.drawerOpen, m.discardAsk = true, true
		return
	}
	a.closeMemberAccess()
}

// memberAccessChanges names what differs from the member's saved access.
func (a *App) memberAccessChanges(member api.Member) []string {
	m := &a.access.members
	var out []string
	if m.role != member.Role {
		out = append(out, "role dasar")
	}
	if m.on45 != slices.Contains(member.Roles, api.Role45) {
		out = append(out, "45")
	}
	if !sameSet(m.groups, member.Groups) {
		out = append(out, "kelompok")
	}
	if !sameSet(m.roleIDs, member.CustomRoleIDs) {
		out = append(out, "role kustom")
	}
	return out
}

func sameSet[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for _, v := range a {
		if !slices.Contains(b, v) {
			return false
		}
	}
	return true
}

// memberStepError is a save that did part of its work: done says what was
// saved before err.
type memberStepError struct {
	done string
	err  error
}

func (e *memberStepError) Error() string { return e.done + " " + e.err.Error() }
func (e *memberStepError) Unwrap() error { return e.err }

// saveMemberAccess saves the drawer: the member's role, groups and custom
// roles in one request, then the 45 (removing the current one first when
// it is replaced), each step's failure told on its own.
func (a *App) saveMemberAccess(member api.Member) {
	m := &a.access.members
	changes := a.memberAccessChanges(member)
	if len(changes) == 0 || m.busy {
		return
	}
	was45 := slices.Contains(member.Roles, api.Role45)
	updateMember := m.role != member.Role || !sameSet(m.groups, member.Groups) || !sameSet(m.roleIDs, member.CustomRoleIDs)
	role := m.role
	req := api.UpdateMemberRequest{Role: &role, Groups: append([]string{}, m.groups...), CustomRoleIDs: append([]id.ID{}, m.roleIDs...)}
	appoint := !was45 && m.on45
	remove := was45 && !m.on45
	removeOther := appoint && m.replace45
	appointReq := api.Appoint45Request{UserID: member.UserID}
	if m.actingCustom {
		appointReq.Capabilities = append([]string{}, m.actingCaps...)
	}
	cap := api.CapMembersManage
	if !updateMember {
		cap = api.Cap45Manage
	}
	a.memberWrite(cap, "Akses "+member.DisplayName+" disimpan.", func(s WorkspaceAPI, p, d id.ID) error {
		done := ""
		if updateMember {
			if _, err := s.UpdateMember(context.Background(), p, d, member.UserID, req); err != nil {
				return err
			}
			done = "Role, kelompok dan role kustom tersimpan, tetapi"
		}
		step := func(err error, what string) error {
			if err == nil {
				return nil
			}
			if done == "" {
				return err
			}
			return &memberStepError{done: done + " " + what + " gagal:", err: err}
		}
		if remove || removeOther {
			if err := step(s.Remove45(context.Background(), p, d), "mencopot 45"); err != nil {
				return err
			}
			if removeOther && done == "" {
				done = "45 sebelumnya sudah dicopot, tetapi"
			}
		}
		if appoint {
			_, err := s.Appoint45(context.Background(), p, d, appointReq)
			if err := step(err, "menunjuk 45"); err != nil {
				return err
			}
		}
		return nil
	})
}

// memberAccessPanel shows the drawer or sheet while one is open.
func (a *App) memberAccessPanel(c *ui.Context) {
	m := &a.access.members
	if m.editing == id.Nil {
		return
	}
	member := a.memberByID(m.editing)
	if member == nil || !a.memberCanEdit(*member) {
		a.closeMemberAccess()
		return
	}
	t := c.Theme()
	vw, vh := c.Size()
	phone := isMobile(c) || vw < 720
	ui.DialogBase(c.Key("member-access"), &m.drawerOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		if phone {
			backdrop.Justify(ui.End).AlignItems(ui.Stretch)
			panel.FillWidth().Height(vh*0.92).Radius(16, 16, 0, 0).Padding(4, 0, 0, 0)
		} else {
			backdrop.Row().Justify(ui.End).AlignItems(ui.Stretch)
			panel.Width(min(float32(480), vw-48)).FillHeight().BorderWidth(0, 0, 0, 1)
		}
		panel.Background(t.Surface).BorderColor(t.Border).Border(1, t.Border).Label("Ubah akses " + member.DisplayName).Children(func() {
			if phone {
				ui.Box(c.Key("grip")).Size(36, 4).Radius(2).Background(t.Border).AlignSelf(ui.Center).Margin(4, 0, 4, 0)
			}
			a.memberAccessHead(c, *member)
			ui.Scroll(c.Key("member-access-body")).Grow(1).MinHeight(0).FillWidth().Label("Isi ubah akses").Children(func() {
				ui.Column(c).FillWidth().MinWidth(0).Padding(20, 20, 24, 20).Gap(20).Children(func() {
					a.memberAccessBody(c, *member)
				})
			})
			a.memberAccessFoot(c, *member)
		})
		if m.replaceAsk {
			a.memberReplace45Dialog(c, *member)
		}
	})
	if !m.drawerOpen && m.editing != id.Nil {
		a.leaveMemberAccess(*member)
	}
}

func (a *App) memberAccessHead(c *ui.Context, member api.Member) {
	t := c.Theme()
	ui.Row(c).FillWidth().MinWidth(0).Padding(16, 20).Gap(12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		avatar(c, member.DisplayName, 40)
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, "Ubah akses · "+member.DisplayName).FontSize(16).Bold().MaxLines(2)
			parts := []string{}
			if len(member.Groups) > 0 {
				parts = append(parts, memberGroups(member))
			}
			if active, _, ok := a.memberWork(member.UserID, time.Now()); ok {
				parts = append(parts, fmt.Sprintf("%d tugas aktif", active))
			}
			if len(parts) > 0 {
				ui.Text(c, strings.Join(parts, " · ")).FontSize(12).TextColor(t.TextMuted).MaxLines(1)
			}
		})
		if spaceIconButton(c, "Tutup ubah akses", "close").Clicked() {
			a.leaveMemberAccess(member)
		}
	})
}

// memberFirstName is how the drawer speaks of a member: "Aruna".
func memberFirstName(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, "[") {
		if i := strings.Index(name, "]"); i >= 0 {
			name = strings.TrimSpace(name[i+1:])
		}
	}
	if f := strings.Fields(name); len(f) > 0 {
		return f[0]
	}
	return name
}

func memberSection(c *ui.Context, title, desc string, fn func()) {
	t := c.Theme()
	ui.Column(c.Key("section-" + title)).FillWidth().MinWidth(0).Gap(8).Children(func() {
		ui.Text(c, title).FontSize(14).Bold()
		if desc != "" {
			ui.Text(c, desc).FontSize(13).TextColor(t.TextMuted).LineHeight(1.5)
		}
		fn()
	})
}

func (a *App) memberAccessBody(c *ui.Context, member api.Member) {
	t := c.Theme()
	m := &a.access.members
	first := memberFirstName(member.DisplayName)
	division := "divisi ini"
	if d := a.memberDivisionInfo(); d != nil {
		division = d.Name
	}
	canMembers := a.memberCan(api.CapMembersManage)
	if canMembers {
		memberSection(c, "Role dasar", "Menentukan apa yang boleh dilakukan "+first+" di "+division+".", func() {
			for _, r := range memberBaseRoles {
				if memberRadioCard(c, r.label, r.desc, m.role == r.code).Disabled(m.busy).Clicked() {
					m.role = r.code
				}
			}
		})
	}
	if a.memberCan45() {
		a.memberAccess45(c, member)
	}
	if canMembers && a.memberDivisionIsMediatek() {
		memberSection(c, "Kelompok", "Hanya pengelompokan untuk filter dan board, tidak menambah kewenangan.", func() {
			ui.Row(c).FillWidth().Gap(8).Wrap().Children(func() {
				for _, g := range []string{api.GroupTekno, api.GroupDesain, api.GroupVideo} {
					label := strings.ToUpper(g[:1]) + g[1:]
					if memberToggleChip(c, label, "Kelompok "+label, slices.Contains(m.groups, g)).Disabled(m.busy).Clicked() {
						m.groups = toggleString(m.groups, g)
					}
				}
			})
		})
	}
	if canMembers {
		desc := "Menambah kewenangan operasional di atas role dasar."
		if len(m.roles) == 0 {
			desc = "Belum ada role kustom."
		}
		memberSection(c, "Role kustom", desc, func() {
			defer func() {
				text := "Atur role kustom"
				if len(m.roles) == 0 {
					text = "Buat role kustom"
				}
				if a.memberCan(api.CapRolesManage) && onbLink(c, text, text).Clicked() {
					a.openRoles()
				}
			}()
			for _, r := range m.roles {
				on := slices.Contains(m.roleIDs, r.ID)
				ui.Column(c.Key(r.ID)).FillWidth().MinWidth(0).Gap(2).Children(func() {
					before := on
					ui.Checkbox(c, &on, r.Name).Disabled(m.busy)
					if on != before {
						if on {
							m.roleIDs = append(m.roleIDs, r.ID)
						} else if i := slices.Index(m.roleIDs, r.ID); i >= 0 {
							m.roleIDs = slices.Delete(m.roleIDs, i, i+1)
						}
					}
					if labels := memberCapLabels(r.Capabilities); labels != "" {
						ui.Text(c, labels).FontSize(12).TextColor(t.TextMuted).Padding(0, 0, 0, 28)
					}
				})
			}
		})
	}
	a.memberAccessSummaryView(c, member, first)
	if canMembers {
		a.memberRemoveBox(c, member, first, division)
	}
}

func (a *App) memberDivisionInfo() *api.Division {
	m := &a.access.members
	if s := a.selectedSpace(); s != nil {
		for i := range s.Divisions {
			if s.Divisions[i].ID == m.division {
				return &s.Divisions[i]
			}
		}
	}
	return nil
}

func (a *App) memberDivisionIsMediatek() bool {
	d := a.memberDivisionInfo()
	return d != nil && d.Slug == "mediatek"
}

// memberCapLabels lists the operational capabilities of a custom role.
func memberCapLabels(caps []string) string {
	var out []string
	for _, r := range operationalRoleChoices {
		if slices.Contains(caps, r.cap) {
			out = append(out, r.label)
		}
	}
	return strings.Join(out, ", ")
}

// memberAccess45 is the "Wakil operasional (45)" switch. Turning it on
// while another member is the 45 asks first: there is one 45 a division.
func (a *App) memberAccess45(c *ui.Context, member api.Member) {
	t := c.Theme()
	m := &a.access.members
	was45 := slices.Contains(member.Roles, api.Role45)
	ui.Column(c.Key("section-45")).FillWidth().MinWidth(0).Gap(8).Children(func() {
		ui.Row(c).FillWidth().MinWidth(0).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Wakil operasional (45)").FontSize(14).Bold().Grow(1).MinWidth(0)
			on := m.on45
			ui.Switch(c.Key("switch-45"), &on).Label("Wakil operasional (45) " + member.DisplayName).Disabled(m.busy)
			if on != m.on45 {
				other := a.member45()
				if on && !was45 && other != nil && other.UserID != member.UserID {
					m.replaceAsk = true
				} else {
					m.on45, m.replace45 = on, false
				}
			}
		})
		desc := "Mendapat kewenangan operasional kadiv, termasuk Bot WhatsApp, sampai dicopot. Tetap tidak bisa mengatur role atau akses."
		switch {
		case was45 && m.on45:
			desc = memberFirstName(member.DisplayName) + " adalah 45 divisi ini. " + desc
		case was45:
			desc = "Kewenangan 45 dicopot saat kamu menyimpan."
		case m.on45 && m.replace45:
			if other := a.member45(); other != nil {
				desc = "Menggantikan " + other.DisplayName + " sebagai 45 saat kamu menyimpan. " + desc
			}
		case !m.on45:
			if other := a.member45(); other != nil {
				desc = "45 divisi ini sekarang " + other.DisplayName + ". Hanya ada satu 45 per divisi."
			}
		}
		ui.Text(c, desc).FontSize(13).TextColor(t.TextMuted).LineHeight(1.5)
		if m.on45 && !was45 {
			if spaceButton(c, "Sesuaikan kewenangan 45", "settings", m.actingCustom).Disabled(m.busy).Clicked() {
				m.actingCustom = !m.actingCustom
			}
			if m.actingCustom {
				ui.Row(c).FillWidth().Gap(8).Wrap().Children(func() {
					for _, r := range operationalRoleChoices {
						if !slices.Contains(authz.Default45, r.cap) {
							continue
						}
						if memberToggleChip(c, r.label, "Kewenangan 45 "+r.label, slices.Contains(m.actingCaps, r.cap)).Disabled(m.busy).Clicked() {
							m.actingCaps = toggleString(m.actingCaps, r.cap)
						}
					}
				})
				if len(m.actingCaps) == 0 {
					ui.Text(c, "Pilih minimal satu kewenangan 45.").FontSize(12).TextColor(t.Danger)
				}
			}
		}
	})
}

// memberReplace45Dialog asks before the switch replaces the current 45.
func (a *App) memberReplace45Dialog(c *ui.Context, member api.Member) {
	t := c.Theme()
	m := &a.access.members
	other := a.member45()
	if other == nil {
		m.replaceAsk = false
		m.on45 = true
		return
	}
	ui.DialogBase(c.Key("replace-45"), &m.replaceAsk, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		panel.Width(min(float32(420), mustWidth(c)-32)).Padding(20).Gap(12).Radius(14).Background(t.Surface).Border(1, t.Border).Label("Dialog ganti 45").Children(func() {
			ui.Text(c, "Ganti 45 dari "+other.DisplayName+" ke "+member.DisplayName+"?").FontSize(16).Bold()
			ui.Text(c, "Satu divisi hanya punya satu 45. Saat kamu menyimpan, "+other.DisplayName+" kehilangan kewenangan 45 dan "+member.DisplayName+" mendapatkannya.").FontSize(13).TextColor(t.TextMuted).LineHeight(1.5)
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				if spaceButton(c, "Batal ganti 45", "", false).Clicked() {
					m.replaceAsk = false
				}
				if primarySpaceButton(c, "Ganti 45", "check").Clicked() {
					m.on45, m.replace45, m.replaceAsk = true, true, false
				}
			})
		})
	})
}

func mustWidth(c *ui.Context) float32 {
	w, _ := c.Size()
	return w
}

func (a *App) memberAccessSummaryView(c *ui.Context, member api.Member, first string) {
	t := c.Theme()
	m := &a.access.members
	var custom []api.CustomRole
	for _, r := range m.roles {
		if slices.Contains(m.roleIDs, r.ID) {
			custom = append(custom, r)
		}
	}
	caps45 := m.actingCaps
	if !m.actingCustom {
		caps45 = authz.Default45
	}
	if slices.Contains(member.Roles, api.Role45) {
		caps45 = member.Capabilities
	}
	can, cannot := memberAccessSummary(m.role, m.on45, caps45, custom)
	ui.Column(c.Key("summary")).FillWidth().MinWidth(0).Gap(6).Padding(14, 16).Radius(10).Background(spaceMix(t.Surface, t.Background, .5)).Border(1, t.Border).Children(func() {
		ui.Text(c, "Yang bisa dilakukan "+first).FontSize(14).Bold()
		for _, s := range can {
			ui.Row(c.Key(s.label)).FillWidth().MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, spaceIcon("check")).Size(14, 14).TextColor(t.Success).Shrink(0)
				ui.Text(c, s.label).FontSize(13).Grow(1).MinWidth(0)
				if s.from != "" {
					ui.Text(c, s.from).FontSize(12).TextColor(t.Accent).Shrink(0)
				}
			})
		}
		if len(cannot) > 0 {
			ui.Text(c, "Tidak bisa: "+strings.Join(cannot, ", ")+".").FontSize(12).TextColor(t.TextMuted).LineHeight(1.5)
		}
		ui.Row(c).FillWidth().MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, spaceIcon("lock")).Size(14, 14).TextColor(faint(t)).Shrink(0)
			ui.Text(c, "Atur role, undangan dan 45 · hanya kadiv").FontSize(12).TextColor(faint(t))
		})
	})
}

// memberRemoveBox is "Keluarkan dari …" in red, at the end of the drawer.
// A 45 is removed from the 45 first; the confirmation is a dialog.
func (a *App) memberRemoveBox(c *ui.Context, member api.Member, first, division string) {
	t := c.Theme()
	m := &a.access.members
	ui.Column(c.Key("remove")).FillWidth().MinWidth(0).Gap(8).Padding(14, 16).Radius(10).Border(1, spaceMix(t.Danger, t.Surface, .55)).Background(spaceMix(t.Danger, t.Surface, .9)).Children(func() {
		ui.Text(c, "Keluarkan dari "+division).FontSize(14).Bold().TextColor(t.Danger)
		if slices.Contains(member.Roles, api.Role45) {
			ui.Text(c, first+" masih 45. Matikan sakelar 45 dan simpan dulu, lalu keluarkan.").FontSize(13).TextColor(t.TextMuted).LineHeight(1.5)
			return
		}
		ui.Text(c, "Akses divisi dicabut. Tugas aktifnya perlu dialihkan dulu.").FontSize(13).TextColor(t.TextMuted).LineHeight(1.5)
		if spaceButton(c, "Keluarkan "+first+"…", "close", false).TextColor(t.Danger).Border(1, spaceMix(t.Danger, t.Surface, .55)).Disabled(m.busy).Clicked() {
			m.removeAsk = true
			a.loadMemberRemoveTasks(member)
		}
	})
	if m.removeAsk {
		a.memberRemoveDialog(c, member, first, division)
	}
}

// MemberTasksAPI asks the server what keeps a member from being removed.
// It is optional: without it the dialog counts the open Space's tasks.
type MemberTasksAPI interface {
	MemberActiveTasks(ctx context.Context, p, d, u id.ID) (api.MemberActiveTasks, error)
}

func (a *App) memberTasksService() (MemberTasksAPI, bool) {
	if live, ok := a.api.(*client.Client); ok {
		return live.AccountSession(), true
	}
	s, ok := a.api.(MemberTasksAPI)
	return s, ok
}

// memberTasksFromError reads the refusal of a removal for active tasks.
func memberTasksFromError(err error) (api.MemberActiveTasks, bool) {
	var ce *client.Error
	if !errors.As(err, &ce) || ce.Code != api.CodeMemberHasTasks {
		return api.MemberActiveTasks{}, false
	}
	var tasks api.MemberActiveTasks
	raw, jerr := json.Marshal(ce.Data)
	if jerr != nil || json.Unmarshal(raw, &tasks) != nil || tasks.Count < 1 {
		// The code alone still says there are tasks.
		return api.MemberActiveTasks{Count: 1}, true
	}
	return tasks, true
}

// loadMemberRemoveTasks asks the server for a member's active tasks, for
// the removal dialog.
func (a *App) loadMemberRemoveTasks(member api.Member) {
	m := &a.access.members
	m.removeTasks, m.removeTasksFor, m.removeTasksLocal, m.removeTasksLoading = nil, member.UserID, false, true
	svc, ok := a.memberTasksService()
	if !ok {
		m.removeTasksLoading, m.removeTasksLocal = false, true
		return
	}
	gen, p, d, seq, uid := a.gen, a.periodID, m.division, m.seq, member.UserID
	a.async(func() func() {
		tasks, err := svc.MemberActiveTasks(context.Background(), p, d, uid)
		return func() {
			m := &a.access.members
			if gen != a.gen || p != a.periodID || d != m.division || seq != m.seq || m.removeTasksFor != uid {
				return
			}
			m.removeTasksLoading = false
			if err != nil {
				// An older server or no connection: count the open Space's
				// tasks; the server checks again when removing.
				m.removeTasksLocal = true
				return
			}
			m.removeTasks = &tasks
		}
	})
}

// memberRemoveCheck is what the removal dialog knows about a member's
// active tasks.
type memberRemoveCheck struct {
	count   int
	places  string                // " (Mediatek 2, Medhum 1)"
	spaces  []api.MemberTaskSpace // where they are, from the server
	known   bool                  // counted, by the server or locally
	local   bool                  // counted from the open Space only
	allowed bool
}

// memberRemoveState is what the removal dialog may do. The server's count
// covers every Space of the division; without it the open Space's tasks
// are counted. Removal waits while counting and is refused while the
// member is still the PJ of an active task (owner decision for stage A3;
// moving tasks in bulk comes later). The server refuses it too.
func (a *App) memberRemoveState(member api.Member) memberRemoveCheck {
	m := &a.access.members
	var r memberRemoveCheck
	switch {
	case m.removeTasksFor == member.UserID && m.removeTasks != nil:
		r.count, r.places, r.spaces, r.known = m.removeTasks.Count, m.removeTasks.Places(), m.removeTasks.Spaces, true
	case m.removeTasksFor == member.UserID && m.removeTasksLocal:
		active, _, ok := a.memberWork(member.UserID, time.Now())
		r.count, r.known, r.local = active, ok, true
	}
	r.allowed = r.known && r.count == 0 && !slices.Contains(member.Roles, api.Role45)
	return r
}

// showMemberTasks closes the drawer and opens a Space's board (the open
// one when space is nil) filtered to the member, to give their tasks
// another PJ.
func (a *App) showMemberTasks(member api.Member, space id.ID) {
	a.closeMemberAccess()
	if space != id.Nil && space != a.selectedSpaceID() {
		a.selectSpace(space)
	}
	// Bind the task list to the Space first: binding it later would clear
	// the filter.
	a.ensureTasks(false)
	a.chrome.tab = "Board tugas"
	a.tasks.filter = member.DisplayName
	a.projectTasks()
}

// memberCanOpenSpace reports whether the viewer may open a Space.
func (a *App) memberCanOpenSpace(space id.ID) bool {
	return slices.ContainsFunc(a.access.spaces, func(s api.Space) bool { return s.ID == space })
}

const memberLocalCountNote = "Dihitung dari Space ini saja; server memeriksa semua Space divisi saat mengeluarkan."

func (a *App) memberRemoveDialog(c *ui.Context, member api.Member, first, division string) {
	t := c.Theme()
	m := &a.access.members
	check := a.memberRemoveState(member)
	ui.DialogBase(c.Key("remove-member"), &m.removeAsk, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		panel.Width(min(float32(440), mustWidth(c)-32)).Padding(20).Gap(12).Radius(14).Background(t.Surface).Border(1, t.Border).Label("Dialog keluarkan anggota").Children(func() {
			ui.Text(c, "Keluarkan "+member.DisplayName+" dari "+division+"?").FontSize(16).Bold()
			for _, l := range []string{"Akses ke board, catatan dan Bot WhatsApp divisi dicabut segera.", "Riwayat komentar dan hasil kerjanya tetap ada."} {
				ui.Text(c.Key(l), "· "+l).FontSize(13).TextColor(t.TextMuted).LineHeight(1.5)
			}
			switch {
			case !check.known:
				ui.Text(c, "Menghitung tugas aktif "+first+"…").FontSize(13).TextColor(t.TextMuted)
			case check.count > 0:
				ui.Column(c.Key("tasks-left")).FillWidth().MinWidth(0).Gap(8).Padding(12, 14).Radius(10).Background(spaceMix(t.Warning, t.Surface, .88)).Border(1, spaceMix(t.Warning, t.Surface, .5)).Children(func() {
					ui.Text(c, fmt.Sprintf("%s masih PJ %d tugas aktif%s.", first, check.count, check.places)).FontSize(13).Bold().LineHeight(1.5)
					ui.Text(c, "Alihkan PJ tugas-tugas itu ke anggota lain dulu, lalu keluarkan "+first+".").FontSize(13).TextColor(t.TextMuted).LineHeight(1.5)
					shown := 0
					for _, sp := range check.spaces {
						if !a.memberCanOpenSpace(sp.SpaceID) {
							continue
						}
						shown++
						if spaceButton(c.Key(sp.SpaceID), "Lihat tugas di "+sp.Name, "board", false).Border(1, t.Border).Clicked() {
							m.removeAsk = false
							a.showMemberTasks(member, sp.SpaceID)
						}
					}
					if shown == 0 && spaceButton(c, "Lihat tugas "+first, "board", false).Border(1, t.Border).Clicked() {
						m.removeAsk = false
						a.showMemberTasks(member, id.Nil)
					}
					if check.local {
						ui.Text(c, memberLocalCountNote).FontSize(12).TextColor(faint(t)).LineHeight(1.5)
					}
				})
			default:
				ui.Text(c, first+" tidak punya tugas aktif.").FontSize(13).TextColor(t.TextMuted)
				if check.local {
					ui.Text(c, memberLocalCountNote).FontSize(12).TextColor(faint(t)).LineHeight(1.5)
				}
			}
			if reason := memberRemoveBlockedReason(check, first); reason != "" {
				// Said in words, so a blocked button never looks like one that
				// just did not react (0.3.4: unclear in the dark theme).
				ui.Row(c.Key("remove-reason")).FillWidth().MinWidth(0).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, spaceIcon("lock")).Size(13, 13).TextColor(t.TextMuted).Shrink(0)
					ui.Text(c, reason).FontSize(12).TextColor(t.TextMuted).Grow(1).MinWidth(0).LineHeight(1.4)
				})
			}
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				if spaceButton(c, "Batal keluarkan", "", false).Disabled(m.busy).Clicked() {
					m.removeAsk = false
				}
				if !check.allowed {
					memberBlockedButton(c, "Keluarkan anggota", memberRemoveBlockedReason(check, first))
					return
				}
				text := "Keluarkan anggota"
				if m.busy {
					text = "Mengeluarkan…"
				}
				b := spaceButton(c.Key("remove-confirm"), text, "close", true).Label("Keluarkan anggota").Disabled(m.busy)
				b.Background(t.Danger).TextColor(ui.RGBA(255, 255, 255, 1))
				if b.Clicked() && check.allowed && !m.busy {
					uid := member.UserID
					a.memberWrite(api.CapMembersManage, member.DisplayName+" dikeluarkan dari "+division+".", func(s WorkspaceAPI, p, d id.ID) error {
						return s.RemoveMember(context.Background(), p, d, uid)
					})
				}
			})
		})
	})
}

// memberRemoveBlockedReason says why removal is not possible yet, or ""
// when it is.
func memberRemoveBlockedReason(check memberRemoveCheck, first string) string {
	switch {
	case check.allowed:
		return ""
	case !check.known:
		return "Belum bisa dikeluarkan: tugas aktif " + first + " masih dihitung."
	case check.count > 0:
		return fmt.Sprintf("Belum bisa dikeluarkan: %s masih PJ %d tugas aktif.", first, check.count)
	}
	return "Belum bisa dikeluarkan."
}

// memberBlockedButton is a button that cannot be used, drawn so in both
// themes: a muted fill, faint text and a lock instead of the action's
// icon; it does nothing when clicked.
func memberBlockedButton(c *ui.Context, label, reason string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key("blocked-"+label)).Label(label).Tooltip(reason).Row().Gap(6).Height(36).Padding(0, 14).Radius(8).AlignItems(ui.Center).
		Background(spaceMix(t.Surface, t.Background, .5)).Border(1, spaceMix(t.Border, t.Background, .5)).TextColor(faint(t)).Cursor(ui.CursorNotAllowed).Disabled(true)
	b.Children(func() {
		ui.Icon(c, spaceIcon("lock")).Size(14, 14)
		ui.Text(c, label).FontSize(13)
	})
	return b
}

func (a *App) memberAccessFoot(c *ui.Context, member api.Member) {
	t := c.Theme()
	m := &a.access.members
	changes := a.memberAccessChanges(member)
	ui.Column(c).FillWidth().MinWidth(0).Padding(12, 20, 16, 20).Gap(8).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		if m.err != "" {
			ui.Text(c, m.err).FontSize(12).TextColor(t.Danger).LineHeight(1.5)
		}
		if m.discardAsk && len(changes) > 0 {
			ui.Text(c, fmt.Sprintf("Buang %d perubahan yang belum disimpan (%s)?", len(changes), strings.Join(changes, ", "))).FontSize(13).Bold().LineHeight(1.5)
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				if spaceButton(c, "Lanjut mengubah", "", false).Clicked() {
					m.discardAsk = false
				}
				if primarySpaceButton(c, "Buang perubahan", "close").Clicked() {
					a.closeMemberAccess()
				}
			})
			return
		}
		ui.Row(c).FillWidth().MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
			status := "Belum ada perubahan"
			color := t.TextMuted
			if n := len(changes); n > 0 {
				status = fmt.Sprintf("%d perubahan belum disimpan", n)
				color = t.Accent
			}
			if m.busy {
				status = "Menyimpan…"
			}
			ui.Text(c, status).FontSize(12).TextColor(color).Grow(1).MinWidth(0).MaxLines(2)
			if spaceButton(c, "Batal", "", false).Disabled(m.busy).Clicked() {
				a.closeMemberAccess()
			}
			blocked := len(changes) == 0 || m.busy || (m.on45 && m.actingCustom && len(m.actingCaps) == 0)
			if primarySpaceButton(c, "Simpan akses", "check").Disabled(blocked).Clicked() {
				a.saveMemberAccess(member)
			}
		})
	})
}

// memberRadioCard is a base-role choice: a radio, its name and meaning.
func memberRadioCard(c *ui.Context, title, desc string, on bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key("role-"+title)).Label("Role dasar "+title).Row().FillWidth().MinWidth(0).Gap(12).Padding(12, 14).Radius(10).Border(1, t.Border).AlignItems(ui.Start)
	if on {
		b.Background(accentSoft(t)).BorderColor(spaceMix(t.Accent, t.Background, .45))
	} else if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		ring := ui.Box(c).Size(18, 18).Radius(9).Border(2, t.Border).Shrink(0).Center().Margin(1, 0, 0, 0)
		if on {
			ring.BorderColor(t.Accent).Children(func() { ui.Box(c).Size(8, 8).Radius(4).Background(t.Accent) })
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, title).FontSize(14).FontWeight(600)
			ui.Text(c, desc).FontSize(13).TextColor(t.TextMuted).LineHeight(1.5)
		})
	})
	return b
}

// memberToggleChip is a chip that is on or off, with a check when on.
func memberToggleChip(c *ui.Context, text, label string, on bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key("toggle-"+label)).Label(label).Row().AlignItems(ui.Center).Gap(6).Height(32).Padding(0, 14).Radius(16).Border(1, t.Border).TextColor(t.TextMuted).Shrink(0)
	if on {
		b.Background(accentSoft(t)).BorderColor(spaceMix(t.Accent, t.Background, .45)).TextColor(t.Accent)
	} else if b.Hovered() {
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		if on {
			ui.Icon(c, spaceIcon("check")).Size(13, 13)
		}
		ui.Text(c, text).FontSize(13)
	})
	return b
}
