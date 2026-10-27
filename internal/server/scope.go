package server

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/authz"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
)

// scope is what the caller may do in a division of a period: the scope of
// the endpoints under /api/periods/{periodId}/divisions/{divisionId}. It is
// loaded on every request, in the request's transaction for writes, so a
// role removed a moment ago grants nothing (ACC-10).
type scope struct {
	period   api.Period
	division api.Division
	admin    bool
	member   *store.Membership // the caller's active membership, if any
	binding  authz.Binding
	caps     []string // effective in an open period
}

// loadScope loads the scope of r's path, reading the period with lock.
func (s *Server) loadScope(ctx context.Context, q store.Querier, r *http.Request, p *principal, lock string) (*scope, error) {
	pid, err := pathID(r, "periodId")
	if err != nil {
		return nil, err
	}
	did, err := pathID(r, "divisionId")
	if err != nil {
		return nil, err
	}
	period, err := s.store.Period(ctx, q, pid, lock)
	if err != nil {
		return nil, notFound(err, errNotFound)
	}
	div, err := s.store.PeriodDivision(ctx, q, pid, did)
	if err != nil {
		return nil, notFound(err, errNotFound)
	}
	sc := &scope{period: period, division: div, caps: []string{}}
	if p.admin() {
		sc.admin, sc.caps = true, authz.Normalize(authz.AdminCaps)
		return sc, nil
	}
	m, err := s.store.ActiveMembership(ctx, q, pid, did, p.User.ID)
	if errors.Is(err, store.ErrNotFound) {
		return sc, nil
	}
	if err != nil {
		return nil, err
	}
	roles, err := s.store.CustomRoles(ctx, q, pid, did)
	if err != nil {
		return nil, err
	}
	sc.member = &m
	sc.binding = bindingOf(m, rolesByID(roles))
	sc.caps = sc.binding.Capabilities()
	return sc, nil
}

func rolesByID(rs []api.CustomRole) map[id.ID]api.CustomRole {
	m := map[id.ID]api.CustomRole{}
	for _, r := range rs {
		m[r.ID] = r
	}
	return m
}

// bindingOf returns a membership as the rules of authz see it.
func bindingOf(m store.Membership, roles map[id.ID]api.CustomRole) authz.Binding {
	b := authz.Binding{Role: m.Role}
	for _, rid := range m.CustomRoleIDs {
		if r, ok := roles[rid]; ok {
			b.CustomCaps = append(b.CustomCaps, r.Capabilities)
		}
	}
	if m.Acting45 != nil {
		b.Acting45, b.Caps45 = true, m.Acting45.Capabilities
	}
	return b
}

func (sc *scope) closed() bool { return sc.period.Status == api.PeriodClosed }

// require fails unless the caller holds c here; a change in a closed
// period fails even with c.
func (sc *scope) require(c string) error {
	if !slices.Contains(sc.caps, c) {
		return errForbidden
	}
	if authz.IsWrite(c) && sc.closed() {
		return errPeriodClosed
	}
	return nil
}

// access returns the caller's access to the scope.
func (sc *scope) access() api.Access {
	a := api.Access{DivisionID: sc.division.ID, Roles: []string{}, CustomRoleIDs: []id.ID{},
		Capabilities: sc.caps, PeriodStatus: sc.period.Status, ReadOnly: sc.closed()}
	switch {
	case sc.admin:
		a.Roles = []string{api.RoleOrganizationAdmin}
	case sc.member != nil:
		a.Roles = sc.binding.Roles()
		a.CustomRoleIDs = nonNil(sc.member.CustomRoleIDs)
	}
	if a.ReadOnly {
		a.Capabilities = authz.ReadOnly(a.Capabilities)
	}
	return a
}

// memberAPI returns a membership as clients see it.
func memberAPI(m store.Membership, roles map[id.ID]api.CustomRole, closed bool) api.Member {
	b := bindingOf(m, roles)
	caps := b.Capabilities()
	if closed {
		caps = authz.ReadOnly(caps)
	}
	return api.Member{ID: m.ID, UserID: m.UserID, PeriodID: m.PeriodID, DivisionID: m.DivisionID,
		DisplayName: m.DisplayName, Role: m.Role, Roles: b.Roles(), Groups: nonNil(m.Groups),
		CustomRoleIDs: nonNil(m.CustomRoleIDs), Capabilities: caps}
}

// spaceViews returns the spaces of a period the caller may open, with the
// caller's access to each. A member opens the spaces of their divisions;
// an archive grant opens one space of a closed period, read-only; the
// organization admin opens all for administration. Anything else is left
// out.
func (s *Server) spaceViews(ctx context.Context, q store.Querier, p *principal, period api.Period, spaces []store.Space) ([]api.Space, error) {
	closed := period.Status == api.PeriodClosed
	finish := func(sp store.Space, access string, roles []api.Access, caps []string) api.Space {
		if closed {
			caps = authz.ReadOnly(caps)
		}
		if sp.Kind == api.SpaceShared {
			// What a shared space adds up to is operational only: access is
			// managed in each division, by its own binding.
			caps = slices.DeleteFunc(caps, func(c string) bool { return !slices.Contains(authz.Operational, c) })
		}
		return api.Space{ID: sp.ID, PeriodID: sp.PeriodID, Code: sp.Code, Name: sp.Name, Kind: sp.Kind,
			Divisions: sp.Divisions, Roles: roles, Capabilities: authz.Normalize(caps),
			PeriodStatus: period.Status, ReadOnly: closed, Access: access}
	}
	out := []api.Space{}
	if p.admin() {
		for _, sp := range spaces {
			var roles []api.Access
			for _, d := range sp.Divisions {
				caps := authz.Normalize(authz.AdminCaps)
				if closed {
					caps = authz.ReadOnly(caps)
				}
				roles = append(roles, api.Access{DivisionID: d.ID, Roles: []string{api.RoleOrganizationAdmin},
					CustomRoleIDs: []id.ID{}, Capabilities: caps, PeriodStatus: period.Status, ReadOnly: closed})
			}
			out = append(out, finish(sp, api.AccessAdmin, roles, authz.AdminCaps))
		}
		return out, nil
	}
	ms, err := s.store.UserMemberships(ctx, q, p.User.ID, period.ID)
	if err != nil {
		return nil, err
	}
	roles, err := s.store.PeriodCustomRoles(ctx, q, period.ID)
	if err != nil {
		return nil, err
	}
	byDivision := map[id.ID]store.Membership{}
	for _, m := range ms {
		byDivision[m.DivisionID] = m
	}
	var archived map[id.ID]bool
	if closed {
		if archived, err = s.store.ArchiveSpaces(ctx, q, p.User.ID, period.ID); err != nil {
			return nil, err
		}
	}
	for _, sp := range spaces {
		var bindings []api.Access
		var caps []string
		for _, d := range sp.Divisions {
			m, ok := byDivision[d.ID]
			if !ok {
				continue
			}
			b := bindingOf(m, roles)
			c := b.Capabilities()
			caps = append(caps, c...)
			if closed {
				c = authz.ReadOnly(c)
			}
			bindings = append(bindings, api.Access{DivisionID: d.ID, Roles: b.Roles(), CustomRoleIDs: nonNil(m.CustomRoleIDs),
				Capabilities: c, PeriodStatus: period.Status, ReadOnly: closed})
		}
		switch {
		case len(bindings) > 0:
			out = append(out, finish(sp, api.AccessMember, bindings, caps))
		case archived[sp.ID]:
			out = append(out, finish(sp, api.AccessArchive, []api.Access{}, authz.ArchiveCaps))
		}
	}
	return out, nil
}
