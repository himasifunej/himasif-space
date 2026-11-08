package server

import (
	"net/http"
	"regexp"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
)

// Bounds of the Space tab preferences (Tahap 5c): enough for every Space an
// account has across periods and every tab, small enough to stay a row.
const (
	maxUISpaces  = 200
	maxUITabKeys = 24
)

var uiTabKey = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// getUIPreferences returns the caller's interface preferences, the same on
// every device they sign in to.
func (s *Server) getUIPreferences(w http.ResponseWriter, r *http.Request, p *principal) error {
	out, err := s.store.UIPreferences(r.Context(), s.store.DB(), p.User.ID)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// putUIPreferences replaces the caller's interface preferences. Tab keys
// are checked for shape only: a tab the account may not open now keeps its
// place, as the client does.
func (s *Server) putUIPreferences(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.UIPreferences
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if err := validUIPreferences(req); err != nil {
		return err
	}
	var out api.UIPreferences
	err := s.store.Tx(r.Context(), func(q store.Querier) error {
		if _, err := s.lockedAccount(r, q, p); err != nil {
			return err
		}
		ctx, now := r.Context(), s.now()
		before, err := s.store.UIPreferences(ctx, q, p.User.ID)
		if err != nil {
			return err
		}
		if err := s.store.SetUIPreferences(ctx, q, p.User.ID, req, now); err != nil {
			return err
		}
		out = api.UIPreferences{SpaceTabs: req.SpaceTabs, UpdatedAt: &now}
		if out.SpaceTabs == nil {
			out.SpaceTabs = map[string]api.SpaceTabPreference{}
		}
		return s.record(r, q, p, store.AuditEvent{Action: "ui.preferences.set", TargetType: "user", TargetID: &p.User.ID,
			Before: map[string]any{"spaces": len(before.SpaceTabs)}, After: map[string]any{"spaces": len(out.SpaceTabs)}})
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func validUIPreferences(p api.UIPreferences) error {
	if len(p.SpaceTabs) > maxUISpaces {
		return badRequest("Terlalu banyak Space dalam preferensi tab.")
	}
	for sid, tabs := range p.SpaceTabs {
		if _, err := id.Parse(sid); err != nil {
			return badRequest("ID Space dalam preferensi tab tidak valid.")
		}
		for _, keys := range [][]string{tabs.Order, tabs.Hidden} {
			if len(keys) > maxUITabKeys {
				return badRequest("Terlalu banyak tab dalam preferensi tab.")
			}
			seen := map[string]bool{}
			for _, k := range keys {
				if !uiTabKey.MatchString(k) || seen[k] {
					return badRequest("Nama tab dalam preferensi tab tidak valid.")
				}
				seen[k] = true
			}
		}
	}
	return nil
}
