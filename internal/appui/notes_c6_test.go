package appui

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// shareNoteAPI is the C2 fake with public links, tags and note creation.
type shareNoteAPI struct {
	*c2NoteAPI
	mu      sync.Mutex
	made    int
	revoked int
	tags    [][]string
	created []api.NoteCreateRequest
}

func (f *shareNoteAPI) CreateNotePublicLink(context.Context, id.ID) (api.NotePublicLinkCreated, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.made++
	return api.NotePublicLinkCreated{URL: "https://space.contoh.test/n/AbCdEfGhIjKlMnOpQrStUvWxYz0123456789_-abcd", Link: api.NotePublicLink{CreatedAt: time.Now(), CreatedBy: api.PersonRef{DisplayName: "Sinta Notulis"}}}, nil
}

func (f *shareNoteAPI) RevokeNotePublicLink(context.Context, id.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.revoked++
	return nil
}

func (f *shareNoteAPI) SetNoteTags(_ context.Context, _ id.ID, tags []string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tags = append(f.tags, tags)
	return tags, nil
}

func (f *shareNoteAPI) CreateNote(_ context.Context, sid id.ID, req api.NoteCreateRequest) (api.Note, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created = append(f.created, req)
	n := api.Note{ID: id.New(), SpaceID: sid, Title: "Notulen Rapat rutin #14", MeetingID: req.MeetingID, TemplateKind: api.TemplateMeeting, Version: 1}
	return n, nil
}

func TestNoteShareAndTags(t *testing.T) {
	h, f := c2Harness(t, true)
	h.app.notes.note.Permissions.Share = true
	s := &shareNoteAPI{c2NoteAPI: f}
	h.app.api = s
	openNotes(h)
	h.click("Bagikan catatan")
	h.want("Hanya divisi ini")
	h.want("Publik lewat tautan")
	h.click("Publik lewat tautan")
	h.settle()
	if s.made != 1 || h.app.notes.note.PublicLink == nil {
		t.Fatalf("not shared: made=%d", s.made)
	}
	h.want("https://space.contoh.test/n/AbCdEfGhIjKlMnOpQrStUvWxYz0123456789_-abcd")
	h.want("Publik")
	h.click("Salin tautan publik")
	h.want("Tersalin ✓")
	h.click("Hanya divisi ini")
	h.settle()
	if s.revoked != 1 || h.app.notes.note.PublicLink != nil {
		t.Fatalf("not revoked: %d", s.revoked)
	}
	// A reader who may not share sees no button on a private note.
	h.app.notes.note.Permissions.Share = false
	h.app.notes.share.open = false
	h.tt.Frame()
	h.dontWant("Bagikan catatan")

	h.app.setNoteTags([]string{"Rapat Rutin", "#evaluasi"})
	h.settle()
	if len(s.tags) != 1 || len(h.app.notes.note.Tags) != 2 || h.app.notes.note.Tags[0] != "rapat-rutin" {
		t.Fatalf("tags %v note %v", s.tags, h.app.notes.note.Tags)
	}
	h.app.setNoteTags([]string{"a/b"})
	if h.app.notes.share.tagErr == "" {
		t.Fatal("bad tag accepted")
	}
}

func TestNoteCreateFromScheduledMeeting(t *testing.T) {
	h, f := c2Harness(t, true)
	s := &shareNoteAPI{c2NoteAPI: f}
	h.app.api = s
	sid := h.app.notes.space
	next := api.Meeting{ID: id.New(), SpaceID: sid, Title: "Rapat rutin #14", StartsAt: time.Now().Add(48 * time.Hour), TimeZone: "Asia/Jakarta"}
	done := api.Meeting{ID: id.New(), SpaceID: sid, Title: "Rapat lama", StartsAt: time.Now().Add(24 * time.Hour), TimeZone: "Asia/Jakarta", NoteID: &h.app.notes.note.ID}
	h.app.meetings = meetingsState{space: sid, loaded: true, items: []api.Meeting{done, next}}
	// The list (with "Buat catatan") shows while no note is open.
	h.app.notes.selected, h.app.notes.note = id.Nil, nil
	openNotes(h)
	h.app.notes.templates = []api.NoteTemplate{{Kind: api.TemplateMeeting, Name: "Notulen rapat divisi", Builtin: true}, {Kind: api.TemplateEvaluation, Name: "Evaluasi", Builtin: true}}
	h.app.beginNote()
	h.tt.Advance(300 * time.Millisecond)
	h.want("Isi dari rapat terjadwal")
	h.want("Rapat rutin #14 · " + noteMeetingWhen(&next, true))
	h.app.createNote()
	h.settle()
	if len(s.created) != 1 || s.created[0].MeetingID == nil || *s.created[0].MeetingID != next.ID || s.created[0].Title != "" {
		t.Fatalf("create %+v", s.created)
	}
	if h.app.meetings.items[1].NoteID == nil {
		t.Fatal("meeting not marked as having a note")
	}
	// Evaluasi has no meeting choice; a title is needed again.
	h.app.beginNote()
	h.app.notes.templateKind = api.TemplateEvaluation
	h.tt.Frame()
	h.dontWant("Isi dari rapat terjadwal")
	h.app.createNote()
	if len(s.created) != 1 || h.app.notes.err == "" {
		t.Fatalf("created without a title: %+v", s.created)
	}
}

// openNotes shows the Catatan tab on a desktop window.
func openNotes(h *harness) {
	h.tt.SetSize(1440, 900)
	h.app.router.Push(PathSpaces)
	h.app.chrome.tab = "Catatan"
	h.tt.Frame()
}

// TestNotesC6C5NativeCapture writes the review's two screenshots when
// SPACE_UI_CAPTURE_DIR is set: the share popover of a public note, and the
// new note filled from a scheduled meeting.
func TestNotesC6C5NativeCapture(t *testing.T) {
	if os.Getenv("SPACE_UI_CAPTURE_DIR") == "" {
		t.Skip("SPACE_UI_CAPTURE_DIR is not set")
	}
	h, f := c2Harness(t, true)
	h.app.notes.note.Permissions.Share = true
	h.app.notes.note.Tags = []string{"rapat-rutin", "dies-natalis"}
	s := &shareNoteAPI{c2NoteAPI: f}
	h.app.api = s
	openNotes(h)
	h.click("Bagikan catatan")
	h.click("Publik lewat tautan")
	h.settle()
	h.tt.Advance(time.Second)
	captureNativeUI(t, h, "catatan-c6-bagikan")

	h.app.notes.share.open = false
	sid := h.app.notes.space
	next := api.Meeting{ID: id.New(), SpaceID: sid, Title: "Rapat rutin #14", StartsAt: time.Now().Add(48 * time.Hour), TimeZone: "Asia/Jakarta"}
	h.app.meetings = meetingsState{space: sid, loaded: true, items: []api.Meeting{next}}
	h.app.notes.templates = []api.NoteTemplate{{Kind: api.TemplateMeeting, Name: "Notulen rapat divisi", Builtin: true}, {Kind: api.TemplateEvaluation, Name: "Evaluasi", Builtin: true},
		{Kind: api.TemplateBrief, Name: "Rapat singkat", Builtin: true}, {Kind: api.TemplateFree, Name: "Kosong", Builtin: true}}
	h.app.notes.selected, h.app.notes.note = id.Nil, nil
	h.tt.Frame()
	h.app.beginNote()
	h.tt.Advance(time.Second)
	captureNativeUI(t, h, "catatan-c5-template")
}
