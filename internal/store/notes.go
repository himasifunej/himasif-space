package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// NoteRow is a note with its server-side period and division. The embedded
// api.Note carries what clients see; Content is the block document.
type NoteRow struct {
	api.Note
}

func marshalBlocks(blocks []api.NoteBlock) ([]byte, error) {
	if blocks == nil {
		blocks = []api.NoteBlock{}
	}
	return json.Marshal(blocks)
}

func unmarshalBlocks(b []byte) ([]api.NoteBlock, error) {
	out := []api.NoteBlock{}
	if len(b) == 0 {
		return out, nil
	}
	err := json.Unmarshal(b, &out)
	return out, err
}

// NotePeriod is a nonlocking peek at a note's period; callers lock the
// period before Note.
func (s *Store) NotePeriod(ctx context.Context, q Querier, nid id.ID) (id.ID, error) {
	var pid id.ID
	err := q.QueryRow(ctx, `select period_id from notes where id = $1`, nid).Scan(&pid)
	if errors.Is(err, pgx.ErrNoRows) {
		return pid, ErrNotFound
	}
	return pid, err
}

const noteColumns = `n.id, n.space_id, n.period_id, n.owner_division_id, n.title, n.content, n.template_kind, n.meeting_id, n.version, n.created_by, u.display_name, n.created_at, n.updated_at`

func scanNote(row pgx.Row) (NoteRow, error) {
	var n NoteRow
	var content []byte
	err := row.Scan(&n.ID, &n.SpaceID, &n.PeriodID, &n.OwnerDivisionID, &n.Title, &content, &n.TemplateKind,
		&n.MeetingID, &n.Version, &n.CreatedBy.ID, &n.CreatedBy.DisplayName, &n.CreatedAt, &n.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return n, ErrNotFound
	}
	if err != nil {
		return n, err
	}
	n.Content, err = unmarshalBlocks(content)
	return n, err
}

// Note loads a note, locked as lock says (on the note row only).
func (s *Store) Note(ctx context.Context, q Querier, nid id.ID, lock string) (NoteRow, error) {
	return scanNote(q.QueryRow(ctx, `select `+noteColumns+` from notes n join users u on u.id = n.created_by where n.id = $1`+noteLock(lock), nid))
}

func noteLock(lock string) string {
	switch lock {
	case LockUpdate:
		return " for update of n"
	case LockShare:
		return " for share of n"
	}
	return ""
}

// SpaceNoteIDs returns the IDs of a space's notes, most recently updated
// first.
func (s *Store) SpaceNoteIDs(ctx context.Context, q Querier, sid id.ID) ([]id.ID, error) {
	rows, err := q.Query(ctx, `select id from notes where space_id = $1 order by updated_at desc, id`, sid)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[id.ID])
}

