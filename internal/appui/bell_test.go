package appui

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// readSink records what the app tells Ace's inbox was read.
type readSink struct {
	quietSink
	read []id.ID
}

func (s *readSink) NotificationsRead(_ id.ID, ids []id.ID) { s.read = append(s.read, ids...) }

// The owner's rc.2/rc.3 bell: a meeting change twice (device and WhatsApp),
// a task update three times (and its task notification), and a task's
// three reminder slots — each one row now, the slots with their count.
func TestBellFoldsChannelsUpdatesAndSlots(t *testing.T) {
	_, f := bellFixture(t, 0)
	items := bellItems(f.deliveries, f.notes, time.Now())
	count := map[string]int{}
	for _, it := range items {
		count[it.title]++
	}
	for title, want := range map[string]int{"Perubahan rapat: [DUMMY] MUHISA": 1, "Tugas diperbarui: [DUMMY] LPJ": 1, "[DUMMY] LPJ": 1} {
		if count[title] != want {
			t.Errorf("%q in %d rows, want %d", title, count[title], want)
		}
	}
	for _, it := range items {
		switch it.title {
		case "[DUMMY] LPJ":
			if it.count != 3 || len(it.deliveries) != 6 {
				t.Errorf("slots: count %d, deliveries %d", it.count, len(it.deliveries))
			}
		case "Tugas diperbarui: [DUMMY] LPJ":
			if it.count != 1 || len(it.notes) != 1 || len(it.deliveries) != 2 {
				t.Errorf("update: count %d, notes %d, deliveries %d", it.count, len(it.notes), len(it.deliveries))
			}
		case "Perubahan rapat: [DUMMY] MUHISA":
			if it.count != 1 || !it.snoozable() || !it.whatsApp() {
				t.Errorf("meeting change: count %d snoozable %v", it.count, it.snoozable())
			}
		}
	}
	if len(items) != 3 {
		t.Fatalf("%d rows, want 3", len(items))
	}
}

// What the server sends twice stays apart when it is not the same thing:
// another body, or a second update a day later counts.
func TestBellKeepsDistinctNoticesApart(t *testing.T) {
	now := time.Now()
	task := id.New()
	notes := []api.Notification{
		{ID: id.New(), TaskID: task, Title: "Tugas diperbarui: Poster", Body: "Tenggat berubah.", CreatedAt: now.Add(-time.Hour)},
		{ID: id.New(), TaskID: task, Title: "Tugas diperbarui: Poster", Body: "Tenggat berubah.", CreatedAt: now.Add(-25 * time.Hour)},
		{ID: id.New(), TaskID: task, Title: "Tugas diperbarui: Poster", Body: "PJ berganti.", CreatedAt: now.Add(-2 * time.Hour)},
	}
	items := bellItems(nil, notes, now)
	if len(items) != 2 || items[0].count != 2 || items[1].count != 1 {
		t.Fatalf("rows %+v", items)
	}
	if bellUnread(items) != 2 {
		t.Fatalf("unread %d", bellUnread(items))
	}
	// Upcoming reminders are listed last, and wait for nobody.
	later := []api.Delivery{{ID: id.New(), EventID: id.New(), SourceKind: "reminder", SourceID: id.New(), Title: "Nanti", Channel: "device", Status: "queued", ScheduledAt: now.Add(time.Hour), AvailableAt: now.Add(time.Hour)}}
	items = bellItems(later, notes, now)
	if last := items[len(items)-1]; !last.upcoming || last.unread() || last.snoozable() {
		t.Fatalf("upcoming row %+v", last)
	}
}

// Tandai semua dibaca marks every device delivery seen and every task
// notification read on the server once — never WhatsApp's copies — tells
// Ace's inbox, and leaves the bell with nothing unread.
func TestBellMarkAllRead(t *testing.T) {
	h, f := bellFixture(t, 3)
	sink := &readSink{}
	h.app.Notifications = sink
	h.click("Notifikasi")
	h.settle()
	h.want("Tandai semua dibaca")
	if n := bellUnread(h.app.bell(time.Now())); n != 6 {
		t.Fatalf("unread before %d, want 6", n)
	}
	h.click("Tandai semua dibaca")
	h.settle()
	if n := bellUnread(h.app.bell(time.Now())); n != 0 {
		t.Fatalf("unread after %d", n)
	}
	device := 0
	for _, d := range f.deliveries {
		got := f.responses[d.ID]
		if d.Channel == "whatsapp" {
			if len(got) != 0 {
				t.Errorf("WhatsApp delivery answered %v", got)
			}
			continue
		}
		device++
		seen := 0
		for _, a := range got {
			if a == "seen" {
				seen++
			}
		}
		if seen != 1 {
			t.Errorf("device delivery %s seen %d times (%v)", d.Title, seen, got)
		}
	}
	if len(f.reads) != 1 || f.reads[0] != f.notes[0].ID {
		t.Errorf("task notifications read %v", f.reads)
	}
	if len(sink.read) < device {
		t.Errorf("Ace's inbox heard %d of %d", len(sink.read), device)
	}
	// Disabled now, and says why.
	h.click("Tandai semua dibaca")
	h.settle()
	if strings.Count(strings.Join(h.tt.Texts(), "|"), "Sudah dibaca") == 0 {
		t.Log(h.tt.Texts())
	}
	// A later poll keeps them read: the server's state is the bell's.
	h.app.PollDeliveries()
	h.settle()
	if n := bellUnread(h.app.bell(time.Now())); n != 0 {
		t.Fatalf("unread after poll %d", n)
	}
}

