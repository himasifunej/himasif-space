package appui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// The Anggota table as on canvas Anggota A (0b2425c385f0, design 1 and 2):
// avatar, a role badge, workload as a bar with the late tasks in red, the
// WhatsApp status, search, filter chips and sorting. Editing access still
// opens under the row from "Ubah role" (moves to a drawer in stage A2).

// Member filters of the toolbar's chips.
const (
	memberFilterAll      = ""
	memberFilterPengurus = "pengurus"
	memberFilterAnggota  = "anggota"
	memberFilterNoWA     = "wa"
	memberFilterLate     = "late"
)

// Member orders of the toolbar's sort button, in the order it cycles.
var memberSorts = []struct{ code, label string }{{"peran", "Peran"}, {"beban", "Beban"}, {"nama", "Nama"}}

// memberRoleRank orders kadiv, 45, pendamping, then members.
func memberRoleRank(m api.Member) int {
	switch memberRoleBadge(m) {
	case "Kadiv":
		return 0
	case "45":
		return 1
	case "Pendamping":
		return 2
	}
	return 3
}

// memberIsPengurus reports a kadiv, 45 or pendamping kadiv.
func memberIsPengurus(m api.Member) bool { return memberRoleRank(m) < 3 }

// memberWork counts a member's unfinished and late tasks among the Space's
// loaded tasks. It reports false while the tasks are not loaded.
func (a *App) memberWork(uid id.ID, now time.Time) (active, late int, ok bool) {
	if !a.tasks.loaded || a.tasks.personal {
		return 0, 0, false
	}
	for _, t := range a.tasks.items {
		if t.Assignee.ID != uid || t.Progress == api.TaskDone || t.Progress == api.TaskCancelled {
			continue
		}
		active++
		if taskLate(t, now) {
			late++
		}
	}
	return active, late, true
}

// memberMatches tells whether a member passes the search text and a filter.
func (a *App) memberMatches(m api.Member, query, filter string, now time.Time) bool {
	if q := strings.ToLower(strings.TrimSpace(query)); q != "" {
		hay := strings.ToLower(m.DisplayName + " " + m.NIM + " " + strings.Join(m.Groups, " ") + " " + memberRoleBadge(m))
		if !strings.Contains(hay, q) {
			return false
		}
	}
	switch filter {
	case memberFilterPengurus:
		return memberIsPengurus(m)
	case memberFilterAnggota:
		return !memberIsPengurus(m)
	case memberFilterNoWA:
		return m.WhatsAppLinked != nil && !*m.WhatsAppLinked
	case memberFilterLate:
		_, late, ok := a.memberWork(m.UserID, now)
		return ok && late > 0
	}
	return true
}

// visibleMembers is the table's rows: searched, filtered and sorted.
func (a *App) visibleMembers(now time.Time) []api.Member {
	m := &a.access.members
	out := []api.Member{}
	for _, member := range m.members {
		if a.memberMatches(member, m.query, m.filter, now) {
			out = append(out, member)
		}
	}
	name := func(x api.Member) string { return strings.ToLower(x.DisplayName) }
	slices.SortStableFunc(out, func(x, y api.Member) int {
		switch m.sort {
		case "nama":
			return strings.Compare(name(x), name(y))
		case "beban":
			xa, xl, _ := a.memberWork(x.UserID, now)
			ya, yl, _ := a.memberWork(y.UserID, now)
			if xl != yl {
				return yl - xl
			}
			if xa != ya {
				return ya - xa
			}
		default:
			if d := memberRoleRank(x) - memberRoleRank(y); d != 0 {
				return d
			}
		}
		return strings.Compare(name(x), name(y))
	})
	if len(m.pinned) > 0 {
		// Pinned rows keep their places; anyone new follows, sorted.
		pos := func(x api.Member) int {
			if i := slices.Index(m.pinned, x.UserID); i >= 0 {
				return i
			}
			return len(m.pinned)
		}
		slices.SortStableFunc(out, func(x, y api.Member) int { return pos(x) - pos(y) })
	}
	return out
}

