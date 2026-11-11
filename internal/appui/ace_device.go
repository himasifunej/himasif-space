package appui

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// NoticeAce is Ace speaking from the island during onboarding: one or two
// lines, with the eyes' Face. Presenters other than the island ignore it.
const NoticeAce = "ace"

// Ace's faces, as the island draws its eyes for a NoticeAce.
const (
	AceFaceHappy = "happy" // short arcs, smiling
	AceFaceLook  = "look"  // looking down at the card below
	AceFaceWink  = "wink"  // the left eye shut
)

// AceDevice is this device's Ace and start-up, for the onboarding: the
// desktop sets it; the web has none, and its onboarding leaves those steps
// out. Every setter saves at once.
type AceDevice interface {
	// Introduced tells whether this device has met Ace (the onboarding's
	// Ace steps, or the dialog after an update); SetIntroduced records it.
	Introduced() bool
	SetIntroduced()
	// Say has Ace speak line from the island with face, until Hush. It
	// reports false where the island cannot (turned off), for the card to
	// say it instead.
	Say(line, face string) bool
	Hush()
	// Try shows the sample notice with Buka, Tunda and Tutup.
	Try()
	// Anchor is where the island sits along the top: 0 left, .5 middle,
	// 1 right. The user may also drag the island itself.
	Anchor() float64
	SetAnchor(float64)
	// Look draws the island's style rows (Ikut tema, Hitam, Kaca, Custom),
	// as Pengaturan › Perangkat ini has them.
	Look(c *ui.Context)
	AlwaysOn() bool
	SetAlwaysOn(bool)
	// QuietHours is on with its hours, from and to (0–23, local time);
	// Quiet tells whether t falls in them.
	QuietHours() (on bool, from, to int)
	SetQuietHours(on bool, from, to int)
	Quiet(t time.Time) bool
	Autostart() bool
	SetAutostart(bool) error
	AutoUpdate() bool
	SetAutoUpdate(bool)
}

// QuietAt tells whether hour h falls in quiet hours from–to, which may
// cross midnight (22–6).
func QuietAt(h, from, to int) bool {
	if from == to {
		return false
	}
	if from < to {
		return h >= from && h < to
	}
	return h >= from || h < to
}
