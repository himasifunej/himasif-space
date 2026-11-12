//go:build !js

package main

import (
	"slices"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/id"
)

// The bell and Ace's inbox share the server's read state: what the bell
// marks read (a row, Tandai semua dibaca, Buka) or a poll finds seen,
// from this device or another, leaves the inbox and its "+N" too. The
// inbox's tags are the deliveries' (deliveryNotice: account:delivery).

// NotificationsRead takes uid's deliveries ids out of the inbox
// (appui.ReadNotificationSink).
func (d *desktopDevice) NotificationsRead(uid id.ID, ids []id.ID) {
	p := d.island
	if p == nil || len(ids) == 0 {
		return
	}
	tags := make([]string, len(ids))
	for i, did := range ids {
		tags[i] = uid.String() + ":" + did.String()
	}
	if p.forgetRead(tags) && d.win != nil {
		d.win.Update(p.refresh)
	}
}

// forgetRead takes the notices tagged tags out of the inbox, saving once; it
// reports whether one was there.
func (p *islandPresenter) forgetRead(tags []string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ensureInbox(time.Now())
	gone := false
	for _, tag := range tags {
		if p.inbox.box.remove(tag) {
			gone = true
		}
	}
	if gone {
		p.inbox.changed = true
		p.saveInbox()
	}
	return gone
}

var _ appui.ReadNotificationSink = (*desktopDevice)(nil)

// readInApp marks the deliveries tagged tags read in the app, as the bell
// does. The inbox has already dropped them, so the sink's echo finds nothing.
func (p *islandPresenter) readInApp(tags []string) {
	byAccount := deliveryTags(tags)
	if len(byAccount) == 0 || p.d == nil || p.d.win == nil || p.d.app == nil {
		return
	}
	p.d.win.Update(func() {
		for uid, ids := range byAccount {
			p.d.app.ReadNotifications(uid, ids)
		}
	})
}

// deliveryTags groups the deliveries tagged tags (account:delivery) by
// account, once each; tags that are not a delivery's (an update, Ace's
// own) are skipped.
func deliveryTags(tags []string) map[id.ID][]id.ID {
	byAccount := map[id.ID][]id.ID{}
	for _, tag := range tags {
		account, delivery, ok := strings.Cut(tag, ":")
		if !ok {
			continue
		}
		uid, err1 := id.Parse(account)
		did, err2 := id.Parse(delivery)
		if err1 != nil || err2 != nil {
			continue
		}
		if !slices.Contains(byAccount[uid], did) {
			byAccount[uid] = append(byAccount[uid], did)
		}
	}
	return byAccount
}
