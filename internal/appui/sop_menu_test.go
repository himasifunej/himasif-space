package appui

import (
	"testing"

	"github.com/himasifunej/himasif-space/internal/id"
)

// navHas tells whether quick navigation offers the page at path.
func navHas(a *App, path string) bool {
	for _, it := range a.navIndex() {
		if it.key == "page"+path {
			return true
		}
	}
	return false
}

func settingsHas(a *App, key string) bool {
	for _, g := range a.settingsGroups() {
		for _, e := range g.entries {
			if e.key == key {
				return true
			}
		}
	}
	return false
}

// Evaluasi SOP stays hidden while no division has an active SOP policy
// (PRD EVA-11), shows once one has, and whoever may configure a policy
// finds it in Pengaturan either way.
func TestSOPMenuHiddenUntilPolicyActive(t *testing.T) {
	h := newChromeHarness(t, nil)
	a := h.app
	if a.period() == nil {
		t.Fatal("harness without a period")
	}
	humas, mediatek := id.New(), id.New()
	a.sopMenu = sopMenuState{user: a.user.ID, period: a.periodID, loaded: true,
		enabled: map[id.ID]bool{humas: false, mediatek: false}}
	h.tt.Frame()
	if a.sopMenuVisible() {
		t.Fatal("menu visible without an active policy")
	}
	h.dontWant("Evaluasi SOP")
	if navHas(a, PathEvaluasi) {
		t.Error("quick navigation offers Evaluasi SOP without an active policy")
	}
	if settingsHas(a, "Evaluasi SOP") {
		t.Error("Pengaturan lists Evaluasi SOP to one who cannot configure it")
	}

	// A kadiv who may configure finds it in Pengaturan, still not in the menus.
	a.sopMenu.configure = true
	h.tt.Frame()
	if !settingsHas(a, "Evaluasi SOP") {
		t.Error("Pengaturan hides Evaluasi SOP from one who may configure it")
	}
	if navHas(a, PathEvaluasi) {
		t.Error("configuring alone shows the page in quick navigation")
	}

	// Turning a policy on (as Evaluasi SOP reports it) shows the menus.
	a.noteSOPPolicy(humas, true)
	h.tt.Frame()
	if !a.sopMenuVisible() || !navHas(a, PathEvaluasi) {
		t.Fatal("menu hidden although a policy is active")
	}
	h.want("Evaluasi SOP")

	// And turning it off hides them again.
	a.noteSOPPolicy(humas, false)
	h.tt.Frame()
	if a.sopMenuVisible() {
		t.Fatal("menu visible after the policy was turned off")
	}
}

// The state belongs to one account and period: another period starts
// hidden again, and a policy noted for another period is ignored.
func TestSOPMenuFollowsPeriod(t *testing.T) {
	h := newChromeHarness(t, nil)
	a := h.app
	other := id.New()
	a.sopMenu = sopMenuState{user: a.user.ID, period: other, loaded: true, enabled: map[id.ID]bool{id.New(): true}}
	if a.sopMenuVisible() {
		t.Fatal("another period's active policy shows the menu")
	}
	if a.sopMenu.period != a.periodID {
		t.Fatalf("state not reset for the current period: %v", a.sopMenu.period)
	}
	a.sopMenu.period = other
	a.noteSOPPolicy(id.New(), true)
	if len(a.sopMenu.enabled) != 0 {
		t.Fatal("a policy was noted for a period that is not shown")
	}
}
