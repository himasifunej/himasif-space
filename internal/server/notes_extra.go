package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
)

// --- editor lock (NTE-06, NTE-08) ---

// acquireNoteLock takes or renews the editor lock. A free, expired, or
// own lock is granted; a live lock held by someone else is a conflict.
func (s *Server) acquireNoteLock(w http.ResponseWriter, r *http.Request, p *principal) error {
	var out api.NoteLock
	err := s.withNote(r, p, true, func(q store.Querier, _ api.Period, n *store.NoteRow) error {
		if !n.Permissions.Manage {
			return errForbidden
		}
		now := s.now().UTC().Truncate(time.Microsecond)
		if n.Lock != nil && n.Lock.Editor.ID != p.User.ID && now.Before(n.Lock.ExpiresAt) {
			return lockConflict(*n.Lock)
		}
		acquired := now
		if n.Lock != nil && n.Lock.Editor.ID == p.User.ID {
			acquired = n.Lock.AcquiredAt
		}
		expires := now.Add(noteLockTTL)
		if err := s.store.SetNoteLock(r.Context(), q, n.ID, p.User.ID, acquired, expires); err != nil {
			return err
		}
		out = api.NoteLock{Editor: taskPerson(p), AcquiredAt: acquired, ExpiresAt: expires, HeldByCaller: true}
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// takeoverNoteLock takes over a lock held by someone else: allowed when the
// lock is expired (the editor dropped), or a manager forces it. It never
// discards the previous editor's draft (NTE-08).
func (s *Server) takeoverNoteLock(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.NoteLockRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	var out api.NoteLock
	err := s.withNote(r, p, true, func(q store.Querier, _ api.Period, n *store.NoteRow) error {
		if !n.Permissions.Manage {
			return errForbidden
		}
		now := s.now().UTC().Truncate(time.Microsecond)
		if n.Lock != nil && n.Lock.Editor.ID != p.User.ID && now.Before(n.Lock.ExpiresAt) && !req.Force {
			return lockConflict(*n.Lock)
		}
		before := map[string]any{"editor": nil}
		if n.Lock != nil {
			before["editor"] = n.Lock.Editor.ID
		}
		expires := now.Add(noteLockTTL)
		if err := s.store.SetNoteLock(r.Context(), q, n.ID, p.User.ID, now, expires); err != nil {
			return err
		}
		out = api.NoteLock{Editor: taskPerson(p), AcquiredAt: now, ExpiresAt: expires, HeldByCaller: true}
		return s.noteEvent(r, q, p, n, "note.lock.takeover", "", before, map[string]any{"editor": p.User.ID})
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) releaseNoteLock(w http.ResponseWriter, r *http.Request, p *principal) error {
	err := s.withNote(r, p, true, func(q store.Querier, _ api.Period, n *store.NoteRow) error {
		if _, err := s.store.ReleaseNoteLock(r.Context(), q, n.ID, p.User.ID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// lockConflict returns a 409 whose error carries the current holder.
func lockConflict(l api.NoteLock) error {
	return &apiError{status: http.StatusConflict, code: api.CodeConflict,
		message: "Note ini sedang diedit oleh " + l.Editor.DisplayName + "."}
}

// --- drafts (NTE-08) ---

func (s *Server) saveNoteDraft(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.NoteDraftRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if _, err := text(req.Title, "judul draf", 0, 200); err != nil {
		return err
	}
	if err := api.ValidateNoteContent(req.Content); err != nil {
		return badRequest(err.Error())
	}
	var out api.NoteDraft
	err := s.withNote(r, p, true, func(q store.Querier, _ api.Period, n *store.NoteRow) error {
		if !n.Permissions.Manage {
			return errForbidden
		}
		now := s.now().UTC().Truncate(time.Microsecond)
		d := api.NoteDraft{Title: req.Title, Content: req.Content, BaseVersion: req.BaseVersion, SavedAt: now}
		if err := s.store.SaveNoteDraft(r.Context(), q, n.ID, p.User.ID, d); err != nil {
			return err
		}
		d.Stale = req.BaseVersion < n.Version
		out = d
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) getNoteDraft(w http.ResponseWriter, r *http.Request, p *principal) error {
	var out api.NoteDraft
	var found bool
	err := s.withNote(r, p, false, func(q store.Querier, _ api.Period, n *store.NoteRow) error {
		if !n.Permissions.Manage {
			return errForbidden
		}
		d, err := s.store.NoteDraft(r.Context(), q, n.ID, p.User.ID)
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		d.Stale = d.BaseVersion < n.Version
		out, found = d, true
		return nil
	})
	if err != nil {
		return err
	}
	if !found {
		return errNotFound
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) deleteNoteDraft(w http.ResponseWriter, r *http.Request, p *principal) error {
	err := s.withNote(r, p, true, func(q store.Querier, _ api.Period, n *store.NoteRow) error {
		_, err := s.store.DeleteNoteDraft(r.Context(), q, n.ID, p.User.ID)
		return err
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// --- comments (NTE-06) ---

func (s *Server) addNoteComment(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.NoteCommentRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	body, err := text(req.Body, "komentar", 1, 10000)
	if err != nil {
		return err
	}
	var out api.NoteComment
	err = s.withNote(r, p, true, func(q store.Querier, _ api.Period, n *store.NoteRow) error {
		if !n.Permissions.Comment {
			return errForbidden
		}
		out = api.NoteComment{ID: id.New(), Author: taskPerson(p), Body: body, CreatedAt: s.now().UTC().Truncate(time.Microsecond)}
		if err := s.store.AddNoteComment(r.Context(), q, n.ID, out); err != nil {
			return err
		}
		return s.noteEvent(r, q, p, n, "note.comment", "", nil, out)
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}

// --- attachments (NTE-09) ---

func (s *Server) addNoteAttachment(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.NoteAttachmentRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	name, err := text(req.Name, "nama lampiran", 1, 300)
	if err != nil {
		return err
	}
	ct, err := text(req.ContentType, "tipe konten", 0, 200)
	if err != nil {
		return err
	}
	if req.SizeBytes < 0 {
		return badRequest("Ukuran lampiran tidak valid.")
	}
	link := strings.TrimSpace(req.URL)
	status := api.AttachmentPending
	if link != "" {
		if u, err := url.Parse(link); err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return badRequest("Tautan lampiran harus http:// atau https://.")
		}
		status = api.AttachmentReady
	}
	var out api.NoteAttachment
	err = s.withNote(r, p, true, func(q store.Querier, _ api.Period, n *store.NoteRow) error {
		if !n.Permissions.Manage {
			return errForbidden
		}
		out = api.NoteAttachment{ID: id.New(), Name: name, URL: link, Status: status, ContentType: ct,
			SizeBytes: req.SizeBytes, AddedBy: taskPerson(p), CreatedAt: s.now().UTC().Truncate(time.Microsecond)}
		if err := s.store.AddNoteAttachment(r.Context(), q, n.ID, out); err != nil {
			return err
		}
		return s.noteEvent(r, q, p, n, "note.attachment.add", "", nil, out)
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}

func (s *Server) deleteNoteAttachment(w http.ResponseWriter, r *http.Request, p *principal) error {
	aid, err := pathID(r, "attachmentId")
	if err != nil {
		return err
	}
	err = s.withNote(r, p, true, func(q store.Querier, _ api.Period, n *store.NoteRow) error {
		if !n.Permissions.Manage {
			return errForbidden
		}
		removed, err := s.store.DeleteNoteAttachment(r.Context(), q, n.ID, aid)
		if err != nil {
			return err
		}
		if !removed {
			return errNotFound
		}
		return s.noteEvent(r, q, p, n, "note.attachment.remove", "", map[string]any{"attachmentId": aid}, nil)
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// --- links to tasks/reminders (NTE-05, NTE-07) ---

func (s *Server) listNoteLinks(w http.ResponseWriter, r *http.Request, p *principal) error {
	var out []api.NoteLink
	err := s.withNote(r, p, false, func(q store.Querier, _ api.Period, n *store.NoteRow) error {
		links, err := s.store.NoteLinks(r.Context(), q, n.ID)
		out = links
		return err
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, api.List[api.NoteLink]{Items: nonNil(out)})
	return nil
}

// addNoteLink records that a note gave rise to a task or reminder. It is a
// reference only; it does not open the note's content (NTE-07). The caller
// must manage the note and own/authorize the target.
func (s *Server) addNoteLink(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.NoteLinkRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if req.TargetType != api.NoteLinkTask && req.TargetType != api.NoteLinkReminder {
		return badRequest("targetType harus task atau reminder.")
	}
	if req.TargetID == id.Nil {
		return badRequest("Pilih objek tujuan.")
	}
	var out api.NoteLink
	err := s.withNote(r, p, true, func(q store.Querier, _ api.Period, n *store.NoteRow) error {
		if !n.Permissions.Manage {
			return errForbidden
		}
		ctx := r.Context()
		switch req.TargetType {
		case api.NoteLinkTask:
			ok, err := s.store.TaskExists(ctx, q, req.TargetID)
			if err != nil {
				return err
			}
			if !ok {
				return badRequest("Tugas tujuan tidak ditemukan.")
			}
		case api.NoteLinkReminder:
			// A personal reminder links only from its owner, so a note never
			// exposes someone else's private reminder (PRI-08, NTE-07).
			owned, err := s.store.ReminderOwnedBy(ctx, q, req.TargetID, p.User.ID)
			if err != nil {
				return err
			}
			if !owned {
				return badRequest("Reminder tujuan tidak ditemukan atau bukan milik Anda.")
			}
		}
		out = api.NoteLink{ID: id.New(), TargetType: req.TargetType, TargetID: req.TargetID,
			CreatedBy: taskPerson(p), CreatedAt: s.now().UTC().Truncate(time.Microsecond)}
		if err := s.store.AddNoteLink(ctx, q, n.ID, out); errors.Is(err, store.ErrConflict) {
			return conflict("Tautan ke objek ini sudah ada.")
		} else if err != nil {
			return err
		}
		return s.noteEvent(r, q, p, n, "note.link.add", "", nil, out)
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}

func (s *Server) deleteNoteLink(w http.ResponseWriter, r *http.Request, p *principal) error {
	lid, err := pathID(r, "linkId")
	if err != nil {
		return err
	}
	err = s.withNote(r, p, true, func(q store.Querier, _ api.Period, n *store.NoteRow) error {
		if !n.Permissions.Manage {
			return errForbidden
		}
		removed, err := s.store.DeleteNoteLink(r.Context(), q, n.ID, lid)
		if err != nil {
			return err
		}
		if !removed {
			return errNotFound
		}
		return s.noteEvent(r, q, p, n, "note.link.remove", "", map[string]any{"linkId": lid}, nil)
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// --- templates (NTE-02, NTE-09) ---

// listNoteTemplates returns the built-in templates plus the saved templates
// of the space's divisions the caller may read.
func (s *Server) listNoteTemplates(w http.ResponseWriter, r *http.Request, p *principal) error {
	sid, err := pathID(r, "spaceId")
	if err != nil {
		return err
	}
	out := builtinTemplates()
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		view, err := s.spaceView(r.Context(), q, p, sid)
		if err != nil {
			return err
		}
		if !slices.Contains(view.Capabilities, api.CapNotesRead) {
			return errForbidden
		}
		saved, err := s.store.SpaceTemplates(r.Context(), q, sid)
		if err != nil {
			return err
		}
		out = append(out, saved...)
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, api.List[api.NoteTemplate]{Items: out})
	return nil
}

func (s *Server) createNoteTemplate(w http.ResponseWriter, r *http.Request, p *principal) error {
	if p.admin() {
		return errForbidden
	}
	var req api.NoteTemplateRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	name, err := text(req.Name, "nama template", 1, 120)
	if err != nil {
		return err
	}
	if err := api.ValidateNoteContent(req.Content); err != nil {
		return badRequest(err.Error())
	}
	sid, err := pathID(r, "spaceId")
	if err != nil {
		return err
	}
	var out api.NoteTemplate
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		ctx, now := r.Context(), s.now().UTC().Truncate(time.Microsecond)
		sp, err := s.store.SpaceByID(ctx, q, sid)
		if err != nil {
			return notFound(err, errNotFound)
		}
		period, err := s.store.Period(ctx, q, sp.PeriodID, store.LockShare)
		if err != nil {
			return err
		}
		if period.Status == api.PeriodClosed {
			return errPeriodClosed
		}
		did, err := s.ownerDivisionFor(ctx, q, p, sp, period)
		if err != nil {
			return err
		}
		out = api.NoteTemplate{ID: id.New(), SpaceID: &sid, Kind: api.TemplateCustom, Name: name,
			Builtin: false, Content: req.Content, CreatedAt: &now, UpdatedAt: &now}
		if err := s.store.InsertTemplate(ctx, q, out, period.ID, did, p.User.ID, now); err != nil {
			return err
		}
		return s.record(r, q, p, store.AuditEvent{Action: "note.template.create", TargetType: "note_template",
			TargetID: &out.ID, PeriodID: &period.ID, ScopeType: "division", ScopeID: &did, After: out})
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}

func (s *Server) updateNoteTemplate(w http.ResponseWriter, r *http.Request, p *principal) error {
	if p.admin() {
		return errForbidden
	}
	var req api.NoteTemplateRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	name, err := text(req.Name, "nama template", 1, 120)
	if err != nil {
		return err
	}
	if err := api.ValidateNoteContent(req.Content); err != nil {
		return badRequest(err.Error())
	}
	tid, err := pathID(r, "templateId")
	if err != nil {
		return err
	}
	var out api.NoteTemplate
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		ctx, now := r.Context(), s.now().UTC().Truncate(time.Microsecond)
		tmpl, pid, did, err := s.store.Template(ctx, q, tid)
		if err != nil {
			return notFound(err, errNotFound)
		}
		period, err := s.store.Period(ctx, q, pid, store.LockShare)
		if err != nil {
			return err
		}
		if period.Status == api.PeriodClosed {
			return errPeriodClosed
		}
		if err := s.requireDivision(ctx, q, p, period, did, api.CapNotesManage); err != nil {
			return err
		}
		before := tmpl
		if err := s.store.UpdateTemplate(ctx, q, tid, name, req.Content, now); err != nil {
			return err
		}
		tmpl.Name, tmpl.Content, tmpl.UpdatedAt = name, req.Content, &now
		out = tmpl
		return s.record(r, q, p, store.AuditEvent{Action: "note.template.update", TargetType: "note_template",
			TargetID: &tid, PeriodID: &pid, ScopeType: "division", ScopeID: &did, Before: before, After: out})
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) deleteNoteTemplate(w http.ResponseWriter, r *http.Request, p *principal) error {
	if p.admin() {
		return errForbidden
	}
	tid, err := pathID(r, "templateId")
	if err != nil {
		return err
	}
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		ctx := r.Context()
		tmpl, pid, did, err := s.store.Template(ctx, q, tid)
		if err != nil {
			return notFound(err, errNotFound)
		}
		period, err := s.store.Period(ctx, q, pid, store.LockShare)
		if err != nil {
			return err
		}
		if period.Status == api.PeriodClosed {
			return errPeriodClosed
		}
		if err := s.requireDivision(ctx, q, p, period, did, api.CapNotesManage); err != nil {
			return err
		}
		if err := s.store.DeleteTemplate(ctx, q, tid); err != nil {
			return err
		}
		return s.record(r, q, p, store.AuditEvent{Action: "note.template.delete", TargetType: "note_template",
			TargetID: &tid, PeriodID: &pid, ScopeType: "division", ScopeID: &did, Before: tmpl})
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// requireDivision fails unless the caller holds capability c on division did
// of the period (open-period write check included).
func (s *Server) requireDivision(ctx context.Context, q store.Querier, p *principal, period api.Period, did id.ID, c string) error {
	rr := &http.Request{}
	rr.SetPathValue("periodId", period.ID.String())
	rr.SetPathValue("divisionId", did.String())
	sc, err := s.loadScope(ctx, q, rr, p, store.LockNone)
	if err != nil {
		return err
	}
	return sc.require(c)
}

// --- built-in templates (NTE-03, NTE-04) ---

func heading(level int, text string) api.NoteBlock {
	return api.NoteBlock{Type: api.BlockHeading, Level: level, Text: text}
}
func emptyPara() api.NoteBlock { return api.NoteBlock{Type: api.BlockParagraph, Spans: []api.Span{}} }

func builtinTemplateContent(kind string) []api.NoteBlock {
	for _, t := range builtinTemplates() {
		if t.Kind == kind {
			return t.Content
		}
	}
	return []api.NoteBlock{}
}

// builtinTemplates returns the three built-in templates. They are not rows:
// the server supplies them so every space has them (NTE-02).
func builtinTemplates() []api.NoteTemplate {
	meeting := []api.NoteBlock{
		heading(1, "Notulensi Rapat"),
		heading(2, "Agenda"), emptyPara(),
		heading(2, "Pembahasan"), emptyPara(),
		heading(2, "Keputusan"), emptyPara(),
		heading(2, "Tindak lanjut"),
		{Type: api.BlockChecklist, Items: []api.NoteListItem{{Spans: []api.Span{}, Checked: false}}},
	}
	schedule := []api.NoteBlock{
		heading(1, "Tabel Jadwal"),
		{Type: api.BlockTable, Columns: []string{"Tanggal", "Jam", "Kegiatan", "PJ", "Keterangan"},
			Rows: [][][]api.Span{{{}, {}, {}, {}, {}}}},
	}
	free := []api.NoteBlock{emptyPara()}
	return []api.NoteTemplate{
		{ID: id.Nil, SpaceID: nil, Kind: api.TemplateMeeting, Name: "Rapat / Notulensi", Builtin: true, Content: meeting},
		{ID: id.Nil, SpaceID: nil, Kind: api.TemplateSchedule, Name: "Tabel Jadwal", Builtin: true, Content: schedule},
		{ID: id.Nil, SpaceID: nil, Kind: api.TemplateFree, Name: "Catatan Bebas", Builtin: true, Content: free},
	}
}
