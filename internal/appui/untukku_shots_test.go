package appui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// shot writes the window to SPACE_SHOTS/name.png, for comparing a page
// with its canvas; without SPACE_SHOTS it does nothing.
func shot(t *testing.T, h *harness, name string) {
	t.Helper()
	dir := os.Getenv("SPACE_SHOTS")
	if dir == "" {
		return
	}
	h.tt.Frame()
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, h.tt.Image()); err != nil {
		t.Fatal(err)
	}
}

func TestUntukkuShots(t *testing.T) {
	if os.Getenv("SPACE_SHOTS") == "" {
		t.Skip("SPACE_SHOTS not set")
	}
	h, _, _ := untukFixture(t)
	shot(t, h, "untukku")
}
