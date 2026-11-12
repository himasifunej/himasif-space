package appui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// The bell (Notifikasi) shows one row per thing that happened. The server
// keeps a delivery row per event, recipient and channel, so a meeting
// change sent to the device and to WhatsApp came back twice; a task update
// is also kept as a task notification beside its ping, a third time; and a
// task's reminder slots (H-2, H-1, the day, overdue days) are separate
// events of one task. bellItems folds them: an event's channels into one
// row, an update's task notification into its ping's row, and the
// repeated reminders of one source into one row with a count.

// bellPage is how many rows the bell shows before "Muat lagi".
const bellPage = 10

// bellItem is one row of the bell.
type bellItem struct {
	key               string
	title, body, zone string
	// at is when the newest of what it folds happened.
	at time.Time
	// count is how many times it happened: events, and task
	// notifications without a ping of their own.
	count      int
	deliveries []api.Delivery
	notes      []api.Notification
	upcoming   bool
}

// ReadNotificationSink hears the deliveries read in the app (the bell, Buka,
// Tandai semua dibaca, another device), for Ace's inbox to drop them too:
// the bell and the island share the server's read state.
type ReadNotificationSink interface {
	NotificationsRead(uid id.ID, ids []id.ID)
}

// bellState is the bell's own: how many rows it shows.
type bellState struct {
	limit int
}

// bellKey names what a delivery or a task notification is about: its
// subject and its words.
func bellKey(subject, title, body string) string {
	return subject + "\x00" + title + "\x00" + body
}

func deliverySubject(d api.Delivery) string {
	if d.RelatedID != nil && d.RelatedKind != "" {
		return d.RelatedKind + ":" + d.RelatedID.String()
	}
	return d.SourceKind + ":" + d.SourceID.String()
}

// bellItems folds the deliveries and the task notifications into the
// bell's rows, newest first; what is scheduled later than now comes
// last, as upcoming.
func bellItems(deliveries []api.Delivery, notes []api.Notification, now time.Time) []bellItem {
	byKey := map[string]int{}
	var out []bellItem
	events := map[string]map[id.ID]time.Time{}
	item := func(key, title, body, zone string, upcoming bool) *bellItem {
		if upcoming {
			key = "later\x00" + key
		}
		i, ok := byKey[key]
		if !ok {
			i = len(out)
			byKey[key] = i
			out = append(out, bellItem{key: key, title: title, body: body, zone: zone, upcoming: upcoming})
			events[key] = map[id.ID]time.Time{}
		}
		return &out[i]
	}
	// An event comes when its delivery to the device does: a WhatsApp
	// copy follows it, as when the device's is put off.
	deviceAt := map[id.ID]time.Time{}
	for _, d := range deliveries {
		if d.Channel == "device" && d.EventID != id.Nil {
			deviceAt[d.EventID] = d.AvailableAt
		}
	}
	for _, d := range deliveries {
		if d.Status == "cancelled" {
			continue
		}
		available := d.AvailableAt
		if at, ok := deviceAt[d.EventID]; ok {
			available = at
		}
		upcoming := available.After(now)
		it := item(bellKey(deliverySubject(d), d.Title, d.Body), d.Title, d.Body, d.TimeZone, upcoming)
		it.deliveries = append(it.deliveries, d)
		event := d.EventID
		if event == id.Nil {
			event = d.ID
		}
		events[it.key][event] = d.ScheduledAt
		if when := bellDeliveryTime(d); when.After(it.at) {
			it.at = when
		}
	}
	for _, n := range notes {
		it := item(bellKey("task:"+n.TaskID.String(), n.Title, n.Body), n.Title, n.Body, timeutil.DefaultZoneName, false)
		it.notes = append(it.notes, n)
		if n.CreatedAt.After(it.at) {
			it.at = n.CreatedAt
		}
		// A task update is a ping and a task notification made at once:
		// one time, unless no ping came within the minute.
		paired := false
		for _, at := range events[it.key] {
			if diff := n.CreatedAt.Sub(at); diff > -time.Minute && diff < time.Minute {
				paired = true
				break
			}
		}
		if !paired {
			events[it.key][n.ID] = n.CreatedAt
		}
	}
	for i := range out {
		out[i].count = max(len(events[out[i].key]), 1)
	}
	slices.SortStableFunc(out, func(a, b bellItem) int {
		if a.upcoming != b.upcoming {
			if a.upcoming {
				return 1
			}
			return -1
		}
		if a.upcoming {
			return a.at.Compare(b.at) // the soonest first
		}
		return b.at.Compare(a.at)
	})
	return out
}

