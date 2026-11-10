package appui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Stage T1: the meeting timer, on this device only (owner decision 9 Nov:
// local first, shared later in T2). It lives in the Rapat tab; while it
// runs a pill sticks above the document with the current agenda, its time
// left and the total, Jeda and Sembunyikan. When an agenda's time is up
// the pill turns yellow with "+5 mnt" and the next agenda. No sound, no
// dialog. Agenda durations are the minutes of numbered list items (or of
// headings), which the notulis sets with the duration chip of an item;
// without them the timer counts the total only. The timer never writes
// the note: it is not content and does not touch the editor lock.
//
// Stage T2: a note of a meeting shows the meeting's shared timer, kept on
// the server (api.MeetingTimer): the notulis on a laptop and the kadiv on
// a phone see the same one, and who may manage the meeting or edit its
// note steers it; the others only see it. The time is worked out here from
// the server's stamps. A note without a meeting keeps the T1 timer on this
// device. Hiding the pill is a choice of this device in both cases.

type noteTimer struct {
	started  time.Time
	pausedAt time.Time     // zero while running
	paused   time.Duration // paused before pausedAt
	agenda   int           // current agenda item
	agendaAt time.Duration // running time when it began
	extra    time.Duration // "+5 mnt" given to it
	done     []time.Duration
	hidden   bool // the pill is hidden on this device; the timer runs on
}

type noteAgendaItem struct {
	title   string
	minutes int
}

// noteAgenda: the items with planned minutes, in document order.
func noteAgenda(blocks []api.NoteBlock) []noteAgendaItem {
	out := []noteAgendaItem{}
	for _, b := range blocks {
		switch {
		case b.Type == api.BlockHeading && b.Minutes > 0:
			out = append(out, noteAgendaItem{title: strings.TrimSpace(b.Text), minutes: b.Minutes})
		case b.Type == api.BlockNumberedList:
			for _, it := range b.Items {
				if it.Minutes > 0 {
					out = append(out, noteAgendaItem{title: strings.TrimSpace(noteSpansText(it.Spans)), minutes: it.Minutes})
				}
			}
		}
	}
	return out
}

type noteTimerView struct {
	elapsed, planned time.Duration
	paused           bool
	agenda           int // -1: no agenda (total only) or all done
	title            string
	agendaElapsed    time.Duration
	agendaPlanned    time.Duration // with "+5 mnt"
	left, over       time.Duration
	finished         bool // every agenda item is done
}

func (t *noteTimer) running(now time.Time) time.Duration {
	end := now
	if !t.pausedAt.IsZero() {
		end = t.pausedAt
	}
	return max(0, end.Sub(t.started)-t.paused)
}

// noteTimerState is the timer at now: running time (pauses left out, so a
// meeting past midnight counts on), the current agenda, its time left or
// how far it is over.
func noteTimerState(t noteTimer, now time.Time, agenda []noteAgendaItem) noteTimerView {
	v := noteTimerView{elapsed: t.running(now), paused: !t.pausedAt.IsZero(), agenda: -1}
	for _, a := range agenda {
		v.planned += time.Duration(a.minutes) * time.Minute
	}
	if len(agenda) == 0 {
		return v
	}
	if t.agenda >= len(agenda) {
		v.finished = true
		return v
	}
	a := agenda[t.agenda]
	v.agenda, v.title = t.agenda, a.title
	v.agendaElapsed = max(0, v.elapsed-t.agendaAt)
	v.agendaPlanned = time.Duration(a.minutes)*time.Minute + t.extra
	if v.agendaElapsed > v.agendaPlanned {
		v.over = v.agendaElapsed - v.agendaPlanned
	} else {
		v.left = v.agendaPlanned - v.agendaElapsed
	}
	return v
}

func (t *noteTimer) pause(now time.Time) {
	if t.pausedAt.IsZero() {
		t.pausedAt = now
	}
}

func (t *noteTimer) resume(now time.Time) {
	if !t.pausedAt.IsZero() {
		t.paused += now.Sub(t.pausedAt)
		t.pausedAt = time.Time{}
	}
}

// next closes the current agenda item with its real duration.
func (t *noteTimer) next(now time.Time) {
	at := t.running(now)
	t.done = append(t.done, max(0, at-t.agendaAt))
	t.agenda++
	t.agendaAt, t.extra = at, 0
}

func (t *noteTimer) extend() { t.extra += 5 * time.Minute }

// noteClock: "08:12", "1:02:05".
func noteClock(d time.Duration) string {
	s := int(max(0, d) / time.Second)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

// noteLeft is a countdown: rounded up to the second.
func noteLeft(d time.Duration) string {
	return noteClock((max(0, d) + time.Second - 1).Truncate(time.Second))
}

func (a *App) noteTimer() *noteTimer {
	if a.notes.note == nil || a.notes.timers == nil {
		return nil
	}
	return a.notes.timers[a.notes.note.ID]
}

func (a *App) noteAgendaItems() []noteAgendaItem {
	if a.notes.editor == nil {
		return nil
	}
	return noteAgenda(a.notes.editor.blocks)
}

// noteTimerCtl is the timer a note shows: its meeting's shared timer, or,
// without a meeting, the one on this device.
type noteTimerCtl struct {
	tm      *noteTimer   // nil: none runs
	meeting *api.Meeting // the shared timer's meeting; nil: on this device
	loading bool         // the meeting is still on its way
	control bool         // may start and steer it
}

func (a *App) noteTimerCtl() noteTimerCtl {
	n := a.notes.note
	if n == nil {
		return noteTimerCtl{}
	}
	if n.MeetingID != nil {
		m, loading := a.noteMeeting()
		if m != nil && !m.Cancelled {
			ctl := noteTimerCtl{meeting: m, control: m.CanRunTimer}
			if m.Timer != nil {
				tm := sharedNoteTimer(*m.Timer, a.meetings.skew)
				tm.hidden = a.notes.sharedTimerHidden[m.ID]
				ctl.tm = &tm
			}
			return ctl
		}
		if loading {
			return noteTimerCtl{loading: true}
		}
	}
	return noteTimerCtl{tm: a.noteTimer(), control: true}
}

// sharedNoteTimer is the server's timer on this device's clock: skew is
// how far the server's clock is ahead of this one.
func sharedNoteTimer(t api.MeetingTimer, skew time.Duration) noteTimer {
	ms := func(v int64) time.Duration { return time.Duration(v) * time.Millisecond }
	out := noteTimer{started: t.StartedAt.Add(-skew), paused: ms(t.PausedMs), agenda: t.Agenda, agendaAt: ms(t.AgendaAtMs), extra: ms(t.ExtraMs)}
	if t.PausedAt != nil {
		out.pausedAt = t.PausedAt.Add(-skew)
	}
	for _, d := range t.DoneMs {
		out.done = append(out.done, ms(d))
	}
	return out
}

// setTimerHidden hides or shows the pill on this device.
func (a *App) setTimerHidden(ctl noteTimerCtl, hidden bool) {
	if ctl.meeting == nil {
		if tm := a.noteTimer(); tm != nil {
			tm.hidden = hidden
		}
		return
	}
	if a.notes.sharedTimerHidden == nil {
		a.notes.sharedTimerHidden = map[id.ID]bool{}
	}
	a.notes.sharedTimerHidden[ctl.meeting.ID] = hidden
}

// MeetingTimerAPI runs the shared timer's actions (stage T2). It is
// optional: a service without it cannot steer a meeting's timer.
type MeetingTimerAPI interface {
	MeetingTimer(context.Context, id.ID, api.MeetingTimerRequest) (api.Meeting, error)
}

func (a *App) meetingTimerService() (MeetingTimerAPI, bool) {
	if live, ok := a.api.(*client.Client); ok {
		return live.AccountSession(), true
	}
	s, ok := a.api.(MeetingTimerAPI)
	return s, ok
}

// noteTimerAct is one timer action: on the server for a meeting's timer,
// here for a timer on this device.
func (a *App) noteTimerAct(ctl noteTimerCtl, action string, now time.Time) {
	if ctl.meeting != nil {
		a.runMeetingTimer(ctl.meeting.ID, ctl.meeting.TimerVersion, action)
		return
	}
	switch action {
	case api.TimerStart:
		a.startNoteTimer(now)
		return
	case api.TimerStop:
		a.stopNoteTimer()
		return
	}
	tm := a.noteTimer()
	if tm == nil {
		return
	}
	switch action {
	case api.TimerPause:
		tm.pause(now)
	case api.TimerResume:
		tm.resume(now)
	case api.TimerNext:
		tm.next(now)
	case api.TimerExtend:
		tm.extend()
	}
}

func (a *App) runMeetingTimer(mid id.ID, version int64, action string) {
	svc, ok := a.meetingTimerService()
	if !ok || a.notes.timerBusy {
		return
	}
	gen := a.gen
	a.notes.timerBusy, a.notes.timerErr = true, ""
	a.async(func() func() {
		m, err := svc.MeetingTimer(context.Background(), mid, api.MeetingTimerRequest{Action: action, ExpectedVersion: version})
		received := time.Now()
		return func() {
			if gen != a.gen {
				return
			}
			a.notes.timerBusy = false
			if err != nil {
				a.notes.timerErr = a.failed(err)
				// Someone else changed it, or the meeting moved on: show
				// the timer as it is now.
				a.loadMeetings()
				return
			}
			a.meetings.noteSkew(m, received)
			for i := range a.meetings.items {
				if a.meetings.items[i].ID == m.ID {
					a.meetings.items[i] = m
				}
			}
		}
	})
}

func (a *App) startNoteTimer(now time.Time) {
	if a.notes.note == nil {
		return
	}
	if a.notes.timers == nil {
		a.notes.timers = map[id.ID]*noteTimer{}
	}
	a.notes.timers[a.notes.note.ID] = &noteTimer{started: now}
}

func (a *App) stopNoteTimer() {
	if a.notes.note != nil {
		delete(a.notes.timers, a.notes.note.ID)
	}
}

// noteTimerSection is the Timer of the Rapat tab.
func (a *App) noteTimerSection(c *ui.Context) {
	t := c.Theme()
	now := c.Now()
	agenda := a.noteAgendaItems()
	ctl := a.noteTimerCtl()
	tm := ctl.tm
	where := "Timer hanya di perangkat ini."
	if ctl.meeting != nil {
		where = "Timer bersama: semua yang membuka rapat ini melihat waktu yang sama."
	}
	notePanelSection(c, "sec-timer", "Timer", func() {
		if ctl.loading {
			muted(c, "Memuat timer rapat…").FontSize(12)
			return
		}
		if a.notes.timerErr != "" {
			ui.Text(c, a.notes.timerErr).FontSize(12).TextColor(t.Danger)
		}
		if tm == nil && !ctl.control {
			muted(c, "Timer rapat belum dimulai. Notulis atau pengelola rapat yang memulainya.").FontSize(12)
			return
		}
		if tm == nil {
			var planned int
			for _, it := range agenda {
				planned += it.minutes
			}
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				if outlineButton(c.Key("timer-start"), "Mulai timer", "play", true).Height(30).Disabled(a.notes.timerBusy).Clicked() {
					a.noteTimerAct(ctl, api.TimerStart, now)
				}
				detail := "opsional · hitung total waktu"
				if len(agenda) > 0 {
					detail = fmt.Sprintf("opsional · %d mnt untuk %d agenda", planned, len(agenda))
				}
				muted(c, detail).FontSize(12).MinWidth(0).Shrink(1)
			})
			if len(agenda) == 0 {
				muted(c, "Isi durasi di butir agenda bernomor (chip “mnt”) supaya timer mengikuti agenda.").FontSize(11)
			}
			muted(c, where).FontSize(11)
			return
		}
		c.After(time.Second)
		v := noteTimerState(*tm, now, agenda)
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.End).Children(func() {
			ui.Text(c, noteClock(v.elapsed)).Font(FontMono).FontSize(26).Bold()
			sub := "total"
			switch {
			case v.planned > 0 && v.agenda >= 0:
				sub = fmt.Sprintf("dari %s · Agenda %d", noteClock(v.planned), v.agenda+1)
			case v.finished:
				sub = fmt.Sprintf("dari %s · agenda selesai", noteClock(v.planned))
			}
			if v.paused {
				sub += " · dijeda"
			}
			muted(c, sub).FontSize(12).Padding(0, 0, 4, 0)
		})
		for i, it := range agenda {
			ui.Column(c.Key(fmt.Sprintf("agenda-%d", i))).FillWidth().MinWidth(0).Gap(4).Padding(3, 0).Children(func() {
				ui.Row(c).FillWidth().MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
					title := it.title
					if title == "" {
						title = fmt.Sprintf("Agenda %d", i+1)
					}
					color := t.Text
					if i < tm.agenda {
						color = faint(t)
					}
					ui.Text(c, fmt.Sprintf("%d. %s", i+1, title)).FontSize(13).TextColor(color).MaxLines(1).Grow(1).MinWidth(0)
					switch {
					case i < tm.agenda && i < len(tm.done):
						ui.Text(c, "✓ "+noteClock(tm.done[i])).Font(FontMono).FontSize(12).TextColor(t.Success)
					case i == tm.agenda && v.over > 0:
						ui.Text(c, "+"+noteClock(v.over)).Font(FontMono).FontSize(12).Bold().TextColor(t.Warning)
					case i == tm.agenda:
						ui.Text(c, noteLeft(v.left)+" tersisa").Font(FontMono).FontSize(12).Bold().TextColor(t.Accent)
					default:
						ui.Text(c, fmt.Sprintf("%d mnt", it.minutes)).FontSize(12).TextColor(faint(t))
					}
				})
				if i == tm.agenda && v.agendaPlanned > 0 {
					ratio := min(1, float32(v.agendaElapsed)/float32(v.agendaPlanned))
					color := t.Accent
					if v.over > 0 {
						color = t.Warning
					}
					ui.Box(c).FillWidth().Height(4).Radius(2).Background(spaceMix(t.Border, t.Surface, .4)).Children(func() {
						ui.Box(c).Height(4).Radius(2).Background(color).Width(ratio * 260)
					})
				}
			})
		}
		if ctl.control {
			ui.Row(c).FillWidth().Wrap().Gap(6).Padding(4, 0, 0, 0).Children(func() {
				a.noteTimerButtons(c, ctl, v, now, false)
				if outlineButton(c.Key("timer-stop"), "Hentikan timer", "close", false).Height(28).Disabled(a.notes.timerBusy).Clicked() {
					a.noteTimerAct(ctl, api.TimerStop, now)
				}
			})
		} else if ctl.meeting != nil && ctl.meeting.Timer != nil && ctl.meeting.Timer.UpdatedBy.DisplayName != "" {
			muted(c, "Diatur oleh "+ctl.meeting.Timer.UpdatedBy.DisplayName+".").FontSize(12)
		}
		if tm.hidden && spaceButton(c, "Tampilkan pil timer", "eye", false).Clicked() {
			a.setTimerHidden(ctl, false)
		}
		muted(c, where).FontSize(11)
	})
}

