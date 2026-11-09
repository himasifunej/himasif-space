package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// Blocks of canvas Catatan rapat A, tahap C1: callout, toggle, action items,
// mentions and agenda minutes.
func c1Document(t *testing.T) []NoteBlock {
	t.Helper()
	user, task := id.New(), id.New()
	due, err := timeutil.ParseDate("2026-11-20")
	if err != nil {
		t.Fatal(err)
	}
	return []NoteBlock{
		{Type: BlockHeading, Level: 2, Text: "Agenda", Minutes: 45},
		{Type: BlockNumberedList, Items: []NoteListItem{{Spans: []Span{{Text: "Evaluasi konten"}}, Minutes: 15}, {Spans: []Span{{Text: "Rencana Desember"}}}}},
		{Type: BlockCallout, Variant: CalloutDecision, Spans: []Span{{Text: "Posting tiap Senin. PJ: "}, {Text: "Aruna", Mention: &Mention{Kind: MentionUser, ID: user}}}},
		{Type: BlockCallout, Variant: CalloutInfo, Spans: []Span{{Text: "Lihat "}, {Text: "#T12", Mention: &Mention{Kind: MentionTask, ID: task}}}},
		{Type: BlockChecklist, Items: []NoteListItem{{Spans: []Span{{Text: "Desain poster"}}, AssigneeID: &user, DueOn: &due, TaskID: &task}, {Spans: []Span{{Text: "Cek grup"}}, Checked: true}}},
		{Type: BlockToggle, Text: "Pembahasan", Children: []NoteBlock{
			{Type: BlockParagraph, Spans: []Span{{Text: "Detail diskusi"}}},
			{Type: BlockCallout, Variant: CalloutDecision, Spans: []Span{{Text: "Setuju"}}},
		}},
		{Type: BlockToggle, Text: ""},
	}
}

func TestValidateNoteContentC1Blocks(t *testing.T) {
	doc := c1Document(t)
	if err := ValidateNoteContent(doc); err != nil {
		t.Fatalf("valid C1 document rejected: %v", err)
	}
	user := id.New()
	zero := timeutil.Date{}
	item := func(it NoteListItem) []NoteListItem {
		it.Spans = []Span{{Text: "x"}}
		return []NoteListItem{it}
	}
	bad := []struct {
		name  string
		block NoteBlock
	}{
		{"callout without variant", NoteBlock{Type: BlockCallout, Spans: []Span{{Text: "x"}}}},
		{"callout unknown variant", NoteBlock{Type: BlockCallout, Variant: "warning", Spans: []Span{{Text: "x"}}}},
		{"variant on paragraph", NoteBlock{Type: BlockParagraph, Variant: CalloutInfo, Spans: []Span{{Text: "x"}}}},
		{"callout with items", NoteBlock{Type: BlockCallout, Variant: CalloutInfo, Items: item(NoteListItem{})}},
		{"callout with children", NoteBlock{Type: BlockCallout, Variant: CalloutInfo, Children: []NoteBlock{{Type: BlockDivider}}}},
		{"nested toggle", NoteBlock{Type: BlockToggle, Text: "a", Children: []NoteBlock{{Type: BlockToggle, Text: "b"}}}},
		{"toggle child invalid", NoteBlock{Type: BlockToggle, Text: "a", Children: []NoteBlock{{Type: BlockHeading, Level: 9, Text: "x"}}}},
		{"toggle child unknown", NoteBlock{Type: BlockToggle, Text: "a", Children: []NoteBlock{{Type: "marquee"}}}},
		{"toggle title too long", NoteBlock{Type: BlockToggle, Text: strings.Repeat("x", NoteMaxHeadingText+1)}},
		{"toggle with spans", NoteBlock{Type: BlockToggle, Text: "a", Spans: []Span{{Text: "x"}}}},
		{"children on paragraph", NoteBlock{Type: BlockParagraph, Children: []NoteBlock{{Type: BlockDivider}}}},
		{"mention without id", NoteBlock{Type: BlockParagraph, Spans: []Span{{Text: "Aruna", Mention: &Mention{Kind: MentionUser}}}}},
		{"mention unknown kind", NoteBlock{Type: BlockParagraph, Spans: []Span{{Text: "x", Mention: &Mention{Kind: "division", ID: user}}}}},
		{"mention without fallback text", NoteBlock{Type: BlockParagraph, Spans: []Span{{Mention: &Mention{Kind: MentionUser, ID: user}}}}},
		{"mention with link", NoteBlock{Type: BlockParagraph, Spans: []Span{{Text: "x", Link: "https://himasif.id", Mention: &Mention{Kind: MentionUser, ID: user}}}}},
		{"assignee on bulleted list", NoteBlock{Type: BlockBulletedList, Items: item(NoteListItem{AssigneeID: &user})}},
		{"task on numbered list", NoteBlock{Type: BlockNumberedList, Items: item(NoteListItem{TaskID: &user})}},
		{"due on bulleted list", NoteBlock{Type: BlockBulletedList, Items: item(NoteListItem{DueOn: &timeutil.Date{Year: 2026, Month: 11, Day: 1}})}},
		{"empty assignee id", NoteBlock{Type: BlockChecklist, Items: item(NoteListItem{AssigneeID: &id.Nil})}},
		{"empty task id", NoteBlock{Type: BlockChecklist, Items: item(NoteListItem{TaskID: &id.Nil})}},
		{"zero due date", NoteBlock{Type: BlockChecklist, Items: item(NoteListItem{DueOn: &zero})}},
		{"minutes on checklist item", NoteBlock{Type: BlockChecklist, Items: item(NoteListItem{Minutes: 5})}},
		{"negative item minutes", NoteBlock{Type: BlockNumberedList, Items: item(NoteListItem{Minutes: -1})}},
		{"item minutes too long", NoteBlock{Type: BlockNumberedList, Items: item(NoteListItem{Minutes: NoteMaxAgendaMinutes + 1})}},
		{"heading minutes too long", NoteBlock{Type: BlockHeading, Level: 2, Text: "x", Minutes: NoteMaxAgendaMinutes + 1}},
		{"minutes on paragraph", NoteBlock{Type: BlockParagraph, Minutes: 5, Spans: []Span{{Text: "x"}}}},
	}
	for _, c := range bad {
		if err := ValidateNoteContent([]NoteBlock{c.block}); err == nil {
			t.Errorf("%s: expected an error", c.name)
		}
	}

	// Toggle children count towards NoteMaxBlocks.
	children := make([]NoteBlock, NoteMaxBlocks)
	for i := range children {
		children[i] = NoteBlock{Type: BlockDivider}
	}
	if err := ValidateNoteContent([]NoteBlock{{Type: BlockToggle, Text: "x", Children: children}}); err == nil {
		t.Error("toggle children beyond NoteMaxBlocks accepted")
	}
	if err := ValidateNoteContent([]NoteBlock{{Type: BlockToggle, Text: "x", Children: children[:NoteMaxBlocks-1]}}); err != nil {
		t.Errorf("toggle with %d children plus itself rejected: %v", NoteMaxBlocks-1, err)
	}
	// Callout text shares the per-block limit.
	if err := ValidateNoteContent([]NoteBlock{{Type: BlockCallout, Variant: CalloutInfo, Spans: []Span{{Text: strings.Repeat("x", NoteMaxBlockText+1)}}}}); err == nil {
		t.Error("oversized callout accepted")
	}
}

