package appui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
)

func noteEditorFixtureContent() []api.NoteBlock {
	return []api.NoteBlock{
		{Type: api.BlockParagraph, Spans: []api.Span{{Text: "Konsep ", Bold: true}, {Text: "referensi", Link: "https://example.test/brief"}}},
		{Type: api.BlockHeading, Level: 2, Text: "Keputusan"},
		{Type: api.BlockBulletedList, Items: []api.NoteListItem{{Spans: []api.Span{{Text: "Poin pertama", Bold: true}}}, {Spans: []api.Span{{Text: "Poin kedua", Link: "https://example.test/dua"}}}}},
		{Type: api.BlockNumberedList, Items: []api.NoteListItem{{Spans: []api.Span{{Text: "Langkah pertama"}}}}},
		{Type: api.BlockChecklist, Items: []api.NoteListItem{{Spans: []api.Span{{Text: "Kirim konsep"}}, Checked: true}, {Spans: []api.Span{{Text: "Desain"}}}}},
		{Type: api.BlockTable, Columns: []string{"Tanggal", "Jam", "PJ"}, Rows: [][][]api.Span{{{{Text: "2026-11-20"}}, {{Text: "19.00"}}, {{Text: "Bima", Bold: true}, {Text: " (PJ)", Link: "https://example.test/pj"}}}}},
		{Type: api.BlockDivider},
	}
}

func TestNoteDocumentWorkingCopyAndReturnedSnapshotNeverAliasCanonicalContent(t *testing.T) {
	source := noteEditorFixtureContent()
	original := cloneNoteDocument(source)
	e := newNoteDocumentEditor(source)
	source[0].Spans[0].Text = "Perangkat lain"
	source[2].Items[0].Spans[0].Bold = false
	source[4].Items[0].Checked = false
	source[5].Columns[0] = "Hari"
	source[5].Rows[0][2][1].Link = "https://example.test/lain"
	if !reflect.DeepEqual(e.Content(), original) {
		t.Fatal("constructing an editor retained aliases into canonical API content")
	}
	snapshot := e.Content()
	snapshot[0].Spans[1].Link = ""
	snapshot[2].Items[1].Spans[0].Text = "Diubah di luar"
	snapshot[4].Items[1].Checked = true
	snapshot[5].Columns[1] = "Waktu"
	snapshot[5].Rows[0][0][0].Text = "2000-01-01"
	if !reflect.DeepEqual(e.Content(), original) || e.TakeChanged() {
		t.Fatal("a caller's snapshot mutation altered the working document or reported a user edit")
	}
}

func TestNoteDocumentReaderCannotChangeBlocksListsSpansOrTable(t *testing.T) {
	source := noteEditorFixtureContent()
	e := newNoteDocumentEditor(source)
	e.addBlock(api.BlockParagraph)
	e.deleteBlock()
	e.moveBlock(1)
	e.addSpan()
	e.deleteSpan()
	e.selectBlock(4)
	e.toggleItem(4, 0)
	e.addItem()
	e.deleteItem()
	e.moveItem(1)
	e.selectBlock(5)
	e.addRow()
	e.deleteRow()
	e.moveRow(1)
	e.addColumn()
	e.deleteColumn()
	if e.TakeChanged() || !reflect.DeepEqual(e.Content(), source) {
		t.Fatal("a read-only document accepted a mutation")
	}
	tt := newNoteDocumentTester(e, 880, 640)
	for _, label := range []string{"Tambah blok", "Hapus blok", "Tambah baris", "Teks bagian", "Tebal", "Checklist butir 1 blok 5"} {
		if _, exists := tt.Find(label); exists {
			t.Errorf("reader received write control %q", label)
		}
	}
	if !reflect.DeepEqual(e.Content(), source) || e.TakeChanged() {
		t.Fatal("rendering a reader normalized or changed source content")
	}
}