// bellDeliveryTime is when a delivery came: its slot, or when a snooze
// brought it back.
func bellDeliveryTime(d api.Delivery) time.Time {
	if d.SnoozedUntil != nil && d.SnoozedUntil.After(d.ScheduledAt) {
		return *d.SnoozedUntil
	}
	return d.ScheduledAt
}

// unread tells whether the row waits for the user: a delivery to this
// device not yet seen, or a task notification not yet read. WhatsApp's
// copies are read on WhatsApp; what is upcoming has not come yet.
func (b bellItem) unread() bool {
	if b.upcoming {
		return false
	}
	return len(b.unseen()) > 0 || len(b.unreadNotes()) > 0
}

// unseen are the row's device deliveries not yet seen.
func (b bellItem) unseen() []id.ID {
	var out []id.ID
	for _, d := range b.deliveries {
		if d.Channel == "device" && d.SeenAt == nil {
			out = append(out, d.ID)
		}
	}
	return out
}

func (b bellItem) unreadNotes() []id.ID {
	var out []id.ID
	for _, n := range b.notes {
		if n.ReadAt == nil {
			out = append(out, n.ID)
		}
	}
	return out
}

// lead is the delivery that opens the row: the newest to the device, else
// the newest at all.
func (b bellItem) lead() (api.Delivery, bool) {
	var best api.Delivery
	found := false
	for _, d := range b.deliveries {
		better := !found || d.Channel == "device" && best.Channel != "device" || d.Channel == best.Channel && bellDeliveryTime(d).After(bellDeliveryTime(best))
		if better {
			best, found = d, true
		}
	}
	return best, found
}

// snoozable tells whether Tunda 15 menit applies: the row has a delivery to
// this device that came.
func (b bellItem) snoozable() bool {
	return !b.upcoming && slices.ContainsFunc(b.deliveries, func(d api.Delivery) bool { return d.Channel == "device" })
}

// status is a line on how the row was delivered, when it says something:
// a failure, or that only WhatsApp carried it.
func (b bellItem) status() string {
	var device, whatsapp bool
	for _, d := range b.deliveries {
		switch d.Status {
		case "failed", "unknown":
			if d.Channel == "device" {
				msg := deliveryStatusLabel(d.Status)
				if d.LastError != "" {
					msg += ": " + d.LastError
				}
				return msg
			}
		}
		device = device || d.Channel == "device"
		whatsapp = whatsapp || d.Channel == "whatsapp"
	}
	if whatsapp && !device && len(b.notes) == 0 {
		return "Dikirim lewat WhatsApp"
	}
	return ""
}

func (b bellItem) whatsApp() bool {
	return slices.ContainsFunc(b.deliveries, func(d api.Delivery) bool { return d.Channel == "whatsapp" })
}

// bell is the bell's rows now.
func (a *App) bell(now time.Time) []bellItem {
	return bellItems(a.deliveries.items, a.tasks.notifications, now)
}

// bellUnread is how many rows wait.
func bellUnread(items []bellItem) int {
	n := 0
	for _, it := range items {
		if it.unread() {
			n++
		}
	}
	return n
}

// markBellRead marks the rows read: their deliveries to this device seen
// and their task notifications read, here at once, then on the server, and
// in Ace's inbox.
func (a *App) markBellRead(items []bellItem) {
	var deliveries, notes []id.ID
	for _, it := range items {
		deliveries = append(deliveries, it.unseen()...)
		notes = append(notes, it.unreadNotes()...)
	}
	a.markDeliveriesSeen(deliveries)
	a.markNotesRead(notes)
}

