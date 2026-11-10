package server

import (
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/guides"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
)

// getOnboarding returns the caller's onboarding progress. periodId, when
// given, is the period whose WhatsApp link counts.
func (s *Server) getOnboarding(w http.ResponseWriter, r *http.Request, p *principal) error {
	pid := id.Nil
	if v := r.URL.Query().Get("periodId"); v != "" {
		parsed, err := id.Parse(v)
		if err != nil {
			return badRequest("ID periode tidak valid.")
		}
		pid = parsed
	}
	ctx, db := r.Context(), s.store.DB()
	out, err := s.store.Onboarding(ctx, db, p.User.ID)
	if err != nil {
		return err
	}
	if out.WhatsAppLinked, err = s.store.WhatsAppLinked(ctx, db, p.User.ID, pid); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// markOnboarding records or clears one onboarding mark of the caller and
// returns the progress after it.
func (s *Server) markOnboarding(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.OnboardingMark
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if !slices.Contains(api.OnboardingKeys, req.Key) {
		return badRequest("Langkah onboarding tidak dikenal.")
	}
	var out api.Onboarding
	err := s.store.Tx(r.Context(), func(q store.Querier) error {
		if _, err := s.lockedAccount(r, q, p); err != nil {
			return err
		}
		ctx := r.Context()
		if err := s.store.SetOnboardingMark(ctx, q, p.User.ID, req.Key, req.Clear, s.now()); err != nil {
			return err
		}
		var err error
		if out, err = s.store.Onboarding(ctx, q, p.User.ID); err != nil {
			return err
		}
		out.WhatsAppLinked, err = s.store.WhatsAppLinked(ctx, q, p.User.ID, id.Nil)
		return err
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// updateProfile changes the caller's display name and records the
// onboarding step "Lengkapi profil".
func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.ProfileUpdate
	if err := readJSON(r, &req); err != nil {
		return err
	}
	name := strings.Join(strings.Fields(req.DisplayName), " ")
	if n := utf8.RuneCountInString(name); n < 2 || n > 80 {
		return badRequest("Nama tampilan 2 sampai 80 huruf.")
	}
	var out api.User
	err := s.store.Tx(r.Context(), func(q store.Querier) error {
		u, err := s.lockedAccount(r, q, p)
		if err != nil {
			return err
		}
		if p.admin() {
			return errForbidden
		}
		ctx, before := r.Context(), u.DisplayName
		if err := s.store.SetDisplayName(ctx, q, u.ID, name); err != nil {
			return err
		}
		if err := s.store.SetOnboardingMark(ctx, q, u.ID, api.OnboardProfile, false, s.now()); err != nil {
			return err
		}
		u.DisplayName = name
		out = u.API()
		return s.record(r, q, p, store.AuditEvent{Action: "account.profile.update", TargetType: "user", TargetID: &u.ID,
			Before: map[string]any{"displayName": before}, After: map[string]any{"displayName": name}})
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// listGuides returns the in-app guide: SPACE_GUIDES_DIR's files when set,
// read on each call so a changed file shows at once, or the built-in ones.
// A broken folder falls back to the built-in guide, with the error logged.
func (s *Server) listGuides(w http.ResponseWriter, r *http.Request, p *principal) error {
	out, err := guides.Load(s.cfg.GuidesDir)
	if err != nil {
		s.log.Warn("guides: folder unreadable, serving the built-in guide", "dir", s.cfg.GuidesDir, "err", err)
		out, _ = guides.Load("")
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}
