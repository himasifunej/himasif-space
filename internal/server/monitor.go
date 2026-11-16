package server

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
)

// Organization monitoring by BPH (OPEN-15, owner's decisions of 16 Nov 2026):
//
//  1. org.monitor is granted per account and per period by the organization
//     admin, to members of BPH only; a successor does not inherit it.
//  2. It opens the work of every division in full (authz.MonitorCaps);
//     personal reminders and drafts stay closed, since they are never
//     division data.
//  3. It opens other divisions' meeting notes.
//  4. Divisions are not notified when a monitor opens them, but every read
//     is written to the audit log, which the admin reads.
//  5. A monitor may approve a submission where a kadiv could, and ping the
//     kadiv; both are logged as the monitor's.
//  6. Closing the period revokes every grant of it.
//
// The server is the only judge: monitor access is added where every other
// access is computed (spaceViews and loadScope), and only for API requests.

// monitorState is org.monitor within one request: the grants looked up so
// far and the divisions the request opened through them.
type monitorState struct {
	mu       sync.Mutex
	active   map[id.ID]bool           // period → holds org.monitor
	opened   map[id.ID]map[id.ID]bool // period → divisions opened
	recorded bool                     // a monitor.act was written in the request's transaction
}

// monitorActive reports whether p holds org.monitor in period, looking it
// up once per request. Requests that are not the API's never do.
func (s *Server) monitorActive(ctx context.Context, q store.Querier, p *principal, period api.Period) (bool, error) {
	if p == nil || p.monitor == nil || p.admin() || period.Status == api.PeriodClosed {
		return false, nil
	}
	m := p.monitor
	m.mu.Lock()
	ok, known := m.active[period.ID]
	m.mu.Unlock()
	if known {
		return ok, nil
	}
	ok, err := s.store.MonitorActive(ctx, q, period.ID, p.User.ID)
	if err != nil {
		return false, err
	}
	m.mu.Lock()
	if m.active == nil {
		m.active = map[id.ID]bool{}
	}
	m.active[period.ID] = ok
	m.mu.Unlock()
	return ok, nil
}

// monitorOpened notes that the request opened divisions of a period
// through org.monitor; the request's end writes it to the audit log.
func (p *principal) monitorOpened(period id.ID, divisions ...api.Division) {
	if p == nil || p.monitor == nil || len(divisions) == 0 {
		return
	}
	m := p.monitor
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.opened == nil {
		m.opened = map[id.ID]map[id.ID]bool{}
	}
	if m.opened[period] == nil {
		m.opened[period] = map[id.ID]bool{}
	}
	for _, d := range divisions {
		m.opened[period][d.ID] = true
	}
}

// monitorOpenings returns what the request opened through org.monitor, by
// period, divisions sorted.
func (p *principal) monitorOpenings() map[id.ID][]id.ID {
	if p == nil || p.monitor == nil {
		return nil
	}
	m := p.monitor
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[id.ID][]id.ID{}
	for period, ds := range m.opened {
		for d := range ds {
			out[period] = append(out[period], d)
		}
		slices.SortFunc(out[period], func(a, b id.ID) int { return strings.Compare(a.String(), b.String()) })
	}
	return out
}

// monitorDetail names what a request read or did, for the admin's log.
func monitorDetail(r *http.Request) string {
	route := r.Pattern
	if _, rest, ok := strings.Cut(route, " "); ok {
		route = rest
	}
	what := "Space"
	for _, w := range []struct{ part, name string }{
		{"/monitor", "dashboard pemantauan"}, {"/evidence", "bukti tugas"}, {"/attachments", "lampiran"},
		{"/pings", "ping"}, {"/tasks", "tugas"}, {"/notes", "catatan"}, {"/note-templates", "template catatan"},
		{"/meetings", "rapat"}, {"/sop", "SOP"}, {"/members", "anggota"}, {"/contents", "konten Medhum"},
		{"/wa-content", "konten WhatsApp"}, {"/access", "akses"},
	} {
		if strings.Contains(route, w.part) {
			what = w.name
			break
		}
	}
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return "Membaca " + what
	}
	return "Bertindak: " + what
}

