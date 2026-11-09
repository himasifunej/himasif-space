package appui

import (
	"testing"

	"github.com/himasifunej/himasif-space/internal/seed"
)

// Every division whose emblem is published by himasif.id must resolve from the
// slug the seeder actually writes, so the sidebar and header show the artwork
// rather than a generic mark.
func TestDivisionLogosCoverSeededSlugs(t *testing.T) {
	published := map[string]bool{
		"humas":    true,
		"kader":    true,
		"litbang":  true,
		"mediatek": true,
		"psdm":     true,
	}
	seen := map[string]bool{}
	for _, d := range seed.Divisions {
		logo := divisionLogo(d.Slug)
		if published[d.Slug] {
			if logo == nil {
				t.Errorf("division %q has no embedded logo", d.Slug)
			}
			seen[d.Slug] = true
			continue
		}
		if logo != nil {
			t.Errorf("division %q must not borrow another division's logo", d.Slug)
		}
	}
	for slug := range published {
		if !seen[slug] {
			t.Errorf("seeded divisions no longer include %q", slug)
		}
	}
}

func TestDivisionLogoMatchesSlugCaseAndSpace(t *testing.T) {
	if divisionLogo("  Mediatek ") == nil {
		t.Fatal("division logo lookup must ignore case and surrounding spaces")
	}
	if divisionLogo("") != nil || divisionLogo("medhum") != nil {
		t.Fatal("unknown divisions must not inherit a division logo")
	}
}
