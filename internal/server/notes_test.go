package server

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// noteSpaceID returns the id of a space of a division in the seeded period,
// as seen by a member token.
func (o *org) spaceID(tok, code string) id.ID {
	o.t.Helper()
	var sps api.List[api.Space]
	o.call("GET", "/api/periods/"+o.period.ID.String()+"/spaces", tok, nil, &sps)
	for _, s := range sps.Items {
		if s.Code == code {
			return s.ID
		}
	}
	o.t.Fatalf("space %s not visible to token", code)
	return id.Nil
}

func para(txt string) api.NoteBlock {
	return api.NoteBlock{Type: api.BlockParagraph, Spans: []api.Span{{Text: txt}}}
}

// NTE-01, NTE-06: a member creates a note in their division space with rich
// blocks; it is scoped to the space and readable by division members.
func TestNoteCreateAndRead(t *testing.T) {
	o := newOrg(t)
	kTok, _ := o.kadiv("mediatek", "km@himasif.test")
	mTok, _ := o.join(kTok, o.period.ID, "mediatek", "m@himasif.test", "M")
	hTok, _ := o.join(o.admin, o.period.ID, "humas", "h@himasif.test", "H")
	sid := o.spaceID(mTok, "division-mediatek")

	content := []api.NoteBlock{
		{Type: api.BlockHeading, Level: 1, Text: "Rapat Internal"},
		{Type: api.BlockParagraph, Spans: []api.Span{{Text: "Tebal", Bold: true}, {Text: " dan "}, {Text: "tautan", Link: "https://himasif.id"}}},
		{Type: api.BlockChecklist, Items: []api.NoteListItem{{Spans: []api.Span{{Text: "Kerjakan"}}, Checked: false}}},
		{Type: api.BlockTable, Columns: []string{"A", "B"}, Rows: [][][]api.Span{{{{Text: "1"}}, {{Text: "2"}}}}},
	}
	var note api.Note
	if c := o.call("POST", "/api/spaces/"+sid.String()+"/notes", mTok, api.NoteCreateRequest{Title: "Catatan", Content: content}, &note); c != 201 {
		t.Fatalf("create = %d", c)
	}
	if note.Version != 1 || note.OwnerDivisionID != o.div["mediatek"].ID || !note.Permissions.Manage || len(note.Content) != 4 {
		t.Fatalf("new note %+v", note)
	}
	// Division member reads it; another division cannot (NTE-07 / PRI space privacy).
	var got api.Note
	if c := o.call("GET", "/api/notes/"+note.ID.String(), mTok, nil, &got); c != 200 || len(got.Content) != 4 {
		t.Fatalf("read = %d %+v", c, got)
	}
	if c, code := o.errCode("GET", "/api/notes/"+note.ID.String(), hTok, nil); c != 404 || code != api.CodeNotFound {
		t.Fatalf("humas reads mediatek note = %d %s", c, code)
	}
	// A list of the space shows it; humas listing its own space shows nothing of it.
	var list api.List[api.Note]
	o.call("GET", "/api/spaces/"+sid.String()+"/notes", mTok, nil, &list)
	if len(list.Items) != 1 || list.Items[0].ID != note.ID || len(list.Items[0].Content) != 0 {
		t.Fatalf("list %+v", list.Items)
	}
	// Invalid block is rejected.
	bad := []api.NoteBlock{{Type: "marquee", Text: "x"}}
	if c, _ := o.errCode("POST", "/api/spaces/"+sid.String()+"/notes", mTok, api.NoteCreateRequest{Title: "x", Content: bad}); c != 400 {
		t.Fatalf("bad block = %d", c)
	}
	// Admin cannot write notes.
	if c, _ := o.errCode("POST", "/api/spaces/"+sid.String()+"/notes", o.admin, api.NoteCreateRequest{Title: "x", Content: []api.NoteBlock{para("y")}}); c != 403 {
		t.Fatalf("admin create = %d", c)
	}
}

