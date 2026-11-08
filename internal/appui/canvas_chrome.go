package appui

import (
	"hash/fnv"
	"sort"
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
)

// Pieces of canvas A's chrome (sidebar, topbar, appearance popover) shared
// by the desktop shell. Sizes follow the canvas' CSS: 32 px navigation rows,
// 11 px section labels, 26 px avatars, Plex Mono key badges.

// canvasTileColors are the canvas' Space tiles and avatars, light enough for
// the dark ink drawn on them in both themes.
var canvasTileColors = []string{"#dfcba8", "#a9c4dd", "#b8c9a3", "#e3a383", "#cdb8e0"}

// canvasInk is the text drawn on a tile or avatar.
var canvasInk = ui.Hex("#24221c")

// faint is the canvas' third text color, between muted text and the page.
func faint(t *ui.Theme) ui.Color { return spaceMix(t.TextMuted, t.Background, .32) }

// accentSoft is the canvas' selected row: the accent at 12 %.
func accentSoft(t *ui.Theme) ui.Color { return ui.RGBA(t.Accent.R, t.Accent.G, t.Accent.B, .12) }

// personColor gives a person the same avatar color everywhere.
func personColor(name string) ui.Color {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(strings.TrimSpace(name))))
	return ui.Hex(canvasTileColors[int(h.Sum32()%uint32(len(canvasTileColors)))])
}

// avatar draws a round avatar with a person's initials.
func avatar(c *ui.Context, name string, size float32) ui.Element {
	return ui.Box(c).Size(size, size).Shrink(0).Radius(size / 2).Center().Background(personColor(name)).Children(func() {
		ui.Text(c, personInitials(name)).FontSize(max(9, size*.38)).FontWeight(600).TextColor(canvasInk)
	})
}

// spaceInitials is a Space's tile text: "M" for Mediatek and, for a Space
// shared by divisions, one letter per division in the order the name uses
// them, so Medhum (Mediatek and Humas) is "MH".
func spaceInitials(s api.Space) string {
	if s.Kind == api.SpaceShared && len(s.Divisions) > 1 {
		name := strings.ToLower(s.Name)
		type part struct {
			at     int
			letter string
		}
		var parts []part
		for _, d := range s.Divisions {
			letter := personInitials(d.Name)
			if letter == "" {
				continue
			}
			prefix := []rune(strings.ToLower(d.Name))
			at := len(name) + len(parts)
			if len(prefix) >= 3 {
				if i := strings.Index(name, string(prefix[:3])); i >= 0 {
					at = i
				}
			}
			parts = append(parts, part{at, string([]rune(letter)[:1])})
		}
		sort.SliceStable(parts, func(i, j int) bool { return parts[i].at < parts[j].at })
		out := ""
		for i, p := range parts {
			if i == 2 {
				break
			}
			out += p.letter
		}
		if out != "" {
			return out
		}
	}
	for _, r := range s.Name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return string(unicode.ToUpper(r))
		}
	}
	return "S"
}

// spaceTile is the colored letter square of a Space in the sidebar.
func spaceTile(c *ui.Context, s api.Space, index int) ui.Element {
	return ui.Box(c).Size(20, 20).Shrink(0).Radius(6).Center().Background(ui.Hex(canvasTileColors[index%len(canvasTileColors)])).Children(func() {
		ui.Text(c, spaceInitials(s)).FontSize(10).FontWeight(600).TextColor(canvasInk)
	})
}

// kbd is a key badge, as "Ctrl B".
func kbd(c *ui.Context, keys string) ui.Element {
	t := c.Theme()
	return ui.Text(c, keys).Font(FontMono).FontSize(11).TextColor(faint(t)).Padding(1, 5).Radius(5).Border(1, t.Border).Shrink(0)
}

// sectionLabel is a sidebar section heading, as "PRIBADI".
func sectionLabel(c *ui.Context, label string) ui.Element {
	return ui.Text(c, strings.ToUpper(label)).FontSize(11).TextColor(faint(c.Theme())).Padding(10, 10, 3, 10).MaxLines(1)
}