// pinMemberOrder keeps every member's current row, before a save reloads
// the table.
func (a *App) pinMemberOrder() {
	m := &a.access.members
	if len(m.pinned) > 0 {
		return
	}
	query, filter := m.query, m.filter
	m.query, m.filter = "", memberFilterAll
	for _, member := range a.visibleMembers(time.Now()) {
		m.pinned = append(m.pinned, member.UserID)
	}
	m.query, m.filter = query, filter
}

// memberFilterCount is the number of members a chip would show.
func (a *App) memberFilterCount(filter string, now time.Time) int {
	n := 0
	for _, member := range a.access.members.members {
		if a.memberMatches(member, "", filter, now) {
			n++
		}
	}
	return n
}

func memberSortLabel(code string) string {
	for _, s := range memberSorts {
		if s.code == code {
			return s.label
		}
	}
	return memberSorts[0].label
}

func nextMemberSort(code string) string {
	for i, s := range memberSorts {
		if s.code == code {
			return memberSorts[(i+1)%len(memberSorts)].code
		}
	}
	return memberSorts[1].code
}

// memberChip is a round filter chip with a count.
func memberChip(c *ui.Context, text string, count int, on bool) ui.Element {
	t := c.Theme()
	label := fmt.Sprintf("%s %d", text, count)
	b := ui.ButtonBase(c.Key("chip-"+text)).Label(label).Tooltip(label).Row().AlignItems(ui.Center).Gap(6).Height(30).Padding(0, 12).Radius(15).Border(1, t.Border).Background(t.Surface).TextColor(t.TextMuted).Shrink(0)
	if on {
		b.Background(accentSoft(t)).BorderColor(spaceMix(t.Accent, t.Background, .45)).TextColor(t.Accent)
	} else if b.Hovered() {
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		txt := ui.Text(c, text).FontSize(12).MaxLines(1)
		if on {
			txt.FontWeight(600)
		}
		n := ui.Text(c, fmt.Sprint(count)).FontSize(12).Font(FontMono).TextColor(faint(t))
		if on {
			n.TextColor(t.Accent)
		}
	})
	return b
}

// memberRoleChip is the colored role badge: Kadiv dark, 45 in the accent,
// Pendamping outlined, Anggota quiet; custom roles follow as quiet badges.
func memberRoleChip(c *ui.Context, m api.Member, roles []api.CustomRole) {
	t := c.Theme()
	ui.Row(c).Gap(4).Wrap().MinWidth(0).Children(func() {
		label := memberRoleBadge(m)
		b := ui.Row(c.Key("role")).Gap(4).AlignItems(ui.Center).Height(22).Padding(0, 8).Radius(6).Shrink(0)
		var text ui.Color
		switch label {
		case "Kadiv":
			b.Background(t.Text)
			text = t.Background
		case "45":
			b.Background(accentSoft(t)).Border(1, spaceMix(t.Accent, t.Background, .55))
			text = t.Accent
			label = "45 · Anggota"
		case "Pendamping":
			b.Border(1, t.Border).Background(t.Surface)
			text = t.Text
			label = "Pendamping kadiv"
		default:
			b.Background(t.SurfacePressed)
			text = t.TextMuted
		}
		b.Children(func() {
			if label == "Kadiv" {
				ui.Icon(c, spaceIcon("shield")).Size(12, 12).TextColor(text)
			}
			ui.Text(c, label).FontSize(12).TextColor(text).MaxLines(1)
		})
		for _, r := range roles {
			if slices.Contains(m.CustomRoleIDs, r.ID) {
				ui.Row(c.Key(r.ID)).Height(22).Padding(0, 8).Radius(6).AlignItems(ui.Center).Background(t.SurfacePressed).Children(func() {
					ui.Text(c, r.Name).FontSize(12).TextColor(t.TextMuted).MaxLines(1)
				})
			}
		}
	})
}

// memberLoadBar draws eight segments: late tasks red, other active tasks in
// the muted ink, the rest empty. More than eight fill the bar.
func memberLoadBar(c *ui.Context, active, late int, width float32) {
	t := c.Theme()
	seg := (width - 7*2) / 8
	ui.Row(c).Gap(2).Shrink(0).Children(func() {
		for i := 0; i < 8; i++ {
			color := t.Border
			switch {
			case i < late:
				color = t.Danger
			case i < active:
				color = t.TextMuted
			}
			ui.Box(c.Key(i)).Size(seg, 6).Radius(2).Background(color)
		}
	})
}

