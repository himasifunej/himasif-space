package appui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// sidebar is canvas A's sidebar: the brand, the period, PRIBADI, SPACE (a
// flat list of the user's Spaces), ORGANISASI and, at the bottom,
// Pengaturan, Tampilan, Ciutkan and the user. Counts show only what the app
// already holds; none is fetched for the sidebar.
func (a *App) sidebar(c *ui.Context) {
	t := c.Theme()
	// Tablets get the icon rail; the saved preference applies on desktop.
	collapsed := a.chrome.preferences.SidebarCollapsed || viewportMode(c) == layoutTablet
	width, padX := float32(248), float32(10)
	if collapsed {
		width, padX = 60, 8
	}
	ui.Column(c.Key("sidebar")).Transition(ui.ElementTransition{Duration: 180 * time.Millisecond, Size: true}).Width(width).Shrink(0).Padding(14, padX).Gap(2).Background(t.Surface).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
		ui.Row(c).Shrink(0).Gap(10).Padding(4, 6, 12, 6).AlignItems(ui.Center).Children(func() {
			ui.Image(c, spaceBrand()).Size(30, 30).Shrink(0).Fit(ui.Contain)
			if !collapsed {
				ui.Column(c).MinWidth(0).Gap(1).Children(func() {
					ui.Text(c, "Space").FontSize(15).FontWeight(600).MaxLines(1)
					ui.Text(c, "Ruang Kerja HIMASIF").FontSize(11).TextColor(faint(t)).MaxLines(1)
				})
			}
		})
		a.periodPicker(c, collapsed)
		ui.Scroll(c.Key("sidebar-navigation")).Grow(1).MinHeight(48).FillWidth().Label("Navigasi").Children(func() {
			ui.Column(c).FillWidth().Gap(2).Children(func() { a.sidebarSections(c, collapsed) })
		})
		ui.Column(c.Key("sidebar-foot")).FillWidth().Shrink(0).Gap(2).Padding(8, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
			a.sidebarFoot(c, collapsed)
		})
	})
}

func (a *App) sidebarSections(c *ui.Context, collapsed bool) {
	now := c.Now()
	path := a.router.Path()
	if !collapsed {
		sectionLabel(c, "Pribadi")
	}
	late, alert := a.jobdeskCount(now)
	if navRow(c, "Jobdesk saya", path == PathJobdesk, collapsed, iconLead(c, "jobdesk"), late, alert, nil).Clicked() {
		a.navigate(PathJobdesk)
	}
	if navRow(c, "Reminder pribadi", path == PathReminder, collapsed, iconLead(c, "reminder"), a.reminderCount(), false, nil).Clicked() {
		a.navigate(PathReminder)
	}

	if !collapsed {
		sectionLabel(c, "Space")
	}
	if len(a.access.spaces) == 0 {
		// Loading, unavailable or no access: the page explains which.
		if navRow(c, "Space saya", path == PathSpaces && a.selectedSpace() == nil, collapsed, iconLead(c, "grid"), "", false, nil).Clicked() {
			a.selectSpace(id.Nil)
			a.navigate(PathSpaces)
		}
	}
	for i, s := range a.access.spaces {
		s, i := s, i
		active := a.selectedSpaceID() == s.ID && (path == PathSpaces || path == PathMedhum)
		lead := func() { spaceTile(c, s, i) }
		if navRow(c.Key(s.ID), s.Name, active, collapsed, lead, a.spaceCount(s), false, nil).Clicked() {
			a.selectSpace(s.ID)
			a.chrome.tab = defaultSpaceTab(s)
			a.navigate(PathSpaces)
		}
	}

	if !collapsed {
		sectionLabel(c, "Organisasi")
	}
	if navRow(c, "Evaluasi SOP", path == PathEvaluasi, collapsed, iconLead(c, "sop"), a.evaluationCount(), false, nil).Clicked() {
		a.navigate(PathEvaluasi)
	}
	if navRow(c, "Arsip", path == PathArsip || path == PathArchiveAccess, collapsed, iconLead(c, "arsip"), "", false, nil).Clicked() {
		a.navigate(PathArsip)
	}
	if a.isAdmin() && navRow(c, "Organisasi", path == PathOrganisasi, collapsed, iconLead(c, "shield"), "", false, nil).Clicked() {
		a.navigate(PathOrganisasi)
	}
}

