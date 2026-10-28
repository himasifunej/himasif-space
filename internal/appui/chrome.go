package appui

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/workspaceui"
)

type chromeState struct {
	preferences                                                        Preferences
	tab, settingsTab                                                   string
	divisionID                                                         id.ID
	appearance, periodOpen, profileOpen, notificationsOpen, searchOpen bool
	search, customAccent, preferencesErr                               string
}

func (a *App) resetWorkspace() {
	p, err := a.chrome.preferences, a.chrome.preferencesErr
	a.chrome = chromeState{preferences: p, preferencesErr: err, tab: "Beranda", settingsTab: "WhatsApp"}
	a.workspace = workspaceui.New()
	a.resetAccess()
	a.tasksContextChanged()
	a.resetReminders()
	a.resetOrganization()
}

func (a *App) loadPreferences() {
	if a.preferencesLoaded {
		return
	}
	a.preferencesLoaded = true
	a.chrome.preferences = DefaultPreferences()
	if a.PreferencesStore != nil {
		p, err := a.PreferencesStore.Load()
		a.chrome.preferences = NormalizePreferences(p)
		if err != nil {
			a.chrome.preferencesErr = "Preferensi perangkat tidak dapat dibaca. Tampilan menggunakan pengaturan default."
		}
	}
}

func (a *App) savePreferences(c *ui.Context) {
	a.chrome.preferences = NormalizePreferences(a.chrome.preferences)
	a.chrome.preferencesErr = ""
	if a.PreferencesStore != nil {
		if err := a.PreferencesStore.Save(a.chrome.preferences); err != nil {
			a.chrome.preferencesErr = "Tampilan berubah untuk sesi ini, tetapi belum tersimpan di perangkat."
			c.AddToast(ui.Toast{Title: "Preferensi belum tersimpan", Description: a.chrome.preferencesErr, Type: "error"})
		}
	}
	c.After(0)
}

func (a *App) selectedDivision() *api.Division {
	if s := a.selectedSpace(); s != nil && s.Kind == api.SpaceDivision && len(s.Divisions) == 1 {
		return &s.Divisions[0]
	}
	return nil
}
func (a *App) spaceTitle() string {
	if s := a.selectedSpace(); s != nil {
		return s.Name
	}
	return "Space saya"
}

func spaceButton(c *ui.Context, label, icon string, active bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Row().AlignItems(ui.Center).Justify(ui.Start).Gap(10).Padding(9, 12).MinHeight(36).Radius(8).TextColor(t.TextMuted)
	if active {
		b.Background(t.SurfaceHover).Border(1, t.Border).TextColor(t.Text)
	} else if b.Hovered() {
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.FocusRing(b.FocusVisible())
	b.Children(func() {
		if icon != "" {
			ui.Icon(c, spaceIcon(icon)).Size(18, 18).Shrink(0)
		}
		ui.Text(c, label).MaxLines(1)
	})
	return b
}
func spaceIconButton(c *ui.Context, label, icon string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Size(34, 34).Center().Radius(8).TextColor(t.TextMuted)
	if b.Hovered() {
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.FocusRing(b.FocusVisible())
	b.Children(func() { ui.Icon(c, spaceIcon(icon)).Size(18, 18) })
	return b
}
func popover(c *ui.Context, anchor ui.Element, open *bool, width float32, fn func()) {
	ui.PopoverBase(c, anchor, open, func(panel ui.Element) {
		t := c.Theme()
		panel.Width(width).MaxHeight(420).Padding(12).Gap(6).Radius(12).Background(t.Surface).Border(1, t.Border).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, .25))
		fn()
	}).AttachTo(anchor, ui.AnchorBottomRight, ui.AnchorTopRight)
}
func (a *App) navigate(path string) {
	a.router.Push(path)
	a.chrome.searchOpen, a.chrome.profileOpen, a.chrome.notificationsOpen = false, false, false
}

