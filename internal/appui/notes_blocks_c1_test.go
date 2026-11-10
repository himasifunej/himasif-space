package appui

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// A document from a newer server: callout, toggle, an action item, a
// mention and agenda minutes this build knows, and an "embed" it does not.
const c1ServerDocument = `[` +
	`{"type":"heading","level":2,"text":"Agenda","minutes":45},` +
	`{"type":"numbered_list","items":[{"spans":[{"text":"Evaluasi"}],"minutes":15}]},` +
	`{"type":"callout","spans":[{"text":"Posting tiap Senin"}],"variant":"decision"},` +
	`{"type":"paragraph","spans":[{"text":"PJ "},{"text":"Aruna","mention":{"kind":"user","id":"01a1bc5e-2011-7e29-b6cb-9cd0fb387a57"}}]},` +
	`{"type":"checklist","items":[{"spans":[{"text":"Desain poster"}],"assigneeId":"01a1bc5e-2011-7e29-b6cb-9cd0fb387a57","dueOn":"2026-11-20","taskId":"01a1bc5e-2011-7e29-b6cb-9cd0fb387a58"}]},` +
	`{"type":"toggle","text":"Pembahasan","children":[{"type":"paragraph","spans":[{"text":"Detail diskusi"}]},{"type":"poll","options":["a","b"]}]},` +
	`{"type":"embed","url":"https://himasif.id/x","height":240}` +
	`]`

func c1Blocks(t *testing.T) []api.NoteBlock {
	t.Helper()
	var doc []api.NoteBlock
	if err := json.Unmarshal([]byte(c1ServerDocument), &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

// The editor shows what it cannot edit and saves the document exactly as
// the server sent it.
func TestNoteDocumentKeepsUnsupportedAndC1Blocks(t *testing.T) {
	e := newNoteDocumentEditor(c1Blocks(t))
	for _, editable := range []bool{false, true} {
		e.SetEditable(editable)
		tt := newNoteDocumentTester(e, 900, 4000)
		tt.Frame()
		// Editors type plain text in inputs (stage C3: callout and toggle
		// too), found by their labels.
		// Stage C4: text with a mention is typed inline too.
		item, callout, toggle, line, mention := "Desain poster", "Posting tiap Senin", "Pembahasan", "Detail diskusi", "Aruna"
		if editable {
			item, callout, toggle, line, mention = "Butir 1 blok 5", "Isi blok 3", "Judul toggle 6", "Isi toggle 6 baris 1", "Paragraf 4"
		}
		for _, s := range []string{"Blok tidak didukung", "Keputusan", callout, toggle, line, mention, item} {
			if !tt.HasText(s) {
				t.Errorf("editable=%v: no %q in %q", editable, s, tt.Texts())
			}
		}
	}
	if e.TakeChanged() {
		t.Fatal("showing the document reported an edit")
	}
	out, err := json.Marshal(e.Content())
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != c1ServerDocument {
		t.Fatalf("saving changed the document:\n%s\n%s", c1ServerDocument, out)
	}
	if err := api.ValidateNoteContentPreserving(e.Content()); err != nil {
		t.Fatalf("the client refuses to save it: %v", err)
	}
}

func TestNoteDocumentCloneDoesNotAliasC1Fields(t *testing.T) {
	source := c1Blocks(t)
	original := cloneNoteDocument(source)
	e := newNoteDocumentEditor(source)
	source[3].Spans[1].Mention.ID = id.New()
	*source[4].Items[0].AssigneeID = id.New()
	source[4].Items[0].DueOn.Day = 1
	source[5].Children[0].Spans[0].Text = "diubah"
	source[6].Raw[2] = 'X'
	if !reflect.DeepEqual(e.Content(), original) {
		t.Fatal("the editor shares mentions, action item fields, children or kept blocks with its source")
	}
}

// Typing into a list item keeps its action item fields; a mention is not
// plain text, so the inline input never replaces it.
func TestNoteDocumentInlineEditingKeepsC1Fields(t *testing.T) {
	doc := c1Blocks(t)
	if notePlainSpans(doc[3].Spans) || notePlainSpans(doc[3].Spans[1:]) {
		t.Fatal("a mention counts as plain text and would be overwritten")
	}
	if !notePlainSpans(doc[4].Items[0].Spans) {
		t.Fatal("a plain action item is not editable inline")
	}
	e := newNoteDocumentEditor(doc)
	e.SetEditable(true)
	tt := newNoteDocumentTester(e, 900, 4000)
	tt.Frame()
	if err := tt.Click("Butir 1 blok 5"); err != nil {
		t.Fatal(err)
	}
	tt.Type(" final")
	got := e.Content()[4].Items[0]
	if got.Spans[0].Text != "Desain poster final" || got.AssigneeID == nil || got.DueOn == nil || got.TaskID == nil || *got.AssigneeID != *doc[4].Items[0].AssigneeID {
		t.Fatalf("inline edit lost the action item: %+v", got)
	}
	if !e.TakeChanged() {
		t.Fatal("the edit was not reported")
	}
}
