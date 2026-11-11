package appui

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// Canvas A of the device page (Pengaturan › Perangkat ini): a column of
// sections at the left, each with its state, above a search; at the right
// the sections, each a box of rows: a label and its description at the
// left, a switch or a small control at the right, saved as it changes.
// Narrow, the search sits above the sections and wide controls go under
// their label. The desktop's device settings (package main) and the web
// app's draw with these.

// settingsPage is the state of the device page now drawn. Only the UI
// thread touches it.
var settingsPage struct {
	query   string
	narrow  bool
	section string // the section being drawn
	matched bool   // the section's own title matches the query
	first   bool   // the next row is the first of its box
	shown   int    // sections drawn this frame
	active  string // the section last chosen in the column
	// words are the texts of each section's rows, as drawn without a
	// query: a search shows a section when one of them matches.
	words map[string][]string
}

// SettingsNavItem is a section in the column of the device page.
type SettingsNavItem struct {
	Title, Icon, Status string
}

// settingsMatch reports whether text holds every word of the query.
func settingsMatch(text string) bool {
	q := strings.Fields(strings.ToLower(settingsPage.query))
	text = strings.ToLower(text)
	for _, w := range q {
		if !strings.Contains(text, w) {
			return false
		}
	}
	return true
}

func settingsSearching() bool { return strings.TrimSpace(settingsPage.query) != "" }

// SettingsLayout draws a device page titled title: the column of nav and
// its search at the left of the sections when the pane is wide, the search
// above them when it is narrow. status draws at the right of the title, as
// the saved state; body draws the sections.
func SettingsLayout(c *ui.Context, title, description string, nav []SettingsNavItem, status func(), body func()) {
	t := c.Theme()
	w, _ := c.Size()
	// The column of sections needs room beside Space's sidebar and the
	// settings tabs; a phone gets the 16 px gutters of the HP design.
	settingsPage.narrow = w < 1400
	phone := w < 600
	settingsPage.shown = 0
	if settingsPage.words == nil {
		settingsPage.words = map[string][]string{}
	}
	sections := func() {
		page := ui.Column(c).FillWidth().MinWidth(0).MaxWidth(820).Padding(28, 40, 64, 40).Gap(28)
		if phone {
			page.Padding(20, 16, 48, 16).Gap(24)
		}
		page.Children(func() {
			var head ui.Element
			if phone {
				head = ui.Column(c.Key("head")).FillWidth().MinWidth(0).Gap(8)
			} else {
				head = ui.Row(c.Key("head")).FillWidth().MinWidth(0).Gap(12).AlignItems(ui.End)
			}
			head.Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Text(c, title).FontSize(22).Bold()
					muted(c, description).FontSize(13)
				})
				if status != nil {
					status()
				}
			})
			if settingsPage.narrow {
				SettingsSearch(c)
			}
			body()
			if settingsSearching() && settingsPage.shown == 0 {
				ui.Column(c.Key("settings-none")).FillWidth().Gap(8).Padding(32, 0).AlignItems(ui.Center).Children(func() {
					muted(c, "Tidak ada pengaturan yang cocok dengan \""+strings.TrimSpace(settingsPage.query)+"\".").FontSize(13)
					if outlineButton(c, "Hapus pencarian", "", false).Clicked() {
						settingsPage.query = ""
					}
				})
			}
		})
	}
	if settingsPage.narrow {
		ui.Scroll(c.Key("settings-body")).Fill().Label("Isi pengaturan").Children(sections)
		return
	}
	ui.Row(c).Fill().MinWidth(0).Children(func() {
		ui.Column(c.Key("settings-sections")).Width(248).Shrink(0).FillHeight().Padding(20, 12).Gap(2).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Label("Bagian pengaturan").Children(func() {
			SettingsSearch(c)
			ui.Column(c).Height(10).Shrink(0)
			settingsNav(c, nav)
		})
		ui.Scroll(c.Key("settings-body")).Grow(1).MinWidth(0).FillHeight().Label("Isi pengaturan").Children(sections)
	})
}

