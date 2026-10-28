package appui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// NotesAPI is optional: an older service never becomes a successful empty
// notebook or accepts a local-only save as if it were a cloud write.
type NotesAPI interface {
	SpaceNotes(context.Context, id.ID) ([]api.Note, error)
	GetNote(context.Context, id.ID) (api.Note, error)
	CreateNote(context.Context, id.ID, api.NoteCreateRequest) (api.Note, error)
	UpdateNote(context.Context, id.ID, api.NoteWriteRequest) (api.Note, error)
	DeleteNote(context.Context, id.ID) error
	NoteHistory(context.Context, id.ID) ([]api.NoteEvent, error)
	AcquireNoteLock(context.Context, id.ID) (api.NoteLock, error)
	TakeoverNoteLock(context.Context, id.ID, api.NoteLockRequest) (api.NoteLock, error)
	ReleaseNoteLock(context.Context, id.ID) error
	SaveNoteDraft(context.Context, id.ID, api.NoteDraftRequest) (api.NoteDraft, error)
	GetNoteDraft(context.Context, id.ID) (api.NoteDraft, error)
	DeleteNoteDraft(context.Context, id.ID) error
	NoteDrafts(context.Context, id.ID) ([]api.NoteDraft, error)
	DeleteNoteRecoveryDraft(context.Context, id.ID, id.ID) error
	AddNoteComment(context.Context, id.ID, api.NoteCommentRequest) (api.NoteComment, error)
	AddNoteAttachment(context.Context, id.ID, api.NoteAttachmentRequest) (api.NoteAttachment, error)
	DeleteNoteAttachment(context.Context, id.ID, id.ID) error
	NoteLinks(context.Context, id.ID) ([]api.NoteLink, error)
	AddNoteLink(context.Context, id.ID, api.NoteLinkRequest) (api.NoteLink, error)
	DeleteNoteLink(context.Context, id.ID, id.ID) error
	NoteTemplates(context.Context, id.ID) ([]api.NoteTemplate, error)
	CreateNoteTemplate(context.Context, id.ID, api.NoteTemplateRequest) (api.NoteTemplate, error)
	UpdateNoteTemplate(context.Context, id.ID, api.NoteTemplateRequest) (api.NoteTemplate, error)
	DeleteNoteTemplate(context.Context, id.ID) error
}

var _ NotesAPI = (*client.Client)(nil)

type noteConversion struct {
	kind, text, title, output, date, clock, zone, category string
	assignee                                               id.ID
	members                                                []api.Member
	membersLoading                                         bool
	err                                                    string
	memberOpen                                             bool
	target                                                 id.ID
	linkPending, uncertain                                 bool
}

type noteDraftKey struct{ user, note id.ID }

// The queue survives scope/account resets. Intent order, rather than goroutine
// scheduling, prevents an old Save from resurrecting a deleted recovery copy.
type noteDiskOrder struct {
	mu   sync.Mutex
	next uint64 // view thread only
	done map[noteDraftKey]uint64
}

type notesUIState struct {
	seq                                                                                  uint64
	space                                                                                id.ID
	items                                                                                []api.Note
	templates                                                                            []api.NoteTemplate
	loaded, loading, busy                                                                bool
	err, notice, filter                                                                  string
	selected                                                                             id.ID
	note                                                                                 *api.Note
	detailSeq                                                                            uint64
	detailLoading                                                                        bool
	detailErr                                                                            string
	title                                                                                string
	editor                                                                               *noteDocumentEditor
	dirty                                                                                bool
	localStatus, localErr, draftErr                                                      string
	recovery                                                                             []api.NoteDraft
	localRecovery                                                                        *LocalNoteDraft
	compare                                                                              *LocalNoteDraft
	takeoverConfirm, deleteConfirm                                                       bool
	comment, attachmentName, attachmentURL, attachmentType, attachmentSize, templateName string
	history                                                                              []api.NoteEvent
	historyErr                                                                           string
	links                                                                                []api.NoteLink
	linksErr                                                                             string
	createOpen                                                                           bool
	newTitle                                                                             string
	templateID                                                                           id.ID
	templateKind                                                                         string
	templateOpen                                                                         bool
	templateEditing, templateDeleteConfirm                                               id.ID
	conversion                                                                           *noteConversion
	pendingLinks                                                                         []api.NoteLinkRequest
	revision, localRevision, serverRevision                                              uint64
	baseVersion                                                                          int64
	conflict                                                                             bool
	changedAt, lastTick, renewedAt                                                       time.Time
	draftBusy, leaseBusy, editing                                                        bool
	leaseService                                                                         NotesAPI
	disk                                                                                 *noteDiskOrder
}

func (a *App) notesService() (NotesAPI, bool) {
	if c, ok := a.api.(*client.Client); ok {
		return c.AccountSession(), true
	}
	s, ok := a.api.(NotesAPI)
	return s, ok
}

func (a *App) noteNow() time.Time {
	if !a.notes.lastTick.IsZero() {
		return a.notes.lastTick
	}
	return time.Now()
}

func (a *App) resetNotes() {
	a.persistNoteLocal(nil)
	a.releaseEditingNote()
	seq, disk := a.notes.seq+1, a.notes.disk
	a.notes = notesUIState{seq: seq, disk: disk}
}

func (a *App) ensureNotes() {
	space := a.selectedSpaceID()
	if a.notes.space != space {
		a.resetNotes()
		a.notes.space = space
	}
	if space != id.Nil && !a.notes.loaded && !a.notes.loading && a.notes.err == "" {
		a.loadNotes()
	}
}

