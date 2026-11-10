package server

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// rawGet fetches path without a session, as a stranger with the link.
func rawGet(t *testing.T, o *taskOrg, path string) (int, http.Header, string) {
	t.Helper()
	resp, err := http.Get(o.url + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(b)
}

func publicToken(t *testing.T, url string) string {
	t.Helper()
	tok, ok := strings.CutPrefix(url, "http://space.test/n/")
	if !ok || !publicTokenRe.MatchString(tok) {
		t.Fatalf("public url %q", url)
	}
	return tok
}

func TestNotePublicLinkAccessAndPrivacy(t *testing.T) {
	o := newTaskOrg(t)
	sid := o.spaces["division-mediatek"]
	other, _ := o.join(o.kadiv, o.period.ID, "mediatek", "note-other@himasif.test", "Anggota Lain")
	due := timeutil.Date{Year: 2026, Month: 11, Day: 20}
	task := id.New()
	content := []api.NoteBlock{
		{Type: api.BlockParagraph, Spans: []api.Span{{Text: "Hadir: "}, {Text: "@PJ Mediatek", Mention: &api.Mention{Kind: api.MentionUser, ID: o.memberID}},
			{Text: " lihat "}, {Text: "board", Link: "http://space.test/spaces"}, {Text: " dan "}, {Text: "situs", Link: "https://example.org/acara"}}},
		{Type: api.BlockChecklist, Items: []api.NoteListItem{{Spans: []api.Span{{Text: "Kirim poster"}}, AssigneeID: &o.memberID, DueOn: &due, TaskID: &task}}},
	}
	var note api.Note
	if c := o.call("POST", "/api/spaces/"+sid.String()+"/notes", o.member, api.NoteCreateRequest{Title: "Rapat rutin #13", Content: content}, &note); c != 201 {
		t.Fatalf("create = %d", c)
	}
	nid := note.ID.String()
	if !note.Permissions.Share || note.PublicLink != nil {
		t.Fatalf("writer share=%v link=%+v", note.Permissions.Share, note.PublicLink)
	}
	// Only the kadiv or the writer may share; another member may not, and a
	// reader outside the division sees nothing at all.
	var seen api.Note
	o.call("GET", "/api/notes/"+nid, other, nil, &seen)
	if seen.Permissions.Share || !seen.Permissions.Manage {
		t.Fatalf("other member %+v", seen.Permissions)
	}
	if c, _ := o.errCode("POST", "/api/notes/"+nid+"/public-link", other, nil); c != 403 {
		t.Fatalf("other member shared: %d", c)
	}
	if c, _ := o.errCode("GET", "/api/notes/"+nid, o.peer, nil); c != 404 {
		t.Fatalf("outside reader = %d", c)
	}
	if c, _ := o.errCode("POST", "/api/notes/"+nid+"/public-link", o.peer, nil); c != 404 && c != 403 {
		t.Fatalf("outside sharer = %d", c)
	}
	var kn api.Note
	o.call("GET", "/api/notes/"+nid, o.kadiv, nil, &kn)
	if !kn.Permissions.Share {
		t.Fatal("kadiv cannot share")
	}

	// Public: the URL once, never in the note again.
	var made api.NotePublicLinkCreated
	if c := o.call("POST", "/api/notes/"+nid+"/public-link", o.member, nil, &made); c != 201 {
		t.Fatalf("share = %d", c)
	}
	tok := publicToken(t, made.URL)
	c, _, body := rawGet(t, o, "/api/notes/"+nid)
	if c != 401 && c != 403 {
		t.Fatalf("note without session = %d", c)
	}
	var again api.Note
	o.call("GET", "/api/notes/"+nid, o.member, nil, &again)
	if again.PublicLink == nil || again.PublicLink.CreatedBy.ID != o.memberID {
		t.Fatalf("link not shown to members: %+v", again.PublicLink)
	}
	var rawNote map[string]any
	o.call("GET", "/api/notes/"+nid, o.kadiv, nil, &rawNote)
	if strings.Contains(strings.ToLower(jsonText(t, rawNote)), strings.ToLower(tok)) {
		t.Fatal("token sent again with the note")
	}

	c, h, body := rawGet(t, o, "/api/public/notes/"+tok)
	if c != 200 || h.Get("X-Robots-Tag") == "" || !strings.Contains(h.Get("Cache-Control"), "no-store") || h.Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("public json %d %v", c, h)
	}
	for _, leak := range []string{o.memberID.String(), task.String(), "mention", "assigneeId", "taskId", "himasif.test", "space.test", "createdBy", "permissions", "lock"} {
		if strings.Contains(body, leak) {
			t.Fatalf("public json leaks %q: %s", leak, body)
		}
	}
	for _, want := range []string{"@PJ Mediatek", "https://example.org/acara", "Kirim poster", `"division":"Mediatek"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("public json lacks %q: %s", want, body)
		}
	}
	c, h, page := rawGet(t, o, "/n/"+tok)
	if c != 200 || !strings.Contains(page, `name="robots" content="noindex`) || !strings.Contains(page, "Rapat rutin #13") || strings.Contains(page, o.memberID.String()) || h.Get("Content-Security-Policy") == "" {
		t.Fatalf("public page %d %v", c, h)
	}

	// Autosaves reach public readers.
	var lock api.NoteLock
	if c := o.call("POST", "/api/notes/"+nid+"/lock", o.member, nil, &lock); c != 200 {
		t.Fatalf("lock = %d", c)
	}
	var saved api.Note
	if c := o.call("PUT", "/api/notes/"+nid, o.member, api.NoteWriteRequest{Title: "Rapat rutin #13", Content: []api.NoteBlock{para("Keputusan: Dies Natalis 2 Des")}, ExpectedVersion: note.Version}, &saved); c != 200 {
		t.Fatalf("save = %d", c)
	}
	if _, _, body = rawGet(t, o, "/api/public/notes/"+tok); !strings.Contains(body, "Dies Natalis 2 Des") {
		t.Fatalf("autosave not public: %s", body)
	}

	// Tags: tidied, shown publicly, bounded.
	var tags api.NoteTagsRequest
	if c := o.call("PUT", "/api/notes/"+nid+"/tags", o.member, api.NoteTagsRequest{Tags: []string{"Rapat Rutin", "#evaluasi", "rapat-rutin"}}, &tags); c != 200 || !slices.Equal(tags.Tags, []string{"rapat-rutin", "evaluasi"}) {
		t.Fatalf("tags = %d %v", c, tags.Tags)
	}
	if c, _ := o.errCode("PUT", "/api/notes/"+nid+"/tags", o.member, api.NoteTagsRequest{Tags: []string{"a/b"}}); c != 400 {
		t.Fatalf("bad tag = %d", c)
	}
	if c, _ := o.errCode("PUT", "/api/notes/"+nid+"/tags", o.peer, api.NoteTagsRequest{Tags: []string{"x"}}); c != 404 && c != 403 {
		t.Fatalf("outside tags = %d", c)
	}
	if _, _, body = rawGet(t, o, "/api/public/notes/"+tok); !strings.Contains(body, "rapat-rutin") {
		t.Fatalf("tags not public: %s", body)
	}

	// A new link ends the old one; turning it off ends it too. Every miss
	// is the same 404.
	var renewed api.NotePublicLinkCreated
	o.call("POST", "/api/notes/"+nid+"/public-link", o.kadiv, nil, &renewed)
	tok2 := publicToken(t, renewed.URL)
	if c, _, _ := rawGet(t, o, "/n/"+tok); c != 404 {
		t.Fatalf("old link after renew = %d", c)
	}
	if c, _, _ := rawGet(t, o, "/api/public/notes/"+tok2); c != 200 {
		t.Fatalf("new link = %d", c)
	}
	if c := o.call("DELETE", "/api/notes/"+nid+"/public-link", o.kadiv, nil, nil); c != 204 {
		t.Fatalf("revoke = %d", c)
	}
	for _, p := range []string{"/n/" + tok2, "/api/public/notes/" + tok2, "/n/" + strings.Repeat("A", 43), "/n/pendek"} {
		if c, h, _ := rawGet(t, o, p); c != 404 || h.Get("X-Robots-Tag") == "" {
			t.Fatalf("%s after revoke = %d", p, c)
		}
	}
	var after api.Note
	o.call("GET", "/api/notes/"+nid, o.member, nil, &after)
	if after.PublicLink != nil {
		t.Fatal("note still public")
	}
	for _, a := range []string{"note.public_link.create", "note.public_link.renew", "note.public_link.revoke", "note.tags.set"} {
		if o.audits(a) == 0 {
			t.Fatalf("no audit %s", a)
		}
	}

	// Closing the period closes the link; an archive grant to another
	// Space of the division does not open this division note.
	var last api.NotePublicLinkCreated
	o.call("POST", "/api/notes/"+nid+"/public-link", o.member, nil, &last)
	tok3 := publicToken(t, last.URL)
	var medhum id.ID = o.spaces["shared-medhum"]
	if medhum == id.Nil {
		for code, s := range o.spaces {
			if code != "division-mediatek" && strings.Contains(code, "medhum") {
				medhum = s
			}
		}
	}
	if c := o.call("POST", "/api/periods/"+o.period.ID.String()+"/close", o.admin, nil, nil); c != 200 {
		t.Fatalf("close = %d", c)
	}
	if c, _, _ := rawGet(t, o, "/n/"+tok3); c != 404 {
		t.Fatalf("link of a closed period = %d", c)
	}
	var next api.Period
	if c := o.call("POST", "/api/periods", o.admin, api.CreatePeriodRequest{Label: "2026/2027", StartsOn: o.period.EndsOn.AddDays(1), EndsOn: o.period.EndsOn.AddDays(365)}, &next); c != 201 {
		t.Fatalf("new period = %d", c)
	}
	o.call("POST", "/api/periods/"+next.ID.String()+"/activate", o.admin, nil, nil)
	newKadiv, person := o.join(o.admin, next.ID, "mediatek", "c6-kadiv@himasif.test", "Kadiv Baru")
	o.call("PUT", o.scope(next.ID, "mediatek")+"/kadiv", o.admin, api.SetKadivRequest{UserID: person.User.ID}, nil)
	reader, readerAcc := o.join(newKadiv, next.ID, "mediatek", "c6-reader@himasif.test", "Pembaca Arsip")
	if medhum == id.Nil {
		t.Fatalf("no Medhum space in %v", o.spaces)
	}
	if c := o.call("POST", o.scope(next.ID, "mediatek")+"/archives", newKadiv, api.ArchiveRequest{UserID: readerAcc.User.ID, SpaceID: medhum}, nil); c != 201 {
		t.Fatalf("archive grant = %d", c)
	}
	var opened api.Space
	if c := o.call("GET", "/api/spaces/"+medhum.String(), reader, nil, &opened); c != 200 || opened.Access != api.AccessArchive {
		t.Fatalf("archive grant did not open Medhum: %d", c)
	}
	var list api.List[api.Note]
	o.call("GET", "/api/spaces/"+medhum.String()+"/notes", reader, nil, &list)
	for _, it := range list.Items {
		if it.ID == note.ID {
			t.Fatal("division note listed in the archived Medhum")
		}
	}
	if c, _ := o.errCode("GET", "/api/notes/"+nid, reader, nil); c != 404 {
		t.Fatalf("archive reader of another space opened the note = %d", c)
	}
	if c, _ := o.errCode("POST", "/api/notes/"+nid+"/public-link", o.member, nil); c != 409 && c != 403 {
		t.Fatalf("shared in a closed period = %d", c)
	}
}

