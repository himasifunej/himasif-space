package appui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/id"
)

// The update card (owner's feedback on rc.2, in the spirit of enowX): a
// card sliding in at the bottom left of the window when the device finds a
// newer Space, with the download's progress, then "Siap dipasang". It
// never takes the focus, waits while the user is writing, stays away
// during the onboarding, and Nanti hides it until the next version or the
// next start. The device's updater keeps the state (Updater).

// Update stages, as the card shows them.
const (
	UpdateAvailable   = "available"
	UpdateDownloading = "downloading"
	UpdateReady       = "ready"
)

// UpdateStatus is what the device's updater has to show.
type UpdateStatus struct {
	Stage   string // UpdateAvailable, UpdateDownloading or UpdateReady
	Version string
	// Note is the first line of the version's signed notes.
	Note        string
	Done, Total int64
	// Auto: automatic updates are on; the version installs itself once
	// Space's window is closed.
	Auto bool
	// Err is the last failure (a download that broke), shown on the card.
	Err string
}

// Updater is the device's updater, for the card; nil on the web.
type Updater interface {
	// UpdateStatus is the version on offer, false while there is none.
	UpdateStatus() (UpdateStatus, bool)
	// InstallUpdate is Perbarui (downloads, then installs) and Mulai ulang
	// sekarang (installs what is downloaded).
	InstallUpdate()
	// UpdateNotes is Lihat perubahan.
	UpdateNotes()
	// UpdateLater is Nanti: a version downloaded then installs when Space
	// closes, even with automatic updates off.
	UpdateLater()
}

// updateCardState is the card's: the version put off with Nanti (until
// the next version or the next start), whether it is folded to a pill,
// and whether it is already on screen (shown, it no longer waits).
type updateCardState struct {
	hidden  string
	small   bool
	showing string
}

// UpdateLater hides the card until a newer version (Nanti, from the card
// or from Ace).
func (a *App) UpdateLater(version string) {
	a.updates.hidden = version
	a.updates.showing = ""
	if a.Updater != nil {
		a.Updater.UpdateLater()
	}
}

// composing tells whether the user is writing something: a task, a
// reminder, a ping, a note, a search. The card does not slide in then.
func (a *App) composing() bool {
	return a.tasks.draft != nil || a.reminders.draft != nil || a.chrome.pingOpen || a.chrome.searchOpen ||
		(a.notes.editor != nil && (a.notes.editing || a.notes.dirty))
}

// updateCard is the card to show, if any.
func (a *App) updateCard() (UpdateStatus, bool) {
	if a.Updater == nil || a.screen != screenShell || a.user.ID == id.Nil {
		return UpdateStatus{}, false
	}
	s, ok := a.Updater.UpdateStatus()
	if !ok || s.Version == "" || a.updates.hidden == s.Version {
		a.updates.showing = ""
		return UpdateStatus{}, false
	}
	// The onboarding (welcome, profile, tour) and "Yang baru" come first.
	if a.Onboarding() || a.aceUpdateDue() {
		return UpdateStatus{}, false
	}
	if a.updates.showing != s.Version && a.composing() {
		return UpdateStatus{}, false
	}
	if a.updates.showing != s.Version {
		a.updates.showing, a.updates.small = s.Version, false
	}
	return s, true
}

// Onboarding tells whether the onboarding is on screen (welcome, profile,
// tour): the updater's announcements wait until it ends.
func (a *App) Onboarding() bool {
	return a.onb.tour.open || a.onboardingActive() && (a.onb.welcome || a.onb.profileOpen)
}

// updatePercent is the download's progress, 0 to 100.
func updatePercent(s UpdateStatus) int {
	if s.Total <= 0 {
		return 0
	}
	return int(min(100, s.Done*100/s.Total))
}

// updateCardText is the card's title and line for its stage.
func updateCardText(s UpdateStatus) (string, string) {
	switch s.Stage {
	case UpdateDownloading:
		return "Mengunduh Space " + s.Version, fmt.Sprintf("%d%% · %.1f dari %.1f MB", updatePercent(s), float64(s.Done)/1e6, float64(s.Total)/1e6)
	case UpdateReady:
		line := "Sudah diunduh dan diperiksa. Space tertutup sebentar lalu terbuka lagi."
		if s.Auto {
			line = "Sudah diunduh dan diperiksa. Dipasang sendiri saat Space ditutup."
		}
		return "Siap dipasang: Space " + s.Version, line
	}
	note := s.Note
	if note == "" {
		note = "Versi baru Space."
	}
	return "Space " + s.Version + " tersedia", note
}