func (a *App) loadNotes() {
	service, ok := a.notesService()
	if !ok {
		a.notes.err = "Layanan Notes belum tersedia."
		return
	}
	if a.notes.space == id.Nil || a.notes.space != a.selectedSpaceID() || a.notes.loading || a.notes.busy || a.screen != screenShell {
		return
	}
	gen, seq, space := a.gen, a.notes.seq, a.notes.space
	a.notes.loading, a.notes.err = true, ""
	a.async(func() func() {
		items, err := service.SpaceNotes(context.Background(), space)
		var templates []api.NoteTemplate
		if err == nil {
			templates, err = service.NoteTemplates(context.Background(), space)
		}
		return func() {
			if gen != a.gen || seq != a.notes.seq || space != a.selectedSpaceID() {
				return
			}
			a.notes.loading = false
			if err != nil {
				a.noteFailure(err)
				return
			}
			a.conn = connOnline
			a.notes.items, a.notes.templates, a.notes.loaded = items, templates, true
			if a.notes.selected != id.Nil && !slices.ContainsFunc(items, func(n api.Note) bool { return n.ID == a.notes.selected }) {
				a.closeNote()
			} else if a.notes.selected != id.Nil {
				a.loadNoteDetail(true)
			}
		}
	})
}

func (a *App) notesWriteBlocked() bool {
	s := a.selectedSpace()
	return a.screen != screenShell || a.conn != connOnline || a.notes.busy || a.notes.loading || a.notes.detailLoading || s == nil || s.ID != a.notes.space || s.ReadOnly || s.PeriodStatus == api.PeriodClosed
}

func (a *App) notesCanCreate() bool {
	if a.isAdmin() || a.notesWriteBlocked() {
		return false
	}
	for _, r := range a.selectedSpace().Roles {
		if !r.ReadOnly && r.PeriodStatus != api.PeriodClosed && slices.Contains(r.Capabilities, api.CapNotesManage) {
			return true
		}
	}
	return false
}

func (a *App) notesCanEdit() bool {
	n := a.notes.note
	return !a.notesWriteBlocked() && !a.notes.conflict && a.notes.editing && n != nil && n.Permissions.Manage && n.Lock != nil && n.Lock.HeldByCaller && a.noteNow().Before(n.Lock.ExpiresAt)
}

func (a *App) noteFailure(err error) {
	if client.IsUnauthenticated(err) {
		a.failed(err)
		return
	}
	var e *client.Error
	if errors.As(err, &e) && (e.Status == http.StatusForbidden || e.Status == http.StatusNotFound) {
		a.persistNoteLocal(nil)
		a.releaseEditingNote()
		a.notes.detailSeq++
		a.notes.selected, a.notes.note, a.notes.editor = id.Nil, nil, nil
		a.notes.localRecovery, a.notes.compare, a.notes.conversion = nil, nil, nil
		a.notes.history, a.notes.links, a.notes.recovery = nil, nil, nil
		a.notes.items, a.notes.templates, a.notes.pendingLinks = nil, nil, nil
		a.notes.loaded = false
		a.notes.detailLoading, a.notes.dirty = false, false
	}
	if errors.As(err, &e) && e.Code == api.CodePeriodClosed {
		a.markPeriodClosed(a.periodID)
		a.notes.editing = false
	}
	a.notes.err = a.failed(err)
}

func cloneNote(n api.Note) api.Note {
	n.Content = cloneNoteDocument(n.Content)
	n.Attachments = slices.Clone(n.Attachments)
	n.Comments = slices.Clone(n.Comments)
	if n.Lock != nil {
		lock := *n.Lock
		n.Lock = &lock
	}
	return n
}

func (a *App) replaceNote(n api.Note) {
	for i := range a.notes.items {
		if a.notes.items[i].ID == n.ID {
			a.notes.items[i] = n
			return
		}
	}
	a.notes.items = append(a.notes.items, n)
}

func (a *App) openNote(nid id.ID) {
	if a.notes.busy || nid == id.Nil || !slices.ContainsFunc(a.notes.items, func(n api.Note) bool { return n.ID == nid }) {
		return
	}
	a.closeNote()
	a.notes.selected = nid
	a.chrome.appearance = false
	a.loadNoteDetail(false)
}

func (a *App) closeNote() {
	if a.notes.busy {
		return
	}
	a.persistNoteLocal(nil)
	a.releaseEditingNote()
	a.notes.detailSeq++
	a.notes.selected, a.notes.note, a.notes.editor = id.Nil, nil, nil
	a.notes.title, a.notes.detailErr, a.notes.localStatus, a.notes.localErr, a.notes.draftErr = "", "", "", "", ""
	a.notes.localRecovery, a.notes.compare, a.notes.conversion = nil, nil, nil
	a.notes.recovery, a.notes.links, a.notes.history, a.notes.pendingLinks = nil, nil, nil, nil
	a.notes.detailLoading, a.notes.dirty, a.notes.draftBusy = false, false, false
	a.notes.leaseBusy = false
	a.notes.comment, a.notes.attachmentName, a.notes.attachmentURL, a.notes.attachmentType, a.notes.attachmentSize, a.notes.templateName = "", "", "", "", "", ""
	a.notes.historyErr, a.notes.linksErr = "", ""
	a.notes.baseVersion, a.notes.conflict = 0, false
	a.notes.revision, a.notes.localRevision, a.notes.serverRevision = 0, 0, 0
	a.notes.deleteConfirm, a.notes.takeoverConfirm = false, false
}

