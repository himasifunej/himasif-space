package appui

import "testing"

func TestNormalizePreferences(t *testing.T) {
	tests := []struct {
		name string
		in   Preferences
		want Preferences
	}{
		{"defaults", Preferences{}, DefaultPreferences()},
		{"preset", Preferences{Theme: " LIGHT ", Accent: " SaGe ", Compact: true, SidebarCollapsed: true}, Preferences{Theme: "light", Accent: "sage", Compact: true, SidebarCollapsed: true}},
		{"system", Preferences{Theme: " SYSTEM ", Accent: "blue"}, Preferences{Theme: "system", Accent: "blue"}},
		{"custom", Preferences{Theme: "dark", Accent: " #A1B2C3 "}, Preferences{Theme: "dark", Accent: "#a1b2c3"}},
		{"invalid theme", Preferences{Theme: "unknown", Accent: "rose"}, Preferences{Theme: "dark", Accent: "rose"}},
		{"invalid hex", Preferences{Theme: "light", Accent: "#xyzxyz"}, Preferences{Theme: "light", Accent: "sand"}},
		{"short hex", Preferences{Theme: "dark", Accent: "#abc"}, DefaultPreferences()},
		{"arbitrary CSS", Preferences{Theme: "dark", Accent: "var(--accent)"}, DefaultPreferences()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NormalizePreferences(test.in); got != test.want {
				t.Fatalf("NormalizePreferences(%+v) = %+v; want %+v", test.in, got, test.want)
			}
		})
	}
}