// markDeliveriesSeen marks deliveries seen, the bell's rows first.
func (a *App) markDeliveriesSeen(ids []id.ID) {
	if len(ids) == 0 {
		return
	}
	now := time.Now()
	for i := range a.deliveries.items {
		if d := &a.deliveries.items[i]; d.SeenAt == nil && slices.Contains(ids, d.ID) {
			d.SeenAt = &now
		}
	}
	a.forgetReadNotices(ids)
	if a.offlineMode() {
		for _, did := range ids {
			a.queueDeliveryResponse(did, "seen", 0)
		}
		return
	}
	svc, ok := a.deliveryService()
	if !ok {
		return
	}
	gen := a.gen
	a.async(func() func() {
		var err error
		for _, did := range ids {
			if e := svc.RespondDelivery(context.Background(), did, api.DeliveryResponse{Action: "seen"}); e != nil && err == nil {
				err = e
			}
		}
		return func() {
			if gen != a.gen {
				return
			}
			if err != nil {
				a.deliveries.err = a.failed(err)
				a.PollDeliveries()
			}
		}
	})
}

// markNotesRead marks task notifications read.
func (a *App) markNotesRead(ids []id.ID) {
	if len(ids) == 0 {
		return
	}
	now := time.Now()
	for i := range a.tasks.notifications {
		if n := &a.tasks.notifications[i]; n.ReadAt == nil && slices.Contains(ids, n.ID) {
			n.ReadAt = &now
		}
	}
	s, ok := a.taskService()
	if !ok {
		return
	}
	gen := a.gen
	a.async(func() func() {
		var err error
		for _, n := range ids {
			if e := s.ReadNotification(context.Background(), n); e != nil && err == nil {
				err = e
			}
		}
		return func() {
			if gen != a.gen || err == nil {
				return
			}
			a.tasks.notificationsErr = a.taskFailure(err)
			a.loadTaskNotifications()
		}
	})
}

// snoozeBell puts the row's deliveries to this device off 15 minutes: it
// leaves the list at once, and comes back then.
func (a *App) snoozeBell(it bellItem) {
	later := time.Now().Add(15 * time.Minute)
	var ids []id.ID
	for i := range a.deliveries.items {
		d := &a.deliveries.items[i]
		if d.Channel == "device" && slices.ContainsFunc(it.deliveries, func(x api.Delivery) bool { return x.ID == d.ID }) {
			d.AvailableAt, d.SnoozedUntil, d.SeenAt = later, &later, nil
			ids = append(ids, d.ID)
		}
	}
	a.markNotesRead(it.unreadNotes())
	a.forgetReadNotices(ids)
	for _, did := range ids {
		a.RespondNotification(did, "snooze", 15)
	}
}

// openBell opens what the row is about, and marks it read.
func (a *App) openBell(it bellItem) {
	a.markBellRead([]bellItem{it})
	if d, ok := it.lead(); ok {
		a.OpenNotification(d)
		return
	}
	if len(it.notes) > 0 {
		a.chrome.notificationsOpen = false
		a.openTaskNotification(it.notes[0])
	}
}

// forgetReadNotices tells Ace's inbox the deliveries read.
func (a *App) forgetReadNotices(ids []id.ID) {
	if len(ids) == 0 {
		return
	}
	if sink, ok := a.Notifications.(ReadNotificationSink); ok {
		sink.NotificationsRead(a.user.ID, ids)
	}
}

