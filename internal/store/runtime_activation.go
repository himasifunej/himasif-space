package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/jackc/pgx/v5"
)

// ActivationPlan is a reviewable, database-specific plan. Empty applyHash only
// previews. The operator is an attribution label, never a fabricated user ID.
type ActivationPlan struct {
	Database     string           `json:"database"`
	Before       RuntimeControl   `json:"before"`
	Cutoff       time.Time        `json:"cutoff"`
	Operator     string           `json:"operator"`
	Reason       string           `json:"reason"`
	Affected     map[string]int64 `json:"affected"`
	Hash         string           `json:"sha256"`
	Applied      bool             `json:"applied"`
	ActivationID *id.ID           `json:"activationId,omitempty"`
}

func (s *Store) ActivateRestore(ctx context.Context, database, restoreID, operator, reason string, cutoff time.Time, applyHash string) (ActivationPlan, error) {
	plan := ActivationPlan{Database: database, Cutoff: cutoff.UTC().Truncate(time.Microsecond), Operator: strings.TrimSpace(operator), Reason: strings.TrimSpace(reason), Affected: map[string]int64{}}
	if err := s.RequireRestoreDatabase(ctx, database); err != nil {
		return plan, err
	}
	now := time.Now().UTC()
	if plan.Operator == "" || len(plan.Operator) > 240 || plan.Reason == "" || len(plan.Reason) > 4000 {
		return plan, errors.New("operator (1..240 bytes) and reason (1..4000 bytes) are required")
	}
	if cutoff.IsZero() || cutoff.After(now) || cutoff.Before(now.Add(-15*time.Minute)) {
		return plan, errors.New("cutoff must be within the last 15 minutes; preview again with a new cutoff if expired")
	}
	acquired, err := s.WithSessionLock(ctx, runtimeLeaseKey, func() error {
		return s.Tx(ctx, func(q Querier) error {
			before, err := s.RuntimeControl(ctx, q, LockUpdate)
			if err != nil {
				return err
			}
			plan.Before = before
			if before.Mode != "isolated_restore" || before.RestoreID == nil || before.RestoreID.String() != restoreID {
				return errors.New("expected isolated restore ID does not match; activation refused")
			}
			if before.RestoredAt == nil || plan.Cutoff.Before(*before.RestoredAt) {
				return errors.New("cutoff predates this restore")
			}
			// With no serving lease, counts must remain stable between preview and apply.
			counts := map[string]string{
				"delivery_events":      "select count(*) from delivery_events where suppressed_at is null",
				"wa_outbox":            "select count(*) from wa_outbox where suppressed_at is null",
				"inbound_pending":      "select count(*) from wa_inbound where state in ('queued','processing')",
				"connectors_enabled":   "select count(*) from connector_configs where enabled",
				"wa_enabled":           "select count(*) from wa_connections where enabled",
				"sop_policies_enabled": "select count(*) from sop_policies where enabled",
				"findings_recheck":     "select count(*) from sop_findings where not needs_recheck",
				"sessions":             "select count(*) from sessions where revoked_at is null",
				"link_challenges":      "select count(*) from wa_link_challenges where used_at is null",
			}
			for key, sql := range counts {
				var count int64
				if err = q.QueryRow(ctx, sql).Scan(&count); err != nil {
					return err
				}
				plan.Affected[key] = count
			}
			raw, err := json.Marshal(plan)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(raw)
			plan.Hash = hex.EncodeToString(digest[:])
			if applyHash == "" {
				return nil
			}
			if applyHash != plan.Hash {
				return errors.New("activation plan changed or hash differs; review a fresh preview")
			}
			aid := id.New()
			statements := []string{
				`update delivery_events set suppressed_at=$1,suppression_reason='Restored queue retained for audit; fresh activation epoch' where suppressed_at is null`,
				`update wa_outbox set suppressed_at=$1,suppression_reason='Restored queue retained for audit; no resend',lease_until=null where suppressed_at is null`,
				`update wa_inbound set state='ignored',error='Pending input retained but not replayed after restore activation' where state in ('queued','processing')`,
				`update connector_configs set enabled=false,version=version+1,updated_at=$1 where enabled`,
				`update wa_connections set enabled=false,version=version+1,updated_at=$1 where enabled`,
				`update sop_policies set enabled=false,version=version+1,updated_at=$1 where enabled`,
				`update sop_findings set needs_recheck=true,version=version+1,updated_at=$1 where not needs_recheck`,
				`update sessions set revoked_at=$1 where revoked_at is null`,
				`update wa_link_challenges set used_at=$1 where used_at is null`,
			}
			for _, sql := range statements {
				var args []any
				if strings.Contains(sql, "$1") {
					args = []any{now}
				}
				if _, err = q.Exec(ctx, sql, args...); err != nil {
					return err
				}
			}
			if _, err = q.Exec(ctx, `update runtime_control set mode='live',outbound_not_before=$1,activation_id=$2,reason=$3 where singleton`, plan.Cutoff, aid, plan.Reason); err != nil {
				return err
			}
			// Reset the persistent database setting in the SAME transaction. Connections
			// opened before this change remain quarantined and must be discarded.
			if _, err = q.Exec(ctx, `alter database `+pgx.Identifier{database}.Sanitize()+` reset space.restore_quarantined`); err != nil {
				return fmt.Errorf("reset quarantine setting: %w", err)
			}
			plan.Applied = true
			plan.ActivationID = &aid
			return s.Audit(ctx, q, AuditEvent{ActorKind: "system", Action: "runtime.restore.activate", Source: "cli", TargetType: "runtime", Before: before, After: plan, OccurredAt: now})
		})
	})
	if err == nil && !acquired {
		err = errors.New("a server is still using this database; stop all serving processes before preview or activation")
	}
	if err != nil {
		plan.Applied = false
		plan.ActivationID = nil
	}
	return plan, err
}
