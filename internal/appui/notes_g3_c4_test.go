package appui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// T1: the timer at a moment, from its stamps alone.
func TestNoteTimerState(t *testing.T) {
	agenda := []noteAgendaItem{{"Evaluasi", 15}, {"Konten", 20}, {"Piket", 10}}
	t0 := time.Date(2026, 11, 9, 23, 50, 0, 0, time.UTC)
	min := func(m float64) time.Duration { return time.Duration(m * float64(time.Minute)) }

	tm := noteTimer{started: t0}
	v := noteTimerState(tm, t0.Add(min(5)), agenda)
	if v.elapsed != min(5) || v.agenda != 0 || v.left != min(10) || v.over != 0 || v.planned != min(45) {
		t.Fatalf("running: %+v", v)
	}
	// Past midnight it counts on; a pause leaves its time out.
	tm.pause(t0.Add(min(14)))
	v = noteTimerState(tm, t0.Add(min(30)), agenda)
	if !v.paused || v.elapsed != min(14) || v.left != min(1) {
		t.Fatalf("paused: %+v", v)
	}
	tm.resume(t0.Add(min(30)))
	v = noteTimerState(tm, t0.Add(min(33)), agenda)
	if v.paused || v.elapsed != min(17) || v.over != min(2) || v.left != 0 {
		t.Fatalf("over: %+v", v)
	}
	// +5 mnt gives the agenda more time; the next agenda starts fresh.
	tm.extend()
	if v = noteTimerState(tm, t0.Add(min(33)), agenda); v.over != 0 || v.left != min(3) || v.agendaPlanned != min(20) {
		t.Fatalf("extended: %+v", v)
	}
	tm.next(t0.Add(min(36)))
	v = noteTimerState(tm, t0.Add(min(40)), agenda)
	if v.agenda != 1 || v.title != "Konten" || v.agendaElapsed != min(4) || v.left != min(16) || tm.done[0] != min(20) {
		t.Fatalf("next: %+v %v", v, tm.done)
	}
	tm.next(t0.Add(min(41)))
	tm.next(t0.Add(min(42)))
	if v = noteTimerState(tm, t0.Add(min(50)), agenda); !v.finished || v.agenda != -1 || v.elapsed != min(34) {
		t.Fatalf("finished: %+v", v)
	}
	// Without durations the timer counts the total only.
	if v = noteTimerState(noteTimer{started: t0}, t0.Add(min(3)), nil); v.agenda != -1 || v.finished || v.elapsed != min(3) || v.planned != 0 {
		t.Fatalf("no agenda: %+v", v)
	}
	if noteClock(min(8)+12*time.Second) != "08:12" || noteClock(time.Hour+2*time.Minute+5*time.Second) != "1:02:05" {
		t.Fatal("clock format")
	}
}

func TestNoteAgendaFromBlocks(t *testing.T) {
	got := noteAgenda([]api.NoteBlock{
		{Type: api.BlockHeading, Level: 2, Text: "Pembukaan", Minutes: 5},
		{Type: api.BlockNumberedList, Items: []api.NoteListItem{{Spans: []api.Span{{Text: "Poster"}}, Minutes: 15}, {Spans: []api.Span{{Text: "Tanpa durasi"}}}}},
		{Type: api.BlockBulletedList, Items: []api.NoteListItem{{Spans: []api.Span{{Text: "Bukan agenda"}}}}},
	})
	if len(got) != 2 || got[0] != (noteAgendaItem{"Pembukaan", 5}) || got[1] != (noteAgendaItem{"Poster", 15}) {
		t.Fatalf("agenda: %+v", got)
	}
}

