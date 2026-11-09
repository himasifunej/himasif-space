package server

import (
	"errors"
	"net/http"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/config"
	"github.com/himasifunej/himasif-space/internal/mail"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
)

type blockedOutbound struct{}

func (blockedOutbound) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("external transport is disabled in this runtime mode")
}
func (s *Server) configureRuntime() {
	if s.cfg.OutboundEnabled() {
		return
	}
	s.sources.HTTP.Transport = blockedOutbound{}
	// Keep a non-nil protocol object so management reads return a precise error.
	s.wa = whatsapp.Disabled("Transport WhatsApp dinonaktifkan pada mode pemeliharaan/restore terisolasi.")
	s.mailer = mail.Disabled()
}
func (s *Server) runtimeHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mode := s.cfg.RunMode
		if mode == "" {
			mode = config.RunLive
		}
		w.Header().Set("X-Space-Run-Mode", mode)
		if mode == config.RunMaintenance && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			writeError(w, http.StatusServiceUnavailable, api.CodeUnavailable, "Server sedang dalam mode pemeliharaan. Perubahan belum diterapkan; simpan draf perangkat dan coba setelah pemeliharaan.")
			return
		}
		next.ServeHTTP(w, r)
	})
}
