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
	datePickers                                                        map[string]*datePickerState
	whens                                                              map[string]*whenState
	drawerWidth                                                        float32
	membersOpen                                                        bool
	tab, settingsTab                                                   string
	divisionID                                                         id.ID
	appearance, periodOpen, profileOpen, notificationsOpen, searchOpen bool
	search, customAccent, preferencesErr                               string
	members                                                            headerMembersState
	syncOpen, pingOpen                                                 bool
	// appearanceQuick is the small appearance popover of the sidebar's
	// Tampilan; appearance is the full panel.
	appearanceQuick bool
	// settingsDetail shows one settings page instead of the grouped list
	// on narrow windows.
	settingsDetail bool
}

func (a *App) resetWorkspace() {
	a.stopRealtime()
	a.operations = operationsUIState{seq: a.operations.seq + 1}
	a.emergency = emergencyUIState{seq: a.emergency.seq + 1}
	a.evaluation = sopUIState{seq: a.evaluation.seq + 1}
	a.waContent = waContentUIState{seq: a.waContent.seq + 1}
	a.whatsapp = whatsappUIState{seq: a.whatsapp.seq + 1}
	a.connectors = connectorUIState{seq: a.connectors.seq + 1}
	a.medhum = medhumState{seq: a.medhum.seq + 1}
	p, err := a.chrome.preferences, a.chrome.preferencesErr
	a.chrome = chromeState{preferences: p, preferencesErr: err, tab: "Beranda", settingsTab: "WhatsApp"}
	a.workspace = workspaceui.New()
	a.resetAccess()
	a.tasksContextChanged()
	a.resetReminders()
	a.meetings = meetingsState{}
	a.deliveries = deliveriesState{}
	a.offlineSync = offlineUIState{}
	a.notificationSettingsState = notificationSettingsState{}
	a.taskNotification = taskNotificationState{}
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
		panel.Margin(6, 0, 0, 0).Width(width).MaxHeight(420).Padding(12).Gap(6).Radius(12).Background(t.Surface).Border(1, t.Border).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, .25))
		fn()
	}).AttachTo(anchor, ui.AnchorBottomRight, ui.AnchorTopRight)
}
func (a *App) navigate(path string) {
	a.router.Push(path)
	a.chrome.searchOpen, a.chrome.profileOpen, a.chrome.notificationsOpen = false, false, false
}

func (a *App) shell(c *ui.Context) {
	if c.Shortcut(ui.Ctrl, ui.KeyB) || c.Shortcut(ui.Cmd, ui.KeyB) {
		a.chrome.preferences.SidebarCollapsed = !a.chrome.preferences.SidebarCollapsed
		a.savePreferences(c)
	}
	// Escape closes the appearance panel, as it closes the popovers. A
	// popover open this frame takes the key itself, so one press closes
	// one thing.
	popoverOpen := a.chrome.searchOpen || a.chrome.notificationsOpen || a.chrome.profileOpen || a.chrome.membersOpen || a.chrome.periodOpen || a.chrome.appearanceQuick || a.chrome.syncOpen
	if a.chrome.appearance && !popoverOpen && c.Shortcut(0, ui.KeyEscape) {
		a.chrome.appearance = false
	}

	a.tickOffline(c.Now())
	a.tickRealtime(c)
	a.tickNotes(c.Now())
	a.tickDeliveries(c.Now())
	c.After(15 * time.Second)
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
	if isMobile(c) {
		a.mobileShell(c)
		a.pingDialog(c)
		a.toasts(c)
		return
	}
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Grow(1).MinHeight(0).FillWidth().AlignItems(ui.Stretch).Children(func() {
			a.sidebar(c)
			ui.Column(c.Key("main-area")).Transition(ui.ElementTransition{Duration: 180 * time.Millisecond, Size: true, Position: true}).Grow(1).MinWidth(0).Children(func() {
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
			if a.taskBoardVisible() && !a.chrome.appearance {
				if a.tasks.draft != nil && a.tasks.draft.taskID == id.Nil {
					a.taskCreatePanel(c)
				} else if a.workspace != nil && a.workspace.SelectedID() != id.Nil {
					a.taskDetailModal(c)
				}
			}
		})
	})
	a.pingDialog(c)
	a.toasts(c)
}

func (a *App) toasts(c *ui.Context) {
	t := c.Theme()
	w, _ := c.Size()
	ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		viewport.Justify(ui.Start).AlignItems(ui.Center).Gap(8).Padding(16)
		for _, toast := range toasts {
			parts := ui.ToastBase(c, toast)
			parts.Root.Width(min(float32(330), w-32)).Row().AlignItems(ui.Center).Padding(14).Gap(10).Radius(10).Background(t.Surface).Border(1, t.Border).Shadow(0, 4, 16, 0, ui.RGBA(0, 0, 0, .2)).Children(func() {
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

func (a *App) taskBoardVisible() bool {
	path := a.router.Path()
	return path == PathJobdesk || ((path == PathSpaces || path == PathMedhum) && a.chrome.tab == "Board tugas")
}

// pingDialog opens the shared ping composer over the window. On phones it
// takes the full width as a sheet from the bottom.
func (a *App) pingDialog(c *ui.Context) {
	if !a.chrome.pingOpen {
		return
	}
	if !canPing(a.selectedSpace()) {
		a.chrome.pingOpen = false
		return
	}
	t := c.Theme()
	w, h := c.Size()
	phone := w < mobileMaxWidth
	ui.DialogBase(c.Key("ping-dialog"), &a.chrome.pingOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, .45))
		if phone {
			backdrop.Justify(ui.End)
			panel.Width(w).MaxHeight(h-40).Radius(18, 18, 0, 0)
		} else {
			panel.Width(min(float32(600), w-48)).MaxHeight(h - 80).Radius(16)
		}
		panel.Column().Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Row(c).FillWidth().Padding(14, 18).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				ui.Text(c, "Kirim ping").FontSize(17).Bold().Grow(1)
				if sp := a.selectedSpace(); sp != nil {
					muted(c, sp.Name).Padding(0, 10)
				}
				if spaceIconButton(c, "Tutup ping", "close").Clicked() {
					a.chrome.pingOpen = false
				}
			})
			ui.Scroll(c.Key("ping-scroll")).Grow(1).MinHeight(0).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Padding(18).Gap(10).Children(func() { a.pingComposer(c) })
			})
		})
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
