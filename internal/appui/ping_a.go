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
)

// Kirim ping as canvas 8f83cee819ac: the dialog is option A (one screen:
// search, quick groups, the default message prefilled, one WhatsApp
// switch), and option C adds "Ingatkan …" on tasks and meetings, sent in one
// click, and a small name box under the header's Ping button.

// pingDefaultMessage is what the server sends when the message is empty.
const pingDefaultMessage = "Mohon periksa pekerjaan atau agenda di Space."

// pingMaxRecipients is the server's limit on one ping.
const pingMaxRecipients = 500

// pingRemindGap keeps an "Ingatkan" button from sending twice by accident.
const pingRemindGap = time.Minute

// pingUIState is what the composers keep beyond the shared composer state:
// the search, the WhatsApp choice remembered between Spaces, the summary
// of the last send, the inline reminders, and a toast to show.
type pingUIState struct {
	query     string
	whatsapp  bool
	boxOpen   bool
	boxErr    string
	sent      *pingSent
	failed    bool
	reminding map[id.ID]bool
	reminded  map[id.ID]time.Time
	toast     *ui.Toast
	// origin is the composer of the send under way: "dialog", "box",
	// or "" for the Beranda and Ping tab bars, which show st.err.
	origin string
}

// pingSent is the dialog's summary after a send.
type pingSent struct {
	names    []string
	whatsapp bool
}

// resetPingState starts the shared composer for a Space: no recipients, the
// app always, WhatsApp as last chosen.
func (a *App) resetPingState(sp *api.Space, division id.ID) {
	a.notificationSettingsState = notificationSettingsState{period: sp.PeriodID, space: sp.ID, division: division, device: true, whatsapp: a.pingUI.whatsapp, pingKind: "general"}
	a.pingUI.query, a.pingUI.sent, a.pingUI.failed, a.pingUI.boxErr = "", nil, false, ""
}

// pingPeople are the Space's members one may ping, without oneself.
func (a *App) pingPeople() []api.PersonRef {
	return slices.DeleteFunc(slices.Clone(a.meetings.members), func(p api.PersonRef) bool { return p.ID == a.user.ID || p.ID == id.Nil })
}

func pingPersonName(people []api.PersonRef, uid id.ID) string {
	for _, p := range people {
		if p.ID == uid {
			return p.DisplayName
		}
	}
	return "Anggota"
}

// pingGroup is a quick group: a label and its members among people.
type pingGroup struct {
	key, label string
	members    []id.ID
}

// pingGroups are Semua, PJ tugas aktif and Belum baca ping terakhir, each
// only with people the server accepts, and shown only when not empty.
func (a *App) pingGroups(people []api.PersonRef) []pingGroup {
	in := func(uid id.ID) bool {
		return slices.ContainsFunc(people, func(p api.PersonRef) bool { return p.ID == uid })
	}
	all := make([]id.ID, 0, len(people))
	for _, p := range people {
		all = append(all, p.ID)
	}
	var pj []id.ID
	for _, t := range a.tasks.items {
		active := t.Progress == api.TaskTodo || t.Progress == api.TaskDoing || t.Progress == api.TaskSubmitted
		if active && in(t.Assignee.ID) && !slices.Contains(pj, t.Assignee.ID) {
			pj = append(pj, t.Assignee.ID)
		}
	}
	var unseen []id.ID
	if h := a.pingHistory; h.space == a.selectedSpaceID() && len(h.items) > 0 {
		unseen = slices.DeleteFunc(pingUnseen(h.items[0]), func(uid id.ID) bool { return !in(uid) })
	}
	out := []pingGroup{{"all", "Semua", all}}
	if len(pj) > 0 {
		out = append(out, pingGroup{"pj", "PJ tugas aktif", pj})
	}
	if len(unseen) > 0 {
		out = append(out, pingGroup{"unseen", "Belum baca ping terakhir", unseen})
	}
	return out
}