// The Rapat tab starts the timer; the pill follows the agenda, turns
// yellow when its time is up, and hides without stopping.
func TestNoteTimerPanelAndPill(t *testing.T) {
	h := gayaCOpen(t, false, 1440, 900)
	// T1 is the timer of a note without a meeting; a meeting's note shows
	// the meeting's shared timer (T2, notes_t2_c7_test.go).
	h.app.notes.note.MeetingID = nil
	h.tt.Frame()
	h.want("Mulai timer")
	h.want("opsional · 45 mnt untuk 3 agenda")
	h.want("Timer hanya di perangkat ini.")
	h.dontWant("Timer rapat")
	h.click("Mulai timer")
	h.tt.Frame()
	h.want("Timer rapat")
	h.want("Evaluasi poster oprec")
	h.want("15:00 tersisa")
	h.tt.Advance(16 * time.Minute)
	h.tt.Frame()
	h.want("Waktu agenda 1 habis")
	h.want("+01:00")
	h.click("Agenda 2 →")
	h.tt.Frame()
	h.dontWant("Waktu agenda 1 habis")
	h.want("20:00 tersisa")
	h.click("Jeda timer")
	h.tt.Advance(5 * time.Minute)
	h.tt.Frame()
	h.want("20:00 tersisa")
	h.click("Sembunyikan timer")
	h.tt.Frame()
	h.dontWant("Timer rapat")
	h.want("Tampilkan pil timer") // still running in the Rapat tab
	h.click("Hentikan timer")
	h.tt.Frame()
	h.want("Mulai timer")
	if h.app.notes.note == nil || h.app.notes.dirty {
		t.Fatal("the timer must not change the note")
	}
}

// G3: undo works on the document; typing on in one block is one step, a
// new block a step of its own.
func TestNoteEditorUndoRedo(t *testing.T) {
	e := c3Editable(para("a"))
	t0 := time.Now()
	set := func(text string, at time.Duration) {
		e.blocks[0].Spans = []api.Span{{Text: text}}
		e.changed = true
		e.TakeChangedAt(t0.Add(at))
	}
	set("ab", 0)
	set("abc", 300*time.Millisecond)
	e.insertBlocks(0, false, []api.NoteBlock{para("baru")})
	e.TakeChangedAt(t0.Add(400 * time.Millisecond))
	set2 := func() string { return noteSpansText(e.blocks[0].Spans) }
	if !e.CanUndo() || !e.Undo() || len(e.blocks) != 1 || set2() != "abc" {
		t.Fatalf("undo the new block: %+v", e.blocks)
	}
	e.TakeChangedAt(t0.Add(500 * time.Millisecond))
	if !e.Undo() || set2() != "a" {
		t.Fatalf("undo the typing: %q", set2())
	}
	e.TakeChangedAt(t0.Add(600 * time.Millisecond))
	if e.Undo() {
		t.Fatal("nothing more to undo")
	}
	if !e.Redo() || set2() != "abc" {
		t.Fatalf("redo: %q", set2())
	}
	e.TakeChangedAt(t0.Add(700 * time.Millisecond))
	// A pause makes the next typing its own step; a new edit drops redo.
	set("abcd", 3*time.Second)
	if e.CanRedo() {
		t.Fatal("an edit keeps redo")
	}
	if !e.Undo() || set2() != "abc" {
		t.Fatalf("undo after a pause: %q", set2())
	}
	e.SetEditable(false)
	if e.CanUndo() || e.Undo() {
		t.Fatal("a reader undoes")
	}
}

// G3 on the page: Ctrl+Z reaches the document (and autosave sees it);
// Baca hides the block tools but keeps the editor lock.
func TestNoteUndoShortcutAndReadMode(t *testing.T) {
	h := gayaCOpen(t, true, 1440, 900)
	e := h.app.notes.editor
	h.click("Paragraf 6") // shortcuts reach the note while it has the focus
	h.tt.Frame()
	e.blocks[5].Spans = []api.Span{{Text: "Rapat berikutnya Selasa."}}
	e.changed = true
	h.tt.Frame()
	if !h.app.notes.dirty || !e.CanUndo() {
		t.Fatal("the edit was not seen")
	}
	h.tt.Key(ui.Ctrl, ui.KeyZ)
	h.tt.Frame()
	if got := noteSpansText(e.blocks[5].Spans); got != "Rapat berikutnya Senin 16 Nov di sekretariat." {
		t.Fatalf("Ctrl+Z: %q", got)
	}
	h.tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
	h.tt.Frame()
	if got := noteSpansText(e.blocks[5].Spans); got != "Rapat berikutnya Selasa." {
		t.Fatalf("Ctrl+Shift+Z: %q", got)
	}
	h.want("Urungkan (Ctrl+Z)")
	h.want("Atur blok 1")
	h.click("Mode baca")
	h.tt.Frame()
	h.dontWant("Atur blok 1")
	h.dontWant("Urungkan (Ctrl+Z)")
	h.want("Kamu memegang editor")
	h.want("Selesai mencatat")
	h.click("Mode edit")
	h.tt.Frame()
	h.want("Atur blok 1")
}

