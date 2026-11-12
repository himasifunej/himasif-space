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

// Space saya (canvas 26495b2dee24, option 2A): the user's Spaces of the
// period as a compact table, one row each: their role, their next meeting
// there, new activity (device notices not seen yet) and their own overdue
// tasks; a search; the period switcher; the Space last opened highlighted.
// The sidebar's logo comes here (3A).

// spacePickerRow is what a row shows of a Space.
type spacePickerRow struct {
	space         api.Space
	index         int
	role          string
	next          string
	unseen, late  int
	current, read bool
}

// spaceRoleLabel is the user's role in a Space: Kadiv, 45, Pendamping,
// Anggota; Bersama for a shared Space; with "hanya baca" when read-only.
func spaceRoleLabel(s api.Space) string {
	role := "Anggota"
	if s.Kind == api.SpaceShared {
		role = "Bersama"
	}
	for _, r := range s.Roles {
		switch {
		case slices.Contains(r.Roles, api.RoleKadiv):
			role = "Kadiv"
		case slices.Contains(r.Roles, api.Role45) && role != "Kadiv":
			role = "45"
		case slices.Contains(r.Roles, api.RoleCompanion) && role == "Anggota":
			role = "Pendamping"
		}
	}
	if s.ReadOnly {
		role += " · hanya baca"
	}
	return role
}

// spacePickerRows are the rows, filtered by query (name or role).
func (a *App) spacePickerRows(now time.Time, query string) []spacePickerRow {
	loc := a.homeZone()
	q := strings.ToLower(strings.TrimSpace(query))
	var rows []spacePickerRow
	for i, s := range a.access.spaces {
		r := spacePickerRow{space: s, index: i, role: spaceRoleLabel(s), current: s.ID == a.access.lastSpace, next: "—"}
		if q != "" && !strings.Contains(strings.ToLower(s.Name+" "+r.role), q) {
			continue
		}
		for _, m := range a.untuk.meetings {
			if m.SpaceID == s.ID && !m.Cancelled && m.StartsAt.After(now) {
				at := m.StartsAt.In(loc)
				r.next = untukWeekdays[at.Weekday()][:3] + " " + at.Format("15.04") + " · " + m.Title
				break
			}
		}
		for _, d := range a.deliveries.items {
			if d.SpaceID != nil && *d.SpaceID == s.ID && d.Channel == "device" && d.SeenAt == nil && (d.Status == "queued" || d.Status == "accepted") {
				r.unseen++
			}
		}
		if a.tasks.personal && a.tasks.loaded {
			for _, t := range a.tasks.items {
				if sp, _, ok := a.taskSpace(t); ok && sp.ID == s.ID && taskLate(t, now) {
					r.late++
				}
			}
		}
		rows = append(rows, r)
	}
	return rows
}

// pageMaxWidth is the widest a list page's content grows (owner's rc.2
// feedback: Space saya stopped at about 1080 and left the right of a wide
// window empty); pageAsideMin is the window width from which Space saya
// adds its "Minggu ini" column beside the table.
const (
	pageMaxWidth = 1440
	pageAsideMin = 1600
)

// spacePicker is the page.
func (a *App) spacePicker(c *ui.Context) {
	t := c.Theme()
	now := c.Now()
	a.ensureTasks(true)
	a.ensureMyMeetings(now)
	phone := isMobile(c)
	w, _ := c.Size()
	wide := !phone && w >= pageAsideMin
	ui.Scroll(c.Key("space-picker")).Fill().Children(func() {
		ui.Column(c).FillWidth().MaxWidth(pageMaxWidth).Padding(phoneSize(phone, 16, 26)).Gap(14).Children(func() {
			ui.Row(c).FillWidth().Wrap().Gap(12).AlignItems(ui.End).Children(func() {
				ui.Column(c).Grow(1).MinWidth(200).Gap(2).Children(func() {
					ui.Text(c, "Space saya").FontSize(20).Bold()
					sub := fmt.Sprintf("%d Space", len(a.access.spaces))
					if p := a.period(); p != nil {
						sub += " di periode " + p.Label
					}
					muted(c, sub).FontSize(13)
				})
				a.pickerPeriods(c)
			})
			list := func() {
				ui.TextInput(c.Key("picker-search"), &a.access.pickerQuery).Label("Cari Space").Placeholder("Cari Space… (Ctrl K mencari di mana saja)").FillWidth()
				if a.access.notice != "" {
					pageNotice(c, "lock", "Akses diperbarui", a.access.notice)
				}
				switch {
				case a.access.loading:
					pageUnavailable(c, "refresh", "Memuat Space…", "Memeriksa akses dari server.")
					return
				case a.access.err != "":
					// The Spaces known stay listed under the notice.
					pageNotice(c, "alert", "Space belum dapat dimuat", a.access.err)
					if spaceButton(c, "Muat ulang Space", "refresh", false).Clicked() {
						a.loadSpaces()
					}
					if len(a.access.spaces) == 0 {
						return
					}
				case len(a.access.spaces) == 0:
					pageEmpty(c, "lock", "Belum ada Space", "Gunakan undangan dari kadiv atau hubungi pengelola untuk mendapatkan akses.")
					return
				}
				rows := a.spacePickerRows(now, a.access.pickerQuery)
				if len(rows) == 0 {
					muted(c, "Tidak ada Space yang cocok.").FontSize(13)
					return
				}
				ui.Column(c.Key("picker-table")).FillWidth().Radius(12).Border(1, t.Border).Background(t.Surface).Clip().Children(func() {
					if !phone {
						a.pickerHeader(c)
					}
					for _, r := range rows {
						a.pickerRow(c, r, phone)
					}
				})
			}
			if !wide {
				list()
				return
			}
			ui.Row(c.Key("picker-columns")).FillWidth().Gap(20).AlignItems(ui.Start).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(14).Children(list)
				a.pickerWeek(c, now)
			})
		})
	})
}

