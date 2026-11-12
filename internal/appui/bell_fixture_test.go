package appui

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// bellAPI serves the bell's two feeds as the server does: a delivery row
// per event, recipient and channel, and the task notifications kept
// beside the pings of the same task updates.
type bellAPI struct {
	*fakeAPI
	TasksAPI
	deliveries []api.Delivery
	notes      []api.Notification
	responses  map[id.ID][]string
	reads      []id.ID
}

func (f *bellAPI) Deliveries(context.Context) ([]api.Delivery, error) {
	return slices.Clone(f.deliveries), nil
}
func (f *bellAPI) RespondDelivery(_ context.Context, d id.ID, r api.DeliveryResponse) error {
	if f.responses == nil {
		f.responses = map[id.ID][]string{}
	}
	f.responses[d] = append(f.responses[d], r.Action)
	now := time.Now()
	for i := range f.deliveries {
		if f.deliveries[i].ID != d {
			continue
		}
		switch r.Action {
		case "seen":
			f.deliveries[i].SeenAt = &now
		case "snooze":
			later := now.Add(time.Duration(r.SnoozeMinutes) * time.Minute)
			f.deliveries[i].AvailableAt, f.deliveries[i].SnoozedUntil = later, &later
		}
	}
	return nil
}
func (f *bellAPI) MyTasks(context.Context, id.ID, api.TaskFilter) ([]api.Task, error) {
	return nil, nil
}
func (f *bellAPI) SpaceTasks(context.Context, id.ID, api.TaskFilter) ([]api.Task, error) {
	return nil, nil
}
func (f *bellAPI) SendPing(context.Context, id.ID, api.PingRequest) error { return nil }
func (f *bellAPI) Notifications(context.Context) ([]api.Notification, error) {
	return slices.Clone(f.notes), nil
}
func (f *bellAPI) ReadNotification(_ context.Context, n id.ID) error {
	f.reads = append(f.reads, n)
	now := time.Now()
	for i := range f.notes {
		if f.notes[i].ID == n {
			f.notes[i].ReadAt = &now
		}
	}
	return nil
}

// event fans one delivery event out to its channels, as enqueue does.
func (f *bellAPI) event(kind string, source id.ID, related *id.ID, title, body string, at time.Time, channels ...string) id.ID {
	eid := id.New()
	for _, ch := range channels {
		d := api.Delivery{ID: id.New(), EventID: eid, SourceKind: kind, SourceID: source, Title: title, Body: body, TimeZone: "Asia/Jakarta", ScheduledAt: at, AvailableAt: at, Channel: ch, Status: "accepted"}
		if related != nil {
			d.RelatedKind, d.RelatedID = "task", related
		}
		f.deliveries = append(f.deliveries, d)
	}
	return eid
}

// bellFixture is what the owner saw in rc.2/rc.3: a meeting change and a
// task update twice (device and WhatsApp), the task update a third time
// from the task notifications, a task's reminder slots, and many others.
func bellFixture(t *testing.T, others int) (*harness, *bellAPI) {
	t.Helper()
	f := &bellAPI{fakeAPI: newFake(api.AccountPerson)}
	now := time.Now()
	meeting, lpj := id.New(), id.New()
	f.event("ping", id.New(), nil, "Perubahan rapat: [DUMMY] MUHISA", "Kamis, 13 Nov · 19.00", now.Add(-20*time.Minute), "device", "whatsapp")
	_ = meeting
	f.event("ping", id.New(), &lpj, "Tugas diperbarui: [DUMMY] LPJ", "PJ mengubah tenggat menjadi 14 Nov.", now.Add(-35*time.Minute), "device", "whatsapp")
	f.notes = append(f.notes, api.Notification{ID: id.New(), TaskID: lpj, Title: "Tugas diperbarui: [DUMMY] LPJ", Body: "PJ mengubah tenggat menjadi 14 Nov.", CreatedAt: now.Add(-35 * time.Minute)})
	for _, days := range []int{0, 1, 2} {
		f.event("task", lpj, nil, "[DUMMY] LPJ", "Pengingat: [DUMMY] LPJ", now.Add(-time.Duration(days)*24*time.Hour-time.Hour), "device", "whatsapp")
	}
	for i := range others {
		f.event("reminder", id.New(), nil, fmt.Sprintf("Pengingat %02d", i+1), "Pengingat: catatan rapat divisi", now.Add(-time.Duration(i+2)*7*time.Hour), "device")
	}
	h := newHarness(t, f.fakeAPI, "")
	h.app.api = f
	h.app.screen = screenShell
	h.app.user = f.user
	h.app.conn = connOnline
	h.app.periodID = f.periods[1].ID
	h.app.periods = f.periods
	h.app.PollDeliveries()
	h.settle()
	return h, f
}

// TestBellScreenshots writes the open bell at 100 %, 150 % and 200 % to
// SPACE_SHOTS, for the review's before and after.
func TestBellScreenshots(t *testing.T) {
	dir := os.Getenv("SPACE_SHOTS")
	if dir == "" {
		t.Skip("SPACE_SHOTS not set")
	}
	for _, scale := range []float32{1, 1.5, 2} {
		h, _ := bellFixture(t, 14)
		h.tt.SetScale(scale)
		h.click("Notifikasi")
		h.settle()
		h.tt.Advance(time.Second)
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("bell-%d.png", int(scale*100))))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, h.tt.Image()); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	// After Tandai semua dibaca: read rows, buttons disabled.
	{
		h, _ := bellFixture(t, 14)
		h.click("Notifikasi")
		h.settle()
		h.app.markBellRead(h.app.bell(time.Now())[1:])
		h.settle()
		h.tt.Advance(time.Second)
		f, err := os.Create(filepath.Join(dir, "bell-read.png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, h.tt.Image()); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	// The bell a moment after Escape: fading, not gone at once.
	h, _ := bellFixture(t, 14)
	h.click("Notifikasi")
	h.settle()
	h.tt.Key(0, ui.KeyEscape)
	h.tt.Advance(50 * time.Millisecond)
	f, err := os.Create(filepath.Join(dir, "bell-closing.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, h.tt.Image()); err != nil {
		t.Fatal(err)
	}
}
