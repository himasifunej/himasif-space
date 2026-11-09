package appui

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/workspaceui"
)

// appearancePopover is canvas A's small Tampilan panel beside the sidebar:
// theme with previews, accent, density, and a link to every appearance
// setting (the full panel, also in Pengaturan › Perangkat ini › Tampilan).
// It closes with Escape, its close button or a press outside.
func (a *App) appearancePopover(c *ui.Context, anchor ui.Element) {
	t := c.Theme()
	p := &a.chrome.preferences
	ui.PopoverBase(c.Key("appearance-quick"), anchor, &a.chrome.appearanceQuick, func(panel ui.Element) {
		panel.Width(320).Padding(16).Gap(0).Radius(12).Background(t.Surface).Border(1, t.Border).Shadow(0, 12, 32, 0, ui.RGBA(0, 0, 0, .35)).MarginStart(18)
		heading := func(s string) {
			ui.Text(c, s).FontSize(12).FontWeight(500).TextColor(faint(t)).Padding(14, 0, 8, 0)
		}
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Tampilan").FontSize(15).FontWeight(600).Grow(1)
			x := ui.ButtonBase(c).Label("Tutup tampilan").Size(28, 28).Center().Radius(8).TextColor(t.TextMuted)
			if x.Hovered() {
				x.Background(t.SurfaceHover).TextColor(t.Text)
			}
			x.FocusRing(x.FocusVisible()).Children(func() { ui.Icon(c, chromeIcon("close")).Size(16, 16) })
			if x.Clicked() {
				a.chrome.appearanceQuick = false
			}
		})
		heading("Tema")
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			dark, light := ui.Hex("#141310"), ui.Hex("#f6f3ec")
			for _, mode := range []struct {
				key, label string
				preview    ui.LinearGradient
				border     bool
			}{
				{"dark", "Gelap", ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{{Color: ui.Hex("#1b1a16")}, {Color: ui.Hex("#1b1a16"), At: .3}, {Color: dark, At: .3}, {Color: dark, At: 1}}}, false},
				{"light", "Terang", ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{{Color: ui.Hex("#fffdf8")}, {Color: ui.Hex("#fffdf8"), At: .3}, {Color: ui.Hex("#f0ebe0"), At: .3}, {Color: ui.Hex("#f0ebe0"), At: 1}}}, true},
				{"system", "Ikuti sistem", ui.LinearGradient{Angle: 135, Stops: []ui.GradientStop{{Color: dark}, {Color: dark, At: .5}, {Color: light, At: .5}, {Color: light, At: 1}}}, false},
			} {
				on := p.Theme == mode.key
				b := ui.ButtonBase(c.Key(mode.key)).Label(mode.label).Column().Grow(1).Width(0).MinWidth(0).Padding(6).Gap(6).Radius(10).Border(1, t.Border).TextColor(t.TextMuted).AlignItems(ui.Center)
				if on {
					b.BorderColor(t.Accent).TextColor(t.Text)
				} else if b.Hovered() {
					b.Background(t.SurfaceHover)
				}
				b.FocusRing(b.FocusVisible()).Children(func() {
					preview := ui.Box(c).FillWidth().Height(44).Radius(6).LinearGradient(mode.preview)
					if mode.border {
						preview.Border(1, ui.Hex("#e0d8c8"))
					}
					ui.Text(c, mode.label).FontSize(12).MaxLines(1)
				})
				if b.Clicked() {
					p.Theme = mode.key
					a.savePreferences(c)
				}
			}
		})
		heading("Aksen")
		// Canvas A's four: the default, sage, sky and clay; the full panel
		// also has lavender and a custom colour.
		a.accentDots(c, 28, quickAccents...)
		heading("Kepadatan")
		ui.Row(c).Padding(3).Gap(2).Radius(9).Border(1, t.Border).Children(func() {
			for _, d := range []struct {
				label   string
				compact bool
			}{{"Nyaman", false}, {"Ringkas", true}} {
				on := p.Compact == d.compact
				b := ui.ButtonBase(c.Key(d.label)).Label(d.label).Padding(6, 12).Radius(7).TextColor(t.TextMuted)
				if on {
					b.Background(t.Accent).TextColor(t.AccentText)
				} else if b.Hovered() {
					b.Background(t.SurfaceHover).TextColor(t.Text)
				}
				b.FocusRing(b.FocusVisible()).Children(func() {
					l := ui.Text(c, d.label).FontSize(13)
					if on {
						l.FontWeight(600)
					}
				})
				if b.Clicked() && !on {
					p.Compact = d.compact
					a.savePreferences(c)
				}
			}
		})
		if a.chrome.preferencesErr != "" {
			ui.Text(c, a.chrome.preferencesErr).FontSize(12).TextColor(t.Warning).Padding(10, 0, 0, 0)
		}
		ui.Box(c).FillWidth().Height(1).Background(t.Border).Margin(14, 0, 8, 0)
		more := ui.ButtonBase(c).Label("Semua pengaturan tampilan…").FillWidth().Row().AlignItems(ui.Center).Gap(8).Height(32).Padding(0, 8).Radius(8).TextColor(t.TextMuted)
		if more.Hovered() {
			more.Background(t.SurfaceHover).TextColor(t.Text)
		}
		more.FocusRing(more.FocusVisible()).Children(func() {
			ui.Icon(c, spaceIcon("palette")).Size(16, 16)
			ui.Text(c, "Semua pengaturan tampilan…").FontSize(13).Grow(1)
			ui.Icon(c, spaceIcon("chevron")).Size(14, 14)
		})
		if more.Clicked() {
			a.chrome.appearanceQuick = false
			a.chrome.appearance = true
			if a.workspace != nil {
				a.workspace.Close()
			}
		}
	}).AttachTo(anchor, ui.AnchorBottomRight, ui.AnchorBottomLeft)
}