// bellButton is the bell in the topbar or the phone's header, with the
// count of rows waiting.
func (a *App) bellButton(c *ui.Context, key string, size, icon float32, unread int) ui.Element {
	t := c.Theme()
	label := "Notifikasi"
	tip := label
	if unread > 0 {
		tip = fmt.Sprintf("Notifikasi · %d belum dibaca", unread)
	}
	b := ui.ButtonBase(c.Key(key)).Label(label).Tooltip(tip).Size(size, size).Shrink(0).Center().Radius(8).TextColor(t.TextMuted).Transition(motionState)
	switch {
	case b.Pressed():
		b.Background(t.SurfacePressed).TextColor(t.Text)
	case b.Hovered() || a.chrome.notificationsOpen:
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		ui.Icon(c, chromeIcon("bell")).Size(icon, icon)
		if unread > 0 {
			text := fmt.Sprint(unread)
			if unread > 9 {
				text = "9+"
			}
			ui.Box(c.Key("bell-count")).Absolute().Top(size/2-icon/2-6).Left(size/2+1).MinWidth(16).Height(16).Padding(0, 4).Radius(8).Center().Background(t.Danger).Border(2, t.Background).
				Transition(ui.ElementTransition{Duration: motionDuration, Ease: motionEase, Enter: &ui.Motion{Opacity: 0, Scale: .6}, Exit: &ui.Motion{Opacity: 0, Scale: .6}}).
				Children(func() {
					ui.Text(c, text).FontSize(9).FontWeight(700).FontFeatures("tnum").TextColor(ui.RGB(255, 255, 255))
				})
		}
	})
	if b.Clicked() {
		open := !a.chrome.notificationsOpen
		a.chrome.membersOpen, a.chrome.searchOpen, a.chrome.profileOpen, a.chrome.periodOpen = false, false, false, false
		a.chrome.notificationsOpen = open
		if open {
			a.chrome.bell.limit = bellPage
			a.PollDeliveries()
			a.loadTaskNotifications()
		}
	}
	return b
}

// bellPanel is the bell's popover: a header with the count and Tandai
// semua dibaca, the rows by day in a region of their own that scrolls, and
// Muat lagi after a page. Nothing shows outside it.
func (a *App) bellPanel(c *ui.Context, anchor ui.Element, width float32) {
	_, wh := c.Size()
	maxH := min(float32(560), wh-72)
	ui.PopoverBase(c.Key("bell-panel"), anchor, &a.chrome.notificationsOpen, func(panel ui.Element) {
		t := c.Theme()
		popPanel(panel, 1).Label("Panel notifikasi").Margin(6, 0, 0, 0).Width(width).MaxHeight(maxH).Clip().Radius(12).Background(t.Surface).Border(1, t.Border).Shadow(0, 12, 32, 0, ui.RGBA(0, 0, 0, .28))
		if !a.tasks.notificationsLoaded && !a.tasks.notificationsBusy && a.tasks.notificationsErr == "" {
			a.loadTaskNotifications()
		}
		items := a.bell(c.Now())
		unread := bellUnread(items)
		a.bellHeader(c, items, unread)
		ui.Box(c).FillWidth().Height(1).Shrink(0).Background(t.Border)
		ui.Scroll(c.Key("bell-scroll")).FillWidth().Grow(1).Shrink(1).MinHeight(0).Children(func() {
			ui.Column(c.Key("bell-rows")).FillWidth().Padding(6, 8, 10, 8).Gap(2).Children(func() {
				a.bellRows(c, items)
			})
		})
	}).AttachTo(anchor, ui.AnchorBottomRight, ui.AnchorTopRight)
}

func (a *App) bellHeader(c *ui.Context, items []bellItem, unread int) {
	t := c.Theme()
	ui.Row(c.Key("bell-head")).FillWidth().Shrink(0).AlignItems(ui.Center).Gap(8).Padding(12, 12, 10, 14).Children(func() {
		ui.Text(c, "Notifikasi").FontSize(15).Bold()
		if unread > 0 {
			ui.Box(c.Key("bell-unread")).Shrink(0).MinWidth(20).Height(20).Padding(0, 7).Radius(10).Center().Background(accentSoft(t)).Label(fmt.Sprintf("%d belum dibaca", unread)).Tooltip(fmt.Sprintf("%d belum dibaca", unread)).Transition(motionState).Children(func() {
				ui.Text(c, fmt.Sprint(unread)).FontSize(11).FontWeight(650).FontFeatures("tnum").TextColor(t.Accent).MaxLines(1)
			})
		}
		ui.Box(c).Grow(1)
		if bellAction(c, "Tandai semua dibaca", "check", bellQuiet, unread == 0, "Semua sudah dibaca").Clicked() {
			a.markBellRead(items)
		}
		refresh := bellAction(c, "Perbarui notifikasi", "refresh", bellIconOnly, a.deliveries.loading || a.tasks.notificationsBusy, "Memuat…")
		if refresh.Clicked() {
			a.PollDeliveries()
			a.loadTaskNotifications()
		}
	})
	if msg := a.deliveries.notificationErr; msg != "" {
		ui.Text(c, msg).FontSize(12).TextColor(t.Danger).Padding(0, 14, 8, 14)
	}
	for _, msg := range []string{a.deliveries.err, a.tasks.notificationsErr} {
		if msg != "" {
			muted(c, msg).Padding(0, 14, 8, 14)
		}
	}
}

