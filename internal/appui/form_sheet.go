package appui

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// One pattern for the forms that open at the side (canvas 3c8b4b7134bc,
// the owner's rc.6 report that "Ubah rapat" sliding in felt confusing):
// a sheet over the dimmed page, from the right edge, 560 px wide (the
// whole screen on a phone). Its head says what is edited and where, with
// the close button; its body is sections, each a small heading, a line on
// what it is for, then its fields, a label above each control and a hint
// under it; its foot stays in view with the state at the left (unsaved
// changes, an error), then Batal and the one primary action. Esc, a click
// beside the sheet and its close button ask first when something typed
// would be lost; Batal drops it as said; Ctrl+Enter saves.

// sheetWidth is a form sheet's width beside the page.
const sheetWidth = 560

// formSheet describes a side sheet.
type formSheet struct {
	key   string
	title string // "Ubah rapat"
	// closeLabel names the close button; "Tutup ubah rapat" when empty.
	closeLabel string
	context    string // the line under it: where, or what
	// head replaces the title and context, for a sheet that shows more
	// there (a member's avatar).
	head func()
	// open is the sheet's own flag when it keeps one; nil shows it while
	// it is drawn.
	open *bool
	// dirty is whether closing would lose what was typed: closing then
	// asks first.
	dirty bool
	// discard is the question asked then, "Buang perubahan …?".
	discard string
	busy    bool
	// state is the foot's line at the left; err shows above it.
	state, err string
	// submit is the primary action's label and icon; blocked disables it.
	submit, icon string
	blocked      bool
	onSubmit     func()
	// onClose closes the sheet: called once closing is decided.
	onClose func()
	// extra draws more of the foot before Batal (a secondary action).
	extra func()
}

// formSheetDiscard is the key of the sheet asking whether to drop its
// changes; only the UI thread touches it.
var formSheetDiscard string

// sideSheet draws f with body as its sections.
func (a *App) sideSheet(c *ui.Context, f formSheet, body func()) {
	t := c.Theme()
	vw, _ := c.Size()
	phone := isMobile(c) || vw < 720
	open := true
	if f.open != nil {
		open = *f.open
	}
	if !f.dirty && formSheetDiscard == f.key {
		formSheetDiscard = ""
	}
	closeAsked := false
	ui.DialogBase(c.Key(f.key), &open, func(backdrop, panel ui.Element) {
		dialogMotion(backdrop, panel)
		backdrop.Background(ui.RGBA(0, 0, 0, .45)).Row().Justify(ui.End).AlignItems(ui.Stretch)
		if phone {
			panel.Width(vw).FillHeight()
		} else {
			panel.Width(min(float32(sheetWidth), vw-48)).FillHeight().BorderWidth(0, 0, 0, 1)
		}
		panel.Column().Background(t.Surface).BorderColor(t.Border).Label(f.title).Children(func() {
			ui.Row(c.Key("sheet-head")).FillWidth().MinWidth(0).Padding(18, 24, 16, 24).Gap(12).AlignItems(ui.Start).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				if f.head != nil {
					ui.Row(c).Grow(1).MinWidth(0).Gap(12).AlignItems(ui.Center).Children(f.head)
				} else {
					ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
						ui.Text(c, f.title).FontSize(18).Bold().MaxLines(2)
						if f.context != "" {
							ui.Text(c, f.context).FontSize(12).TextColor(t.TextMuted).MaxLines(1)
						}
					})
				}
				if !phone {
					ui.Text(c, "Esc").FontSize(11).TextColor(faint(t)).Padding(1, 5).Radius(4).Border(1, t.Border).AlignSelf(ui.Center).Shrink(0)
				}
				closeLabel := f.closeLabel
				if closeLabel == "" {
					closeLabel = "Tutup " + lowerFirst(f.title)
				}
				if spaceIconButton(c, closeLabel, "close").Disabled(f.busy).Clicked() {
					closeAsked = true
				}
			})
			ui.Scroll(c.Key("sheet-body")).Grow(1).MinHeight(0).FillWidth().Label("Isi " + lowerFirst(f.title)).Children(func() {
				pad := float32(24)
				if phone {
					pad = 16
				}
				ui.Column(c).FillWidth().MinWidth(0).Padding(4, pad, 24, pad).Children(body)
			})
			a.formSheetFoot(c, f, phone)
		})
		if !f.blocked && !f.busy && f.onSubmit != nil && panel.Shortcut(ui.Ctrl, ui.KeyEnter) {
			f.onSubmit()
		}
	})
	if !open && !f.busy {
		closeAsked = true
	}
	if f.open != nil {
		*f.open = true
	}
	if closeAsked {
		if f.dirty && formSheetDiscard != f.key {
			formSheetDiscard = f.key
			return
		}
		formSheetDiscard = ""
		f.onClose()
	}
}

