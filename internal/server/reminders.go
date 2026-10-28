package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// Personal reminders (PRD §8). They belong to one account and are private
// by default. There is no division or space scope here: authorization is
// ownership. Only the owner may create, change, complete, share or delete a
// reminder; a recipient may only read it (PRI-07). Space, archive and
// directory endpoints never read these, so a private reminder cannot leak
// through them (PRI-08).

// reminderAPI shapes a reminder for the caller. The owner sees the sharing
// toggle and, when recipients is given, the recipient list; a recipient
// sees neither (PRI-05).
func reminderAPI(r store.Reminder, caller id.ID, recipients []api.ReminderShare) api.Reminder {
	out := api.Reminder{
		ID: r.ID, OwnerID: r.OwnerID, Title: r.Title, Notes: r.Notes, DueKind: r.DueKind,
		DueOn: r.DueOn, DueAt: utcPtr(r.DueAt), TimeZone: r.TimeZone,
		Completed: r.CompletedAt != nil, CompletedAt: utcPtr(r.CompletedAt),
		CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
	}
	if r.OwnerID == caller {
		out.Relation = api.ReminderOwner
		out.Shared = r.Shared
		out.Recipients = recipients
	} else {
		out.Relation = api.ReminderRecipient
	}
	return out
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// reminderContent validates a create/replace body into a Reminder's
// content fields: title, notes, due kind and time, and time zone. It never
// gives a date-only reminder a made-up hour (MED-04).
func reminderContent(req api.ReminderRequest) (store.Reminder, error) {
	var r store.Reminder
	title, err := text(req.Title, "judul reminder", 1, 200)
	if err != nil {
		return r, err
	}
	notes, err := text(req.Notes, "catatan reminder", 0, 4000)
	if err != nil {
		return r, err
	}
	zone := req.TimeZone
	if zone == "" {
		zone = timeutil.DefaultZoneName
	}
	if _, err := timeutil.LoadZone(zone); err != nil {
		return r, badRequest("Zona waktu tidak dikenal.")
	}
	switch req.DueKind {
	case api.DueDate:
		if req.DueOn == nil || req.DueOn.IsZero() {
			return r, badRequest("Isi tanggal reminder (dueOn) untuk dueKind \"date\".")
		}
		if req.DueAt != nil {
			return r, badRequest("Reminder tanggal tidak memiliki jam; kosongkan dueAt.")
		}
		r.DueOn = req.DueOn
	case api.DueInstant:
		if req.DueAt == nil {
			return r, badRequest("Isi waktu reminder (dueAt) untuk dueKind \"instant\".")
		}
		if req.DueOn != nil {
			return r, badRequest("Reminder berwaktu tidak memakai dueOn.")
		}
		at := req.DueAt.UTC()
		r.DueAt = &at
	default:
		return r, badRequest("dueKind harus \"date\" atau \"instant\".")
	}
	r.Title, r.Notes, r.DueKind, r.TimeZone = title, notes, req.DueKind, zone
	return r, nil
}

// person requires a personal account: the shared organization admin account
// keeps no personal reminders (PRD §4.1, ACC-12).
func (s *Server) person(p *principal) error {
	if p.admin() {
		return fail(http.StatusForbidden, api.CodeForbidden, "Reminder pribadi hanya untuk akun pengurus, bukan akun admin organisasi.")
	}
	return nil
}

// recordReminder appends an audit event of a reminder change, in the
// change's transaction, so the owner's history and the change commit
// together (PRI-05, AC-09).
func (s *Server) recordReminder(r *http.Request, q store.Querier, p *principal, action string, rid id.ID, before, after any) error {
	return s.record(r, q, p, store.AuditEvent{Action: action, TargetType: "reminder", TargetID: &rid, Before: before, After: after})
}

func reminderState(r store.Reminder) map[string]any {
	m := map[string]any{
		"title": r.Title, "notes": r.Notes, "dueKind": r.DueKind, "timeZone": r.TimeZone,
		"shared": r.Shared, "completed": r.CompletedAt != nil,
	}
	if r.DueOn != nil {
		m["dueOn"] = r.DueOn.String()
	}
	if r.DueAt != nil {
		m["dueAt"] = r.DueAt.UTC()
	}
	return m
}

// listReminders lists the caller's own reminders and the reminders shared
// with them, each tagged with the caller's relation.
func (s *Server) listReminders(w http.ResponseWriter, r *http.Request, p *principal) error {
	if err := s.person(p); err != nil {
		return err
	}
	ctx, q := r.Context(), s.store.DB()
	visible, err := s.store.VisibleReminders(ctx, q, p.User.ID)
	if err != nil {
		return err
	}
	out := []api.Reminder{}
	for _, rm := range visible {
		var recipients []api.ReminderShare
		if rm.OwnerID == p.User.ID && rm.Shared {
			recipients, err = s.store.ReminderRecipients(ctx, q, rm.ID)
			if err != nil {
				return err
			}
		}
		out = append(out, reminderAPI(rm, p.User.ID, recipients))
	}
	writeJSON(w, http.StatusOK, api.List[api.Reminder]{Items: out})
	return nil
}

// getReminder returns one reminder the caller may see: their own, or one
// currently shared with them. Anything else is 404, so a private reminder
// cannot be told apart from one that does not exist (PRI-08).
func (s *Server) getReminder(w http.ResponseWriter, r *http.Request, p *principal) error {
	if err := s.person(p); err != nil {
		return err
	}
	rid, err := pathID(r, "reminderId")
	if err != nil {
		return err
	}
	ctx, q := r.Context(), s.store.DB()
	rm, err := s.store.ReminderVisibleTo(ctx, q, rid, p.User.ID)
	if err != nil {
		return notFound(err, errNotFound)
	}
	var recipients []api.ReminderShare
	if rm.OwnerID == p.User.ID && rm.Shared {
		if recipients, err = s.store.ReminderRecipients(ctx, q, rm.ID); err != nil {
			return err
		}
	}
	writeJSON(w, http.StatusOK, reminderAPI(rm, p.User.ID, recipients))
	return nil
}

// createReminder makes a reminder owned by the caller. It starts private
// unless the caller's default is to share; that default is read now and
// never changes an existing reminder (PRI-02, PRI-05).
func (s *Server) createReminder(w http.ResponseWriter, r *http.Request, p *principal) error {
	if err := s.person(p); err != nil {
		return err
	}
	var req api.ReminderRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	content, err := reminderContent(req)
	if err != nil {
		return err
	}
	var out api.Reminder
	err = s.reminderWrite(r, p, id.Nil, func(q store.Querier) error {
		ctx, now := r.Context(), s.now()
		def, err := s.store.ReminderDefaultShare(ctx, q, p.User.ID)
		if err != nil {
			return err
		}
		content.ID = id.New()
		content.OwnerID = p.User.ID
		content.Shared = def
		content.CreatedAt = now.UTC()
		content.UpdatedAt = now.UTC()
		if err := s.store.CreateReminder(ctx, q, content); err != nil {
			return err
		}
		if err := s.recordReminder(r, q, p, "reminder.create", content.ID, nil, reminderState(content)); err != nil {
			return err
		}
		out = reminderAPI(content, p.User.ID, []api.ReminderShare{})
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}

// updateReminder replaces a reminder's content. Only the owner may; a
// recipient's read-only access grants nothing here (PRI-07).
func (s *Server) updateReminder(w http.ResponseWriter, r *http.Request, p *principal) error {
	if err := s.person(p); err != nil {
		return err
	}
	rid, err := pathID(r, "reminderId")
	if err != nil {
		return err
	}
	var req api.ReminderRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	content, err := reminderContent(req)
	if err != nil {
		return err
	}
	var out api.Reminder
	err = s.reminderWrite(r, p, id.Nil, func(q store.Querier) error {
		ctx, now := r.Context(), s.now()
		rm, err := s.ownedReminder(ctx, q, rid, p)
		if err != nil {
			return err
		}
		before := reminderState(rm)
		rm.Title, rm.Notes, rm.DueKind = content.Title, content.Notes, content.DueKind
		rm.DueOn, rm.DueAt, rm.TimeZone = content.DueOn, content.DueAt, content.TimeZone
		if err := s.store.UpdateReminderContent(ctx, q, rm, now); err != nil {
			return err
		}
		rm.UpdatedAt = now.UTC()
		if err := s.recordReminder(r, q, p, "reminder.update", rid, before, reminderState(rm)); err != nil {
			return err
		}
		recipients, err := s.recipientsIfShared(ctx, q, rm)
		if err != nil {
			return err
		}
		out = reminderAPI(rm, p.User.ID, recipients)
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// completeReminder marks the owner's reminder done or not done. A recipient
// may not finish it (PRI-07).
func (s *Server) completeReminder(w http.ResponseWriter, r *http.Request, p *principal) error {
	if err := s.person(p); err != nil {
		return err
	}
	rid, err := pathID(r, "reminderId")
	if err != nil {
		return err
	}
	var req api.ReminderCompleteRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	var out api.Reminder
	err = s.reminderWrite(r, p, id.Nil, func(q store.Querier) error {
		ctx, now := r.Context(), s.now()
		rm, err := s.ownedReminder(ctx, q, rid, p)
		if err != nil {
			return err
		}
		if (rm.CompletedAt != nil) == req.Completed {
			recipients, err := s.recipientsIfShared(ctx, q, rm)
			if err != nil {
				return err
			}
			out = reminderAPI(rm, p.User.ID, recipients)
			return nil
		}
		before := reminderState(rm)
		if err := s.store.SetReminderCompleted(ctx, q, rid, req.Completed, now); err != nil {
			return err
		}
		if req.Completed {
			at := now.UTC()
			rm.CompletedAt = &at
		} else {
			rm.CompletedAt = nil
		}
		rm.UpdatedAt = now.UTC()
		action := "reminder.reopen"
		if req.Completed {
			action = "reminder.complete"
		}
		if err := s.recordReminder(r, q, p, action, rid, before, reminderState(rm)); err != nil {
			return err
		}
		recipients, err := s.recipientsIfShared(ctx, q, rm)
		if err != nil {
			return err
		}
		out = reminderAPI(rm, p.User.ID, recipients)
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// deleteReminder removes the owner's reminder and every share of it.
func (s *Server) deleteReminder(w http.ResponseWriter, r *http.Request, p *principal) error {
	if err := s.person(p); err != nil {
		return err
	}
	rid, err := pathID(r, "reminderId")
	if err != nil {
		return err
	}
	err = s.reminderWrite(r, p, id.Nil, func(q store.Querier) error {
		ctx := r.Context()
		rm, err := s.ownedReminder(ctx, q, rid, p)
		if err != nil {
			return err
		}
		if err := s.store.DeleteReminder(ctx, q, rid); err != nil {
			return err
		}
		return s.recordReminder(r, q, p, "reminder.delete", rid, reminderState(rm), nil)
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// setReminderSharing turns a reminder's sharing toggle on or off, at any
// time, without naming recipients (PRI-02). The recipient list is kept.
func (s *Server) setReminderSharing(w http.ResponseWriter, r *http.Request, p *principal) error {
	if err := s.person(p); err != nil {
		return err
	}
	rid, err := pathID(r, "reminderId")
	if err != nil {
		return err
	}
	var req api.ReminderSharingRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	var out api.Reminder
	err = s.reminderWrite(r, p, id.Nil, func(q store.Querier) error {
		ctx, now := r.Context(), s.now()
		rm, err := s.ownedReminder(ctx, q, rid, p)
		if err != nil {
			return err
		}
		if rm.Shared != req.Shared {
			before := reminderState(rm)
			if err := s.store.SetReminderShared(ctx, q, rid, req.Shared, now); err != nil {
				return err
			}
			rm.Shared = req.Shared
			rm.UpdatedAt = now.UTC()
			if err := s.recordReminder(r, q, p, "reminder.sharing", rid, before, reminderState(rm)); err != nil {
				return err
			}
		}
		recipients, err := s.recipientsIfShared(ctx, q, rm)
		if err != nil {
			return err
		}
		out = reminderAPI(rm, p.User.ID, recipients)
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// listReminderRecipients returns who the owner has shared a reminder with
// (PRI-05). Only the owner may read it.
func (s *Server) listReminderRecipients(w http.ResponseWriter, r *http.Request, p *principal) error {
	if err := s.person(p); err != nil {
		return err
	}
	rid, err := pathID(r, "reminderId")
	if err != nil {
		return err
	}
	ctx, q := r.Context(), s.store.DB()
	if _, err := s.ownedReminder(ctx, q, rid, p); err != nil {
		return err
	}
	recipients, err := s.store.ReminderRecipients(ctx, q, rid)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, api.List[api.ReminderShare]{Items: nonNil(recipients)})
	return nil
}

// shareReminder adds a read-only recipient, chosen as an account, not a
// position (PRI-03). The recipient is a personal account, not the owner,
// and not the organization admin.
func (s *Server) shareReminder(w http.ResponseWriter, r *http.Request, p *principal) error {
	if err := s.person(p); err != nil {
		return err
	}
	rid, err := pathID(r, "reminderId")
	if err != nil {
		return err
	}
	var req api.ReminderShareRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if req.RecipientID == id.Nil {
		return badRequest("Pilih penerima reminder.")
	}
	if req.RecipientID == p.User.ID {
		return badRequest("Reminder tidak dapat dibagikan kepada diri sendiri.")
	}
	var out api.Reminder
	err = s.reminderWrite(r, p, req.RecipientID, func(q store.Querier) error {
		ctx, now := r.Context(), s.now()
		rm, err := s.ownedReminder(ctx, q, rid, p)
		if err != nil {
			return err
		}
		u, err := s.store.UserForUpdate(ctx, q, req.RecipientID)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
		if err != nil || u.Kind != api.AccountPerson || u.DisabledAt != nil {
			return badRequest("Penerima harus akun pengurus yang aktif.")
		}
		if err := s.store.AddReminderShare(ctx, q, rid, req.RecipientID, p.User.ID, now); errors.Is(err, store.ErrConflict) {
			return conflict("Reminder ini sudah dibagikan kepada akun tersebut.")
		} else if err != nil {
			return err
		}
		rm.UpdatedAt = now.UTC()
		if err := s.store.TouchReminder(ctx, q, rid, now); err != nil {
			return err
		}
		if err := s.recordReminder(r, q, p, "reminder.share.add", rid, nil,
			map[string]any{"recipientId": req.RecipientID}); err != nil {
			return err
		}
		recipients, err := s.store.ReminderRecipients(ctx, q, rid)
		if err != nil {
			return err
		}
		out = reminderAPI(rm, p.User.ID, recipients)
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}

// unshareReminder revokes a recipient's access (PRI-05).
func (s *Server) unshareReminder(w http.ResponseWriter, r *http.Request, p *principal) error {
	if err := s.person(p); err != nil {
		return err
	}
	rid, err := pathID(r, "reminderId")
	if err != nil {
		return err
	}
	recipient, err := pathID(r, "recipientId")
	if err != nil {
		return err
	}
	err = s.reminderWrite(r, p, id.Nil, func(q store.Querier) error {
		ctx, now := r.Context(), s.now()
		if _, err := s.ownedReminder(ctx, q, rid, p); err != nil {
			return err
		}
		removed, err := s.store.RemoveReminderShare(ctx, q, rid, recipient)
		if err != nil {
			return err
		}
		if !removed {
			return errNotFound
		}
		if err := s.store.TouchReminder(ctx, q, rid, now); err != nil {
			return err
		}
		return s.recordReminder(r, q, p, "reminder.share.remove", rid,
			map[string]any{"recipientId": recipient}, nil)
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// reminderHistory returns the recorded changes to a reminder, for its owner
// (AC-09). A recipient may not read the history.
func (s *Server) reminderHistory(w http.ResponseWriter, r *http.Request, p *principal) error {
	if err := s.person(p); err != nil {
		return err
	}
	rid, err := pathID(r, "reminderId")
	if err != nil {
		return err
	}
	ctx, q := r.Context(), s.store.DB()
	if _, err := s.ownedReminder(ctx, q, rid, p); err != nil {
		return err
	}
	entries, err := s.store.ReminderAudit(ctx, q, rid)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, api.List[api.ReminderAuditEntry]{Items: nonNil(entries)})
	return nil
}

// getReminderPreferences returns the caller's default for new reminders.
func (s *Server) getReminderPreferences(w http.ResponseWriter, r *http.Request, p *principal) error {
	if err := s.person(p); err != nil {
		return err
	}
	def, err := s.store.ReminderDefaultShare(r.Context(), s.store.DB(), p.User.ID)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, api.ReminderPreferences{DefaultShare: def})
	return nil
}

// putReminderPreferences sets the caller's default for new reminders. It
// never reopens an existing reminder (PRI-05).
func (s *Server) putReminderPreferences(w http.ResponseWriter, r *http.Request, p *principal) error {
	if err := s.person(p); err != nil {
		return err
	}
	var req api.ReminderPreferences
	if err := readJSON(r, &req); err != nil {
		return err
	}
	err := s.reminderWrite(r, p, id.Nil, func(q store.Querier) error {
		ctx, now := r.Context(), s.now()
		before, err := s.store.ReminderDefaultShare(ctx, q, p.User.ID)
		if err != nil {
			return err
		}
		if before == req.DefaultShare {
			return nil
		}
		if err := s.store.SetReminderDefaultShare(ctx, q, p.User.ID, req.DefaultShare, now); err != nil {
			return err
		}
		return s.record(r, q, p, store.AuditEvent{Action: "reminder.preferences.set", TargetType: "user", TargetID: &p.User.ID,
			Before: map[string]any{"defaultShare": before}, After: map[string]any{"defaultShare": req.DefaultShare}})
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, api.ReminderPreferences{DefaultShare: req.DefaultShare})
	return nil
}

// reminderWrite binds a queued mutation to its original live session. Account
// locks are globally ordered before reminder rows: reciprocal sharing must not
// acquire the peer first and then deadlock on the owner's foreign-key lock.
func (s *Server) reminderWrite(r *http.Request, p *principal, recipient id.ID, fn func(store.Querier) error) error {
	return s.store.Tx(r.Context(), func(q store.Querier) error {
		accounts := []id.ID{p.User.ID}
		if recipient != id.Nil && recipient != p.User.ID {
			accounts = append(accounts, recipient)
		}
		slices.SortFunc(accounts, func(a, b id.ID) int { return bytes.Compare(a[:], b[:]) })
		for _, account := range accounts {
			if account == p.User.ID {
				if _, err := s.lockedAccount(r, q, p); err != nil {
					return err
				}
			} else if _, err := s.store.UserForUpdate(r.Context(), q, account); err != nil {
				if errors.Is(err, store.ErrNotFound) {
					return badRequest("Penerima harus akun pengurus yang aktif.")
				}
				return err
			}
		}
		return fn(q)
	})
}

// ownedReminder loads a reminder locked for update and fails unless the
// caller owns it. A recipient's read-only access never reaches a write
// (PRI-07); a reminder the caller cannot see is 404 (PRI-08).
func (s *Server) ownedReminder(ctx context.Context, q store.Querier, rid id.ID, p *principal) (store.Reminder, error) {
	rm, err := s.store.Reminder(ctx, q, rid, store.LockUpdate)
	if err != nil {
		return store.Reminder{}, notFound(err, errNotFound)
	}
	if rm.OwnerID != p.User.ID {
		// Do not reveal a reminder the caller does not own. If it is shared
		// with them it is theirs to read, not to change; otherwise it is as
		// good as absent.
		if rm.Shared {
			if shared, err := s.sharedWith(ctx, q, rid, p.User.ID); err != nil {
				return store.Reminder{}, err
			} else if shared {
				return store.Reminder{}, errForbidden
			}
		}
		return store.Reminder{}, errNotFound
	}
	return rm, nil
}

func (s *Server) sharedWith(ctx context.Context, q store.Querier, rid, user id.ID) (bool, error) {
	if _, err := s.store.ReminderVisibleTo(ctx, q, rid, user); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// recipientsIfShared returns the recipient list when the reminder is shared,
// an empty list otherwise, for shaping the owner's view.
func (s *Server) recipientsIfShared(ctx context.Context, q store.Querier, rm store.Reminder) ([]api.ReminderShare, error) {
	if !rm.Shared {
		return []api.ReminderShare{}, nil
	}
	return s.store.ReminderRecipients(ctx, q, rm.ID)
}
