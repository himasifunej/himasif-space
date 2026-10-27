// Package server is Space's HTTP API, and serves the web app beside it.
//
// Authorization is decided here, on the server, for every client: desktop,
// web, the WhatsApp bridge and later mobile. Hiding a button in a client is
// never the only check (PRD ACC-10, ARC-05).
package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/config"
	"github.com/himasifunej/himasif-space/internal/mail"
	"github.com/himasifunej/himasif-space/internal/store"
)

// Version is the server's version, set at build time with
// -ldflags "-X github.com/himasifunej/himasif-space/internal/server.Version=…".
var Version = "dev"

// maxBodyBytes bounds request bodies.
const maxBodyBytes = 1 << 20

// Server handles the API.
type Server struct {
	cfg    config.Config
	store  *store.Store
	mailer mail.Mailer
	log    *slog.Logger
	now    func() time.Time

	loginLimit *limiter
	// mailLimit bounds the emails sent to one address; publicLimit the
	// requests of one client address to the public account endpoints.
	mailLimit   *limiter
	publicLimit *limiter

	// bg tracks work that outlives its request: recovery emails.
	bg sync.WaitGroup
}

// New returns a server.
func New(cfg config.Config, st *store.Store, mailer mail.Mailer, log *slog.Logger) *Server {
	return &Server{
		cfg:    cfg,
		store:  st,
		mailer: mailer,
		log:    log,
		now:    time.Now,
		// Ten attempts per email and per address in a quarter of an hour.
		loginLimit:  newLimiter(10, 15*time.Minute),
		mailLimit:   newLimiter(5, 15*time.Minute),
		publicLimit: newLimiter(30, 15*time.Minute),
	}
}

// Wait waits for the work started by requests that outlives them, as
// sending a recovery email, for a graceful shutdown.
func (s *Server) Wait() { s.bg.Wait() }

// Handler returns the server's routes, wrapped in its middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.Handle("POST /api/auth/logout", s.authed(s.logout))
	mux.Handle("GET /api/me", s.authed(s.me))
	mux.Handle("GET /api/periods", s.authed(s.periods))
	mux.Handle("GET /api/divisions", s.authed(s.divisions))

	// Accounts (Sesi 1B).
	mux.Handle("POST /api/auth/invitations/inspect", s.handlePublic(s.inspectInvitation))
	mux.Handle("POST /api/auth/invitations/verify", s.handlePublic(s.verifyInvitationEmail))
	mux.Handle("POST /api/auth/invitations/accept", s.handlePublic(s.acceptInvitation))
	mux.Handle("POST /api/auth/recovery", s.handlePublic(s.requestRecovery))
	mux.Handle("POST /api/auth/recovery/reset", s.handlePublic(s.resetPassword))
	mux.Handle("POST /api/auth/password", s.handle(s.changePassword))
	mux.Handle("POST /api/auth/admin-email", s.handle(s.requestAdminEmail))
	mux.Handle("POST /api/auth/admin-handover", s.handle(s.adminHandover))
	mux.Handle("GET /api/users", s.handle(s.listPeople))
	mux.Handle("POST /api/users/{userId}/disable", s.handle(s.disableUser))
	mux.Handle("POST /api/users/{userId}/enable", s.handle(s.enableUser))

	// Periods and handover.
	mux.Handle("POST /api/periods", s.handle(s.createPeriod))
	mux.Handle("POST /api/periods/{periodId}/activate", s.handle(s.activatePeriod))
	mux.Handle("POST /api/periods/{periodId}/close", s.handle(s.closePeriod))
	mux.Handle("GET /api/periods/{periodId}/handover", s.handle(s.getHandover))
	mux.Handle("PUT /api/periods/{periodId}/handover", s.handle(s.putHandover))

	// Spaces and the access of a division (Sesi 1C).
	mux.Handle("GET /api/periods/{periodId}/spaces", s.handle(s.periodSpaces))
	mux.Handle("GET /api/spaces/{spaceId}", s.handle(s.getSpace))
	const div = "/api/periods/{periodId}/divisions/{divisionId}"
	mux.Handle("GET "+div+"/access", s.handle(s.divisionAccess))
	mux.Handle("GET "+div+"/members", s.handle(s.listMembers))
	mux.Handle("PATCH "+div+"/members/{userId}", s.handle(s.updateMember))
	mux.Handle("DELETE "+div+"/members/{userId}", s.handle(s.removeMember))
	mux.Handle("PUT "+div+"/kadiv", s.handle(s.setKadiv))
	mux.Handle("GET "+div+"/invitations", s.handle(s.listInvitations))
	mux.Handle("POST "+div+"/invitations", s.handle(s.invite))
	mux.Handle("POST "+div+"/invitations/{invitationId}/resend", s.handle(s.resendInvitation))
	mux.Handle("DELETE "+div+"/invitations/{invitationId}", s.handle(s.revokeInvitation))
	mux.Handle("GET "+div+"/roles", s.handle(s.listRoles))
	mux.Handle("POST "+div+"/roles", s.handle(s.saveRole))
	mux.Handle("PUT "+div+"/roles/{roleId}", s.handle(s.saveRole))
	mux.Handle("DELETE "+div+"/roles/{roleId}", s.handle(s.deleteRole))
	mux.Handle("PUT "+div+"/45", s.handle(s.appoint45))
	mux.Handle("DELETE "+div+"/45", s.handle(s.remove45))
	mux.Handle("GET "+div+"/archives", s.handle(s.listArchives))
	mux.Handle("POST "+div+"/archives", s.handle(s.grantArchive))
	mux.Handle("DELETE "+div+"/archives/{grantId}", s.handle(s.revokeArchive))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, api.CodeNotFound, "Alamat API tidak dikenal.")
	})
	if s.cfg.WebDir != "" {
		mux.Handle("/", webApp(s.cfg.WebDir))
	}
	return s.middleware(mux)
}

