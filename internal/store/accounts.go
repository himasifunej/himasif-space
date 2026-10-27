package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Invitation is an invitation row, without its token hash.
type Invitation struct {
	ID         id.ID
	PeriodID   id.ID
	DivisionID id.ID
	Email      string
	InvitedBy  id.ID
	CreatedAt  time.Time
	SentAt     time.Time
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	AcceptedAt *time.Time
}

// Status returns the status of the invitation at now.
func (i Invitation) Status(now time.Time) string {
	switch {
	case i.AcceptedAt != nil:
		return api.InvitationAccepted
	case i.RevokedAt != nil:
		return api.InvitationRevoked
	case !now.Before(i.ExpiresAt):
		return api.InvitationExpired
	}
	return api.InvitationPending
}

// API returns the invitation as its managers see it.
func (i Invitation) API(now time.Time) api.Invitation {
	return api.Invitation{ID: i.ID, PeriodID: i.PeriodID, DivisionID: i.DivisionID, Email: i.Email,
		Status: i.Status(now), ExpiresAt: i.ExpiresAt.UTC(), CreatedAt: i.CreatedAt.UTC()}
}

const invitationColumns = `id, period_id, division_id, email, invited_by, created_at, sent_at, expires_at, revoked_at, accepted_at`

func scanInvitation(r pgx.CollectableRow) (Invitation, error) {
	var i Invitation
	err := r.Scan(&i.ID, &i.PeriodID, &i.DivisionID, &i.Email, &i.InvitedBy, &i.CreatedAt, &i.SentAt, &i.ExpiresAt, &i.RevokedAt, &i.AcceptedAt)
	return i, err
}

func oneInvitation(rows pgx.Rows, err error) (Invitation, error) {
	if err != nil {
		return Invitation{}, err
	}
	i, err := pgx.CollectExactlyOneRow(rows, scanInvitation)
	if errors.Is(err, pgx.ErrNoRows) {
		return i, ErrNotFound
	}
	return i, err
}

// CreateInvitation adds an invitation whose token hashes to tokenHash;
// ErrConflict when the email has an open (pending or expired) invitation
// to the scope.
func (s *Store) CreateInvitation(ctx context.Context, q Querier, i Invitation, tokenHash []byte) error {
	_, err := q.Exec(ctx, `insert into invitations (id, period_id, division_id, email, token_hash, invited_by, created_at, sent_at, expires_at)
		values ($1, $2, $3, $4, $5, $6, $7, $7, $8)`, i.ID, i.PeriodID, i.DivisionID, i.Email, tokenHash, i.InvitedBy, i.CreatedAt, i.ExpiresAt)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

// Invitations returns the invitations of a scope, the latest first.
func (s *Store) Invitations(ctx context.Context, q Querier, pid, did id.ID) ([]Invitation, error) {
	rows, err := q.Query(ctx, `select `+invitationColumns+` from invitations where period_id = $1 and division_id = $2 order by created_at desc, id desc`, pid, did)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanInvitation)
}

// InvitationInScope returns an invitation of a scope, locked as lock says.
func (s *Store) InvitationInScope(ctx context.Context, q Querier, pid, did, iid id.ID, lock string) (Invitation, error) {
	return oneInvitation(q.Query(ctx, `select `+invitationColumns+` from invitations where period_id = $1 and division_id = $2 and id = $3`+lock, pid, did, iid))
}

// InvitationByToken returns the invitation of a token hash, locked as lock
// says.
func (s *Store) InvitationByToken(ctx context.Context, q Querier, tokenHash []byte, lock string) (Invitation, error) {
	return oneInvitation(q.Query(ctx, `select `+invitationColumns+` from invitations where token_hash = $1`+lock, tokenHash))
}

// RotateInvitation gives an invitation a new token and expiry.
func (s *Store) RotateInvitation(ctx context.Context, q Querier, iid id.ID, tokenHash []byte, sentAt, expiresAt time.Time) error {
	_, err := q.Exec(ctx, `update invitations set token_hash = $2, sent_at = $3, expires_at = $4 where id = $1`, iid, tokenHash, sentAt, expiresAt)
	return err
}

// RevokeInvitation revokes an invitation.
func (s *Store) RevokeInvitation(ctx context.Context, q Querier, iid, by id.ID, now time.Time) error {
	_, err := q.Exec(ctx, `update invitations set revoked_at = $2, revoked_by = $3 where id = $1 and revoked_at is null and accepted_at is null`, iid, now, by)
	return err
}

// MarkInvitationAccepted records who accepted an invitation.
func (s *Store) MarkInvitationAccepted(ctx context.Context, q Querier, iid, uid id.ID, now time.Time) error {
	_, err := q.Exec(ctx, `update invitations set accepted_at = $2, accepted_by = $3 where id = $1`, iid, now, uid)
	return err
}

// Token purposes.
const (
	TokenInvitationEmail = "invitation_email"
	TokenRecovery        = "recovery"
	TokenAdminEmail      = "admin_email"
)