// noteTimerButtons: Jeda/Lanjut, and, for an agenda, +5 mnt and the next.
func (a *App) noteTimerButtons(c *ui.Context, ctl noteTimerCtl, v noteTimerView, now time.Time, pill bool) {
	busy := a.notes.timerBusy
	small := func(key, label, icon string, primary bool) ui.Element {
		if pill {
			return spaceIconButton(c.Key(key), label, icon).Disabled(busy)
		}
		return outlineButton(c.Key(key), label, icon, primary).Height(28).Disabled(busy)
	}
	if v.paused {
		if small("timer-resume", "Lanjutkan timer", "play", true).Clicked() {
			a.noteTimerAct(ctl, api.TimerResume, now)
		}
	} else if small("timer-pause", "Jeda timer", "pause", false).Clicked() {
		a.noteTimerAct(ctl, api.TimerPause, now)
	}
	if v.agenda < 0 || pill {
		return
	}
	if outlineButton(c.Key("timer-extend"), "+5 mnt", "", false).Height(28).Disabled(busy).Clicked() {
		a.noteTimerAct(ctl, api.TimerExtend, now)
	}
	if outlineButton(c.Key("timer-next"), noteNextAgendaLabel(v.agenda, len(a.noteAgendaItems())), "right", false).Height(28).Disabled(busy).Clicked() {
		a.noteTimerAct(ctl, api.TimerNext, now)
	}
}