// bellRows draws the rows by day: a page, then Muat lagi.
func (a *App) bellRows(c *ui.Context, items []bellItem) {
	t := c.Theme()
	if len(items) == 0 {
		ui.Column(c.Key("bell-empty")).FillWidth().AlignItems(ui.Center).Gap(6).Padding(28, 12).Children(func() {
			ui.Icon(c, chromeIcon("bell")).Size(22, 22).TextColor(faint(t))
			ui.Text(c, "Belum ada notifikasi").FontSize(13).FontWeight(600)
			muted(c, "Pengingat, ping, dan pembaruan tugas muncul di sini.")
		})
		return
	}
	limit := a.chrome.bell.limit
	if limit <= 0 {
		limit = bellPage
	}
	now := time.Now()
	day := ""
	for i, it := range items {
		if i >= limit {
			break
		}
		label := pingDayLabel(it.at, now, timeutil.DefaultZoneName)
		if it.upcoming {
			label = "Terjadwal"
		}
		if label != day {
			day = label
			ui.Text(c.Key("day-"+label), strings.ToUpper(label)).FontSize(10).FontWeight(600).TextColor(faint(t)).Padding(10, 6, 4, 6).Transition(motionRow)
		}
		a.bellRow(c, it)
	}
	if rest := len(items) - limit; rest > 0 {
		b := bellAction(c.Key("bell-more"), fmt.Sprintf("Muat lagi · %d lagi", rest), "down", bellQuiet, false, "")
		b.AlignSelf(ui.Center).Margin(8, 0, 0, 0)
		if b.Clicked() {
			a.chrome.bell.limit = limit + bellPage
		}
	}
}

func (a *App) bellRow(c *ui.Context, it bellItem) {
	t := c.Theme()
	unread := it.unread()
	row := ui.Column(c.Key(it.key)).FillWidth().Gap(6).Padding(10, 10, 10, 10).Radius(10).Transition(motionRow)
	if unread {
		row.Background(spaceMix(t.SurfaceHover, t.Surface, .35))
	}
	row.Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Start).Children(func() {
			// The dot says unread; read rows keep its room.
			dot := ui.Box(c.Key("dot")).Size(8, 8).Radius(4).Shrink(0).Margin(5, 0, 0, 0).Transition(motionState)
			if unread {
				dot.Background(t.Accent)
			} else {
				dot.Background(ui.RGBA(t.Accent.R, t.Accent.G, t.Accent.B, 0))
			}
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
					title := ui.Text(c, it.title).FontSize(13).MaxLines(2).MinWidth(0).Shrink(1)
					if unread {
						title.FontWeight(650)
					} else {
						title.TextColor(t.TextMuted)
					}
					if it.count > 1 {
						ui.Box(c.Key("count")).Shrink(0).Padding(1, 6).Radius(999).Background(t.SurfaceHover).Border(1, t.Border).Tooltip(fmt.Sprintf("%d kali", it.count)).Children(func() {
							ui.Text(c, fmt.Sprintf("×%d", it.count)).FontSize(10).FontWeight(600).FontFeatures("tnum").TextColor(t.TextMuted)
						})
					}
				})
				if it.body != "" {
					muted(c, it.body).MaxLines(2)
				}
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					loc, _ := timeutil.LoadZone(it.zone)
					if loc == nil {
						loc = time.Local
					}
					ui.Text(c, it.at.In(loc).Format("15.04")).FontSize(11).FontFeatures("tnum").TextColor(faint(t))
					if it.whatsApp() {
						ui.Text(c, "· WhatsApp").FontSize(11).TextColor(faint(t))
					}
					if s := it.status(); s != "" {
						ui.Text(c, "· "+s).FontSize(11).TextColor(t.Danger).MaxLines(1).MinWidth(0)
					}
				})
			})
		})
		ui.Row(c).Wrap().Gap(5).Padding(0, 0, 0, 16).Children(func() {
			if bellAction(c, "Buka", "open", bellPrimary, false, "").Clicked() {
				a.openBell(it)
			}
			read := bellAction(c, "Tandai dibaca", "check", bellQuiet, !unread, "Sudah dibaca")
			if read.Clicked() {
				a.markBellRead([]bellItem{it})
			}
			why := "Hanya untuk pengingat di perangkat ini"
			if it.upcoming {
				why = "Belum waktunya"
			}
			if bellAction(c, "Tunda 15 menit", "clock", bellQuiet, !it.snoozable(), why).Clicked() {
				a.snoozeBell(it)
			}
		})
	})
}

