package store

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// ArchiveTargets lists closed-period space metadata for a grant manager's
// division. Authorization belongs to the caller; this query never grants read
// access or joins the contents of a space.
func (s *Store) ArchiveTargets(ctx context.Context, q Querier, divisionID id.ID) ([]api.ArchiveTarget, error) {
	rows, err := q.Query(ctx, `select s.id, p.id, p.label, s.name, s.kind
		from spaces s join periods p on p.id = s.period_id
		join space_divisions sd on sd.space_id = s.id
		where p.status = $1 and sd.division_id = $2
		order by p.starts_on desc, p.id, s.kind, s.name, s.id`, api.PeriodClosed, divisionID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (api.ArchiveTarget, error) {
		var target api.ArchiveTarget
		err := row.Scan(&target.SpaceID, &target.PeriodID, &target.PeriodLabel, &target.SpaceName, &target.SpaceKind)
		return target, err
	})
}
