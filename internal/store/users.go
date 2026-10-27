package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// ErrEmailTaken is what CreateUser returns when the email has an account.
var ErrEmailTaken = errors.New("store: email already has an account")

// User is an account row.
type User struct {
	ID              id.ID
	Kind            string
	Email           string
	DisplayName     string
	PasswordHash    string
	EmailVerifiedAt *time.Time
	DisabledAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// API returns the user as its owner sees it, without the password hash.
func (u User) API() api.User {
	return api.User{
		ID:            u.ID,
		Kind:          u.Kind,
		Email:         u.Email,
		DisplayName:   u.DisplayName,
		EmailVerified: u.EmailVerifiedAt != nil,
		CreatedAt:     u.CreatedAt,
		DisabledAt:    u.DisabledAt,
	}
}

// NormalizeEmail returns the form emails are stored and compared in.
func NormalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

const userColumns = `id, kind, email, display_name, coalesce(password_hash, ''), email_verified_at, disabled_at, created_at, updated_at`

// userColumnsU is userColumns of the table aliased u, for joins.
const userColumnsU = `u.id, u.kind, u.email, u.display_name, coalesce(u.password_hash, ''), u.email_verified_at, u.disabled_at, u.created_at, u.updated_at`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Kind, &u.Email, &u.DisplayName, &u.PasswordHash, &u.EmailVerifiedAt, &u.DisabledAt, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// UserByEmail returns the account of an email.
func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `select `+userColumns+` from users where email = $1`, NormalizeEmail(email)))
}

// UserByID returns an account.
func (s *Store) UserByID(ctx context.Context, uid id.ID) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `select `+userColumns+` from users where id = $1`, uid))
}

// OrganizationAdmin returns the organization admin account.
func (s *Store) OrganizationAdmin(ctx context.Context) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `select `+userColumns+` from users where kind = 'organization_admin'`))
}

// CreateUser adds an account, with u.ID made when it is nil.
func (s *Store) CreateUser(ctx context.Context, q Querier, u User) (User, error) {
	if u.ID == id.Nil {
		u.ID = id.New()
	}
	u.Email = NormalizeEmail(u.Email)
	var hash *string
	if u.PasswordHash != "" {
		hash = &u.PasswordHash
	}
	row := q.QueryRow(ctx, `insert into users (id, kind, email, display_name, password_hash, email_verified_at)
		values ($1, $2, $3, $4, $5, $6) returning `+userColumns,
		u.ID, u.Kind, u.Email, u.DisplayName, hash, u.EmailVerifiedAt)
	created, err := scanUser(row)
	if isUniqueViolation(err) {
		return User{}, ErrEmailTaken
	}
	return created, err
}