func (a *App) sidebarFoot(c *ui.Context, collapsed bool) {
	if navRow(c, "Pengaturan", a.router.Path() == PathPengaturan, collapsed, iconLead(c, "settings"), "", false, nil).Clicked() {
		a.chrome.settingsDetail = false
		a.navigate(PathPengaturan)
	}
	look := navRow(c, "Tampilan", a.chrome.appearanceQuick || a.chrome.appearance, collapsed, iconLead(c, "tampilan"), "", false, nil)
	if look.Clicked() {
		a.chrome.appearanceQuick = !a.chrome.appearanceQuick
		a.chrome.profileOpen, a.chrome.periodOpen = false, false
	}
	a.appearancePopover(c, look)
	label := "Ciutkan"
	if collapsed {
		label = "Buka sidebar"
	}
	toggle := navRow(c, label, false, collapsed, iconLead(c, "collapse"), "", false, func() { kbd(c, "Ctrl B") })
	if toggle.Clicked() {
		a.chrome.preferences.SidebarCollapsed = !collapsed
		a.savePreferences(c)
	}
	a.profile(c, collapsed)
}

// periodPicker is the canvas' period button under the brand.
func (a *App) periodPicker(c *ui.Context, collapsed bool) {
	t := c.Theme()
	label := "Periode"
	text := "Pilih periode"
	if p := a.period(); p != nil {
		label = periodLabel(*p)
		text = "Periode " + p.Label
	}
	b := ui.ButtonBase(c).Label(label).Tooltip(label).FillWidth().Shrink(0).Margin(0, 2, 6, 2).Row().AlignItems(ui.Center).Gap(8).Padding(7, 10).Radius(8).Border(1, t.Border).TextColor(t.TextMuted)
	if collapsed {
		b.Margin(0, 0, 6, 0).Padding(7, 0).Justify(ui.Center)
	}
	if b.Hovered() {
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.FocusRing(b.FocusVisible())
	b.Children(func() {
		ui.Icon(c, chromeIcon("period")).Size(16, 16).Shrink(0)
		if !collapsed {
			ui.Text(c, text).FontSize(12).Grow(1).MinWidth(0).MaxLines(1)
			ui.Icon(c, chromeIcon("down")).Size(16, 16).Shrink(0)
		}
	})
	if b.Clicked() {
		a.chrome.periodOpen = !a.chrome.periodOpen
	}
	popover(c, b, &a.chrome.periodOpen, 260, func() {
		ui.Text(c, "Pilih periode").Bold().Padding(6)
		if a.periodsLoading {
			muted(c, "Memuat periode…")
		}
		if a.periodsErr != "" {
			muted(c, a.periodsErr)
			if spaceButton(c, "Muat ulang", "refresh", false).Clicked() {
				a.loadPeriods()
			}
		}
		if len(a.periods) == 0 && !a.periodsLoading && a.periodsErr == "" {
			muted(c, "Belum ada periode.")
		}
		if len(a.periods) > 0 {
			ui.Scroll(c.Key("period-options")).Label("Daftar periode").FillWidth().Height(min(float32(len(a.periods))*42, 264)).Children(func() {
				ui.Column(c).FillWidth().Gap(4).Children(func() {
					for _, p := range a.periods {
						if spaceButton(c.Key(p.ID), periodLabel(p), "calendar", a.periodID == p.ID).FillWidth().Clicked() {
							a.changePeriod(p.ID)
							a.chrome.periodOpen = false
						}
					}
				})
			})
		}
	})
}

// profile is the user card at the bottom: the name and "Role · Divisi".
// It opens the account menu.
func (a *App) profile(c *ui.Context, collapsed bool) {
	t := c.Theme()
	b := ui.ButtonBase(c).Label("Menu profil").Tooltip(a.user.DisplayName).FillWidth().Shrink(0).Row().AlignItems(ui.Center).Padding(6, 8).Gap(10).Radius(8)
	if collapsed {
		b.Padding(6, 0).Justify(ui.Center)
	}
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.FocusRing(b.FocusVisible())
	b.Children(func() {
		ui.Box(c).Size(26, 26).Shrink(0).Radius(13).Center().Background(t.Accent).Children(func() {
			ui.Text(c, personInitials(a.user.DisplayName)).FontSize(11).FontWeight(600).TextColor(t.AccentText)
		})
		if !collapsed {
			ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
				ui.Text(c, a.user.DisplayName).FontSize(13).FontWeight(600).MaxLines(1)
				ui.Text(c, a.userRoleLine()).FontSize(11).TextColor(faint(t)).MaxLines(1)
			})
		}
	})
	if b.Clicked() {
		a.chrome.membersOpen = false
		a.chrome.profileOpen = !a.chrome.profileOpen
		a.chrome.notificationsOpen = false
		a.chrome.searchOpen = false
		a.chrome.appearanceQuick = false
	}
	ui.PopoverBase(c, b, &a.chrome.profileOpen, func(panel ui.Element) {
		panel.Width(270).Padding(12).Gap(6).Radius(12).Background(t.Surface).Border(1, t.Border).Shadow(0, 12, 32, 0, ui.RGBA(0, 0, 0, .35)).MarginStart(8)
		ui.Text(c, a.user.DisplayName).Bold()
		muted(c, a.user.Email)
		if a.isAdmin() && spaceButton(c, "Pengaturan organisasi", "shield", false).FillWidth().Clicked() {
			a.navigate(PathOrganisasi)
		}
		if spaceButton(c, "Keluar akun", "logout", false).FillWidth().Disabled(a.loggingOut).Clicked() {
			a.logout()
		}
	}).AttachTo(b, ui.AnchorBottomRight, ui.AnchorBottomLeft)
}

