package appui

import (
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
)

// c3Tester draws an editor with room on the left for the hanging handles.
func c3Tester(e *noteDocumentEditor, width, height int) *ui.Tester {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).FillWidth().MinWidth(0).Padding(16, 16, 16, 72).Children(func() { e.View(c) })
		})
	}, width, height)
	tt.Frame()
	return tt
}

func c3Frames(tt *ui.Tester) {
	tt.Frame()
	tt.Advance(50 * time.Millisecond)
	tt.Frame()
}

func c3Click(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatalf("%v; texts %q", err, tt.Texts())
	}
	c3Frames(tt)
}

func c3Editable(blocks ...api.NoteBlock) *noteDocumentEditor {
	e := newNoteDocumentEditor(blocks)
	e.SetEditable(true)
	return e
}

func para(s string) api.NoteBlock {
	return api.NoteBlock{Type: api.BlockParagraph, Spans: []api.Span{{Text: s}}}
}

// The block menu puts the Notulen group (Action item, Keputusan, Kehadiran)
// above the basic blocks, and "/" filters it.
func TestNoteBlockMenuOrderAndFilter(t *testing.T) {
	all := noteMenuMatches("")
	if all[0].label != "Action item" || all[1].label != "Keputusan" || all[2].label != "Kehadiran" || all[3].group != noteMenuBasic {
		t.Fatalf("menu order: %s, %s, %s, %s", all[0].label, all[1].label, all[2].label, all[3].group)
	}
	for _, m := range all[:3] {
		if m.group != noteMenuNotulen {
			t.Fatalf("%s not in the Notulen group", m.label)
		}
	}
	for q, want := range map[string]string{"kep": "Keputusan", "agenda": "Daftar bernomor", "hadir": "Kehadiran", "lipat": "Toggle"} {
		if got := noteMenuMatches(q); len(got) == 0 || got[0].label != want {
			t.Errorf("/%s → %v; want %s first", q, got, want)
		}
	}
	if len(noteMenuMatches("zzz")) != 0 {
		t.Fatal("an unknown query matched")
	}
}