// formSheetFoot is the sheet's foot: the state, Batal and the primary
// action; or, when closing would lose changes, the question.
func (a *App) formSheetFoot(c *ui.Context, f formSheet, phone bool) {
	t := c.Theme()
	pad := float32(24)
	if phone {
		pad = 16
	}
	ui.Column(c.Key("sheet-foot")).FillWidth().MinWidth(0).Padding(12, pad, 14, pad).Gap(8).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Background(t.Surface).Children(func() {
		if f.err != "" {
			ui.Text(c, f.err).FontSize(12).TextColor(t.Danger).LineHeight(1.5)
		}
		if formSheetDiscard == f.key {
			question := f.discard
			if question == "" {
				question = "Buang isian yang belum disimpan?"
			}
			ui.Text(c, question).FontSize(13).Bold().LineHeight(1.5)
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Wrap().Children(func() {
				if spaceButton(c, "Lanjut mengisi", "", false).Clicked() {
					formSheetDiscard = ""
				}
				if primarySpaceButton(c, "Buang perubahan", "close").Clicked() {
					formSheetDiscard = ""
					f.onClose()
				}
			})
			return
		}
		ui.Row(c).FillWidth().MinWidth(0).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
			state, color := f.state, t.TextMuted
			if f.busy {
				state = "Menyimpan…"
			} else if state == "" && f.dirty {
				state, color = "Perubahan belum disimpan", t.Accent
			}
			ui.Text(c, state).FontSize(12).TextColor(color).Grow(1).Basis(120).MinWidth(0).MaxLines(2)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if f.extra != nil {
					f.extra()
				}
				if spaceButton(c, "Batal", "", false).Disabled(f.busy).Clicked() {
					formSheetDiscard = ""
					f.onClose()
				}
				if f.onSubmit != nil {
					b := primarySpaceButton(c, f.submit, f.icon).Disabled(f.blocked || f.busy)
					if !phone {
						b.Tooltip("Ctrl+Enter")
					}
					if b.Clicked() {
						f.onSubmit()
					}
				}
			})
		})
	})
}

// formSection is a group of a sheet's fields: its heading, a line on what
// it is for, then fn's fields, apart from the next group by a rule.
func formSection(c *ui.Context, title, lead string, fn func()) {
	t := c.Theme()
	ui.Column(c.Key("form-sec-"+title)).FillWidth().MinWidth(0).Gap(16).Padding(20, 0).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Column(c).FillWidth().MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, strings.ToUpper(title)).FontSize(11).FontWeight(600).TextColor(t.TextMuted)
			if lead != "" {
				ui.Text(c, lead).FontSize(12).TextColor(faint(t)).LineHeight(1.5)
			}
		})
		fn()
	})
}

// formPair puts two fields side by side, under each other on a phone.
func formPair(c *ui.Context, left, right func()) {
	vw, _ := c.Size()
	if isMobile(c) || vw < 720 {
		ui.Column(c).FillWidth().MinWidth(0).Gap(16).Children(func() {
			ui.Column(c.Key("left")).FillWidth().MinWidth(0).Children(left)
			ui.Column(c.Key("right")).FillWidth().MinWidth(0).Children(right)
		})
		return
	}
	// Halves of equal width: each grows from nothing along the row.
	ui.Row(c).FillWidth().MinWidth(0).Gap(12).AlignItems(ui.Start).Children(func() {
		ui.Column(c.Key("left")).Grow(1).Basis(0).MinWidth(0).Children(left)
		ui.Column(c.Key("right")).Grow(1).Basis(0).MinWidth(0).Children(right)
	})
}

// formHint is a line under a field or a group of them.
func formHint(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(12).TextColor(faint(c.Theme())).LineHeight(1.5)
}

// lowerFirst is s with its first letter small, for "Tutup ubah rapat".
func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'A' && r[0] <= 'Z' {
		r[0] += 'a' - 'A'
	}
	return string(r)
}