// pickerWeek is Space saya's second column on wide windows: the week
// across the Spaces, the meetings of the next 7 days and the sums of the
// table, each meeting opening its Space.
func (a *App) pickerWeek(c *ui.Context, now time.Time) {
	t := c.Theme()
	loc := a.homeZone()
	rows := a.spacePickerRows(now, "")
	unseen, late := 0, 0
	for _, r := range rows {
		unseen += r.unseen
		late += r.late
	}
	var week []api.Meeting
	for _, m := range a.untuk.meetings {
		if !m.Cancelled && m.StartsAt.After(now) && m.StartsAt.Before(now.Add(7*24*time.Hour)) {
			week = append(week, m)
		}
	}
	slices.SortFunc(week, func(x, y api.Meeting) int { return x.StartsAt.Compare(y.StartsAt) })
	ui.Column(c.Key("picker-week")).Width(320).Shrink(0).Padding(16).Gap(14).Radius(12).Border(1, t.Border).Background(t.Surface).Label("Minggu ini").Children(func() {
		ui.Text(c, "Minggu ini").FontSize(15).FontWeight(600)
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			stat := func(n int, label string, bad bool) {
				ui.Column(c.Key(label)).Grow(1).Basis(0).MinWidth(0).Padding(10).Gap(2).Radius(9).Background(t.Background).Children(func() {
					tx := ui.Text(c, fmt.Sprint(n)).FontSize(20).FontWeight(600).FontFeatures("tnum")
					if bad && n > 0 {
						tx.TextColor(t.Danger)
					}
					ui.Text(c, label).FontSize(11).TextColor(t.TextMuted).MaxLines(1)
				})
			}
			stat(len(week), "Rapat 7 hari", false)
			stat(unseen, "Kabar baru", false)
			stat(late, "Tugas lewat", true)
		})
		ui.Text(c, "RAPAT BERIKUTNYA").Font(FontMono).FontSize(10).TextColor(faint(t))
		if len(week) == 0 {
			muted(c, "Tidak ada rapat dalam 7 hari ke depan.").FontSize(13)
			return
		}
		for i, m := range week[:min(len(week), 6)] {
			idx := slices.IndexFunc(a.access.spaces, func(s api.Space) bool { return s.ID == m.SpaceID })
			if idx < 0 {
				continue
			}
			s := a.access.spaces[idx]
			at := m.StartsAt.In(loc)
			b := ui.ButtonBase(c.Key(i)).Label("Buka "+s.Name+": "+m.Title).FillWidth().Row().Gap(10).AlignItems(ui.Center).Padding(6, 8).Radius(8)
			if b.Hovered() {
				b.Background(t.SurfaceHover)
			}
			b.FocusRing(b.FocusVisible()).Children(func() {
				ui.Box(c).Size(24, 24).Shrink(0).Radius(7).Center().Background(ui.Hex(canvasTileColors[idx%len(canvasTileColors)])).Children(func() {
					ui.Text(c, spaceInitials(s)).FontSize(10).FontWeight(600).TextColor(canvasInk)
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
					ui.Text(c, m.Title).FontSize(13).MaxLines(1)
					ui.Text(c, untukWeekdays[at.Weekday()][:3]+" "+at.Format("15.04")+" · "+s.Name).FontSize(11).TextColor(faint(t)).MaxLines(1)
				})
			})
			if b.Clicked() {
				a.openSpaceFromPicker(s)
			}
		}
	})
}

// pickerPeriods switches the period: the four newest, most recent first.
func (a *App) pickerPeriods(c *ui.Context) {
	if len(a.periods) < 2 {
		return
	}
	ps := a.periods[:min(len(a.periods), 4)]
	labels := make([]string, len(ps))
	sel := -1
	for i, p := range ps {
		labels[i] = periodLabel(p)
		if p.ID == a.periodID {
			sel = i
		}
	}
	if i := segmented(c, "Periode", labels, sel); i >= 0 && i != sel {
		a.changePeriod(ps[i].ID)
	}
}