// userRoleLine is the user card's second line, as "Kadiv · Mediatek": the
// user's highest role in the selected division Space, or the first one.
// Without a known role it falls back to the email.
func (a *App) userRoleLine() string {
	if a.isAdmin() {
		return "Admin organisasi"
	}
	rank := map[string]int{api.RoleKadiv: 4, api.RoleCompanion: 3, api.Role45: 2, api.RoleMember: 1}
	names := map[string]string{api.RoleKadiv: "Kadiv", api.RoleCompanion: "Pendamping", api.Role45: "45", api.RoleMember: "Anggota"}
	line := func(s api.Space) string {
		best := ""
		for _, binding := range s.Roles {
			for _, r := range binding.Roles {
				if rank[r] > rank[best] {
					best = r
				}
			}
		}
		if best == "" {
			return ""
		}
		return names[best] + " · " + s.Name
	}
	if s := a.selectedSpace(); s != nil && s.Kind == api.SpaceDivision {
		if l := line(*s); l != "" {
			return l
		}
	}
	for _, s := range a.access.spaces {
		if s.Kind == api.SpaceDivision {
			if l := line(s); l != "" {
				return l
			}
		}
	}
	return a.user.Email
}

// jobdeskCount is "N lewat" once the user's own tasks are loaded.
func (a *App) jobdeskCount(now time.Time) (string, bool) {
	if !a.tasks.personal || !a.tasks.loaded {
		return "", false
	}
	late := 0
	for _, task := range a.tasks.items {
		if taskLate(task, now) {
			late++
		}
	}
	if late == 0 {
		return "", false
	}
	return fmt.Sprintf("%d lewat", late), true
}

// reminderCount counts the open reminders once they are loaded.
func (a *App) reminderCount() string {
	if !a.reminders.loaded {
		return ""
	}
	open := 0
	for _, r := range a.reminders.items {
		if !r.Completed {
			open++
		}
	}
	if open == 0 {
		return ""
	}
	return fmt.Sprint(open)
}

// spaceCount is "bersama" for a shared Space and, for the Space whose board
// is loaded, its active tasks.
func (a *App) spaceCount(s api.Space) string {
	if s.Kind == api.SpaceShared {
		return "bersama"
	}
	if a.tasks.personal || !a.tasks.loaded || a.tasks.scope != s.ID {
		return ""
	}
	active := 0
	for _, task := range a.tasks.items {
		if task.Progress != api.TaskDone && task.Progress != api.TaskCancelled {
			active++
		}
	}
	if active == 0 {
		return ""
	}
	return fmt.Sprint(active)
}

// evaluationCount is "N tinjau": the findings awaiting review, when the
// loaded page holds all of them.
func (a *App) evaluationCount() string {
	st := &a.evaluation
	if st.more || (st.state != "" && st.state != "needs_review") {
		return ""
	}
	n := 0
	for _, f := range st.items {
		if f.State == "needs_review" {
			n++
		}
	}
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d tinjau", n)
}
