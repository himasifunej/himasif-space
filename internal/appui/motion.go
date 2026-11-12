package appui

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Space's motion tokens (rc.4 feedback: popovers, rows and dialogs came and
// went too snappily). Things appear in motionDuration along motionEase, a
// relaxed ease-out; MyGo gives an element one curve for coming and going,
// and on this one a leaving element covers most of the way in its first
// third, so going reads quicker than coming without a curve of its own.
// State changes (hover, pressed, read) take motionStateDuration. Nothing
// bounces. MyGo moves nothing while the desktop asks for less motion
// (Preferences.ReduceMotion): these fade in place, and colors still ease.
const (
	motionDuration      = 220 * time.Millisecond
	motionStateDuration = 140 * time.Millisecond
)

var (
	// motionEase is an ease-out that settles gently.
	motionEase = ui.CubicBezier(.2, .8, .2, 1)

	// motionPop is a popover's or a menu's panel: it fades in growing a
	// little from its anchor's side, and fades out shrinking.
	motionPop = ui.ElementTransition{Duration: motionDuration, Ease: motionEase, Colors: true,
		Enter: &ui.Motion{Opacity: 0, Scale: .96, Y: -4},
		Exit:  &ui.Motion{Opacity: 0, Scale: .97, Y: -2}}
	// motionBackdrop is a dialog's dimmed window, and motionDialog its
	// panel, which rises a little as it comes.
	motionBackdrop = ui.ElementTransition{Duration: motionDuration, Ease: motionEase, Colors: true,
		Enter: &ui.Motion{Opacity: 0}, Exit: &ui.Motion{Opacity: 0}}
	motionDialog = ui.ElementTransition{Duration: motionDuration, Ease: motionEase, Colors: true,
		Enter: &ui.Motion{Opacity: 0, Scale: .97, Y: 8}}
	// motionRow is a row of a list: it fades in from a little below, and
	// slides out to the side, closing its gap, as it is dismissed.
	motionRow = ui.ElementTransition{Duration: motionDuration, Ease: motionEase,
		Enter: &ui.Motion{Opacity: 0, Y: 4},
		Exit:  &ui.Motion{Opacity: 0, X: 24, Collapse: true}}
	// motionToast comes up from below and fades away.
	motionToast = ui.ElementTransition{Duration: motionDuration, Ease: motionEase, Colors: true,
		Enter: &ui.Motion{Opacity: 0, Y: 12, Scale: .98},
		Exit:  &ui.Motion{Opacity: 0, Scale: .98}}
	// motionState eases a control's background and border between rest,
	// hovered, pressed, selected and disabled.
	motionState = ui.ElementTransition{Duration: motionStateDuration, Ease: ui.EaseOut, Colors: true}
	// motionStateAnim is motionState for a color animated by hand
	// (Element.AnimateColor).
	motionStateAnim = ui.Animation{Duration: motionStateDuration, Ease: ui.EaseOut}
)

// keepOverlayLayer builds the window's overlay layer in every frame, empty
// when nothing is open, so that a popover or a dialog just closed has a
// parent to leave from: MyGo animates an Exit only under a parent still
// built.
func keepOverlayLayer(c *ui.Context) { ui.Overlay(c, func() {}) }

// popPanel gives a popover's panel motionPop, growing from the corner at
// its anchor: originX 0 for a panel starting at its anchor's left, 1 for
// one ending at its right.
func popPanel(panel ui.Element, originX float32) ui.Element {
	return panel.TransformOrigin(originX, 0).Transition(motionPop)
}

// dialogMotion gives a dialog's backdrop and panel their motion.
func dialogMotion(backdrop, panel ui.Element) {
	backdrop.Transition(motionBackdrop)
	panel.Transition(motionDialog)
}