type ctxKey int

const requestIDKey ctxKey = iota

// requestID returns the identity of the request, for the log and audit.
func requestID(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

// middleware adds a request ID, security headers, a body limit, recovery
// from panics and an access log.
func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		b := make([]byte, 8)
		rand.Read(b)
		rid := hex.EncodeToString(b)
		ctx := context.WithValue(r.Context(), requestIDKey, rid)
		r = r.WithContext(ctx)

		h := w.Header()
		h.Set("X-Request-Id", rid)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		}
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if p := recover(); p != nil {
				s.log.Error("panic", "request_id", rid, "panic", p, "stack", string(debug.Stack()))
				if !rec.wrote {
					writeError(rec, http.StatusInternalServerError, api.CodeInternal, "Terjadi kesalahan di server.")
				}
			}
			// The query is left out: it may carry tokens of links.
			s.log.Info("request", "request_id", rid, "method", r.Method, "path", r.URL.Path,
				"status", rec.status, "duration_ms", time.Since(start).Milliseconds())
		}()
		next.ServeHTTP(rec, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wrote {
		r.status, r.wrote = code, true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.wrote = true
	return r.ResponseWriter.Write(b)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	h := api.Health{Status: "ok", Database: "ok", Version: Version, Time: s.now().UTC()}
	status := http.StatusOK
	if err := s.store.Ping(ctx); err != nil {
		s.log.Warn("health: database", "err", err)
		h.Status, h.Database = "degraded", "unavailable"
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, h)
}

func (s *Server) periods(w http.ResponseWriter, r *http.Request, _ *principal) {
	ps, err := s.store.Periods(r.Context())
	if err != nil {
		s.internal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, api.List[api.Period]{Items: nonNil(ps)})
}

func (s *Server) divisions(w http.ResponseWriter, r *http.Request, _ *principal) {
	ds, err := s.store.Divisions(r.Context())
	if err != nil {
		s.internal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, api.List[api.Division]{Items: nonNil(ds)})
}

func nonNil[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}

// internal logs an unexpected error and answers without its details.
func (s *Server) internal(w http.ResponseWriter, r *http.Request, err error) {
	s.log.Error("internal error", "request_id", requestID(r.Context()), "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, api.CodeInternal, "Terjadi kesalahan di server.")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, api.ErrorResponse{Error: api.Error{Code: code, Message: message}})
}

// decode reads a JSON body into v, rejecting unknown fields and trailing
// data. It writes the error response itself and reports false on failure.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, api.CodeBadRequest, "Permintaan terlalu besar.")
		} else {
			writeError(w, http.StatusBadRequest, api.CodeBadRequest, "Isi permintaan tidak valid.")
		}
		return false
	}
	if dec.More() {
		writeError(w, http.StatusBadRequest, api.CodeBadRequest, "Isi permintaan tidak valid.")
		return false
	}
	return true
}

// webApp serves the built web app (mygo build -platform web). The
// WebAssembly is named by its content, so it is cached for good; the page
// that names it is revalidated each time.
func webApp(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		p := r.URL.Path
		switch {
		case p == "/" || p == "/index.html":
			w.Header().Set("Cache-Control", "no-cache")
		case strings.HasSuffix(p, ".wasm"):
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			w.Header().Set("Content-Type", "application/wasm")
			w.Header().Add("Vary", "Accept-Encoding")
			// The app's WebAssembly is some 18 MB, under 5 MB gzipped:
			// serve the copy that make web compresses beside it.
			gz := filepath.Join(dir, filepath.FromSlash(filepath.Clean("/"+p))) + ".gz"
			if acceptsGzip(r) {
				if f, err := os.Open(gz); err == nil {
					defer f.Close()
					if st, err := f.Stat(); err == nil && !st.IsDir() {
						w.Header().Set("Content-Encoding", "gzip")
						http.ServeContent(w, r, "", st.ModTime(), f)
						return
					}
				}
			}
		default:
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		// The router of the app keeps its pages in the URL's fragment, so
		// any other path that is not a file is the page itself.
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(filepath.Clean("/"+p)))); err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}

// acceptsGzip reports whether the client takes gzip-encoded responses: it
// names gzip in Accept-Encoding without refusing it with q=0.
func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		enc, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if !strings.EqualFold(strings.TrimSpace(enc), "gzip") {
			continue
		}
		q := strings.TrimSpace(params)
		if v, ok := strings.CutPrefix(q, "q="); ok {
			return strings.Trim(v, "0.") != "" // q=0, q=0.0 and q=0.000 refuse it
		}
		return true
	}
	return false
}