func (a *App) loadNoteDetail(preserve bool) {
	service, ok := a.notesService()
	nid := a.notes.selected
	if !ok || nid == id.Nil || a.notes.detailLoading {
		return
	}
	gen, seq, space, user, admin := a.gen, a.notes.seq, a.notes.space, a.user.ID, a.isAdmin()
	a.notes.detailSeq++
	detail := a.notes.detailSeq
	a.notes.detailLoading, a.notes.detailErr = true, ""
	store := a.NoteDrafts
	a.async(func() func() {
		n, err := service.GetNote(context.Background(), nid)
		var history []api.NoteEvent
		var links []api.NoteLink
		var drafts []api.NoteDraft
		var historyErr, linksErr, draftErr, localErr error
		var local *LocalNoteDraft
		if err == nil {
			if n.Permissions.Manage || n.CreatedBy.ID == user {
				history, historyErr = service.NoteHistory(context.Background(), nid)
			}
			links, linksErr = service.NoteLinks(context.Background(), nid)
			if !admin {
				drafts, draftErr = service.NoteDrafts(context.Background(), nid)
			}
			if store != nil {
				local, localErr = store.Load(user, nid)
			}
		}
		return func() {
			if gen != a.gen || seq != a.notes.seq || detail != a.notes.detailSeq || nid != a.notes.selected || space != a.selectedSpaceID() {
				return
			}
			a.notes.detailLoading = false
			if err != nil {
				a.noteFailure(err)
				return
			}
			if n.SpaceID != space {
				a.noteFailure(&client.Error{Status: 404, Code: api.CodeNotFound, Message: "Note tidak tersedia pada ruang ini."})
				return
			}
			for _, failure := range []error{historyErr, linksErr, draftErr} {
				if client.IsUnauthenticated(failure) {
					a.failed(failure)
					return
				}
			}
			a.conn = connOnline
			canonical := cloneNote(n)
			a.notes.note = &canonical
			a.replaceNote(n)
			a.notes.history, a.notes.links, a.notes.recovery = history, links, drafts
			a.notes.historyErr, a.notes.linksErr, a.notes.draftErr = "", "", ""
			if historyErr != nil {
				a.notes.historyErr = a.failed(historyErr)
			}
			if linksErr != nil {
				a.notes.linksErr = a.failed(linksErr)
			}
			if draftErr != nil {
				a.notes.draftErr = a.failed(draftErr)
			}
			if localErr != nil {
				a.notes.localErr = "Draf perangkat belum dapat dibaca: " + localErr.Error()
			}
			if local != nil && local.NoteID == nid && local.SpaceID == space {
				a.notes.localRecovery = copyLocalNoteDraft(local)
			}
			if preserve && a.notes.dirty && a.notes.baseVersion != n.Version {
				a.notes.compare = a.noteLocalSnapshot()
				a.notes.conflict = true
			}
			if !preserve || !a.notes.dirty {
				a.notes.title = n.Title
				a.notes.editor = newNoteDocumentEditor(n.Content)
				a.notes.baseVersion, a.notes.conflict = n.Version, false
			}
			if n.Lock != nil && n.Lock.HeldByCaller {
				a.notes.editing, a.notes.leaseService, a.notes.renewedAt = true, service, a.noteNow()
			} else {
				a.notes.editing = false
			}
			if a.notes.editor != nil {
				a.notes.editor.SetEditable(a.notesCanEdit())
			}
			if local != nil && local.NoteID == nid && local.SpaceID == space && len(local.PendingLinks) > 0 {
				a.notes.pendingLinks = slices.Clone(local.PendingLinks)
				a.showPendingNoteLink()
			}
		}
	})
}

func (a *App) beginNote() {
	if !a.notesCanCreate() {
		return
	}
	a.notes.createOpen, a.notes.newTitle, a.notes.templateID = true, "", id.Nil
	if len(a.notes.templates) > 0 {
		a.notes.templateKind = a.notes.templates[0].Kind
	}
}

func (a *App) createNote() {
	if !a.notes.createOpen || !a.notesCanCreate() {
		return
	}
	title := strings.TrimSpace(a.notes.newTitle)
	if title == "" || utf8.RuneCountInString(title) > 200 {
		a.notes.err = "Isi judul note, maksimal 200 karakter."
		return
	}
	req := api.NoteCreateRequest{Title: title, TemplateKind: a.notes.templateKind}
	if a.notes.templateID != id.Nil {
		if !slices.ContainsFunc(a.notes.templates, func(t api.NoteTemplate) bool { return !t.Builtin && t.ID == a.notes.templateID }) {
			return
		}
		v := a.notes.templateID
		req.TemplateID = &v
	}
	space := a.notes.space
	a.writeNote(func(service NotesAPI) (func(), error) {
		n, err := service.CreateNote(context.Background(), space, req)
		return func() {
			a.notes.createOpen = false
			a.replaceNote(n)
			a.openNote(n.ID)
			a.notes.notice = "Note tersimpan di server."
		}, err
	})
}

func (a *App) writeNote(work func(NotesAPI) (func(), error), abandoned ...func(NotesAPI)) {
	service, ok := a.notesService()
	if !ok || a.notesWriteBlocked() {
		return
	}
	gen, seq, detail, selected, space, user := a.gen, a.notes.seq, a.notes.detailSeq, a.notes.selected, a.notes.space, a.user.ID
	a.notes.busy, a.notes.err, a.notes.notice = true, "", ""
	a.async(func() func() {
		after, err := work(service)
		return func() {
			if gen != a.gen || seq != a.notes.seq || detail != a.notes.detailSeq || selected != a.notes.selected || space != a.selectedSpaceID() {
				if err == nil && len(abandoned) > 0 && (a.notes.selected != selected || a.user.ID != user) {
					a.async(func() func() { abandoned[0](service); return nil })
				}
				return
			}
			a.notes.busy = false
			if err != nil {
				a.noteFailure(err)
				var e *client.Error
				if errors.As(err, &e) && e.Status == http.StatusConflict && a.notes.note != nil {
					a.persistNoteLocal(nil)
					if a.notes.dirty {
						a.notes.compare = a.noteLocalSnapshot()
						a.notes.conflict = true
					}
					a.notes.editing = false
					a.loadNoteDetail(true)
				}
				return
			}
			a.conn = connOnline
			if after != nil {
				after()
			}
		}
	})
}