func (a *App) shell(c *ui.Context) {
	a.tickNotes(c.Now())
	if a.notes.editor != nil && (a.notes.editing || a.notes.dirty) {
		c.After(500 * time.Millisecond)
	}
	if a.lastPing.IsZero() {
		a.lastPing = c.Now()
	}
	if next := a.lastPing.Add(pingEvery); !c.Now().Before(next) {
		a.ping(c.Now())
	} else {
		c.After(next.Sub(c.Now()))
	}
	t := c.Theme()
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Grow(1).MinHeight(0).FillWidth().AlignItems(ui.Stretch).Children(func() {
			a.sidebar(c)
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				a.header(c)
				ui.Row(c).Grow(1).MinHeight(0).FillWidth().AlignItems(ui.Stretch).Children(func() {
					a.router.View(c, func(r *ui.Route) { a.page(c, r) }).Grow(1).MinWidth(0).MinHeight(0)
					// Keep one inspector at a time so the board remains usable
					// at the minimum window width.
					if a.workspace != nil && a.workspace.SelectedID() != id.Nil {
						a.chrome.appearance = false
					}
					if a.chrome.appearance {
						a.appearance(c)
					}
				})
			})
		})
		a.statusbar(c)
	})
	ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		viewport.Gap(8).Padding(16)
		for _, toast := range toasts {
			parts := ui.ToastBase(c, toast)
			parts.Root.Width(330).Row().AlignItems(ui.Center).Padding(14).Gap(10).Radius(10).Background(t.Surface).Border(1, t.Border).Shadow(0, 4, 16, 0, ui.RGBA(0, 0, 0, .2)).Children(func() {
				ui.Icon(c, spaceIcon("info")).Size(18, 18).TextColor(t.Accent)
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Text(c, toast.Title).Bold()
					if toast.Description != "" {
						muted(c, toast.Description)
					}
				})
				parts.CloseButton().Label("Tutup pesan").Size(26, 26).Center().Children(func() { ui.Icon(c, spaceIcon("close")).Size(14, 14) })
			})
		}
	})
}