func TestNoteRespanKeepsMentions(t *testing.T) {
	sinta := &api.Mention{Kind: api.MentionUser, ID: id.New()}
	old := []api.Span{{Text: "PJ "}, {Text: "@Sinta", Mention: sinta}, {Text: " revisi"}}
	got := noteRespan(old, "PJ @Sinta revisi poster")
	if len(got) != 3 || got[1].Mention == nil || got[1].Text != "@Sinta" || got[2].Text != " revisi poster" || got[0].Text != "PJ " {
		t.Fatalf("kept: %+v", got)
	}
	// An edited mention becomes plain text.
	got = noteRespan(old, "PJ @Sintia revisi")
	if len(got) != 1 || got[0].Mention != nil || got[0].Text != "PJ @Sintia revisi" {
		t.Fatalf("edited: %+v", got)
	}
	if !noteInlineSpans(old) || noteInlineSpans([]api.Span{{Text: "x", Bold: true}}) {
		t.Fatal("inline spans")
	}
	for text, want := range map[string]string{"Halo @si": "si", "@": "", "a@b": "-", "Halo @Sinta N": "Sinta N", "@a\nb": "-"} {
		q, ok := noteMentionQuery([]api.Span{{Text: text}})
		if (want == "-") == ok || (ok && q != want) {
			t.Errorf("%q: %q %v", text, q, ok)
		}
	}
	if _, ok := noteMentionQuery([]api.Span{{Text: "@Sinta", Mention: sinta}, {Text: " "}}); ok {
		t.Error("a finished mention reopens the picker")
	}
}

