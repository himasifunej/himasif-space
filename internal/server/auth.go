package server

import (
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/auth"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
)

// principal is who makes a request: the account and its session.
type principal struct {
	User    store.User
	Session store.Session
}

// actor returns the principal as the actor of an audit event.
func (p *principal) actor() (*id.ID, string) {
	uid := p.User.ID
	return &uid, p.User.Kind
}

// authed requires a valid session, and passes its principal to h.
func (s *Server) authed(h func(http.ResponseWriter, *http.Request, *principal)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := s.principal(r)
		if err != nil {
			s.answerError(w, r, nil, err)
			return
		}
		h(w, r, p)
	})
}

// principal returns who makes r, from its bearer session.
func (s *Server) principal(r *http.Request) (*principal, error) {
	token, ok := bearer(r)
	if !ok {
		return nil, fail(http.StatusUnauthorized, api.CodeUnauthenticated, "Silakan masuk terlebih dahulu.")
	}
	sess, u, err := s.store.SessionUser(r.Context(), auth.HashToken(token), s.now())
	if errors.Is(err, store.ErrNotFound) {
		return nil, fail(http.StatusUnauthorized, api.CodeUnauthenticated, "Sesi berakhir. Silakan masuk lagi.")
	}
	if err != nil {
		return nil, err
	}
	return &principal{User: u, Session: sess}, nil
}

// bearer returns the token of the Authorization header.
func bearer(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	scheme, token, ok := strings.Cut(h, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != "" && len(token) <= 128
}

const wrongCredentials = "Email atau kata sandi salah."

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req api.LoginRequest
	if !decode(w, r, &req) {
		return
	}
	email := store.NormalizeEmail(req.Email)
	if email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, api.CodeBadRequest, "Isi email dan kata sandi.")
		return
	}
	if len(email) > 254 {
		writeError(w, http.StatusBadRequest, api.CodeBadRequest, "Alamat email terlalu panjang.")
		return
	}
	switch req.Client {
	case "", "web", "desktop", "mobile":
	default:
		writeError(w, http.StatusBadRequest, api.CodeBadRequest, "Jenis aplikasi tidak dikenal.")
		return
	}
	now := s.now()
	// Refuse a blocked address before retaining any additional email keys.
	if !s.loginLimit.allow("ip:"+clientIP(r), now) || !s.loginLimit.allow("email:"+email, now) {
		writeError(w, http.StatusTooManyRequests, api.CodeRateLimited, "Terlalu banyak percobaan masuk. Coba lagi beberapa menit lagi.")
		return
	}
	u, err := s.store.UserByEmail(r.Context(), email)
	switch {
	case errors.Is(err, store.ErrNotFound):
		auth.BurnTime(req.Password)
		writeError(w, http.StatusUnauthorized, api.CodeUnauthenticated, wrongCredentials)
		return
	case err != nil:
		s.internal(w, r, err)
		return
	}
	if u.PasswordHash == "" {
		// An account still waiting for its owner to set a password.
		auth.BurnTime(req.Password)
		writeError(w, http.StatusUnauthorized, api.CodeUnauthenticated, wrongCredentials)
		return
	}
	if err := auth.VerifyPassword(u.PasswordHash, req.Password); err != nil {
		if !errors.Is(err, auth.ErrMismatch) {
			s.log.Error("login: password hash", "user_id", u.ID, "err", err)
		}
		writeError(w, http.StatusUnauthorized, api.CodeUnauthenticated, wrongCredentials)
		return
	}
	if u.DisabledAt != nil {
		writeError(w, http.StatusForbidden, api.CodeForbidden, "Akun ini dinonaktifkan. Hubungi admin organisasi.")
		return
	}
	token, hash, err := auth.NewToken()
	if err != nil {
		s.internal(w, r, err)
		return
	}
	expires := now.Add(s.cfg.SessionTTL).UTC()
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		sess, err := s.store.CreateSession(r.Context(), q, u.ID, hash, req.Client, expires)
		if err != nil {
			return err
		}
		uid := u.ID
		return s.store.Audit(r.Context(), q, store.AuditEvent{
			ActorUserID: &uid, ActorKind: u.Kind, Action: "auth.login", Source: "api",
			TargetType: "session", TargetID: &sess.ID, RequestID: requestID(r.Context()),
			After: map[string]any{"client": req.Client}, OccurredAt: now,
		})
	})
	if err != nil {
		s.internal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, api.LoginResponse{Token: token, ExpiresAt: expires, User: u.API()})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request, p *principal) {
	now := s.now()
	err := s.store.Tx(r.Context(), func(q store.Querier) error {
		if err := s.store.RevokeSession(r.Context(), q, p.Session.ID, now); err != nil {
			return err
		}
		uid, kind := p.actor()
		return s.store.Audit(r.Context(), q, store.AuditEvent{
			ActorUserID: uid, ActorKind: kind, Action: "auth.logout", Source: "api",
			TargetType: "session", TargetID: &p.Session.ID, RequestID: requestID(r.Context()), OccurredAt: now,
		})
	})
	if err != nil {
		s.internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request, p *principal) {
	writeJSON(w, http.StatusOK, p.User.API())
}

// clientIP returns the address of the client. Behind Coolify's proxy the
// connection comes from the proxy, which puts the client first in
// X-Forwarded-For; the header is trusted only from a loopback or private
// address, where that proxy runs.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate()) {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			first, _, _ := strings.Cut(fwd, ",")
			if f := net.ParseIP(strings.TrimSpace(first)); f != nil {
				return f.String()
			}
		}
	}
	return host
}
