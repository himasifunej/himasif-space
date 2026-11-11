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

// The Ping tab as canvas "Ping" option C (Tahap 7c): a one-line send bar,
// "Belum melihat" per member, and the history per day with filters,
// Ping ulang and pages of 50 (GET …/pings?before=&limit=50&filter=).

var pingFilters = []struct{ key, label string }{{"", "Semua"}, {"unseen", "Belum dilihat"}, {"failed", "Gagal"}}

// pingComposerState is the shared composer's state, reset for a new Space.
func (a *App) pingComposerState(sp *api.Space) *notificationSettingsState {
	st := &a.notificationSettingsState
	if st.period != sp.PeriodID || st.space != sp.ID {
		a.resetPingState(sp, st.division)
	}
	return st
}

// pingPerson is one recipient of a ping: seen on some channel, failed on
// some channel.
type pingPerson struct {
	ref          api.PersonRef
	seen, failed bool
}

func pingPeople(p api.PingRecord) []pingPerson {
	var out []pingPerson
	index := map[id.ID]int{}
	for _, r := range p.Recipients {
		i, ok := index[r.Recipient.ID]
		if !ok {
			i = len(out)
			index[r.Recipient.ID] = i
			out = append(out, pingPerson{ref: r.Recipient})
		}
		if r.SeenAt != nil {
			out[i].seen = true
		}
		if r.Status == "failed" {
			out[i].failed = true
		}
	}
	return out
}

// pingUnseen are the recipients who saw the ping on no channel.
func pingUnseen(p api.PingRecord) []id.ID {
	var out []id.ID
	for _, person := range pingPeople(p) {
		if !person.seen {
			out = append(out, person.ref.ID)
		}
	}
	return out
}

// pingDayLabel is "Hari ini · Sen 9 Nov", "Kemarin · Min 8 Nov" or "Jum 6 Nov".
func pingDayLabel(t, now time.Time, zone string) string {
	loc, err := timeutil.LoadZone(zone)
	if err != nil || loc == nil {
		loc = time.Local
	}
	day, today := timeutil.DateOf(t.In(loc)), timeutil.DateOf(now.In(loc))
	label := fmt.Sprintf("%s %d %s", shortDays[t.In(loc).Weekday()], day.Day, monthName(day.Month)[:3])
	switch {
	case day == today:
		return "Hari ini · " + label
	case day == today.AddDays(-1):
		return "Kemarin · " + label
	}
	return label
}

// pingLoad (re)loads the shown page of the history.
func (a *App) pingLoad() {
	st := &a.pingHistory
	svc, ok := a.pingHistoryService()
	if !ok || st.loading {
		return
	}
	sid, gen, filter := st.space, a.gen, st.filter
	var before time.Time
	if st.page > 0 && st.page < len(st.cursors) {
		before = st.cursors[st.page]
	}
	st.loading = true
	a.async(func() func() {
		items, err := svc.Pings(context.Background(), sid, before, pingHistoryLimit, filter)
		return func() {
			if gen != a.gen || a.pingHistory.space != sid || a.pingHistory.filter != filter {
				return
			}
			st := &a.pingHistory
			st.loading, st.loaded = false, true
			if uiPreferencesMissing(err) {
				st.missing = true
				return
			}
			if err != nil {
				st.loaded = false
				st.err = a.failed(err)
				return
			}
			st.items, st.missing = items, false
			st.hasMore = len(items) == pingHistoryLimit
		}
	})
}

