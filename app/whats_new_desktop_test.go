//go:build !js

package main

import "testing"

// "Yang baru" shows once after an update, never on a fresh install, on
// the version already seen, or after going back to an older one; the
// signed notes kept at install come before the bundled ones.
func TestWhatsNewOnceAfterAnUpdate(t *testing.T) {
	old := devicePreferences{SeenVersion: "0.4.4"}
	if wn := whatsNew(old, true, "0.4.5"); wn != nil {
		t.Fatal("shown on a fresh install")
	}
	wn := whatsNew(old, false, "0.4.5")
	if wn == nil || wn.Version != "0.4.5" || len(wn.Notes) == 0 {
		t.Fatalf("after 0.4.4: %+v", wn)
	}
	// 0.4.4 kept no SeenVersion: an existing file still means an update.
	if wn := whatsNew(devicePreferences{}, false, "0.4.5-rc2"); wn == nil {
		t.Fatal("not shown after updating from a version that kept none")
	}
	if wn := whatsNew(devicePreferences{SeenVersion: "0.4.5"}, false, "0.4.5"); wn != nil {
		t.Fatal("shown twice")
	}
	if wn := whatsNew(devicePreferences{SeenVersion: "0.4.6"}, false, "0.4.5"); wn != nil {
		t.Fatal("shown after a downgrade")
	}
	signed := devicePreferences{SeenVersion: "0.4.4", UpdateNotes: "• Satu\n• Dua", UpdateNotesV: "0.4.5"}
	if wn := whatsNew(signed, false, "0.4.5"); wn == nil || len(wn.Notes) != 2 || wn.Notes[1] != "Dua" {
		t.Fatalf("signed notes %+v", wn)
	}
	if wn := whatsNew(devicePreferences{SeenVersion: "0.4.4"}, false, "9.9.9"); wn != nil {
		t.Fatal("a version without notes shows an empty dialog")
	}
}