// firstNoteLine is the first line of a version's notes, without its bullet.
func firstNoteLine(notes string) string {
	for _, l := range strings.Split(notes, "\n") {
		l = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(l), "-•*"))
		if l != "" {
			return l
		}
	}
	return ""
}

// FirstNoteLine is firstNoteLine for the device.
func FirstNoteLine(notes string) string { return firstNoteLine(notes) }

// updateCardView draws the card, or its pill when folded.
func (a *App) updateCardView(c *ui.Context) {
	s, ok := a.updateCard()
	if !ok {
		return
	}
	t := c.Theme()
	w, _ := c.Size()
	width := min(float32(340), w-32)
	bottom := float32(16)
	if isMobile(c) {
		bottom = 88 // above the tab bar
	}
	enter := ui.ElementTransition{Duration: 260 * time.Millisecond, Position: true, Enter: &ui.Motion{Y: 24, Opacity: 0}, Exit: &ui.Motion{Y: 24, Opacity: 0}}
	ui.Overlay(c, func() {
		if a.updates.small && s.Stage == UpdateDownloading {
			pill := ui.ButtonBase(c.Key("update-pill")).Label(fmt.Sprintf("Mengunduh Space %s, %d%%. Tampilkan", s.Version, updatePercent(s))).
				Absolute().Left(16).Bottom(bottom).Row().Gap(8).AlignItems(ui.Center).Padding(8, 12).Radius(18).
				Background(t.Surface).Border(1, t.Border).Shadow(0, 6, 20, 0, ui.RGBA(0, 0, 0, .25)).Transition(enter)
			pill.Children(func() {
				ui.Icon(c, spaceIcon("refresh")).Size(14, 14).TextColor(t.Accent)
				ui.Text(c, fmt.Sprintf("%d%%", updatePercent(s))).FontSize(12).FontFeatures("tnum")
			})
			if pill.Clicked() {
				a.updates.small = false
			}
			return
		}
		title, line := updateCardText(s)
		ui.Column(c.Key("update-card")).Label(title).Absolute().Left(16).Bottom(bottom).Width(width).Padding(14, 16).Gap(10).Radius(14).
			Background(t.Surface).Border(1, t.Border).Shadow(0, 10, 30, 0, ui.RGBA(0, 0, 0, .3)).Transition(enter).Children(func() {
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Start).Children(func() {
				ui.Box(c).Size(28, 28).Radius(8).Center().Shrink(0).Background(accentSoft(t)).Children(func() {
					ui.Icon(c, spaceIcon("refresh")).Size(15, 15).TextColor(t.Accent)
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
					ui.Text(c, title).FontSize(14).FontWeight(600)
					ui.Text(c, line).FontSize(12).TextColor(t.TextMuted).MaxLines(2)
				})
				if s.Stage == UpdateDownloading && spaceIconButton(c, "Kecilkan", "down").Clicked() {
					a.updates.small = true
				}
			})
			if s.Stage == UpdateDownloading {
				ui.Box(c).FillWidth().Height(6).Radius(3).Background(t.Border).Children(func() {
					ui.Box(c).Height(6).Radius(3).Background(t.Accent).WidthPercent(float32(updatePercent(s)))
				})
			}
			if s.Err != "" {
				ui.Text(c, s.Err).FontSize(12).TextColor(t.Danger).MaxLines(3)
			}
			ui.Row(c).FillWidth().Wrap().Gap(6).Justify(ui.End).Children(func() {
				switch s.Stage {
				case UpdateAvailable:
					if spaceButton(c, "Lihat perubahan", "", false).MinHeight(30).Clicked() {
						a.Updater.UpdateNotes()
					}
					if spaceButton(c, "Nanti", "", false).MinHeight(30).Clicked() {
						a.UpdateLater(s.Version)
					}
					if primarySpaceButton(c, "Perbarui", "check").MinHeight(30).Clicked() {
						a.Updater.InstallUpdate()
					}
				case UpdateReady:
					if spaceButton(c, "Nanti (saat Space ditutup)", "", false).MinHeight(30).Clicked() {
						a.UpdateLater(s.Version)
					}
					if primarySpaceButton(c, "Mulai ulang sekarang", "refresh").MinHeight(30).Clicked() {
						a.Updater.InstallUpdate()
					}
				}
			})
		})
	})
}