// togglePingGroup adds the group's members, or takes them all out when
// every one of them is already chosen.
func togglePingGroup(st *notificationSettingsState, g pingGroup) {
	if pingGroupChosen(st, g) {
		st.recipients = slices.DeleteFunc(st.recipients, func(uid id.ID) bool { return slices.Contains(g.members, uid) })
		return
	}
	for _, uid := range g.members {
		if len(st.recipients) >= pingMaxRecipients {
			break
		}
		if !slices.Contains(st.recipients, uid) {
			st.recipients = append(st.recipients, uid)
		}
	}
}

func pingGroupChosen(st *notificationSettingsState, g pingGroup) bool {
	if len(g.members) == 0 {
		return false
	}
	for _, uid := range g.members {
		if !slices.Contains(st.recipients, uid) {
			return false
		}
	}
	return true
}

func togglePingPerson(st *notificationSettingsState, uid id.ID) {
	if slices.Contains(st.recipients, uid) {
		st.recipients = slices.DeleteFunc(st.recipients, func(u id.ID) bool { return u == uid })
	} else if len(st.recipients) < pingMaxRecipients {
		st.recipients = append(st.recipients, uid)
	}
}

// pingMatches are people whose name holds every word of the query.
func pingMatches(people []api.PersonRef, query string) []api.PersonRef {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return people
	}
	var out []api.PersonRef
	for _, p := range people {
		name := strings.ToLower(p.DisplayName)
		ok := true
		for _, w := range words {
			if !strings.Contains(name, w) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, p)
		}
	}
	return out
}

// pingSendLabel is the send button: "Kirim ke 3 orang".
func pingSendLabel(n int) string {
	if n == 0 {
		return "Kirim"
	}
	return fmt.Sprintf("Kirim ke %d orang", n)
}

// pingCanSend: someone chosen, within the limit, online, not sending, and
// a task or meeting chosen when the ping is about one.
func (a *App) pingCanSend(st *notificationSettingsState) bool {
	n := len(st.recipients)
	return !st.busy && a.conn == connOnline && n > 0 && n <= pingMaxRecipients && (st.pingKind == "general" || st.pingTarget != id.Nil)
}

// pingComposer is option A's body and footer, shared by the dialog and the
// Pengaturan page.
func (a *App) pingComposer(c *ui.Context) {
	if !a.pingComposerReady() {
		return
	}
	if a.pingUI.sent != nil {
		a.pingSentView(c)
		return
	}
	a.pingComposerBody(c)
	a.pingComposerFooter(c)
}

// pingComposerReady loads what the composer needs and resets it for a new
// Space; false when the user may not ping here.
func (a *App) pingComposerReady() bool {
	a.ensureMeetings()
	sp := a.selectedSpace()
	if !canPing(sp) {
		return false
	}
	st := &a.notificationSettingsState
	if st.period != sp.PeriodID || st.space != sp.ID {
		a.resetPingState(sp, st.division)
	}
	return true
}