func TestNoteDocumentNativeSpanEditingPreservesOtherRichRuns(t *testing.T) {
	source := noteEditorFixtureContent()[:1]
	original := cloneNoteDocument(source)
	e := newNoteDocumentEditor(source)
	e.SetEditable(true)
	tt := newNoteDocumentTester(e, 360, 640)
	revealDocumentControl(t, tt, "Teks bagian", 360, 640)
	if err := tt.Click("Teks bagian"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Ctrl, ui.KeyA)
	tt.Type("Konsep diperjelas ")
	revealDocumentControl(t, tt, "Tebal", 360, 640)
	if err := tt.Click("Tebal"); err != nil {
		t.Fatal(err)
	}
	got := e.Content()
	if got[0].Spans[0].Text != "Konsep diperjelas " || got[0].Spans[0].Bold || got[0].Spans[1] != original[0].Spans[1] {
		t.Fatalf("editing one run lost its siblings' formatting or links: %+v", got[0].Spans)
	}
	if !reflect.DeepEqual(source, original) || !e.TakeChanged() || e.TakeChanged() {
		t.Fatal("native editing changed canonical content or failed to consume its changed flag")
	}
	e.SetEditable(false)
	tt.Frame()
	if _, exists := tt.Find("Teks bagian"); exists {
		t.Fatal("revoking edit mode retained a writable text input")
	}
}

func TestNoteDocumentTableAndListReorderingPreserveAllRichValues(t *testing.T) {
	rows := make([][][]api.Span, 11)
	for row := range rows {
		rows[row] = make([][]api.Span, 7)
		for column := range rows[row] {
			rows[row][column] = []api.Span{{Text: fmt.Sprintf("%d:%d", row, column), Bold: true}, {Text: " rincian", Link: "https://example.test/sel"}}
		}
	}
	rows[10][6][0].Text = "2026-11-20"
	source := []api.NoteBlock{{Type: api.BlockTable, Columns: []string{"A", "B", "C", "D", "E", "F", "G"}, Rows: rows}, noteEditorFixtureContent()[4]}
	original := cloneNoteDocument(source)
	e := newNoteDocumentEditor(source)
	e.SetEditable(true)
	e.page(0).rows, e.page(0).columns = 1, 1
	e.selectBlock(0)
	e.row, e.column = 10, 6
	e.moveRow(-1)
	e.addColumn()
	e.deleteColumn()
	e.addRow()
	e.deleteRow()
	got := e.Content()
	if len(got[0].Rows) != 11 || len(got[0].Columns) != 7 || !reflect.DeepEqual(got[0].Rows[9], original[0].Rows[10]) || !reflect.DeepEqual(got[0].Rows[10], original[0].Rows[9]) {
		t.Fatal("table pages or row/column operations discarded rich cells outside the visible page")
	}
	for row := 0; row < 9; row++ {
		if !reflect.DeepEqual(got[0].Rows[row], original[0].Rows[row]) {
			t.Fatalf("an unrelated table row %d was flattened or changed", row)
		}
	}
	e.selectBlock(1)
	e.moveItem(1)
	e.toggleItem(1, 0)
	e.moveBlock(-1)
	got = e.Content()
	if got[0].Type != api.BlockChecklist || got[0].Items[1].Spans[0].Text != "Kirim konsep" || !got[0].Items[1].Checked || !got[0].Items[0].Checked || got[1].Rows[9][6][0].Text != "2026-11-20" || got[1].Rows[9][6][1].Link != "https://example.test/sel" {
		t.Fatal("reordering lists or blocks changed checks, rich runs, links or date-like cell strings")
	}
	if !reflect.DeepEqual(source, original) || !e.TakeChanged() || e.TakeChanged() {
		t.Fatal("working-copy operations leaked into canonical content or lost their change signal")
	}
}

