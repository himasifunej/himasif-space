package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/himasifunej/himasif-space/internal/id"
)

// Session is a signed-in client.
type Session struct {
	ID        id.ID
	UserID    id.ID
	Client    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// CreateSession records a session whose token hashes to tokenHash.
func (s *Store) CreateSession(ctx context.Context, q Querier, userID id.ID, tokenHash []byte, client string, expiresAt time.Time) (Session, error) {
	sess := Session{ID: id.New(), UserID: userID, Client: client, ExpiresAt: expiresAt}
	err := q.QueryRow(ctx, `insert into sessions (id, user_id, token_hash, client, expires_at)
		values ($1, $2, $3, $4, $5) returning created_at`,
		sess.ID, userID, tokenHash, client, expiresAt).Scan(&sess.CreatedAt)
	return sess, err
}

// SessionUser returns the session of a token hash and its account, when the
// session is neither expired nor revoked and the account is not disabled.
// It notes the session as seen, at most once a minute.
func (s *Store) SessionUser(ctx context.Context, tokenHash []byte, now time.Time) (Session, User, error) {
	var sess Session
	row := s.pool.QueryRow(ctx, `select s.id, s.user_id, s.client, s.created_at, s.expires_at, `+userColumnsU+`
		from sessions s join users u on u.id = s.user_id
		where s.token_hash = $1 and s.revoked_at is null and s.expires_at > $2 and u.disabled_at is null`,
		tokenHash, now)
	var u User
	err := row.Scan(&sess.ID, &sess.UserID, &sess.Client, &sess.CreatedAt, &sess.ExpiresAt,
		&u.ID, &u.Kind, &u.Email, &u.DisplayName, &u.PasswordHash, &u.EmailVerifiedAt, &u.DisabledAt, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, User{}, ErrNotFound
	}
	if err != nil {
		return Session{}, User{}, err
	}
	_, err = s.pool.Exec(ctx, `update sessions set last_seen_at = $2 where id = $1 and last_seen_at < $2::timestamptz - interval '1 minute'`, sess.ID, now)
	return sess, u, err
}

// RevokeSession ends a session.
func (s *Store) RevokeSession(ctx context.Context, q Querier, sessionID id.ID, now time.Time) error {
	_, err := q.Exec(ctx, `update sessions set revoked_at = $2 where id = $1 and revoked_at is null`, sessionID, now)
	return err
}

// SessionActive revalidates a session within an account transaction. Callers
// hold the user row lock so credential rotation cannot interleave with it.
func (s *Store) SessionActive(ctx context.Context, q Querier, sessionID, userID id.ID, now time.Time) (bool, error) {
	var active bool
	err := q.QueryRow(ctx, `select exists(select 1 from sessions where id=$1 and user_id=$2 and revoked_at is null and expires_at>$3)`, sessionID, userID, now).Scan(&active)
	return active, err
}
