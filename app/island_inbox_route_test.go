//go:build !js

package main

import (
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/island"
)

// inboxPresenter is an island at rest with n notices waiting for the
// signed-in account.
func inboxPresenter(t *testing.T, n int) (*islandPresenter, string) {
	t.Helper()
	uid := id.New()
	d := &desktopDevice{root: t.TempDir(), activeUser: uid}
	d.notificationPrefs.Store(devicePreferences{IslandEnabled: true})
	p := &islandPresenter{d: d}
	d.island = p
	t.Cleanup(p.inbox.saves.Wait)
	account := uid.String()
	var xs []appui.Notice
	for i := range n {
		xs = append(xs, inboxNotice(account, i))
	}
	p.store(xs)
	return p, account
}

// frames drives the island's frame logic (no window) in a tester, the
// pointer over the pill or not.
type frames struct {
	p    *islandPresenter
	tt   *ui.Tester
	over bool
	f    islandFrame
}

func newFrames(p *islandPresenter) *frames {
	fr := &frames{p: p}
	fr.tt = ui.NewTester(func(c *ui.Context) {
		fr.f = p.frame(c, c.Now(), devicePreferences{IslandEnabled: true}, fr.over)
		p.openToday(fr.f)
	}, 200, 100)
	return fr
}

// run advances d in 50 ms frames.
func (fr *frames) run(d time.Duration) {
	for step := time.Duration(0); step <= d; step += 50 * time.Millisecond {
		fr.tt.Advance(50 * time.Millisecond)
		fr.tt.Frame()
	}
}

func (fr *frames) title() string {
	if fr.f.notice == nil {
		return ""
	}
	return fr.f.notice.n.Title
}

// The owner's rc.2/rc.3 report: Ace showed "+1", and under the pointer it
// opened Hari ini with no way to the notice or to clear it. The details
// opened at rest are the inbox while notices wait, newest first; Hari ini
// is chosen from there and comes back to the inbox once the pointer left;
// "Tandai dibaca" takes one out and the "+N" goes down to nothing, and
// Hari ini is what opens then.
func TestIslandInboxOpensBeforeToday(t *testing.T) {
	p, account := inboxPresenter(t, 2)
	fr := newFrames(p)
	fr.run(100)
	if fr.f.inbox != 2 || fr.f.notice != nil || fr.f.expanded {
		t.Fatalf("at rest: %+v", fr.f)
	}
	// The pointer rests on Ace: the details open on the newest notice.
	fr.over = true
	fr.run(400 * time.Millisecond)
	if fr.title() != "Ping 1" || !fr.f.expanded || fr.f.scene != island.SceneNotice || !p.inboxShowing(fr.f.notice.n.Tag) {
		t.Fatalf("hovered with notices waiting: %q expanded %v scene %v", fr.title(), fr.f.expanded, fr.f.scene)
	}
	// Hari ini: the notice goes back unread, the day's details open.
	p.showToday()
	fr.run(300 * time.Millisecond)
	if fr.f.notice != nil || !fr.f.expanded || fr.f.inbox != 2 {
		t.Fatalf("Hari ini: %q expanded %v inbox %d", fr.title(), fr.f.expanded, fr.f.inbox)
	}
	// The pointer leaves, the details fold; back on Ace, the inbox again.
	fr.over = false
	fr.run(800 * time.Millisecond)
	if fr.f.expanded || p.today {
		t.Fatal("Hari ini stayed after the pointer left")
	}
	fr.over = true
	fr.run(400 * time.Millisecond)
	if fr.title() != "Ping 1" {
		t.Fatalf("hovered again: %q", fr.title())
	}
	// Tandai dibaca: out, the next follows, "+N" down.
	p.dismiss()
	fr.run(200 * time.Millisecond)
	if fr.f.inbox != 1 || fr.title() != "Ping 0" {
		t.Fatalf("after one read: inbox %d, %q", fr.f.inbox, fr.title())
	}
	// Opening it (Buka) marks it read too: the inbox is empty, no "+N".
	p.dismiss()
	fr.over = false
	fr.run(800 * time.Millisecond)
	if fr.f.inbox != 0 || fr.f.notice != nil {
		t.Fatalf("all read: inbox %d, %q", fr.f.inbox, fr.title())
	}
	fr.over = true
	fr.run(400 * time.Millisecond)
	if fr.f.notice != nil || !fr.f.expanded {
		t.Fatalf("empty inbox: Hari ini expected, got %q expanded %v", fr.title(), fr.f.expanded)
	}
	// Read stays read after a restart.
	p.inbox.saves.Wait()
	if got := loadIslandInbox(p.d.root, time.Now()); len(got.of(account)) != 0 {
		t.Fatalf("after restart %d back", len(got.of(account)))
	}
}