// SettingsSearch is the page's search: it keeps the sections and rows that
// hold every word typed.
func SettingsSearch(c *ui.Context) {
	ui.TextInput(c.Key("settings-search"), &settingsPage.query).Label("Cari di Perangkat ini").Placeholder("Cari di Perangkat ini").FillWidth()
}

// settingsNav is the column of sections: a click scrolls to the section and
// outlines it, as quick navigation does.
func settingsNav(c *ui.Context, items []SettingsNavItem) {
	t := c.Theme()
	if settingsPage.active == "" && len(items) > 0 {
		settingsPage.active = items[0].Title
	}
	for _, it := range items {
		on := settingsPage.active == it.Title
		b := ui.ButtonBase(c.Key("nav-"+it.Title)).Label(it.Title+", "+it.Status).Row().Gap(10).Padding(8, 10).MinHeight(48).Radius(8).AlignItems(ui.Start).FillWidth().TextColor(t.TextMuted)
		switch {
		case on:
			b.Background(t.SurfaceHover).TextColor(t.Text)
		case b.Hovered():
			b.Background(t.SurfaceHover).TextColor(t.Text)
		}
		b.FocusRing(b.FocusVisible()).Children(func() {
			ui.Icon(c, spaceIcon(it.Icon)).Size(16, 16).Shrink(0).Margin(2, 0, 0, 0)
			ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
				name := ui.Text(c, it.Title).FontSize(13).MaxLines(1)
				if on {
					name.FontWeight(500)
				}
				if it.Status != "" {
					ui.Text(c, it.Status).FontSize(12).TextColor(t.TextMuted).MaxLines(1)
				}
			})
		})
		if b.Clicked() {
			settingsPage.active = it.Title
			settingsPage.query = ""
			RevealSettingsSection(it.Title)
		}
	}
}

// RevealSettingsSection scrolls the open page to the section titled title
// and outlines it for a moment.
func RevealSettingsSection(title string) {
	revealing.title, revealing.asked, revealing.until, revealing.scrolled = title, time.Now(), time.Time{}, false
}

// settingsSectionShown reports whether the section titled title shows with
// the query now typed.
func settingsSectionShown(title string) bool {
	if !settingsSearching() || settingsMatch(title) {
		return true
	}
	for _, w := range settingsPage.words[title] {
		if settingsMatch(w) {
			return true
		}
	}
	return false
}

// SettingSection draws a section titled title: its heading and description
// above a box of the rows fn draws. A search hides it when neither its
// title nor any of its rows matches.
func SettingSection(c *ui.Context, title, description string, fn func()) {
	if !settingsSectionShown(title) {
		return
	}
	t := c.Theme()
	settingsPage.shown++
	prev, prevMatched := settingsPage.section, settingsPage.matched
	settingsPage.section, settingsPage.matched, settingsPage.first = title, settingsSearching() && settingsMatch(title), true
	if !settingsSearching() {
		settingsPage.words[title] = settingsPage.words[title][:0]
	}
	ui.Column(c.Key("section-" + title)).FillWidth().MinWidth(0).Gap(10).Children(func() {
		ui.Column(c).FillWidth().MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, title).FontSize(15).Bold()
			if description != "" {
				muted(c, description).FontSize(13)
			}
		})
		box := ui.Column(c).FillWidth().MinWidth(0).Radius(12).Background(t.Surface).Border(1, t.Border)
		RevealSection(c, box, title)
		box.Children(fn)
	})
	settingsPage.section, settingsPage.matched = prev, prevMatched
}

// SettingPreview draws a picture at the top of a section's box, as the
// island's preview; a search hides it.
func SettingPreview(c *ui.Context, height float32, fn func()) {
	if settingsSearching() {
		return
	}
	t := c.Theme()
	settingsPage.first = false
	ui.Column(c.Key("preview")).FillWidth().Height(height).Radius(12, 12, 0, 0).Background(t.SurfaceHover).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(fn)
}