// navRow is a sidebar row: an icon or tile, the label and, at the end, a
// count. alert colors the count as a warning. Collapsed, the row keeps its
// icon alone and names itself in a tooltip.
func navRow(c *ui.Context, label string, active, collapsed bool, lead func(), count string, alert bool, trail func()) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Tooltip(label).FillWidth().Row().AlignItems(ui.Center).Gap(10).Height(32).Shrink(0).Padding(0, 10).Radius(8).TextColor(t.TextMuted)
	if collapsed {
		b.Justify(ui.Center).Padding(0)
	}
	if active {
		b.Background(accentSoft(t)).TextColor(t.Text)
	} else if b.Hovered() {
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.FocusRing(b.FocusVisible())
	b.Children(func() {
		lead()
		if collapsed {
			return
		}
		ui.Text(c, label).Grow(1).MinWidth(0).MaxLines(1)
		if count != "" {
			n := ui.Text(c, count).FontSize(11).TextColor(faint(t)).Shrink(0)
			if alert {
				n.TextColor(t.Danger)
			}
		}
		if trail != nil {
			trail()
		}
	})
	return b
}

// chromeIcons are canvas A's sidebar and topbar icons, from its inline SVGs
// (stroke 1.7, round caps). Others come from spaceIcon.
var chromeIcons = func() map[string]*ui.SVG {
	paths := map[string]string{
		"jobdesk":  `<path d="M9 11l3 3 8-8"/><path d="M20 12v7a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h9"/>`,
		"reminder": `<path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/><path d="M10 21h4"/>`,
		"sop":      `<path d="M4 19V5M4 19h16"/><path d="M8 15l3-4 3 2 4-6"/>`,
		"arsip":    `<rect x="3" y="4" width="18" height="5" rx="1"/><path d="M5 9v10h14V9M10 13h4"/>`,
		"settings": `<circle cx="12" cy="12" r="3"/><path d="M19 12a7 7 0 0 0-.1-1.3l2-1.5-2-3.4-2.3 1a7 7 0 0 0-2.3-1.3L14 3h-4l-.3 2.5a7 7 0 0 0-2.3 1.3l-2.3-1-2 3.4 2 1.5A7 7 0 0 0 5 12c0 .4 0 .9.1 1.3l-2 1.5 2 3.4 2.3-1a7 7 0 0 0 2.3 1.3L10 21h4l.3-2.5a7 7 0 0 0 2.3-1.3l2.3 1 2-3.4-2-1.5c.1-.4.1-.9.1-1.3Z"/>`,
		"tampilan": `<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/>`,
		"collapse": `<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M9 4v16"/>`,
		"period":   `<rect x="3" y="5" width="18" height="16" rx="2"/><path d="M3 10h18M8 3v4M16 3v4"/>`,
		"search":   `<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>`,
		"bell":     `<path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/><path d="M10 21h4"/>`,
		"ping":     `<path d="M3 11v2a1 1 0 0 0 1 1h2l5 4V6L6 10H4a1 1 0 0 0-1 1Z"/><path d="M15 9a4 4 0 0 1 0 6M18 6a8 8 0 0 1 0 12"/>`,
		"plus":     `<path d="M12 5v14M5 12h14"/>`,
		"close":    `<path d="M6 6l12 12M18 6 6 18"/>`,
		"down":     `<path d="m6 9 6 6 6-6"/>`,
	}
	icons := make(map[string]*ui.SVG, len(paths))
	for name, path := range paths {
		icons[name] = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">` + path + `</svg>`))
	}
	return icons
}()

func chromeIcon(name string) *ui.SVG {
	if icon := chromeIcons[name]; icon != nil {
		return icon
	}
	return spaceIcon(name)
}

// iconLead draws a navigation row's 16 px icon.
func iconLead(c *ui.Context, icon string) func() {
	return func() { ui.Icon(c, chromeIcon(icon)).Size(16, 16).Shrink(0) }
}