var pickerColumns = []struct {
	title string
	grow  float32
}{{"Space", 2.2}, {"Peranmu", 1}, {"Rapat berikutnya", 1.7}, {"Kabar baru", .8}, {"Tugasmu lewat", .9}, {"", .6}}

func (a *App) pickerHeader(c *ui.Context) {
	t := c.Theme()
	ui.Row(c.Key("picker-head")).FillWidth().Height(36).Padding(0, 16).Gap(12).AlignItems(ui.Center).Background(spaceMix(t.Border, t.Surface, .5)).Children(func() {
		for _, col := range pickerColumns {
			ui.Text(c, strings.ToUpper(col.title)).Font(FontMono).FontSize(10).TextColor(faint(t)).Grow(col.grow).Basis(0).MinWidth(0).MaxLines(1)
		}
	})
}

// pickerRow is one Space: a click opens it.
func (a *App) pickerRow(c *ui.Context, r spacePickerRow, phone bool) {
	t := c.Theme()
	s := r.space
	b := ui.ButtonBase(c.Key(s.ID)).Label("Buka "+s.Name).FillWidth().Row().Gap(12).AlignItems(ui.Center).Padding(10, 16).MinHeight(56).BorderWidth(1, 0, 0, 0).BorderColor(t.Border)
	switch {
	case r.current:
		b.Background(accentSoft(t))
	case b.Hovered():
		b.Background(t.SurfaceHover)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		name := func(grow float32) {
			ui.Row(c).Grow(grow).Basis(0).MinWidth(0).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(32, 32).Shrink(0).Radius(9).Center().Background(ui.Hex(canvasTileColors[r.index%len(canvasTileColors)])).Children(func() {
					ui.Text(c, spaceInitials(s)).FontSize(12).FontWeight(600).TextColor(canvasInk)
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, s.Name).FontSize(14).FontWeight(600).MaxLines(1)
					kind := "Ruang divisi"
					if s.Kind == api.SpaceShared {
						kind = "Kolaborasi lintas divisi"
					}
					if phone {
						kind = r.role + " · " + r.next
					}
					ui.Text(c, kind).FontSize(12).TextColor(faint(t)).MaxLines(1)
				})
			})
		}
		count := func(n int, grow float32, bad bool) {
			ui.Row(c).Grow(grow).Basis(0).MinWidth(0).Gap(6).AlignItems(ui.Center).Children(func() {
				if n > 0 && !bad {
					ui.Box(c).Size(7, 7).Radius(4).Background(t.Accent)
				}
				tx := ui.Text(c, fmt.Sprint(n)).FontSize(13).FontFeatures("tnum").TextColor(faint(t))
				if n > 0 {
					tx.TextColor(t.Text)
					if bad {
						tx.TextColor(t.Danger)
					}
				}
			})
		}
		if phone {
			name(1)
			if r.late > 0 {
				ui.Text(c, fmt.Sprintf("%d lewat", r.late)).FontSize(12).TextColor(t.Danger).Shrink(0)
			}
			return
		}
		name(pickerColumns[0].grow)
		ui.Row(c).Grow(pickerColumns[1].grow).Basis(0).MinWidth(0).Children(func() {
			chip := ui.Row(c).Padding(2, 7).Radius(6).Border(1, t.Border)
			if strings.HasPrefix(r.role, "Kadiv") {
				chip.BorderColor(t.Accent)
			}
			chip.Children(func() {
				tx := ui.Text(c, r.role).FontSize(11).TextColor(t.TextMuted).MaxLines(1)
				if strings.HasPrefix(r.role, "Kadiv") {
					tx.TextColor(t.Accent)
				}
			})
		})
		ui.Text(c, r.next).FontSize(13).TextColor(t.TextMuted).Grow(pickerColumns[2].grow).Basis(0).MinWidth(0).MaxLines(1)
		count(r.unseen, pickerColumns[3].grow, false)
		count(r.late, pickerColumns[4].grow, true)
		ui.Row(c).Grow(pickerColumns[5].grow).Basis(0).Justify(ui.End).Children(func() {
			ui.Text(c, "Buka").FontSize(13).FontWeight(500)
		})
	})
	if b.Clicked() {
		a.openSpaceFromPicker(s)
	}
}

// openSpaceFromPicker opens a Space on its first tab.
func (a *App) openSpaceFromPicker(s api.Space) {
	a.selectSpace(s.ID)
	a.chrome.tab = defaultSpaceTab(s)
	a.navigate(PathSpaces)
}

// openSpacePicker is the logo's action: Space saya, the last Space
// highlighted.
func (a *App) openSpacePicker() {
	a.selectSpace(id.Nil)
	a.navigate(PathSpaces)
}