// recordMonitorAct writes, in the transaction of an action, that a monitor
// did it, once per division it reached through org.monitor. record calls it
// with every audit event of a request.
func (s *Server) recordMonitorAct(r *http.Request, q store.Querier, p *principal, action string) error {
	if p == nil || p.monitor == nil || strings.HasPrefix(action, "monitor.") {
		return nil
	}
	openings := p.monitorOpenings()
	if len(openings) == 0 {
		return nil
	}
	p.monitor.mu.Lock()
	done := p.monitor.recorded
	p.monitor.recorded = true
	p.monitor.mu.Unlock()
	if done {
		return nil
	}
	uid, kind := p.actor()
	for period, ds := range openings {
		for _, d := range ds {
			pid, did := period, d
			if err := s.store.Audit(r.Context(), q, store.AuditEvent{ActorUserID: uid, ActorKind: kind, Action: "monitor.act", Source: "api",
				PeriodID: &pid, ScopeType: "division", ScopeID: &did, RequestID: requestID(r.Context()), OccurredAt: s.now(),
				After: map[string]any{"detail": monitorDetail(r) + " (" + action + ")", "method": r.Method, "route": r.Pattern, "path": r.URL.Path, "action": action}}); err != nil {
				return err
			}
		}
	}
	return nil
}

// auditMonitor writes, after a request succeeded, every division it opened
// through org.monitor: monitor.read for a read, monitor.act for an action
// whose transaction did not record it already.
func (s *Server) auditMonitor(r *http.Request, p *principal) {
	openings := p.monitorOpenings()
	if len(openings) == 0 {
		return
	}
	action := "monitor.read"
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		p.monitor.mu.Lock()
		done := p.monitor.recorded
		p.monitor.mu.Unlock()
		if done {
			return
		}
		action = "monitor.act"
	}
	uid, kind := p.actor()
	for period, ds := range openings {
		for _, d := range ds {
			pid, did := period, d
			e := store.AuditEvent{ActorUserID: uid, ActorKind: kind, Action: action, Source: "api", PeriodID: &pid,
				ScopeType: "division", ScopeID: &did, RequestID: requestID(r.Context()), OccurredAt: s.now(),
				After: map[string]any{"detail": monitorDetail(r), "method": r.Method, "route": r.Pattern, "path": r.URL.Path}}
			if err := s.store.Audit(context.WithoutCancel(r.Context()), s.store.DB(), e); err != nil {
				s.log.Error("audit monitor read", "request_id", requestID(r.Context()), "err", err)
			}
		}
	}
}

// requireMonitor fails unless p holds org.monitor in the period of r.
func (s *Server) requireMonitor(ctx context.Context, q store.Querier, r *http.Request, p *principal) (api.Period, error) {
	pid, err := pathID(r, "periodId")
	if err != nil {
		return api.Period{}, err
	}
	period, err := s.store.Period(ctx, q, pid, store.LockNone)
	if err != nil {
		return api.Period{}, notFound(err, errNotFound)
	}
	ok, err := s.monitorActive(ctx, q, p, period)
	if err != nil {
		return period, err
	}
	if !ok {
		return period, errForbidden
	}
	return period, nil
}

