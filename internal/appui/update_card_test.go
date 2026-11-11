package appui

import (
	"testing"
	"time"
)

// fakeUpdater is the device's updater as the card sees it.
type fakeUpdater struct {
	status                 UpdateStatus
	ok                     bool
	installs, notes, later int
}

func (f *fakeUpdater) UpdateStatus() (UpdateStatus, bool) { return f.status, f.ok }
func (f *fakeUpdater) InstallUpdate()                     { f.installs++ }
func (f *fakeUpdater) UpdateNotes()                       { f.notes++ }
func (f *fakeUpdater) UpdateLater()                       { f.later++ }

func updateCardHarness(t *testing.T) (*harness, *fakeUpdater) {
	t.Helper()
	h := newChromeHarness(t, nil)
	h.tt.SetSize(1440, 900)
	f := &fakeUpdater{}
	h.app.Updater = f
	return h, f
}

func (h *harness) step() {
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
}

// Found, downloading (folded and unfolded), ready, then Nanti until the
// next version.
func TestUpdateCardStages(t *testing.T) {
	h, f := updateCardHarness(t)
	h.step()
	h.dontWant("Perbarui")

	f.status, f.ok = UpdateStatus{Stage: UpdateAvailable, Version: "0.4.6-rc.3", Note: "Kartu pembaruan di pojok kiri bawah."}, true
	h.step()
	for _, want := range []string{"Space 0.4.6-rc.3 tersedia", "Kartu pembaruan di pojok kiri bawah.", "Perbarui", "Lihat perubahan", "Nanti"} {
		h.want(want)
	}
	h.click("Lihat perubahan")
	h.click("Perbarui")
	if f.notes != 1 || f.installs != 1 {
		t.Fatalf("notes %d installs %d", f.notes, f.installs)
	}

	f.status.Stage, f.status.Done, f.status.Total = UpdateDownloading, 42, 100
	h.step()
	h.want("Mengunduh Space 0.4.6-rc.3")
	h.want("42% · 0.0 dari 0.0 MB")
	h.dontWant("Perbarui")
	h.click("Kecilkan")
	h.step()
	h.dontWant("Kecilkan")
	h.want("42%")
	h.click("Mengunduh Space 0.4.6-rc.3, 42%. Tampilkan")
	h.step()
	h.want("Mengunduh Space 0.4.6-rc.3")

	f.status.Stage = UpdateReady
	h.step()
	h.want("Siap dipasang: Space 0.4.6-rc.3")
	h.want("Nanti (saat Space ditutup)")
	h.click("Mulai ulang sekarang")
	if f.installs != 2 {
		t.Fatalf("restart did not install: %d", f.installs)
	}
	h.click("Nanti (saat Space ditutup)")
	h.step()
	h.dontWant("Siap dipasang: Space 0.4.6-rc.3")
	if f.later != 1 {
		t.Fatalf("Nanti not told to the device: %d", f.later)
	}
	// Still hidden at the next check of the same version...
	f.status.Stage = UpdateAvailable
	h.step()
	h.dontWant("Space 0.4.6-rc.3 tersedia")
	// ...but a newer version comes back.
	f.status.Version = "0.4.6-rc.4"
	h.step()
	h.want("Space 0.4.6-rc.4 tersedia")
}

// The card waits while the user writes, and stays once shown; the
// onboarding's tour keeps it away.
func TestUpdateCardWaitsForTypingAndOnboarding(t *testing.T) {
	h, f := updateCardHarness(t)
	h.app.tasks.draft = &taskDraft{} // writing a task
	f.status, f.ok = UpdateStatus{Stage: UpdateAvailable, Version: "0.4.6-rc.3"}, true
	h.step()
	h.dontWant("Space 0.4.6-rc.3 tersedia")
	h.app.tasks.draft = nil
	h.step()
	h.want("Space 0.4.6-rc.3 tersedia")
	h.app.tasks.draft = &taskDraft{} // shown: writing no longer hides it
	h.step()
	h.want("Space 0.4.6-rc.3 tersedia")
	h.app.tasks.draft = nil

	h.app.onb.tour.open = true
	h.step()
	h.dontWant("Space 0.4.6-rc.3 tersedia")
	h.app.onb.tour.open = false
	h.step()
	h.want("Space 0.4.6-rc.3 tersedia")
}

func TestUpdateCardText(t *testing.T) {
	if got := FirstNoteLine("\n- Untukku: satu daftar.\n- Lainnya"); got != "Untukku: satu daftar." {
		t.Fatalf("first line %q", got)
	}
	_, line := updateCardText(UpdateStatus{Stage: UpdateReady, Version: "1", Auto: true})
	if line != "Sudah diunduh dan diperiksa. Dipasang sendiri saat Space ditutup." {
		t.Fatalf("ready line %q", line)
	}
	if p := updatePercent(UpdateStatus{Done: 150, Total: 100}); p != 100 {
		t.Fatalf("percent %d", p)
	}
}