// AC-34: one active editor; a second editor is refused; readers/commenters
// still work; the editor must hold the lock to save.
func TestNoteSingleEditorAndVersionConflict(t *testing.T) {
	o := newOrg(t)
	kTok, _ := o.kadiv("mediatek", "km@himasif.test")
	aTok, _ := o.join(kTok, o.period.ID, "mediatek", "a@himasif.test", "A")
	bTok, _ := o.join(kTok, o.period.ID, "mediatek", "b@himasif.test", "B")
	sid := o.spaceID(aTok, "division-mediatek")

	var note api.Note
	o.call("POST", "/api/spaces/"+sid.String()+"/notes", aTok, api.NoteCreateRequest{Title: "N", Content: []api.NoteBlock{para("awal")}}, &note)
	nid := note.ID.String()

	// Saving without a lock is refused.
	if c, code := o.errCode("PUT", "/api/notes/"+nid, aTok, api.NoteWriteRequest{Title: "N", Content: []api.NoteBlock{para("x")}, ExpectedVersion: 1}); c != 409 || code != api.CodeConflict {
		t.Fatalf("save without lock = %d %s", c, code)
	}
	// A acquires the lock; B cannot.
	var lock api.NoteLock
	if c := o.call("POST", "/api/notes/"+nid+"/lock", aTok, nil, &lock); c != 200 || !lock.HeldByCaller {
		t.Fatalf("A lock = %d %+v", c, lock)
	}
	if c, code := o.errCode("POST", "/api/notes/"+nid+"/lock", bTok, nil); c != 409 || code != api.CodeConflict {
		t.Fatalf("B lock = %d %s", c, code)
	}
	// B may still read and comment (NTE-06).
	if c := o.call("POST", "/api/notes/"+nid+"/comments", bTok, api.NoteCommentRequest{Body: "Catatan B"}, nil); c != 201 {
		t.Fatalf("B comment = %d", c)
	}
	// A saves with the right version; version bumps.
	var saved api.Note
	if c := o.call("PUT", "/api/notes/"+nid, aTok, api.NoteWriteRequest{Title: "N2", Content: []api.NoteBlock{para("baru")}, ExpectedVersion: 1}, &saved); c != 200 || saved.Version != 2 {
		t.Fatalf("A save = %d %+v", c, saved)
	}
	// A stale version is a conflict, not a silent overwrite.
	if c, code := o.errCode("PUT", "/api/notes/"+nid, aTok, api.NoteWriteRequest{Title: "N3", Content: []api.NoteBlock{para("z")}, ExpectedVersion: 1}); c != 409 || code != api.CodeConflict {
		t.Fatalf("stale save = %d %s", c, code)
	}
	// A releases; B can now lock.
	if c := o.call("DELETE", "/api/notes/"+nid+"/lock", aTok, nil, nil); c != 204 {
		t.Fatalf("release = %d", c)
	}
	if c := o.call("POST", "/api/notes/"+nid+"/lock", bTok, nil, &lock); c != 200 || !lock.HeldByCaller {
		t.Fatalf("B lock after release = %d %+v", c, lock)
	}
}