func (a *App) saveNote() {
	if !a.notesCanEdit() || a.notes.editor == nil {
		return
	}
	nid, revision := a.notes.selected, a.notes.revision
	title, content := strings.TrimSpace(a.notes.title), a.notes.editor.Content()
	if title == "" || utf8.RuneCountInString(title) > 200 {
		a.notes.err = "Isi judul note, maksimal 200 karakter."
		return
	}
	if err := api.ValidateNoteContent(content); err != nil {
		a.notes.err = err.Error()
		return
	}
	req := api.NoteWriteRequest{Title: title, Content: content, ExpectedVersion: a.notes.baseVersion}
	a.persistNoteLocal(nil)
	a.writeNote(func(service NotesAPI) (func(), error) {
		n, err := service.UpdateNote(context.Background(), nid, req)
		return func() {
			canonical := cloneNote(n)
			a.notes.note = &canonical
			a.replaceNote(n)
			if revision == a.notes.revision {
				a.notes.title, a.notes.editor, a.notes.dirty = n.Title, newNoteDocumentEditor(n.Content), false
				a.notes.baseVersion, a.notes.conflict = n.Version, false
				a.notes.editor.SetEditable(a.notesCanEdit())
				a.notes.recovery = slices.DeleteFunc(a.notes.recovery, func(d api.NoteDraft) bool { return d.CurrentSession })
				if len(a.notes.pendingLinks) == 0 {
					a.deleteNoteLocal()
				} else {
					a.persistNoteLocal(nil)
				}
			}
			a.notes.notice = "Note tersimpan di server."
		}, err
	})
}

func (a *App) beginNoteEditing() { a.acquireEditingNote(false) }
func (a *App) takeoverNoteEditing() {
	if !a.notes.takeoverConfirm {
		return
	}
	a.acquireEditingNote(true)
}
func (a *App) acquireEditingNote(force bool) {
	n := a.notes.note
	if n == nil || !n.Permissions.Manage || a.notesWriteBlocked() {
		return
	}
	nid := n.ID
	a.writeNote(func(service NotesAPI) (func(), error) {
		var lock api.NoteLock
		var err error
		if force {
			lock, err = service.TakeoverNoteLock(context.Background(), nid, api.NoteLockRequest{Force: true})
		} else {
			lock, err = service.AcquireNoteLock(context.Background(), nid)
		}
		return func() {
			a.notes.note.Lock = &lock
			a.notes.editing, a.notes.takeoverConfirm, a.notes.leaseService, a.notes.renewedAt = true, false, service, a.noteNow()
			if a.notes.editor != nil {
				a.notes.editor.SetEditable(a.notesCanEdit())
			}
		}, err
	}, func(service NotesAPI) { _ = service.ReleaseNoteLock(context.Background(), nid) })
}

func (a *App) releaseEditingNote() {
	n, service := a.notes.note, a.notes.leaseService
	a.notes.editing = false
	if a.notes.editor != nil {
		a.notes.editor.SetEditable(false)
	}
	if n == nil || n.Lock == nil || !n.Lock.HeldByCaller || service == nil {
		return
	}
	nid := n.ID
	a.notes.leaseService = nil
	gen, seq, detail := a.gen, a.notes.seq, a.notes.detailSeq
	a.async(func() func() {
		err := service.ReleaseNoteLock(context.Background(), nid)
		return func() {
			if gen != a.gen || seq != a.notes.seq || detail != a.notes.detailSeq || a.notes.selected != nid {
				return
			}
			if err != nil {
				a.notes.draftErr = "Kunci editor belum dapat dilepas; kunci tetap memiliki masa berlaku."
				return
			}
			if a.notes.note != nil {
				a.notes.note.Lock = nil
			}
		}
	})
}

func (a *App) finishNoteEditing() {
	if a.notes.busy {
		return
	}
	a.persistNoteLocal(nil)
	a.saveNoteRecovery()
	a.releaseEditingNote()
}

func (a *App) deleteNote() {
	if a.notes.note == nil || !a.notes.note.Permissions.Manage || !a.notes.deleteConfirm {
		return
	}
	nid := a.notes.selected
	a.writeNote(func(service NotesAPI) (func(), error) {
		err := service.DeleteNote(context.Background(), nid)
		return func() {
			a.notes.dirty = false
			a.notes.pendingLinks = nil
			a.deleteNoteLocal()
			a.notes.items = slices.DeleteFunc(a.notes.items, func(n api.Note) bool { return n.ID == nid })
			a.closeNote()
			a.notes.notice = "Note dihapus dari ruang. Riwayat server tetap tersimpan."
		}, err
	})
}

func (a *App) markNoteChanged() {
	if a.notes.note == nil || a.notes.editor == nil {
		return
	}
	a.notes.revision++
	a.notes.dirty, a.notes.changedAt = true, a.noteNow()
	a.notes.localStatus = "Perubahan belum tersimpan."
}

func (a *App) tickNotes(now time.Time) {
	s := a.selectedSpace()
	readable := false
	if s != nil && s.ID == a.notes.space {
		readable = slices.Contains(s.Capabilities, api.CapNotesRead)
		for _, role := range s.Roles {
			readable = readable || slices.Contains(role.Capabilities, api.CapNotesRead)
		}
	}
	if a.notes.space != id.Nil && (s == nil || s.ID != a.notes.space || !readable) {
		a.resetNotes()
		return
	}
	a.notes.lastTick = now
	if a.notes.editor != nil {
		a.notes.editor.SetEditable(a.notesCanEdit())
	}
	if a.notes.dirty && now.Sub(a.notes.changedAt) >= 750*time.Millisecond && a.notes.localRevision != a.notes.revision {
		a.persistNoteLocal(nil)
	}
	if a.notes.dirty && now.Sub(a.notes.changedAt) >= 3*time.Second && a.notes.serverRevision != a.notes.revision && !a.notes.draftBusy && a.notes.draftErr == "" {
		a.saveNoteRecovery()
	}
	n := a.notes.note
	if !a.notes.editing || n == nil || n.Lock == nil || !n.Lock.HeldByCaller || a.notes.leaseBusy || a.notes.busy || a.conn != connOnline || now.Sub(a.notes.renewedAt) < time.Minute {
		return
	}
	service := a.notes.leaseService
	if service == nil {
		return
	}
	gen, seq, detail, nid := a.gen, a.notes.seq, a.notes.detailSeq, n.ID
	a.notes.leaseBusy = true
	a.async(func() func() {
		lock, err := service.AcquireNoteLock(context.Background(), nid)
		return func() {
			if gen != a.gen || seq != a.notes.seq || detail != a.notes.detailSeq || nid != a.notes.selected {
				return
			}
			a.notes.leaseBusy = false
			a.notes.renewedAt = now
			if err != nil {
				a.notes.editing = false
				a.persistNoteLocal(nil)
				a.noteFailure(err)
				if a.notes.note != nil {
					a.loadNoteDetail(true)
				}
				return
			}
			a.notes.note.Lock = &lock
		}
	})
}