// spacePingPage is the Ping tab.
func (a *App) spacePingPage(c *ui.Context) {
	sp := a.selectedSpace()
	a.ensurePingHistory()
	phone := isMobile(c)
	// Canvas C: the page does not scroll; the history scrolls inside its
	// panel and its pager stays in view.
	body := ui.Column(c.Key("ping-page")).Fill().MinWidth(0).MinHeight(0).Padding(24).Gap(16)
	if phone {
		body.Padding(12, 16).Gap(12)
	}
	body.Children(func() {
		if !phone {
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "Ping").FontSize(20).Bold()
				muted(c, "Tulis di bilah atas; riwayat dikelompokkan per hari, dengan siapa yang belum melihat.").FontSize(13)
			})
		}
		if canPing(sp) {
			a.pingSendBar(c, sp, phone)
		}
		split := ui.Column(c.Key("ping-split")).FillWidth().Grow(1).MinWidth(0).MinHeight(0).Gap(16)
		wide := !phone && split.Bounds().W >= 960
		if wide {
			split.Row().AlignItems(ui.Stretch)
		}
		split.Children(func() {
			if wide && canPing(sp) {
				ui.Column(c).Width(340).Shrink(0).MinHeight(0).Children(func() { a.pingUnseenPanel(c, sp) })
			}
			right := ui.Column(c).FillWidth().MinWidth(0).MinHeight(0).Grow(1)
			if wide {
				right.Basis(0)
			}
			right.Children(func() { a.pingTimeline(c, sp, phone) })
		})
	})
}

// pingSendBar is the one-line composer: recipients, message, source,
// channels and Kirim ping. On a phone (canvas C · HP): the message, then
// one picker "2 penerima · Perangkat" holding recipients, source and
// channels, beside Kirim.
func (a *App) pingSendBar(c *ui.Context, sp *api.Space, phone bool) {
	a.ensureMeetings()
	t := c.Theme()
	st := a.pingComposerState(sp)
	canSend := !st.busy && a.conn == connOnline && len(st.recipients) > 0 && (st.pingKind == "general" || st.pingTarget != id.Nil)
	if phone {
		ui.Column(c.Key("ping-bar")).FillWidth().MinWidth(0).Padding(12).Gap(8).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.TextInput(c.Key("ping-msg"), &st.pingMessage).Label("Pesan ping").Placeholder("Mohon perbarui progres tugas hari ini.").FillWidth()
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				label := a.pingRecipientsLabel(st) + " · " + pingChannelsLabel(st)
				pick := spaceButton(c.Key("ping-pick"), label, "users", a.chrome.pingWhoOpen).Grow(1).MinWidth(0).MinHeight(44)
				if pick.Clicked() {
					a.chrome.pingWhoOpen = !a.chrome.pingWhoOpen
					a.ensureTasks(false)
				}
				popover(c, pick, &a.chrome.pingWhoOpen, 320, func() {
					a.pingRecipientsList(c, st)
					a.pingSourceList(c, st)
					ui.Text(c, "Kanal").FontSize(11).TextColor(faint(t))
					pingChannels(c, st)
				})
				if primarySpaceButton(c.Key("ping-send"), "Kirim", "send").MinHeight(44).Shrink(0).Disabled(!canSend).Clicked() {
					a.sendManualPing()
				}
			})
		})
		if st.err != "" {
			muted(c, st.err).FontSize(12)
		}
		return
	}
	ui.Row(c.Key("ping-bar")).FillWidth().MinWidth(0).Padding(12).Gap(10).AlignItems(ui.Center).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
		who := spaceButton(c.Key("ping-who"), a.pingRecipientsLabel(st), "users", a.chrome.pingWhoOpen).Shrink(0)
		if who.Clicked() {
			a.chrome.pingWhoOpen = !a.chrome.pingWhoOpen
		}
		popover(c, who, &a.chrome.pingWhoOpen, 280, func() { a.pingRecipientsList(c, st) })
		ui.TextInput(c.Key("ping-msg"), &st.pingMessage).Label("Pesan ping").Placeholder("Mohon perbarui progres tugas hari ini.").Grow(1).MinWidth(0)
		src := spaceButton(c.Key("ping-src"), a.pingSourceLabel(st), map[string]string{"general": "bell", "task": "task", "meeting": "calendar"}[st.pingKind], a.chrome.pingSrcOpen).Shrink(0).MaxWidth(260)
		if src.Clicked() {
			a.chrome.pingSrcOpen = !a.chrome.pingSrcOpen
			a.ensureTasks(false)
		}
		popover(c, src, &a.chrome.pingSrcOpen, 300, func() { a.pingSourceList(c, st) })
		pingChannels(c, st)
		if primarySpaceButton(c.Key("ping-send"), "Kirim ping", "send").Shrink(0).Disabled(!canSend).Clicked() {
			a.sendManualPing()
		}
	})
	if st.err != "" {
		muted(c, st.err).FontSize(12)
	}
}