// memberLoadLabel is the workload's text: "5 aktif · 1 lewat", "Kosong".
func memberLoadLabel(active, late int) string {
	switch {
	case active == 0:
		return "Kosong"
	case late > 0:
		return fmt.Sprintf("%d aktif · %d lewat", active, late)
	}
	return fmt.Sprintf("%d aktif", active)
}

// memberWhatsApp is the WhatsApp status: "Tertaut", "Belum tertaut", or
// "—" while the server does not tell.
func memberWhatsApp(member api.Member) string {
	switch {
	case member.WhatsAppLinked == nil:
		return "—"
	case *member.WhatsAppLinked:
		return "Tertaut"
	}
	return "Belum tertaut"
}

func memberWhatsAppStatus(c *ui.Context, member api.Member) ui.Element {
	t := c.Theme()
	color, icon := faint(t), ""
	switch {
	case member.WhatsAppLinked == nil:
	case *member.WhatsAppLinked:
		color, icon = t.Success, "whatsapp"
	default:
		color, icon = t.Warning, "clock"
	}
	row := ui.Row(c).Gap(4).AlignItems(ui.Center).MinWidth(0)
	if member.WhatsAppLinked == nil {
		row.Tooltip("Status tautan WhatsApp belum tersedia dari server")
	}
	row.Children(func() {
		if icon != "" {
			ui.Icon(c, spaceIcon(icon)).Size(14, 14).TextColor(color).Shrink(0)
		}
		ui.Text(c, memberWhatsApp(member)).FontSize(12).TextColor(color).MaxLines(1)
	})
	return row
}

// memberColumn is a column of the table after Anggota, which grows.
type memberColumn struct {
	label string
	width float32
}

// memberColumns are the table's fixed columns, narrow enough that names
// keep their room at 1280 px. Kelompok shows only when someone has a
// group (Mediatek).
func memberColumns(groups bool) []memberColumn {
	cols := []memberColumn{{"Role", 150}}
	if groups {
		cols = append(cols, memberColumn{"Kelompok", 80})
	}
	return append(cols, memberColumn{"Beban aktif", 116}, memberColumn{"WhatsApp", 112}, memberColumn{"", 136})
}

// memberHasGroups reports whether any member belongs to a group.
func memberHasGroups(ms []api.Member) bool {
	for _, m := range ms {
		if len(m.Groups) > 0 {
			return true
		}
	}
	return false
}

// memberToolbar is the search field, the filter chips and the sort button.
func (a *App) memberToolbar(c *ui.Context, phone bool, now time.Time) {
	t := c.Theme()
	m := &a.access.members
	box := ui.Column(c).FillWidth().MinWidth(0).Gap(8)
	wide := !phone && box.Bounds().W >= 900
	if wide {
		box.Row().AlignItems(ui.Center)
	}
	box.Children(func() {
		in := ui.TextInput(c.Key("member-search"), &m.query).Label("Cari anggota").Placeholder("Cari nama, NIM atau kelompok")
		if wide {
			in.Width(260).Shrink(0)
		} else {
			in.FillWidth()
		}
		ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).MinWidth(0).Grow(1).Children(func() {
			for _, f := range []struct{ code, text string }{{memberFilterAll, "Semua"}, {memberFilterPengurus, "Pengurus"}, {memberFilterAnggota, "Anggota"}, {memberFilterNoWA, "Belum tertaut"}, {memberFilterLate, "Ada yang lewat"}} {
				if f.code == memberFilterLate && !a.tasks.loaded {
					continue
				}
				if memberChip(c, f.text, a.memberFilterCount(f.code, now), m.filter == f.code).Clicked() {
					m.filter = f.code
				}
			}
			ui.Box(c).Grow(1)
			label := "Urutkan: " + memberSortLabel(m.sort)
			b := ui.ButtonBase(c.Key("member-sort")).Label(label).Tooltip("Ganti urutan").Row().AlignItems(ui.Center).Gap(6).Height(30).Padding(0, 10).Radius(8).TextColor(t.TextMuted).Shrink(0)
			if b.Hovered() {
				b.Background(t.SurfaceHover).TextColor(t.Text)
			}
			b.FocusRing(b.FocusVisible()).Children(func() {
				ui.Text(c, label).FontSize(12)
				ui.Icon(c, spaceIcon("down")).Size(14, 14)
			})
			if b.Clicked() {
				m.sort = nextMemberSort(m.sort)
				m.pinned = nil
			}
		})
	})
}