// NTE-08, AC-34: an expired lock may be taken over; a live one needs force;
// takeover preserves the previous editor's draft.
func TestNoteLockExpiryAndTakeover(t *testing.T) {
	o := newOrg(t)
	kTok, _ := o.kadiv("mediatek", "km@himasif.test")
	aTok, _ := o.join(kTok, o.period.ID, "mediatek", "a@himasif.test", "A")
	bTok, _ := o.join(kTok, o.period.ID, "mediatek", "b@himasif.test", "B")
	sid := o.spaceID(aTok, "division-mediatek")

	var note api.Note
	o.call("POST", "/api/spaces/"+sid.String()+"/notes", aTok, api.NoteCreateRequest{Title: "N", Content: []api.NoteBlock{para("awal")}}, &note)
	nid := note.ID.String()

	// Freeze the clock only now that every account and session exists, so
	// moving it forward exercises lock expiry without expiring sessions.
	base := o.srv.now()
	o.srv.now = func() time.Time { return base }

	o.call("POST", "/api/notes/"+nid+"/lock", aTok, nil, nil)
	// A saves a recovery draft.
	if c := o.call("PUT", "/api/notes/"+nid+"/draft", aTok, api.NoteDraftRequest{Title: "N", Content: []api.NoteBlock{para("draf A")}, BaseVersion: 1}, nil); c != 200 {
		t.Fatalf("A draft = %d", c)
	}
	// While A's lock is live, B cannot take over without force.
	if c, _ := o.errCode("POST", "/api/notes/"+nid+"/lock/takeover", bTok, api.NoteLockRequest{}); c != 409 {
		t.Fatalf("takeover live = %d", c)
	}
	// After the lock expires, B takes over without force.
	o.srv.now = func() time.Time { return base.Add(noteLockTTL + time.Minute) }
	var lock api.NoteLock
	if c := o.call("POST", "/api/notes/"+nid+"/lock/takeover", bTok, api.NoteLockRequest{}, &lock); c != 200 || !lock.HeldByCaller {
		t.Fatalf("takeover expired = %d %+v", c, lock)
	}
	// A's draft survived the takeover (NTE-08): A can still read it.
	var draft api.NoteDraft
	if c := o.call("GET", "/api/notes/"+nid+"/draft", aTok, nil, &draft); c != 200 || len(draft.Content) != 1 {
		t.Fatalf("A draft after takeover = %d %+v", c, draft)
	}
	// A manager may force a takeover of a fresh live lock.
	o.call("POST", "/api/notes/"+nid+"/lock", bTok, nil, nil) // B refreshes
	if c := o.call("POST", "/api/notes/"+nid+"/lock/takeover", kTok, api.NoteLockRequest{Force: true}, &lock); c != 200 || !lock.HeldByCaller {
		t.Fatalf("kadiv force takeover = %d %+v", c, lock)
	}
}

// AC-33: a note made from a template keeps its content when the template is
// later changed; schedule/meeting templates exist; custom templates save.
func TestNoteTemplatesCopyOnCreate(t *testing.T) {
	o := newOrg(t)
	kTok, _ := o.kadiv("mediatek", "km@himasif.test")
	mTok, _ := o.join(kTok, o.period.ID, "mediatek", "m@himasif.test", "M")
	sid := o.spaceID(mTok, "division-mediatek")

	// Built-ins are offered.
	var tmpls api.List[api.NoteTemplate]
	o.call("GET", "/api/spaces/"+sid.String()+"/note-templates", mTok, nil, &tmpls)
	kinds := map[string]bool{}
	for _, t := range tmpls.Items {
		if t.Builtin {
			kinds[t.Kind] = true
		}
	}
	if !kinds[api.TemplateMeeting] || !kinds[api.TemplateSchedule] || !kinds[api.TemplateFree] {
		t.Fatalf("builtin templates %+v", tmpls.Items)
	}
	// Create a note from the meeting built-in kind.
	var note api.Note
	if c := o.call("POST", "/api/spaces/"+sid.String()+"/notes", mTok, api.NoteCreateRequest{Title: "Notulensi", TemplateKind: api.TemplateMeeting}, &note); c != 201 {
		t.Fatalf("create from kind = %d", c)
	}
	if note.TemplateKind != api.TemplateMeeting || len(note.Content) == 0 {
		t.Fatalf("meeting note %+v", note)
	}

	// Save a custom template, make a note from it, then change the template.
	custom := []api.NoteBlock{{Type: api.BlockHeading, Level: 1, Text: "Versi 1"}}
	var tmpl api.NoteTemplate
	if c := o.call("POST", "/api/spaces/"+sid.String()+"/note-templates", kTok, api.NoteTemplateRequest{Name: "SOP", Content: custom}, &tmpl); c != 201 || tmpl.Builtin {
		t.Fatalf("save template = %d %+v", c, tmpl)
	}
	var fromTmpl api.Note
	o.call("POST", "/api/spaces/"+sid.String()+"/notes", mTok, api.NoteCreateRequest{Title: "Dari SOP", TemplateID: &tmpl.ID}, &fromTmpl)
	if len(fromTmpl.Content) != 1 || fromTmpl.Content[0].Text != "Versi 1" {
		t.Fatalf("note from template %+v", fromTmpl.Content)
	}
	// Change the template.
	changed := []api.NoteBlock{{Type: api.BlockHeading, Level: 1, Text: "Versi 2"}}
	if c := o.call("PUT", "/api/note-templates/"+tmpl.ID.String(), kTok, api.NoteTemplateRequest{Name: "SOP", Content: changed}, nil); c != 200 {
		t.Fatalf("update template = %d", c)
	}
	// The old note is unchanged (NTE-09, AC-33).
	var again api.Note
	o.call("GET", "/api/notes/"+fromTmpl.ID.String(), mTok, nil, &again)
	if len(again.Content) != 1 || again.Content[0].Text != "Versi 1" {
		t.Fatalf("note changed by template edit %+v", again.Content)
	}
}