// settingsRowShown records the row's text for the search and reports
// whether it shows with the query now typed.
func settingsRowShown(title, description string) bool {
	text := title + " " + description
	if !settingsSearching() {
		settingsPage.words[settingsPage.section] = append(settingsPage.words[settingsPage.section], text)
		return true
	}
	return settingsPage.matched || settingsMatch(text)
}

// settingRow is a row of the box: the label and its description, then the
// control; stack puts the control under the label on a narrow page. It
// reports a click on the label, which a switch row takes as a toggle.
func settingRow(c *ui.Context, title, description string, stack bool, control func()) (labelClicked bool) {
	t := c.Theme()
	var row ui.Element
	if stack && settingsPage.narrow {
		row = ui.Column(c.Key("row-"+title)).FillWidth().MinWidth(0).Padding(14, 16).Gap(10)
	} else {
		row = ui.Row(c.Key("row-"+title)).FillWidth().MinWidth(0).Padding(14, 16).Gap(16).AlignItems(ui.Center)
	}
	if !settingsPage.first {
		row.BorderWidth(1, 0, 0, 0).BorderColor(t.Border)
	}
	settingsPage.first = false
	row.Children(func() {
		label := ui.Column(c.Key("label")).Grow(1).MinWidth(0).MaxWidth(600).Gap(2)
		labelClicked = label.Clicked()
		label.Children(func() {
			ui.Text(c, title).FontSize(14).FontWeight(500)
			if description != "" {
				muted(c, description).FontSize(13)
			}
		})
		ui.Row(c).Shrink(0).Gap(10).AlignItems(ui.Center).Wrap().Children(control)
	})
	return labelClicked
}

// SettingRow is a row with a control of its own: a choice, a slider,
// buttons. On a narrow page the control goes under the label.
func SettingRow(c *ui.Context, title, description string, control func()) {
	if !settingsRowShown(title, description) {
		return
	}
	settingRow(c, title, description, true, control)
}

// SettingSwitch is a row with a switch for *on; it reports whether the
// switch changed it this frame, for the caller to save and apply.
func SettingSwitch(c *ui.Context, title, description string, on *bool) bool {
	if !settingsRowShown(title, description) {
		return false
	}
	changed := false
	if settingRow(c, title, description, false, func() {
		changed = ui.Switch(c.Key("switch"), on).Label(title).Changed()
	}) && !changed {
		*on, changed = !*on, true
	}
	return changed
}

// SettingButton is a bordered button for a row's control.
func SettingButton(c *ui.Context, label, icon string) ui.Element {
	return outlineButton(c.Key("btn-"+label), label, icon, false)
}

// Segmented is the canvas' segmented control; it returns the option
// clicked this frame, or -1.
func Segmented(c *ui.Context, label string, options []string, selected int) int {
	return segmented(c, label, options, selected)
}

// SettingsSaved shows that the page saves as it changes: "Tersimpan
// otomatis" until a first change, then the time of the last.
func SettingsSaved(c *ui.Context, at time.Time) {
	t := c.Theme()
	text := "Tersimpan otomatis"
	if !at.IsZero() {
		text = "Tersimpan · " + at.Format("15.04")
	}
	ui.Row(c.Key("saved")).Shrink(0).Gap(6).AlignItems(ui.Center).Label(text).Children(func() {
		ui.Column(c).Size(8, 8).Radius(4).Background(t.Success)
		ui.Text(c, text).FontSize(12).TextColor(t.TextMuted).MaxLines(1)
	})
}

// SettingsNote is a line under the sections; a search hides it.
func SettingsNote(c *ui.Context, text string) {
	if settingsSearching() {
		return
	}
	muted(c, text).FontSize(13)
}

// SettingsBlock draws sections of the device page's kind (SettingSection)
// outside it, as the app's updates in Status layanan: the page's search
// does not hide them.
func SettingsBlock(c *ui.Context, fn func()) {
	query, narrow := settingsPage.query, settingsPage.narrow
	settingsPage.query = ""
	w, _ := c.Size()
	settingsPage.narrow = w < 1400
	if settingsPage.words == nil {
		settingsPage.words = map[string][]string{}
	}
	fn()
	settingsPage.query, settingsPage.narrow = query, narrow
}
