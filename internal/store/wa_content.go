package store

import (
	"context"
	"errors"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/jackc/pgx/v5"
)

const waContentColumns = `x.id,x.inbound_id,x.space_id,x.kind,x.position,x.parser_version,x.parsed,x.candidates,x.content_id,x.evidence_id,x.sender_user_id,coalesce(u.display_name,''),x.state,x.issue,i.body,x.source_at,i.received_at,x.matched_at,x.reviewed_at,x.reviewed_by,x.version`

func scanWAContent(row pgx.Row) (api.WAContentItem, error) {
	var x api.WAContentItem
	err := row.Scan(&x.ID, &x.InboundID, &x.SpaceID, &x.Kind, &x.Position, &x.ParserVersion, &x.Parsed, &x.Candidates, &x.ContentID, &x.EvidenceID, &x.SenderUserID, &x.SenderName, &x.State, &x.Issue, &x.Body, &x.SourceAt, &x.ReceivedAt, &x.MatchedAt, &x.ReviewedAt, &x.ReviewedBy, &x.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return x, err
}
func (s *Store) WAContentItem(ctx context.Context, q Querier, iid id.ID, lock string) (api.WAContentItem, error) {
	suffix := ""
	if lock == LockUpdate {
		suffix = " for update of x"
	}
	return scanWAContent(q.QueryRow(ctx, `select `+waContentColumns+` from wa_content_items x join wa_inbound i on i.id=x.inbound_id left join users u on u.id=x.sender_user_id where x.id=$1`+suffix, iid))
}
func (s *Store) WAContentItems(ctx context.Context, q Querier, sid id.ID) ([]api.WAContentItem, error) {
	rows, err := q.Query(ctx, `select `+waContentColumns+` from wa_content_items x join wa_inbound i on i.id=x.inbound_id left join users u on u.id=x.sender_user_id where x.space_id=$1 order by (x.state='pending') desc,x.source_at desc,x.id desc limit 200`, sid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []api.WAContentItem{}
	for rows.Next() {
		x, err := scanWAContent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Store) SaveWAContentItem(ctx context.Context, q Querier, x api.WAContentItem, create bool) error {
	if create {
		_, err := q.Exec(ctx, `insert into wa_content_items(id,inbound_id,space_id,kind,position,parser_version,parsed,candidates,content_id,evidence_id,sender_user_id,state,issue,source_at,matched_at,reviewed_at,reviewed_by,version) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`, x.ID, x.InboundID, x.SpaceID, x.Kind, x.Position, x.ParserVersion, x.Parsed, x.Candidates, x.ContentID, x.EvidenceID, x.SenderUserID, x.State, x.Issue, x.SourceAt, x.MatchedAt, x.ReviewedAt, x.ReviewedBy, x.Version)
		return err
	}
	_, err := q.Exec(ctx, `update wa_content_items set candidates=$2,content_id=$3,evidence_id=$4,state=$5,issue=$6,matched_at=$7,reviewed_at=$8,reviewed_by=$9,version=$10,sender_user_id=$11 where id=$1`, x.ID, x.Candidates, x.ContentID, x.EvidenceID, x.State, x.Issue, x.MatchedAt, x.ReviewedAt, x.ReviewedBy, x.Version, x.SenderUserID)
	return err
}