// A click on Ace at rest opens the inbox too, and "Tandai semua dibaca"
// clears it for good, past a restart, however the writes interleave.
func TestIslandInboxClickAndClearAll(t *testing.T) {
	p, account := inboxPresenter(t, 3)
	fr := newFrames(p)
	fr.run(50)
	p.click(fr.tt.Now(), fr.f)
	fr.run(50)
	if fr.title() != "Ping 2" || !fr.f.expanded {
		t.Fatalf("click: %q", fr.title())
	}
	// Writes in a burst: the last one is what stays.
	for range 20 {
		p.mu.Lock()
		p.inbox.changed = true
		p.saveInbox()
		p.mu.Unlock()
	}
	p.dismissAll()
	fr.run(300 * time.Millisecond)
	if fr.f.inbox != 0 || fr.f.notice != nil {
		t.Fatalf("all read: %d %q", fr.f.inbox, fr.title())
	}
	p.inbox.saves.Wait()
	if got := loadIslandInbox(p.d.root, time.Now()); len(got.of(account)) != 0 {
		t.Fatalf("cleared notices back after restart: %d", len(got.of(account)))
	}
}

// The inbox's details and Hari ini with notices waiting, in the island's
// colors; SPACE_UI_CAPTURE_DIR keeps both.
func TestIslandInboxDetailsPicture(t *testing.T) {
	p, account := inboxPresenter(t, 3)
	p.mu.Lock()
	p.openInbox(time.Now(), account)
	x := p.m.Current.Value.(islandNotice)
	p.mu.Unlock()
	islandPaletteFor(devicePreferences{IslandTheme: islandThemeBlack}, nil).use()
	pill := func(c *ui.Context, fn func()) {
		g := island.Geometry(island.SceneNotice, true)
		left := float32(island.ContentLeft(island.SceneNotice, true))
		ui.Column(c).Fill().Padding(16).Background(ui.RGB(96, 110, 140)).Children(func() {
			ui.Box(c).Size(float32(g[0]), float32(g[1])).Radius(float32(g[2])).Background(islandInk).TextColor(islandText).Children(func() {
				ui.Box(c).Absolute().Left(left).Top(24).Width(float32(g[0]) - left - 28).Children(fn)
			})
		})
	}
	tt := ui.NewTester(func(c *ui.Context) { pill(c, func() { p.noticeDetails(c, x, 0) }) }, 472, 216)
	tt.Frame()
	for _, want := range []string{"Ping 2", "1 dari 3", "Tandai dibaca", "Hari ini", "Tandai semua dibaca"} {
		if !tt.HasText(want) {
			t.Fatalf("inbox details without %q: %q", want, tt.Texts())
		}
	}
	shots := []image.Image{cloneImage(tt.Image())}
	tt2 := ui.NewTester(func(c *ui.Context) { pill(c, func() { p.restDetails(c, time.Now(), 3) }) }, 472, 216)
	tt2.Frame()
	if !tt2.HasText("Kabar · 3") {
		t.Fatalf("Hari ini without the way back: %q", tt2.Texts())
	}
	shots = append(shots, cloneImage(tt2.Image()))
	// Clicked, Tandai dibaca takes the notice out.
	if err := tt.Click("Tandai dibaca"); err != nil {
		t.Fatal(err)
	}
	if p.inboxCount(account) != 2 {
		t.Fatalf("after Tandai dibaca: %d", p.inboxCount(account))
	}
	if dir := os.Getenv("SPACE_UI_CAPTURE_DIR"); dir != "" {
		writePNG(t, filepath.Join(dir, "island-inbox.png"), joinImages(shots, false))
	}
}
