package server

import (
	"encoding/json"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// Catatan rapat A, tahap C1: the server stores callout, toggle, action item
// fields, mentions and agenda minutes as sent, and refuses what it does not
// know or what breaks the rules.
func TestNoteC1BlocksRoundTripAndValidation(t *testing.T) {
	o := newOrg(t)
	kTok, kID := o.kadiv("mediatek", "km@himasif.test")
	sid := o.spaceID(kTok, "division-mediatek")
	task := id.New()
	due := timeutil.Date{Year: 2026, Month: 11, Day: 20}
	content := []api.NoteBlock{
		{Type: api.BlockHeading, Level: 2, Text: "Agenda", Minutes: 45},
		{Type: api.BlockNumberedList, Items: []api.NoteListItem{{Spans: []api.Span{{Text: "Evaluasi"}}, Minutes: 15}, {Spans: []api.Span{{Text: "Lain-lain"}}}}},
		{Type: api.BlockCallout, Variant: api.CalloutDecision, Spans: []api.Span{{Text: "PJ "}, {Text: "Kadiv", Mention: &api.Mention{Kind: api.MentionUser, ID: kID}}}},
		{Type: api.BlockChecklist, Items: []api.NoteListItem{{Spans: []api.Span{{Text: "Poster"}}, AssigneeID: &kID, DueOn: &due, TaskID: &task}}},
		{Type: api.BlockToggle, Text: "Pembahasan", Children: []api.NoteBlock{para("Detail"), {Type: api.BlockCallout, Variant: api.CalloutInfo, Spans: []api.Span{{Text: "Info"}}}}},
	}
	want, err := json.Marshal(content)
	if err != nil {
		t.Fatal(err)
	}
	var note api.Note
	if c := o.call("POST", "/api/spaces/"+sid.String()+"/notes", kTok, api.NoteCreateRequest{Title: "Notulen", Content: content}, &note); c != 201 {
		t.Fatalf("create = %d", c)
	}
	same := func(label string, blocks []api.NoteBlock) {
		t.Helper()
		got, err := json.Marshal(blocks)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Fatalf("%s changed the document:\n%s\n%s", label, want, got)
		}
	}
	same("create", note.Content)
	var read api.Note
	if c := o.call("GET", "/api/notes/"+note.ID.String(), kTok, nil, &read); c != 200 {
		t.Fatalf("read = %d", c)
	}
	same("read", read.Content)

	// Saving through the editor lock keeps it too.
	nid := note.ID.String()
	o.wantOK(t, "POST", "/api/notes/"+nid+"/lock", kTok, nil)
	var saved api.Note
	if c := o.call("PUT", "/api/notes/"+nid, kTok, api.NoteWriteRequest{Title: "Notulen", Content: content, ExpectedVersion: note.Version}, &saved); c != 200 {
		t.Fatalf("save = %d", c)
	}
	same("save", saved.Content)

	// Refused: a type the server does not know (sent whole by a newer
	// client), a nested toggle, a mention without a target, action item
	// fields on a plain list.
	var unknown []api.NoteBlock
	if err := json.Unmarshal([]byte(`[{"type":"embed","url":"https://himasif.id"}]`), &unknown); err != nil {
		t.Fatal(err)
	}
	for name, blocks := range map[string][]api.NoteBlock{
		"unknown type":   unknown,
		"nested toggle":  {{Type: api.BlockToggle, Text: "a", Children: []api.NoteBlock{{Type: api.BlockToggle, Text: "b"}}}},
		"empty mention":  {{Type: api.BlockParagraph, Spans: []api.Span{{Text: "x", Mention: &api.Mention{Kind: api.MentionUser}}}}},
		"assignee on ul": {{Type: api.BlockBulletedList, Items: []api.NoteListItem{{Spans: []api.Span{{Text: "x"}}, AssigneeID: &kID}}}},
		"bad variant":    {{Type: api.BlockCallout, Variant: "warning", Spans: []api.Span{{Text: "x"}}}},
	} {
		if c, _ := o.errCode("POST", "/api/spaces/"+sid.String()+"/notes", kTok, api.NoteCreateRequest{Title: "x", Content: blocks}); c != 400 {
			t.Errorf("create with %s = %d, want 400", name, c)
		}
		if c, _ := o.errCode("PUT", "/api/notes/"+nid, kTok, api.NoteWriteRequest{Title: "x", Content: blocks, ExpectedVersion: saved.Version}); c != 400 {
			t.Errorf("save with %s = %d, want 400", name, c)
		}
	}
}

func (o *org) wantOK(t *testing.T, method, path, tok string, body any) {
	t.Helper()
	if c := o.call(method, path, tok, body, nil); c < 200 || c > 299 {
		t.Fatalf("%s %s = %d", method, path, c)
	}
}
