package store

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// ErrConflict is what writes return when a uniqueness rule refuses them:
// a second 45, kadiv, open invitation, membership or role name.
var ErrConflict = errors.New("store: conflict")

// Lock modes of rows read in a transaction. A write in a period reads the
// period with LockShare, so closing it (LockUpdate) waits for the write,
// and the write after the close sees it closed.
const (
	LockNone   = ""
	LockShare  = " for share"
	LockUpdate = " for update"
)

const periodColumns = `id, label, starts_on, ends_on, time_zone, status`

func scanPeriod(row pgx.Row) (api.Period, error) {
	var p api.Period
	err := row.Scan(&p.ID, &p.Label, &p.StartsOn, &p.EndsOn, &p.TimeZone, &p.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

// Period returns a period, locked as lock says.
func (s *Store) Period(ctx context.Context, q Querier, pid id.ID, lock string) (api.Period, error) {
	return scanPeriod(q.QueryRow(ctx, `select `+periodColumns+` from periods where id = $1`+lock, pid))
}

// SetPeriodStatus changes the status of a period; closing records when.
func (s *Store) SetPeriodStatus(ctx context.Context, q Querier, pid id.ID, status string, now time.Time) (api.Period, error) {
	var closed *time.Time
	if status == api.PeriodClosed {
		closed = &now
	}
	return scanPeriod(q.QueryRow(ctx, `update periods set status = $2, closed_at = $3 where id = $1 returning `+periodColumns, pid, status, closed))
}

// PeriodDivisions returns the divisions of a period, by name.
func (s *Store) PeriodDivisions(ctx context.Context, q Querier, pid id.ID) ([]api.Division, error) {
	rows, err := q.Query(ctx, `select d.id, d.slug, d.name from period_divisions pd join divisions d on d.id = pd.division_id
		where pd.period_id = $1 order by d.name`, pid)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanDivision)
}

func scanDivision(r pgx.CollectableRow) (api.Division, error) {
	var d api.Division
	err := r.Scan(&d.ID, &d.Slug, &d.Name)
	return d, err
}

// PeriodDivision returns a division of a period, ErrNotFound when the
// period does not have it.
func (s *Store) PeriodDivision(ctx context.Context, q Querier, pid, did id.ID) (api.Division, error) {
	var d api.Division
	err := q.QueryRow(ctx, `select d.id, d.slug, d.name from period_divisions pd join divisions d on d.id = pd.division_id
		where pd.period_id = $1 and pd.division_id = $2`, pid, did).Scan(&d.ID, &d.Slug, &d.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrNotFound
	}
	return d, err
}

// Space is a space of a period with its divisions.
type Space struct {
	ID        id.ID
	PeriodID  id.ID
	Code      string
	Name      string
	Kind      string
	Divisions []api.Division
}

// HasDivision reports whether d is one of the space's divisions.
func (sp Space) HasDivision(d id.ID) bool {
	return slices.ContainsFunc(sp.Divisions, func(x api.Division) bool { return x.ID == d })
}

// EnsurePeriodStructure gives a period the divisions and adds the spaces
// they need that are missing: one per division, and Medhum when the period
// has both Humas and Mediatek. It returns the spaces it added. Nothing is
// copied from another period.
func (s *Store) EnsurePeriodStructure(ctx context.Context, q Querier, pid id.ID, divisionIDs []id.ID) ([]Space, error) {
	for _, d := range divisionIDs {
		if _, err := q.Exec(ctx, `insert into period_divisions (period_id, division_id) values ($1, $2) on conflict do nothing`, pid, d); err != nil {
			return nil, err
		}
	}
	divs, err := s.PeriodDivisions(ctx, q, pid)
	if err != nil {
		return nil, err
	}
	var added []Space
	add := func(code, name, kind string, members []api.Division) error {
		sp := Space{ID: id.New(), PeriodID: pid, Code: code, Name: name, Kind: kind, Divisions: members}
		tag, err := q.Exec(ctx, `insert into spaces (id, period_id, code, name, kind) values ($1, $2, $3, $4, $5)
			on conflict (period_id, code) do nothing`, sp.ID, pid, code, name, kind)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		for _, d := range members {
			if _, err := q.Exec(ctx, `insert into space_divisions (space_id, period_id, division_id) values ($1, $2, $3)`, sp.ID, pid, d.ID); err != nil {
				return err
			}
		}
		added = append(added, sp)
		return nil
	}
	var humas, mediatek *api.Division
	for i, d := range divs {
		if err := add("division-"+d.Slug, d.Name, api.SpaceDivision, []api.Division{d}); err != nil {
			return nil, err
		}
		switch d.Slug {
		case "humas":
			humas = &divs[i]
		case "mediatek":
			mediatek = &divs[i]
		}
	}
	if humas != nil && mediatek != nil {
		if err := add("medhum", "Medhum", api.SpaceShared, []api.Division{*humas, *mediatek}); err != nil {
			return nil, err
		}
	}
	return added, nil
}

// Spaces returns the spaces of a period, division spaces by name first.
func (s *Store) Spaces(ctx context.Context, q Querier, pid id.ID) ([]Space, error) {
	return s.spaces(ctx, q, `s.period_id = $1`, pid)
}

// SpaceByID returns a space.
func (s *Store) SpaceByID(ctx context.Context, q Querier, sid id.ID) (Space, error) {
	sps, err := s.spaces(ctx, q, `s.id = $1`, sid)
	if err != nil {
		return Space{}, err
	}
	if len(sps) == 0 {
		return Space{}, ErrNotFound
	}
	return sps[0], nil
}

func (s *Store) spaces(ctx context.Context, q Querier, where string, args ...any) ([]Space, error) {
	rows, err := q.Query(ctx, `select s.id, s.period_id, s.code, s.name, s.kind, d.id, d.slug, d.name
		from spaces s join space_divisions sd on sd.space_id = s.id join divisions d on d.id = sd.division_id
		where `+where+` order by s.kind, s.name, s.id, d.name`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Space
	for rows.Next() {
		var sp Space
		var d api.Division
		if err := rows.Scan(&sp.ID, &sp.PeriodID, &sp.Code, &sp.Name, &sp.Kind, &d.ID, &d.Slug, &d.Name); err != nil {
			return nil, err
		}
		if n := len(out); n > 0 && out[n-1].ID == sp.ID {
			out[n-1].Divisions = append(out[n-1].Divisions, d)
			continue
		}
		sp.Divisions = []api.Division{d}
		out = append(out, sp)
	}
	return out, rows.Err()
}

// Membership is an active membership, with its custom roles and the 45
// appointment of its holder, if any.
type Membership struct {
	ID            id.ID
	PeriodID      id.ID
	DivisionID    id.ID
	UserID        id.ID
	DisplayName   string
	Role          string
	Groups        []string
	CustomRoleIDs []id.ID
	Acting45      *Appointment45
}

// Appointment45 is an active 45 appointment.
type Appointment45 struct {
	ID           id.ID
	PeriodID     id.ID
	DivisionID   id.ID
	UserID       id.ID
	Capabilities []string
	AppointedBy  id.ID
	AppointedAt  time.Time
}

// API returns the appointment as clients see it.
func (a Appointment45) API() api.Appointment45 {
	return api.Appointment45{ID: a.ID, PeriodID: a.PeriodID, DivisionID: a.DivisionID, UserID: a.UserID,
		Capabilities: a.Capabilities, AppointedAt: a.AppointedAt}
}

func (s *Store) memberships(ctx context.Context, q Querier, where string, args ...any) ([]Membership, error) {
	rows, err := q.Query(ctx, `select m.id, m.period_id, m.division_id, m.user_id, u.display_name, m.role, m.groups,
			coalesce((select array_agg(mcr.role_id order by mcr.role_id) from membership_custom_roles mcr where mcr.membership_id = m.id), '{}'),
			a.id, a.capabilities, a.appointed_by, a.appointed_at
		from memberships m join users u on u.id = m.user_id
		left join acting45 a on a.period_id = m.period_id and a.division_id = m.division_id and a.user_id = m.user_id and a.removed_at is null
		where m.removed_at is null and `+where+` order by lower(u.display_name), m.id`, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Membership, error) {
		var m Membership
		var aID *id.ID
		var aCaps []string
		var aBy *id.ID
		var aAt *time.Time
		err := r.Scan(&m.ID, &m.PeriodID, &m.DivisionID, &m.UserID, &m.DisplayName, &m.Role, &m.Groups, &m.CustomRoleIDs,
			&aID, &aCaps, &aBy, &aAt)
		if aID != nil {
			m.Acting45 = &Appointment45{ID: *aID, PeriodID: m.PeriodID, DivisionID: m.DivisionID, UserID: m.UserID,
				Capabilities: aCaps, AppointedBy: *aBy, AppointedAt: *aAt}
		}
		return m, err
	})
}

func one(ms []Membership, err error) (Membership, error) {
	if err != nil {
		return Membership{}, err
	}
	if len(ms) == 0 {
		return Membership{}, ErrNotFound
	}
	return ms[0], nil
}

// ActiveMembership returns the active membership of a user in a division
// of a period.
func (s *Store) ActiveMembership(ctx context.Context, q Querier, pid, did, uid id.ID) (Membership, error) {
	return one(s.memberships(ctx, q, `m.period_id = $1 and m.division_id = $2 and m.user_id = $3`, pid, did, uid))
}

// Kadiv returns the membership of a division's kadiv in a period.
func (s *Store) Kadiv(ctx context.Context, q Querier, pid, did id.ID) (Membership, error) {
	return one(s.memberships(ctx, q, `m.period_id = $1 and m.division_id = $2 and m.role = 'kadiv'`, pid, did))
}

// DivisionMemberships returns the active members of a division in a period.
func (s *Store) DivisionMemberships(ctx context.Context, q Querier, pid, did id.ID) ([]Membership, error) {
	return s.memberships(ctx, q, `m.period_id = $1 and m.division_id = $2`, pid, did)
}

// UserMemberships returns a user's active memberships in a period.
func (s *Store) UserMemberships(ctx context.Context, q Querier, uid, pid id.ID) ([]Membership, error) {
	return s.memberships(ctx, q, `m.user_id = $1 and m.period_id = $2`, uid, pid)
}

// CreateMembership adds an active membership; ErrConflict when the user is
// already a member, or the division already has a kadiv.
func (s *Store) CreateMembership(ctx context.Context, q Querier, pid, did, uid id.ID, role, source string, invitationID *id.ID, now time.Time) (id.ID, error) {
	mid := id.New()
	_, err := q.Exec(ctx, `insert into memberships (id, period_id, division_id, user_id, role, source, invitation_id, created_at, updated_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $8)`, mid, pid, did, uid, role, source, invitationID, now)
	if isUniqueViolation(err) {
		return mid, ErrConflict
	}
	return mid, err
}

// SetMembershipRole changes the built-in role of a membership.
func (s *Store) SetMembershipRole(ctx context.Context, q Querier, mid id.ID, role string, now time.Time) error {
	_, err := q.Exec(ctx, `update memberships set role = $2, updated_at = $3 where id = $1`, mid, role, now)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

// ConfigureMembership replaces the role, groups and custom roles of a
// membership.
func (s *Store) ConfigureMembership(ctx context.Context, q Querier, mid id.ID, role string, groups []string, roleIDs []id.ID, now time.Time) error {
	if _, err := q.Exec(ctx, `update memberships set role = $2, groups = $3, updated_at = $4 where id = $1`, mid, role, groups, now); err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `delete from membership_custom_roles where membership_id = $1`, mid); err != nil {
		return err
	}
	for _, r := range roleIDs {
		if _, err := q.Exec(ctx, `insert into membership_custom_roles (membership_id, role_id) values ($1, $2)`, mid, r); err != nil {
			return err
		}
	}
	return nil
}

// RemoveMembership ends a membership, keeping its row for history.
func (s *Store) RemoveMembership(ctx context.Context, q Querier, mid id.ID, now time.Time) error {
	_, err := q.Exec(ctx, `update memberships set removed_at = $2, updated_at = $2 where id = $1 and removed_at is null`, mid, now)
	return err
}

const customRoleColumns = `id, period_id, division_id, name, capabilities`

func scanCustomRole(r pgx.CollectableRow) (api.CustomRole, error) {
	var c api.CustomRole
	err := r.Scan(&c.ID, &c.PeriodID, &c.DivisionID, &c.Name, &c.Capabilities)
	return c, err
}

// CustomRoles returns the custom roles of a period's division, by name.
func (s *Store) CustomRoles(ctx context.Context, q Querier, pid, did id.ID) ([]api.CustomRole, error) {
	rows, err := q.Query(ctx, `select `+customRoleColumns+` from custom_roles where period_id = $1 and division_id = $2 order by lower(name)`, pid, did)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanCustomRole)
}

