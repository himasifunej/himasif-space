package appui

import (
	"context"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

// c2NoteAPI is a notes service with one note: UpdateNote checks the version
// like the server, and hold (when set) keeps a save in flight.
type c2NoteAPI struct {
	*fakeAPI
	NotesAPI
	mu      sync.Mutex
	note    api.Note
	updates []api.NoteWriteRequest
	drafts  int
	hold    chan struct{}
	history []api.NoteEvent
}

func (f *c2NoteAPI) GetNote(context.Context, id.ID) (api.Note, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return cloneNote(f.note), nil
}
func (f *c2NoteAPI) UpdateNote(_ context.Context, _ id.ID, req api.NoteWriteRequest) (api.Note, error) {
	if f.hold != nil {
		<-f.hold
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates = append(f.updates, req)
	if req.ExpectedVersion != f.note.Version {
		return api.Note{}, &client.Error{Status: http.StatusConflict, Code: api.CodeConflict, Message: "Note telah berubah. Muat ulang sebelum menyimpan."}
	}
	f.note.Title, f.note.Content = req.Title, cloneNoteDocument(req.Content)
	f.note.Version++
	f.note.UpdatedAt = time.Now()
	return cloneNote(f.note), nil
}
func (f *c2NoteAPI) SaveNoteDraft(context.Context, id.ID, api.NoteDraftRequest) (api.NoteDraft, error) {
	f.mu.Lock()
	f.drafts++
	f.mu.Unlock()
	return api.NoteDraft{ID: id.New(), CurrentSession: true}, nil
}
func (f *c2NoteAPI) NoteLinks(context.Context, id.ID) ([]api.NoteLink, error)   { return nil, nil }
func (f *c2NoteAPI) NoteDrafts(context.Context, id.ID) ([]api.NoteDraft, error) { return nil, nil }
func (f *c2NoteAPI) NoteHistory(context.Context, id.ID) ([]api.NoteEvent, error) {
	return f.history, nil
}
func (f *c2NoteAPI) AcquireNoteLock(context.Context, id.ID) (api.NoteLock, error) {
	return *f.note.Lock, nil
}
func (f *c2NoteAPI) ReleaseNoteLock(context.Context, id.ID) error { return nil }

func (f *c2NoteAPI) updateCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.updates)
}

type memNoteDrafts struct {
	mu     sync.Mutex
	drafts map[id.ID]LocalNoteDraft
}

func (m *memNoteDrafts) Load(_, note id.ID) (*LocalNoteDraft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d, ok := m.drafts[note]; ok {
		return &d, nil
	}
	return nil, nil
}
func (m *memNoteDrafts) Save(_, note id.ID, d LocalNoteDraft) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.drafts == nil {
		m.drafts = map[id.ID]LocalNoteDraft{}
	}
	m.drafts[note] = d
	return nil
}
func (m *memNoteDrafts) Delete(_, note id.ID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.drafts, note)
	return nil
}

// c2Meeting is the dummy meeting of the canvas: Monday 9 Nov, 19.30–20.45
// WIB, six present and one excused.
func c2Meeting(space id.ID) api.Meeting {
	jkt := noteZone("Asia/Jakarta")
	start := time.Date(2026, 11, 9, 19, 30, 0, 0, jkt)
	end := start.Add(75 * time.Minute)
	people := []struct{ name, status, note string }{
		{"Demo Mediatek", "present", ""}, {"Aruna Desain", "present", ""}, {"Nara Tekno", "present", ""},
		{"Raka Video", "present", ""}, {"Bima Konten", "present", ""}, {"Sinta Notulis", "present", ""},
		{"Laras Foto", "excused", "kuliah pengganti"},
	}
	m := api.Meeting{ID: id.New(), SpaceID: space, Title: "Rapat koordinasi Mediatek · Minggu 10", Location: "Sekretariat HIMASIF", JoinURL: "https://meet.contoh.test/abc",
		StartsAt: start, EndsAt: &end, TimeZone: "Asia/Jakarta"}
	for _, p := range people {
		m.Participants = append(m.Participants, api.MeetingParticipant{UserID: id.New(), DisplayName: p.name, AttendanceStatus: p.status, AttendanceNote: p.note})
	}
	return m
}