// C4: "@" opens the picker; Enter puts the mention in, as a span.
func TestNoteMentionPicker(t *testing.T) {
	e := c3Editable(para(""))
	sinta := id.New()
	e.Mentions = func(q string) []noteMentionTarget {
		all := []noteMentionTarget{{kind: api.MentionUser, id: sinta, text: "@Sinta Notulis"}, {kind: api.MentionTask, id: id.New(), text: "#T22 Poster"}}
		out := []noteMentionTarget{}
		for _, m := range all {
			if q == "" || strings.Contains(strings.ToLower(m.text), strings.ToLower(q)) {
				out = append(out, m)
			}
		}
		return out
	}
	tt := c3Tester(e, 900, 700)
	c3Click(t, tt, "Paragraf 1")
	tt.Type("Halo @si")
	c3Frames(tt)
	if !tt.HasText("SEBUT ANGGOTA ATAU TUGAS") || !tt.HasText("@Sinta Notulis") {
		t.Fatalf("no picker: %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	c3Frames(tt)
	got := e.Content()[0].Spans
	if len(got) != 3 || got[0].Text != "Halo " || got[1].Mention == nil || got[1].Mention.ID != sinta || got[1].Text != "@Sinta Notulis" || got[2].Text != " " {
		t.Fatalf("mention: %+v", got)
	}
	if len(e.Content()) != 1 || tt.HasText("SEBUT ANGGOTA ATAU TUGAS") {
		t.Fatal("Enter made a block or kept the picker")
	}
}

// C4: an action item has PJ and tenggat chips; "Jadikan tugas" fills the
// task form with them and the task's id goes back into the item.
func TestNoteActionItemMakesTask(t *testing.T) {
	h := gayaCOpen(t, true, 1440, 900)
	a := h.app
	caps := append(a.access.spaces[0].Capabilities, api.CapTasksManage)
	a.access.spaces[0].Capabilities, a.access.spaces[0].Roles[0].Capabilities = caps, caps
	aruna, raka := id.New(), id.New()
	a.notes.membersFor, a.notes.membersPeriod = a.notes.note.OwnerDivisionID, a.periodID
	a.notes.members = []api.Member{{UserID: aruna, DisplayName: "Aruna Desain"}, {UserID: raka, DisplayName: "Raka Video"}}
	e := a.notes.editor
	due := timeutil.Date{Year: 2026, Month: 11, Day: 12}
	e.blocks[4].Items[0].AssigneeID, e.blocks[4].Items[0].DueOn = &raka, &due
	h.tt.Frame()
	h.want("Raka")
	h.want("12 Nov")
	h.want("Jadikan tugas butir 1 blok 5")
	// The PJ chip opens the picker.
	h.click("PJ butir 1 blok 5")
	h.tt.Frame()
	h.click("Pilih PJ Aruna Desain")
	h.tt.Frame()
	if it := e.blocks[4].Items[0]; it.AssigneeID == nil || *it.AssigneeID != aruna {
		t.Fatalf("PJ: %+v", it)
	}
	h.want("Aruna")
	// "Jadikan tugas" opens the form in the panel with PJ and tenggat.
	h.click("Jadikan tugas butir 1 blok 5")
	h.tt.Frame()
	h.want("Jadikan tugas")
	cv := a.notes.conversion
	if cv == nil || cv.kind != api.NoteLinkTask || cv.title != "Revisi poster oprec — Aruna" || cv.assignee != aruna || cv.date != "2026-11-12" || cv.source == nil {
		t.Fatalf("form: %+v", cv)
	}
	// The task made: its id goes into the item (the notulis moved it).
	e.insertBlocks(0, false, []api.NoteBlock{para("baru")})
	task := id.New()
	if !e.SetItemTask(4, 0, cv.source.text, task) {
		t.Fatal("item not found")
	}
	if it := e.blocks[5].Items[0]; it.TaskID == nil || *it.TaskID != task {
		t.Fatalf("taskId: %+v", it)
	}
	if e.SetItemTask(5, 0, cv.source.text, id.New()) {
		t.Fatal("a task replaced another")
	}
	if err := api.ValidateNoteContentPreserving(noteSaveContent(e.Content())); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	h.want("Buka tugas Tugas")
}

// SPACE_UI_CAPTURE_DIR opts into the two review PNGs: the notulis on a
// desktop with the timer running and an action item, and the phone with
// the pill.

func TestNotesG3TimerC4NativeCapture(t *testing.T) {
	if os.Getenv("SPACE_UI_CAPTURE_DIR") == "" {
		t.Skip("set SPACE_UI_CAPTURE_DIR for native screenshots")
	}
	prep := func(h *harness, phone bool) {
		a := h.app
		aruna := id.New()
		a.notes.membersFor, a.notes.membersPeriod = a.notes.note.OwnerDivisionID, a.periodID
		a.notes.members = []api.Member{{UserID: aruna, DisplayName: "Aruna Desain"}}
		due := timeutil.Date{Year: 2026, Month: 11, Day: 12}
		e := a.notes.editor
		e.blocks[4].Items[0].AssigneeID, e.blocks[4].Items[0].DueOn = &aruna, &due
		e.blocks[5].Spans = []api.Span{{Text: "Rapat berikutnya Senin 16 Nov; "}, {Text: "@Aruna Desain", Mention: &api.Mention{Kind: api.MentionUser, ID: aruna}}, {Text: " bawa draf poster."}}
		a.notes.note.Lock.ExpiresAt = time.Now().Add(3 * time.Hour) // still the notulis after the timer ran
		a.chrome.preferences.Theme = "light"
		a.chrome.preferences.Accent = "rose"
		a.notes.savedAt = time.Now()
		h.tt.Frame()
		if a.notes.sheetOpen = phone; phone {
			h.tt.Frame()
		}
		h.click("Mulai timer")
		a.notes.sheetOpen = false
		h.tt.Advance(17 * time.Minute)
		h.tt.Frame()
	}
	d := gayaCOpen(t, true, 1440, 900)
	prep(d, false)
	captureNativeUI(t, d, "catatan-g3-timer-c4-desktop")
	p := gayaCOpen(t, true, 390, 844)
	prep(p, true)
	captureNativeUI(t, p, "catatan-g3-timer-c4-hp")
}
