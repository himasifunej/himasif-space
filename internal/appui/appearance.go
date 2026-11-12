package appui

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/workspaceui"
	"hash/fnv"
)

type spacePalette struct {
	key, name, accent string
	dark, light       [7]string
}

var spacePalettes = []spacePalette{
	{"champagne", "Champagne", "#dbc6a1", [7]string{"#151411", "#11110f", "#1e1d18", "#29271f", "#3c382f", "#efebe3", "#b7b0a2"}, [7]string{"#f7f5ef", "#eeeae1", "#fffdf8", "#e9e3d7", "#d2c9b9", "#28251f", "#615b50"}},
	{"obsidian", "Obsidian Silver", "#d9dce4", [7]string{"#111214", "#0b0d10", "#1b1d21", "#25282e", "#3b3f48", "#edf0f6", "#b1bac8"}, [7]string{"#f4f5f7", "#e9ecf0", "#ffffff", "#e4e8ee", "#c9d0db", "#242a35", "#566173"}},
	{"midnight", "Midnight", "#a9c0f5", [7]string{"#101621", "#0b101a", "#192234", "#232f46", "#354760", "#eaf0ff", "#acbfdc"}, [7]string{"#f0f4fb", "#e4ebf6", "#fcfdff", "#dce5f5", "#bdcde5", "#1c2e4a", "#526788"}},
	{"copper", "Copper", "#dca471", [7]string{"#1b1613", "#130f0c", "#28201b", "#362a22", "#514032", "#f5e8df", "#c7af9c"}, [7]string{"#faf3ec", "#f1e4d7", "#fffbf7", "#efddcc", "#d6baa0", "#36281f", "#725b48"}},
	{"graphite", "Graphite", "#88aaff", [7]string{"#15171c", "#0e1014", "#1e222a", "#292f3c", "#3c4556", "#edf0f7", "#b2bdd1"}, [7]string{"#f3f5fa", "#e7ebf4", "#ffffff", "#e0e6f2", "#c3cde0", "#232c40", "#56647e"}},
	// Sakura (owner's rc.2 feedback, "cewe suka pink"): plum-dark and
	// blush-light, pink without going sweet; picking it brings the pink
	// accent along unless an accent was chosen (paletteAccent).
	{"sakura", "Sakura", "#f0a6c4", [7]string{"#1b1218", "#140d12", "#251820", "#33212c", "#4e3443", "#fbeaf2", "#d4b0c2"}, [7]string{"#fdf3f7", "#f7e5ec", "#fffafc", "#f7dfe9", "#e9c3d3", "#3b1d2b", "#7a4d61"}},
}

func paletteTheme(dark bool, p Preferences) *ui.Theme {
	t := spaceTheme(dark, p.Accent, p.Compact)
	for _, palette := range spacePalettes {
		if palette.key == p.Palette {
			colors := palette.dark
			if !dark {
				colors = palette.light
			}
			t.Background = ui.Hex(colors[0])
			t.Surface = ui.Hex(colors[2])
			t.SurfaceHover = ui.Hex(colors[3])
			t.SurfacePressed = t.SurfaceHover
			t.Border = ui.Hex(colors[4])
			t.Text = ui.Hex(colors[5])
			t.TextMuted = ui.Hex(colors[6])
			t.Inverse = t.Text
			t.InverseText = t.Background
			break
		}
	}
	return t
}

func primarySpaceButton(c *ui.Context, label, icon string) ui.Element {
	t := c.Theme()
	b := spaceButton(c, label, icon, false).Background(t.Accent).TextColor(t.AccentText).Border(1, t.Accent)
	if b.Hovered() {
		b.Background(t.AccentHover)
	}
	return onbPress(b)
}

func (a *App) paletteChoices(c *ui.Context) {
	p := &a.chrome.preferences
	t := c.Theme()
	ui.Text(c, "Palet tema").Bold()
	for row := 0; row < 3; row++ {
		ui.Row(c.Key(row)).FillWidth().Gap(8).Children(func() {
			for col := 0; col < 2; col++ {
				index := row*2 + col
				if index >= len(spacePalettes) {
					ui.Box(c).Grow(1).MinWidth(0)
					continue
				}
				palette := spacePalettes[index]
				active := p.Palette == palette.key || p.Palette == "" && palette.key == "champagne"
				b := ui.ButtonBase(c.Key(palette.key)).Label("Palet "+palette.name).Column().Grow(1).Width(0).MinWidth(0).Padding(10).Gap(8).Radius(10).Border(1, t.Border)
				if active {
					b.BorderColor(t.Accent)
				}
				if b.Hovered() {
					b.Background(t.SurfaceHover)
				}
				b.FocusRing(b.FocusVisible()).Children(func() {
					colors := palette.dark
					if !t.Dark {
						colors = palette.light
					}
					ui.Row(c).Height(35).FillWidth().Radius(5).Background(ui.Hex(colors[0])).Padding(5).Gap(4).Children(func() {
						ui.Box(c).Width(24).FillHeight().Radius(3).Background(ui.Hex(colors[3]))
						for i := 0; i < 3; i++ {
							ui.Column(c.Key(i)).Grow(1).FillHeight().Gap(3).Children(func() {
								ui.Box(c).Height(3).FillWidth().Radius(2).Background(ui.Hex(palette.accent))
								ui.Box(c).Grow(1).FillWidth().Radius(3).Background(ui.Hex(colors[2]))
							})
						}
					})
					ui.Text(c, palette.name).FontSize(12)
				})
				if b.Clicked() {
					p.Palette = palette.key
					p.Accent = paletteAccent(palette.key, p.Accent)
					a.savePreferences(c)
				}
			}
		})
	}
}