func copyLocalNoteDraft(d *LocalNoteDraft) *LocalNoteDraft {
	if d == nil {
		return nil
	}
	copy := *d
	copy.Content = cloneNoteDocument(d.Content)
	copy.PendingLinks = slices.Clone(d.PendingLinks)
	return &copy
}

func (a *App) noteLocalSnapshot() *LocalNoteDraft {
	if a.notes.note == nil || a.notes.editor == nil || a.notes.selected == id.Nil {
		return nil
	}
	return &LocalNoteDraft{SpaceID: a.notes.space, NoteID: a.notes.selected, Title: a.notes.title, Content: a.notes.editor.Content(), BaseVersion: a.notes.baseVersion, SavedAt: a.noteNow().UTC(), PendingLinks: slices.Clone(a.notes.pendingLinks)}
}

func (a *App) queueNoteDisk(remove bool, draft *LocalNoteDraft, after func(bool)) {
	store := a.NoteDrafts
	if store == nil {
		a.notes.localErr = "Penyimpanan draf perangkat belum tersedia."
		if after != nil {
			after(false)
		}
		return
	}
	if draft == nil {
		return
	}
	if a.notes.disk == nil {
		a.notes.disk = &noteDiskOrder{done: map[noteDraftKey]uint64{}}
	}
	order := a.notes.disk
	order.next++
	operation := order.next
	key := noteDraftKey{user: a.user.ID, note: draft.NoteID}
	gen, seq, detail, revision := a.gen, a.notes.seq, a.notes.detailSeq, a.notes.revision
	draft = copyLocalNoteDraft(draft)
	if !remove {
		a.notes.localRevision = revision
	}
	a.async(func() func() {
		order.mu.Lock()
		var err error
		if operation > order.done[key] {
			order.done[key] = operation
			if remove {
				err = store.Delete(key.user, key.note)
			} else {
				err = store.Save(key.user, key.note, *draft)
			}
		}
		order.mu.Unlock()
		return func() {
			if gen != a.gen || seq != a.notes.seq || detail != a.notes.detailSeq || key.note != a.notes.selected {
				return
			}
			order.mu.Lock()
			latest := order.done[key] == operation
			order.mu.Unlock()
			if !latest {
				return
			}
			if err != nil {
				a.notes.localErr = "Draf perangkat belum tersimpan: " + err.Error()
				if after != nil {
					after(false)
				}
				return
			}
			a.notes.localErr = ""
			if remove {
				a.notes.localRecovery = nil
				if !a.notes.dirty && revision == a.notes.revision {
					a.notes.localStatus = "Tersimpan di server."
				}
			} else {
				a.notes.localRecovery = copyLocalNoteDraft(draft)
				if revision == a.notes.revision {
					a.notes.localStatus = "Tersimpan di perangkat — belum disimpan sebagai note resmi."
				}
			}
			if after != nil {
				after(true)
			}
		}
	})
}

func (a *App) persistNoteLocal(after func(bool)) {
	if !a.notes.dirty && len(a.notes.pendingLinks) == 0 {
		if after != nil {
			after(true)
		}
		return
	}
	a.queueNoteDisk(false, a.noteLocalSnapshot(), after)
}

func (a *App) deleteNoteLocal() { a.queueNoteDisk(true, a.noteLocalSnapshot(), nil) }

// FlushNoteDraft is the window-close boundary: persist the last keystrokes
// synchronously, ordered after queued writes, before the process can exit.
// Closing may be prevented by the caller when this reports an error.
func (a *App) FlushNoteDraft() error {
	if a.notes.dirty || len(a.notes.pendingLinks) > 0 {
		draft := a.noteLocalSnapshot()
		if draft != nil {
			if a.NoteDrafts == nil {
				return errors.New("draf perangkat belum dapat disimpan")
			}
			if a.notes.disk == nil {
				a.notes.disk = &noteDiskOrder{done: map[noteDraftKey]uint64{}}
			}
			order := a.notes.disk
			order.next++
			operation := order.next
			key := noteDraftKey{user: a.user.ID, note: draft.NoteID}
			order.mu.Lock()
			order.done[key] = operation
			err := a.NoteDrafts.Save(key.user, key.note, *draft)
			order.mu.Unlock()
			if err != nil {
				a.notes.localErr = err.Error()
				return err
			}
			a.notes.localRecovery = copyLocalNoteDraft(draft)
			a.notes.localStatus = "Tersimpan di perangkat — belum disimpan sebagai note resmi."
		}
	}
	// No Window.Update callback is queued during window destruction. Expiry
	// remains the fallback when shutdown happens before this request finishes.
	if n, service := a.notes.note, a.notes.leaseService; n != nil && n.Lock != nil && n.Lock.HeldByCaller && service != nil {
		nid := n.ID
		a.notes.editing, a.notes.leaseService = false, nil
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = service.ReleaseNoteLock(ctx, nid)
		}()
	}
	return nil
}

