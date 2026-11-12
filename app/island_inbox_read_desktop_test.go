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
