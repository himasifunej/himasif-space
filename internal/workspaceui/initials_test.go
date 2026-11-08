package workspaceui

import "testing"

// Demo accounts are named "[DUMMY] Aruna Desain"; their avatars showed "[A".
func TestInitialsSkipTagsAndSymbols(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"Demo Mediatek", "DM"},
		{"[DUMMY] Aruna Desain", "AD"},
		{"[DUMMY] Nara", "N"},
		{"  [DUMMY]  (uji) Raka Video Editor", "RV"},
		{"aruna", "A"},
		{"Ana - Budi", "AB"},
		{"#1 Fan", "1F"},
		{"élan vital", "ÉV"},
		{"[DUMMY]", "D"},
		{"[]", ""},
		{"", ""},
		{"[unclosed Raka", "UR"},
	} {
		if got := Initials(tc.name); got != tc.want {
			t.Errorf("Initials(%q) = %q; want %q", tc.name, got, tc.want)
		}
	}
}