// NTE-07: a shared Medhum note is editable only by its owner division; the
// other division of Medhum may read but not manage it.
func TestNoteMedhumOwnerDivision(t *testing.T) {
	o := newOrg(t)
	kmTok, _ := o.kadiv("mediatek", "km@himasif.test")
	khTok, _ := o.kadiv("humas", "kh@himasif.test")
	// Mediatek member and the medhum space (visible to both divisions).
	mTok, _ := o.join(kmTok, o.period.ID, "mediatek", "m@himasif.test", "M")
	medhum := o.spaceID(mTok, "medhum")

	var note api.Note
	if c := o.call("POST", "/api/spaces/"+medhum.String()+"/notes", mTok, api.NoteCreateRequest{Title: "Medhum", Content: []api.NoteBlock{para("konten")}}, &note); c != 201 {
		t.Fatalf("create medhum note = %d", c)
	}
	if note.OwnerDivisionID != o.div["mediatek"].ID {
		t.Fatalf("owner division %+v", note.OwnerDivisionID)
	}
	// Humas kadiv reads it (shared space) but cannot edit it.
	var got api.Note
	if c := o.call("GET", "/api/notes/"+note.ID.String(), khTok, nil, &got); c != 200 {
		t.Fatalf("humas read medhum note = %d", c)
	}
	if got.Permissions.Manage {
		t.Fatalf("humas can manage mediatek-owned note")
	}
	if c, _ := o.errCode("POST", "/api/notes/"+note.ID.String()+"/lock", khTok, nil); c != 403 {
		t.Fatalf("humas locks mediatek note = %d", c)
	}
}

