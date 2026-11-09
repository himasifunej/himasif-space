package appui

import (
	"os"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func gayaCOpen(t *testing.T, editing bool, w, h int) *harness {
	t.Helper()
	hs, _ := c2Harness(t, editing)
	hs.app.notes.note.Comments = []api.NoteComment{{ID: id.New(), Author: api.PersonRef{ID: id.New(), DisplayName: "Nara Tekno"}, Body: "Ukuran teks minimal 48 px.", CreatedAt: time.Now().Add(-3 * time.Minute)}}
	hs.tt.SetSize(w, h)
	hs.app.router.Push(PathSpaces)
	hs.app.chrome.tab = "Catatan"
	hs.tt.Frame()
	return hs
}

// Gaya C (owner decision after pilot 0.3.5): the document holds only the
// title, chips and blocks; meeting, comments and history live in one right
// panel with tabs; the thin bar carries the lock and the save state.
func TestNoteGayaCPanelTabsOnDesktop(t *testing.T) {
	h := gayaCOpen(t, false, 1440, 900)
	for _, s := range []string{"Sinta Notulis sedang mencatat", "Diedit baru saja", "Sen 9 Nov · 19.30", "6 hadir · 1 izin", "1 komentar",
		"KEHADIRAN · 6 HADIR · 1 IZIN", "Laras Foto", "izin · kuliah pengganti", "DI HALAMAN INI", "Agenda", "LAMPIRAN · 0"} {
		h.want(s)
	}
	// Nothing of the old bottom cards in the document.
	for _, s := range []string{"Draf pemulihan milikmu", "Template ruang", "Riwayat catatan", "Simpan sekarang", "Hapus catatan", "Ukuran teks minimal 48 px."} {
		h.dontWant(s)
	}
	h.click("Komentar")
	h.tt.Frame()
	h.want("Ukuran teks minimal 48 px.")
	h.want("Kirim komentar catatan")
	h.dontWant("KEHADIRAN · 6 HADIR · 1 IZIN")

	// A draft shows only as a chip in the bar, which opens Riwayat.
	h.dontWant("Ada draf pemulihan")
	h.app.notes.localRecovery = &LocalNoteDraft{NoteID: h.app.notes.selected, SpaceID: h.app.notes.space, Title: "x", BaseVersion: 2, SavedAt: time.Now().Add(-10 * time.Minute)}
	h.tt.Frame()
	h.click("Ada draf pemulihan")
	h.tt.Frame()
	h.want("DRAF PEMULIHAN")
	h.want("Tinjau draf perangkat")

	// The panel can be closed and opened again from the bar.
	h.click("Tutup panel catatan")
	h.tt.Frame()
	h.dontWant("DRAF PEMULIHAN")
	h.click("Buka panel catatan")
	h.tt.Frame()
	h.want("DRAF PEMULIHAN")
}

// The notulis: lock chip with "Selesai mencatat" in the bar, no lease strip
// or save bar under the document; the conversion form opens in the panel.
func TestNoteGayaCEditorBarAndConversionInPanel(t *testing.T) {
	h := gayaCOpen(t, true, 1440, 900)
	h.want("Kamu memegang editor")
	h.want("Selesai mencatat")
	h.dontWant("Simpan sekarang")
	h.dontWant("Tersimpan otomatis beberapa detik setelah kamu berhenti mengetik.")
	h.app.notes.panelTab = notePanelKomentar
	h.app.beginNoteConversion(api.NoteLinkReminder, "Siapkan poster")
	h.tt.Frame()
	h.want("Jadikan reminder pribadi")
	h.want("Ukuran teks minimal 48 px.") // the tab stays under the form
}

// "Di halaman ini" jumps to a heading on another block page.
func TestNoteGayaCOutlineRevealsHeading(t *testing.T) {
	h := gayaCOpen(t, false, 1440, 900)
	blocks := []api.NoteBlock{}
	for i := 0; i < 12; i++ {
		blocks = append(blocks, para("Baris"))
	}
	blocks = append(blocks, api.NoteBlock{Type: api.BlockHeading, Level: 2, Text: "Tindak lanjut"})
	h.app.notes.editor = newNoteDocumentEditor(blocks)
	h.tt.Frame()
	if h.app.notes.editor.blockPage != 0 {
		t.Fatal("not on the first page")
	}
	h.click("Ke Tindak lanjut")
	h.tt.Frame()
	if e := h.app.notes.editor; e.blockPage != 1 || e.selected != 12 {
		t.Fatalf("outline: page %d selected %d", e.blockPage, e.selected)
	}
}

// Phone: chips under the title; the panel is one sheet with the same tabs.
func TestNoteGayaCPhoneSheet(t *testing.T) {
	h := gayaCOpen(t, false, 390, 844)
	h.want("6 hadir · 1 izin")
	h.dontWant("KEHADIRAN · 6 HADIR · 1 IZIN")
	h.click("6 hadir · 1 izin")
	h.tt.Frame()
	h.want("KEHADIRAN · 6 HADIR · 1 IZIN")
	h.click("Komentar")
	h.tt.Frame()
	h.want("Ukuran teks minimal 48 px.")
	h.click("Tutup lembar catatan")
	h.tt.Frame()
	h.dontWant("Ukuran teks minimal 48 px.")
	h.click("Rapat, komentar, riwayat")
	h.tt.Frame()
	h.want("Ukuran teks minimal 48 px.")
}

// The "/" menu in one column shows working shortcuts on the right.
func TestNoteSlashMenuShowsShortcuts(t *testing.T) {
	want := map[string]string{"h1": "#", "h2": "##", "h3": "###", "bullet": "-", "numbered": "1.", "checklist": "[]", "divider": "---", "decision": "/kep", "attendance": "/hadir", "action": "/tugas"}
	for _, m := range noteMenuEntries() {
		if s, ok := want[m.key]; ok && m.short != s {
			t.Errorf("%s shortcut %q; want %q", m.key, m.short, s)
		}
		// A "/word" shortcut must find its own entry first.
		if len(m.short) > 1 && m.short[0] == '/' {
			if got := noteMenuMatches(m.short[1:]); len(got) == 0 || got[0].key != m.key {
				t.Errorf("%s: /%s finds %v", m.key, m.short[1:], got)
			}
		}
	}
	e := c3Editable(para(""))
	tt := c3Tester(e, 900, 700)
	c3Click(t, tt, "Paragraf 1")
	tt.Type("---")
	c3Frames(tt)
	if got := e.Content(); len(got) != 2 || got[0].Type != api.BlockDivider || got[1].Type != api.BlockParagraph || !tt.Focused("Paragraf 2") {
		t.Fatalf("--- did not make a divider: %+v", got)
	}
}

// SPACE_UI_CAPTURE_DIR opts into the two review PNGs of gaya C: the notulis
// on a desktop with the Rapat panel, and the phone with the sheet open.
func TestNotesGayaCNativeCapture(t *testing.T) {
	if os.Getenv("SPACE_UI_CAPTURE_DIR") == "" {
		t.Skip("set SPACE_UI_CAPTURE_DIR for native screenshots")
	}
	shoot := func(h *harness, name string) {
		h.app.chrome.preferences.Theme = "light"
		h.app.chrome.preferences.Accent = "rose"
		h.app.notes.savedAt = time.Now()
		h.tt.Frame()
		h.tt.Advance(400 * time.Millisecond)
		h.tt.Frame()
		captureNativeUI(t, h, name)
	}
	shoot(gayaCOpen(t, true, 1440, 900), "catatan-gaya-c-desktop")
	p := gayaCOpen(t, true, 390, 844)
	p.app.notes.sheetOpen = true
	shoot(p, "catatan-gaya-c-hp")
}
