// Package api holds the JSON messages between Space's server and its
// clients (desktop, web and, later, the WhatsApp bridge and mobile). Both
// sides import it so that a field renamed on one side fails to compile on
// the other.
package api

import (
	"time"

	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// Account kinds. The organization admin is one shared account handed over
// between periods, apart from personal accounts (PRD ACC-12).
const (
	AccountPerson            = "person"
	AccountOrganizationAdmin = "organization_admin"
)

// Period statuses. A closed period is read-only (PRD PER-05).
const (
	PeriodPreparing = "preparing"
	PeriodActive    = "active"
	PeriodClosed    = "closed"
)

// Error codes, which clients may switch on; Message is for people.
const (
	CodeBadRequest      = "bad_request"
	CodeUnauthenticated = "unauthenticated"
	CodeForbidden       = "forbidden"
	CodeNotFound        = "not_found"
	CodeRateLimited     = "rate_limited"
	CodeUnavailable     = "unavailable"
	CodeInternal        = "internal"
)

// Error is the body of every failed response.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse wraps Error, as {"error": {...}}.
type ErrorResponse struct {
	Error Error `json:"error"`
}

// Health reports whether the server and its database answer.
type Health struct {
	Status   string    `json:"status"` // "ok" or "degraded"
	Database string    `json:"database"`
	Version  string    `json:"version"`
	Time     time.Time `json:"time"`
}

// LoginRequest signs in with an email and a password.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	// Client names the app signing in, "web" or "desktop", for the list of
	// sessions; it grants nothing.
	Client string `json:"client"`
}

// LoginResponse carries the session token, which clients send back as
// "Authorization: Bearer <token>".
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
	User      User      `json:"user"`
}

// User is an account as its owner sees it.
type User struct {
	ID            id.ID      `json:"id"`
	Kind          string     `json:"kind"`
	Email         string     `json:"email"`
	DisplayName   string     `json:"displayName"`
	EmailVerified bool       `json:"emailVerified"`
	CreatedAt     time.Time  `json:"createdAt"`
	DisabledAt    *time.Time `json:"disabledAt,omitempty"`
}

// Period is one kepengurusan, as 2025/2026.
type Period struct {
	ID       id.ID         `json:"id"`
	Label    string        `json:"label"`
	StartsOn timeutil.Date `json:"startsOn"`
	EndsOn   timeutil.Date `json:"endsOn"`
	TimeZone string        `json:"timeZone"`
	Status   string        `json:"status"`
}

// Division is a division whose identity lasts across periods.
type Division struct {
	ID   id.ID  `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// List wraps the items of a list response, leaving room for paging.
type List[T any] struct {
	Items []T `json:"items"`
}