func c2Content() []api.NoteBlock {
	text := func(s string) []api.Span { return []api.Span{{Text: s}} }
	return []api.NoteBlock{
		{Type: api.BlockHeading, Level: 2, Text: "Agenda"},
		{Type: api.BlockNumberedList, Items: []api.NoteListItem{{Spans: text("Evaluasi poster oprec"), Minutes: 15}, {Spans: text("Konten Dies Natalis"), Minutes: 20}, {Spans: text("Piket Bot WhatsApp"), Minutes: 10}}},
		{Type: api.BlockCallout, Variant: api.CalloutDecision, Spans: text("Poster direvisi ke versi satu foto; terbit Kamis 12 Nov.")},
		{Type: api.BlockHeading, Level: 2, Text: "Action item"},
		{Type: api.BlockChecklist, Items: []api.NoteListItem{{Spans: text("Revisi poster oprec — Aruna")}, {Spans: text("Storyboard teaser — Raka")}}},
		{Type: api.BlockParagraph, Spans: text("Rapat berikutnya Senin 16 Nov di sekretariat.")},
	}
}

// c2Harness opens a meeting note in the Catatan tab. editing: this session
// holds the editor (the notulis); otherwise it reads while Sinta writes.
func c2Harness(t *testing.T, editing bool) (*harness, *c2NoteAPI) {
	t.Helper()
	h := newChromeHarness(t, nil)
	d := h.api.divisions[0]
	sp := testAuthorizedSpace(h.app, d)
	caps := []string{api.CapSpaceRead, api.CapMembersRead, api.CapNotesRead, api.CapNotesManage}
	h.app.access.spaces[0].Capabilities = caps
	h.app.access.spaces[0].Roles[0].Capabilities = caps
	meeting := c2Meeting(sp.ID)
	editor := api.PersonRef{ID: id.New(), DisplayName: "Sinta Notulis"}
	if editing {
		editor = api.PersonRef{ID: h.app.user.ID, DisplayName: "Sinta Notulis"}
	}
	now := time.Now()
	note := api.Note{ID: id.New(), SpaceID: sp.ID, OwnerDivisionID: d.ID, Title: meeting.Title, Content: c2Content(), TemplateKind: api.TemplateMeeting,
		MeetingID: &meeting.ID, Version: 3, CreatedBy: editor, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-20 * time.Second),
		Lock:        &api.NoteLock{Editor: editor, HeldByCaller: editing, ExpiresAt: now.Add(5 * time.Minute)},
		Permissions: api.NotePermissions{Manage: editing, Comment: true}}
	f := &c2NoteAPI{fakeAPI: h.api, note: note}
	h.app.api = f
	h.app.NoteDrafts = &memNoteDrafts{}
	view := cloneNote(note)
	h.app.notes = notesUIState{space: sp.ID, loaded: true, items: []api.Note{note}, selected: note.ID, note: &view, title: note.Title,
		editor: newNoteDocumentEditor(note.Content), baseVersion: note.Version, baselineTitle: note.Title, baselineContent: cloneNoteDocument(note.Content),
		baselineKnown: true, editing: editing, renewedAt: now, lastTick: now}
	if editing {
		h.app.notes.leaseService = f
	}
	h.app.meetings = meetingsState{space: sp.ID, loaded: true, items: []api.Meeting{meeting}}
	return h, f
}

// drainNotes runs finished async calls without drawing a frame, so the
// test alone decides what time tickNotes sees.
func drainNotes(t *testing.T, h *harness) {
	t.Helper()
	for h.app.inflight > 0 {
		select {
		case fn := <-h.queue:
			fn()
		case <-time.After(3 * time.Second):
			t.Fatalf("%d calls never returned", h.app.inflight)
		}
	}
}