func noteNextAgendaLabel(current, total int) string {
	if current+1 >= total {
		return "Selesaikan agenda"
	}
	return fmt.Sprintf("Agenda %d", current+2)
}

// noteTimerPill sticks above the document while the timer runs and is not
// hidden: right on a desktop, centred on a phone; yellow when the agenda's
// time is up.
func (a *App) noteTimerPill(c *ui.Context, phone bool) {
	ctl := a.noteTimerCtl()
	tm := ctl.tm
	if tm == nil || tm.hidden {
		return
	}
	t := c.Theme()
	now := c.Now()
	c.After(time.Second)
	agenda := a.noteAgendaItems()
	v := noteTimerState(*tm, now, agenda)
	over := v.over > 0
	ui.Row(c.Key("note-timer-pill-row")).FillWidth().MinWidth(0).Padding(8, 16, 0, 16).Justify(map[bool]ui.Align{true: ui.Center, false: ui.End}[phone]).Children(func() {
		pill := ui.Row(c.Key("note-timer-pill")).MinWidth(0).Shrink(1).Height(36).Padding(0, 4, 0, 12).Gap(8).AlignItems(ui.Center).Radius(18).Background(t.Surface).Border(1, t.Border).Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, .08)).Label("Timer rapat")
		if over {
			pill.Background(spaceMix(t.Warning, t.Surface, .82)).BorderColor(spaceMix(t.Warning, t.Border, .5))
		}
		pill.Children(func() {
			ui.Icon(c, spaceIcon("timer")).Size(15, 15).TextColor(map[bool]ui.Color{true: t.Warning, false: t.Accent}[over]).Shrink(0)
			switch {
			case over:
				label := fmt.Sprintf("Waktu agenda %d habis", v.agenda+1)
				if phone {
					label = "Habis"
				}
				ui.Text(c, label).FontSize(12).Bold().MaxLines(1).MinWidth(0).Shrink(1)
				ui.Text(c, "+"+noteClock(v.over)).Font(FontMono).FontSize(13).Bold().TextColor(t.Warning).Shrink(0)
			case v.agenda >= 0:
				title := v.title
				if title == "" || phone {
					title = fmt.Sprintf("Agenda %d", v.agenda+1)
				}
				ui.Text(c, title).FontSize(12).MaxLines(1).MinWidth(0).Shrink(1)
				ui.Text(c, noteLeft(v.left)).Font(FontMono).FontSize(13).Bold().Shrink(0)
				ui.Text(c, "· "+noteClock(v.elapsed)).Font(FontMono).FontSize(12).TextColor(faint(t)).Shrink(0)
			default:
				label := "Rapat"
				if v.finished {
					label = "Agenda selesai"
				}
				ui.Text(c, label).FontSize(12).MaxLines(1).MinWidth(0).Shrink(1)
				ui.Text(c, noteClock(v.elapsed)).Font(FontMono).FontSize(13).Bold().Shrink(0)
			}
			if v.paused {
				ui.Text(c, "dijeda").FontSize(11).TextColor(faint(t)).Shrink(0)
			}
			switch {
			case !ctl.control:
			case over:
				if outlineButton(c.Key("pill-extend"), "+5 mnt", "", false).Height(28).Disabled(a.notes.timerBusy).Clicked() {
					a.noteTimerAct(ctl, api.TimerExtend, now)
				}
				if outlineButton(c.Key("pill-next"), noteNextAgendaLabel(v.agenda, len(agenda))+" →", "", true).Height(28).Disabled(a.notes.timerBusy).Clicked() {
					a.noteTimerAct(ctl, api.TimerNext, now)
				}
			default:
				a.noteTimerButtons(c, ctl, v, now, true)
			}
			if spaceIconButton(c.Key("pill-hide"), "Sembunyikan timer", "close").Clicked() {
				a.setTimerHidden(ctl, true)
			}
		})
	})
}