func (a *App) memberTable(c *ui.Context, phone bool) {
	t := c.Theme()
	m := &a.access.members
	if len(m.members) == 0 {
		pageEmpty(c, "users", "Belum ada anggota", "Belum ada keanggotaan aktif dalam divisi ini. Undang anggota lewat email.")
		return
	}
	now := c.Now()
	a.memberToolbar(c, phone, now)
	rows := a.visibleMembers(now)
	panel := ui.Column(c).FillWidth().MinWidth(0).Radius(12).Background(t.Surface).Border(1, t.Border)
	table := !phone && panel.Bounds().W >= 900
	groups := memberHasGroups(m.members)
	cols := memberColumns(groups)
	panel.Children(func() {
		if table {
			ui.Row(c).FillWidth().Height(40).Padding(0, 16).Gap(16).AlignItems(ui.Center).Background(spaceMix(t.Surface, t.Background, .6)).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				ui.Text(c, "Anggota").FontSize(12).TextColor(faint(t)).Grow(1).MinWidth(0)
				for _, col := range cols {
					ui.Text(c.Key(col.label), col.label).FontSize(12).TextColor(faint(t)).Width(col.width).Shrink(0)
				}
			})
		}
		if len(rows) == 0 {
			ui.Column(c).FillWidth().Padding(32, 24).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, spaceIcon("search")).Size(24, 24).TextColor(faint(t))
				title := "Tidak ada anggota yang cocok"
				if q := strings.TrimSpace(m.query); q != "" {
					title = "Tidak ada anggota bernama “" + q + "”"
				}
				ui.Text(c, title).FontSize(14).Bold()
				muted(c, "Cek ejaan atau ganti filter. Undang lewat email kalau ia belum bergabung.")
				if spaceButton(c, "Hapus pencarian dan filter", "close", false).Border(1, t.Border).Clicked() {
					m.query = ""
					m.filter = memberFilterAll
				}
			})
		}
		for i, member := range rows {
			row := ui.Column(c.Key(member.UserID)).FillWidth().MinWidth(0)
			if i < len(rows)-1 || m.editing == member.UserID {
				row.BorderWidth(0, 0, 1, 0).BorderColor(spaceMix(t.Border, t.Surface, .4))
			}
			if m.editing == member.UserID {
				row.Background(accentSoft(t))
			}
			row.Children(func() {
				if table {
					a.memberRow(c, member, now, groups)
				} else {
					a.memberCard(c, member, now)
				}
			})
		}
	})
}

func (a *App) memberRow(c *ui.Context, member api.Member, now time.Time, groups bool) {
	t := c.Theme()
	m := &a.access.members
	active, late, loaded := a.memberWork(member.UserID, now)
	ui.Row(c).FillWidth().MinWidth(0).MinHeight(56).Padding(8, 16).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Grow(1).MinWidth(0).Gap(12).AlignItems(ui.Center).Children(func() {
			avatar(c, member.DisplayName, 32)
			ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).MinWidth(0).Children(func() {
					ui.Text(c, member.DisplayName).FontSize(13).FontWeight(500).MaxLines(1).MinWidth(0).Tooltip(member.DisplayName)
					if member.UserID == a.user.ID {
						ui.Text(c, "kamu").FontSize(12).TextColor(faint(t)).Shrink(0)
					}
				})
				if member.NIM != "" {
					ui.Text(c, member.NIM).FontSize(11).TextColor(t.TextMuted).MaxLines(1).Selectable()
				}
			})
		})
		cols := memberColumns(groups)
		col := func(label string) float32 {
			for _, c := range cols {
				if c.label == label {
					return c.width
				}
			}
			return 0
		}
		ui.Box(c).Width(col("Role")).Shrink(0).Children(func() { memberRoleChip(c, member, m.roles) })
		if groups {
			ui.Text(c, memberGroups(member)).FontSize(13).TextColor(t.TextMuted).Width(col("Kelompok")).Shrink(0).MaxLines(1)
		}
		ui.Column(c).Width(col("Beban aktif")).Shrink(0).Gap(4).Children(func() {
			if !loaded {
				ui.Text(c, "—").FontSize(12).TextColor(faint(t))
				return
			}
			memberLoadBar(c, active, late, 72)
			ui.Row(c).Gap(0).Children(func() {
				if active == 0 {
					ui.Text(c, "Kosong").FontSize(12).TextColor(faint(t))
					return
				}
				ui.Text(c, fmt.Sprintf("%d aktif", active)).FontSize(12).TextColor(t.TextMuted)
				if late > 0 {
					ui.Text(c, fmt.Sprintf(" · %d lewat", late)).FontSize(12).TextColor(t.Danger)
				}
			})
		})
		ui.Box(c).Width(col("WhatsApp")).Shrink(0).Children(func() { memberWhatsAppStatus(c, member) })
		ui.Row(c).Width(col("")).Shrink(0).Gap(4).Justify(ui.End).AlignItems(ui.Center).Children(func() {
			if member.Role == api.RoleKadiv && !a.memberCanEdit(member) && member.UserID == a.user.ID {
				ui.Text(c, "Pemilik divisi").FontSize(12).TextColor(faint(t))
				return
			}
			a.memberActions(c, member)
		})
	})
}