// Owner decision 9 Nov: the note saves itself a few seconds after the
// notulis stops typing, without replacing the editor (focus and page stay).
func TestNoteAutosaveAfterQuietPauseKeepsEditor(t *testing.T) {
	h, f := c2Harness(t, true)
	a := h.app
	base := a.notes.lastTick
	editor := a.notes.editor
	a.notes.title = "Rapat koordinasi Mediatek · Minggu 10 (revisi)"
	a.markNoteChanged()
	a.tickNotes(base.Add(2 * time.Second))
	drainNotes(t, h)
	if f.updateCount() != 0 || a.notes.saving {
		t.Fatal("saved while the notulis was still within the quiet pause")
	}
	if label, _, _ := a.noteSaveState(base.Add(2 * time.Second)); label != "Menyimpan otomatis…" {
		t.Fatalf("status while waiting = %q", label)
	}
	a.tickNotes(base.Add(noteAutosaveQuiet))
	if !a.notes.saving || !a.notesCanEdit() {
		t.Fatal("autosave did not start, or made the editor read-only while saving")
	}
	if label, _, _ := a.noteSaveState(base.Add(noteAutosaveQuiet)); label != "Menyimpan…" {
		t.Fatalf("status while saving = %q", label)
	}
	drainNotes(t, h)
	if f.updateCount() != 1 || f.updates[0].ExpectedVersion != 3 || !strings.HasSuffix(f.updates[0].Title, "(revisi)") {
		t.Fatalf("updates = %+v", f.updates)
	}
	if a.notes.editor != editor || a.notes.dirty || a.notes.baseVersion != 4 || a.notes.note.Version != 4 {
		t.Fatalf("after autosave: same editor %v, dirty %v, base %d", a.notes.editor == editor, a.notes.dirty, a.notes.baseVersion)
	}
	if label, _, _ := a.noteSaveState(a.notes.savedAt.Add(10 * time.Second)); label != "Tersimpan · baru saja" {
		t.Fatalf("status after save = %q", label)
	}
	// No second write while nothing changed, and no private recovery draft
	// while autosave works.
	a.tickNotes(base.Add(20 * time.Second))
	drainNotes(t, h)
	if f.updateCount() != 1 || f.drafts != 0 {
		t.Fatalf("updates %d, recovery drafts %d", f.updateCount(), f.drafts)
	}
}

// Typing without a pause still saves at least every 30 seconds.
func TestNoteAutosaveAtMostThirtySecondsWhileTyping(t *testing.T) {
	h, f := c2Harness(t, true)
	a := h.app
	base := a.notes.lastTick
	for i := 0; i <= 15; i++ {
		now := base.Add(time.Duration(i) * 2 * time.Second)
		a.notes.lastTick = now
		a.notes.title = a.notes.title + "."
		a.markNoteChanged()
		a.tickNotes(now)
		drainNotes(t, h)
		if want := 0; i < 15 && f.updateCount() != want {
			t.Fatalf("saved after %ds of steady typing", i*2)
		}
	}
	if f.updateCount() != 1 {
		t.Fatalf("no save after 30 s of typing: %d", f.updateCount())
	}
}

// Keystrokes made while a save is in flight stay in the editor and stay
// unsaved; the next save starts from the version the first one returned.
func TestNoteTypingDuringAutosaveIsKeptAndSavedNext(t *testing.T) {
	h, f := c2Harness(t, true)
	a := h.app
	f.hold = make(chan struct{})
	base := a.notes.lastTick
	a.notes.title = "Judul pertama"
	a.markNoteChanged()
	a.tickNotes(base.Add(noteAutosaveQuiet))
	if !a.notes.saving {
		t.Fatal("autosave did not start")
	}
	later := base.Add(noteAutosaveQuiet + time.Second)
	a.notes.lastTick = later
	a.notes.title = "Judul kedua"
	a.markNoteChanged()
	close(f.hold)
	drainNotes(t, h)
	if !a.notes.dirty || a.notes.baseVersion != 4 || a.notes.title != "Judul kedua" || a.notes.compare != nil {
		t.Fatalf("dirty %v base %d title %q compare %v", a.notes.dirty, a.notes.baseVersion, a.notes.title, a.notes.compare != nil)
	}
	f.hold = nil
	a.tickNotes(later.Add(noteAutosaveQuiet))
	drainNotes(t, h)
	if f.updateCount() != 2 || f.updates[1].ExpectedVersion != 4 || f.updates[1].Title != "Judul kedua" || a.notes.dirty {
		t.Fatalf("second save: %+v dirty %v", f.updates, a.notes.dirty)
	}
}