// minutesChip: the planned minutes of an agenda item (numbered list).
func (e *noteDocumentEditor) minutesChip(c *ui.Context, index, i int, it api.NoteListItem, shown bool) {
	if it.Minutes == 0 && !(e.editable && shown) {
		return
	}
	t := c.Theme()
	label := "mnt"
	if it.Minutes > 0 {
		label = fmt.Sprintf("%d mnt", it.Minutes)
	}
	b := ui.ButtonBase(c.Key("minutes-chip")).Label(fmt.Sprintf("Durasi butir %d blok %d", i+1, index+1)).Tooltip("Durasi agenda untuk timer").Row().Height(24).Padding(0, 8).Gap(5).AlignItems(ui.Center).Radius(12).Shrink(0).TextColor(faint(t)).Disabled(!e.editable)
	if it.Minutes > 0 {
		b.Background(spaceMix(t.Accent, t.Surface, .9)).TextColor(t.Accent)
	} else {
		b.Border(1, t.Border)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		ui.Icon(c, spaceIcon("timer")).Size(12, 12)
		ui.Text(c, label).FontSize(12).MaxLines(1)
	})
	if e.editable && b.Clicked() {
		e.togglePick(index, i, notePickMinutes)
	}
	if !e.editable || e.pick.kind != notePickMinutes || e.pick.block != index || e.pick.item != i {
		return
	}
	open := true
	set := func(m int) {
		if index < len(e.blocks) && e.blocks[index].Type == api.BlockNumberedList && i < len(e.blocks[index].Items) {
			e.blocks[index].Items[i].Minutes, e.changed = m, true
		}
		e.pick = notePick{}
	}
	popover(c, b, &open, 240, func() {
		ui.Text(c, "Durasi agenda").FontSize(12).Bold()
		ui.Row(c).FillWidth().Wrap().Gap(5).Children(func() {
			for _, m := range []int{5, 10, 15, 20, 30, 45, 60} {
				if spaceButton(c.Key(m), fmt.Sprintf("%d mnt", m), "", it.Minutes == m).Clicked() {
					set(m)
				}
			}
		})
		if it.Minutes > 0 && spaceButton(c, "Hapus durasi", "close", false).FillWidth().Clicked() {
			set(0)
		}
	})
	if !open {
		e.pick = notePick{}
	}
}

const notePickMinutes = "minutes"
