package appui

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

type realtimeTestWatch struct {
	ctx           context.Context
	period, space id.ID
	event         func(string)
	end           chan error
}

type realtimeNoteAPI struct {
	*fakeAPI
	NotesAPI
	note api.Note
}

func (f *realtimeNoteAPI) GetNote(context.Context, id.ID) (api.Note, error)         { return f.note, nil }
func (f *realtimeNoteAPI) NoteLinks(context.Context, id.ID) ([]api.NoteLink, error) { return nil, nil }
func (f *realtimeNoteAPI) NoteDrafts(context.Context, id.ID) ([]api.NoteDraft, error) {
	return nil, nil
}

func TestRealtimeNoteDetailRefreshKeepsDirtyBaselineAndUpdatesComments(t *testing.T) {
	h := newChromeHarness(t, nil)
	space := testAuthorizedSpace(h.app, h.api.divisions[0])
	note := api.Note{ID: id.New(), SpaceID: space.ID, Title: "Judul server baru", Version: 2,
		Content:  []api.NoteBlock{{Type: api.BlockParagraph, Text: "Konten server baru"}},
		Comments: []api.NoteComment{{ID: id.New(), Body: "Komentar baru"}}}
	h.app.api = &realtimeNoteAPI{fakeAPI: h.api, note: note}
	editor := newNoteDocumentEditor([]api.NoteBlock{{Type: api.BlockParagraph, Text: "Draf lokal"}})
	h.app.notes = notesUIState{space: space.ID, selected: note.ID, note: &note, title: "Judul lokal",
		editor: editor, dirty: true, baseVersion: 1, baselineTitle: "Judul awal", baselineKnown: true}
	h.app.loadNoteDetail(true)
	for h.app.inflight > 0 {
		select {
		case fn := <-h.queue:
			fn()
		case <-time.After(time.Second):
			t.Fatal("note refresh never returned")
		}
	}
	n := h.app.notes
	if n.editor != editor || n.title != "Judul lokal" || n.baseVersion != 1 || n.baselineTitle != "Judul awal" || !n.dirty || !n.conflict {
		t.Fatal("refresh replaced dirty note or lost conflict baseline")
	}
	if n.note.Version != 2 || len(n.note.Comments) != 1 || n.note.Comments[0].Body != "Komentar baru" {
		t.Fatal("preserving draft hid remote comments")
	}
}

type realtimeTestAPI struct {
	*taskTestAPI
	watches chan realtimeTestWatch
}

func (f *realtimeTestAPI) WatchChanges(ctx context.Context, period, space id.ID, event func(string)) error {
	w := realtimeTestWatch{ctx: ctx, period: period, space: space, event: event, end: make(chan error, 1)}
	f.watches <- w
	select {
	case err := <-w.end:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestRealtimeReplacementReconnectAndDraftPreservation(t *testing.T) {
	h, tasks := taskHarness(t)
	f := &realtimeTestAPI{taskTestAPI: tasks, watches: make(chan realtimeTestWatch, 8)}
	h.app.api = f
	t.Cleanup(h.app.stopRealtime)
	tick := ui.NewTester(h.app.tickRealtime, 1000, 700)
	nextWatch := func() realtimeTestWatch {
		t.Helper()
		select {
		case w := <-f.watches:
			return w
		case <-time.After(time.Second):
			t.Fatal("stream not started")
			return realtimeTestWatch{}
		}
	}
	apply := func() {
		t.Helper()
		select {
		case fn := <-h.queue:
			fn()
		case <-time.After(time.Second):
			t.Fatal("stream callback not queued")
		}
	}
	w := nextWatch()
	if w.period != h.app.periodID || w.space != id.Nil {
		t.Fatal("wrong initial scope")
	}
	// Settle initial polling without the full shell spawning more requests.
	for h.app.inflight > 0 {
		apply()
	}
	h.app.tasks.draft = &taskDraft{title: "Draf belum disimpan"}
	h.app.tasks.comment = "Balasan belum dikirim"
	w.event("ready")
	apply()
	if !h.app.realtime.connected {
		t.Fatal("ready did not mark stream connected")
	}
	tasks.items[0].Title = "Diubah pengguna lain"
	tick.Frame() // Window.Update also schedules a new frame after its callback.
	tick.Advance(time.Second)
	for h.app.inflight > 0 {
		apply()
	}
	if h.app.tasks.items[0].Title != "Diubah pengguna lain" || h.app.tasks.draft.title != "Draf belum disimpan" || h.app.tasks.comment != "Balasan belum dikirim" {
		t.Fatal("refresh lost remote change or local draft")
	}
	w.end <- errors.New("proxy disconnected")
	apply()
	if h.app.realtime.connected {
		t.Fatal("disconnected stream still claims realtime")
	}
	h.app.realtime.retryAt = time.Time{}
	tick.Frame()
	w2 := nextWatch()
	w2.event("ready")
	apply()
	h.app.periodID = id.New()
	tick.Frame()
	w3 := nextWatch()
	if w2.ctx.Err() == nil || w3.period != h.app.periodID {
		t.Fatal("old stream not cancelled on context change")
	}
	// A callback already in flight must not mark the new scope healthy.
	w2.event("change")
	apply()
	if h.app.realtime.connected || h.app.realtime.dirty {
		t.Fatal("old scope callback affected new stream")
	}
	h.app.stopRealtime()
	w3.event("ready")
	apply()
	if h.app.realtime.connected {
		t.Fatal("stopped session callback was accepted")
	}
}
