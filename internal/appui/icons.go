package appui

import (
	_ "embed"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Embedded identity assets are unchanged copies of the approved prototype.
// See assets/SOURCES.md for their provenance.
//
//go:embed assets/space.svg
var spaceBrandData []byte

//go:embed assets/mediatek.png
var mediatekLogoData []byte

//go:embed assets/humas.png
var humasLogoData []byte

var spaceBrandSVG = ui.MustParseSVG(spaceBrandData)

// SVGs and bitmaps are parsed once, before the first frame.
var spaceIcons = func() map[string]*ui.SVG {
	paths := map[string]string{
		"alert":    `<path d="m10.3 4-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.7-3l-8-14a2 2 0 0 0-3.4 0Z"/><path d="M12 9v4m0 4v.1"/>`,
		"archive":  `<rect x="3" y="3" width="18" height="5" rx="1"/><path d="M5 8v12h14V8m-10 4h6"/>`,
		"arrow":    `<path d="M4 12h16m-6-6 6 6-6 6"/>`,
		"bell":     `<path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M10 21h4"/>`,
		"board":    `<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M9 4v16M15 4v16M5 8h2m4 0h2m4 0h2M5 12h2m4 0h2"/>`,
		"calendar": `<rect x="3" y="5" width="18" height="16" rx="3"/><path d="M16 3v4M8 3v4M3 11h18m-12 4h2m3 0h2m-7 3h2"/>`,
		"chat":     `<path d="M21 12a8 8 0 0 1-8 8H4l-2 2V10a8 8 0 0 1 8-8h3a8 8 0 0 1 8 8Z"/><path d="M7 8h9M7 12h7"/>`,
		"check":    `<path d="m5 12 4 4L19 6"/>`,
		"chevron":  `<path d="m9 5 7 7-7 7"/>`,
		"clock":    `<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>`,
		"close":    `<path d="m6 6 12 12M6 18 18 6"/>`,
		"down":     `<path d="m6 9 6 6 6-6"/>`,
		"edit":     `<path d="m15 4 5 5M4 20l4-1L21 6l-5-5L3 14Z"/>`,
		"filter":   `<path d="M4 7h16M7 12h10M10 17h4"/><circle cx="8" cy="7" r="2" fill="currentColor" stroke="none"/><circle cx="15" cy="12" r="2" fill="currentColor" stroke="none"/>`,
		"folder":   `<path d="M3 6a2 2 0 0 1 2-2h5l2 3h7a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z"/>`,
		"globe":    `<circle cx="12" cy="12" r="9"/><ellipse cx="12" cy="12" rx="4" ry="9"/><path d="M3 12h18"/>`,
		"grid":     `<rect x="3" y="3" width="7" height="7" rx="2"/><rect x="14" y="3" width="7" height="7" rx="2"/><rect x="3" y="14" width="7" height="7" rx="2"/><rect x="14" y="14" width="7" height="7" rx="2"/>`,
		"grip":     `<circle cx="9" cy="5" r=".8"/><circle cx="15" cy="5" r=".8"/><circle cx="9" cy="12" r=".8"/><circle cx="15" cy="12" r=".8"/><circle cx="9" cy="19" r=".8"/><circle cx="15" cy="19" r=".8"/>`,
		"home":     `<path d="m3 10 9-7 9 7v10a1 1 0 0 1-1 1h-5v-7H9v7H4a1 1 0 0 1-1-1Z"/>`,
		"info":     `<circle cx="12" cy="12" r="9"/><path d="M12 11v6m0-10v.1"/>`,
		"keyboard": `<rect x="2" y="5" width="20" height="14" rx="2"/><path d="M6 9h1m3 0h1m3 0h1m3 0h1M6 13h1m3 0h1m3 0h1m3 0h1M7 16h10"/>`,
		"link":     `<path d="m10 13 4-4m-5 7-2 2a4 4 0 0 1-6-6l5-5a4 4 0 0 1 6 0m0 10a4 4 0 0 0 6 0l5-5a4 4 0 0 0-6-6l-2 2" transform="translate(1 0) scale(.9)"/>`,
		"lock":     `<rect x="5" y="10" width="14" height="11" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3m-4 5v2"/>`,
		"logout":   `<path d="M10 4H4v16h6m5-12 4 4-4 4m-7-4h11"/>`,
		"message":  `<path d="M21 11.5a8.5 8.5 0 0 1-8.5 8.5H4l-2 2V11.5A8.5 8.5 0 0 1 10.5 3h2a8.5 8.5 0 0 1 8.5 8.5Z"/><path d="M7 9h10M7 13h6"/>`,
		"monitor":  `<rect x="3" y="4" width="18" height="13" rx="2"/><path d="M8 21h8m-4-4v4"/>`,
		"moon":     `<path d="M20.5 13A8.5 8.5 0 0 1 11 3.5 8.5 8.5 0 1 0 20.5 13Z"/>`,
		"more":     `<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>`,
		"note":     `<path d="M14 3H5a1 1 0 0 0-1 1v16a1 1 0 0 0 1 1h14a1 1 0 0 0 1-1V9Z"/><path d="M14 3v6h6M8 13h8M8 17h5"/>`,
		"palette":  `<path d="M12 3a9 9 0 1 0 0 18h1a2 2 0 0 0 2-2c0-1-1-1.5-1-2.5s1-1.5 2-1.5h2a3 3 0 0 0 3-3 9 9 0 0 0-9-9Z"/><circle cx="7" cy="10" r=".7"/><circle cx="10" cy="7" r=".7"/><circle cx="15" cy="7" r=".7"/><circle cx="18" cy="10" r=".7"/>`,
		"panel":    `<rect x="3" y="4" width="18" height="16" rx="3"/><path d="M9 4v16m-3-9 2 1-2 1"/>`,
		"plus":     `<path d="M12 5v14M5 12h14"/>`,
		"qr":       `<path d="M3 3h6v6H3zm12 0h6v6h-6zM3 15h6v6H3zm12 0h2v2h-2zm4 4h2v2h-2zM12 3v3m0 5v3m-9-2h4m5 6v3m6-9h3m-3 3h3m-6 3v3"/>`,
		"refresh":  `<path d="M20 7v5h-5M4 17v-5h5M6.1 6a7 7 0 0 1 11.5 1L20 12M4 12l2.4 5A7 7 0 0 0 18 18"/>`,
		"search":   `<circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 5 5"/>`,
		"send":     `<path d="m22 2-7 20-4-9L2 9Z M11 13 22 2"/>`,
		"settings": `<path d="M9.671 4.136a2.34 2.34 0 0 1 4.659 0 2.34 2.34 0 0 0 3.319 1.915 2.34 2.34 0 0 1 2.33 4.033 2.34 2.34 0 0 0 0 3.831 2.34 2.34 0 0 1-2.33 4.033 2.34 2.34 0 0 0-3.319 1.915 2.34 2.34 0 0 1-4.659 0 2.34 2.34 0 0 0-3.32-1.915 2.34 2.34 0 0 1-2.33-4.033 2.34 2.34 0 0 0 0-3.831A2.34 2.34 0 0 1 6.35 6.051a2.34 2.34 0 0 0 3.319-1.915"/><circle cx="12" cy="12" r="3"/>`,
		"shield":   `<path d="m12 3 8 3v6c0 5-8 9-8 9s-8-4-8-9V6Z"/><path d="m8 12 3 3 5-6"/>`,
		"spark":    `<path d="m12 3 2.5 6.5L21 12l-6.5 2.5L12 21l-2.5-6.5L3 12l6.5-2.5Z"/>`,
		"sticker":  `<path d="M4 3h16v11l-7 7H4Z"/><path d="M13 21v-7h7M8 8h.1m6 0h.1m-6 4c2 2 4 2 6 0"/>`,
		"sun":      `<circle cx="12" cy="12" r="4"/><path d="M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.5 1.5m11 11L19 19M5 19l1.5-1.5m11-11L19 5"/>`,
		"task":     `<rect x="4" y="4" width="16" height="17" rx="3"/><path d="M9 3h6v4H9zM8 12l2 2 5-5M8 18h8"/>`,
		"terminal": `<rect x="3" y="4" width="18" height="16" rx="3"/><path d="m7 9 3 3-3 3m6 0h4"/>`,
		"unlink":   `<path d="m3 3 18 18M8 11l-3 3a4 4 0 0 0 6 6l3-3m2-4 3-3a4 4 0 0 0-6-6l-3 3M9 15l6-6"/>`,
		"upload":   `<path d="M12 16V3m-5 5 5-5 5 5M4 16v4h16v-4"/>`,
		"user":     `<circle cx="12" cy="8" r="4"/><path d="M4 21v-2a8 8 0 0 1 16 0v2"/>`,
		"users":    `<circle cx="9" cy="8" r="3"/><path d="M3 21v-3a6 6 0 0 1 12 0v3m1-16a3 3 0 0 1 0 6m3 10v-3a6 6 0 0 0-3-5"/>`,
		"whatsapp": `<path d="M3 21l1.65 -3.8a9 9 0 1 1 3.4 2.9l-5.05 .9"/><path d="M9 10a.5 .5 0 0 0 1 0v-1a.5 .5 0 0 0 -1 0v1a5 5 0 0 0 5 5h1a.5 .5 0 0 0 0 -1h-1a.5 .5 0 0 0 0 1"/>`,
		"wifi":     `<path d="M2 8a16 16 0 0 1 20 0M5 12a11 11 0 0 1 14 0m-11 4a6 6 0 0 1 8 0"/><circle cx="12" cy="20" r=".8"/>`,
	}
	icons := make(map[string]*ui.SVG, len(paths))
	for name, path := range paths {
		icons[name] = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round">` + path + `</svg>`))
	}
	return icons
}()

var spaceDivisionLogos = map[string]*ui.Bitmap{
	"mediatek": mustDivisionBitmap(mediatekLogoData),
	"humas":    mustDivisionBitmap(humasLogoData),
}

func mustDivisionBitmap(data []byte) *ui.Bitmap {
	bitmap, err := ui.DecodeBitmap(data)
	if err != nil {
		panic("appui: invalid embedded division logo: " + err.Error())
	}
	return bitmap
}

// spaceIcon returns a shared monochrome icon, tinted by ui.Icon.
func spaceIcon(name string) *ui.SVG {
	if icon, ok := spaceIcons[strings.ToLower(strings.TrimSpace(name))]; ok {
		return icon
	}
	return spaceIcons["grid"]
}

// spaceBrand must be displayed with ui.Image to preserve its identity colors.
func spaceBrand() *ui.SVG { return spaceBrandSVG }

// divisionLogo returns nil for unknown divisions, including shared Medhum.
// The caller can then render a neutral mark rather than another division's logo.
// Use ui.Image without tinting to retain the original artwork.
func divisionLogo(slug string) *ui.Bitmap {
	return spaceDivisionLogos[strings.ToLower(strings.TrimSpace(slug))]
}
