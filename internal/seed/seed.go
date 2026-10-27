// Package seed puts the organization's starting data into an empty
// database: the current period, the divisions and the organization admin
// account. Running it again changes nothing that exists.
package seed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/auth"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// Divisions are HIMASIF's divisions, by slug.
var Divisions = []struct{ Slug, Name string }{
	{"bph", "BPH"},
	{"humas", "Humas"},
	{"kader", "Kader"},
	{"litbang", "Litbang"},
	{"mediatek", "Mediatek"},
	{"psdm", "PSDM"},
}

// FirstPeriod is the period running when Space starts: December 2025 to
// the end of November 2026, as the user recalled it; correct it in the
// app once the dates are confirmed.
var FirstPeriod = api.Period{
	Label:    "2025/2026",
	StartsOn: timeutil.Date{Year: 2025, Month: time.December, Day: 1},
	EndsOn:   timeutil.Date{Year: 2026, Month: time.November, Day: 30},
	TimeZone: timeutil.DefaultZoneName,
	Status:   api.PeriodActive,
}

// Options are what Run needs beyond the store.
type Options struct {
	AdminEmail    string
	AdminPassword string
}

// Report says what Run added.
type Report struct {
	Period    bool
	Divisions []string
	Spaces    []string
	Admin     bool
}

// Run adds what is missing, in one transaction.
func Run(ctx context.Context, st *store.Store, opt Options) (Report, error) {
	var rep Report
	adminHash := ""
	if _, err := st.OrganizationAdmin(ctx); errors.Is(err, store.ErrNotFound) {
		if opt.AdminEmail == "" || opt.AdminPassword == "" {
			return rep, errors.New("seed: SPACE_ADMIN_EMAIL and SPACE_ADMIN_PASSWORD are needed to create the organization admin")
		}
		if msg := auth.CheckPassword(opt.AdminPassword); msg != "" {
			return rep, fmt.Errorf("seed: SPACE_ADMIN_PASSWORD: %s", msg)
		}
		h, err := auth.HashPassword(opt.AdminPassword)
		if err != nil {
			return rep, err
		}
		adminHash = h
	} else if err != nil {
		return rep, err
	}

	err := st.Tx(ctx, func(q store.Querier) error {
		system := store.AuditEvent{ActorKind: "system", Source: "cli"}
		var divisionIDs []id.ID
		if _, err := st.PeriodByLabelQ(ctx, q, FirstPeriod.Label); errors.Is(err, store.ErrNotFound) {
			p, err := st.CreatePeriod(ctx, q, FirstPeriod)
			if err != nil {
				return err
			}
			e := system
			e.Action, e.TargetType, e.TargetID, e.After = "period.create", "period", &p.ID, p
			if err := st.Audit(ctx, q, e); err != nil {
				return err
			}
			rep.Period = true
		} else if err != nil {
			return err
		}
		for _, d := range Divisions {
			div, added, err := st.EnsureDivision(ctx, q, d.Slug, d.Name)
			if err != nil {
				return err
			}
			divisionIDs = append(divisionIDs, div.ID)
			if added {
				e := system
				e.Action, e.TargetType, e.TargetID, e.After = "division.create", "division", &div.ID, div
				if err := st.Audit(ctx, q, e); err != nil {
					return err
				}
				rep.Divisions = append(rep.Divisions, d.Slug)
			}
		}
		// The first period gets the divisions and their spaces once: when it
		// has none yet, as a database seeded before Sesi 1B.
		first, err := st.PeriodByLabelQ(ctx, q, FirstPeriod.Label)
		if err != nil {
			return err
		}
		if divs, err := st.PeriodDivisions(ctx, q, first.ID); err != nil {
			return err
		} else if len(divs) == 0 {
			spaces, err := st.EnsurePeriodStructure(ctx, q, first.ID, divisionIDs)
			if err != nil {
				return err
			}
			for _, sp := range spaces {
				e := system
				e.Action, e.PeriodID, e.TargetType, e.TargetID = "space.create", &first.ID, "space", &sp.ID
				e.After = map[string]any{"code": sp.Code, "name": sp.Name, "kind": sp.Kind}
				if err := st.Audit(ctx, q, e); err != nil {
					return err
				}
				rep.Spaces = append(rep.Spaces, sp.Code)
			}
		}
		if adminHash != "" {
			now := time.Now()
			u, err := st.CreateUser(ctx, q, store.User{
				Kind:         api.AccountOrganizationAdmin,
				Email:        opt.AdminEmail,
				DisplayName:  "Admin Organisasi HIMASIF",
				PasswordHash: adminHash,
				// The seeding operator vouches for the address; the email
				// flow arrives with invitations (Sesi 1B).
				EmailVerifiedAt: &now,
			})
			if err != nil {
				return err
			}
			e := system
			e.Action, e.TargetType, e.TargetID = "user.create", "user", &u.ID
			e.After = map[string]any{"kind": u.Kind, "email": u.Email}
			if err := st.Audit(ctx, q, e); err != nil {
				return err
			}
			rep.Admin = true
		}
		return nil
	})
	return rep, err
}
