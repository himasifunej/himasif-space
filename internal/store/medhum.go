package store

import (
	"context"
	"errors"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"github.com/jackc/pgx/v5"
)

const medhumColumns = `id,period_id,space_id,humas_division_id,design_division_id,title,posting_date,posting_at,design_date,design_at,humas_pj_id,design_pj_id,humas_task_id,design_task_id,concept_url,caption,trello_card_id,concept_submitted_at,published_at,version,created_by,created_at,updated_at`

func scanMedhum(row pgx.Row) (api.MedhumContent, error) {
	var c api.MedhumContent
	var post, design timeutil.Date
	var hasPost, hasDesign bool
	err := row.Scan(&c.ID, &c.PeriodID, &c.SpaceID, &c.HumasDivisionID, &c.DesignDivisionID, &c.Title, nullDate{&post, &hasPost}, &c.PostingAt, nullDate{&design, &hasDesign}, &c.DesignAt, &c.HumasPJID, &c.DesignPJID, &c.HumasTaskID, &c.DesignTaskID, &c.ConceptURL, &c.Caption, &c.TrelloCardID, &c.ConceptSubmittedAt, &c.PublishedAt, &c.Version, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if hasPost {
		c.PostingDate = &post
	}
	if hasDesign {
		c.DesignDate = &design
	}
	return c, err
}
func (s *Store) Medhum(ctx context.Context, q Querier, cid id.ID, lock string) (api.MedhumContent, error) {
	return scanMedhum(q.QueryRow(ctx, `select `+medhumColumns+` from medhum_contents where id=$1`+lock, cid))
}
func (s *Store) MedhumContents(ctx context.Context, q Querier, sid id.ID) ([]api.MedhumContent, error) {
	rows, err := q.Query(ctx, `select `+medhumColumns+` from medhum_contents where space_id=$1 order by coalesce(posting_at,posting_date::timestamptz),id`, sid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []api.MedhumContent{}
	for rows.Next() {
		c, err := scanMedhum(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Store) SaveMedhum(ctx context.Context, q Querier, c api.MedhumContent, create bool) error {
	if create {
		_, err := q.Exec(ctx, `insert into medhum_contents (`+medhumColumns+`) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)`, c.ID, c.PeriodID, c.SpaceID, c.HumasDivisionID, c.DesignDivisionID, c.Title, c.PostingDate, c.PostingAt, c.DesignDate, c.DesignAt, c.HumasPJID, c.DesignPJID, c.HumasTaskID, c.DesignTaskID, c.ConceptURL, c.Caption, c.TrelloCardID, c.ConceptSubmittedAt, c.PublishedAt, c.Version, c.CreatedBy, c.CreatedAt, c.UpdatedAt)
		return err
	}
	_, err := q.Exec(ctx, `update medhum_contents set title=$2,posting_date=$3,posting_at=$4,design_date=$5,design_at=$6,humas_pj_id=$7,design_pj_id=$8,humas_task_id=$9,design_task_id=$10,concept_url=$11,caption=$12,trello_card_id=$13,concept_submitted_at=$14,published_at=$15,version=$16,updated_at=$17 where id=$1`, c.ID, c.Title, c.PostingDate, c.PostingAt, c.DesignDate, c.DesignAt, c.HumasPJID, c.DesignPJID, c.HumasTaskID, c.DesignTaskID, c.ConceptURL, c.Caption, c.TrelloCardID, c.ConceptSubmittedAt, c.PublishedAt, c.Version, c.UpdatedAt)
	return err
}

// MedhumDesign is the content a design task belongs to, for the shared ACC.
type MedhumDesign struct {
	ContentID       id.ID
	Title           string
	HumasPJID       id.ID
	HumasDivisionID id.ID
}

// MedhumDesignOf reports whether tid is the design task of a Medhum content.
func (s *Store) MedhumDesignOf(ctx context.Context, q Querier, tid id.ID) (MedhumDesign, bool, error) {
	var d MedhumDesign
	err := q.QueryRow(ctx, `select id,title,humas_pj_id,humas_division_id from medhum_contents where design_task_id=$1`, tid).Scan(&d.ContentID, &d.Title, &d.HumasPJID, &d.HumasDivisionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, false, nil
	}
	return d, err == nil, err
}

// MedhumAcc is the approval of the design task's latest submission, if
// that submission is approved and the task is not cancelled.
func (s *Store) MedhumAcc(ctx context.Context, q Querier, designTask id.ID) (*api.MedhumAcc, error) {
	var a api.MedhumAcc
	var status, progress string
	var at *time.Time
	var by *id.ID
	var name *string
	err := q.QueryRow(ctx, `select e.match_status,t.progress,e.reviewed_at,r.id,r.display_name,e.review_side
 from task_evidence e join tasks t on t.id=e.task_id left join users r on r.id=e.reviewed_by
 where e.task_id=$1 order by e.recorded_at desc,e.id desc limit 1`, designTask).Scan(&status, &progress, &at, &by, &name, &a.Side)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil || status != api.MatchVerified || progress == api.TaskCancelled || at == nil || by == nil {
		return nil, err
	}
	a.At, a.By = *at, api.PersonRef{ID: *by, DisplayName: *name}
	return &a, nil
}
