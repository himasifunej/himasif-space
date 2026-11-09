package appui

import (
	"os"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// A lock past its expiry is no lock: nobody "sedang mencatat", and taking
// the editor needs no takeover confirmation (0.3.4 showed "Demo Mediatek
// sedang mencatat · sampai 16.40" at 18.14).
func TestNoteExpiredLockShowsUnlocked(t *testing.T) {
	h, f := c2Harness(t, false)
	h.app.notes.note.Permissions.Manage = true
	h.tt.SetSize(1440, 900)
	h.app.router.Push(PathSpaces)
	h.app.chrome.tab = "Catatan"
	expired := time.Now().Add(-90 * time.Minute)
	h.app.notes.note.Lock = &api.NoteLock{Editor: api.PersonRef{ID: id.New(), DisplayName: "Demo Mediatek"}, ExpiresAt: expired}
	h.tt.Frame()
	h.dontWant("Demo Mediatek sedang mencatat")
	h.want("Belum ada pencatat aktif")
	h.want("kunci Demo Mediatek berakhir " + noteZoneClock(expired, h.app.noteViewZone()))
	h.want("Ambil editor")
	h.dontWant("Ambil alih editor")
	if os.Getenv("SPACE_UI_CAPTURE_DIR") != "" {
		h.app.chrome.preferences.Theme = "dark"
		h.tt.Frame()
		h.tt.Advance(400 * time.Millisecond)
		h.tt.Frame()
		captureNativeUI(t, h, "catatan-035-kunci-kedaluwarsa")
	}
	if editor := h.app.noteLiveEditor(time.Now()); editor != "" {
		t.Fatalf("live editor of an expired lock: %q", editor)
	}

	// A live lock of someone else still reads as such.
	h.app.notes.note.Lock.ExpiresAt = time.Now().Add(5 * time.Minute)
	h.tt.Frame()
	h.want("Demo Mediatek sedang mencatat")
	h.want("Ambil alih editor")

	// The caller's own expired lock.
	h.app.notes.note.Lock = &api.NoteLock{Editor: api.PersonRef{ID: h.app.user.ID, DisplayName: "Saya"}, HeldByCaller: true, ExpiresAt: expired}
	h.tt.Frame()
	h.dontWant("Kamu memegang editor")
	h.want("Belum ada pencatat aktif")
	_ = f
}