func (a *App) pingComposerBody(c *ui.Context) {
	t := c.Theme()
	st := &a.notificationSettingsState
	people := a.pingPeople()
	if st.pingKind != "general" && st.pingTarget != id.Nil {
		ui.Row(c.Key("ping-about")).FillWidth().Gap(8).AlignItems(ui.Center).Padding(8, 10).Radius(8).Background(accentSoft(t)).Children(func() {
			ui.Icon(c, spaceIcon(map[string]string{"task": "task", "meeting": "calendar"}[st.pingKind])).Size(16, 16).TextColor(t.Accent).Shrink(0)
			ui.Text(c, "Tentang "+a.pingSourceLabel(st)).FontSize(13).Grow(1).MinWidth(0).MaxLines(1)
			if spaceButton(c.Key("ping-about-clear"), "Jadikan ping umum", "close", false).Padding(4, 8).MinHeight(28).Shrink(0).Clicked() {
				st.pingKind, st.pingTarget = "general", id.Nil
			}
		})
	}
	q := ui.SearchField(c.Key("ping-search"), &a.pingUI.query).Label("Cari nama").Placeholder("Cari nama").FillWidth().AutoFocus()
	matches := pingMatches(people, a.pingUI.query)
	if q.Submitted() && strings.TrimSpace(a.pingUI.query) != "" && len(matches) > 0 {
		togglePingPerson(st, matches[0].ID)
		a.pingUI.query = ""
	}
	if strings.TrimSpace(a.pingUI.query) == "" && len(people) > 0 {
		ui.Row(c.Key("ping-groups")).FillWidth().Wrap().Gap(8).Children(func() {
			for _, g := range a.pingGroups(people) {
				g := g
				on := pingGroupChosen(st, g)
				label := fmt.Sprintf("%s %d", g.label, len(g.members))
				b := ui.ButtonBase(c.Key("group-"+g.key)).Label(label).Row().Gap(6).AlignItems(ui.Center).Height(32).Padding(0, 12).Radius(8).Border(1, t.Border).TextColor(t.Text)
				if on {
					b.Background(accentSoft(t)).BorderColor(t.Accent)
				} else if b.Hovered() {
					b.Background(t.SurfaceHover)
				}
				b.FocusRing(b.FocusVisible()).Children(func() {
					ui.Text(c, g.label).FontSize(13)
					ui.Text(c, fmt.Sprint(len(g.members))).FontSize(13).TextColor(faint(t))
				})
				if b.Clicked() {
					togglePingGroup(st, g)
				}
			}
		})
	}
	ui.Column(c.Key("ping-people")).FillWidth().Gap(2).Children(func() {
		switch {
		case len(people) == 0 && !a.meetings.loaded:
			muted(c, "Memuat anggota…").FontSize(13)
		case len(people) == 0:
			muted(c, "Belum ada anggota lain di Space ini.").FontSize(13)
		case len(matches) == 0:
			muted(c, "Tidak ada nama yang cocok.").FontSize(13)
		}
		for _, p := range matches {
			on := slices.Contains(st.recipients, p.ID)
			was := on
			if ui.Checkbox(c.Key(p.ID), &on, p.DisplayName).FillWidth().Padding(6, 4).Changed() && on != was {
				togglePingPerson(st, p.ID)
			}
		}
	})
}

func (a *App) pingComposerFooter(c *ui.Context) {
	t := c.Theme()
	st := &a.notificationSettingsState
	people := a.pingPeople()
	ui.Column(c.Key("ping-footer")).FillWidth().Gap(12).Children(func() {
		if n := len(st.recipients); n > 0 {
			ui.Row(c.Key("ping-chosen")).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				names := make([]string, 0, min(n, 3))
				for _, uid := range st.recipients[:min(n, 3)] {
					names = append(names, firstName(pingPersonName(people, uid)))
				}
				label := strings.Join(names, ", ")
				if n > 3 {
					label += fmt.Sprintf(" dan %d lainnya", n-3)
				}
				ui.Text(c, fmt.Sprintf("%d dipilih: %s", n, label)).FontSize(13).Grow(1).MinWidth(0).MaxLines(2)
				if spaceButton(c.Key("ping-clear"), "Kosongkan", "", false).Padding(4, 10).MinHeight(28).Shrink(0).Clicked() {
					st.recipients = nil
				}
			})
		}
		ui.TextInput(c.Key("ping-message"), &st.pingMessage).Label("Pesan, boleh dikosongkan").Placeholder(pingDefaultMessage).FillWidth()
		if a.pingUI.failed && st.err != "" {
			ui.Row(c.Key("ping-failed")).FillWidth().Gap(10).Padding(10, 12).Radius(8).Background(ui.RGBA(t.Danger.R, t.Danger.G, t.Danger.B, .12)).Children(func() {
				ui.Icon(c, spaceIcon("alert")).Size(16, 16).TextColor(t.Danger).Shrink(0)
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, "Ping belum terkirim.").FontSize(13).Bold()
					ui.Text(c, st.err+" Pilihan Anda tetap tersimpan.").FontSize(13)
				})
			})
		}
		ui.Row(c.Key("ping-send-row")).FillWidth().Wrap().Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(220).Gap(4).Children(func() {
				if ui.Switch(c.Key("ping-wa"), &st.whatsapp).Label("Juga ke WhatsApp").Changed() {
					a.pingUI.whatsapp = st.whatsapp
				}
				hint := "Semua menerima di aplikasi Space."
				if st.whatsapp {
					hint = "Semua menerima di aplikasi Space; yang sudah menautkan WhatsApp juga di WhatsApp."
				}
				ui.Text(c, hint).FontSize(12).TextColor(faint(t))
			})
			label := pingSendLabel(len(st.recipients))
			if a.pingUI.failed {
				label = "Coba kirim lagi"
			}
			if st.busy {
				label = "Mengirim…"
			}
			if primarySpaceButton(c.Key("ping-send"), label, "send").Shrink(0).Disabled(!a.pingCanSend(st)).Clicked() {
				a.pingUI.origin = "dialog"
				st.device = true
				a.sendManualPing()
			}
		})
		if len(st.recipients) >= pingMaxRecipients {
			ui.Text(c, fmt.Sprintf("Satu ping paling banyak untuk %d orang.", pingMaxRecipients)).FontSize(12).TextColor(faint(t))
		}
	})
}

