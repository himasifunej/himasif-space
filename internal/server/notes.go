package server

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
)

// Notes and templates (PRD §14, NTE-01–09). A note is scoped to one space
// (a division space or Medhum). Reading is gated by notes.read through space
// membership; managing (create/edit/lock/delete/templates) by notes.manage
// on the note's owner division; commenting by comments.write. A closed
// period is read-only. Making a task from a note does not open the note
// (NTE-07).

// noteLockTTL is how long an editor's lock lasts before it may be taken
// over (NTE-08). The editor renews it while editing.
const noteLockTTL = 5 * time.Minute

// noteAccess fills a note's permissions and lock from the caller's current
// bindings, independently of who created it. Read needs notes.read in the
// note's space; manage needs notes.manage on the owner division; comment
// needs comments.write in the space. All writes require an open period.
func (s *Server) noteAccess(ctx context.Context, q store.Querier, p *principal, period api.Period, n *store.NoteRow) error {
	view, err := s.spaceView(ctx, q, p, n.SpaceID)
	if err != nil {
		return err
	}
	if !slices.Contains(view.Capabilities, api.CapNotesRead) {
		return errForbidden
	}
	comment := slices.Contains(view.Capabilities, api.CapCommentsWrite)

	// Management belongs to the note's owner division, even in a shared
	// space: a Humas kadiv cannot edit a Mediatek note shared into Medhum.
	rr := &http.Request{}
	rr.SetPathValue("periodId", period.ID.String())
	rr.SetPathValue("divisionId", n.OwnerDivisionID.String())
	sc, err := s.loadScope(ctx, q, rr, p, store.LockNone)
	if err != nil {
		return err
	}
	open := period.Status != api.PeriodClosed
	manage := open && slices.Contains(sc.caps, api.CapNotesManage)

	lock, held, err := s.store.NoteLock(ctx, q, n.ID)
	if err != nil {
		return err
	}
	now := s.now()
	if held {
		lock.HeldByCaller = lock.Editor.ID == p.User.ID
		n.Lock = &lock
	} else {
		n.Lock = nil
	}
	// canEdit: may manage, and no live lock held by someone else.
	canEdit := manage
	if held && lock.Editor.ID != p.User.ID && now.Before(lock.ExpiresAt) {
		canEdit = false
	}
	n.Permissions = api.NotePermissions{Manage: manage, Comment: open && comment, CanEdit: canEdit}
	return nil
}

// withNote wraps a note transaction: peek the period, lock period→note→
// accounts in a fixed order, recompute access, reject closed-period writes,
// then run fn. Lock order matches tasks and membership writes.
func (s *Server) withNote(r *http.Request, p *principal, write bool, fn func(store.Querier, api.Period, *store.NoteRow) error) error {
	if write && p.admin() {
		return errForbidden
	}
	nid, err := pathID(r, "noteId")
	if err != nil {
		return err
	}
	pid, err := s.store.NotePeriod(r.Context(), s.store.DB(), nid)
	if err != nil {
		return notFound(err, errNotFound)
	}
	return s.store.Tx(r.Context(), func(q store.Querier) error {
		ctx := r.Context()
		lock := store.LockShare
		nl := store.LockNone
		if write {
			lock, nl = store.LockUpdate, store.LockUpdate
		}
		period, err := s.store.Period(ctx, q, pid, lock)
		if err != nil {
			return err
		}
		n, err := s.store.Note(ctx, q, nid, nl)
		if err != nil {
			return notFound(err, errNotFound)
		}
		if write {
			if _, err := s.lockedAccount(r, q, p); err != nil {
				return err
			}
		}
		if err := s.noteAccess(ctx, q, p, period, &n); err != nil {
			return err
		}
		if write && period.Status == api.PeriodClosed {
			return errPeriodClosed
		}
		return fn(q, period, &n)
	})
}

func (s *Server) noteEvent(r *http.Request, q store.Querier, p *principal, n *store.NoteRow, action, reason string, before, after any) error {
	e := api.NoteEvent{ID: id.New(), Actor: taskPerson(p), Action: action, Reason: reason,
		CreatedAt: s.now().UTC().Truncate(time.Microsecond), Before: before, After: after}
	if err := s.store.AddNoteEvent(r.Context(), q, n.ID, e); err != nil {
		return err
	}
	return s.record(r, q, p, store.AuditEvent{Action: action, TargetType: "note", TargetID: &n.ID, PeriodID: &n.PeriodID,
		ScopeType: "division", ScopeID: &n.OwnerDivisionID, Before: before,
		After: map[string]any{"value": after, "reason": reason}, OccurredAt: e.CreatedAt})
}