// A newer version on the server (a takeover) is a conflict: the typed copy
// goes to the side-by-side review instead of being overwritten.
func TestNoteAutosaveConflictOpensReview(t *testing.T) {
	h, f := c2Harness(t, true)
	a := h.app
	f.note.Version = 5 // Demo took over and saved
	f.note.Lock = &api.NoteLock{Editor: api.PersonRef{ID: id.New(), DisplayName: "Demo Mediatek"}, ExpiresAt: time.Now().Add(5 * time.Minute)}
	base := a.notes.lastTick
	a.notes.title = "Ketikan saya"
	a.markNoteChanged()
	a.tickNotes(base.Add(noteAutosaveQuiet))
	drainNotes(t, h)
	if a.notes.compare == nil || !a.notes.conflict || a.notes.editing || a.notes.compare.Title != "Ketikan saya" {
		t.Fatalf("conflict not opened for review: compare %v conflict %v editing %v", a.notes.compare != nil, a.notes.conflict, a.notes.editing)
	}
}

// A failed save (network) keeps the copy, says so, and retries later.
func TestNoteAutosaveFailureRetriesLater(t *testing.T) {
	h, f := c2Harness(t, true)
	a := h.app
	f.note.Lock = &api.NoteLock{Editor: api.PersonRef{ID: a.user.ID}, HeldByCaller: true, ExpiresAt: time.Now().Add(time.Hour)}
	base := a.notes.lastTick
	a.notes.title = "Belum terkirim"
	a.markNoteChanged()
	a.notes.saveErr, a.notes.saveFailedAt = "Server tidak dapat dihubungi.", base
	if a.noteAutosaveDue(base.Add(noteAutosaveQuiet)) {
		t.Fatal("retried before the retry delay")
	}
	if label, _, warn := a.noteSaveState(base); label != "Belum tersimpan — coba lagi" || !warn {
		t.Fatalf("failure status = %q", label)
	}
	a.tickNotes(base.Add(noteAutosaveRetry))
	drainNotes(t, h)
	if f.updateCount() != 1 || a.notes.saveErr != "" || a.notes.dirty {
		t.Fatalf("retry: updates %d err %q dirty %v", f.updateCount(), a.notes.saveErr, a.notes.dirty)
	}
}

// A reader sees the notulis' autosave arrive in place: the same editor
// (no jump), the same block page, and who updated it.
func TestNoteReaderSeesAutosaveWithoutJump(t *testing.T) {
	h, f := c2Harness(t, false)
	a := h.app
	blocks := []api.NoteBlock{}
	for i := 0; i < 20; i++ {
		blocks = append(blocks, api.NoteBlock{Type: api.BlockParagraph, Spans: []api.Span{{Text: "Baris lama"}}})
	}
	a.notes.editor = newNoteDocumentEditor(blocks)
	a.notes.editor.blockPage = 2
	reader := a.notes.editor
	f.note.Version = 4
	f.note.Content = cloneNoteDocument(blocks)
	f.note.Content[17].Spans = []api.Span{{Text: "Baris baru dari notulis"}}
	a.loadNoteDetail(true)
	drainNotes(t, h)
	if a.notes.editor != reader || reader.blockPage != 2 || noteBlockText(reader.blocks[17]) != "Baris baru dari notulis" {
		t.Fatalf("reader refresh: same editor %v, page %d", a.notes.editor == reader, reader.blockPage)
	}
	label, _, _ := a.noteSaveState(a.notes.remoteAt.Add(2 * time.Second))
	if !strings.HasPrefix(label, "Diperbarui ") || !strings.HasSuffix(label, " oleh Sinta") {
		t.Fatalf("reader status = %q", label)
	}
	if label, _, _ := a.noteSaveState(a.notes.remoteAt.Add(time.Minute)); !strings.HasPrefix(label, "Diedit ") {
		t.Fatalf("status a minute later = %q", label)
	}
}

