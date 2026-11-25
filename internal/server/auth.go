package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/auth"
	"github.com/himasifunej/himasif-space/internal/config"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
)

// principal is who makes a request: the account and its session.
type principal struct {
	User    store.User
	Session store.Session
	// monitor is org.monitor in this request: set only for API requests
	// with a session, so WhatsApp and workers never act as a monitor.
	monitor *monitorState
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
	return &principal{User: u, Session: sess, monitor: &monitorState{}}, nil
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

// hashWait is how long a login waits for a free password hashing slot
// before it is answered 429 (auth.ErrBusy).
const hashWait = 5 * time.Second

var errHashBusy = fail(http.StatusTooManyRequests, api.CodeRateLimited, "Server sedang memproses banyak permintaan masuk. Coba lagi beberapa detik lagi.")

func writeHashBusy(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "5")
	writeError(w, http.StatusTooManyRequests, api.CodeRateLimited, "Server sedang memproses banyak permintaan masuk. Coba lagi beberapa detik lagi.")
}

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
	ipKey, emailKey := "ip:"+clientIP(r), "email:"+email
	// Refuse a blocked address before retaining any additional email keys.
	// Every attempt is counted up front, so a burst in parallel cannot
	// guess more than the limit; a successful login is forgiven below.
	if !s.loginLimit.allow(ipKey, now) {
		writeError(w, http.StatusTooManyRequests, api.CodeRateLimited, "Terlalu banyak percobaan masuk. Coba lagi beberapa menit lagi.")
		return
	}
	if !s.loginLimit.allow(emailKey, now) {
		s.loginLimit.forgive(ipKey, now)
		writeError(w, http.StatusTooManyRequests, api.CodeRateLimited, "Terlalu banyak percobaan masuk. Coba lagi beberapa menit lagi.")
		return
	}
	// busy answers a login that found no free hashing slot: it was not a
	// guess, so it is not counted against the address or the account.
	busy := func() {
		s.loginLimit.forgive(ipKey, now)
		s.loginLimit.forgive(emailKey, now)
		writeHashBusy(w)
	}
	hashCtx, cancel := context.WithTimeout(r.Context(), hashWait)
	defer cancel()
	u, err := s.store.UserByEmail(r.Context(), email)
	switch {
	case errors.Is(err, store.ErrNotFound):
		if auth.BurnTimeContext(hashCtx, req.Password) != nil {
			busy()
			return
		}
		writeError(w, http.StatusUnauthorized, api.CodeUnauthenticated, wrongCredentials)
		return
	case err != nil:
		s.internal(w, r, err)
		return
	}
	if u.PasswordHash == "" {
		// An account still waiting for its owner to set a password.
		if auth.BurnTimeContext(hashCtx, req.Password) != nil {
			busy()
			return
		}
		writeError(w, http.StatusUnauthorized, api.CodeUnauthenticated, wrongCredentials)
		return
	}
	if err := auth.VerifyPasswordContext(hashCtx, u.PasswordHash, req.Password); err != nil {
		if errors.Is(err, auth.ErrBusy) {
			busy()
			return
		}
		if !errors.Is(err, auth.ErrMismatch) {
			s.log.Error("login: password hash", "user_id", u.ID, "err", err)
		}
		writeError(w, http.StatusUnauthorized, api.CodeUnauthenticated, wrongCredentials)
		return
	}
	// The right password: not a guess. Forgive both counts, so people
	// behind one campus address are not locked out by logging in.
	s.loginLimit.forgive(ipKey, now)
	s.loginLimit.forgive(emailKey, now)
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
		// Serialize session creation with password reset, email handover and
		// disable. Password verification above may have used an older row.
		current, err := s.store.UserForUpdate(r.Context(), q, u.ID)
		if err != nil {
			return err
		}
		if current.Email != email || current.PasswordHash != u.PasswordHash {
			return fail(http.StatusUnauthorized, api.CodeUnauthenticated, wrongCredentials)
		}
		if current.DisabledAt != nil {
			return fail(http.StatusForbidden, api.CodeForbidden, "Akun ini dinonaktifkan. Hubungi admin organisasi.")
		}
		u = current
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
		s.answerError(w, r, nil, err)
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

// clientIPs decides the client address of a request. Behind Cloudflare
// Tunnel, cloudflared connects from loopback and Cloudflare sets
// CF-Connecting-IP to the real client, overwriting whatever the client
// sent. X-Forwarded-For is not used then: its leftmost entry is whatever
// the client wrote (SEC-02).
type clientIPs struct {
	header  string // "CF-Connecting-IP", "X-Forwarded-For" or "none"
	trusted []netip.Prefix
}

func newClientIPs(cfg config.Config) clientIPs {
	c := clientIPs{header: cfg.ClientIPHeader, trusted: cfg.TrustedProxies}
	if c.header == "" {
		// A Config not made by config.Load (tests): the defaults.
		c.header = "CF-Connecting-IP"
		c.trusted, _ = config.ParsePrefixes(config.DefaultTrustedProxies)
	}
	return c
}

func (c clientIPs) isTrusted(a netip.Addr) bool {
	a = a.Unmap()
	for _, p := range c.trusted {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// resolve returns the client address of r.
func (c clientIPs) resolve(r *http.Request) string {
	remote := remoteHost(r)
	peer, err := netip.ParseAddr(remote)
	if err != nil || c.header == "none" || !c.isTrusted(peer) {
		return remote
	}
	switch c.header {
	case "CF-Connecting-IP":
		if a, err := netip.ParseAddr(strings.TrimSpace(r.Header.Get("CF-Connecting-IP"))); err == nil {
			return a.Unmap().String()
		}
	case "X-Forwarded-For":
		// Each proxy appends the address it got the request from; the
		// rightmost hop that is not one of ours is the client. Entries
		// left of it are the client's own words.
		hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
		for i := len(hops) - 1; i >= 0; i-- {
			a, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
			if err != nil {
				break
			}
			if !c.isTrusted(a) || i == 0 {
				return a.Unmap().String()
			}
		}
	}
	return remote
}

func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if a, err := netip.ParseAddr(host); err == nil {
		return a.Unmap().String()
	}
	return host
}

// clientIP returns the address of the client, as the middleware resolved
// it; without the middleware, the connection's own address.
func clientIP(r *http.Request) string {
	if v, ok := r.Context().Value(clientIPKey).(string); ok && v != "" {
		return v
	}
	return remoteHost(r)
}