// AuthToken is a single-use token sent by email.
type AuthToken struct {
	ID           id.ID
	Purpose      string
	Email        string
	UserID       *id.ID
	InvitationID *id.ID
	ExpiresAt    time.Time
	UsedAt       *time.Time
}

// CreateAuthToken records a token whose value hashes to tokenHash.
func (s *Store) CreateAuthToken(ctx context.Context, q Querier, t AuthToken, tokenHash []byte, now time.Time) error {
	_, err := q.Exec(ctx, `insert into auth_tokens (id, purpose, token_hash, email, user_id, invitation_id, created_at, expires_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8)`, id.New(), t.Purpose, tokenHash, t.Email, t.UserID, t.InvitationID, now, t.ExpiresAt)
	return err
}

// UsableAuthToken returns, locked, the token of a hash and purpose when it
// is neither used nor expired; ErrNotFound otherwise.
func (s *Store) UsableAuthToken(ctx context.Context, q Querier, tokenHash []byte, purpose string, now time.Time) (AuthToken, error) {
	var t AuthToken
	err := q.QueryRow(ctx, `select id, purpose, email, user_id, invitation_id, expires_at, used_at from auth_tokens
		where token_hash = $1 and purpose = $2 and used_at is null and expires_at > $3 for update`, tokenHash, purpose, now).
		Scan(&t.ID, &t.Purpose, &t.Email, &t.UserID, &t.InvitationID, &t.ExpiresAt, &t.UsedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

// AuthTokenUser returns an account hint without locking the token. This is
// not authorization: callers must lock that account first, then validate and
// lock the token with UsableAuthToken. All account/token writers use that order.
func (s *Store) AuthTokenUser(ctx context.Context, q Querier, tokenHash []byte, purpose string) (id.ID, error) {
	var uid id.ID
	err := q.QueryRow(ctx, `select user_id from auth_tokens where token_hash=$1 and purpose=$2 and user_id is not null`, tokenHash, purpose).Scan(&uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return id.Nil, ErrNotFound
	}
	return uid, err
}

// UseInvitationTokens spends every unused email proof of an invitation.
func (s *Store) UseInvitationTokens(ctx context.Context, q Querier, iid id.ID, now time.Time) error {
	_, err := q.Exec(ctx, `update auth_tokens set used_at = $2 where invitation_id = $1 and used_at is null`, iid, now)
	return err
}

// UseUserTokens spends every unused token of a user for a purpose.
func (s *Store) UseUserTokens(ctx context.Context, q Querier, uid id.ID, purpose string, now time.Time) error {
	_, err := q.Exec(ctx, `update auth_tokens set used_at = $3 where user_id = $1 and purpose = $2 and used_at is null`, uid, purpose, now)
	return err
}

// UserForUpdate returns an account, locked.
func (s *Store) UserForUpdate(ctx context.Context, q Querier, uid id.ID) (User, error) {
	return scanUser(q.QueryRow(ctx, `select `+userColumns+` from users where id = $1 for update`, uid))
}

// UserByEmailQ returns the account of an email, in q.
func (s *Store) UserByEmailQ(ctx context.Context, q Querier, email string) (User, error) {
	return scanUser(q.QueryRow(ctx, `select `+userColumns+` from users where email = $1`, NormalizeEmail(email)))
}

// SetPassword sets the password hash of an account.
func (s *Store) SetPassword(ctx context.Context, q Querier, uid id.ID, hash string, now time.Time) error {
	_, err := q.Exec(ctx, `update users set password_hash = $2, updated_at = $3 where id = $1`, uid, hash, now)
	return err
}

// SetVerifiedEmail sets the email of an account, proved at now;
// ErrEmailTaken when another account has it.
func (s *Store) SetVerifiedEmail(ctx context.Context, q Querier, uid id.ID, email string, now time.Time) error {
	_, err := q.Exec(ctx, `update users set email = $2, email_verified_at = $3, updated_at = $3 where id = $1`, uid, NormalizeEmail(email), now)
	if isUniqueViolation(err) {
		return ErrEmailTaken
	}
	return err
}

// SetDisabled disables or enables an account.
func (s *Store) SetDisabled(ctx context.Context, q Querier, uid id.ID, disabled bool, now time.Time) (User, error) {
	var at *time.Time
	if disabled {
		at = &now
	}
	return scanUser(q.QueryRow(ctx, `update users set disabled_at = $2, updated_at = $3 where id = $1 returning `+userColumns, uid, at, now))
}

// People returns the personal accounts, by name.
func (s *Store) People(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx, `select `+userColumns+` from users where kind = 'person' order by lower(display_name), id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (User, error) { return scanUser(r) })
}

// RevokeUserSessions ends every session of an account but except, if not
// nil, returning how many it ended.
func (s *Store) RevokeUserSessions(ctx context.Context, q Querier, uid id.ID, except *id.ID, now time.Time) (int64, error) {
	tag, err := q.Exec(ctx, `update sessions set revoked_at = $2 where user_id = $1 and revoked_at is null
		and ($3::uuid is null or id <> $3)`, uid, now, except)
	return tag.RowsAffected(), err
}