// Runs of autosaves by one person show as one history line; the server
// keeps one event per save.
func TestGroupNoteHistoryMergesAutosaves(t *testing.T) {
	sinta, demo := api.PersonRef{ID: id.New(), DisplayName: "Sinta"}, api.PersonRef{ID: id.New(), DisplayName: "Demo"}
	at := time.Date(2026, 11, 9, 13, 0, 0, 0, time.UTC)
	ev := func(who api.PersonRef, action string, minutes int) api.NoteEvent {
		return api.NoteEvent{ID: id.New(), Actor: who, Action: action, CreatedAt: at.Add(time.Duration(minutes) * time.Minute)}
	}
	// Newest first, as the server sends them.
	events := []api.NoteEvent{ev(sinta, "note.update", 30), ev(sinta, "note.update", 28), ev(sinta, "note.update", 25),
		ev(demo, "note.comment.add", 20), ev(sinta, "note.update", 12), ev(sinta, "note.update", 2), ev(demo, "note.update", 1)}
	got := groupNoteHistory(events)
	counts := []int{}
	for _, e := range got {
		counts = append(counts, e.count)
	}
	want := []int{3, 1, 1, 1, 1}
	if len(counts) != len(want) {
		t.Fatalf("groups = %v; want %v", counts, want)
	}
	for i := range want {
		if counts[i] != want[i] {
			t.Fatalf("groups = %v; want %v", counts, want)
		}
	}
	if !got[0].first.Equal(at.Add(25 * time.Minute)) {
		t.Fatalf("first of the run = %v", got[0].first)
	}
}

func TestNoteRelativeTimes(t *testing.T) {
	now := time.Date(2026, 11, 9, 13, 0, 0, 0, time.UTC) // 20.00 WIB
	for _, tc := range []struct {
		ago  time.Duration
		want string
	}{{20 * time.Second, "baru saja"}, {12 * time.Minute, "12 menit lalu"}, {3 * time.Hour, "pukul 17.00"}, {22 * time.Hour, "kemarin 22.00"}, {72 * time.Hour, "6 Nov 20.00"}} {
		if got := noteRelative(now.Add(-tc.ago), now, "Asia/Jakarta"); got != tc.want {
			t.Errorf("%v ago = %q; want %q", tc.ago, got, tc.want)
		}
	}
}

// The document: bar with breadcrumb, kind, save state and privacy; the
// meeting's date, place and attendance; on a phone one folded line.
func TestNoteDocumentLayoutDesktopAndPhone(t *testing.T) {
	h, _ := c2Harness(t, false)
	h.tt.SetSize(1440, 900)
	h.app.router.Push(PathSpaces)
	h.app.chrome.tab = "Catatan"
	h.tt.Frame()
	for _, s := range []string{"Rapat", "Notulen", "Hanya divisi ini", "Tanggal", "Senin, 9 Nov 2026 · 19.30–20.45", "Tempat", "Sekretariat HIMASIF + rapat daring",
		"Kehadiran", "Laras", "izin · kuliah pengganti", "6 hadir · 1 izin", "Sinta Notulis sedang mencatat", "Diedit baru saja"} {
		h.want(s)
	}
	for _, s := range []string{"Isi resmi versi", "Simpan draf pemulihan ke server", "Simpan sekarang"} {
		h.dontWant(s)
	}
	h.tt.SetSize(390, 844)
	h.tt.Frame()
	h.want("Sen 9 Nov · 19.30 · 6 hadir · 1 izin")
	h.dontWant("Tanggal")
	h.dontWant("Hanya divisi ini") // the phone bar keeps only the lock icon
	h.click("Properti catatan")
	h.tt.Frame()
	h.want("Tanggal")
	h.want("izin · kuliah pengganti")
	h.click("Kembali ke daftar catatan")
	h.tt.Frame()
	h.dontWant("Tanggal")
}

