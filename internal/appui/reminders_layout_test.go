package appui

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Form B (canvas 3747dbbac0d0 design 3): the chosen reminder reads as a
// card with Tandai selesai, Ubah and ⋯; Ubah opens the sentence dialog,
// whose Simpan stays in the window at the largest and smallest sizes.
func TestReminderCardAndDialogFit(t *testing.T) {
	h, f := reminderHarness(t)
	h.app.router.Replace(PathReminder)
	h.app.openReminder(f.items[0].ID)
	settleReminders(t, h)
	for _, size := range [][2]int{{1440, 900}, {880, 640}} {
		h.tt.SetSize(size[0], size[1])
		h.tt.Advance(400 * time.Millisecond)
		h.tt.Frame()
		for _, s := range []string{"Tandai selesai", "Ubah", "Begini Ace akan mengingatkanmu", "Hanya kamu"} {
			h.want(s)
		}
		h.click("Ubah")
		h.tt.Advance(400 * time.Millisecond)
		h.tt.Frame()
		for _, label := range []string{"Simpan", "Batal", "Judul reminder", "Tanggal reminder: Jum, 30 Okt", "Tutup reminder"} {
			r, ok := h.tt.Find(label)
			if !ok || r.W <= 0 || r.X < 0 || r.Y < 0 || r.X+r.W > float32(size[0])+1 || r.Y+r.H > float32(size[1])+1 {
				t.Fatalf("%dx%d: %s clipped: %+v exists=%v", size[0], size[1], label, r, ok)
			}
		}
		// Always WIB: no zone to choose.
		h.dontWant("Zona waktu reminder")
		h.want("WIB")
		captureNativeUI(t, h, "reminder-edit-"+map[int]string{1440: "large", 880: "minimum"}[size[0]])
		h.click("Batal")
		h.tt.Advance(400 * time.Millisecond)
		h.tt.Frame()
		if h.app.reminders.dialog {
			t.Fatal("Batal left the dialog open")
		}
	}
}

// The sentence makes a reminder: an empty title is refused in words, the
// date comes from Space's picker, Tanpa jam makes it a date alone; a
// shared reminder is read-only.
func TestReminderNativeFormAndReadOnlyRecipient(t *testing.T) {
	h, f := reminderHarness(t)
	h.tt.SetSize(1440, 900)
	h.app.router.Replace(PathReminder)
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	h.click("Reminder baru")
	h.tt.Frame()
	first := h.app.reminders.placeholder
	h.click("Buat reminder")
	h.tt.Frame()
	h.want("Isi dulu mau diingatkan soal apa.")
	if len(f.createRequests) != 0 {
		t.Fatal("an empty title was sent")
	}
	h.click("Judul reminder")
	h.tt.Type("Ingat notulensi rapat")
	h.tt.Frame()
	h.want("Ingat notulensi rapat")
	h.click("Tanggal reminder: " + strings.SplitN(reminderDraftWhen(h.app.reminders.draft), " · ", 2)[0])
	h.tt.Advance(400 * time.Millisecond)
	h.click("Tanggal 2026-11-02")
	h.click("Tanpa jam")
	h.click("Selesai")
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	h.want("(tanpa jam)")
	h.click("Buat reminder")
	settleReminders(t, h)
	h.tt.Frame()
	if len(f.createRequests) != 1 || f.createRequests[0].DueKind != api.DueDate || f.createRequests[0].DueAt != nil || f.createRequests[0].TimeZone != "Asia/Jakarta" || h.app.reminders.selected == id.Nil || h.app.reminders.dialog {
		t.Fatalf("form did not create one date-only WIB reminder: %+v", f.createRequests)
	}
	captureNativeUI(t, h, "reminder-created")
	// The next new reminder shows another example.
	h.app.beginReminder()
	if h.app.reminders.placeholder == first {
		t.Fatal("placeholder did not rotate")
	}
	h.app.cancelReminderDialog()
	h.app.closeReminder()
	f.items[0].OwnerID, f.items[0].Relation = id.New(), api.ReminderRecipient
	h.app.loadReminders()
	settleReminders(t, h)
	h.app.openReminder(f.items[0].ID)
	settleReminders(t, h)
	h.tt.Frame()
	h.want("Dibagikan kepadamu · hanya baca")
	for _, label := range []string{"Ubah", "Tandai selesai", "Lainnya", "Pilih Ingat dokumen pribadi"} {
		if _, exists := h.tt.Find(label); exists {
			t.Errorf("recipient received owner control %q", label)
		}
	}
	captureNativeUI(t, h, "reminder-recipient")
}

// The owner's ask: circles pick several reminders, one button finishes
// them, and Urungkan opens them again.
func TestReminderPickSeveralAndFinish(t *testing.T) {
	h, f := reminderHarness(t)
	second := f.items[0]
	second.ID, second.Title = id.New(), "Bayar kas"
	f.items = append(f.items, second)
	h.app.loadReminders()
	settleReminders(t, h)
	h.tt.SetSize(1440, 900)
	h.app.router.Replace(PathReminder)
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	h.click("Pilih Ingat dokumen pribadi")
	h.tt.Frame()
	h.want("Tandai selesai (1)")
	h.click("Pilih semua")
	h.tt.Frame()
	h.want("Tandai selesai (2)")
	h.click("Tandai selesai (2)")
	settleReminders(t, h)
	h.tt.Frame()
	for _, r := range h.app.reminders.items {
		if !r.Completed {
			t.Fatalf("%q not finished", r.Title)
		}
	}
	h.want("2 reminder ditandai selesai")
	h.click("Urungkan")
	settleReminders(t, h)
	for _, r := range h.app.reminders.items {
		if r.Completed {
			t.Fatalf("Urungkan left %q finished", r.Title)
		}
	}
}

func captureNativeUI(t *testing.T, h *harness, name string) {
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
