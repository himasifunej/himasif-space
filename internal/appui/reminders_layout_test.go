package appui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func TestReminderInspectorKeepsSaveAndFooterReachable(t *testing.T) {
	h, f := reminderHarness(t)
	h.app.router.Replace(PathReminder)
	h.app.openReminder(f.items[0].ID)
	settleReminders(t, h)
	for _, size := range [][2]int{{1440, 900}, {880, 640}} {
		h.tt.SetSize(size[0], size[1])
		h.tt.Advance(400 * time.Millisecond)
		h.tt.Frame()
		for _, label := range []string{"Simpan reminder", "Tutup reminder", "Tersambung", "Menu profil"} {
			r, ok := h.tt.Find(label)
			if !ok || r.W <= 0 || r.H <= 0 || r.X < 0 || r.Y < 0 || r.X+r.W > float32(size[0])+1 || r.Y+r.H > float32(size[1])+1 {
				t.Fatalf("%dx%d: %s clipped: %+v exists=%v", size[0], size[1], label, r, ok)
			}
		}
		h.tt.Scroll(float32(size[0]-40), float32(size[1]-140), 0, 10000)
		h.tt.Advance(time.Second)
		h.tt.Frame()
		r, ok := h.tt.Find("Hapus reminder")
		if !ok || r.H <= 0 || r.Y < 85 || r.Y+r.H > float32(size[1]-28) {
			t.Fatalf("%dx%d: last owner action unreachable after scrolling: %+v exists=%v", size[0], size[1], r, ok)
		}
		captureReminder(t, h, "reminder-owner-"+map[int]string{1440: "large", 880: "minimum"}[size[0]])
	}
}

func TestReminderNativeFormAndReadOnlyRecipient(t *testing.T) {
	h, f := reminderHarness(t)
	h.tt.SetSize(1440, 900)
	h.app.router.Replace(PathReminder)
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	h.click("Buat reminder")
	h.tt.Frame()
	h.click("Judul reminder *")
	h.tt.Type("Ingat notulensi rapat")
	h.click("Tanggal reminder *")
	h.tt.Type("2026-11-02")
	h.click("Simpan reminder")
	settleReminders(t, h)
	h.tt.Frame()
	if len(f.createRequests) != 1 || f.createRequests[0].DueKind != api.DueDate || f.createRequests[0].DueAt != nil || h.app.reminders.selected == id.Nil {
		t.Fatal("native form did not create one canonical date-only reminder")
	}
	h.want("Perubahan reminder tersimpan di server.")
	captureReminder(t, h, "reminder-created")
	h.app.closeReminder()
	f.items[0].OwnerID, f.items[0].Relation = id.New(), api.ReminderRecipient
	h.app.loadReminders()
	settleReminders(t, h)
	h.app.openReminder(f.items[0].ID)
	settleReminders(t, h)
	h.tt.Frame()
	h.want("Dibagikan kepadamu")
	for _, label := range []string{"Judul reminder *", "Simpan reminder", "Tandai reminder selesai", "Tambah penerima", "Hapus reminder", "Riwayat reminder"} {
		if _, exists := h.tt.Find(label); exists {
			t.Errorf("recipient received owner control %q", label)
		}
	}
	captureReminder(t, h, "reminder-recipient")
}

func captureReminder(t *testing.T, h *harness, name string) {
	t.Helper()
	dir := os.Getenv("SPACE_UI_CAPTURE_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	err = png.Encode(f, h.tt.Image())
	closeErr := f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
}
