//go:build !js

package main

import (
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// What the bell marks read leaves Ace's inbox, and its "+N", too; another
// account's notices and the unread ones stay.
func TestIslandInboxForgetsWhatTheBellRead(t *testing.T) {
	uid, other := id.New(), id.New()
	d := &desktopDevice{root: t.TempDir(), activeUser: uid}
	d.notificationPrefs.Store(devicePreferences{IslandEnabled: true})
	p := &islandPresenter{d: d}
	d.island = p
	t.Cleanup(p.inbox.saves.Wait)
	read, unread := id.New(), id.New()
	for _, did := range []id.ID{read, unread} {
		if err := d.NotifyQuiet(uid, []api.Delivery{{ID: did, SourceKind: "reminder", Title: "Pengingat"}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.NotifyQuiet(other, []api.Delivery{{ID: read, SourceKind: "reminder", Title: "Pengingat"}}); err != nil {
		t.Fatal(err)
	}
	d.NotificationsRead(uid, []id.ID{read, id.New()})
	p.mu.Lock()
	mine, theirs := p.inboxCount(uid.String()), p.inboxCount(other.String())
	p.mu.Unlock()
	if mine != 1 || theirs != 1 || p.inboxHas(uid.String()+":"+read.String()) {
		t.Fatalf("mine %d, theirs %d", mine, theirs)
	}
}

// What Ace's inbox marks read reaches the app as deliveries per account,
// once each; an update's or Ace's own tags are not deliveries.
func TestIslandInboxReadTags(t *testing.T) {
	uid, other, a, b := id.New(), id.New(), id.New(), id.New()
	got := deliveryTags([]string{uid.String() + ":" + a.String(), uid.String() + ":" + a.String(), other.String() + ":" + b.String(), "update:0.4.6-rc.4", "ace-hello", uid.String() + ":x"})
	if len(got) != 2 || len(got[uid]) != 1 || got[uid][0] != a || len(got[other]) != 1 || got[other][0] != b {
		t.Fatalf("%v", got)
	}
}
