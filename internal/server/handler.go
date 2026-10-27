package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/auth"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
)

// apiError is a failure a handler answers with: its status, code and a
// message for people. Returned from a transaction it also rolls it back.
type apiError struct {
	status  int
	code    string
	message string
}

func (e *apiError) Error() string { return e.code + ": " + e.message }

func fail(status int, code, message string) error {
	return &apiError{status: status, code: code, message: message}
}

func badRequest(message string) error {
	return fail(http.StatusBadRequest, api.CodeBadRequest, message)
}
func conflict(message string) error { return fail(http.StatusConflict, api.CodeConflict, message) }

var (
	errForbidden    = fail(http.StatusForbidden, api.CodeForbidden, "Anda tidak berwenang pada scope ini.")
	errNotFound     = fail(http.StatusNotFound, api.CodeNotFound, "Data tidak ditemukan.")
	errPeriodClosed = fail(http.StatusConflict, api.CodePeriodClosed, "Periode ini sudah ditutup dan hanya dapat dibaca.")
	errInvalidToken = fail(http.StatusGone, api.CodeInvalidToken, "Tautan tidak berlaku lagi. Minta tautan baru.")
	errRateLimited  = fail(http.StatusTooManyRequests, api.CodeRateLimited, "Terlalu banyak permintaan. Coba lagi beberapa menit lagi.")
	errMailFailed   = fail(http.StatusServiceUnavailable, api.CodeUnavailable, "Email tidak dapat dikirim saat ini. Coba lagi nanti.")
)

// handle wraps a handler that needs a session and returns its failure.
func (s *Server) handle(h func(http.ResponseWriter, *http.Request, *principal) error) http.Handler {
	return s.authed(func(w http.ResponseWriter, r *http.Request, p *principal) {
		if err := h(w, r, p); err != nil {
			s.answerError(w, r, p, err)
		}
	})
}

// handlePublic wraps a handler that needs no session.
func (s *Server) handlePublic(h func(http.ResponseWriter, *http.Request) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			s.answerError(w, r, nil, err)
		}
	})
}

func (s *Server) answerError(w http.ResponseWriter, r *http.Request, p *principal, err error) {
	var ae *apiError
	switch {
	case errors.As(err, &ae):
		if ae.status == http.StatusForbidden && p != nil && r.Method != http.MethodGet {
			s.auditDenied(r, p)
		}
		writeError(w, ae.status, ae.code, ae.message)
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, api.CodeNotFound, "Data tidak ditemukan.")
	default:
		s.internal(w, r, err)
	}
}

// auditDenied records a refused change, as a 45 trying to manage roles
// (AC-06). It is written on its own: the refused change was rolled back.
func (s *Server) auditDenied(r *http.Request, p *principal) {
	uid, kind := p.actor()
	e := store.AuditEvent{
		ActorUserID: uid, ActorKind: kind, Action: "authz.denied", Source: "api",
		RequestID: requestID(r.Context()), OccurredAt: s.now(),
		After: map[string]any{"method": r.Method, "route": r.Pattern},
	}
	if pid, err := id.Parse(r.PathValue("periodId")); err == nil {
		e.PeriodID = &pid
	}
	if did, err := id.Parse(r.PathValue("divisionId")); err == nil {
		e.ScopeType, e.ScopeID = "division", &did
	}
	if err := s.store.Audit(r.Context(), s.store.DB(), e); err != nil {
		s.log.Error("audit denied request", "request_id", requestID(r.Context()), "err", err)
	}
}

// record appends an audit event of p's action in the request r.
func (s *Server) record(r *http.Request, q store.Querier, p *principal, e store.AuditEvent) error {
	if p != nil && e.ActorKind == "" {
		e.ActorUserID, e.ActorKind = p.actor()
	}
	if e.Source == "" {
		e.Source = "api"
	}
	e.RequestID = requestID(r.Context())
	if e.OccurredAt.IsZero() {
		e.OccurredAt = s.now()
	}
	return s.store.Audit(r.Context(), q, e)
}

// readJSON reads a JSON body into v, rejecting unknown fields and trailing
// data.
func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return fail(http.StatusRequestEntityTooLarge, api.CodeBadRequest, "Permintaan terlalu besar.")
		}
		return badRequest("Isi permintaan tidak valid.")
	}
	if dec.More() {
		return badRequest("Isi permintaan tidak valid.")
	}
	return nil
}

// pathID reads an ID of the URL path; one that is not an ID names nothing.
func pathID(r *http.Request, name string) (id.ID, error) {
	v, err := id.Parse(r.PathValue(name))
	if err != nil {
		return id.Nil, errNotFound
	}
	return v, nil
}

// notFound turns store.ErrNotFound into err, leaving other errors.
func notFound(err, as error) error {
	if errors.Is(err, store.ErrNotFound) {
		return as
	}
	return err
}

// validEmail returns the normalized form of an email address people type,
// or a failure.
func validEmail(s string) (string, error) {
	e := store.NormalizeEmail(s)
	if e == "" {
		return "", badRequest("Isi alamat email.")
	}
	a, err := mail.ParseAddress(e)
	if err != nil || a.Address != e || a.Name != "" || len(e) > 254 || !strings.Contains(e[strings.IndexByte(e, '@'):], ".") {
		return "", badRequest("Alamat email tidak valid.")
	}
	return e, nil
}

// text trims s and checks its length in characters.
func text(s, field string, min, max int) (string, error) {
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	if n < min || n > max || strings.ContainsRune(s, 0) {
		if min > 0 && n == 0 {
			return "", badRequest("Isi " + field + ".")
		}
		return "", badRequest(field + " terlalu panjang atau tidak valid.")
	}
	return s, nil
}

// token checks the form of a token of an email link, and returns its hash.
// A malformed token is as invalid as an unknown one.
func tokenHash(t string) ([]byte, error) {
	t = strings.TrimSpace(t)
	if t == "" || len(t) > 128 {
		return nil, errInvalidToken
	}
	return auth.HashToken(t), nil
}

func (p *principal) admin() bool { return p.User.Kind == api.AccountOrganizationAdmin }