// noteSummary is the list form of a note, without the big content, comments
// and attachments.
func noteSummary(n store.NoteRow) api.Note {
	n.Content = nil
	n.Comments = nil
	n.Attachments = nil
	return n.Note
}

func validateNoteTitle(title string) (string, error) {
	return text(title, "judul note", 1, 200)
}

// ownerDivisionFor picks the division that will own a note in a space: the
// caller's division in that space where they hold notes.manage. For a
// division space it is that division; for Medhum it is the caller's own
// division (Humas or Mediatek) with the capability.
func (s *Server) ownerDivisionFor(ctx context.Context, q store.Querier, p *principal, sp store.Space, period api.Period) (id.ID, error) {
	var chosen id.ID
	for _, d := range sp.Divisions {
		rr := &http.Request{}
		rr.SetPathValue("periodId", period.ID.String())
		rr.SetPathValue("divisionId", d.ID.String())
		sc, err := s.loadScope(ctx, q, rr, p, store.LockNone)
		if err != nil {
			return id.Nil, err
		}
		if slices.Contains(sc.caps, api.CapNotesManage) {
			chosen = d.ID
			break
		}
	}
	if chosen == id.Nil {
		return id.Nil, errForbidden
	}
	return chosen, nil
}

func (s *Server) listSpaceNotes(w http.ResponseWriter, r *http.Request, p *principal) error {
	sid, err := pathID(r, "spaceId")
	if err != nil {
		return err
	}
	out := []api.Note{}
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		ctx := r.Context()
		sp, err := s.store.SpaceByID(ctx, q, sid)
		if err != nil {
			return notFound(err, errNotFound)
		}
		period, err := s.store.Period(ctx, q, sp.PeriodID, store.LockShare)
		if err != nil {
			return err
		}
		view, err := s.spaceView(ctx, q, p, sid)
		if err != nil {
			return err
		}
		if !slices.Contains(view.Capabilities, api.CapNotesRead) {
			return errForbidden
		}
		ids, err := s.store.SpaceNoteIDs(ctx, q, sid)
		if err != nil {
			return err
		}
		for _, nid := range ids {
			n, err := s.store.Note(ctx, q, nid, store.LockNone)
			if err != nil {
				return err
			}
			if err := s.noteAccess(ctx, q, p, period, &n); errors.Is(err, errForbidden) {
				continue
			} else if err != nil {
				return err
			}
			out = append(out, noteSummary(n))
		}
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, api.List[api.Note]{Items: out})
	return nil
}

func (s *Server) createNote(w http.ResponseWriter, r *http.Request, p *principal) error {
	if p.admin() {
		return errForbidden
	}
	var req api.NoteCreateRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	title, err := validateNoteTitle(req.Title)
	if err != nil {
		return err
	}
	if req.TemplateID != nil && len(req.Content) > 0 {
		return badRequest("Pilih salah satu: templateId atau content.")
	}
	sid, err := pathID(r, "spaceId")
	if err != nil {
		return err
	}
	var out api.Note
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		ctx, now := r.Context(), s.now().UTC().Truncate(time.Microsecond)
		sp, err := s.store.SpaceByID(ctx, q, sid)
		if err != nil {
			return notFound(err, errNotFound)
		}
		period, err := s.store.Period(ctx, q, sp.PeriodID, store.LockUpdate)
		if err != nil {
			return err
		}
		if period.Status == api.PeriodClosed {
			return errPeriodClosed
		}
		if _, err := s.lockedAccount(r, q, p); err != nil {
			return err
		}
		did, err := s.ownerDivisionFor(ctx, q, p, sp, period)
		if err != nil {
			return err
		}
		content := req.Content
		kind := api.TemplateFree
		switch {
		case req.TemplateID != nil:
			tmpl, tpid, tdid, err := s.store.Template(ctx, q, *req.TemplateID)
			if err != nil {
				return notFound(err, badRequest("Template tidak ditemukan."))
			}
			if tpid != period.ID || *tmpl.SpaceID != sid || tdid != did {
				return badRequest("Template bukan milik ruang/divisi ini.")
			}
			content, kind = tmpl.Content, api.TemplateCustom
		case len(req.Content) > 0:
			content = req.Content
			if req.TemplateKind == api.TemplateMeeting || req.TemplateKind == api.TemplateSchedule {
				kind = req.TemplateKind
			}
		default:
			content = builtinTemplateContent(req.TemplateKind)
			if req.TemplateKind == api.TemplateMeeting || req.TemplateKind == api.TemplateSchedule {
				kind = req.TemplateKind
			}
		}
		if err := api.ValidateNoteContent(content); err != nil {
			return badRequest(err.Error())
		}
		n := store.NoteRow{Note: api.Note{ID: id.New(), SpaceID: sid, PeriodID: period.ID, OwnerDivisionID: did,
			Title: title, Content: content, TemplateKind: kind, Version: 1, CreatedBy: taskPerson(p),
			CreatedAt: now, UpdatedAt: now}}
		if err := s.store.InsertNote(ctx, q, n); err != nil {
			return err
		}
		if err := s.noteAccess(ctx, q, p, period, &n); err != nil {
			return err
		}
		if err := s.noteEvent(r, q, p, &n, "note.create", "", nil, n.Note); err != nil {
			return err
		}
		out = n.Note
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}

