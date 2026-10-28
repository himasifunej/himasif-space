package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// This literal is independent of DTO marshaling, so the wire assertions catch
// changed field names, dropped emphasis/links and rewritten date-like cells.
const noteDocumentJSON = `[{"type":"heading","level":2,"text":"Rapat"},{"type":"paragraph","spans":[{"text":"Caption ","bold":true},{"text":"referensi","link":"https://example.test/brief"}]},{"type":"checklist","items":[{"spans":[{"text":"Kirim konsep"}],"checked":true},{"spans":[{"text":"Desain"}]}]},{"type":"table","columns":["Tanggal","Jam","PJ"],"rows":[[[{"text":"2026-11-02"}],[{"text":"19.00"}],[{"text":"Bima"}]]]}]`

func noteClientDocument() []api.NoteBlock {
	return []api.NoteBlock{
		{Type: api.BlockHeading, Level: 2, Text: "Rapat"},
		{Type: api.BlockParagraph, Spans: []api.Span{{Text: "Caption ", Bold: true}, {Text: "referensi", Link: "https://example.test/brief"}}},
		{Type: api.BlockChecklist, Items: []api.NoteListItem{{Spans: []api.Span{{Text: "Kirim konsep"}}, Checked: true}, {Spans: []api.Span{{Text: "Desain"}}}}},
		{Type: api.BlockTable, Columns: []string{"Tanggal", "Jam", "PJ"}, Rows: [][][]api.Span{{{{Text: "2026-11-02"}}, {{Text: "19.00"}}, {{Text: "Bima"}}}}},
	}
}