// InsertNote inserts a note.
func (s *Store) InsertNote(ctx context.Context, q Querier, n NoteRow) error {
	content, err := marshalBlocks(n.Content)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `insert into notes
		(id, space_id, period_id, owner_division_id, title, content, template_kind, meeting_id, version, created_by, created_at, updated_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		n.ID, n.SpaceID, n.PeriodID, n.OwnerDivisionID, n.Title, content, n.TemplateKind, n.MeetingID, n.Version, n.CreatedBy.ID, n.CreatedAt, n.UpdatedAt)
	return err
}

// SaveNote updates a note's title, content and version. The caller holds
// the note write lock and has checked the version.
func (s *Store) SaveNote(ctx context.Context, q Querier, n NoteRow) error {
	content, err := marshalBlocks(n.Content)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `update notes set title = $2, content = $3, version = $4, updated_at = $5 where id = $1`,
		n.ID, n.Title, content, n.Version, n.UpdatedAt)
	return err
}

// DeleteNote removes a note and its children (they cascade), and the
// append-only events, which have no cascade, are left for history.
func (s *Store) DeleteNote(ctx context.Context, q Querier, nid id.ID) error {
	// Children with ON DELETE CASCADE: locks, drafts, comments, attachments,
	// links. note_events has no cascade and keeps the history.
	_, err := q.Exec(ctx, `delete from notes where id = $1`, nid)
	return err
}

// --- editor lock (NTE-06, NTE-08) ---

func scanLock(row pgx.Row) (api.NoteLock, bool, error) {
	var l api.NoteLock
	err := row.Scan(&l.Editor.ID, &l.Editor.DisplayName, &l.AcquiredAt, &l.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return l, false, nil
	}
	return l, err == nil, err
}

// NoteLock returns the current lock of a note, if any.
func (s *Store) NoteLock(ctx context.Context, q Querier, nid id.ID) (api.NoteLock, bool, error) {
	return scanLock(q.QueryRow(ctx, `select l.editor_id, u.display_name, l.acquired_at, l.expires_at
		from note_locks l join users u on u.id = l.editor_id where l.note_id = $1`, nid))
}

// SetNoteLock inserts or replaces a note's lock.
func (s *Store) SetNoteLock(ctx context.Context, q Querier, nid, editor id.ID, acquiredAt, expiresAt time.Time) error {
	_, err := q.Exec(ctx, `insert into note_locks (note_id, editor_id, acquired_at, expires_at) values ($1, $2, $3, $4)
		on conflict (note_id) do update set editor_id = excluded.editor_id, acquired_at = excluded.acquired_at, expires_at = excluded.expires_at`,
		nid, editor, acquiredAt, expiresAt)
	return err
}

// ReleaseNoteLock removes a note's lock when held by editor; reports whether
// a row was removed.
func (s *Store) ReleaseNoteLock(ctx context.Context, q Querier, nid, editor id.ID) (bool, error) {
	tag, err := q.Exec(ctx, `delete from note_locks where note_id = $1 and editor_id = $2`, nid, editor)
	return tag.RowsAffected() > 0, err
}

// --- drafts (NTE-08) ---

// NoteDraft returns the caller's draft of a note; ErrNotFound when none.
func (s *Store) NoteDraft(ctx context.Context, q Querier, nid, uid id.ID) (api.NoteDraft, error) {
	var d api.NoteDraft
	var content []byte
	err := q.QueryRow(ctx, `select title, content, base_version, saved_at from note_drafts where note_id = $1 and user_id = $2`, nid, uid).
		Scan(&d.Title, &content, &d.BaseVersion, &d.SavedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrNotFound
	}
	if err != nil {
		return d, err
	}
	d.Content, err = unmarshalBlocks(content)
	return d, err
}

// SaveNoteDraft inserts or replaces the caller's draft.
func (s *Store) SaveNoteDraft(ctx context.Context, q Querier, nid, uid id.ID, d api.NoteDraft) error {
	content, err := marshalBlocks(d.Content)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `insert into note_drafts (note_id, user_id, title, content, base_version, saved_at)
		values ($1, $2, $3, $4, $5, $6)
		on conflict (note_id, user_id) do update set title = excluded.title, content = excluded.content,
			base_version = excluded.base_version, saved_at = excluded.saved_at`,
		nid, uid, d.Title, content, d.BaseVersion, d.SavedAt)
	return err
}

// DeleteNoteDraft removes the caller's draft; reports whether one existed.
func (s *Store) DeleteNoteDraft(ctx context.Context, q Querier, nid, uid id.ID) (bool, error) {
	tag, err := q.Exec(ctx, `delete from note_drafts where note_id = $1 and user_id = $2`, nid, uid)
	return tag.RowsAffected() > 0, err
}

// --- comments (NTE-06) ---

func (s *Store) NoteComments(ctx context.Context, q Querier, nid id.ID) ([]api.NoteComment, error) {
	rows, err := q.Query(ctx, `select c.id, u.id, u.display_name, c.body, c.created_at from note_comments c
		join users u on u.id = c.author_id where c.note_id = $1 order by c.created_at, c.id`, nid)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (api.NoteComment, error) {
		var c api.NoteComment
		e := r.Scan(&c.ID, &c.Author.ID, &c.Author.DisplayName, &c.Body, &c.CreatedAt)
		return c, e
	})
}

func (s *Store) AddNoteComment(ctx context.Context, q Querier, nid id.ID, c api.NoteComment) error {
	_, err := q.Exec(ctx, `insert into note_comments (id, note_id, author_id, body, created_at) values ($1, $2, $3, $4, $5)`,
		c.ID, nid, c.Author.ID, c.Body, c.CreatedAt)
	return err
}

// --- attachments (NTE-09) ---

func (s *Store) NoteAttachments(ctx context.Context, q Querier, nid id.ID) ([]api.NoteAttachment, error) {
	rows, err := q.Query(ctx, `select a.id, a.name, a.url, a.status, a.content_type, a.size_bytes, u.id, u.display_name, a.created_at
		from note_attachments a join users u on u.id = a.added_by where a.note_id = $1 order by a.created_at, a.id`, nid)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (api.NoteAttachment, error) {
		var a api.NoteAttachment
		e := r.Scan(&a.ID, &a.Name, &a.URL, &a.Status, &a.ContentType, &a.SizeBytes, &a.AddedBy.ID, &a.AddedBy.DisplayName, &a.CreatedAt)
		return a, e
	})
}

func (s *Store) AddNoteAttachment(ctx context.Context, q Querier, nid id.ID, a api.NoteAttachment) error {
	_, err := q.Exec(ctx, `insert into note_attachments (id, note_id, name, url, content_type, size_bytes, status, added_by, created_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		a.ID, nid, a.Name, a.URL, a.ContentType, a.SizeBytes, a.Status, a.AddedBy.ID, a.CreatedAt)
	return err
}