func (s *Server) getNote(w http.ResponseWriter, r *http.Request, p *principal) error {
	var out api.Note
	err := s.withNote(r, p, false, func(q store.Querier, _ api.Period, n *store.NoteRow) error {
		ctx := r.Context()
		comments, err := s.store.NoteComments(ctx, q, n.ID)
		if err != nil {
			return err
		}
		attachments, err := s.store.NoteAttachments(ctx, q, n.ID)
		if err != nil {
			return err
		}
		n.Comments = nonNil(comments)
		n.Attachments = nonNil(attachments)
		out = n.Note
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) updateNote(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.NoteWriteRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	title, err := validateNoteTitle(req.Title)
	if err != nil {
		return err
	}
	if err := api.ValidateNoteContent(req.Content); err != nil {
		return badRequest(err.Error())
	}
	var out api.Note
	err = s.withNote(r, p, true, func(q store.Querier, period api.Period, n *store.NoteRow) error {
		ctx, now := r.Context(), s.now().UTC().Truncate(time.Microsecond)
		if !n.Permissions.Manage {
			return errForbidden
		}
		if req.ExpectedVersion < 1 {
			return badRequest("Versi note wajib diisi.")
		}
		if n.Version != req.ExpectedVersion {
			return conflict("Note telah berubah. Muat ulang sebelum menyimpan.")
		}
		// The caller must hold a live editor lock.
		if n.Lock == nil || n.Lock.Editor.ID != p.User.ID || !now.Before(n.Lock.ExpiresAt) {
			return conflict("Anda belum memegang kunci editor note ini, atau kunci sudah kedaluwarsa. Ambil kunci lalu coba lagi.")
		}
		before := n.Note
		n.Title, n.Content = title, req.Content
		n.Version++
		n.UpdatedAt = now
		if err := s.store.SaveNote(ctx, q, *n); err != nil {
			return err
		}
		if err := s.noteEvent(r, q, p, n, "note.update", "", before, n.Note); err != nil {
			return err
		}
		// The caller's own draft is superseded by the save.
		if _, err := s.store.DeleteNoteDraft(ctx, q, n.ID, p.User.ID); err != nil {
			return err
		}
		out = n.Note
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) deleteNote(w http.ResponseWriter, r *http.Request, p *principal) error {
	err := s.withNote(r, p, true, func(q store.Querier, _ api.Period, n *store.NoteRow) error {
		if !n.Permissions.Manage {
			return errForbidden
		}
		before := n.Note
		if err := s.store.DeleteNote(r.Context(), q, n.ID); err != nil {
			return err
		}
		return s.noteEvent(r, q, p, n, "note.delete", "", before, nil)
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) noteHistory(w http.ResponseWriter, r *http.Request, p *principal) error {
	var out []api.NoteEvent
	err := s.withNote(r, p, false, func(q store.Querier, _ api.Period, n *store.NoteRow) error {
		if !n.Permissions.Manage && n.CreatedBy.ID != p.User.ID {
			return errForbidden
		}
		h, err := s.store.NoteHistory(r.Context(), q, n.ID)
		out = h
		return err
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, api.List[api.NoteEvent]{Items: nonNil(out)})
	return nil
}