func (a *App) pingRecipientsLabel(st *notificationSettingsState) string {
	if len(st.recipients) == 0 {
		return "Pilih penerima"
	}
	return fmt.Sprintf("%d penerima", len(st.recipients))
}

func pingChannelsLabel(st *notificationSettingsState) string {
	switch {
	case st.device && st.whatsapp:
		return "Perangkat + WA"
	case st.whatsapp:
		return "WhatsApp"
	case st.device:
		return "Perangkat"
	}
	return "tanpa kanal"
}

func (a *App) pingSourceLabel(st *notificationSettingsState) string {
	switch st.pingKind {
	case "task":
		for _, tk := range a.tasks.items {
			if tk.ID == st.pingTarget {
				return a.taskCode(tk) + " " + tk.Title
			}
		}
		return "Pilih tugas"
	case "meeting":
		for _, m := range a.meetings.items {
			if m.ID == st.pingTarget {
				return m.Title
			}
		}
		return "Pilih rapat"
	}
	return "Umum"
}

func (a *App) pingRecipientsList(c *ui.Context, st *notificationSettingsState) {
	ui.Text(c, "Penerima").FontSize(13).Bold()
	if len(a.meetings.members) == 0 {
		muted(c, "Memuat anggota…")
	}
	for _, m := range a.meetings.members {
		m := m
		on := slices.Contains(st.recipients, m.ID)
		if spaceButton(c.Key(m.ID), m.DisplayName, "user", on).FillWidth().Clicked() {
			if on {
				st.recipients = slices.DeleteFunc(st.recipients, func(u id.ID) bool { return u == m.ID })
			} else {
				st.recipients = append(st.recipients, m.ID)
			}
		}
	}
}

func (a *App) pingSourceList(c *ui.Context, st *notificationSettingsState) {
	t := c.Theme()
	ui.Text(c, "Sumber").FontSize(11).TextColor(faint(t))
	if spaceButton(c.Key("src-umum"), "Umum", "bell", st.pingKind == "general").FillWidth().Clicked() {
		st.pingKind, st.pingTarget, a.chrome.pingSrcOpen = "general", id.Nil, false
	}
	ui.Scroll(c.Key("src-list")).MaxHeight(180).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Gap(2).Children(func() {
			for _, tk := range a.tasks.items {
				tk := tk
				if tk.Permissions.Manage && spaceButton(c.Key(tk.ID), a.taskCode(tk)+" "+tk.Title, "task", st.pingTarget == tk.ID).FillWidth().Clicked() {
					st.pingKind, st.pingTarget, a.chrome.pingSrcOpen = "task", tk.ID, false
				}
			}
			for _, m := range a.meetings.items {
				m := m
				if m.CanManage && spaceButton(c.Key(m.ID), m.Title, "calendar", st.pingTarget == m.ID).FillWidth().Clicked() {
					st.pingKind, st.pingTarget, a.chrome.pingSrcOpen = "meeting", m.ID, false
				}
			}
		})
	})
}