func TestNoteClientWritesKeepDocumentVersionAndExplicitIntent(t *testing.T) {
	s, n, user, templateID, attachmentID, linkID, taskID := id.New(), id.New(), id.New(), id.New(), id.New(), id.New(), id.New()
	ctx := context.Background()
	at := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	doc := noteClientDocument()
	person := api.PersonRef{ID: user, DisplayName: "Ayu"}
	note := api.Note{ID: n, SpaceID: s, Title: "Notulensi", Content: doc, TemplateKind: api.TemplateMeeting, Version: 10, Permissions: api.NotePermissions{Manage: true, CanEdit: true, Comment: true}}
	lock := api.NoteLock{Editor: person, AcquiredAt: at, ExpiresAt: at.Add(5 * time.Minute), HeldByCaller: true}
	draft := api.NoteDraft{Title: "Draf", Content: doc, BaseVersion: 8, SavedAt: at, Stale: true}
	comment := api.NoteComment{ID: id.New(), Author: person, Body: "Konsep sudah diperiksa", CreatedAt: at}
	pending := api.NoteAttachment{ID: attachmentID, Name: "brief.pdf", ContentType: "application/pdf", SizeBytes: 120, Status: api.AttachmentPending, AddedBy: person, CreatedAt: at}
	ready := pending
	ready.URL, ready.Status = "https://example.test/brief.pdf", api.AttachmentReady
	link := api.NoteLink{ID: linkID, TargetType: api.NoteLinkTask, TargetID: taskID, CreatedBy: person, CreatedAt: at}
	template := api.NoteTemplate{ID: templateID, SpaceID: &s, Kind: api.TemplateCustom, Name: "Rapat divisi", Content: doc, CreatedAt: &at, UpdatedAt: &at}
	base := "/api/notes/" + n.String()
	templates := "/api/spaces/" + s.String() + "/note-templates"
	tests := []struct {
		name, method, path, body string
		status                   int
		response                 any
		call                     func(*Client) (any, error)
	}{
		{"create from document", "POST", "/api/spaces/" + s.String() + "/notes", `{"title":"Notulensi","templateKind":"meeting","content":` + noteDocumentJSON + `}`, 201, note,
			func(c *Client) (any, error) {
				return c.CreateNote(ctx, s, api.NoteCreateRequest{Title: "Notulensi", TemplateKind: api.TemplateMeeting, Content: doc})
			}},
		{"copy template by identity", "POST", "/api/spaces/" + s.String() + "/notes", fmt.Sprintf(`{"title":"Notulensi","templateId":"%s"}`, templateID), 201, note,
			func(c *Client) (any, error) {
				return c.CreateNote(ctx, s, api.NoteCreateRequest{Title: "Notulensi", TemplateID: &templateID})
			}},
		{"replace content with expected version", "PUT", base, `{"title":"Notulensi","content":` + noteDocumentJSON + `,"expectedVersion":9}`, 200, note,
			func(c *Client) (any, error) {
				return c.UpdateNote(ctx, n, api.NoteWriteRequest{Title: "Notulensi", Content: doc, ExpectedVersion: 9})
			}},
		{"save an empty document", "PUT", base, `{"title":"Kosong","content":[],"expectedVersion":9}`, 200, api.Note{ID: n, Title: "Kosong", Version: 10},
			func(c *Client) (any, error) {
				return c.UpdateNote(ctx, n, api.NoteWriteRequest{Title: "Kosong", Content: []api.NoteBlock{}, ExpectedVersion: 9})
			}},
		{"acquire or renew lock", "POST", base + "/lock", "", 200, lock,
			func(c *Client) (any, error) { return c.AcquireNoteLock(ctx, n) }},
		{"take expired lock without forcing", "POST", base + "/lock/takeover", `{}`, 200, lock,
			func(c *Client) (any, error) { return c.TakeoverNoteLock(ctx, n, api.NoteLockRequest{}) }},
		{"explicit forced takeover", "POST", base + "/lock/takeover", `{"force":true}`, 200, lock,
			func(c *Client) (any, error) { return c.TakeoverNoteLock(ctx, n, api.NoteLockRequest{Force: true}) }},
		{"save recoverable draft with original baseline", "PUT", base + "/draft", `{"title":"Draf","content":` + noteDocumentJSON + `,"baseVersion":8}`, 200, draft,
			func(c *Client) (any, error) {
				return c.SaveNoteDraft(ctx, n, api.NoteDraftRequest{Title: "Draf", Content: doc, BaseVersion: 8})
			}},
		{"reader comment without lock data", "POST", base + "/comments", `{"body":"Konsep sudah diperiksa"}`, 201, comment,
			func(c *Client) (any, error) {
				return c.AddNoteComment(ctx, n, api.NoteCommentRequest{Body: "Konsep sudah diperiksa"})
			}},
		{"attachment remains pending without URL", "POST", base + "/attachments", `{"name":"brief.pdf","url":"","contentType":"application/pdf","sizeBytes":120}`, 201, pending,
			func(c *Client) (any, error) {
				return c.AddNoteAttachment(ctx, n, api.NoteAttachmentRequest{Name: "brief.pdf", ContentType: "application/pdf", SizeBytes: 120})
			}},
		{"attachment ready URL", "POST", base + "/attachments", `{"name":"brief.pdf","url":"https://example.test/brief.pdf","contentType":"application/pdf","sizeBytes":120}`, 201, ready,
			func(c *Client) (any, error) {
				return c.AddNoteAttachment(ctx, n, api.NoteAttachmentRequest{Name: "brief.pdf", URL: ready.URL, ContentType: "application/pdf", SizeBytes: 120})
			}},
		{"reference confirmed task only", "POST", base + "/links", fmt.Sprintf(`{"targetType":"task","targetId":"%s"}`, taskID), 201, link,
			func(c *Client) (any, error) {
				return c.AddNoteLink(ctx, n, api.NoteLinkRequest{TargetType: api.NoteLinkTask, TargetID: taskID})
			}},
		{"create reusable template", "POST", templates, `{"name":"Rapat divisi","content":` + noteDocumentJSON + `}`, 201, template,
			func(c *Client) (any, error) {
				return c.CreateNoteTemplate(ctx, s, api.NoteTemplateRequest{Name: "Rapat divisi", Content: doc})
			}},
		{"update reusable template", "PUT", "/api/note-templates/" + templateID.String(), `{"name":"Rapat divisi","content":` + noteDocumentJSON + `}`, 200, template,
			func(c *Client) (any, error) {
				return c.UpdateNoteTemplate(ctx, templateID, api.NoteTemplateRequest{Name: "Rapat divisi", Content: doc})
			}},
		{"delete note", "DELETE", base, "", 204, nil,
			func(c *Client) (any, error) { return nil, c.DeleteNote(ctx, n) }},
		{"release own lock", "DELETE", base + "/lock", "", 204, nil,
			func(c *Client) (any, error) { return nil, c.ReleaseNoteLock(ctx, n) }},
		{"discard own draft", "DELETE", base + "/draft", "", 204, nil,
			func(c *Client) (any, error) { return nil, c.DeleteNoteDraft(ctx, n) }},
		{"remove attachment", "DELETE", base + "/attachments/" + attachmentID.String(), "", 204, nil,
			func(c *Client) (any, error) { return nil, c.DeleteNoteAttachment(ctx, n, attachmentID) }},
		{"remove reference", "DELETE", base + "/links/" + linkID.String(), "", 204, nil,
			func(c *Client) (any, error) { return nil, c.DeleteNoteLink(ctx, n, linkID) }},
		{"delete saved template", "DELETE", "/api/note-templates/" + templateID.String(), "", 204, nil,
			func(c *Client) (any, error) { return nil, c.DeleteNoteTemplate(ctx, templateID) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method || r.URL.RequestURI() != tt.path || r.Header.Get("Authorization") != "Bearer note-session" {
					t.Errorf("request = %s %s; authorized=%v", r.Method, r.URL.RequestURI(), r.Header.Get("Authorization") == "Bearer note-session")
				}
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if tt.body == "" {
					if len(raw) != 0 {
						t.Errorf("unexpected request body: %s", raw)
					}
				} else {
					var actual, expected any
					if err := json.Unmarshal(raw, &actual); err != nil {
						t.Error(err)
					}
					if err := json.Unmarshal([]byte(tt.body), &expected); err != nil {
						t.Error(err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					if !reflect.DeepEqual(actual, expected) {
						t.Errorf("body = %s; want %s", raw, tt.body)
					}
				}
				if tt.status == http.StatusNoContent {
					w.WriteHeader(tt.status)
					return
				}
				writeJSON(w, tt.status, tt.response)
			}))
			defer srv.Close()
			c := New(srv.URL)
			c.SetToken("note-session")
			got, err := tt.call(c)
			if err != nil || !reflect.DeepEqual(got, tt.response) {
				t.Fatalf("response = %+v, error = %v; want %+v", got, err, tt.response)
			}
		})
	}
}