// pingSentView is the dialog after a send: who it went to, Kirim lagi and
// Selesai.
func (a *App) pingSentView(c *ui.Context) {
	t := c.Theme()
	s := a.pingUI.sent
	ui.Column(c.Key("ping-sent")).FillWidth().Gap(10).AlignItems(ui.Center).Padding(12, 0).Children(func() {
		ui.Box(c).Size(44, 44).Radius(22).Center().Background(ui.RGBA(t.Success.R, t.Success.G, t.Success.B, .14)).Children(func() {
			ui.Icon(c, spaceIcon("check")).Size(22, 22).TextColor(t.Success)
		})
		ui.Text(c, fmt.Sprintf("Ping terkirim ke %d orang", len(s.names))).FontSize(17).Bold()
		line := "Masuk ke aplikasi Space mereka."
		if s.whatsapp {
			line = "Masuk ke aplikasi Space mereka, dan ke WhatsApp bagi yang sudah menautkannya."
		}
		muted(c, line).FontSize(13)
		names := strings.Join(s.names[:min(len(s.names), 6)], ", ")
		if len(s.names) > 6 {
			names += fmt.Sprintf(" dan %d lainnya", len(s.names)-6)
		}
		ui.Text(c, names).FontSize(13).MaxLines(3)
		ui.Text(c, "Siapa yang sudah membaca ada di tab Ping.").FontSize(12).TextColor(faint(t))
	})
	ui.Row(c.Key("ping-sent-actions")).FillWidth().Gap(8).Justify(ui.End).Children(func() {
		if spaceButton(c.Key("ping-again"), "Kirim lagi", "send", false).Border(1, t.Border).Clicked() {
			st := &a.notificationSettingsState
			st.recipients, st.pingMessage, st.err = nil, "", ""
			a.pingUI.sent, a.pingUI.query = nil, ""
		}
		if primarySpaceButton(c.Key("ping-done"), "Selesai", "check").Clicked() {
			a.pingUI.sent = nil
			a.notificationSettingsState.recipients, a.notificationSettingsState.pingMessage, a.notificationSettingsState.err = nil, "", ""
			a.chrome.pingOpen = false
		}
	})
}