func (a *App) boardColorChoices(c *ui.Context) {
	p := &a.chrome.preferences
	ui.Text(c, "Warna board").Bold()
	muted(c, "Warna lajur dan kartu disimpan di perangkat ini.")
	for lane := workspaceui.Todo; lane <= workspaceui.Done; lane++ {
		ui.Row(c.Key(lane)).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, lane.String()).FontSize(12).Grow(1)
			color := p.LaneColors[lane]
			if color == "" {
				color = []string{"#aaa294", "#a2bbd2", "#d7c3a0", "#a8c2ac"}[lane]
			}
			chosen := ui.Hex(color)
			if ui.ColorWell(c, &chosen).Label("Warna lajur " + lane.String()).Height(30).Changed() {
				p.LaneColors[lane] = fmt.Sprintf("#%02x%02x%02x", chosen.R, chosen.G, chosen.B)
				a.savePreferences(c)
			}
		})
	}
	a.cardColorChoices(c)
	if spaceButton(c, "Reset warna lajur", "refresh", false).FillWidth().Clicked() {
		p.LaneColors = [5]string{}
		a.savePreferences(c)
	}
}

func tagColor(category string) ui.Color {
	h := fnv.New32a()
	_, _ = h.Write([]byte(category))
	colors := []string{"#d7c3a0", "#a8c2ac", "#a2bbd2", "#b6a8d0", "#d2a499", "#dca471"}
	return ui.Hex(colors[int(h.Sum32())%len(colors)])
}

// taskCardColor follows the board's color mode: the task's first label by
// default, its status, its PJ, or none. Labels and people keep one color
// everywhere a task appears.
func (a *App) taskCardColor(category, assignee string, lane workspaceui.Lane) ui.Color {
	switch a.chrome.preferences.CardColor {
	case "plain":
		return ui.Color{}
	case "status":
		if color := a.chrome.preferences.LaneColors[lane]; color != "" {
			return spaceAccent(color)
		}
		return ui.Hex([]string{"#8f8878", "#a2bbd2", "#d7c3a0", "#a8c2ac", "#6d6c60"}[lane])
	case "pj":
		if assignee != "" {
			return personColor(assignee)
		}
		return ui.Color{}
	}
	if labels := taskLabels(category); len(labels) > 0 {
		return tagColor(labels[0])
	}
	return ui.Color{}
}

// cardColorChoices is the board's segmented "Warna: label · Status · PJ ·
// Polos" control, also shown in Tampilan.
func (a *App) cardColorChoices(c *ui.Context) {
	p := &a.chrome.preferences
	t := c.Theme()
	ui.Row(c.Key("card-colors")).Gap(2).Padding(3).Radius(9).Border(1, t.Border).AlignItems(ui.Center).Shrink(0).Children(func() {
		for _, mode := range []struct{ key, label string }{{"", "Warna: label"}, {"status", "Status"}, {"pj", "PJ"}, {"plain", "Polos"}} {
			on := p.CardColor == mode.key
			b := ui.ButtonBase(c.Key("card-"+mode.key)).Label("Warna kartu: "+mode.label).Height(30).Padding(0, 12).Radius(7).Center()
			switch {
			case on:
				b.Background(t.Accent).TextColor(t.AccentText)
			case b.Hovered():
				b.Background(t.SurfaceHover).TextColor(t.Text)
			default:
				b.TextColor(t.TextMuted)
			}
			b.FocusRing(b.FocusVisible()).Children(func() {
				label := ui.Text(c, mode.label).FontSize(13)
				if on {
					label.FontWeight(600)
				}
			})
			if b.Clicked() {
				p.CardColor = mode.key
				a.savePreferences(c)
				a.projectTasks()
			}
		}
	})
}