func TestNoteDocumentPaginationBoundsMountedControlsWithoutTruncatingContent(t *testing.T) {
	rows := make([][][]api.Span, api.NoteMaxTableRows)
	columns := make([]string, api.NoteMaxTableCols)
	for column := range columns {
		columns[column] = fmt.Sprintf("Kolom %d", column+1)
	}
	for row := range rows {
		rows[row] = make([][]api.Span, api.NoteMaxTableCols)
	}
	rows[len(rows)-1][len(columns)-1] = []api.Span{{Text: "Isi terakhir tetap ada", Bold: true}}
	blocks := make([]api.NoteBlock, api.NoteMaxBlocks)
	blocks[0] = api.NoteBlock{Type: api.BlockTable, Columns: columns, Rows: rows}
	for index := 1; index < len(blocks); index++ {
		blocks[index] = api.NoteBlock{Type: api.BlockParagraph, Spans: []api.Span{{Text: fmt.Sprintf("Blok %d", index+1)}}}
	}
	e := newNoteDocumentEditor(blocks)
	tt := newNoteDocumentTester(e, 880, 640)
	cells := 0
	for _, label := range tt.Texts() {
		if strings.HasPrefix(label, "Sel ") && strings.Contains(label, " blok ") {
			cells++
		}
	}
	if cells != noteRowsPerPage*noteColsPerPage {
		t.Fatalf("mounted %d table cells; the current row/column page should mount %d", cells, noteRowsPerPage*noteColsPerPage)
	}
	if tt.HasText("Isi terakhir tetap ada") || tt.HasText("Blok 500") {
		t.Fatal("off-page content was mounted eagerly")
	}
	if err := tt.Click("Blok berikutnya"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Blok 9") || e.TakeChanged() || !reflect.DeepEqual(e.Content(), blocks) {
		t.Fatal("pagination edited or truncated the document")
	}
}

func TestNoteDocumentExtractionCopiesOnlyChosenTextAndNeverEditsReader(t *testing.T) {
	content := []api.NoteBlock{noteEditorFixtureContent()[0], noteEditorFixtureContent()[5]}
	e := newNoteDocumentEditor(content)
	var text, target string
	e.OnExtract = func(selected, kind string) { text, target = selected, kind }
	tt := newNoteDocumentTester(e, 1000, 900)
	if err := tt.Click("Jadikan tugas dari blok 1"); err != nil {
		t.Fatal(err)
	}
	if text != "Konsep referensi" || target != api.NoteLinkTask {
		t.Fatal("block extraction did not copy the selected visible text")
	}
	revealDocumentControl(t, tt, "Jadikan reminder dari baris 1 blok 2", 1000, 900)
	if err := tt.Click("Jadikan reminder dari baris 1 blok 2"); err != nil {
		t.Fatal(err)
	}
	if text != "2026-11-20 | 19.00 | Bima (PJ)" || target != api.NoteLinkReminder || e.TakeChanged() || !reflect.DeepEqual(e.Content(), content) {
		t.Fatal("row extraction changed the note or included content outside the chosen row")
	}
}

func TestNoteDocumentNativeBlockDeletionHandlesLastVisibleBlock(t *testing.T) {
	e := newNoteDocumentEditor([]api.NoteBlock{{Type: api.BlockParagraph, Spans: []api.Span{{Text: "Hapus draf blok ini"}}}})
	e.SetEditable(true)
	tt := newNoteDocumentTester(e, 360, 640)
	if err := tt.Click("Hapus blok"); err != nil {
		t.Fatal(err)
	}
	if len(e.Content()) != 0 || !e.TakeChanged() || !tt.HasText("Catatan belum memiliki isi.") {
		t.Fatal("deleting the final block did not leave a usable empty editor")
	}
}

func newNoteDocumentTester(e *noteDocumentEditor, width, height int) *ui.Tester {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).FillWidth().MinWidth(0).Padding(12).Children(func() { e.View(c) })
		})
	}, width, height)
	tt.Frame()
	return tt
}

func revealDocumentControl(t *testing.T, tt *ui.Tester, label string, width, height int) {
	t.Helper()
	for range 12 {
		r, found := tt.Find(label)
		if !found {
			t.Fatalf("document control %q missing in %q", label, tt.Texts())
		}
		if r.X >= 0 && r.Y >= 0 && r.X+r.W <= float32(width)+1 && r.Y+r.H <= float32(height)+1 {
			return
		}
		dy := r.Y + r.H - float32(height) + 30
		if r.Y < 0 {
			dy = r.Y - 30
		}
		tt.Scroll(float32(width-16), float32(height-30), 0, dy)
		tt.Advance(300 * time.Millisecond)
		tt.Frame()
	}
	t.Fatalf("document control %q could not be reached in %dx%d", label, width, height)
}