func (a *App) saveNoteRecovery() {
	n, service := a.notes.note, a.notes.leaseService
	if service == nil {
		service, _ = a.notesService()
	}
	if n == nil || service == nil || !n.Permissions.Manage || a.notes.editor == nil || a.notesWriteBlocked() || a.notes.draftBusy || !a.notes.dirty {
		return
	}
	content := a.notes.editor.Content()
	if err := api.ValidateNoteContent(content); err != nil {
		a.notes.draftErr = err.Error()
		return
	}
	req := api.NoteDraftRequest{Title: a.notes.title, Content: content, BaseVersion: a.notes.baseVersion}
	gen, seq, detail, nid, revision := a.gen, a.notes.seq, a.notes.detailSeq, n.ID, a.notes.revision
	a.notes.draftBusy, a.notes.draftErr = true, ""
	a.async(func() func() {
		draft, err := service.SaveNoteDraft(context.Background(), nid, req)
		return func() {
			if gen != a.gen || seq != a.notes.seq || detail != a.notes.detailSeq || nid != a.notes.selected {
				return
			}
			a.notes.draftBusy = false
			if err != nil {
				if client.IsUnauthenticated(err) {
					a.failed(err)
					return
				}
				var apiErr *client.Error
				if errors.As(err, &apiErr) && apiErr.Code == api.CodePeriodClosed {
					a.noteFailure(err)
				}
				a.notes.draftErr = a.failed(err)
				return
			}
			a.notes.serverRevision = revision
			a.notes.recovery = slices.DeleteFunc(a.notes.recovery, func(d api.NoteDraft) bool { return d.CurrentSession })
			a.notes.recovery = append(a.notes.recovery, draft)
		}
	})
}

func (a *App) restoreNoteRecovery(index int, local bool) {
	if a.notes.note == nil || a.notes.busy {
		return
	}
	var draft *LocalNoteDraft
	if local {
		draft = copyLocalNoteDraft(a.notes.localRecovery)
	} else if index >= 0 && index < len(a.notes.recovery) {
		d := a.notes.recovery[index]
		draft = &LocalNoteDraft{SpaceID: a.notes.space, NoteID: a.notes.selected, Title: d.Title, Content: cloneNoteDocument(d.Content), BaseVersion: d.BaseVersion, SavedAt: d.SavedAt}
	}
	if draft == nil || draft.NoteID != a.notes.selected || draft.SpaceID != a.notes.space {
		return
	}
	a.notes.compare = draft
}

func (a *App) useComparedNoteDraft() {
	d := a.notes.compare
	if d == nil || a.notes.note == nil || d.NoteID != a.notes.selected || d.SpaceID != a.notes.space || a.notes.busy {
		return
	}
	// This explicit action is the only point at which an older working copy is
	// applied to the fresh official version. It still needs a valid lease to save.
	a.notes.title, a.notes.editor = d.Title, newNoteDocumentEditor(d.Content)
	a.notes.baseVersion, a.notes.conflict = a.notes.note.Version, false
	for _, link := range d.PendingLinks {
		if !slices.Contains(a.notes.pendingLinks, link) {
			a.notes.pendingLinks = append(a.notes.pendingLinks, link)
		}
	}
	a.notes.compare = nil
	a.markNoteChanged()
	a.persistNoteLocal(nil)
	a.notes.editor.SetEditable(a.notesCanEdit())
	a.showPendingNoteLink()
}

func (a *App) discardNoteRecovery(draft id.ID) {
	if a.notes.note == nil || draft == id.Nil {
		return
	}
	nid := a.notes.selected
	a.writeNote(func(service NotesAPI) (func(), error) {
		err := service.DeleteNoteRecoveryDraft(context.Background(), nid, draft)
		return func() {
			a.notes.recovery = slices.DeleteFunc(a.notes.recovery, func(d api.NoteDraft) bool { return d.ID == draft })
		}, err
	})
}

func (a *App) addNoteComment() {
	if a.notes.note == nil || !a.notes.note.Permissions.Comment {
		return
	}
	body, nid := strings.TrimSpace(a.notes.comment), a.notes.selected
	if body == "" {
		a.notes.err = "Isi komentar terlebih dahulu."
		return
	}
	a.writeNote(func(service NotesAPI) (func(), error) {
		comment, err := service.AddNoteComment(context.Background(), nid, api.NoteCommentRequest{Body: body})
		return func() {
			a.notes.note.Comments = append(a.notes.note.Comments, comment)
			a.notes.comment = ""
			a.notes.notice = "Komentar tersimpan."
		}, err
	})
}

func (a *App) addNoteAttachment() {
	if a.notes.note == nil || !a.notes.note.Permissions.Manage {
		return
	}
	size := int64(0)
	if strings.TrimSpace(a.notes.attachmentSize) != "" {
		var err error
		size, err = strconv.ParseInt(strings.TrimSpace(a.notes.attachmentSize), 10, 64)
		if err != nil || size < 0 {
			a.notes.err = "Ukuran lampiran harus bilangan byte nol atau lebih."
			return
		}
	}
	req := api.NoteAttachmentRequest{Name: strings.TrimSpace(a.notes.attachmentName), URL: strings.TrimSpace(a.notes.attachmentURL), ContentType: strings.TrimSpace(a.notes.attachmentType), SizeBytes: size}
	if req.Name == "" {
		a.notes.err = "Nama lampiran wajib diisi."
		return
	}
	nid := a.notes.selected
	a.writeNote(func(service NotesAPI) (func(), error) {
		attachment, err := service.AddNoteAttachment(context.Background(), nid, req)
		return func() {
			a.notes.note.Attachments = append(a.notes.note.Attachments, attachment)
			a.notes.attachmentName, a.notes.attachmentURL, a.notes.attachmentType, a.notes.attachmentSize = "", "", "", ""
			a.notes.notice = "Metadata lampiran tersimpan."
		}, err
	})
}

