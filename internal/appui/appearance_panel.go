package appui

import (
	"fmt"
	"slices"
	"strings"
	"time"

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
		popPanel(panel, 0)
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
	allAccents   = []string{"sand", "sage", "blue", "lavender", "rose", "pink"}
)

// accentNames name the presets for people (tooltips).
var accentNames = map[string]string{"sand": "Pasir (bawaan)", "sage": "Sage", "blue": "Langit", "lavender": "Lavender", "rose": "Tanah liat", "pink": "Merah muda"}

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

// appearanceSettings is Tampilan, option B (the owner's pick): the
// settings as rows at the left, and at the right a preview of the sidebar
// and the board in the colors chosen. Theme, palette and accent change the
// whole app as they are picked; the preview shows the board's colors.
func (a *App) appearanceSettings(c *ui.Context) {
	t := c.Theme()
	p := &a.chrome.preferences
	settingsPageView(c, "Tampilan", "Disimpan di perangkat ini; tidak ikut ke perangkat lain.", true, func() { SettingsSaved(c, time.Time{}) }, func() {
		w, _ := c.Size()
		controls := func() {
			ui.Column(c.Key("appearance-rows")).Grow(1).MinWidth(0).Gap(28).Children(func() {
				SettingSection(c, "Tema", "", func() {
					SettingRow(c, "Mode", "Ikuti sistem mengikuti mode gelap/terang Windows atau browser.", func() {
						modes := []string{"dark", "light", "system"}
						if i := segmented(c, "Mode tema", []string{"Gelap", "Terang", "Ikuti sistem"}, slices.Index(modes, p.Theme)); i >= 0 {
							p.Theme = modes[i]
							a.savePreferences(c)
						}
					})
					SettingRow(c, "Palet", "Warna dasar permukaan dan teks.", func() {})
					settingsRowBody(c, "palettes", func() { a.paletteRow(c) })
					SettingRow(c, "Warna aksen", "Tombol utama, pilihan, dan penanda.", func() { a.accentDots(c, 28, allAccents...) })
					SettingRow(c, "Kode warna", "Enam digit, misalnya #a8c2ac.", func() {
						ui.TextInput(c.Key("accent-custom"), &a.chrome.customAccent).Label("Warna aksen khusus").Placeholder("#d7c3a0").Width(110)
						if SettingButton(c, "Terapkan warna", "palette").Clicked() {
							a.applyCustomAccent(c)
						}
					})
					a.displayScaleRow(c)
					if SettingSwitch(c, "Tampilan ringkas", "Jarak dan tinggi baris lebih rapat di semua halaman.", &p.Compact) {
						a.savePreferences(c)
					}
				})
				SettingSection(c, "Board tugas", "Warna lajur dan kartu di board.", func() {
					SettingRow(c, "Warna lajur", "Klik warna untuk mengganti.", func() {})
					settingsRowBody(c, "lanes", func() {
						ui.Row(c).FillWidth().Gap(12).Wrap().Children(func() {
							for lane := workspaceui.Todo; lane <= workspaceui.Done; lane++ {
								ui.Row(c.Key(lane)).Gap(8).AlignItems(ui.Center).Children(func() {
									chosen := ui.Hex(laneColor(*p, lane))
									if ui.ColorWell(c, &chosen).Label("Warna lajur " + lane.String()).Width(44).Height(30).Changed() {
										p.LaneColors[lane] = fmt.Sprintf("#%02x%02x%02x", chosen.R, chosen.G, chosen.B)
										a.savePreferences(c)
									}
									ui.Text(c, lane.String()).FontSize(13)
								})
							}
						})
					})
					SettingRow(c, "Warna kartu", "Dasar warna setiap kartu.", func() { a.cardColorChoices(c) })
					SettingRow(c, "Kembalikan warna lajur", "", func() {
						if SettingButton(c, "Reset warna lajur", "refresh").Clicked() {
							p.LaneColors = [5]string{}
							a.savePreferences(c)
						}
					})
				})
				if a.chrome.preferencesErr != "" {
					ui.Text(c, a.chrome.preferencesErr).FontSize(12).TextColor(t.Warning)
				}
			})
		}
		// Beside the rows where there is room; above them otherwise.
		if w >= 1280 {
			ui.Row(c.Key("appearance-panes")).FillWidth().Gap(24).AlignItems(ui.Start).Children(func() {
				controls()
				ui.Column(c).Width(380).Shrink(0).Children(func() { a.appearancePreview(c) })
			})
			return
		}
		a.appearancePreview(c)
		controls()
	})
}

// laneColor is a lane's color: the device's choice, else the default.
func laneColor(p Preferences, lane workspaceui.Lane) string {
	if color := p.LaneColors[lane]; color != "" {
		return color
	}
	return []string{"#aaa294", "#a2bbd2", "#d7c3a0", "#a8c2ac"}[lane]
}