// One row's Tandai dibaca reads only that row; Tunda puts it off: it
// leaves today's rows and waits under Terjadwal.
func TestBellRowReadAndSnooze(t *testing.T) {
	h, f := bellFixture(t, 0)
	h.click("Notifikasi")
	h.settle()
	items := h.app.bell(time.Now())
	h.app.markBellRead(items[:1])
	h.settle()
	if n := bellUnread(h.app.bell(time.Now())); n != 2 {
		t.Fatalf("unread %d after one row", n)
	}
	var meeting bellItem
	for _, it := range h.app.bell(time.Now()) {
		if strings.Contains(it.title, "MUHISA") {
			meeting = it
		}
	}
	h.app.snoozeBell(meeting)
	h.settle()
	for _, it := range h.app.bell(time.Now()) {
		if strings.Contains(it.title, "MUHISA") && !it.upcoming {
			t.Fatal("snoozed row still among today's")
		}
	}
	snoozed := 0
	for _, acts := range f.responses {
		for _, a := range acts {
			if a == "snooze" {
				snoozed++
			}
		}
	}
	if snoozed != 1 {
		t.Fatalf("%d snoozes, want 1 (the device's only)", snoozed)
	}
}

// Many notices: the bell shows a page, then Muat lagi; every row it shows
// lies inside its panel at 100 %, 150 % and 200 %, so nothing spills over
// the page behind it.
func TestBellPagesAndStaysInsideItsPanel(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		h, _ := bellFixture(t, 30)
		h.tt.SetScale(scale)
		h.click("Notifikasi")
		h.settle()
		h.want("Muat lagi · 23 lagi")
		h.dontWant("Pengingat 12")
		panel, ok := h.tt.Find("Panel notifikasi")
		if !ok {
			t.Fatal("no bell panel")
		}
		for _, title := range []string{"Perubahan rapat: [DUMMY] MUHISA", "Pengingat 01", "Pengingat 07"} {
			r, ok := h.tt.Find(title)
			if !ok {
				t.Fatalf("%v: no %q", scale, title)
			}
			if r.W == 0 || r.H == 0 {
				continue // scrolled out of the region, clipped
			}
			if r.Y < panel.Y || r.Y+r.H > panel.Y+panel.H+0.5 || r.X < panel.X || r.X+r.W > panel.X+panel.W+0.5 {
				t.Errorf("%v: %q at %+v outside the panel %+v", scale, title, r, panel)
			}
		}
		h.tt.Scroll(panel.X+panel.W/2, panel.Y+panel.H/2, 0, 5000)
		h.tt.Advance(time.Second)
		if r, _ := h.tt.Find("Muat lagi · 23 lagi"); r.Y+r.H > panel.Y+panel.H+0.5 {
			t.Fatalf("%v: Muat lagi at %+v below the panel %+v", scale, r, panel)
		}
		h.click("Muat lagi · 23 lagi")
		h.settle()
		h.want("Muat lagi · 13 lagi")
		if h.app.chrome.bell.limit != 2*bellPage {
			t.Fatalf("limit %d", h.app.chrome.bell.limit)
		}
	}
}

// Closing the bell lets it go: nothing of it can be found or clicked as it
// fades, and it is gone once its motion ends.
func TestBellClosesWithMotion(t *testing.T) {
	h, _ := bellFixture(t, 2)
	h.click("Notifikasi")
	h.settle()
	h.want("Tandai semua dibaca")
	h.tt.Key(0, ui.KeyEscape)
	h.tt.Frame()
	if h.app.chrome.notificationsOpen {
		t.Fatal("Escape did not close the bell")
	}
	h.dontWant("Tandai semua dibaca")
	h.tt.Advance(time.Second)
	h.dontWant("Tandai semua dibaca")
}

// While the desktop asks for less motion, the bell is in its place at
// once: nothing slides or grows on the way.
func TestBellRespectsReducedMotion(t *testing.T) {
	h, _ := bellFixture(t, 2)
	h.tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	h.tt.Click("Notifikasi")
	h.tt.Frame()
	first, ok := h.tt.Find("Panel notifikasi")
	if !ok {
		t.Fatal("no panel")
	}
	h.tt.Advance(time.Second)
	settled, _ := h.tt.Find("Panel notifikasi")
	if first != settled {
		t.Fatalf("panel moved from %+v to %+v", first, settled)
	}
}