func (a *App) removeNoteAttachment(attachment id.ID) {
	if a.notes.note == nil || !a.notes.note.Permissions.Manage || !slices.ContainsFunc(a.notes.note.Attachments, func(x api.NoteAttachment) bool { return x.ID == attachment }) {
		return
	}
	nid := a.notes.selected
	a.writeNote(func(service NotesAPI) (func(), error) {
		err := service.DeleteNoteAttachment(context.Background(), nid, attachment)
		return func() {
			a.notes.note.Attachments = slices.DeleteFunc(a.notes.note.Attachments, func(x api.NoteAttachment) bool { return x.ID == attachment })
		}, err
	})
}

func (a *App) noteTemplateRequest() (api.NoteTemplateRequest, bool) {
	if !a.notesCanCreate() || a.notes.editor == nil || strings.TrimSpace(a.notes.templateName) == "" {
		a.notes.err = "Isi nama template dan buka dokumen yang akan disimpan."
		return api.NoteTemplateRequest{}, false
	}
	req := api.NoteTemplateRequest{Name: strings.TrimSpace(a.notes.templateName), Content: a.notes.editor.Content()}
	if err := api.ValidateNoteContent(req.Content); err != nil {
		a.notes.err = err.Error()
		return req, false
	}
	return req, true
}

func (a *App) saveNoteTemplate() {
	req, ok := a.noteTemplateRequest()
	if !ok {
		return
	}
	space := a.notes.space
	a.writeNote(func(service NotesAPI) (func(), error) {
		template, err := service.CreateNoteTemplate(context.Background(), space, req)
		return func() {
			a.notes.templates = append(a.notes.templates, template)
			a.notes.templateName = ""
			a.notes.notice = "Template tersimpan; note lain tidak diubah."
		}, err
	})
}

func (a *App) updateNoteTemplate(tid id.ID) {
	req, ok := a.noteTemplateRequest()
	if !ok || !slices.ContainsFunc(a.notes.templates, func(t api.NoteTemplate) bool { return t.ID == tid && !t.Builtin }) {
		return
	}
	a.writeNote(func(service NotesAPI) (func(), error) {
		template, err := service.UpdateNoteTemplate(context.Background(), tid, req)
		return func() {
			for i := range a.notes.templates {
				if a.notes.templates[i].ID == tid {
					a.notes.templates[i] = template
				}
			}
			a.notes.templateEditing = id.Nil
			a.notes.notice = "Template diperbarui; note yang sudah dibuat tetap sama."
		}, err
	})
}

func (a *App) deleteNoteTemplate(tid id.ID) {
	if !a.notesCanCreate() || a.notes.templateDeleteConfirm != tid || !slices.ContainsFunc(a.notes.templates, func(t api.NoteTemplate) bool { return t.ID == tid && !t.Builtin }) {
		return
	}
	a.writeNote(func(service NotesAPI) (func(), error) {
		err := service.DeleteNoteTemplate(context.Background(), tid)
		return func() {
			a.notes.templates = slices.DeleteFunc(a.notes.templates, func(t api.NoteTemplate) bool { return t.ID == tid })
			a.notes.templateDeleteConfirm = id.Nil
			if a.notes.templateID == tid {
				a.notes.templateID = id.Nil
			}
			a.notes.notice = "Template dihapus; note lama tetap tersimpan."
		}, err
	})
}

func (a *App) beginNoteConversion(kind, text string) {
	n := a.notes.note
	if n == nil || !n.Permissions.Manage || a.notesWriteBlocked() || (kind != api.NoteLinkTask && kind != api.NoteLinkReminder) {
		return
	}
	if a.notes.conversion != nil && (a.notes.conversion.target != id.Nil || a.notes.conversion.uncertain) {
		return
	}
	zone := timeutil.DefaultZoneName
	if p := a.period(); p != nil && p.TimeZone != "" {
		zone = p.TimeZone
	}
	c := &noteConversion{kind: kind, text: text, zone: zone}
	a.notes.conversion = c
	if kind != api.NoteLinkTask {
		return
	}
	if !a.spacePermissionsForDivision(n.OwnerDivisionID, api.CapTasksManage) {
		c.err = "Pembuatan tugas memerlukan kewenangan delegasi pada divisi pemilik note."
		return
	}
	service, ok := a.workspaceService()
	if !ok {
		c.err = "Direktori anggota belum tersedia."
		return
	}
	gen, seq, detail, space, period, division := a.gen, a.notes.seq, a.notes.detailSeq, a.notes.space, a.periodID, n.OwnerDivisionID
	c.membersLoading = true
	a.async(func() func() {
		members, err := service.Members(context.Background(), period, division)
		return func() {
			if gen != a.gen || seq != a.notes.seq || detail != a.notes.detailSeq || space != a.selectedSpaceID() || a.notes.conversion != c {
				return
			}
			c.membersLoading = false
			if err != nil {
				if client.IsUnauthenticated(err) {
					a.failed(err)
					return
				}
				c.err = a.failed(err)
				return
			}
			c.members = members
		}
	})
}