// PeriodCustomRoles returns every custom role of a period, by ID.
func (s *Store) PeriodCustomRoles(ctx context.Context, q Querier, pid id.ID) (map[id.ID]api.CustomRole, error) {
	rows, err := q.Query(ctx, `select `+customRoleColumns+` from custom_roles where period_id = $1`, pid)
	if err != nil {
		return nil, err
	}
	rs, err := pgx.CollectRows(rows, scanCustomRole)
	m := map[id.ID]api.CustomRole{}
	for _, r := range rs {
		m[r.ID] = r
	}
	return m, err
}

// CustomRole returns a custom role of a period's division.
func (s *Store) CustomRole(ctx context.Context, q Querier, pid, did, rid id.ID) (api.CustomRole, error) {
	rows, err := q.Query(ctx, `select `+customRoleColumns+` from custom_roles where period_id = $1 and division_id = $2 and id = $3`, pid, did, rid)
	if err != nil {
		return api.CustomRole{}, err
	}
	c, err := pgx.CollectExactlyOneRow(rows, scanCustomRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// SaveCustomRole adds c, or updates it when it exists; ErrConflict when
// another role of the division has its name.
func (s *Store) SaveCustomRole(ctx context.Context, q Querier, c api.CustomRole, now time.Time) error {
	_, err := q.Exec(ctx, `insert into custom_roles (id, period_id, division_id, name, capabilities, created_at, updated_at)
		values ($1, $2, $3, $4, $5, $6, $6)
		on conflict (id) do update set name = excluded.name, capabilities = excluded.capabilities, updated_at = excluded.updated_at`,
		c.ID, c.PeriodID, c.DivisionID, c.Name, c.Capabilities, now)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

// DeleteCustomRole deletes a custom role and its assignments, returning the
// memberships that held it.
func (s *Store) DeleteCustomRole(ctx context.Context, q Querier, rid id.ID) ([]id.ID, error) {
	rows, err := q.Query(ctx, `delete from membership_custom_roles where role_id = $1 returning membership_id`, rid)
	if err != nil {
		return nil, err
	}
	held, err := pgx.CollectRows(rows, pgx.RowTo[id.ID])
	if err != nil {
		return nil, err
	}
	_, err = q.Exec(ctx, `delete from custom_roles where id = $1`, rid)
	return held, err
}

// Active45 returns the active 45 of a division in a period.
func (s *Store) Active45(ctx context.Context, q Querier, pid, did id.ID) (Appointment45, error) {
	var a Appointment45
	err := q.QueryRow(ctx, `select id, period_id, division_id, user_id, capabilities, appointed_by, appointed_at from acting45
		where period_id = $1 and division_id = $2 and removed_at is null`, pid, did).
		Scan(&a.ID, &a.PeriodID, &a.DivisionID, &a.UserID, &a.Capabilities, &a.AppointedBy, &a.AppointedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

// Appoint45 records an appointment; ErrConflict when the division already
// has an active 45, even one appointed by a concurrent request.
func (s *Store) Appoint45(ctx context.Context, q Querier, a Appointment45) error {
	_, err := q.Exec(ctx, `insert into acting45 (id, period_id, division_id, user_id, capabilities, appointed_by, appointed_at)
		values ($1, $2, $3, $4, $5, $6, $7)`, a.ID, a.PeriodID, a.DivisionID, a.UserID, a.Capabilities, a.AppointedBy, a.AppointedAt)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

// Remove45 ends an active appointment.
func (s *Store) Remove45(ctx context.Context, q Querier, aid, by id.ID, now time.Time) error {
	_, err := q.Exec(ctx, `update acting45 set removed_at = $2, removed_by = $3 where id = $1 and removed_at is null`, aid, now, by)
	return err
}

const archiveColumns = `id, period_id, division_id, user_id, space_id, granted_by, created_at`

func scanArchive(r pgx.CollectableRow) (api.ArchiveGrant, error) {
	var g api.ArchiveGrant
	err := r.Scan(&g.ID, &g.PeriodID, &g.DivisionID, &g.UserID, &g.SpaceID, &g.GrantedBy, &g.CreatedAt)
	return g, err
}

// ArchiveGrants returns the active grants given from a scope.
func (s *Store) ArchiveGrants(ctx context.Context, q Querier, pid, did id.ID) ([]api.ArchiveGrant, error) {
	rows, err := q.Query(ctx, `select `+archiveColumns+` from archive_grants
		where period_id = $1 and division_id = $2 and revoked_at is null order by created_at, id`, pid, did)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanArchive)
}

// ArchiveGrant returns an active grant of a scope.
func (s *Store) ArchiveGrant(ctx context.Context, q Querier, pid, did, gid id.ID) (api.ArchiveGrant, error) {
	rows, err := q.Query(ctx, `select `+archiveColumns+` from archive_grants
		where period_id = $1 and division_id = $2 and id = $3 and revoked_at is null`, pid, did, gid)
	if err != nil {
		return api.ArchiveGrant{}, err
	}
	g, err := pgx.CollectExactlyOneRow(rows, scanArchive)
	if errors.Is(err, pgx.ErrNoRows) {
		return g, ErrNotFound
	}
	return g, err
}

// CreateArchiveGrant records a grant; ErrConflict when the same scope
// already grants the user that space.
func (s *Store) CreateArchiveGrant(ctx context.Context, q Querier, g api.ArchiveGrant) error {
	_, err := q.Exec(ctx, `insert into archive_grants (id, period_id, division_id, user_id, space_id, granted_by, created_at)
		values ($1, $2, $3, $4, $5, $6, $7)`, g.ID, g.PeriodID, g.DivisionID, g.UserID, g.SpaceID, g.GrantedBy, g.CreatedAt)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

// RevokeArchiveGrant ends a grant.
func (s *Store) RevokeArchiveGrant(ctx context.Context, q Querier, gid, by id.ID, now time.Time) error {
	_, err := q.Exec(ctx, `update archive_grants set revoked_at = $2, revoked_by = $3 where id = $1 and revoked_at is null`, gid, now, by)
	return err
}

// ArchiveSpaces returns which of the spaces of a period a user may read by
// an archive grant: one not revoked, from a scope where the user is still
// an active member.
func (s *Store) ArchiveSpaces(ctx context.Context, q Querier, uid, pid id.ID) (map[id.ID]bool, error) {
	rows, err := q.Query(ctx, `select distinct g.space_id from archive_grants g
		join spaces sp on sp.id = g.space_id
		join memberships m on m.period_id = g.period_id and m.division_id = g.division_id and m.user_id = g.user_id and m.removed_at is null
		where g.user_id = $1 and sp.period_id = $2 and g.revoked_at is null`, uid, pid)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[id.ID])
	out := map[id.ID]bool{}
	for _, i := range ids {
		out[i] = true
	}
	return out, err
}

// Handover returns the handover checklist of a period; ErrNotFound until
// one is written.
func (s *Store) Handover(ctx context.Context, q Querier, pid id.ID) (api.Handover, error) {
	h := api.Handover{PeriodID: pid}
	var at time.Time
	err := q.QueryRow(ctx, `select successor_period_id, incoming_leadership, admin_custodian, bot_management, archive_plan, remaining_work, updated_at
		from period_handovers where period_id = $1`, pid).
		Scan(&h.SuccessorPeriodID, &h.IncomingLeadership, &h.AdminCustodian, &h.BotManagement, &h.ArchivePlan, &h.RemainingWork, &at)
	if errors.Is(err, pgx.ErrNoRows) {
		return h, ErrNotFound
	}
	h.UpdatedAt = &at
	return h, err
}

// PutHandover writes the handover checklist of a period.
func (s *Store) PutHandover(ctx context.Context, q Querier, h api.Handover, by id.ID, now time.Time) error {
	_, err := q.Exec(ctx, `insert into period_handovers
		(period_id, successor_period_id, incoming_leadership, admin_custodian, bot_management, archive_plan, remaining_work, updated_at, updated_by)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		on conflict (period_id) do update set successor_period_id = excluded.successor_period_id,
			incoming_leadership = excluded.incoming_leadership, admin_custodian = excluded.admin_custodian,
			bot_management = excluded.bot_management, archive_plan = excluded.archive_plan,
			remaining_work = excluded.remaining_work, updated_at = excluded.updated_at, updated_by = excluded.updated_by`,
		h.PeriodID, h.SuccessorPeriodID, h.IncomingLeadership, h.AdminCustodian, h.BotManagement, h.ArchivePlan, h.RemainingWork, now, by)
	return err
}