func TestNoteContentC1RoundTrip(t *testing.T) {
	doc := c1Document(t)
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var back []NoteBlock
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	again, err := json.Marshal(back)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(again) {
		t.Fatalf("round trip changed the document:\n%s\n%s", raw, again)
	}
	for _, want := range []string{`"variant":"decision"`, `"minutes":45`, `"minutes":15`, `"mention":{"kind":"user","id":"`, `"assigneeId":"`, `"dueOn":"2026-11-20"`, `"taskId":"`, `"children":[`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("JSON lacks %s: %s", want, raw)
		}
	}
	// Older documents keep their exact form: no new keys appear.
	old := `[{"type":"paragraph","spans":[{"text":"halo","bold":true}]},{"type":"checklist","items":[{"spans":[{"text":"a"}],"checked":true}]}]`
	var oldDoc []NoteBlock
	if err := json.Unmarshal([]byte(old), &oldDoc); err != nil {
		t.Fatal(err)
	}
	if out, _ := json.Marshal(oldDoc); string(out) != old {
		t.Errorf("old document changed:\n%s\n%s", old, out)
	}
}

// A block type this build does not know (from a newer server) is kept
// exactly, and the client may save the document with it; the server, whose
// rules are strict, still refuses a type it does not know.
func TestNoteContentKeepsUnknownBlocks(t *testing.T) {
	in := `[{"type":"paragraph","spans":[{"text":"sebelum"}]},{"type":"embed","url":"https://himasif.id/x","caption":[{"text":"c"}],"height":240},{"type":"toggle","text":"T","children":[{"type":"poll","options":["a","b"]}]}]`
	var doc []NoteBlock
	if err := json.Unmarshal([]byte(in), &doc); err != nil {
		t.Fatal(err)
	}
	if doc[1].Type != "embed" || len(doc[1].Raw) == 0 || !doc[1].Unsupported() {
		t.Fatalf("unknown block not kept: %+v", doc[1])
	}
	if doc[0].Unsupported() || doc[2].Unsupported() || !doc[2].Children[0].Unsupported() {
		t.Fatal("unsupported flag wrong")
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != in {
		t.Fatalf("unknown block changed on save:\n%s\n%s", in, out)
	}
	if err := ValidateNoteContentPreserving(doc); err != nil {
		t.Fatalf("client refuses to save a document with a kept block: %v", err)
	}
	if err := ValidateNoteContent(doc); err == nil {
		t.Fatal("server accepts an unknown block type")
	}
	// A locally made block of an unknown type is never accepted.
	if err := ValidateNoteContentPreserving([]NoteBlock{{Type: "embed"}}); err == nil {
		t.Fatal("client accepts an unknown type it made itself")
	}
	// Kept blocks still count towards the limits.
	many := make([]NoteBlock, NoteMaxBlocks+1)
	for i := range many {
		many[i] = doc[1]
	}
	if err := ValidateNoteContentPreserving(many); err == nil {
		t.Fatal("kept blocks escape NoteMaxBlocks")
	}
	huge := doc[1]
	huge.Raw = json.RawMessage(`{"type":"embed","blob":"` + strings.Repeat("x", NoteMaxContentBytes) + `"}`)
	if err := ValidateNoteContentPreserving([]NoteBlock{huge}); err == nil {
		t.Fatal("kept block escapes the size cap")
	}
	// Malformed JSON is still an error.
	if err := json.Unmarshal([]byte(`[{"type":"embed",`), &doc); err == nil {
		t.Fatal("malformed JSON accepted")
	}
}