// NoteAttachmentNote returns the note a given attachment belongs to.
func (s *Store) NoteAttachmentNote(ctx context.Context, q Querier, aid id.ID) (id.ID, error) {
	var nid id.ID
	err := q.QueryRow(ctx, `select note_id from note_attachments where id = $1`, aid).Scan(&nid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nid, ErrNotFound
	}
	return nid, err
}

func (s *Store) DeleteNoteAttachment(ctx context.Context, q Querier, nid, aid id.ID) (bool, error) {
	tag, err := q.Exec(ctx, `delete from note_attachments where id = $1 and note_id = $2`, aid, nid)
	return tag.RowsAffected() > 0, err
}

// --- links to tasks/reminders (NTE-05) ---

func (s *Store) NoteLinks(ctx context.Context, q Querier, nid id.ID) ([]api.NoteLink, error) {
	rows, err := q.Query(ctx, `select l.id, l.target_type, l.target_id, u.id, u.display_name, l.created_at from note_links l
		join users u on u.id = l.created_by where l.note_id = $1 order by l.created_at, l.id`, nid)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (api.NoteLink, error) {
		var l api.NoteLink
		e := r.Scan(&l.ID, &l.TargetType, &l.TargetID, &l.CreatedBy.ID, &l.CreatedBy.DisplayName, &l.CreatedAt)
		return l, e
	})
}

// AddNoteLink records a reference; ErrConflict when it already exists.
func (s *Store) AddNoteLink(ctx context.Context, q Querier, nid id.ID, l api.NoteLink) error {
	_, err := q.Exec(ctx, `insert into note_links (id, note_id, target_type, target_id, created_by, created_at) values ($1, $2, $3, $4, $5, $6)`,
		l.ID, nid, l.TargetType, l.TargetID, l.CreatedBy.ID, l.CreatedAt)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

func (s *Store) NoteLinkNote(ctx context.Context, q Querier, lid id.ID) (id.ID, error) {
	var nid id.ID
	err := q.QueryRow(ctx, `select note_id from note_links where id = $1`, lid).Scan(&nid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nid, ErrNotFound
	}
	return nid, err
}

func (s *Store) DeleteNoteLink(ctx context.Context, q Querier, nid, lid id.ID) (bool, error) {
	tag, err := q.Exec(ctx, `delete from note_links where id = $1 and note_id = $2`, lid, nid)
	return tag.RowsAffected() > 0, err
}

// TaskExistsInPeriod reports whether a task with id tid exists.
func (s *Store) TaskExists(ctx context.Context, q Querier, tid id.ID) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `select exists (select 1 from tasks where id = $1)`, tid).Scan(&ok)
	return ok, err
}

// ReminderOwnedBy reports whether a reminder is owned by uid.
func (s *Store) ReminderOwnedBy(ctx context.Context, q Querier, rid, uid id.ID) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `select exists (select 1 from reminders where id = $1 and owner_id = $2)`, rid, uid).Scan(&ok)
	return ok, err
}

// --- events / history (AC-33, AC-34) ---