func (a *App) memberCard(c *ui.Context, member api.Member, now time.Time) {
	t := c.Theme()
	m := &a.access.members
	active, late, loaded := a.memberWork(member.UserID, now)
	ui.Column(c).FillWidth().MinWidth(0).Padding(12).Gap(8).Children(func() {
		ui.Row(c).FillWidth().MinWidth(0).Gap(12).AlignItems(ui.Center).Children(func() {
			avatar(c, member.DisplayName, 40)
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).MinWidth(0).Children(func() {
					ui.Text(c, member.DisplayName).FontSize(14).FontWeight(500).MaxLines(1).MinWidth(0)
					if member.UserID == a.user.ID {
						ui.Text(c, "kamu").FontSize(12).TextColor(faint(t)).Shrink(0)
					}
					if member.WhatsAppLinked != nil && !*member.WhatsAppLinked {
						ui.Icon(c, spaceIcon("whatsapp")).Size(14, 14).TextColor(t.Warning).Tooltip("WhatsApp belum tertaut").Shrink(0)
					}
				})
				if member.NIM != "" {
					ui.Text(c, member.NIM).FontSize(12).TextColor(t.TextMuted).MaxLines(1).Selectable()
				}
				ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().MinWidth(0).Children(func() {
					memberRoleChip(c, member, m.roles)
					if len(member.Groups) > 0 {
						ui.Text(c, memberGroups(member)).FontSize(12).TextColor(t.TextMuted).MaxLines(1)
					}
					if loaded {
						memberLoadBar(c, active, late, 48)
						txt := ui.Text(c, memberLoadLabel(active, late)).FontSize(12).TextColor(t.TextMuted).MaxLines(1)
						if late > 0 {
							txt.TextColor(t.Danger)
						}
					}
				})
			})
			if canPing(a.selectedSpace()) && member.UserID != a.user.ID {
				ui.Box(c).Shrink(0).Children(func() {
					if spaceIconButton(c, "Ping "+member.DisplayName, "send").Size(44, 44).Clicked() {
						a.pingMember(member.UserID)
					}
				})
			}
		})
		if a.memberCanEdit(member) {
			ui.Row(c).FillWidth().Gap(4).Justify(ui.End).Children(func() {
				if memberAction(c, "Ubah akses", "Ubah akses "+member.DisplayName, m.editing == member.UserID).Height(36).Disabled(m.busy).Clicked() {
					a.openMemberAccess(member)
				}
			})
		}
	})
}

// memberGroups is a member's groups for display: "Desain, Video", or "—".
func memberGroups(member api.Member) string {
	if len(member.Groups) == 0 {
		return "—"
	}
	out := make([]string, len(member.Groups))
	for i, g := range member.Groups {
		if r := []rune(g); len(r) > 0 {
			out[i] = strings.ToUpper(string(r[0])) + string(r[1:])
		}
	}
	return strings.Join(out, ", ")
}
