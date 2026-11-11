//go:build !js

package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/island"
)

// The inbox on the island: what lapsed is kept (keepLapsed), a click on
// Ace at rest opens the newest (openInbox), the details page through them
// (inboxPager), Tutup takes one out, Tutup semua all. p.mu guards it, as
// the machine.

// inboxState is the presenter's: the notices kept, whether the notice
// shown came from them, and the device's folder to keep them in.
type inboxState struct {
	box     islandInbox
	open    bool
	root    string
	loaded  bool
	changed bool
	saves   sync.WaitGroup // writes still going
}

// account is the signed-in account, as notices name it.
func (p *islandPresenter) account() string {
	p.d.mu.Lock()
	defer p.d.mu.Unlock()
	if p.d.activeUser == id.Nil {
		return ""
	}
	return p.d.activeUser.String()
}

// ensureInbox reads the inbox kept on the device, once. Caller holds p.mu.
func (p *islandPresenter) ensureInbox(now time.Time) {
	if p.inbox.loaded {
		return
	}
	p.inbox.loaded = true
	if p.d.root != "" {
		p.inbox.root = p.d.root
		p.inbox.box = loadIslandInbox(p.d.root, now)
	}
}

// saveInbox writes the inbox when it changed, off the UI thread. Caller
// holds p.mu.
func (p *islandPresenter) saveInbox() {
	if !p.inbox.changed || p.inbox.root == "" {
		return
	}
	p.inbox.changed = false
	box := islandInbox{Entries: append([]inboxEntry(nil), p.inbox.box.Entries...)}
	root := p.inbox.root
	p.inbox.saves.Add(1)
	go func() {
		defer p.inbox.saves.Done()
		_ = saveIslandInbox(root, box)
	}()
}

// keepLapsed moves what closed by itself into the inbox: a notice already
// there keeps its place. Caller holds p.mu.
func (p *islandPresenter) keepLapsed(now time.Time) {
	p.ensureInbox(now)
	for _, it := range p.m.Lapsed {
		x, ok := it.Value.(islandNotice)
		if !ok || p.inboxHas(x.n.Tag) {
			continue
		}
		if p.inbox.box.add(x.n, x.first) {
			p.inbox.changed = true
		}
	}
	p.m.Lapsed = nil
	if p.inbox.box.expire(now) {
		p.inbox.changed = true
	}
	if p.m.Current == nil {
		p.inbox.open = false
	}
	p.saveInbox()
}

func (p *islandPresenter) inboxHas(tag string) bool {
	for _, e := range p.inbox.box.Entries {
		if e.Notice.Tag == tag {
			return true
		}
	}
	return false
}

// inboxCount is how many notices wait for the account. Caller holds p.mu.
func (p *islandPresenter) inboxCount(account string) int {
	return len(p.inbox.box.of(account))
}

func inboxItem(e inboxEntry) island.Item {
	return island.Item{Tag: e.Notice.Tag, Value: islandNotice{n: e.Notice, count: 1, first: e.At, latest: e.Notice.Title}}
}

// openInbox shows account's newest notice kept, open in its details; it
// reports whether there was one. Caller holds p.mu (and not p.d.mu).
func (p *islandPresenter) openInbox(now time.Time, account string) bool {
	entries := p.inbox.box.of(account)
	if len(entries) == 0 {
		return false
	}
	p.inbox.open = true
	p.m.Show(inboxItem(entries[0]), now)
	return true
}

// inboxPosition is where the notice tagged tag is among account's kept
// (from 0), and how many there are; -1 when it is not one of them. Caller
// holds p.mu.
func (p *islandPresenter) inboxPosition(tag, account string) (int, int) {
	entries := p.inbox.box.of(account)
	for i, e := range entries {
		if e.Notice.Tag == tag {
			return i, len(entries)
		}
	}
	return -1, len(entries)
}

// inboxPage shows the notice delta places from the one shown.
func (p *islandPresenter) inboxPage(delta int) {
	account := p.account()
	p.mu.Lock()
	if cur := p.m.Current; cur != nil {
		entries := p.inbox.box.of(account)
		if i, n := p.inboxPosition(cur.Tag, account); i >= 0 && n > 0 {
			j := ((i+delta)%n + n) % n
			p.m.Show(inboxItem(entries[j]), time.Now())
		}
	}
	p.mu.Unlock()
	p.win.Update(func() {})
}