// The notulis gets the frameless title, the autosave hint and Simpan
// sekarang; a note without a meeting shows when it was made.
func TestNoteDocumentEditorBarAndPlainNote(t *testing.T) {
	h, _ := c2Harness(t, true)
	h.tt.SetSize(1440, 900)
	h.app.router.Push(PathSpaces)
	h.app.chrome.tab = "Catatan"
	h.tt.Frame()
	h.want("Tersimpan otomatis beberapa detik setelah kamu berhenti mengetik.")
	h.want("Simpan sekarang")
	h.app.notes.note.MeetingID = nil
	h.app.notes.note.TemplateKind = api.TemplateFree
	h.tt.Frame()
	h.want("Dibuat")
	h.dontWant("Kehadiran")
}

// SPACE_UI_CAPTURE_DIR opts into PNGs of the C2 document: the notulis on a
// desktop, a reader on a desktop, and the phone with properties folded and
// opened. Light theme, dummy data.
func TestNotesC2NativeCapture(t *testing.T) {
	if os.Getenv("SPACE_UI_CAPTURE_DIR") == "" {
		t.Skip("set SPACE_UI_CAPTURE_DIR for native screenshots")
	}
	shoot := func(h *harness, name string) {
		h.tt.Frame()
		h.tt.Advance(400 * time.Millisecond)
		h.tt.Frame()
		captureNativeUI(t, h, name)
	}
	for _, editing := range []bool{true, false} {
		h, _ := c2Harness(t, editing)
		h.app.chrome.preferences.Theme = "light"
		h.app.chrome.preferences.Accent = "rose"
		h.app.router.Push(PathSpaces)
		h.app.chrome.tab = "Catatan"
		h.tt.SetSize(1440, 900)
		if editing {
			h.app.notes.savedAt = time.Now()
			shoot(h, "catatan-c2-desktop-notulis")
			h.tt.SetSize(390, 844)
			shoot(h, "catatan-c2-hp")
			h.app.notes.propsOpen = true
			shoot(h, "catatan-c2-hp-properti")
			h.app.notes.propsOpen = false
			long := "Rapat koordinasi Mediatek · Minggu 10 · evaluasi poster oprec, konten Dies Natalis dan piket Bot WhatsApp"
			h.app.notes.title, h.app.notes.note.Title = long, long
			shoot(h, "catatan-c2-hp-judul-panjang")
			h.tt.SetSize(1440, 900)
			shoot(h, "catatan-c2-desktop-judul-panjang")
			continue
		}
		shoot(h, "catatan-c2-desktop-pembaca")
	}
}

// A long title wraps to more lines instead of being cut, for the notulis
// (a text area as high as its text) and for readers, on a phone and on a
// desktop; Enter adds no line break and a pasted one is saved as a space.
func TestNoteTitleWrapsInsteadOfCut(t *testing.T) {
	long := "Rapat koordinasi Mediatek · Minggu 10 · evaluasi poster oprec, konten Dies Natalis dan piket Bot WhatsApp"
	for _, editing := range []bool{true, false} {
		h, _ := c2Harness(t, editing)
		h.app.notes.title = long
		h.app.notes.note.Title = long
		h.app.router.Push(PathSpaces)
		h.app.chrome.tab = "Catatan"
		for _, size := range []struct{ w, h, lines int }{{390, 844, 3}, {1440, 900, 2}} {
			h.tt.SetSize(size.w, size.h)
			h.tt.Frame()
			name := "Judul catatan"
			r, ok := h.tt.Find(name)
			if !ok {
				t.Fatalf("editing %v, %dpx: title not found", editing, size.w)
			}
			font := float32(30)
			if size.w < mobileMaxWidth {
				font = 24
			}
			if r.H < font*float32(size.lines) || r.X+r.W > float32(size.w) {
				t.Fatalf("editing %v, %dpx: title %+v is not wrapped to %d lines within the window", editing, size.w, r, size.lines)
			}
		}
		if editing {
			h.click("Judul catatan")
			h.tt.Key(0, ui.KeyEnter)
			h.tt.Frame()
			if h.app.notes.title != long {
				t.Fatalf("Enter changed the title: %q", h.app.notes.title)
			}
		}
	}
	if got := noteTitleText(" Rapat\nkoordinasi\r\nMinggu 10 "); got != "Rapat koordinasi Minggu 10" {
		t.Fatalf("noteTitleText = %q", got)
	}
}
