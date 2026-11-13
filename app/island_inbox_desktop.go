//go:build !js

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/update"
)

// Ace's inbox (rc4 feedback): a notice nobody acted on (Buka, Tunda,
// Tutup) does not vanish when the island folds. It waits here, the pill
// shows "+N", and a click pages through them. The inbox is the device's,
// kept across restarts in island-inbox.json, per account, newest first, at
// most islandInboxCap, for islandInboxKeep.

const (
	islandInboxCap  = 50
	islandInboxKeep = 7 * 24 * time.Hour
)

// inboxEntry is one notice kept, and when it came.
type inboxEntry struct {
	Notice appui.Notice `json:"notice"`
	At     time.Time    `json:"at"`
}

// islandInbox is the notices kept, newest first.
type islandInbox struct {
	Entries []inboxEntry `json:"entries"`
}

// inboxKeeps tells whether a notice belongs in the inbox: one of an
// account's deliveries, or a version on offer newer than this Space; not
// Ace speaking or a test.
func inboxKeeps(n appui.Notice) bool {
	if n.AccountID == "" || n.Tag == "" || n.Kind == appui.NoticeAce || n.Kind == appui.NoticeTest || n.Kind == appui.NoticeEmergency {
		return false
	}
	if n.Kind == "update" {
		v, ok := strings.CutPrefix(n.Tag, "update:")
		return ok && update.Compare(v, appVersion()) > 0
	}
	return true
}

// add keeps n as of at: newest first, once per tag, within the cap.
func (b *islandInbox) add(n appui.Notice, at time.Time) bool {
	if !inboxKeeps(n) {
		return false
	}
	b.remove(n.Tag)
	b.Entries = slices.Insert(b.Entries, 0, inboxEntry{Notice: n, At: at})
	if len(b.Entries) > islandInboxCap {
		b.Entries = b.Entries[:islandInboxCap]
	}
	return true
}

// remove drops the notice tagged tag; it reports whether there was one.
func (b *islandInbox) remove(tag string) bool {
	n := len(b.Entries)
	b.Entries = slices.DeleteFunc(b.Entries, func(e inboxEntry) bool { return e.Notice.Tag == tag })
	return len(b.Entries) != n
}

// expire drops what is older than islandInboxKeep; it reports a change.
func (b *islandInbox) expire(now time.Time) bool {
	n := len(b.Entries)
	b.Entries = slices.DeleteFunc(b.Entries, func(e inboxEntry) bool { return now.Sub(e.At) > islandInboxKeep })
	return len(b.Entries) != n
}

// clear drops account's notices ("Tutup semua").
func (b *islandInbox) clear(account string) bool {
	n := len(b.Entries)
	b.Entries = slices.DeleteFunc(b.Entries, func(e inboxEntry) bool { return e.Notice.AccountID == account })
	return len(b.Entries) != n
}

// of is account's notices, newest first.
func (b *islandInbox) of(account string) []inboxEntry {
	var out []inboxEntry
	for _, e := range b.Entries {
		if account != "" && e.Notice.AccountID == account {
			out = append(out, e)
		}
	}
	return out
}

func islandInboxPath(root string) string { return filepath.Join(root, "island-inbox.json") }

// loadIslandInbox reads the inbox kept on the device, as now: a missing or
// broken file is an empty inbox.
func loadIslandInbox(root string, now time.Time) islandInbox {
	var b islandInbox
	if data, err := os.ReadFile(islandInboxPath(root)); err == nil {
		_ = json.Unmarshal(data, &b)
	}
	b.Entries = slices.DeleteFunc(b.Entries, func(e inboxEntry) bool { return !inboxKeeps(e.Notice) })
	b.expire(now)
	if len(b.Entries) > islandInboxCap {
		b.Entries = b.Entries[:islandInboxCap]
	}
	return b
}

// saveIslandInbox writes the inbox to the device.
func saveIslandInbox(root string, b islandInbox) error {
	data, err := json.Marshal(b)
	if err != nil {
		return err
	}
	return atomicDeviceFile(islandInboxPath(root), data)
}