func TestNoteClientReadsPreserveRichContentLockDraftAndTemplateScope(t *testing.T) {
	s, n, user, task, templateID := id.New(), id.New(), id.New(), id.New(), id.New()
	at := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	person := api.PersonRef{ID: user, DisplayName: "Bima"}
	lock := api.NoteLock{Editor: person, AcquiredAt: at, ExpiresAt: at.Add(5 * time.Minute)}
	summary := api.Note{ID: n, SpaceID: s, Title: "Rapat", TemplateKind: api.TemplateMeeting, Version: 9, Lock: &lock, Permissions: api.NotePermissions{Comment: true}}
	full := summary
	full.Content = noteClientDocument()
	full.Attachments = []api.NoteAttachment{{ID: id.New(), Name: "brief.pdf", Status: api.AttachmentPending, AddedBy: person, CreatedAt: at}}
	full.Comments = []api.NoteComment{{ID: id.New(), Author: person, Body: "Siap", CreatedAt: at}}
	draft := api.NoteDraft{Title: "Draf belum digabung", Content: noteClientDocument(), BaseVersion: 8, SavedAt: at, Stale: true}
	history := []api.NoteEvent{{ID: id.New(), Actor: person, Action: "note.update", CreatedAt: at, Before: map[string]any{"version": float64(8)}, After: map[string]any{"version": float64(9)}}}
	links := []api.NoteLink{{ID: id.New(), TargetType: api.NoteLinkTask, TargetID: task, CreatedBy: person, CreatedAt: at}}
	templates := []api.NoteTemplate{{ID: id.New(), Kind: api.TemplateFree, Name: "Catatan bebas", Builtin: true, Content: []api.NoteBlock{}}, {ID: templateID, SpaceID: &s, Kind: api.TemplateCustom, Name: "Rapat divisi", Content: noteClientDocument(), CreatedAt: &at, UpdatedAt: &at}}
	base := "/api/notes/" + n.String()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer reader-session" {
			t.Errorf("unexpected read method or bearer: %s", r.Method)
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil || len(raw) != 0 {
			t.Errorf("read had body %s, error=%v", raw, err)
		}
		switch r.URL.RequestURI() {
		case "/api/spaces/" + s.String() + "/notes":
			writeJSON(w, 200, api.List[api.Note]{Items: []api.Note{summary}})
		case base:
			writeJSON(w, 200, full)
		case base + "/draft":
			writeJSON(w, 200, draft)
		case base + "/history":
			writeJSON(w, 200, api.List[api.NoteEvent]{Items: history})
		case base + "/links":
			writeJSON(w, 200, api.List[api.NoteLink]{Items: links})
		case "/api/spaces/" + s.String() + "/note-templates":
			writeJSON(w, 200, api.List[api.NoteTemplate]{Items: templates})
		default:
			t.Errorf("unexpected read route: %s", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c, ctx := New(srv.URL), context.Background()
	c.SetToken("reader-session")
	if got, err := c.SpaceNotes(ctx, s); err != nil || !reflect.DeepEqual(got, []api.Note{summary}) {
		t.Fatalf("summary permissions/lock changed: %+v, %v", got, err)
	}
	if got, err := c.GetNote(ctx, n); err != nil || !reflect.DeepEqual(got, full) {
		t.Fatalf("rich document, pending attachment or read-only permission changed: %+v, %v", got, err)
	}
	if got, err := c.GetNoteDraft(ctx, n); err != nil || !reflect.DeepEqual(got, draft) {
		t.Fatalf("draft baseline/staleness changed: %+v, %v", got, err)
	}
	if got, err := c.NoteHistory(ctx, n); err != nil || !reflect.DeepEqual(got, history) {
		t.Fatalf("history changed: %+v, %v", got, err)
	}
	if got, err := c.NoteLinks(ctx, n); err != nil || !reflect.DeepEqual(got, links) {
		t.Fatalf("link identities changed: %+v, %v", got, err)
	}
	if got, err := c.NoteTemplates(ctx, s); err != nil || !reflect.DeepEqual(got, templates) {
		t.Fatalf("built-in/saved template scopes changed: %+v, %v", got, err)
	}
}

func TestNoteClientDoesNotConvertConflictsOrHiddenObjectsIntoSuccess(t *testing.T) {
	n := id.New()
	ctx := context.Background()
	tests := []struct {
		name, code, message string
		status              int
		call                func(*Client) error
	}{
		{"version conflict", api.CodeConflict, "Versi note sudah berubah.", 409,
			func(c *Client) error {
				_, err := c.UpdateNote(ctx, n, api.NoteWriteRequest{Title: "Lama", Content: []api.NoteBlock{}, ExpectedVersion: 3})
				return err
			}},
		{"another live editor", api.CodeConflict, "Note ini sedang diedit oleh Bima.", 409,
			func(c *Client) error { _, err := c.AcquireNoteLock(ctx, n); return err }},
		{"closed period", api.CodePeriodClosed, "Periode sudah ditutup.", 409,
			func(c *Client) error { return c.DeleteNote(ctx, n) }},
		{"hidden note", api.CodeNotFound, "Note tidak ditemukan.", 404,
			func(c *Client) error { _, err := c.GetNote(ctx, n); return err }},
		{"no saved draft", api.CodeNotFound, "Draf tidak ditemukan.", 404,
			func(c *Client) error { _, err := c.GetNoteDraft(ctx, n); return err }},
		{"readonly cannot save draft", api.CodeForbidden, "Anda tidak berwenang.", 403,
			func(c *Client) error {
				_, err := c.SaveNoteDraft(ctx, n, api.NoteDraftRequest{Title: "Draf", Content: []api.NoteBlock{}, BaseVersion: 3})
				return err
			}},
		{"expired session", api.CodeUnauthenticated, "Sesi berakhir.", 401,
			func(c *Client) error { _, err := c.SpaceNotes(ctx, id.New()); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				apiError(w, tt.status, tt.code, tt.message)
			}))
			defer srv.Close()
			err := tt.call(New(srv.URL))
			var apiErr *Error
			if !errors.As(err, &apiErr) || apiErr.Status != tt.status || apiErr.Code != tt.code || apiErr.Message != tt.message {
				t.Fatalf("rejection changed or appeared successful: %+v", err)
			}
		})
	}
}