// pingChannels is the Perangkat · WhatsApp toggle pair.
func pingChannels(c *ui.Context, st *notificationSettingsState) {
	t := c.Theme()
	ui.Row(c.Key("ping-ch")).Gap(2).Padding(3).Radius(9).Border(1, t.Border).Shrink(0).Children(func() {
		for _, ch := range []struct {
			label string
			on    *bool
		}{{"Perangkat", &st.device}, {"WhatsApp", &st.whatsapp}} {
			b := ui.ButtonBase(c.Key(ch.label)).Label(ch.label).Height(30).Padding(0, 12).Radius(7).Center().TextColor(t.TextMuted)
			if *ch.on {
				b.Background(t.Accent).TextColor(t.AccentText)
			}
			b.FocusRing(b.FocusVisible()).Children(func() { ui.Text(c, ch.label).FontSize(12) })
			if b.Clicked() {
				*ch.on = !*ch.on
			}
		}
	})
}

// pingUnseenPanel is "Belum melihat": members with pings they saw on no
// channel, from the shown page, with a reason and a Ping per person.
func (a *App) pingUnseenPanel(c *ui.Context, sp *api.Space) {
	t := c.Theme()
	type row struct {
		ref    api.PersonRef
		count  int
		failed bool
		last   api.PingRecord
	}
	var rows []row
	index := map[id.ID]int{}
	for _, p := range a.pingHistory.items {
		for _, person := range pingPeople(p) {
			if person.seen {
				continue
			}
			i, ok := index[person.ref.ID]
			if !ok {
				i = len(rows)
				index[person.ref.ID] = i
				rows = append(rows, row{ref: person.ref, last: p})
			}
			rows[i].count++
			rows[i].failed = rows[i].failed || person.failed
		}
	}
	ui.Column(c.Key("ping-unseen")).FillWidth().MinWidth(0).MaxHeight(10000).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Row(c).FillWidth().Padding(14, 16).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, "Belum melihat").FontSize(15).Bold().Grow(1)
			muted(c, fmt.Sprintf("%d anggota", len(rows))).FontSize(12)
		})
		ui.Scroll(c.Key("unseen-list")).FillWidth().Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().Padding(6, 10).Children(func() {
				if len(rows) == 0 {
					ui.Column(c).Padding(10, 6).Children(func() {
						muted(c, "Semua penerima di halaman ini sudah melihat.").FontSize(13)
					})
				}
				for _, r := range rows {
					r := r
					ui.Row(c.Key(r.ref.ID)).FillWidth().MinWidth(0).Gap(10).Padding(8, 6).AlignItems(ui.Center).Children(func() {
						avatar(c, r.ref.DisplayName, 30)
						ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
							ui.Text(c, r.ref.DisplayName).FontSize(13).FontWeight(600).MaxLines(1)
							why := fmt.Sprintf("%d ping belum dilihat", r.count)
							if r.failed {
								why += " · WA gagal"
							} else {
								why += " · " + strings.TrimSpace(r.last.Message)
							}
							muted(c, why).FontSize(12).MaxLines(2)
						})
						if canPing(sp) && spaceButton(c.Key("ping-one"), "Ping", "send", false).Label("Ping "+r.ref.DisplayName).Padding(4, 10).MinHeight(30).Shrink(0).Clicked() {
							st := a.pingComposerState(sp)
							st.recipients = []id.ID{r.ref.ID}
						}
					})
				}
			})
		})
	})
}