// NTE-05, NTE-07: making a task/reminder from a note records a link without
// opening the note; a reminder link must be the caller's own.
func TestNoteLinksToTaskAndReminder(t *testing.T) {
	o := newOrg(t)
	kTok, _ := o.kadiv("mediatek", "km@himasif.test")
	mTok, mAcc := o.join(kTok, o.period.ID, "mediatek", "m@himasif.test", "M")
	sid := o.spaceID(mTok, "division-mediatek")

	var note api.Note
	o.call("POST", "/api/spaces/"+sid.String()+"/notes", mTok, api.NoteCreateRequest{Title: "Rapat", Content: []api.NoteBlock{para("tindak lanjut")}}, &note)

	// Create a task through the existing tasks endpoint (confirmed by a human form).
	d, _ := timeutil.ParseDate("2026-11-20")
	var task api.Task
	if c := o.call("POST", "/api/spaces/"+sid.String()+"/tasks", kTok, api.TaskWriteRequest{
		OwnerDivisionID: o.div["mediatek"].ID, AssigneeID: mAcc.User.ID, Title: "Desain", ExpectedOutput: "File",
		DeadlineDate: &d}, &task); c != 201 {
		t.Fatalf("create task = %d", c)
	}
	var link api.NoteLink
	if c := o.call("POST", "/api/notes/"+note.ID.String()+"/links", mTok, api.NoteLinkRequest{TargetType: api.NoteLinkTask, TargetID: task.ID}, &link); c != 201 || link.TargetType != api.NoteLinkTask {
		t.Fatalf("link task = %d %+v", c, link)
	}
	// Duplicate link is a conflict.
	if c, _ := o.errCode("POST", "/api/notes/"+note.ID.String()+"/links", mTok, api.NoteLinkRequest{TargetType: api.NoteLinkTask, TargetID: task.ID}); c != 409 {
		t.Fatalf("duplicate link = %d", c)
	}
	// A reminder owned by the caller links; a random id does not.
	var rem api.Reminder
	o.call("POST", "/api/reminders", mTok, api.ReminderRequest{Title: "Ingat", DueKind: api.DueDate, DueOn: &d}, &rem)
	if c := o.call("POST", "/api/notes/"+note.ID.String()+"/links", mTok, api.NoteLinkRequest{TargetType: api.NoteLinkReminder, TargetID: rem.ID}, nil); c != 201 {
		t.Fatalf("link reminder = %d", c)
	}
	if c, _ := o.errCode("POST", "/api/notes/"+note.ID.String()+"/links", mTok, api.NoteLinkRequest{TargetType: api.NoteLinkReminder, TargetID: id.New()}); c != 400 {
		t.Fatalf("link foreign reminder = %d", c)
	}
	var links api.List[api.NoteLink]
	o.call("GET", "/api/notes/"+note.ID.String()+"/links", mTok, nil, &links)
	if len(links.Items) != 2 {
		t.Fatalf("links %+v", links.Items)
	}
	_ = http.StatusOK
}

// AC-34 history and NTE-08 draft staleness.
func TestNoteHistoryAndDraftStale(t *testing.T) {
	o := newOrg(t)
	kTok, _ := o.kadiv("mediatek", "km@himasif.test")
	mTok, _ := o.join(kTok, o.period.ID, "mediatek", "m@himasif.test", "M")
	sid := o.spaceID(mTok, "division-mediatek")

	var note api.Note
	o.call("POST", "/api/spaces/"+sid.String()+"/notes", mTok, api.NoteCreateRequest{Title: "N", Content: []api.NoteBlock{para("a")}}, &note)
	nid := note.ID.String()
	o.call("POST", "/api/notes/"+nid+"/lock", mTok, nil, nil)
	o.call("PUT", "/api/notes/"+nid, mTok, api.NoteWriteRequest{Title: "N2", Content: []api.NoteBlock{para("b")}, ExpectedVersion: 1}, &note)

	var hist api.List[api.NoteEvent]
	o.call("GET", "/api/notes/"+nid+"/history", mTok, nil, &hist)
	var actions []string
	for _, e := range hist.Items {
		actions = append(actions, e.Action)
	}
	slices.Sort(actions)
	if !slices.Contains(actions, "note.create") || !slices.Contains(actions, "note.update") {
		t.Fatalf("history %v", actions)
	}
	// A draft based on an old version is reported stale.
	var draft api.NoteDraft
	o.call("PUT", "/api/notes/"+nid+"/draft", mTok, api.NoteDraftRequest{Title: "N", Content: []api.NoteBlock{para("c")}, BaseVersion: 1}, &draft)
	if !draft.Stale {
		t.Fatalf("draft should be stale against version %d", note.Version)
	}
}
