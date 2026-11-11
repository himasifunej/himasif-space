//go:build !js

package main

import (
	"net/url"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/island"
)

// aceLink is the scheme of the sample notice's actions: Tunda brings it
// back a little later, as a real one would.
const aceLink = "himasif-space://ace"

// aceSayTag is Ace's line during the onboarding, one at a time.
const aceSayTag = "ace-say"

// aceSayHold is how long a line of Ace stays: the card's step usually
// changes sooner, which replaces it.
const aceSayHold = 2 * time.Minute

// aceTrySnooze is how soon the sample notice comes back after Tunda.
const aceTrySnooze = 10 * time.Second

// aceDevice is the desktop's appui.AceDevice: the island and the device
// preferences it is set with. It runs on the main window's thread.
type aceDevice struct{ d *desktopDevice }

func (a aceDevice) Introduced() bool { return a.d.prefs.AceIntroduced }
func (a aceDevice) SetIntroduced() {
	if !a.d.prefs.AceIntroduced {
		a.d.prefs.AceIntroduced = true
		a.d.save()
	}
}

func (a aceDevice) Say(line, face string) bool {
	if !islandSupported() || !a.d.prefs.IslandEnabled {
		return false
	}
	n := appui.Notice{Kind: appui.NoticeAce, Title: "Ace", Body: line, Face: face, Tag: aceSayTag}
	return a.d.island.offerHeld(n, aceSayHold) == nil
}

func (a aceDevice) Hush() { a.d.island.drop(aceSayTag) }

func (a aceDevice) Try() {
	_ = a.d.app.Notifier.Present(aceTryNotice())
}

func aceTryNotice() appui.Notice {
	return appui.Notice{Kind: appui.NoticeTest, Title: "Kabar percobaan", Body: "Begini rasanya kalau ada ping. Tunda membawanya kembali sebentar lagi.", Tag: "ace-try",
		Actions: []appui.NoticeAction{{Label: "Tunda", Link: aceLink + "?action=snooze"}}}
}

// handleAceLink answers the sample notice's Tunda.
func (d *desktopDevice) handleAceLink(u *url.URL) {
	if u.Query().Get("action") == "snooze" {
		time.AfterFunc(aceTrySnooze, func() {
			d.win.Update(func() {
				n := aceTryNotice()
				n.Title = "Kabar percobaan, lagi"
				n.Body = "Ditunda tadi, sekarang muncul lagi. Kabar asli ditunda 15 menit."
				_ = d.app.Notifier.Present(n)
			})
		})
	}
}

func (a aceDevice) Anchor() float64 { return a.d.prefs.IslandAnchor }
func (a aceDevice) SetAnchor(v float64) {
	a.d.prefs.IslandAnchor = islandAnchor(v)
	a.d.saveIslandLook()
}

func (a aceDevice) Look(c *ui.Context) { a.d.islandStyleSettings(c) }

func (a aceDevice) AlwaysOn() bool { return a.d.prefs.IslandAlwaysOn }
func (a aceDevice) SetAlwaysOn(on bool) {
	a.d.prefs.IslandAlwaysOn = on
	a.d.save()
	if on {
		a.d.island.wake()
	} else if a.d.island.win != nil {
		a.d.island.win.Update(func() {})
	}
}

func (a aceDevice) QuietHours() (bool, int, int) {
	p := a.d.prefs
	return p.QuietHours, p.QuietFrom, p.QuietTo
}
func (a aceDevice) SetQuietHours(on bool, from, to int) {
	a.d.prefs.QuietHours, a.d.prefs.QuietFrom, a.d.prefs.QuietTo = on, min(max(from, 0), 23), min(max(to, 0), 23)
	a.d.save()
}
func (a aceDevice) Quiet(t time.Time) bool { return a.d.prefs.quietAt(t) }

func (a aceDevice) Autostart() bool { return a.d.prefs.Autostart }
func (a aceDevice) SetAutostart(on bool) error {
	if err := setDeviceAutostart(on); err != nil {
		return err
	}
	a.d.prefs.Autostart = on
	a.d.save()
	return nil
}

func (a aceDevice) AutoUpdate() bool { return !a.d.prefs.UpdateManual }
func (a aceDevice) SetAutoUpdate(on bool) {
	a.d.prefs.UpdateManual = !on
	a.d.save()
}

// quietAt tells whether t, local time, falls in the quiet hours.
func (p devicePreferences) quietAt(t time.Time) bool {
	return p.QuietHours && appui.QuietAt(t.Local().Hour(), p.QuietFrom, p.QuietTo)
}

// offerHeld offers n to the island to stay for hold, without a Windows
// toast in its place when the island cannot show it.
func (p *islandPresenter) offerHeld(n appui.Notice, hold time.Duration) error {
	p.hold = hold
	defer func() { p.hold = 0 }()
	return p.offer(n, nil)
}

// drop takes the notice tagged tag away, shown or waiting.
func (p *islandPresenter) drop(tag string) {
	p.mu.Lock()
	for i := len(p.m.Queue) - 1; i >= 0; i-- {
		if p.m.Queue[i].Tag == tag {
			p.m.Queue = append(p.m.Queue[:i], p.m.Queue[i+1:]...)
		}
	}
	shown := p.m.Current != nil && p.m.Current.Tag == tag
	if shown {
		p.m.Dismiss(time.Now())
	}
	p.mu.Unlock()
	if shown && p.win != nil {
		p.win.Update(func() {})
	}
}

// aceFace is how the eyes look for a notice of Ace: their height, whether
// the left one winks, and how far down they look.
func aceFace(x *islandNotice) (eyeH float64, wink bool, down float64, ok bool) {
	if x == nil || x.n.Kind != appui.NoticeAce {
		return 0, false, 0, false
	}
	switch x.n.Face {
	case appui.AceFaceHappy:
		return 6, false, 0, true
	case appui.AceFaceWink:
		return island.EyeHeight, true, 0, true
	case appui.AceFaceLook:
		return island.EyeHeight, false, 2.5, true
	}
	return 16, false, 0, true
}