// listMonitorGrants is the admin's view of monitoring in a period: the BPH
// members, who of them holds org.monitor, and the grants.
func (s *Server) listMonitorGrants(w http.ResponseWriter, r *http.Request, p *principal) error {
	if !p.admin() {
		return errForbidden
	}
	pid, err := pathID(r, "periodId")
	if err != nil {
		return err
	}
	ctx, q := r.Context(), s.store.DB()
	if _, err := s.store.Period(ctx, q, pid, store.LockNone); err != nil {
		return notFound(err, errNotFound)
	}
	grants, err := s.store.MonitorGrants(ctx, q, pid)
	if err != nil {
		return err
	}
	bph, err := s.store.BPHMemberships(ctx, q, pid)
	if err != nil {
		return err
	}
	out := api.MonitorGrants{PeriodID: pid, Grants: nonNil(grants), Candidates: []api.MonitorCandidate{}}
	for _, m := range bph {
		c := api.MonitorCandidate{UserID: m.UserID, DisplayName: m.DisplayName, Role: m.Role}
		for i := range grants {
			if grants[i].UserID == m.UserID {
				g := grants[i]
				c.Grant = &g
			}
		}
		out.Candidates = append(out.Candidates, c)
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// grantMonitor gives org.monitor to a BPH member for the period.
func (s *Server) grantMonitor(w http.ResponseWriter, r *http.Request, p *principal) error {
	if !p.admin() {
		return errForbidden
	}
	pid, err := pathID(r, "periodId")
	if err != nil {
		return err
	}
	var req api.MonitorGrantRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if req.UserID == id.Nil {
		return badRequest("Pilih akun BPH yang diberi izin pemantauan.")
	}
	var out api.MonitorGrant
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		ctx := r.Context()
		period, err := s.store.Period(ctx, q, pid, store.LockUpdate)
		if err != nil {
			return notFound(err, errNotFound)
		}
		if period.Status == api.PeriodClosed {
			return errPeriodClosed
		}
		bph, err := s.store.BPHMemberships(ctx, q, pid)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(bph, func(m store.Membership) bool { return m.UserID == req.UserID })
		if i < 0 {
			return badRequest("Izin pemantauan hanya untuk anggota BPH periode ini.")
		}
		out = api.MonitorGrant{ID: id.New(), PeriodID: pid, UserID: req.UserID, DisplayName: bph[i].DisplayName, GrantedBy: p.User.ID, GrantedAt: s.now()}
		if err := s.store.CreateMonitorGrant(ctx, q, out); errors.Is(err, store.ErrConflict) {
			return conflict("Akun ini sudah memegang izin pemantauan pada periode ini.")
		} else if err != nil {
			return err
		}
		uid := req.UserID
		return s.record(r, q, p, store.AuditEvent{Action: "monitor.grant", PeriodID: &pid, TargetType: "user", TargetID: &uid,
			After: map[string]any{"detail": "Admin memberi izin pemantauan", "grant": out.ID, "capability": api.CapOrgMonitor}})
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}

// revokeMonitor ends a grant before the period closes.
func (s *Server) revokeMonitor(w http.ResponseWriter, r *http.Request, p *principal) error {
	if !p.admin() {
		return errForbidden
	}
	pid, err := pathID(r, "periodId")
	if err != nil {
		return err
	}
	gid, err := pathID(r, "grantId")
	if err != nil {
		return err
	}
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		ctx := r.Context()
		if _, err := s.store.Period(ctx, q, pid, store.LockUpdate); err != nil {
			return notFound(err, errNotFound)
		}
		g, err := s.store.MonitorGrant(ctx, q, pid, gid)
		if err != nil {
			return notFound(err, errNotFound)
		}
		if err := s.store.RevokeMonitorGrant(ctx, q, gid, p.User.ID, api.MonitorRevokedAdmin, s.now()); err != nil {
			return err
		}
		uid := g.UserID
		return s.record(r, q, p, store.AuditEvent{Action: "monitor.revoke", PeriodID: &pid, TargetType: "user", TargetID: &uid,
			Before: map[string]any{"grant": g.ID}, After: map[string]any{"detail": "Admin mencabut izin pemantauan", "reason": api.MonitorRevokedAdmin}})
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// revokeMonitorsOnClose ends every grant of a period being closed, in the
// transaction of the close, and logs each.
func (s *Server) revokeMonitorsOnClose(r *http.Request, q store.Querier, p *principal, pid id.ID) error {
	ended, err := s.store.RevokePeriodMonitors(r.Context(), q, pid, p.User.ID, s.now())
	if err != nil {
		return err
	}
	for _, g := range ended {
		uid := g.UserID
		if err := s.record(r, q, p, store.AuditEvent{Action: "monitor.revoke", PeriodID: &pid, TargetType: "user", TargetID: &uid,
			Before: map[string]any{"grant": g.ID}, After: map[string]any{"detail": "Dicabut otomatis: periode ditutup", "reason": api.MonitorRevokedClosed}}); err != nil {
			return err
		}
	}
	return nil
}

// monitorAudit is the admin's log of monitoring in a period.
func (s *Server) monitorAudit(w http.ResponseWriter, r *http.Request, p *principal) error {
	if !p.admin() {
		return errForbidden
	}
	pid, err := pathID(r, "periodId")
	if err != nil {
		return err
	}
	if _, err := s.store.Period(r.Context(), s.store.DB(), pid, store.LockNone); err != nil {
		return notFound(err, errNotFound)
	}
	events, err := s.store.MonitorAudit(r.Context(), s.store.DB(), pid, 300)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, api.List[api.MonitorAuditEvent]{Items: nonNil(events)})
	return nil
}