func (s *Store) AddNoteEvent(ctx context.Context, q Querier, nid id.ID, e api.NoteEvent) error {
	before, err := jsonOrNil(e.Before)
	if err != nil {
		return err
	}
	after, err := jsonOrNil(e.After)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `insert into note_events (id, note_id, actor_id, action, reason, created_at, before, after)
		values ($1, $2, $3, $4, $5, $6, $7, $8)`, e.ID, nid, e.Actor.ID, e.Action, e.Reason, e.CreatedAt, before, after)
	return err
}

func (s *Store) NoteHistory(ctx context.Context, q Querier, nid id.ID) ([]api.NoteEvent, error) {
	rows, err := q.Query(ctx, `select e.id, u.id, u.display_name, e.action, e.reason, e.created_at, e.before, e.after
		from note_events e join users u on u.id = e.actor_id where e.note_id = $1 order by e.created_at desc, e.id desc`, nid)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (api.NoteEvent, error) {
		var e api.NoteEvent
		var before, after []byte
		if err := r.Scan(&e.ID, &e.Actor.ID, &e.Actor.DisplayName, &e.Action, &e.Reason, &e.CreatedAt, &before, &after); err != nil {
			return e, err
		}
		if len(before) > 0 {
			e.Before = json.RawMessage(before)
		}
		if len(after) > 0 {
			e.After = json.RawMessage(after)
		}
		return e, nil
	})
}

// --- templates (NTE-02, NTE-09) ---

func scanTemplate(r pgx.CollectableRow) (api.NoteTemplate, error) {
	var t api.NoteTemplate
	var sid id.ID
	var content []byte
	var created, updated time.Time
	if err := r.Scan(&t.ID, &sid, &t.Name, &content, &created, &updated); err != nil {
		return t, err
	}
	t.SpaceID = &sid
	t.Kind = api.TemplateCustom
	t.Builtin = false
	t.CreatedAt = &created
	t.UpdatedAt = &updated
	blocks, err := unmarshalBlocks(content)
	t.Content = blocks
	return t, err
}

// SpaceTemplates returns the saved custom templates of a space, by name.
func (s *Store) SpaceTemplates(ctx context.Context, q Querier, sid id.ID) ([]api.NoteTemplate, error) {
	rows, err := q.Query(ctx, `select id, space_id, name, content, created_at, updated_at from note_templates
		where space_id = $1 order by lower(name), id`, sid)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanTemplate)
}

// Template returns a saved template with its period and division.
func (s *Store) Template(ctx context.Context, q Querier, tid id.ID) (api.NoteTemplate, id.ID, id.ID, error) {
	var t api.NoteTemplate
	var sid, pid, did id.ID
	var content []byte
	var created, updated time.Time
	err := q.QueryRow(ctx, `select id, space_id, period_id, division_id, name, content, created_at, updated_at from note_templates where id = $1`, tid).
		Scan(&t.ID, &sid, &pid, &did, &t.Name, &content, &created, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, pid, did, ErrNotFound
	}
	if err != nil {
		return t, pid, did, err
	}
	t.SpaceID, t.Kind, t.Builtin, t.CreatedAt, t.UpdatedAt = &sid, api.TemplateCustom, false, &created, &updated
	t.Content, err = unmarshalBlocks(content)
	return t, pid, did, err
}

func (s *Store) InsertTemplate(ctx context.Context, q Querier, t api.NoteTemplate, pid, did, createdBy id.ID, now time.Time) error {
	content, err := marshalBlocks(t.Content)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `insert into note_templates (id, space_id, period_id, division_id, name, content, created_by, created_at, updated_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $8)`, t.ID, t.SpaceID, pid, did, t.Name, content, createdBy, now)
	return err
}

func (s *Store) UpdateTemplate(ctx context.Context, q Querier, tid id.ID, name string, content []api.NoteBlock, now time.Time) error {
	raw, err := marshalBlocks(content)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `update note_templates set name = $2, content = $3, updated_at = $4 where id = $1`, tid, name, raw, now)
	return err
}

func (s *Store) DeleteTemplate(ctx context.Context, q Querier, tid id.ID) error {
	_, err := q.Exec(ctx, `delete from note_templates where id = $1`, tid)
	return err
}