// sendManualPing sends the shared composer's ping. The dialog shows its
// summary; the name box and the inline buttons say it in a toast.
func (a *App) sendManualPing() {
	svc, ok := a.deliveryService()
	sp := a.selectedSpace()
	if !ok || sp == nil {
		return
	}
	st := &a.notificationSettingsState
	if len(st.recipients) == 0 || len(st.recipients) > pingMaxRecipients {
		st.err = fmt.Sprintf("Pilih 1–%d penerima.", pingMaxRecipients)
		return
	}
	req := api.PingRequest{Message: strings.TrimSpace(st.pingMessage), Recipients: slices.Clone(st.recipients)}
	if st.pingKind != "general" {
		req.SourceKind = st.pingKind
		target := st.pingTarget
		req.SourceID = &target
	}
	if st.device {
		req.Channels = append(req.Channels, "device")
	}
	if st.whatsapp {
		req.Channels = append(req.Channels, "whatsapp")
	}
	if len(req.Channels) == 0 {
		st.err = "Pilih setidaknya satu kanal."
		return
	}
	people := a.pingPeople()
	names := make([]string, 0, len(req.Recipients))
	for _, uid := range req.Recipients {
		names = append(names, pingPersonName(people, uid))
	}
	gen, sid, origin := a.gen, sp.ID, a.pingUI.origin
	a.pingUI.origin = ""
	st.busy, st.err, a.pingUI.failed = true, "", false
	a.async(func() func() {
		err := svc.SendPing(context.Background(), sid, req)
		return func() {
			if gen != a.gen || sid != a.selectedSpaceID() {
				return
			}
			st := &a.notificationSettingsState
			st.busy = false
			if err != nil {
				st.err = a.failed(err)
				a.pingUI.failed = origin == "dialog"
				if origin == "box" {
					a.pingUI.boxErr = st.err
				}
				return
			}
			a.pingHistory.loaded = false
			wa := slices.Contains(req.Channels, "whatsapp")
			switch origin {
			case "":
				st.err = fmt.Sprintf("Ping terkirim ke %d orang.", len(names))
			case "box":
				a.pingUI.boxOpen, a.pingUI.boxErr = false, ""
				st.recipients, st.pingMessage, st.err = nil, "", ""
				a.pingToast(fmt.Sprintf("Ping terkirim ke %d orang.", len(names)), strings.Join(names, ", "), "success")
			default:
				a.pingUI.sent = &pingSent{names: names, whatsapp: wa}
			}
		}
	})
}

// pingToast queues a toast for the next frame.
func (a *App) pingToast(title, description, kind string) {
	a.pingUI.toast = &ui.Toast{Title: title, Description: description, Type: kind, Timeout: 5 * time.Second}
}

// showPingToast shows a queued toast; the shell calls it every frame.
func (a *App) showPingToast(c *ui.Context) {
	if a.pingUI.toast != nil {
		c.AddToast(*a.pingUI.toast)
		a.pingUI.toast = nil
	}
}

