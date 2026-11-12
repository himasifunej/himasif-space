package appui

import "strings"

// Preferences contains device-local appearance choices only. Session tokens and
// other credentials must stay in the separate TokenStore.
type Preferences struct {
	Theme            string    `json:"theme"`
	Accent           string    `json:"accent"`
	Compact          bool      `json:"compact"`
	SidebarCollapsed bool      `json:"sidebarCollapsed"`
	Palette          string    `json:"palette,omitempty"`
	LaneColors       [5]string `json:"laneColors,omitempty"`
	ColorCardsByTag  bool      `json:"colorCardsByTag,omitempty"`
	// CardColor colors board cards by "" (their label), "status", "pj" or "plain".
	CardColor string `json:"cardColor,omitempty"`
	// MeetingListWidth is the Rapat page's list, dragged to this width in
	// DIP; 0 the layout's own. From meetingListNarrow it shows the dates
	// only.
	MeetingListWidth float32 `json:"meetingListWidth,omitempty"`
	// SpaceTabs keeps each account's Space tab bar, by "user/space" ID. It
	// is a pointer, replaced on every change, so Preferences stays
	// comparable.
	SpaceTabs *SpaceTabLayouts `json:"spaceTabs,omitempty"`
}

type PreferencesStore interface {
	Load() (Preferences, error)
	Save(Preferences) error
}

func DefaultPreferences() Preferences {
	return Preferences{Theme: "dark", Accent: "sand"}
}

// NormalizePreferences accepts only supported themes and accent values. It is
// safe to call for imported, partially populated or older preference records.
func NormalizePreferences(p Preferences) Preferences {
	p.MeetingListWidth = meetingListWidth(p.MeetingListWidth)
	p.Palette = strings.ToLower(strings.TrimSpace(p.Palette))
	switch p.Palette {
	case "", "champagne", "obsidian", "midnight", "copper", "graphite", "sakura":
	default:
		p.Palette = ""
	}
	for i, color := range p.LaneColors {
		if color != "" {
			if validHex(color) {
				p.LaneColors[i] = strings.ToLower(strings.TrimSpace(color))
			} else {
				p.LaneColors[i] = ""
			}
		}
	}
	p.Theme = strings.ToLower(strings.TrimSpace(p.Theme))
	if p.Theme != "light" && p.Theme != "dark" && p.Theme != "system" {
		p.Theme = "dark"
	}
	p.Accent = strings.ToLower(strings.TrimSpace(p.Accent))
	switch p.Accent {
	case "sand", "sage", "blue", "lavender", "rose", "pink":
		return p
	}
	if len(p.Accent) == 7 && p.Accent[0] == '#' {
		valid := true
		for _, digit := range p.Accent[1:] {
			if !(digit >= '0' && digit <= '9' || digit >= 'a' && digit <= 'f') {
				valid = false
				break
			}
		}
		if valid {
			return p
		}
	}
	p.Accent = "sand"
	return p
}

func validHex(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, r := range value[1:] {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
