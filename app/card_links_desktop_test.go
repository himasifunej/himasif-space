//go:build !js

package main

import (
	"testing"

	"github.com/himasifunej/himasif-space/internal/id"
)

// The web's card links open the card: space://tugas/<id> and
// space://rapat/<id>, and himasif-space:// alike; anything else is not a
// card.
func TestCardLinkTarget(t *testing.T) {
	cid := id.New()
	for raw, want := range map[string]string{"space://tugas/" + cid.String(): "task", "space://rapat/" + cid.String(): "meeting", "himasif-space://tugas/" + cid.String() + "/": "task"} {
		kind, got, ok := cardLinkTarget(raw)
		if !ok || kind != want || got != cid {
			t.Errorf("%s: %q %v %v", raw, kind, got, ok)
		}
	}
	for _, raw := range []string{"space://tugas/bukan-id", "space://catatan/" + cid.String(), "https://tugas/" + cid.String(), "himasif-space://notification?x=1"} {
		if _, _, ok := cardLinkTarget(raw); ok {
			t.Errorf("%s read as a card", raw)
		}
	}
}
