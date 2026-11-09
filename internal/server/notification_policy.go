package server

import (
	"errors"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/jackc/pgx/v5"
	"net/http"
	"slices"
)

func validateNotificationPolicy(p api.NotificationPolicy) error {
	if len(p.TaskDays) > 30 || p.LocalHour < 0 || p.LocalHour > 23 || p.LocalMinute < 0 || p.LocalMinute > 59 || p.OverdueDays < 1 || p.OverdueDays > 30 {
		return badRequest("Konfigurasi pengingat tidak valid.")
	}
	for _, d := range p.TaskDays {
		if d < 0 || d > 365 {
			return badRequest("Hari pengingat harus 0–365 hari sebelum tenggat.")
		}
	}
	return nil
}
func (s *Server) divisionNotificationPolicy(w http.ResponseWriter, r *http.Request, p *principal) error {
	out := api.DefaultNotificationPolicy()
	if r.Method == http.MethodPut {
		if err := readJSON(r, &out); err != nil {
			return err
		}
		if err := validateNotificationPolicy(out); err != nil {
			return err
		}
		slices.Sort(out.TaskDays)
		out.TaskDays = slices.Compact(out.TaskDays)
		if out.TaskDays == nil {
			out.TaskDays = []int{}
		}
	}
	err := s.store.Tx(r.Context(), func(q store.Querier) error {
		lock := store.LockShare
		if r.Method == http.MethodPut {
			lock = store.LockUpdate
		}
		sc, err := s.loadScope(r.Context(), q, r, p, lock)
		if err != nil {
			return err
		}
		if r.Method == http.MethodGet {
			if err := sc.require(api.CapSpaceRead); err != nil {
				return err
			}
			err = q.QueryRow(r.Context(), `select task_days,local_hour,local_minute,overdue_days from notification_policies where period_id=$1 and division_id=$2`, sc.period.ID, sc.division.ID).Scan(&out.TaskDays, &out.LocalHour, &out.LocalMinute, &out.OverdueDays)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		if p.admin() {
			return errForbidden
		}
		if err := sc.require(api.CapTasksManage); err != nil {
			return err
		}
		if _, err := s.lockedAccount(r, q, p); err != nil {
			return err
		}
		_, err = q.Exec(r.Context(), `insert into notification_policies(period_id,division_id,task_days,local_hour,local_minute,overdue_days) values($1,$2,$3,$4,$5,$6) on conflict(period_id,division_id) do update set task_days=excluded.task_days,local_hour=excluded.local_hour,local_minute=excluded.local_minute,overdue_days=excluded.overdue_days`, sc.period.ID, sc.division.ID, out.TaskDays, out.LocalHour, out.LocalMinute, out.OverdueDays)
		if err != nil {
			return err
		}
		if _, err := q.Exec(r.Context(), `update tasks set notification_revision=notification_revision+1 where period_id=$1 and owner_division_id=$2`, sc.period.ID, sc.division.ID); err != nil {
			return err
		}
		return s.record(r, q, p, store.AuditEvent{Action: "notifications.policy", PeriodID: &sc.period.ID, ScopeType: "division", ScopeID: &sc.division.ID, After: out})
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}
func (s *Server) taskNotificationPolicy(w http.ResponseWriter, r *http.Request, p *principal) error {
	out := api.DefaultNotificationPolicy()
	if r.Method == http.MethodPut {
		if err := readJSON(r, &out); err != nil {
			return err
		}
		if err := validateNotificationPolicy(out); err != nil {
			return err
		}
		if out.TaskDays == nil {
			out.TaskDays = []int{}
		}
	}
	err := s.withTask(r, p, r.Method != http.MethodGet, func(q store.Querier, period api.Period, t *store.TaskRow) error {
		if r.Method == http.MethodGet {
			err := q.QueryRow(r.Context(), `select task_days,local_hour,local_minute,overdue_days from task_notification_policies where task_id=$1`, t.ID).Scan(&out.TaskDays, &out.LocalHour, &out.LocalMinute, &out.OverdueDays)
			if errors.Is(err, pgx.ErrNoRows) {
				err = q.QueryRow(r.Context(), `select task_days,local_hour,local_minute,overdue_days from notification_policies where period_id=$1 and division_id=$2`, period.ID, t.OwnerDivisionID).Scan(&out.TaskDays, &out.LocalHour, &out.LocalMinute, &out.OverdueDays)
				if errors.Is(err, pgx.ErrNoRows) {
					return nil
				}
			}
			return err
		}
		if !t.Permissions.Manage {
			return errForbidden
		}
		if r.Method == http.MethodDelete {
			if _, err := q.Exec(r.Context(), `delete from task_notification_policies where task_id=$1`, t.ID); err != nil {
				return err
			}
			_, err := q.Exec(r.Context(), `update tasks set notification_revision=notification_revision+1 where id=$1`, t.ID)
			return err
		}
		_, err := q.Exec(r.Context(), `insert into task_notification_policies(task_id,task_days,local_hour,local_minute,overdue_days) values($1,$2,$3,$4,$5) on conflict(task_id) do update set task_days=excluded.task_days,local_hour=excluded.local_hour,local_minute=excluded.local_minute,overdue_days=excluded.overdue_days`, t.ID, out.TaskDays, out.LocalHour, out.LocalMinute, out.OverdueDays)
		if err != nil {
			return err
		}
		if _, err := q.Exec(r.Context(), `update tasks set notification_revision=notification_revision+1 where id=$1`, t.ID); err != nil {
			return err
		}
		return s.record(r, q, p, store.AuditEvent{Action: "task.notifications.policy", TargetType: "task", TargetID: &t.ID, PeriodID: &period.ID, After: out})
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}
