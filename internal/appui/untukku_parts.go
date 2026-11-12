package appui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
)

// Parts the Untukku pages of canvas 3747dbbac0d0 share: the quiet
// segmented control, the back link, the boxes of a detail page, Ace's pill
// and a toast that can be undone.

// quietSeg is the canvas' .seg: the chosen option on the hover color, not
// the accent, each with an optional count after it. Returns the option
// clicked this frame, or -1.
func quietSeg(c *ui.Context, label string, options []string, counts []int, selected int) int {
	t := c.Theme()
	clicked := -1
	ui.Row(c.Key("qseg-"+label)).Label(label).Gap(2).Padding(2).Radius(8).Border(1, t.Border).Shrink(0).Children(func() {
		for i, o := range options {
			name := o
			if i < len(counts) {
				name = fmt.Sprintf("%s, %d", o, counts[i])
			}
			b := ui.ButtonBase(c.Key(i)).Label(name).Row().Gap(6).AlignItems(ui.Center).Height(28).Padding(0, 12).Radius(6).TextColor(t.TextMuted)
			switch {
			case i == selected:
				b.Background(t.SurfaceHover).TextColor(t.Text)
			case b.Hovered():
				b.TextColor(t.Text)
			}
			b.FocusRing(b.FocusVisible()).Children(func() {
				txt := ui.Text(c, o).FontSize(12).MaxLines(1)
				if i == selected {
					txt.FontWeight(500)
				}
				if i < len(counts) {
					n := ui.Text(c, fmt.Sprint(counts[i])).Font(FontMono).FontSize(11).TextColor(faint(t))
					if i == selected {
						n.TextColor(t.TextMuted)
					}
				}
			})
			if b.Clicked() {
				clicked = i
			}
		}
	})
	return clicked
}

// backLink is "← Untukku" above a page's title.
func backLink(c *ui.Context, text string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key("back")).Label("Kembali ke "+text).Row().Gap(6).AlignItems(ui.Center).AlignSelf(ui.Start).Padding(2, 0).Shrink(0).TextColor(t.TextMuted)
	if b.Hovered() {
		b.TextColor(t.Text)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		ui.Icon(c, spaceIcon("left")).Size(14, 14)
		ui.Text(c, text).FontSize(12)
	})
	return b
}

// detailBox is the canvas' .box: a bordered card of a detail page.
func detailBox(c *ui.Context, key string, fn func()) ui.Element {
	t := c.Theme()
	return ui.Column(c.Key(key)).FillWidth().MinWidth(0).Gap(10).Padding(16, 18).Radius(12).Border(1, t.Border).Children(fn)
}

// boxTitle is a box's small heading, as "Deskripsi".
func boxTitle(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).FontSize(12).FontWeight(600).TextColor(c.Theme().TextMuted)
}

// factRow is one line of the canvas' .kv grid: a muted label, then the
// value.
func factRow(c *ui.Context, key, label string, value func()) {
	ui.Row(c.Key(key)).FillWidth().Gap(12).AlignItems(ui.Start).Children(func() {
		ui.Text(c, label).FontSize(13).TextColor(c.Theme().TextMuted).Width(84).Shrink(0)
		ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(value)
	})
}

// personLine is an avatar and a name, as the canvas' .person.
func personLine(c *ui.Context, key any, name, note string) {
	ui.Row(c.Key(key)).Gap(8).AlignItems(ui.Center).MinWidth(0).Children(func() {
		avatar(c, name, 24)
		ui.Text(c, name).FontSize(13).MaxLines(1).MinWidth(0)
		if note != "" {
			ui.Text(c, note).FontSize(12).TextColor(faint(c.Theme())).Shrink(0)
		}
	})
}

// acePill is how Ace shows a notice on the device: a black pill with its
// two eyes, the title and a line under it.
func acePill(c *ui.Context, key, title, sub string) ui.Element {
	return ui.Row(c.Key(key)).Label("Pratinjau Ace: "+title).Height(56).MaxWidth(360).FillWidth().Gap(14).AlignItems(ui.Center).Padding(0, 20, 0, 22).Radius(28).Background(ui.Hex("#000000")).Children(func() {
		ui.Row(c).Gap(10).Shrink(0).Children(func() {
			for i := range 2 {
				ui.Box(c.Key(i)).Size(9, 14).Radius(5).Background(ui.Hex("#f6f1e6"))
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, title).FontSize(13).FontWeight(500).TextColor(ui.Hex("#e9e5dc")).MaxLines(1)
			ui.Text(c, sub).FontSize(11).TextColor(ui.Hex("#aaa294")).MaxLines(1)
		})
	})
}

// toast shows a message on the next frame; with undo, an Urungkan button
// that runs it.
func (a *App) toast(title string, undo func()) {
	tt := ui.Toast{Title: title, Type: "success", Timeout: 6 * time.Second}
	if undo != nil {
		tt.Action, tt.OnAction = "Urungkan", undo
	}
	a.chrome.toastQueue = append(a.chrome.toastQueue, tt)
}