// pingNameBox is option C's small box under the header's Ping button:
// type a name, Enter adds it, Kirim. "Pilih banyak" opens the dialog with
// the same choice.
func (a *App) pingNameBox(c *ui.Context, anchor ui.Element) {
	ui.PopoverBase(c, anchor, &a.pingUI.boxOpen, func(panel ui.Element) {
		t := c.Theme()
		w, _ := c.Size()
		panel.Margin(6, 0, 0, 0).Width(min(float32(380), w-24)).Padding(12).Gap(10).Radius(12).Background(t.Surface).Border(1, t.Border).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, .25)).Label("Kirim ping cepat")
		if !a.pingComposerReady() {
			return
		}
		st := &a.notificationSettingsState
		people := a.pingPeople()
		if len(st.recipients) > 0 {
			ui.Row(c.Key("box-chosen")).FillWidth().Wrap().Gap(6).Children(func() {
				for _, uid := range st.recipients {
					name := pingPersonName(people, uid)
					b := ui.ButtonBase(c.Key(uid)).Label("Hapus "+name).Row().Gap(4).AlignItems(ui.Center).Height(28).Padding(0, 6, 0, 10).Radius(7).Background(t.SurfaceHover).Border(1, t.Border).TextColor(t.Text)
					b.FocusRing(b.FocusVisible()).Children(func() {
						ui.Text(c, firstName(name)).FontSize(13).MaxLines(1)
						ui.Icon(c, spaceIcon("close")).Size(12, 12).TextColor(t.TextMuted)
					})
					if b.Clicked() {
						togglePingPerson(st, uid)
					}
				}
			})
		}
		q := ui.SearchField(c.Key("box-search"), &a.pingUI.query).Label("Ketik nama").Placeholder("Ketik nama, Enter menambah").FillWidth().AutoFocus()
		query := strings.TrimSpace(a.pingUI.query)
		var matches []api.PersonRef
		if query != "" {
			matches = slices.DeleteFunc(pingMatches(people, query), func(p api.PersonRef) bool { return slices.Contains(st.recipients, p.ID) })
		}
		if q.Submitted() && len(matches) > 0 {
			togglePingPerson(st, matches[0].ID)
			a.pingUI.query, matches = "", nil
		}
		if query != "" {
			ui.Column(c.Key("box-matches")).FillWidth().Gap(2).Children(func() {
				if len(matches) == 0 {
					muted(c, "Tidak ada nama yang cocok.").FontSize(13)
				}
				for i, p := range matches[:min(len(matches), 5)] {
					b := spaceButton(c.Key(p.ID), p.DisplayName, "user", i == 0).FillWidth()
					if b.Clicked() {
						togglePingPerson(st, p.ID)
						a.pingUI.query = ""
					}
				}
			})
		} else if len(st.recipients) == 0 {
			ui.Row(c.Key("box-groups")).FillWidth().Wrap().Gap(6).Children(func() {
				for _, g := range a.pingGroups(people) {
					g := g
					if spaceButton(c.Key("box-"+g.key), fmt.Sprintf("%s %d", g.label, len(g.members)), "users", false).Border(1, t.Border).Padding(4, 10).MinHeight(30).Clicked() {
						togglePingGroup(st, g)
					}
				}
			})
		}
		ui.TextInput(c.Key("box-message"), &st.pingMessage).Label("Pesan, boleh dikosongkan").Placeholder(pingDefaultMessage).FillWidth()
		if a.pingUI.boxErr != "" {
			ui.Text(c, "Ping belum terkirim. "+a.pingUI.boxErr).FontSize(12).TextColor(t.Danger)
		}
		ui.Row(c.Key("box-actions")).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			if ui.Switch(c.Key("box-wa"), &st.whatsapp).Label("Juga ke WhatsApp").Changed() {
				a.pingUI.whatsapp = st.whatsapp
			}
			ui.Text(c, "WhatsApp").FontSize(12).TextColor(t.TextMuted)
			ui.Box(c).Grow(1)
			if spaceButton(c.Key("box-more"), "Pilih banyak", "", false).Padding(4, 10).MinHeight(32).Clicked() {
				a.pingUI.boxOpen = false
				a.chrome.pingOpen = true
			}
			label := pingSendLabel(len(st.recipients))
			if st.busy {
				label = "Mengirim…"
			}
			if primarySpaceButton(c.Key("box-send"), label, "send").Shrink(0).Disabled(!a.pingCanSend(st)).Clicked() {
				a.pingUI.origin = "box"
				st.device = true
				a.sendManualPing()
			}
		})
	}).AttachTo(anchor, ui.AnchorBottomRight, ui.AnchorTopRight)
}

