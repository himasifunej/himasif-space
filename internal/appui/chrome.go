package appui

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/workspaceui"
)

type chromeState struct {
	// notice is a message shown as a toast on the next frame.
	notice      string
	preferences Preferences
	datePickers map[string]*datePickerState
	whens       map[string]*whenState
	// toastQueue are toasts to add on the next frame (App.toast).
	toastQueue                                                         []ui.Toast
	drawerWidth                                                        float32
	membersOpen                                                        bool
	tab, settingsTab                                                   string
	divisionID                                                         id.ID
	appearance, periodOpen, profileOpen, notificationsOpen, searchOpen bool
	// searchQuery is the query searchCursor, the highlighted result, is
	// for; searchSubmit is Enter pressed in the search field.
	searchQuery                                  string
	searchCursor                                 int
	searchSubmit                                 bool
	search, customAccent, preferencesErr         string
	members                                      headerMembersState
	syncOpen, pingOpen, pingWhoOpen, pingSrcOpen bool
	// appearanceQuick is the small appearance popover of the sidebar's
	// Tampilan; appearance is the full panel.
	appearanceQuick bool
	// settingsDetail shows one settings page instead of the grouped list
	// on narrow windows.
	settingsDetail bool
	// bell is the bell popover's paging.
	bell bellState
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
	a.pingUI = pingUIState{}
	a.card = cardState{}
	a.untuk = untukState{}
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
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Row().AlignItems(ui.Center).Justify(ui.Start).Gap(10).Padding(9, 12).MinHeight(36).Radius(8).TextColor(t.TextMuted).Transition(motionState)
	switch {
	case active:
		b.Background(t.SurfaceHover).Border(1, t.Border).TextColor(t.Text)
	case b.Pressed():
		b.Background(t.SurfacePressed).TextColor(t.Text)
	case b.Hovered():
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
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Size(34, 34).Center().Radius(8).TextColor(t.TextMuted).Transition(motionState)
	switch {
	case b.Pressed():
		b.Background(t.SurfacePressed).TextColor(t.Text)
	case b.Hovered():
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.FocusRing(b.FocusVisible())
	b.Children(func() { ui.Icon(c, spaceIcon(icon)).Size(18, 18) })
	return b
}
func popover(c *ui.Context, anchor ui.Element, open *bool, width float32, fn func()) {
	ui.PopoverBase(c, anchor, open, func(panel ui.Element) {
		t := c.Theme()
		_, wh := c.Size()
		popPanel(panel, 1).Margin(6, 0, 0, 0).Width(width).MaxHeight(min(420, wh-72)).Clip().Padding(12).Gap(6).Radius(12).Background(t.Surface).Border(1, t.Border).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, .25))
		// What does not fit scrolls inside the panel, never past it.
		ui.Scroll(c).FillWidth().Grow(1).Shrink(1).MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().Gap(6).Children(fn)
		})
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
	a.whatsNewDialog(c)
	// Escape closes the appearance panel, as it closes the popovers. A
	// popover open this frame takes the key itself, so one press closes
	// one thing.
	popoverOpen := a.chrome.searchOpen || a.chrome.notificationsOpen || a.chrome.profileOpen || a.chrome.membersOpen || a.chrome.periodOpen || a.chrome.appearanceQuick || a.chrome.syncOpen
	if a.chrome.appearance && !popoverOpen && c.Shortcut(0, ui.KeyEscape) {
		a.chrome.appearance = false
	}

	keepOverlayLayer(c)
	a.tourFrameStart()
	a.ensureOnboarding(c.Now())
	a.watchNotificationPermission()
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
		a.onboardingOverlays(c)
		a.updateCardView(c)
		a.toasts(c)
		return
	}
	ui.Column(c).Fill().Children(func() {
		a.webBanner(c)
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
					} else if a.guide.panel {
						a.guidePanel(c)
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
	a.onboardingOverlays(c)
	a.updateCardView(c)
	a.toasts(c)
}

func (a *App) toasts(c *ui.Context) {
	t := c.Theme()
	w, _ := c.Size()
	for _, q := range a.chrome.toastQueue {
		c.AddToast(q)
	}
	a.chrome.toastQueue = nil
	ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		viewport.Justify(ui.Start).AlignItems(ui.Center).Gap(8).Padding(16)
		for _, toast := range toasts {
			parts := ui.ToastBase(c, toast)
			parts.Root.Transition(motionToast).Width(min(float32(330), w-32)).Row().AlignItems(ui.Center).Padding(14).Gap(10).Radius(10).Background(t.Surface).Border(1, t.Border).Shadow(0, 4, 16, 0, ui.RGBA(0, 0, 0, .2)).Children(func() {
				icon, color := "info", t.Accent
				switch toast.Type {
				case "success":
					icon, color = "check", t.Success
				case "error":
					icon, color = "alert", t.Danger
				}
				ui.Icon(c, spaceIcon(icon)).Size(18, 18).TextColor(color)
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Text(c, toast.Title).Bold()
					if toast.Description != "" {
						muted(c, toast.Description)
					}
				})
				if toast.Action != "" {
					act := parts.ActionButton().Label(toast.Action).Height(28).Padding(0, 8).Radius(6).Center().Shrink(0)
					if act.Hovered() {
						act.Background(t.SurfaceHover)
					}
					act.Children(func() { ui.Text(c, toast.Action).FontSize(13).FontWeight(500).TextColor(t.Accent) })
				}
				parts.CloseButton().Label("Tutup pesan").Size(26, 26).Center().Children(func() { ui.Icon(c, spaceIcon("close")).Size(14, 14) })
			})
		}
	})
}

func (a *App) taskBoardVisible() bool {
	path := a.router.Path()
	return path == PathJobdesk || path == PathUntukku || ((path == PathSpaces || path == PathMedhum) && a.chrome.tab == "Board tugas")
}

// pingDialog opens the shared ping composer (canvas 8f83cee819ac A) over
// the window: the list scrolls, the message and Kirim stay below it. On
// phones it takes the full width as a sheet from the bottom.
func (a *App) pingDialog(c *ui.Context) {
	a.showPingToast(c)
	if !a.chrome.pingOpen {
		if a.pingUI.sent != nil {
			st := &a.notificationSettingsState
			st.recipients, st.pingMessage, st.err = nil, "", ""
			a.pingUI.sent = nil
		}
		return
	}
	if !canPing(a.selectedSpace()) {
		a.chrome.pingOpen = false
		return
	}
	a.pingUI.boxOpen = false
	t := c.Theme()
	w, h := c.Size()
	phone := w < mobileMaxWidth
	ui.DialogBase(c.Key("ping-dialog"), &a.chrome.pingOpen, func(backdrop, panel ui.Element) {
		dialogMotion(backdrop, panel)
		backdrop.Background(ui.RGBA(0, 0, 0, .45))
		if phone {
			backdrop.Justify(ui.End)
			panel.Width(w).MaxHeight(h-40).Radius(18, 18, 0, 0)
		} else {
			panel.Width(min(float32(560), w-48)).MaxHeight(h - 80).Radius(16)
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
			if !a.pingComposerReady() {
				return
			}
			if a.pingUI.sent != nil {
				ui.Column(c).FillWidth().Padding(18).Gap(10).Children(func() { a.pingSentView(c) })
				return
			}
			ui.Scroll(c.Key("ping-scroll")).Grow(1).MinHeight(0).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Padding(18, 18, 12, 18).Gap(12).Children(func() { a.pingComposerBody(c) })
			})
			ui.Column(c).FillWidth().Padding(14, 18, 18, 18).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() { a.pingComposerFooter(c) })
		})
	})
}