// applyCustomAccent applies the typed accent, or says it is not a color.
func (a *App) applyCustomAccent(c *ui.Context) {
	value := strings.ToLower(strings.TrimSpace(a.chrome.customAccent))
	normalized := NormalizePreferences(Preferences{Accent: value})
	if len(value) == 7 && normalized.Accent == value {
		a.chrome.preferences.Accent = value
		a.savePreferences(c)
		return
	}
	c.AddToast(ui.Toast{Title: "Format warna belum sesuai", Description: "Gunakan kode enam digit, misalnya #a8c2ac.", Type: "error"})
}

// paletteRow is the palettes in one row of cards.
func (a *App) paletteRow(c *ui.Context) {
	p := &a.chrome.preferences
	t := c.Theme()
	ui.Row(c.Key("palette-row")).FillWidth().Gap(8).Children(func() {
		for _, palette := range spacePalettes {
			active := p.Palette == palette.key || p.Palette == "" && palette.key == "champagne"
			b := ui.ButtonBase(c.Key(palette.key)).Label("Palet "+palette.name).Column().Grow(1).Basis(0).MinWidth(0).Padding(8).Gap(6).Radius(10).Border(1, t.Border)
			if active {
				b.BorderColor(t.Accent)
			} else if b.Hovered() {
				b.Background(t.SurfaceHover)
			}
			b.FocusRing(b.FocusVisible()).Children(func() {
				colors := palette.dark
				if !t.Dark {
					colors = palette.light
				}
				ui.Row(c).Height(34).FillWidth().Radius(5).Background(ui.Hex(colors[0])).Padding(5).Gap(4).Children(func() {
					ui.Box(c).Width(16).FillHeight().Radius(3).Background(ui.Hex(colors[3]))
					ui.Column(c).Grow(1).FillHeight().Gap(3).Children(func() {
						ui.Box(c).Height(3).Width(20).Radius(2).Background(ui.Hex(palette.accent))
						ui.Box(c).Grow(1).FillWidth().Radius(3).Background(ui.Hex(colors[2]))
					})
				})
				ui.Text(c, palette.name).FontSize(12).MaxLines(1)
			})
			if b.Clicked() {
				p.Palette = palette.key
				p.Accent = paletteAccent(palette.key, p.Accent)
				a.savePreferences(c)
			}
		}
	})
}

// paletteAccent is the accent after picking a palette: Sakura brings its
// pink while the accent is still the default sand; a chosen one stays.
func paletteAccent(palette, accent string) string {
	if palette == "sakura" && (accent == "" || accent == "sand") {
		return "pink"
	}
	return accent
}

// appearancePreview draws a small Space: the sidebar, the four lanes in
// their colors with two cards each, colored as the board's cards are, and
// a primary button in the accent.
func (a *App) appearancePreview(c *ui.Context) {
	t := c.Theme()
	p := a.chrome.preferences
	ui.Column(c.Key("appearance-preview")).FillWidth().Padding(14).Gap(10).Radius(12).Background(t.Surface).Border(1, t.Border).Label("Pratinjau tampilan").Children(func() {
		muted(c, "Pratinjau").FontSize(12)
		ui.Row(c).FillWidth().Height(280).Radius(10).Background(t.Background).Border(1, t.Border).Clip().Children(func() {
			ui.Column(c).Width(64).FillHeight().Padding(10).Gap(8).Background(t.Surface).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
				ui.Box(c).Height(18).FillWidth().Radius(5).Background(t.Accent)
				for i := 0; i < 5; i++ {
					bg := t.SurfaceHover
					if i == 1 {
						bg = accentSoft(t)
					}
					ui.Box(c.Key(i)).Height(8).FillWidth().Radius(3).Background(bg)
				}
			})
			ui.Row(c).Grow(1).MinWidth(0).FillHeight().Padding(10).Gap(6).Children(func() {
				samples := [][2]string{{"Desain", "Ani"}, {"Konten", "Bima"}}
				for lane := workspaceui.Todo; lane <= workspaceui.Done; lane++ {
					ui.Column(c.Key(lane)).Grow(1).Basis(0).MinWidth(0).Gap(6).Children(func() {
						ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
							ui.Box(c).Size(6, 6).Radius(3).Background(ui.Hex(laneColor(p, lane)))
							ui.Text(c, lane.String()).FontSize(9).TextColor(t.TextMuted).MaxLines(1)
						})
						for i, sample := range samples {
							stripe := a.taskCardColor(sample[0], sample[1], lane)
							card := ui.Row(c.Key(i)).FillWidth().Height(40).Radius(6).Background(t.Surface).Border(1, t.Border)
							card.Children(func() {
								if stripe != (ui.Color{}) {
									ui.Box(c).Width(3).FillHeight().Radius(2).Background(stripe)
								}
							})
						}
					})
				}
			})
		})
		ui.Row(c).Gap(8).Children(func() {
			primarySpaceButton(c, "Buat tugas", "plus").Label("Contoh tombol utama")
			spaceButton(c, "Batal", "", false).Label("Contoh tombol biasa")
		})
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
