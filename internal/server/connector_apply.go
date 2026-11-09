package server

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"net/http"
	"reflect"
	"strings"
)

type backgroundAuditKey struct{}

func connectorRuleRequest(ctx context.Context, run id.ID) *http.Request {
	r := (&http.Request{}).WithContext(context.WithValue(ctx, backgroundAuditKey{}, run))
	return r
}
func (s *Server) connectorEvent(ctx context.Context, q store.Querier, cid id.ID, actor *id.ID, action string, before, after any) error {
	var b, a []byte
	var err error
	if before != nil {
		b, err = json.Marshal(before)
		if err != nil {
			return err
		}
	}
	if after != nil {
		a, err = json.Marshal(after)
		if err != nil {
			return err
		}
	}
	_, err = q.Exec(ctx, `insert into connector_events(id,connector_id,actor_id,action,before,after,occurred_at) values($1,$2,$3,$4,$5,$6,$7,$8)`, id.New(), cid, actor, action, b, a, s.now())
	return err
}

type connectorApplyTarget struct {
	item, content id.ID
	source        api.ConnectorSourceItem
	baseline      api.MedhumContent
	oldSource     api.ConnectorSourceItem
}

func (s *Server) applyConnectorBindings(ctx context.Context, snapshot api.Connector, runID id.ID) error {
	actor, err := s.connectorSystemActor(ctx, snapshot)
	if err != nil {
		return err
	}
	return s.store.Tx(ctx, func(q store.Querier) error {
		period, err := s.store.Period(ctx, q, snapshot.PeriodID, store.LockUpdate)
		if err != nil {
			return err
		}
		if period.Status == api.PeriodClosed {
			return errPeriodClosed
		}
		c, err := s.store.Connector(ctx, q, snapshot.ID, store.LockUpdate)
		if err != nil {
			return err
		}
		if c.Version != snapshot.Version {
			return conflict("Konfigurasi berubah sebelum penerapan binding.")
		}
		r := connectorRuleRequest(ctx, runID)
		r.SetPathValue("periodId", c.PeriodID.String())
		r.SetPathValue("divisionId", c.DivisionID.String())
		sc, err := s.loadScope(ctx, q, r, actor, store.LockNone)
		if err != nil {
			return err
		}
		if err := sc.require(api.CapTasksManage); err != nil {
			return err
		}
		rows, err := q.Query(ctx, `select i.id,b.content_id,i.data,b.accepted_data,b.accepted_content from connector_items i join connector_bindings b on b.connector_id=i.connector_id and b.external_key=i.external_key where i.connector_id=$1 and b.follow_updates and i.state='pending' and i.issue='' and i.fingerprint<>b.accepted_fingerprint order by i.id`, c.ID)
		if err != nil {
			return err
		}
		targets := []connectorApplyTarget{}
		for rows.Next() {
			var t connectorApplyTarget
			var source, old, baseline []byte
			if err := rows.Scan(&t.item, &t.content, &source, &old, &baseline); err != nil {
				rows.Close()
				return err
			}
			if json.Unmarshal(source, &t.source) != nil || json.Unmarshal(old, &t.oldSource) != nil || json.Unmarshal(baseline, &t.baseline) != nil {
				rows.Close()
				return errors.New("dasar binding belum lengkap; tinjau ulang pemetaan")
			}
			targets = append(targets, t)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		// Lock every potentially referenced account in a single global order before
		// any task mutation. Period serialization stabilizes mappings and assignees.
		accounts := []id.ID{actor.User.ID}
		people, err := q.Query(ctx, `select user_id from connector_people where connector_id=$1 union select mc.humas_pj_id from medhum_contents mc join connector_bindings b on b.content_id=mc.id where b.connector_id=$1 union select mc.design_pj_id from medhum_contents mc join connector_bindings b on b.content_id=mc.id where b.connector_id=$1 and mc.design_pj_id is not null`, c.ID)
		if err != nil {
			return err
		}
		for people.Next() {
			var uid id.ID
			if err := people.Scan(&uid); err != nil {
				people.Close()
				return err
			}
			accounts = append(accounts, uid)
		}
		err = people.Err()
		people.Close()
		if err != nil {
			return err
		}
		if err := s.lockTaskAccounts(ctx, q, accounts); err != nil {
			return err
		}
		var active bool
		if err := q.QueryRow(ctx, `select exists(select 1 from users where id=$1 and disabled_at is null)`, actor.User.ID).Scan(&active); err != nil {
			return err
		}
		if !active {
			return errForbidden
		}
		for _, target := range targets {
			if _, err := q.Exec(ctx, `savepoint connector_apply_item`); err != nil {
				return err
			}
			err := s.applyConnectorTarget(r, q, actor, period, c, target)
			if err != nil {
				if _, rollbackErr := q.Exec(ctx, `rollback to savepoint connector_apply_item`); rollbackErr != nil {
					return rollbackErr
				}
				message := "Perubahan sumber belum diterapkan. Tinjau bidang yang berubah dan pemetaan akun."
				var ae *apiError
				if errors.As(err, &ae) {
					message = ae.message
				}
				if _, updateErr := q.Exec(ctx, `update connector_items set state='conflict',issue=$2 where id=$1`, target.item, message); updateErr != nil {
					return updateErr
				}
				if eventErr := s.connectorEvent(ctx, q, c.ID, nil, "source.conflict", target.baseline, map[string]any{"source": target.source, "reason": message}); eventErr != nil {
					return eventErr
				}
			}
			if _, err := q.Exec(ctx, `release savepoint connector_apply_item`); err != nil {
				return err
			}
		}
		var remaining bool
		if err := q.QueryRow(ctx, `select exists(select 1 from connector_items where connector_id=$1 and state in ('pending','conflict','missing'))`, c.ID).Scan(&remaining); err != nil {
			return err
		}
		if !remaining {
			_, err = q.Exec(ctx, `update connector_runs set status='succeeded' where id=$1 and status='needs_review'`, runID)
		}
		return err
	})
}
func (s *Server) applyConnectorTarget(r *http.Request, q store.Querier, p *principal, period api.Period, connector api.Connector, t connectorApplyTarget) error {
	current, err := s.store.Medhum(r.Context(), q, t.content, store.LockUpdate)
	if err != nil {
		return err
	}
	if current.SpaceID != connector.SpaceID || current.HumasDivisionID != connector.DivisionID {
		return errForbidden
	}
	before := current
	baseline := t.baseline
	conflictField := func(field string) error {
		return conflict("Bidang " + field + " berubah di sumber dan aplikasi. Kedua versi dipertahankan untuk ditinjau.")
	}
	mergeString := func(field string, oldSource, newSource string, oldApp string, app *string, base *string) error {
		if oldSource == newSource {
			return nil
		}
		if *app != oldApp && *app != newSource {
			return conflictField(field)
		}
		*app = newSource
		*base = newSource
		return nil
	}
	if connector.Kind == "sheets" {
		if err := mergeString("judul", t.oldSource.Title, t.source.Title, baseline.Title, &current.Title, &baseline.Title); err != nil {
			return err
		}
		if !reflect.DeepEqual(t.oldSource.PostingDate, t.source.PostingDate) {
			if current.PostingAt != nil || !reflect.DeepEqual(current.PostingDate, baseline.PostingDate) && !reflect.DeepEqual(current.PostingDate, t.source.PostingDate) {
				return conflictField("jadwal posting")
			}
			current.PostingDate = t.source.PostingDate
			current.PostingAt = nil
			baseline.PostingDate = t.source.PostingDate
			baseline.PostingAt = nil
		}
		if t.oldSource.HumasName != t.source.HumasName {
			var uid id.ID
			name := strings.ToLower(strings.Join(strings.Fields(t.source.HumasName), " "))
			if err := q.QueryRow(r.Context(), `select user_id from connector_people where connector_id=$1 and source_name=$2 and division_id=$3`, connector.ID, name, current.HumasDivisionID).Scan(&uid); err != nil {
				return conflict("Nama PJ sumber belum dipetakan ke akun yang disetujui.")
			}
			if current.HumasPJID != baseline.HumasPJID && current.HumasPJID != uid {
				return conflictField("PJ Humas")
			}
			current.HumasPJID = uid
			baseline.HumasPJID = uid
		}
	} else {
		for _, field := range []struct {
			name, old, new, oldApp string
			app, base              *string
		}{{"caption", t.oldSource.Caption, t.source.Caption, baseline.Caption, &current.Caption, &baseline.Caption}, {"tautan konsep", t.oldSource.ConceptURL, t.source.ConceptURL, baseline.ConceptURL, &current.ConceptURL, &baseline.ConceptURL}, {"kartu Trello", t.oldSource.TrelloCardID, t.source.TrelloCardID, baseline.TrelloCardID, &current.TrelloCardID, &baseline.TrelloCardID}} {
			if err := mergeString(field.name, field.old, field.new, field.oldApp, field.app, field.base); err != nil {
				return err
			}
		}
	}
	if err := validateMedhum(&current, period.TimeZone); err != nil {
		return err
	}
	if !reflect.DeepEqual(before, current) {
		if err := s.syncMedhumTasks(r, q, p, period, &current); err != nil {
			return err
		}
		current.Version++
		current.UpdatedAt = s.now().UTC()
		if err := s.store.SaveMedhum(r.Context(), q, current, false); err != nil {
			return err
		}
		if err := s.record(r, q, p, store.AuditEvent{Action: "connector.content.update", TargetType: "medhum_content", TargetID: &current.ID, PeriodID: &period.ID, Before: before, After: current}); err != nil {
			return err
		}
	}
	source, _ := json.Marshal(t.source)
	base, _ := json.Marshal(baseline)
	if _, err := q.Exec(r.Context(), `update connector_bindings set accepted_fingerprint=$3,accepted_data=$4,accepted_content=$5 where connector_id=$1 and external_key=$2`, connector.ID, t.source.Key, t.source.Fingerprint, source, base); err != nil {
		return err
	}
	if _, err := q.Exec(r.Context(), `update connector_items set state='applied',content_id=$2,issue='' where id=$1`, t.item, current.ID); err != nil {
		return err
	}
	return s.connectorEvent(r.Context(), q, connector.ID, nil, "source.applied", before, map[string]any{"content": current, "source": t.source})
}