func (a *App) sidebar(c *ui.Context) {
	t := c.Theme()
	_, h := c.Size()
	compact := h < 900 || a.chrome.preferences.Compact
	collapsed := a.chrome.preferences.SidebarCollapsed
	width := float32(244)
	if collapsed {
		width = 66
	}
	gap := float32(4)
	if compact {
		gap = 2
	}
	ui.Column(c.Key("sidebar")).Width(width).Shrink(0).Padding(12).Gap(gap).Background(t.Surface).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
		brandHeight := float32(62)
		if compact {
			brandHeight = 48
		}
		ui.Row(c).Height(brandHeight).Shrink(0).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Image(c, spaceBrand()).Size(34, 34).Shrink(0).Fit(ui.Contain)
			if !collapsed {
				ui.Column(c).Gap(2).Children(func() {
					ui.Text(c, "space.").FontSize(29).Bold()
					ui.Text(c, "RUANG KERJA HIMASIF").FontSize(8).TextColor(t.TextMuted)
				})
			}
		})
		a.periodPicker(c, collapsed)
		if !collapsed {
			muted(c, "PRIBADI").FontSize(10).Padding(10, 8, 2, 8)
		}
		a.navButton(c, PathJobdesk, "Jobdesk saya", "task", collapsed)
		a.navButton(c, PathReminder, "Reminder pribadi", "bell", collapsed)
		if !collapsed {
			muted(c, "SPACE").FontSize(10).Padding(10, 8, 2, 8)
		}
		a.navButton(c, PathSpaces, "Space saya", "grid", collapsed)
		limit := int((h - 780) / 40)
		if compact {
			limit = int((h - 600) / 40)
		}
		limit = max(0, min(limit, 5))
		for i, s := range a.access.spaces {
			if i >= limit {
				break
			}
			icon := "grid"
			if s.Kind == api.SpaceShared {
				icon = "link"
			}
			active := a.selectedSpaceID() == s.ID && a.router.Path() == PathSpaces
			if a.sidebarSpaceButton(c.Key(s.ID), s, icon, active, collapsed).Clicked() {
				a.selectSpace(s.ID)
				a.navigate(PathSpaces)
			}
		}

		if !compact && a.selectedSpace() != nil && a.router.Path() == PathSpaces {
			for _, tab := range []struct{ label, icon string }{{"Beranda", "home"}, {"Board tugas", "board"}, {"Agenda", "calendar"}, {"Notes", "note"}} {
				if a.sideButton(c, tab.label, tab.icon, a.chrome.tab == tab.label, collapsed).Clicked() {
					a.chrome.tab = tab.label
				}
			}
		}

		ui.Box(c).Grow(1).MinHeight(0)
		a.navButton(c, PathArsip, "Arsip periode", "archive", collapsed)
		if a.sideButton(c, "Penampilan", "palette", a.chrome.appearance, collapsed).Clicked() {
			a.chrome.appearance = !a.chrome.appearance
			if a.chrome.appearance && a.workspace != nil {
				a.workspace.Close()
			}
		}
		label := "Ciutkan sidebar"
		if collapsed {
			label = "Buka sidebar"
		}
		if a.sideButton(c, label, "panel", false, collapsed).Clicked() {
			a.chrome.preferences.SidebarCollapsed = !collapsed
			a.savePreferences(c)
		}
		a.profile(c, collapsed)
	})
}
func (a *App) sidebarSpaceButton(c *ui.Context, s api.Space, icon string, active, collapsed bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(s.Name).Tooltip(s.Name).FillWidth().Row().AlignItems(ui.Center).Padding(8, 10).Gap(9).MinHeight(36).Radius(8).TextColor(t.TextMuted)
	if active {
		b.Background(t.SurfaceHover).Border(1, t.Border).TextColor(t.Text)
	} else if b.Hovered() {
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.FocusRing(b.FocusVisible())
	b.Children(func() {
		if s.Kind == api.SpaceDivision && len(s.Divisions) == 1 && divisionLogo(s.Divisions[0].Slug) != nil {
			ui.Image(c, divisionLogo(s.Divisions[0].Slug)).Size(22, 22).Shrink(0).Fit(ui.Contain)
		} else {
			ui.Icon(c, spaceIcon(icon)).Size(20, 20).Shrink(0)
		}
		if !collapsed {
			ui.Text(c, s.Name).MinWidth(0).MaxLines(1)
		}
	})
	return b
}
func (a *App) sideButton(c *ui.Context, label, icon string, active, collapsed bool) ui.Element {
	if collapsed {
		b := spaceIconButton(c, label, icon).FillWidth()
		if active {
			b.Background(c.Theme().SurfaceHover).TextColor(c.Theme().Accent)
		}
		return b
	}
	return spaceButton(c, label, icon, active).FillWidth()
}
func (a *App) navButton(c *ui.Context, path, label, icon string, collapsed bool) {
	if a.sideButton(c, label, icon, a.router.Path() == path, collapsed).Clicked() {
		if path == PathSpaces {
			a.selectSpace(id.Nil)
		}
		a.navigate(path)
	}
}
func (a *App) periodPicker(c *ui.Context, collapsed bool) {
	t := c.Theme()
	label := "Periode"
	text := "Pilih periode"
	if p := a.period(); p != nil {
		label = periodLabel(*p)
		text = p.Label
	}
	var anchor ui.Element
	if collapsed {
		anchor = spaceIconButton(c, label, "calendar").FillWidth()
	} else {
		anchor = ui.ButtonBase(c).Label(label).FillWidth().Row().AlignItems(ui.Center).Padding(11).Gap(10).Radius(10).Border(1, t.Border)
		anchor.FocusRing(anchor.FocusVisible())
		anchor.Children(func() {
			ui.Icon(c, spaceIcon("archive")).Size(18, 18)
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, text).MaxLines(1)
				if p := a.period(); p != nil {
					muted(c, map[string]string{api.PeriodActive: "Periode aktif", api.PeriodClosed: "Periode ditutup", api.PeriodPreparing: "Persiapan periode"}[p.Status])
				} else {
					muted(c, "Periode organisasi")
				}
			})
			ui.Icon(c, spaceIcon("down")).Size(14, 14)
		})
	}
	if anchor.Clicked() {
		a.chrome.periodOpen = !a.chrome.periodOpen
	}
	popover(c, anchor, &a.chrome.periodOpen, 260, func() {
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
func (a *App) profile(c *ui.Context, collapsed bool) {
	t := c.Theme()
	b := ui.ButtonBase(c).Label("Menu profil").FillWidth().Row().AlignItems(ui.Center).Padding(8, 4).Gap(8).Radius(8)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.FocusRing(b.FocusVisible())
	b.Children(func() {
		initials := "S"
		fields := strings.Fields(a.user.DisplayName)
		if len(fields) > 0 {
			initials = strings.ToUpper(string([]rune(fields[0])[0]))
		}
		ui.Box(c).Size(30, 30).Radius(10).Background(t.SurfaceHover).Center().Children(func() { ui.Text(c, initials).Bold().TextColor(t.Text) })
		if !collapsed {
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, a.user.DisplayName).FontSize(12).Bold().MaxLines(1)
				ui.Text(c, a.user.Email).FontSize(10).TextColor(t.TextMuted).MaxLines(1)
			})
			ui.Icon(c, spaceIcon("more")).Size(18, 18)
		}
	})
	if b.Clicked() {
		a.chrome.profileOpen = !a.chrome.profileOpen
		a.chrome.notificationsOpen = false
		a.chrome.searchOpen = false
	}
	popover(c, b, &a.chrome.profileOpen, 270, func() {
		ui.Text(c, a.user.DisplayName).Bold()
		muted(c, a.user.Email)
		if spaceButton(c, "Pengaturan", "settings", false).FillWidth().Clicked() {
			a.navigate(PathPengaturan)
		}
		if a.isAdmin() && spaceButton(c, "Pengaturan organisasi", "shield", false).FillWidth().Clicked() {
			a.navigate(PathOrganisasi)
		}
		if spaceButton(c, "Evaluasi SOP", "shield", false).FillWidth().Clicked() {
			a.navigate(PathEvaluasi)
		}
		if spaceButton(c, "Keluar akun", "logout", false).FillWidth().Disabled(a.loggingOut).Clicked() {
			a.logout()
		}
	})
}
func (a *App) header(c *ui.Context) {
	t := c.Theme()
	w, _ := c.Size()
	title := map[string]string{PathJobdesk: "Jobdesk saya", PathReminder: "Reminder pribadi", PathArsip: "Arsip periode", PathArchiveAccess: "Izin arsip", PathPengaturan: "Pengaturan", PathOrganisasi: "Organisasi", PathEvaluasi: "Evaluasi SOP"}[a.router.Path()]
	if title == "" {
		title = a.spaceTitle()
	}
	ui.Row(c).FillWidth().Height(84).Shrink(0).Padding(16, 24).Gap(16).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Column(c).Grow(1).MinWidth(90).Gap(5).Children(func() {
			ui.Text(c, title).FontSize(24).Bold().MaxLines(1)
			s := "HIMASIF"
			if p := a.period(); p != nil {
				s += " · " + p.Label
			}
			muted(c, s).FontSize(10)
		})
		searchWidth := float32(360)
		if w < 1150 {
			searchWidth = 240
		}
		ui.Column(c).Width(searchWidth).Shrink(0).Children(func() {
			var input ui.Element
			shortcut := c.Shortcut(ui.Ctrl, ui.KeyK) || c.Shortcut(ui.Cmd, ui.KeyK)
			ui.Row(c).FillWidth().Height(40).Padding(0, 10).Gap(8).AlignItems(ui.Center).Radius(9).Border(1, t.Border).Children(func() {
				ui.Icon(c, spaceIcon("search")).Size(16, 16).TextColor(t.TextMuted)
				input = ui.TextInput(c.Key("global-search"), &a.chrome.search).Label("Cari di Space").Placeholder("Cari tugas, notes, atau anggota…").Grow(1).MinWidth(0).Background(ui.RGBA(0, 0, 0, 0)).BorderWidth(0)
				if shortcut {
					input.Focus()
				}
				if input.Clicked() || input.Changed() || shortcut {
					a.chrome.searchOpen = true
					a.chrome.notificationsOpen = false
					a.chrome.profileOpen = false
				}
				ui.Text(c, "Ctrl K").FontSize(9).TextColor(t.TextMuted)
			})
			popover(c, input, &a.chrome.searchOpen, searchWidth, func() { a.searchResults(c) })
		})
		b := spaceIconButton(c, "Notifikasi", "bell")
		if b.Clicked() {
			a.chrome.notificationsOpen = !a.chrome.notificationsOpen
			a.chrome.searchOpen = false
			a.chrome.profileOpen = false
		}
		popover(c, b, &a.chrome.notificationsOpen, 310, func() {
			ui.Text(c, "Notifikasi").FontSize(17).Bold()
			muted(c, "Pembaruan yang memerlukan perhatianmu.")
			a.taskNotifications(c)
		})
		a.taskHeaderAction(c)
	})
}
func (a *App) searchResults(c *ui.Context) {
	ui.Text(c, "Navigasi cepat").Bold().Padding(6)
	q := strings.ToLower(strings.TrimSpace(a.chrome.search))
	matches := 0
	items := []struct{ label, path, icon string }{{"Jobdesk saya", PathJobdesk, "task"}, {"Reminder pribadi", PathReminder, "bell"}, {"Space saya", PathSpaces, "grid"}, {"Arsip periode", PathArsip, "archive"}, {"Pengaturan", PathPengaturan, "settings"}}
	for _, item := range items {
		if q == "" || strings.Contains(strings.ToLower(item.label), q) {
			matches++
			if spaceButton(c, item.label, item.icon, false).FillWidth().Clicked() {
				if item.path == PathSpaces {
					a.selectSpace(id.Nil)
				}
				a.navigate(item.path)
			}
		}
	}
	for _, s := range a.access.spaces {
		if q == "" || strings.Contains(strings.ToLower(s.Name), q) {
			matches++
			if spaceButton(c.Key(s.ID), s.Name, "grid", false).FillWidth().Clicked() {
				a.selectSpace(s.ID)
				a.navigate(PathSpaces)
			}
		}
	}
	matches += a.taskSearchResults(c, q)
	if matches == 0 {
		muted(c, "Tidak ada halaman yang cocok.")
	}
	muted(c, "Tugas pada tampilan aktif; pencarian notes dan anggota belum tersedia.").Padding(6)
}
func (a *App) statusbar(c *ui.Context) {
	t := c.Theme()
	color, text := t.Success, "Tersambung"
	switch a.conn {
	case connOffline:
		color, text = t.Danger, "Tidak ada koneksi"
	case connConnecting:
		color, text = t.Warning, "Menyambungkan…"
	}
	ui.Row(c).FillWidth().Height(29).Shrink(0).Padding(0, 14).Gap(8).AlignItems(ui.Center).Background(t.Surface).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(color)
		ui.Text(c, text).FontSize(10).TextColor(t.TextMuted)
		if a.conn == connOffline && spaceButton(c, "Coba lagi", "refresh", false).Padding(2, 6).MinHeight(22).Clicked() {
			a.ping(c.Now())
		}
		ui.Box(c).Grow(1)
		ui.Text(c, "Ctrl K · Cari    Esc · Tutup menu").FontSize(10).TextColor(t.TextMuted)
		ui.Text(c, "Space · "+a.Version).FontSize(10).TextColor(t.TextMuted)
	})
}
func (a *App) appearance(c *ui.Context) {
	t := c.Theme()
	p := &a.chrome.preferences
	ui.Column(c).Width(288).Shrink(0).FillHeight().Background(t.Surface).BorderWidth(0, 0, 0, 1).BorderColor(t.Border).Children(func() {
		ui.Row(c).FillWidth().Padding(18).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Penampilan").FontSize(18).Bold().Grow(1)
			if spaceIconButton(c, "Tutup penampilan", "close").Clicked() {
				a.chrome.appearance = false
			}
		})
		ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(18).Gap(18).Children(func() {
				muted(c, "Sesuaikan ruang kerjamu. Preferensi disimpan di perangkat ini.")
				ui.Text(c, "Tema").Bold()
				for _, mode := range []struct{ key, label, icon string }{{"dark", "Gelap", "moon"}, {"light", "Terang", "sun"}, {"system", "Ikuti sistem", "monitor"}} {
					if spaceButton(c, mode.label, mode.icon, p.Theme == mode.key).FillWidth().Clicked() {
						p.Theme = mode.key
						a.savePreferences(c)
					}
				}
				ui.Text(c, "Warna aksen").Bold()
				ui.Row(c).Gap(8).Children(func() {
					for _, accent := range []string{"sand", "sage", "blue", "lavender", "rose"} {
						b := ui.ButtonBase(c.Key(accent)).Label("Aksen "+accent).Tooltip(accent).Size(33, 33).Center().Radius(11).Background(spaceAccent(accent))
						b.FocusRing(b.FocusVisible())
						if p.Accent == accent {
							b.Children(func() { ui.Icon(c, spaceIcon("check")).Size(16, 16).TextColor(ui.Hex("#24251f")) })
						}
						if b.Clicked() {
							p.Accent = accent
							a.savePreferences(c)
						}
					}
				})
				ui.TextInput(c, &a.chrome.customAccent).Label("Warna aksen khusus").Placeholder("#RRGGBB").FillWidth()
				if spaceButton(c, "Terapkan warna", "palette", false).FillWidth().Clicked() {
					value := strings.ToLower(strings.TrimSpace(a.chrome.customAccent))
					normalized := NormalizePreferences(Preferences{Accent: value})
					if len(value) == 7 && normalized.Accent == value {
						p.Accent = value
						a.savePreferences(c)
					} else {
						c.AddToast(ui.Toast{Title: "Format warna belum sesuai", Description: "Gunakan kode enam digit, misalnya #a8c2ac.", Type: "error"})
					}
				}
				ui.Text(c, "Tata letak").Bold()
				if spaceButton(c, "Tampilan ringkas", "grid", p.Compact).FillWidth().Clicked() {
					p.Compact = !p.Compact
					a.savePreferences(c)
				}
				if a.chrome.preferencesErr != "" {
					ui.Text(c, a.chrome.preferencesErr).FontSize(12).TextColor(t.Warning)
				}
			})
		})
	})
}