func TestNoteTemplatesAndMeetingFill(t *testing.T) {
	o := newTaskOrg(t)
	sid := o.spaces["division-mediatek"].String()
	var tmpls api.List[api.NoteTemplate]
	if c := o.call("GET", "/api/spaces/"+sid+"/note-templates", o.kadiv, nil, &tmpls); c != 200 {
		t.Fatalf("templates = %d", c)
	}
	var names []string
	for _, tm := range tmpls.Items {
		names = append(names, tm.Name)
	}
	for _, want := range []string{"Notulen rapat divisi", "Evaluasi", "Rapat singkat", "Kosong"} {
		if !slices.Contains(names, want) {
			t.Fatalf("templates %v lack %q", names, want)
		}
	}
	var ev api.Note
	if c := o.call("POST", "/api/spaces/"+sid+"/notes", o.member, api.NoteCreateRequest{Title: "Evaluasi Makrab", TemplateKind: api.TemplateEvaluation}, &ev); c != 201 || ev.TemplateKind != api.TemplateEvaluation {
		t.Fatalf("evaluation = %d %s", c, ev.TemplateKind)
	}

	starts := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	ends := starts.Add(time.Hour)
	var m api.Meeting
	req := api.MeetingRequest{Title: "Rapat rutin #13", Description: "1. Laporan divisi\n- Persiapan Dies Natalis\n\n• Evaluasi Makrab", Location: "Sekret HIMASIF",
		StartsAt: starts, EndsAt: &ends, TimeZone: "Asia/Jakarta", Participants: []id.ID{o.memberID}}
	if c := o.call("POST", "/api/spaces/"+sid+"/meetings", o.kadiv, req, &m); c != 201 {
		t.Fatalf("meeting = %d", c)
	}
	var n api.Note
	if c := o.call("POST", "/api/spaces/"+sid+"/notes", o.member, api.NoteCreateRequest{TemplateKind: api.TemplateMeeting, MeetingID: &m.ID}, &n); c != 201 {
		t.Fatalf("note from meeting = %d", c)
	}
	if n.Title != "Notulen Rapat rutin #13 — 17 Nov 2026" || n.TemplateKind != api.TemplateMeeting || n.MeetingID == nil || *n.MeetingID != m.ID {
		t.Fatalf("note %q %s %v", n.Title, n.TemplateKind, n.MeetingID)
	}
	var headings []string
	var agenda []string
	for _, b := range n.Content {
		if b.Type == api.BlockHeading {
			headings = append(headings, b.Text)
		}
		if b.Type == api.BlockNumberedList {
			for _, it := range b.Items {
				if it.Minutes != meetingAgendaMinutes || len(it.Spans) != 1 {
					t.Fatalf("agenda item %+v", it)
				}
				agenda = append(agenda, it.Spans[0].Text)
			}
		}
	}
	if !slices.Equal(headings, []string{"Agenda", "Pembahasan", "Keputusan", "Action item"}) || !slices.Equal(agenda, []string{"Laporan divisi", "Persiapan Dies Natalis", "Evaluasi Makrab"}) {
		t.Fatalf("headings %v agenda %v", headings, agenda)
	}
	if !strings.Contains(jsonText(t, n.Content[0]), "Sekret HIMASIF") || !strings.Contains(jsonText(t, n.Content[0]), "17 November 2026, 19.00") {
		t.Fatalf("meeting line %+v", n.Content[0])
	}
	var got api.Meeting
	o.call("GET", "/api/meetings/"+m.ID.String(), o.kadiv, nil, &got)
	if got.NoteID == nil || *got.NoteID != n.ID {
		t.Fatalf("meeting not linked: %v", got.NoteID)
	}
	if c, _ := o.errCode("POST", "/api/spaces/"+sid+"/notes", o.member, api.NoteCreateRequest{TemplateKind: api.TemplateMeeting, MeetingID: &m.ID}); c != 409 {
		t.Fatalf("second note for one meeting = %d", c)
	}
	if c, _ := o.errCode("POST", "/api/spaces/"+o.spaces["division-humas"].String()+"/notes", o.peer, api.NoteCreateRequest{TemplateKind: api.TemplateMeeting, MeetingID: &m.ID}); c != 400 {
		t.Fatalf("meeting of another space = %d", c)
	}
}

func jsonText(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