// Typing "/" at the start of an empty line opens the menu; Enter turns the
// line into the chosen block and the caret goes into it.
func TestNoteSlashMenuTurnsLineIntoBlock(t *testing.T) {
	e := c3Editable(para("Pembukaan"), para(""))
	tt := c3Tester(e, 900, 700)
	c3Click(t, tt, "Paragraf 2")
	tt.Type("/kep")
	c3Frames(tt)
	if !e.menu.open || e.menu.block != 1 || e.menu.query != "kep" || !tt.HasText("Keputusan") || !tt.HasText("/kep") {
		t.Fatalf("menu %+v; texts %q", e.menu, tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	c3Frames(tt)
	b := e.Content()[1]
	if len(e.Content()) != 2 || b.Type != api.BlockCallout || b.Variant != api.CalloutDecision || e.menu.open {
		t.Fatalf("after Enter: %+v menu %+v", e.Content(), e.menu)
	}
	if !tt.Focused("Isi blok 2") {
		t.Fatal("the caret did not move into the new decision")
	}
	tt.Type("Poster terbit Kamis")
	c3Frames(tt)
	if got := noteSpansText(e.Content()[1].Spans); got != "Poster terbit Kamis" || !e.TakeChanged() {
		t.Fatalf("decision text = %q", got)
	}
	// Escape closes the menu and keeps the typed text.
	c3Click(t, tt, "Paragraf 1")
	tt.Key(ui.Ctrl, ui.KeyA)
	tt.Type("/ta")
	c3Frames(tt)
	tt.Key(0, ui.KeyEscape)
	c3Frames(tt)
	if e.menu.open || noteSpansText(e.Content()[0].Spans) != "/ta" {
		t.Fatalf("Escape: menu %+v text %q", e.menu, noteSpansText(e.Content()[0].Spans))
	}
}

// "+" adds an empty line below with the menu open; a pick fills it.
func TestNotePlusHandleAddsBlockBelow(t *testing.T) {
	e := c3Editable(para("Satu"), para("Dua"))
	e.Attendance = func() []string { return []string{"Demo Mediatek — hadir", "Laras Foto — izin (kuliah pengganti)"} }
	tt := c3Tester(e, 900, 900)
	c3Click(t, tt, "Tambah blok di bawah blok 1")
	if len(e.Content()) != 3 || !e.menu.open || e.menu.block != 1 || !tt.HasText("Notulen") || !tt.HasText("Blok dasar") {
		t.Fatalf("after +: %d blocks, menu %+v", len(e.Content()), e.menu)
	}
	c3Click(t, tt, "Kehadiran")
	got := e.Content()
	if len(got) != 4 || got[1].Type != api.BlockHeading || got[1].Text != "Kehadiran" || got[2].Type != api.BlockBulletedList || len(got[2].Items) != 2 || noteSpansText(got[3].Spans) != "Dua" {
		t.Fatalf("attendance block: %+v", got)
	}
	c3Click(t, tt, "Tambah blok di bawah blok 4")
	c3Click(t, tt, "Action item")
	if got := e.Content(); got[4].Type != api.BlockChecklist || !tt.Focused("Butir 1 blok 5") {
		t.Fatalf("action item: %+v focused %v", got[4], tt.Focused("Butir 1 blok 5"))
	}
}

// Dragging "⠇" onto another block moves the block there; a click opens the
// block tools (move, delete, extract).
func TestNoteGripDragsBlockAndOpensTools(t *testing.T) {
	e := c3Editable(para("Satu"), para("Dua"), para("Tiga"))
	tt := c3Tester(e, 900, 700)
	g, ok := tt.Find("Atur blok 1")
	d, ok2 := tt.Find("Paragraf 3")
	if !ok || !ok2 {
		t.Fatal("grip or target missing")
	}
	tt.Press(g.X+g.W/2, g.Y+g.H/2)
	for s := float32(1); s <= 10; s++ {
		tt.Move(g.X+g.W/2, g.Y+g.H/2+(d.Y+4-g.Y-g.H/2)*s/10)
		tt.Frame()
	}
	tt.Release(g.X+g.W/2, d.Y+4)
	c3Frames(tt)
	order := []string{}
	for _, b := range e.Content() {
		order = append(order, noteSpansText(b.Spans))
	}
	if order[0] != "Dua" || order[1] != "Tiga" || order[2] != "Satu" || !e.TakeChanged() {
		t.Fatalf("order after drag = %v", order)
	}
	c3Click(t, tt, "Atur blok 1")
	for _, s := range []string{"Naikkan blok", "Turunkan blok", "Hapus blok"} {
		if !tt.HasText(s) {
			t.Fatalf("block tools miss %q", s)
		}
	}
}

// Enter makes the next item; Enter on the last empty item leaves the list
// for a new line; Backspace on an empty line goes back to the item above.
func TestNoteEnterAndBackspaceMoveBetweenBlocks(t *testing.T) {
	e := c3Editable(api.NoteBlock{Type: api.BlockBulletedList, Items: []api.NoteListItem{{Spans: []api.Span{{Text: "Poster"}}}}})
	tt := c3Tester(e, 900, 700)
	c3Click(t, tt, "Butir 1 blok 1")
	tt.Key(0, ui.KeyEnter)
	c3Frames(tt)
	if len(e.Content()[0].Items) != 2 || !tt.Focused("Butir 2 blok 1") {
		t.Fatalf("Enter in a list: %+v", e.Content()[0].Items)
	}
	tt.Key(0, ui.KeyEnter)
	c3Frames(tt)
	got := e.Content()
	if len(got) != 2 || len(got[0].Items) != 1 || got[1].Type != api.BlockParagraph || !tt.Focused("Paragraf 2") {
		t.Fatalf("Enter on the empty last item: %+v", got)
	}
	tt.Key(0, ui.KeyBackspace)
	c3Frames(tt)
	if len(e.Content()) != 1 || !tt.Focused("Butir 1 blok 1") {
		t.Fatalf("Backspace on an empty line: %d blocks, focus on item %v", len(e.Content()), tt.Focused("Butir 1 blok 1"))
	}
}

// Callout (decision) and toggle are made from the menu, edited in place,
// and what is saved passes the server's strict check; a heading still being
// named is saved as an empty line so autosave is not refused.
func TestNoteCalloutAndToggleAreCreatedAndSaved(t *testing.T) {
	e := c3Editable(para(""))
	tt := c3Tester(e, 900, 900)
	for _, m := range noteMenuEntries() {
		if m.key == "toggle" {
			e.menu = noteBlockMenu{block: 0, replace: true}
			e.applyMenu(m)
		}
	}
	c3Frames(tt)
	if !tt.Focused("Judul toggle 1") {
		t.Fatalf("toggle title not focused; texts %q", tt.Texts())
	}
	tt.Type("Pembahasan")
	c3Click(t, tt, "Isi toggle 1 baris 1")
	tt.Type("Detail diskusi")
	c3Click(t, tt, "Tambah baris di toggle 1")
	c3Click(t, tt, "Tutup toggle 1")
	if tt.HasText("Isi toggle 1 baris 1") {
		t.Fatal("a folded toggle still shows its lines")
	}
	c3Click(t, tt, "Buka toggle 1")
	c3Click(t, tt, "Tambah blok di bawah blok 1")
	c3Click(t, tt, "Judul 2")
	got := e.Content()
	if got[0].Type != api.BlockToggle || got[0].Text != "Pembahasan" || len(got[0].Children) != 2 || noteSpansText(got[0].Children[0].Spans) != "Detail diskusi" || got[1].Type != api.BlockHeading {
		t.Fatalf("content = %+v", got)
	}
	if err := api.ValidateNoteContent(noteSaveContent(e.Content())); err != nil {
		t.Fatalf("saved content refused: %v", err)
	}
	if e.Content()[1].Type != api.BlockHeading {
		t.Fatal("normalizing the saved copy changed the editor's heading")
	}
}

// No technical labels in the document, and reminder bells are not drawn
// all the time (they are there for the keyboard, drawn on hover).
func TestNoteDocumentHasNoTechnicalLabels(t *testing.T) {
	blocks := []api.NoteBlock{{Type: api.BlockHeading, Level: 2, Text: "Agenda"},
		{Type: api.BlockNumberedList, Items: []api.NoteListItem{{Spans: []api.Span{{Text: "Evaluasi"}}}, {Spans: []api.Span{{Text: "Konten"}}}}}, para("Penutup")}
	for _, editable := range []bool{true, false} {
		e := newNoteDocumentEditor(blocks)
		e.SetEditable(editable)
		e.OnExtract = func(string, string) {}
		tt := c3Tester(e, 900, 700)
		for _, s := range []string{"Format dan atur blok", "Butir blok 2 1–2 dari 2", "Blok 1–3 dari 3", "Isi catatan · hanya baca", "Tambah poin"} {
			if tt.HasText(s) {
				t.Errorf("editable %v: technical label %q shown", editable, s)
			}
		}
		if _, ok := tt.Find("Jadikan reminder dari butir 1 blok 2"); !ok {
			t.Errorf("editable %v: the item's reminder action is gone", editable)
		}
	}
}

// The phone's block bar: a decision replaces the empty line it is on;
// move up/down replaces dragging.
func TestNoteMobileBlockBar(t *testing.T) {
	e := c3Editable(para("Satu"), para(""))
	e.Compact = true
	e.selectBlock(1)
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() { e.View(c) })
			e.MobileBar(c)
		})
	}, 390, 700)
	tt.Frame()
	c3Click(t, tt, "Keputusan")
	if got := e.Content(); len(got) != 2 || got[1].Type != api.BlockCallout {
		t.Fatalf("bar decision: %+v", got)
	}
	c3Click(t, tt, "Naikkan blok")
	if got := e.Content(); got[0].Type != api.BlockCallout {
		t.Fatalf("bar move up: %+v", got)
	}
	c3Click(t, tt, "Tambah blok")
	if !e.menu.open || !tt.HasText("Action item") {
		t.Fatal("bar + did not open the block grid")
	}
}