// The kinds of the bell's buttons.
type bellActionKind int

const (
	bellQuiet bellActionKind = iota
	bellPrimary
	bellIconOnly
)

// bellAction is a button of the bell: an icon and its label (or the icon
// alone, labeled), at rest, hovered, pressed, focused and disabled each its
// own, easing between them. A disabled one says why in its tooltip.
func bellAction(c *ui.Context, label, icon string, kind bellActionKind, disabled bool, why string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key("act-" + label)).Label(label).Row().AlignItems(ui.Center).Gap(5).Height(28).Radius(7).Shrink(0).Disabled(disabled).Transition(motionState)
	if kind == bellIconOnly {
		b.Width(28).Center()
	} else {
		b.Padding(0, 9, 0, 7)
	}
	tip := label
	if disabled && why != "" {
		tip = why
	}
	b.Tooltip(tip)
	bg, border, fg := ui.RGBA(0, 0, 0, 0), t.Border, t.TextMuted
	switch kind {
	case bellPrimary:
		bg, border, fg = accentSoft(t), ui.RGBA(t.Accent.R, t.Accent.G, t.Accent.B, .28), t.Accent
	case bellIconOnly:
		border = ui.RGBA(0, 0, 0, 0)
	}
	switch {
	case disabled:
		bg, fg = ui.RGBA(0, 0, 0, 0), faint(t)
		border = spaceMix(t.Border, t.Surface, .45)
	case b.Pressed():
		if kind == bellPrimary {
			bg = ui.RGBA(t.Accent.R, t.Accent.G, t.Accent.B, .3)
		} else {
			bg, fg = t.SurfacePressed, t.Text
		}
	case b.Hovered():
		if kind == bellPrimary {
			bg = ui.RGBA(t.Accent.R, t.Accent.G, t.Accent.B, .2)
		} else {
			bg, fg = t.SurfaceHover, t.Text
		}
	}
	b.Background(bg).Border(1, border).TextColor(b.AnimateColor("fg", fg, motionStateAnim))
	b.FocusRing(b.FocusVisible())
	b.Children(func() {
		ui.Icon(c, chromeIcon(icon)).Size(13, 13).Shrink(0)
		if kind != bellIconOnly {
			ui.Text(c, label).FontSize(12).FontWeight(550).MaxLines(1)
		}
	})
	return b
}

// ReadNotifications marks uid's deliveries ids read from Ace's inbox
// (Tandai dibaca, Tandai semua dibaca): the same seen the bell sends, so
// both show them read. Another account's are left alone.
func (a *App) ReadNotifications(uid id.ID, ids []id.ID) {
	if !a.IsCurrentAccount(uid) {
		return
	}
	a.markDeliveriesSeen(ids)
}