// pingRemind sends a ping about a task or meeting at once, from its
// "Ingatkan" button: the app always, WhatsApp as last chosen.
func (a *App) pingRemind(kind string, target id.ID, to []id.ID, message, who string) {
	svc, ok := a.deliveryService()
	sp := a.selectedSpace()
	if !ok || !canPing(sp) || len(to) == 0 || a.pingUI.reminding[target] {
		return
	}
	if len(to) > pingMaxRecipients {
		to = to[:pingMaxRecipients]
	}
	req := api.PingRequest{SourceKind: kind, SourceID: &target, Message: message, Recipients: slices.Clone(to), Channels: []string{"device"}}
	if a.pingUI.whatsapp {
		req.Channels = append(req.Channels, "whatsapp")
	}
	if a.pingUI.reminding == nil {
		a.pingUI.reminding, a.pingUI.reminded = map[id.ID]bool{}, map[id.ID]time.Time{}
	}
	a.pingUI.reminding[target] = true
	gen, sid := a.gen, sp.ID
	a.async(func() func() {
		err := svc.SendPing(context.Background(), sid, req)
		sent := time.Now()
		return func() {
			if gen != a.gen {
				return
			}
			delete(a.pingUI.reminding, target)
			if err != nil {
				a.pingToast("Ping ke "+who+" belum terkirim.", a.failed(err)+" Tekan Ingatkan lagi untuk mencoba.", "error")
				return
			}
			if sid == a.selectedSpaceID() {
				a.pingHistory.loaded = false
			}
			a.pingUI.reminded[target] = sent
			a.pingToast("Ping ke "+who+" terkirim.", "", "success")
		}
	})
}

// pingRemindLabel is an "Ingatkan" button's label: Mengirim… while it
// sends, "Diingatkan 14.05" for a minute after, else "Ingatkan who".
// ready says whether a click may send.
func (a *App) pingRemindLabel(target id.ID, who string, now time.Time) (label string, ready bool) {
	if a.pingUI.reminding[target] {
		return "Mengirim…", false
	}
	if at, ok := a.pingUI.reminded[target]; ok && now.Sub(at) < pingRemindGap {
		return "Diingatkan " + at.Local().Format("15.04"), false
	}
	return "Ingatkan " + who, true
}

// taskRemindable: the user may ping about this task, and it has a PJ other
// than the user who is still working on it.
func (a *App) taskRemindable(t api.Task) bool {
	if !t.Permissions.Manage || !canPing(a.selectedSpace()) || t.Assignee.ID == id.Nil || t.Assignee.ID == a.user.ID {
		return false
	}
	return t.Progress == api.TaskTodo || t.Progress == api.TaskDoing || t.Progress == api.TaskSubmitted
}

// remindTask pings the task's PJ about it.
func (a *App) remindTask(t api.Task) {
	if !a.taskRemindable(t) {
		return
	}
	a.pingRemind("task", t.ID, []id.ID{t.Assignee.ID}, "Mohon cek tugas: "+t.Title, t.Assignee.DisplayName)
}

// meetingRemindees are a meeting's participants one may still ping: not
// oneself, and still a member of the Space when that is known.
func (a *App) meetingRemindees(m api.Meeting) []id.ID {
	var out []id.ID
	for _, p := range m.Participants {
		if p.UserID == a.user.ID || p.UserID == id.Nil {
			continue
		}
		if len(a.meetings.members) > 0 && !slices.ContainsFunc(a.meetings.members, func(r api.PersonRef) bool { return r.ID == p.UserID }) {
			continue
		}
		out = append(out, p.UserID)
	}
	return out
}

// meetingRemindable: the user manages this meeting, may ping, and it is
// neither cancelled nor over.
func (a *App) meetingRemindable(m api.Meeting, now time.Time) bool {
	if !m.CanManage || m.Cancelled || !canPing(a.selectedSpace()) || len(a.meetingRemindees(m)) == 0 {
		return false
	}
	end := m.StartsAt.Add(2 * time.Hour)
	if m.EndsAt != nil {
		end = *m.EndsAt
	}
	return now.Before(end)
}

// remindMeeting pings the meeting's participants about it.
func (a *App) remindMeeting(m api.Meeting) {
	to := a.meetingRemindees(m)
	msg := "Pengingat rapat: " + m.Title + ", " + taskTimestamp(m.StartsAt, m.TimeZone) + "."
	a.pingRemind("meeting", m.ID, to, msg, fmt.Sprintf("%d peserta", len(to)))
}
