package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func nonNilTags(t []string) []string {
	if t == nil {
		return []string{}
	}
	return t
}

// SetNoteTags replaces a note's tags.
func (s *Store) SetNoteTags(ctx context.Context, q Querier, nid id.ID, tags []string) error {
	_, err := q.Exec(ctx, `update notes set tags=$2 where id=$1`, nid, nonNilTags(tags))
	return err
}

// NotePublicLink returns a note's live public link, if any.
func (s *Store) NotePublicLink(ctx context.Context, q Querier, nid id.ID) (*api.NotePublicLink, error) {
	var l api.NotePublicLink
	err := q.QueryRow(ctx, `select l.created_at, l.created_by, u.display_name, l.view_count, l.last_viewed_at
		from note_public_links l join users u on u.id=l.created_by where l.note_id=$1 and l.revoked_at is null`, nid).
		Scan(&l.CreatedAt, &l.CreatedBy.ID, &l.CreatedBy.DisplayName, &l.ViewCount, &l.LastViewedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// RevokeNotePublicLink ends a note's live public link; it reports whether
// there was one.
func (s *Store) RevokeNotePublicLink(ctx context.Context, q Querier, nid, by id.ID, now time.Time) (bool, error) {
	tag, err := q.Exec(ctx, `update note_public_links set revoked_at=$3, revoked_by=$2 where note_id=$1 and revoked_at is null`, nid, by, now)
	return tag.RowsAffected() > 0, err
}

// InsertNotePublicLink records a new live link by its token's hash.
func (s *Store) InsertNotePublicLink(ctx context.Context, q Querier, nid, by id.ID, hash []byte, now time.Time) error {
	_, err := q.Exec(ctx, `insert into note_public_links(id,note_id,token_hash,created_by,created_at) values($1,$2,$3,$4,$5)`, id.New(), nid, hash, by, now)
	return err
}

// PublicNoteByToken finds the live note a token hash opens, counting the
// view: a revoked link, a deleted note or a closed period opens nothing.
func (s *Store) PublicNoteByToken(ctx context.Context, q Querier, hash []byte, now time.Time) (NoteRow, error) {
	var nid id.ID
	err := q.QueryRow(ctx, `update note_public_links l set view_count=view_count+1, last_viewed_at=$2
		from notes n join periods p on p.id=n.period_id
		where l.token_hash=$1 and l.revoked_at is null and n.id=l.note_id and n.deleted_at is null and p.status<>'closed'
		returning l.note_id`, hash, now).Scan(&nid)
	if errors.Is(err, pgx.ErrNoRows) {
		return NoteRow{}, ErrNotFound
	}
	if err != nil {
		return NoteRow{}, err
	}
	return s.Note(ctx, q, nid, LockNone)
}
