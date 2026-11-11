//go:build !js

package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/island"
)

func inboxNotice(account string, i int) appui.Notice {
	return appui.Notice{Kind: "ping", Title: fmt.Sprintf("Ping %d", i), Tag: fmt.Sprintf("%s:%d", account, i), AccountID: account}
}

// Newest first, once per tag, at most 50, for 7 days, per account; Ace
// speaking, tests and updates are never kept; it survives a restart.
func TestIslandInboxModel(t *testing.T) {
	now := time.Now()
	var b islandInbox
	for i := range 60 {
		b.add(inboxNotice("a", i), now)
	}
	if len(b.Entries) != islandInboxCap || b.Entries[0].Notice.Title != "Ping 59" {
		t.Fatalf("cap/order: %d first %q", len(b.Entries), b.Entries[0].Notice.Title)
	}
	b.add(inboxNotice("a", 30), now)
	if len(b.Entries) != islandInboxCap || b.Entries[0].Notice.Title != "Ping 30" {
		t.Fatal("same tag twice")
	}
	for _, n := range []appui.Notice{{Kind: appui.NoticeAce, Tag: "x", AccountID: "a"}, {Kind: appui.NoticeTest, Tag: "y", AccountID: "a"}, {Kind: "update", Tag: "z", AccountID: "a"}, {Kind: "ping", Tag: "w"}} {
		if b.add(n, now) {
			t.Errorf("kept %+v", n)
		}
	}
	b.add(inboxNotice("b", 1), now.Add(-8*24*time.Hour))
	if len(b.of("b")) != 1 || !b.expire(now) || len(b.of("b")) != 0 {
		t.Fatal("expiry")
	}
	root := t.TempDir()
	if err := saveIslandInbox(root, b); err != nil {
		t.Fatal(err)
	}
	if got := loadIslandInbox(root, now); len(got.Entries) != len(b.Entries) {
		t.Fatalf("after restart %d", len(got.Entries))
	}
	if !b.clear("a") || len(b.of("a")) != 0 {
		t.Fatal("clear")
	}
}

// A notice that times out unseen waits in the inbox; a click on Ace at
// rest shows the newest, the pager goes through them, Tutup takes one out
// and shows the next, Tutup semua empties it. Quiet hours put deliveries
// there at once.
func TestIslandInboxFlow(t *testing.T) {
	uid := id.New()
	d := &desktopDevice{root: t.TempDir(), activeUser: uid}
	d.notificationPrefs.Store(devicePreferences{IslandEnabled: true})
	p := &islandPresenter{d: d}
	d.island = p
	account := uid.String()
	now := time.Now()
	for i := range 3 {
		n := inboxNotice(account, i)
		p.m.Push(island.Item{Tag: n.Tag, Value: islandNotice{n: n, count: 1, first: now, latest: n.Title}}, now)
		p.m.Tick(now.Add(island.BannerDuration+time.Duration(i)*time.Second), false, false, false)
		p.m.Tick(now.Add(island.BannerDuration+time.Duration(i)*time.Second+time.Second), false, false, false)
	}
	p.keepLapsed(now)
	if got := p.inboxCount(account); got != 3 {
		t.Fatalf("kept %d", got)
	}
	if !p.openInbox(now, account) || p.m.Current == nil || p.m.Current.Value.(islandNotice).n.Title != "Ping 2" {
		t.Fatalf("open: %+v", p.m.Current)
	}
	if i, n := p.inboxPosition(p.m.Current.Tag, account); i != 0 || n != 3 {
		t.Fatalf("position %d of %d", i, n)
	}
	// Tutup: out of the inbox, the next follows.
	if !p.handled(now, account) || p.inboxCount(account) != 2 || p.m.Current.Value.(islandNotice).n.Title != "Ping 1" {
		t.Fatalf("after Tutup: %d, %+v", p.inboxCount(account), p.m.Current)
	}
	// Folding again while paging does not reorder or duplicate.
	p.m.Tick(now.Add(time.Hour), false, false, false)
	p.m.Tick(now.Add(time.Hour+2*time.Second), false, false, false)
	p.keepLapsed(now)
	if p.inboxCount(account) != 2 {
		t.Fatalf("after folding %d", p.inboxCount(account))
	}
	// Another account sees none of them.
	if p.inboxCount(id.New().String()) != 0 {
		t.Fatal("another account's inbox")
	}
	// Quiet hours: straight into the inbox.
	if err := d.NotifyQuiet(uid, []api.Delivery{{ID: id.New(), SourceKind: "ping", Title: "Ping · Mediatek"}}); err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	n := p.inboxCount(account)
	p.mu.Unlock()
	if n != 3 {
		t.Fatalf("quiet arrival: %d", n)
	}
	// Tutup semua.
	p.inbox.box.clear(account)
	if p.inboxCount(account) != 0 {
		t.Fatal("clear")
	}
	// It was kept on the device (written in the background).
	deadline := time.Now().Add(2 * time.Second)
	for {
		got := loadIslandInbox(d.root, now)
		if len(got.of(account)) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("not saved")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Every kind has its own icon: a manual ping about a task is a ping.
func TestNoticeKinds(t *testing.T) {
	tid := id.New()
	ping := api.Delivery{SourceKind: "ping", Title: "Ping · Mediatek", RelatedKind: "task", RelatedID: &tid}
	notice := api.Delivery{SourceKind: "ping", Title: "Tugas baru", RelatedKind: "task", RelatedID: &tid}
	if appui.DeliveryKind(ping) != "ping" || appui.DeliveryKind(notice) != "task" {
		t.Fatalf("kinds %q %q", appui.DeliveryKind(ping), appui.DeliveryKind(notice))
	}
	if appui.NoticeGlyph("ping") == appui.NoticeGlyph("task") || appui.NoticeGlyph("meeting") != appui.NoticeGlyph("reminder") || appui.NoticeGlyph("task") != appui.NoticeGlyph("review") {
		t.Fatal("glyphs")
	}
}