// handled takes the notice shown out of the inbox (Buka, Tunda, Tutup);
// with more of account's kept and the inbox open, the next one follows.
// Caller holds p.mu; it reports whether one followed.
func (p *islandPresenter) handled(now time.Time, account string) bool {
	p.ensureInbox(now)
	cur := p.m.Current
	if cur == nil {
		return false
	}
	i, _ := p.inboxPosition(cur.Tag, account)
	if p.inbox.box.remove(cur.Tag) {
		p.inbox.changed = true
		p.saveInbox()
	}
	if !p.inbox.open || i < 0 {
		return false
	}
	entries := p.inbox.box.of(account)
	if len(entries) == 0 {
		p.inbox.open = false
		return false
	}
	p.m.Show(inboxItem(entries[min(i, len(entries)-1)]), now)
	return true
}

// inboxPager is the details' line of the inbox: ‹ 3 dari 8 ›.
func (p *islandPresenter) inboxPager(c *ui.Context, x islandNotice) {
	account := p.account()
	p.mu.Lock()
	i, n := p.inboxPosition(x.n.Tag, account)
	open := p.inbox.open
	p.mu.Unlock()
	if !open || i < 0 || n < 1 {
		return
	}
	ui.Row(c.Key("inbox-pager")).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
		if n > 1 && p.islandAction(c, "‹", nil, false) {
			p.inboxPage(-1)
		}
		ui.Text(c, fmt.Sprintf("%d dari %d", i+1, n)).FontSize(12).FontFeatures("tnum").TextColor(islandMuted)
		if n > 1 && p.islandAction(c, "›", nil, false) {
			p.inboxPage(1)
		}
	})
}

// inboxBadge is "+N" on Ace at rest while notices wait.
func inboxBadge(c *ui.Context, n int, gw, gh float32) {
	if n <= 0 {
		return
	}
	label := fmt.Sprintf("+%d", n)
	if n > 99 {
		label = "+99"
	}
	ui.Box(c.Key("inbox-badge")).Absolute().Left(gw-34).Top(gh/2-8).Size(28, 16).Radius(8).Background(islandButton).Center().Children(func() {
		ui.Text(c, label).FontSize(10).FontWeight(600).FontFeatures("tnum").TextColor(islandText)
	})
}

// NotifyQuiet is quiet hours' way in: the deliveries go to the inbox,
// without a banner, a toast or the bell (appui.QuietNotificationSink).
func (d *desktopDevice) NotifyQuiet(uid id.ID, ns []api.Delivery) error {
	p := d.island
	if p == nil || !d.notificationPrefs.Load().(devicePreferences).IslandEnabled {
		return errIslandUnavailable
	}
	now := time.Now()
	p.mu.Lock()
	p.ensureInbox(now)
	for _, n := range ns {
		x := deliveryNotice(uid, n)
		if p.inbox.box.add(x, now) {
			p.inbox.changed = true
		}
	}
	p.saveInbox()
	p.mu.Unlock()
	if d.win != nil {
		d.win.Update(func() {
			if p.win != nil {
				p.win.Update(func() {})
			} else {
				p.resume()
			}
		})
	}
	return nil
}

var _ appui.QuietNotificationSink = (*desktopDevice)(nil)

// inboxShowing tells whether the notice tagged tag is shown from the
// inbox.
func (p *islandPresenter) inboxShowing(tag string) bool {
	account := p.account()
	p.mu.Lock()
	defer p.mu.Unlock()
	i, _ := p.inboxPosition(tag, account)
	return p.inbox.open && i >= 0
}

// islandKindIcon is a notice kind's icon in its accent: always shown, so a
// ping, a reminder and a task never look alike. Ace speaking has none.
func islandKindIcon(c *ui.Context, kind string, size float32) {
	if kind == appui.NoticeAce {
		return
	}
	ui.Icon(c, appui.NoticeGlyph(kind)).Size(size, size).Shrink(0).TextColor(islandGlyphColor("A", kind))
}
