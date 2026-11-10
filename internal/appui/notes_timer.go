package appui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
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
	tm := a.noteTimer()
	notePanelSection(c, "sec-timer", "Timer", func() {
		if tm == nil {
			var planned int
			for _, it := range agenda {
				planned += it.minutes
			}
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				if outlineButton(c.Key("timer-start"), "Mulai timer", "play", true).Height(30).Clicked() {
					a.startNoteTimer(now)
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
			muted(c, "Timer hanya di perangkat ini.").FontSize(11)
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
		ui.Row(c).FillWidth().Wrap().Gap(6).Padding(4, 0, 0, 0).Children(func() {
			a.noteTimerButtons(c, tm, v, now, false)
			if outlineButton(c.Key("timer-stop"), "Hentikan timer", "close", false).Height(28).Clicked() {
				a.stopNoteTimer()
			}
		})
		if tm.hidden && spaceButton(c, "Tampilkan pil timer", "eye", false).Clicked() {
			tm.hidden = false
		}
		muted(c, "Timer hanya di perangkat ini.").FontSize(11)
	})
}

// noteTimerButtons: Jeda/Lanjut, and, for an agenda, +5 mnt and the next.
func (a *App) noteTimerButtons(c *ui.Context, tm *noteTimer, v noteTimerView, now time.Time, pill bool) {
	small := func(key, label, icon string, primary bool) ui.Element {
		if pill {
			return spaceIconButton(c.Key(key), label, icon)
		}
		return outlineButton(c.Key(key), label, icon, primary).Height(28)
	}
	if v.paused {
		if small("timer-resume", "Lanjutkan timer", "play", true).Clicked() {
			tm.resume(now)
		}
	} else if small("timer-pause", "Jeda timer", "pause", false).Clicked() {
		tm.pause(now)
	}
	if v.agenda < 0 || pill {
		return
	}
	if outlineButton(c.Key("timer-extend"), "+5 mnt", "", false).Height(28).Clicked() {
		tm.extend()
	}
	if outlineButton(c.Key("timer-next"), noteNextAgendaLabel(v.agenda, len(a.noteAgendaItems())), "right", false).Height(28).Clicked() {
		tm.next(now)
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
	tm := a.noteTimer()
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
			if over {
				if outlineButton(c.Key("pill-extend"), "+5 mnt", "", false).Height(28).Clicked() {
					tm.extend()
				}
				if outlineButton(c.Key("pill-next"), noteNextAgendaLabel(v.agenda, len(agenda))+" →", "", true).Height(28).Clicked() {
					tm.next(now)
				}
			} else {
				a.noteTimerButtons(c, tm, v, now, true)
			}
			if spaceIconButton(c.Key("pill-hide"), "Sembunyikan timer", "close").Clicked() {
				tm.hidden = true
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