// quickAccents are the popover's presets; allAccents the full panel's.
var (
	quickAccents = []string{"sand", "sage", "blue", "rose"}
	allAccents   = []string{"sand", "sage", "blue", "lavender", "rose"}
)

// accentNames name the presets for people (tooltips).
var accentNames = map[string]string{"sand": "Pasir (bawaan)", "sage": "Sage", "blue": "Langit", "lavender": "Lavender", "rose": "Tanah liat"}

// accentDots are preset accents as round swatches.
func (a *App) accentDots(c *ui.Context, size float32, accents ...string) {
	t := c.Theme()
	p := &a.chrome.preferences
	ui.Row(c).Gap(10).Children(func() {
		for _, accent := range accents {
			on := p.Accent == accent || p.Accent == "" && accent == "sand"
			b := ui.ButtonBase(c.Key(accent)).Label("Aksen "+accent).Tooltip(accentNames[accent]).Size(size, size).Center().Radius(size/2).Background(spaceAccentFor(accent, t.Dark)).Border(2, ui.RGBA(0, 0, 0, 0))
			if on {
				b.BorderColor(t.Text)
			}
			b.FocusRing(b.FocusVisible())
			if b.Clicked() {
				p.Accent = accent
				a.savePreferences(c)
			}
		}
	})
}

// appearance is the full appearance panel (palettes, custom accent, board
// colors) as a drawer beside the page; on phones it fills the screen.
func (a *App) appearance(c *ui.Context) {
	workspaceui.Drawer(c.Key("appearance-drawer"), &a.chrome.drawerWidth, func() {
		ui.Row(c).FillWidth().Padding(18).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Tampilan").FontSize(18).Bold().Grow(1)
			if spaceIconButton(c, "Tutup panel tampilan", "close").Clicked() {
				a.chrome.appearance = false
			}
		})
		ui.Scroll(c).Label("Scroll tampilan").Grow(1).MinHeight(0).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(18).Gap(14).Children(func() {
				muted(c, "Sesuaikan ruang kerjamu. Preferensi disimpan di perangkat ini.")
				a.appearanceOptions(c)
			})
		})
	})
}

// appearanceSettings is the same panel as a page of Pengaturan.
func (a *App) appearanceSettings(c *ui.Context) {
	settingsBody(c, "Tampilan", "Tema, palet, aksen, kepadatan dan warna board. Preferensi disimpan di perangkat ini.", func() {
		ui.Column(c).FillWidth().MaxWidth(560).Gap(14).Children(func() { a.appearanceOptions(c) })
	})
}

// appearanceOptions are all appearance settings, shared by the drawer and
// the settings page.
func (a *App) appearanceOptions(c *ui.Context) {
	t := c.Theme()
	p := &a.chrome.preferences
	ui.Text(c, "Tema").Bold()
	ui.Row(c).FillWidth().Gap(4).Children(func() {
		for _, mode := range []struct{ key, label, icon string }{{"dark", "Gelap", "moon"}, {"light", "Terang", "sun"}, {"system", "Ikuti sistem", "monitor"}} {
			if spaceButton(c, mode.label, mode.icon, p.Theme == mode.key).Grow(1).MinWidth(0).Padding(7, 5).Gap(4).Clicked() {
				p.Theme = mode.key
				a.savePreferences(c)
			}
		}
	})
	a.paletteChoices(c)
	ui.Text(c, "Warna aksen").Bold()
	a.accentDots(c, 33, allAccents...)
	taskInput(c, "accent-custom", "Warna aksen khusus", &a.chrome.customAccent)
	if spaceButton(c, "Terapkan warna", "palette", false).FillWidth().Clicked() {
		value := strings.ToLower(strings.TrimSpace(a.chrome.customAccent))
		normalized := NormalizePreferences(Preferences{Accent: value})
		if len(value) == 7 && normalized.Accent == value {
			p.Accent = value
			a.savePreferences(c)
		} else {
			c.AddToast(ui.Toast{Title: "Format warna belum sesuai", Description: "Gunakan kode enam digit, misalnya #a8c2ac.", Type: "error"})
		}
	}
	a.boardColorChoices(c)
	ui.Text(c, "Tata letak").Bold()
	if spaceButton(c, "Tampilan ringkas", "grid", p.Compact).FillWidth().Clicked() {
		p.Compact = !p.Compact
		a.savePreferences(c)
	}
	if a.chrome.preferencesErr != "" {
		ui.Text(c, a.chrome.preferencesErr).FontSize(12).TextColor(t.Warning)
	}
}