func (a *App) saveNoteConversion() {
	c, n := a.notes.conversion, a.notes.note
	if c == nil || n == nil || !n.Permissions.Manage || a.notesWriteBlocked() || c.uncertain {
		return
	}
	if c.target != id.Nil {
		a.retryNoteConversionLink()
		return
	}
	if strings.TrimSpace(c.title) == "" {
		c.err = "Isi judul tujuan secara eksplisit."
		return
	}
	date, at, err := parseTaskDate(c.date, c.clock, c.zone, true)
	if err != nil {
		c.err = "Tenggat: " + err.Error()
		return
	}
	if _, err := timeutil.LoadZone(c.zone); err != nil {
		c.err = "Zona waktu tidak valid."
		return
	}
	var create func() (id.ID, error)
	if c.kind == api.NoteLinkTask {
		service, ok := a.taskService()
		if !ok || !a.spacePermissionsForDivision(n.OwnerDivisionID, api.CapTasksManage) || c.membersLoading || !slices.ContainsFunc(c.members, func(m api.Member) bool { return m.UserID == c.assignee }) {
			c.err = "Pilih PJ aktif dari divisi yang dapat kamu delegasikan."
			return
		}
		if strings.TrimSpace(c.output) == "" {
			c.err = "Isi hasil tugas yang diminta."
			return
		}
		req := api.TaskWriteRequest{OwnerDivisionID: n.OwnerDivisionID, Title: strings.TrimSpace(c.title), Description: c.text, ExpectedOutput: c.output, AssigneeID: c.assignee, DeadlineDate: date, DeadlineAt: at, Category: c.category}
		space := a.notes.space
		create = func() (id.ID, error) {
			task, err := service.CreateTask(context.Background(), space, req)
			return task.ID, err
		}
	} else if c.kind == api.NoteLinkReminder {
		service, ok := a.reminderService()
		if !ok {
			c.err = "Layanan reminder belum tersedia."
			return
		}
		req := api.ReminderRequest{Title: strings.TrimSpace(c.title), Notes: c.text, DueKind: api.DueDate, DueOn: date, DueAt: at, TimeZone: c.zone}
		if at != nil {
			req.DueKind = api.DueInstant
		}
		create = func() (id.ID, error) {
			reminder, err := service.CreateReminder(context.Background(), req)
			return reminder.ID, err
		}
	} else {
		return
	}
	gen, seq, detail, nid, space := a.gen, a.notes.seq, a.notes.detailSeq, n.ID, a.notes.space
	owner, draftStore, sourceDraft := a.user.ID, a.NoteDrafts, a.noteLocalSnapshot()
	if a.notes.disk == nil {
		a.notes.disk = &noteDiskOrder{done: map[noteDraftKey]uint64{}}
	}
	disk := a.notes.disk
	a.notes.busy, c.err = true, ""
	a.async(func() func() {
		target, err := create()
		return func() {
			if gen != a.gen || seq != a.notes.seq || detail != a.notes.detailSeq || space != a.selectedSpaceID() || a.notes.selected != nid || a.notes.conversion != c {
				if err == nil && target != id.Nil && sourceDraft != nil && draftStore != nil {
					sourceDraft.PendingLinks = append(sourceDraft.PendingLinks, api.NoteLinkRequest{TargetType: c.kind, TargetID: target})
					a.persistDetachedNoteDraft(draftStore, disk, owner, sourceDraft)
				}
				return
			}
			a.notes.busy = false
			if err != nil {
				if client.IsUnauthenticated(err) {
					a.failed(err)
					return
				}
				c.err = a.failed(err)
				var response *client.Error
				if client.IsUnreachable(err) || (errors.As(err, &response) && response.Status >= 500) {
					c.uncertain = true
					c.err += " Hasil pembuatan belum dapat dipastikan. Periksa daftar tujuan sebelum membuat ulang."
				}
				return
			}
			c.target, c.linkPending = target, true
			link := api.NoteLinkRequest{TargetType: c.kind, TargetID: target}
			if !slices.Contains(a.notes.pendingLinks, link) {
				a.notes.pendingLinks = append(a.notes.pendingLinks, link)
			}
			a.persistNoteLocal(func(saved bool) {
				if saved {
					a.retryNoteConversionLink()
				} else {
					c.err = "Tujuan sudah dibuat. Identitasnya belum tersimpan di perangkat; coba tautkan tujuan ini tanpa membuat ulang."
				}
			})
		}
	})
}

// An acknowledged conversion may finish after the user leaves its source.
// Keep that target's identity under the original account without publishing
// old source data into the new account/Space or repeating target creation.
func (a *App) persistDetachedNoteDraft(store NoteDraftStore, order *noteDiskOrder, user id.ID, draft *LocalNoteDraft) {
	draft = copyLocalNoteDraft(draft)
	order.next++
	operation := order.next
	key := noteDraftKey{user: user, note: draft.NoteID}
	a.async(func() func() {
		order.mu.Lock()
		defer order.mu.Unlock()
		if operation > order.done[key] {
			order.done[key] = operation
			_ = store.Save(user, draft.NoteID, *draft)
		}
		return nil
	})
}

func (a *App) showPendingNoteLink() {
	if len(a.notes.pendingLinks) == 0 || a.notes.conversion != nil {
		return
	}
	p := a.notes.pendingLinks[0]
	a.notes.conversion = &noteConversion{kind: p.TargetType, target: p.TargetID, linkPending: true}
}

func (a *App) retryNoteConversionLink() {
	c, n := a.notes.conversion, a.notes.note
	if c == nil || c.target == id.Nil || n == nil || !n.Permissions.Manage || a.notesWriteBlocked() {
		return
	}
	req, nid := api.NoteLinkRequest{TargetType: c.kind, TargetID: c.target}, n.ID
	a.writeNote(func(service NotesAPI) (func(), error) {
		link, err := service.AddNoteLink(context.Background(), nid, req)
		var apiErr *client.Error
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusConflict {
			var current []api.NoteLink
			current, err = service.NoteLinks(context.Background(), nid)
			found := false
			if err == nil {
				for _, existing := range current {
					if existing.TargetType == req.TargetType && existing.TargetID == req.TargetID {
						link, found = existing, true
						break
					}
				}
				if !found {
					err = fmt.Errorf("tautan belum dapat dipastikan; muat ulang sebelum mencoba kembali")
				}
			}
		}
		return func() {
			if !slices.ContainsFunc(a.notes.links, func(l api.NoteLink) bool { return l.ID == link.ID }) {
				a.notes.links = append(a.notes.links, link)
			}
			a.notes.pendingLinks = slices.DeleteFunc(a.notes.pendingLinks, func(p api.NoteLinkRequest) bool { return p == req })
			a.notes.conversion = nil
			if a.notes.dirty || len(a.notes.pendingLinks) > 0 {
				a.persistNoteLocal(nil)
			} else {
				a.deleteNoteLocal()
			}
			a.showPendingNoteLink()
			a.notes.notice = "Tujuan dan rujukan note tersimpan. Isi note tidak dibagikan otomatis."
		}, err
	})
}