// SPACE_UI_CAPTURE_DIR opts into PNGs of stage C3: the notulis hovering a
// block (handles), the "/" menu, a reader hovering an item (bell), and the
// phone with its block bar and the block grid.
func TestNotesC3NativeCapture(t *testing.T) {
	if os.Getenv("SPACE_UI_CAPTURE_DIR") == "" {
		t.Skip("set SPACE_UI_CAPTURE_DIR for native screenshots")
	}
	shoot := func(h *harness, name string) {
		h.tt.Frame()
		h.tt.Advance(400 * time.Millisecond)
		h.tt.Frame()
		captureNativeUI(t, h, name)
	}
	open := func(editing bool, w, hgt int) *harness {
		h, _ := c2Harness(t, editing)
		h.app.chrome.preferences.Theme = "light"
		h.app.chrome.preferences.Accent = "rose"
		h.app.router.Push(PathSpaces)
		h.app.chrome.tab = "Catatan"
		h.tt.SetSize(w, hgt)
		h.app.notes.savedAt = time.Now()
		h.tt.Frame()
		return h
	}
	h := open(true, 1440, 900)
	if r, ok := h.tt.Find("Butir 2 blok 2"); ok {
		h.tt.Move(r.X+40, r.Y+r.H/2)
	}
	shoot(h, "catatan-c3-desktop-hover")
	e := h.app.notes.editor
	e.blocks = append(e.blocks[:2], append([]api.NoteBlock{para("/")}, e.blocks[2:]...)...)
	e.menu = noteBlockMenu{open: true, block: 2, replace: true}
	e.focusOn(2, -1)
	h.tt.Move(1400, 880)
	shoot(h, "catatan-c3-desktop-menu")

	r := open(false, 1440, 900)
	if b, ok := r.tt.Find("Revisi poster oprec — Aruna"); ok {
		r.tt.Move(b.X+b.W/2, b.Y+b.H/2)
	}
	shoot(r, "catatan-c3-desktop-pembaca-hover")

	p := open(true, 390, 844)
	p.app.notes.editor.selectBlock(2)
	shoot(p, "catatan-c3-hp")
	pe := p.app.notes.editor
	pe.blocks = append(pe.blocks[:3], append([]api.NoteBlock{para("")}, pe.blocks[3:]...)...)
	pe.menu = noteBlockMenu{open: true, block: 3, replace: true}
	pe.selectBlock(3)
	shoot(p, "catatan-c3-hp-menu")
}
