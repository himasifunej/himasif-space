package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/id"
)

type RuntimeControl struct {
	OutboundNotBefore *time.Time `json:"outboundNotBefore,omitempty"`
	ActivationID      *id.ID     `json:"activationId,omitempty"`
	Mode              string     `json:"mode"`
	RestoreID         *id.ID     `json:"restoreId,omitempty"`
	RestoredAt        *time.Time `json:"restoredAt,omitempty"`
	Reason            string     `json:"reason"`
}

func (s *Store) RuntimeControl(ctx context.Context, q Querier, lock string) (RuntimeControl, error) {
	var out RuntimeControl
	err := q.QueryRow(ctx, `select mode,restore_id,restored_at,reason,outbound_not_before,activation_id from runtime_control where singleton`+lock).Scan(&out.Mode, &out.RestoreID, &out.RestoredAt, &out.Reason, &out.OutboundNotBefore, &out.ActivationID)
	return out, err
}
func (s *Store) VerifyRunMode(ctx context.Context, mode string) error {
	if mode == "" {
		mode = "live"
	}
	var databaseQuarantined bool
	if err := s.DB().QueryRow(ctx, `select coalesce(current_setting('space.restore_quarantined',true),'false')='true'`).Scan(&databaseQuarantined); err != nil {
		return err
	}
	if databaseQuarantined && mode == "live" {
		return errors.New("this database is persistently quarantined for restore; live serving is disabled")
	}
	control, err := s.RuntimeControl(ctx, s.DB(), LockNone)
	if err != nil {
		return fmt.Errorf("read runtime quarantine: %w", err)
	}
	if mode == "maintenance" {
		return nil
	}
	if mode != control.Mode {
		return fmt.Errorf("runtime mode %q conflicts with database marker %q; isolated restore cannot be activated by changing environment variables", mode, control.Mode)
	}
	return nil
}
func (s *Store) RequireRestoreDatabase(ctx context.Context, expected string) error {
	if (!strings.HasPrefix(expected, "space_restore_") && !strings.HasPrefix(expected, "space_shadow_")) || len(expected) > 63 {
		return errors.New("restore database name must start with space_restore_ or space_shadow_ and fit PostgreSQL's 63-byte identifier limit")
	}
	var actual string
	if err := s.DB().QueryRow(ctx, `select current_database()`).Scan(&actual); err != nil {
		return err
	}
	if actual != expected {
		return errors.New("database confirmation does not match the connected database; no quarantine changes made")
	}
	return nil
}
func (s *Store) QuarantineRestore(ctx context.Context, expected, reason string) (RuntimeControl, error) {
	var out RuntimeControl
	if err := s.RequireRestoreDatabase(ctx, expected); err != nil {
		return out, err
	}
	if strings.TrimSpace(reason) == "" || len(reason) > 4000 {
		return out, errors.New("a restore reason of 1 to 4000 bytes is required")
	}
	acquired, err := s.WithSessionLock(ctx, runtimeLeaseKey, func() error {
		return s.Tx(ctx, func(q Querier) error {
			before, err := s.RuntimeControl(ctx, q, LockUpdate)
			if err != nil {
				return err
			}
			if before.Mode == "isolated_restore" {
				out = before
				return nil
			}
			now := time.Now().UTC()
			rid := id.New()
			out = RuntimeControl{Mode: "isolated_restore", RestoreID: &rid, RestoredAt: &now, Reason: reason}
			if _, err = q.Exec(ctx, `update runtime_control set mode='isolated_restore',restore_id=$1,restored_at=$2,reason=$3 where singleton`, rid, now, reason); err != nil {
				return err
			}
			// Restored sessions cannot authenticate clients from the original system.
			// Original delivery queues/receipts remain intact for the restore audit;
			// quarantine, not deleting history, prevents them from being dispatched.
			if _, err = q.Exec(ctx, `update sessions set revoked_at=$1 where revoked_at is null`, now); err != nil {
				return err
			}
			return s.Audit(ctx, q, AuditEvent{ActorKind: "system", Action: "runtime.restore.quarantine", Source: "cli", TargetType: "runtime", Before: before, After: out, OccurredAt: now})
		})
	})
	if err == nil && !acquired {
		err = errors.New("a server is still using this database; stop it before quarantining a restored copy")
	}
	return out, err
}