// pingTimeline is "Riwayat ping": filters, days, items and the pager.
func (a *App) pingTimeline(c *ui.Context, sp *api.Space, phone bool) {
	t := c.Theme()
	st := &a.pingHistory
	ui.Column(c.Key("ping-history")).FillWidth().Grow(1).MinWidth(0).MinHeight(0).Radius(12).Background(t.Surface).Border(1, t.Border).Clip().Children(func() {
		head := ui.Row(c).FillWidth().Padding(12, 16).Gap(12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border)
		if phone {
			head.Column().AlignItems(ui.Start)
		}
		head.Children(func() {
			ui.Text(c, "Riwayat ping").FontSize(15).Bold().Grow(1)
			current := 0
			for i, f := range pingFilters {
				if f.key == st.filter {
					current = i
				}
			}
			labels := []string{}
			for _, f := range pingFilters {
				labels = append(labels, f.label)
			}
			if chosen := agendaSegmented(c, current, labels...); chosen != current {
				st.filter, st.page, st.cursors, st.items, st.loaded = pingFilters[chosen].key, 0, nil, nil, false
				a.pingLoad()
			}
		})
		ui.Scroll(c.Key("ping-list")).FillWidth().Grow(1).MinHeight(0).Label("Riwayat ping").Children(func() {
			ui.Column(c).FillWidth().MinWidth(0).Padding(0, 16, 8, 16).Children(func() {
				switch {
				case st.err != "":
					ui.Column(c).Padding(16, 0).Children(func() {
						ui.Text(c, "Riwayat ping belum dapat dimuat: "+st.err).FontSize(12).TextColor(t.Danger)
					})
					return
				case st.missing:
					pageEmpty(c, "clock", "Riwayat ping belum tersedia", "Server ini belum menyediakan riwayat ping.")
					return
				case st.loading && len(st.items) == 0:
					ui.Column(c).Padding(16, 0).Children(func() { muted(c, "Memuat riwayat ping…") })
					return
				case len(st.items) == 0:
					title, text := "Belum ada ping", "Tulis pesan di bilah atas lalu Kirim ping."
					if st.filter == "unseen" {
						title, text = "Semua sudah melihat", "Tidak ada ping yang belum dilihat pada halaman ini."
					} else if st.filter == "failed" {
						title, text = "Tidak ada yang gagal", "Kiriman yang gagal akan muncul di sini."
					}
					pageEmpty(c, "clock", title, text)
					return
				}
				now := time.Now()
				last := ""
				for _, p := range st.items {
					p := p
					day := pingDayLabel(p.SentAt, now, p.TimeZone)
					if day != last {
						last = day
						ui.Text(c.Key("day"+day), strings.ToUpper(day)).FontSize(11).TextColor(faint(t)).Padding(14, 0, 6, 0)
					}
					a.pingItem(c, sp, p, phone)
				}
			})
		})
		if len(st.items) > 0 || st.page > 0 {
			pager := ui.Row(c).FillWidth().Padding(10, 16).Gap(8).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(t.Border)
			pager.Children(func() {
				muted(c, fmt.Sprintf("Halaman %d · 50 per halaman", st.page+1)).FontSize(12).Grow(1)
				if spaceButton(c.Key("newer"), "Lebih baru", "left", false).Padding(4, 10).MinHeight(30).Disabled(st.page == 0 || st.loading).Clicked() {
					st.page--
					a.pingLoad()
				}
				if spaceButton(c.Key("older"), "Lebih lama", "right", false).Padding(4, 10).MinHeight(30).Disabled(!st.hasMore || st.loading).Clicked() {
					// cursors[i] is page i's before=; page 0 has none.
					if len(st.cursors) == 0 {
						st.cursors = []time.Time{{}}
					}
					st.cursors = append(st.cursors[:st.page+1], st.items[len(st.items)-1].SentAt)
					st.page++
					a.pingLoad()
				}
			})
		}
	})
}