func TestNoteRecoveryDraftClientUsesExplicitDraftIdentity(t *testing.T) {
	note, draftID := id.New(), id.New()
	at := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	drafts := []api.NoteDraft{{Title: "Draf sebelum terputus", Content: noteClientDocument(), BaseVersion: 8, SavedAt: at, Stale: true}}
	base := "/api/notes/" + note.String() + "/drafts"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer recovery-owner" {
			t.Error("recovery request did not retain its account bearer")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || len(body) != 0 {
			t.Errorf("recovery request carried an unexpected body: %s, %v", body, err)
		}
		switch r.Method + " " + r.URL.RequestURI() {
		case "GET " + base:
			writeJSON(w, 200, api.List[api.NoteDraft]{Items: drafts})
		case "DELETE " + base + "/" + draftID.String():
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected recovery route: %s %s", r.Method, r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c, ctx := New(srv.URL), context.Background()
	c.SetToken("recovery-owner")
	if got, err := c.NoteDrafts(ctx, note); err != nil || !reflect.DeepEqual(got, drafts) {
		t.Fatalf("recoverable drafts lost baseline/content/staleness: %+v, %v", got, err)
	}
	if err := c.DeleteNoteRecoveryDraft(ctx, note, draftID); err != nil {
		t.Fatal(err)
	}
}
