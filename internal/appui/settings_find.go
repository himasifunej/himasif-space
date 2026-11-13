package appui

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// settingsFindLimit is how many results the settings search lists.
const settingsFindLimit = 12

// settingsFinder is the one search of Pengaturan, at the top of its tabs:
// it looks through every page, section, row and button, from the same
// index as Ctrl K, and lists what matches in place of the tabs. A result
// opens its page scrolled to its section; Enter opens the highlighted one,
// the arrows move it, Esc clears the search. It reports whether results
// are shown, so the tabs are not drawn under them.
func (a *App) settingsFinder(c *ui.Context) bool {
	t := c.Theme()
	query := strings.TrimSpace(a.chrome.settingsFind)
	var results []navItem
	if query != "" {
		results = navMatches(a.navSettings(), query)
		if len(results) > settingsFindLimit {
			results = results[:settingsFindLimit]
		}
	}
	if a.chrome.settingsCursor >= len(results) {
		a.chrome.settingsCursor = 0
	}
	in := ui.TextInput(c.Key("settings-find"), &a.chrome.settingsFind).Label("Cari pengaturan").Placeholder("Cari pengaturan…").FillWidth()
	open := -1
	switch {
	case query != "" && in.Shortcut(0, ui.KeyEscape):
		a.chrome.settingsFind, a.chrome.settingsCursor = "", 0
		return false
	case len(results) > 0 && in.Shortcut(0, ui.KeyDown):
		a.chrome.settingsCursor = (a.chrome.settingsCursor + 1) % len(results)
	case len(results) > 0 && in.Shortcut(0, ui.KeyUp):
		a.chrome.settingsCursor = (a.chrome.settingsCursor - 1 + len(results)) % len(results)
	case len(results) > 0 && in.Submitted():
		open = a.chrome.settingsCursor
	}
	if query == "" {
		ui.Column(c).Height(6).Shrink(0)
		return false
	}
	ui.Column(c.Key("settings-found")).FillWidth().Gap(2).Padding(8, 0, 0, 0).Label("Hasil pencarian pengaturan").Children(func() {
		if len(results) == 0 {
			muted(c, "Tidak ada pengaturan yang cocok dengan “"+query+"”.").FontSize(12).Padding(8)
			return
		}
		for i, it := range results {
			place := a.settingsPlace(it.trail)
			b := ui.ButtonBase(c.Key(it.key)).Label(it.title+" — "+place).FillWidth().Row().Gap(10).Padding(7, 8).Radius(8).AlignItems(ui.Start)
			if i == a.chrome.settingsCursor || b.Hovered() {
				b.Background(t.SurfaceHover)
			}
			b.FocusRing(b.FocusVisible()).Children(func() {
				ui.Icon(c, spaceIcon(it.icon)).Size(16, 16).TextColor(t.TextMuted).Shrink(0).Margin(2, 0, 0, 0)
				ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
					ui.Text(c, it.title).FontSize(13).MaxLines(2)
					muted(c, place).FontSize(11).MaxLines(1)
				})
			})
			if b.Clicked() {
				open = i
			}
		}
	})
	if open >= 0 {
		it := results[open]
		a.chrome.settingsFind, a.chrome.settingsCursor = "", 0
		it.open(c)
	}
	return true
}

// settingsPlace is where a result lives, for the settings search: its page
// by the name the tabs show, then its section ("Tampilan dan tema › Tema").
func (a *App) settingsPlace(trail string) string {
	parts := strings.Split(strings.TrimPrefix(trail, "Pengaturan › "), " › ")
	for _, g := range a.settingsGroups() {
		for _, e := range g.entries {
			if e.key == parts[0] {
				parts[0] = e.label
			}
		}
	}
	return strings.Join(parts, " › ")
}