// pingItem is one ping of the timeline: time, a dot by state, sender and
// message, source and seen count, the recipients and Ping ulang.
func (a *App) pingItem(c *ui.Context, sp *api.Space, p api.PingRecord, phone bool) {
	t := c.Theme()
	st := &a.pingHistory
	people := pingPeople(p)
	unseen := pingUnseen(p)
	failed := false
	for _, person := range people {
		failed = failed || person.failed
	}
	dot := t.Success
	switch {
	case failed:
		dot = t.Danger
	case len(unseen) > 0:
		dot = t.Warning
	}
	loc, _ := timeutil.LoadZone(p.TimeZone)
	if loc == nil {
		loc = time.Local
	}
	ui.Row(c.Key(p.ID)).FillWidth().MinWidth(0).Gap(10).Padding(8, 0).AlignItems(ui.Start).Children(func() {
		ui.Text(c, p.SentAt.In(loc).Format("15.04")).Font(FontMono).FontSize(12).TextColor(t.TextMuted).Width(44).Shrink(0)
		ui.Box(c).Size(9, 9).Radius(4.5).Background(dot).Margin(5, 0, 0, 0).Shrink(0)
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			sender := "Space"
			if p.Sender != nil {
				sender = strings.Fields(p.Sender.DisplayName + " ")[0]
			}
			ui.Text(c, sender+" · "+p.Message).FontSize(13).MaxLines(3)
			meta := "Umum"
			switch p.RelatedKind {
			case "task":
				meta = "Tugas"
			case "meeting":
				meta = "Rapat"
			}
			meta += fmt.Sprintf(" · %d/%d dilihat", len(people)-len(unseen), len(people))
			if failed {
				meta += " · ada yang gagal"
			}
			muted(c, meta).FontSize(12).MaxLines(1)
			ui.Row(c).FillWidth().MinWidth(0).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
				ui.Row(c).Gap(4).Grow(1).MinWidth(0).Children(func() {
					for _, person := range people {
						person := person
						color := t.TextMuted
						switch {
						case person.failed:
							color = t.Danger
						case person.seen:
							color = t.Success
						}
						ui.Box(c.Key(person.ref.ID)).Size(28, 28).Tooltip(person.ref.DisplayName).Children(func() {
							avatar(c, person.ref.DisplayName, 26)
							ui.Box(c).Absolute().Right(0).Bottom(0).Size(9, 9).Radius(4.5).Background(color).Border(2, t.Surface)
						})
					}
				})
				if !canPing(sp) {
					return
				}
				if len(unseen) == 0 {
					muted(c, "Semua sudah melihat").FontSize(12)
					return
				}
				if st.resendConfirm == p.ID {
					ui.Row(c.Key("confirm")).Gap(6).AlignItems(ui.Center).Children(func() {
						muted(c, fmt.Sprintf("Ping ulang %d orang?", len(unseen))).FontSize(12)
						if primarySpaceButton(c.Key("yes"), "Ya, kirim", "send").Padding(4, 10).MinHeight(30).Disabled(st.resending).Clicked() {
							a.resendPing(p, unseen)
						}
						if spaceButton(c.Key("no"), "Batal", "", false).Padding(4, 10).MinHeight(30).Clicked() {
							st.resendConfirm = id.Nil
						}
					})
					return
				}
				if spaceButton(c.Key("again"), fmt.Sprintf("Ping ulang %d", len(unseen)), "refresh", false).Padding(4, 10).MinHeight(30).Disabled(st.resending).Clicked() {
					st.resendConfirm = p.ID
				}
			})
		})
	})
}

// resendPing sends the same message again to who has not seen it, on the
// channels the ping used, with the same source.
func (a *App) resendPing(p api.PingRecord, to []id.ID) {
	svc, ok := a.deliveryService()
	sp := a.selectedSpace()
	if !ok || sp == nil {
		return
	}
	req := api.PingRequest{Message: p.Message, Recipients: slices.Clone(to), SourceKind: p.RelatedKind, SourceID: p.RelatedID}
	for _, r := range p.Recipients {
		if !slices.Contains(req.Channels, r.Channel) {
			req.Channels = append(req.Channels, r.Channel)
		}
	}
	st := &a.pingHistory
	gen, sid := a.gen, sp.ID
	st.resending = true
	a.async(func() func() {
		err := svc.SendPing(context.Background(), sid, req)
		return func() {
			if gen != a.gen || a.pingHistory.space != sid {
				return
			}
			st := &a.pingHistory
			st.resending, st.resendConfirm = false, id.Nil
			if err != nil {
				a.notificationSettingsState.err = a.failed(err)
				return
			}
			a.notificationSettingsState.err = fmt.Sprintf("Ping ulang dikirim ke %d orang.", len(req.Recipients))
			st.loaded = false
			a.pingLoad()
		}
	})
}
