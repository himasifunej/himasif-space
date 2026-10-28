package server

import (
	"context"
	"slices"
	"testing"

	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
)

// The task and reminder branches can reach a database in either order. In
// particular, seeing 0004 in the ledger must not make the runner skip 0003.
func TestMigrationsIndependentBranchOrder(t *testing.T) {
	for _, tc := range []struct {
		name, missing, remove, preserved string
	}{
		{
			name:    "reminders_before_tasks",
			missing: "0003_tasks",
			remove: `drop table task_notifications, task_events, task_evidence,
				task_participants, task_help_requests, task_comments, task_spaces, tasks`,
			preserved: `select jsonb_build_object(
				'reminders', (select jsonb_agg(to_jsonb(r) order by id) from reminders r),
				'shares', (select jsonb_agg(to_jsonb(s) order by reminder_id,recipient_id) from reminder_shares s),
				'preferences', (select jsonb_agg(to_jsonb(p) order by user_id) from reminder_preferences p))::text`,
		},
		{
			name:    "tasks_before_reminders",
			missing: "0004_personal_reminders",
			remove:  `drop table reminder_shares, reminder_preferences, reminders`,
			preserved: `select jsonb_build_object(
				'tasks', (select jsonb_agg(to_jsonb(t) order by id) from tasks t),
				'spaces', (select jsonb_agg(to_jsonb(s) order by task_id,space_id) from task_spaces s),
				'comments', (select jsonb_agg(to_jsonb(c) order by id) from task_comments c),
				'evidence', (select jsonb_agg(to_jsonb(e) order by id) from task_evidence e),
				'events', (select jsonb_agg(to_jsonb(e) order by id) from task_events e),
				'notifications', (select jsonb_agg(to_jsonb(n) order by id) from task_notifications n))::text`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			st := testDB(t)
			seedMigrationOrderData(t, st)
			q := st.DB()

			// testDB owns a disposable database. Remove only the absent branch's
			// tables and ledger entry to reproduce the schema before it arrives.
			if err := st.Tx(ctx, func(tx store.Querier) error {
				if _, err := tx.Exec(ctx, tc.remove); err != nil {
					return err
				}
				_, err := tx.Exec(ctx, `delete from schema_migrations where name=$1`, tc.missing)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			snapshot := func(query string, args ...any) string {
				t.Helper()
				var value string
				if err := q.QueryRow(ctx, query, args...).Scan(&value); err != nil {
					t.Fatal(err)
				}
				return value
			}
			const ledgerQuery = `select jsonb_agg(to_jsonb(m) order by name)::text
				from schema_migrations m where name<>$1`
			beforeData := snapshot(tc.preserved)
			beforeLedger := snapshot(ledgerQuery, tc.missing)

			applied, err := st.Migrate(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(applied, []string{tc.missing}) {
				t.Fatalf("applied %v, want only %s", applied, tc.missing)
			}
			if after := snapshot(tc.preserved); after != beforeData {
				t.Fatalf("existing branch data changed:\nbefore %s\nafter  %s", beforeData, after)
			}
			if after := snapshot(ledgerQuery, tc.missing); after != beforeLedger {
				t.Fatalf("existing migration ledger changed:\nbefore %s\nafter  %s", beforeLedger, after)
			}
			var tables int
			if err := q.QueryRow(ctx, `select count(*) from information_schema.tables
				where table_schema='public' and table_name in (
				'tasks','task_spaces','task_comments','task_help_requests','task_participants',
				'task_evidence','task_events','task_notifications',
				'reminders','reminder_shares','reminder_preferences')`).Scan(&tables); err != nil {
				t.Fatal(err)
			}
			if tables != 11 {
				t.Fatalf("branch tables after migration = %d, want 11", tables)
			}
			ledger := snapshot(`select jsonb_agg(to_jsonb(m) order by name)::text from schema_migrations m`)
			applied, err = st.Migrate(ctx)
			if err != nil || len(applied) != 0 {
				t.Fatalf("repeat migration applied=%v err=%v", applied, err)
			}
			if snapshot(tc.preserved) != beforeData || snapshot(`select jsonb_agg(to_jsonb(m) order by name)::text from schema_migrations m`) != ledger {
				t.Fatal("repeat migration changed existing data or migration timestamps")
			}
		})
	}
}

func seedMigrationOrderData(t *testing.T, st *store.Store) {
	t.Helper()
	ctx := context.Background()
	owner, recipient := id.New(), id.New()
	period, division, space, task := id.New(), id.New(), id.New(), id.New()
	dateReminder, instantReminder := id.New(), id.New()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := st.DB().Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`insert into users(id,kind,email,display_name,email_verified_at)
		values ($1,'person','migration-owner@himasif.test','Owner',now()),
		($2,'person','migration-recipient@himasif.test','Recipient',now())`, owner, recipient)
	exec(`insert into periods(id,label,starts_on,ends_on,status)
		values ($1,'migration-order','2026-01-01','2026-12-31','active')`, period)
	exec(`insert into divisions(id,slug,name) values ($1,'migration-test','Migration test')`, division)
	exec(`insert into period_divisions(period_id,division_id) values ($1,$2)`, period, division)
	exec(`insert into spaces(id,period_id,code,name,kind) values ($1,$2,'division-migration-test','Migration test','division')`, space, period)
	exec(`insert into tasks(id,period_id,owner_division_id,title,expected_output,assignee_id,
		deadline_date,progress,match_status,rank,version,created_by,created_at,updated_at)
		values ($1,$2,$3,'Preserve canonical task','Design URL',$4,'2026-11-05',
		'submitted','pending',3072,7,$4,'2026-10-28T02:00:00Z','2026-10-28T03:00:00Z')`, task, period, division, owner)
	exec(`insert into task_spaces(task_id,space_id,period_id) values ($1,$2,$3)`, task, space, period)
	exec(`insert into task_comments(id,task_id,author_id,body,created_at)
		values ($1,$2,$3,'Keep this discussion','2026-10-28T02:30:00Z')`, id.New(), task, owner)
	exec(`insert into task_evidence(id,task_id,submitted_by,note,match_status,submitted_at)
		values ($1,$2,$3,'Original evidence','pending','2026-10-28T03:00:00Z')`, id.New(), task, owner)
	exec(`insert into task_events(id,task_id,actor_id,action,created_at,after)
		values ($1,$2,$3,'task.evidence.submit','2026-10-28T03:00:00Z','{"progress":"submitted"}')`, id.New(), task, owner)
	exec(`insert into task_notifications(id,task_id,recipient_id,title,body,created_at)
		values ($1,$2,$3,'Result submitted','Preserve notification','2026-10-28T03:00:00Z')`, id.New(), task, recipient)
	exec(`insert into reminders(id,owner_id,title,notes,due_kind,due_on,shared,completed_at)
		values ($1,$2,'Preserve date-only reminder','Private notes','date','2026-11-10',true,'2026-10-28T03:00:00Z')`, dateReminder, owner)
	exec(`insert into reminders(id,owner_id,title,due_kind,due_at,time_zone)
		values ($1,$2,'Preserve instant reminder','instant','2026-11-11T06:30:00Z','Asia/Jakarta')`, instantReminder, owner)
	exec(`insert into reminder_shares(reminder_id,recipient_id,shared_by) values ($1,$2,$3)`, dateReminder, recipient, owner)
	exec(`insert into reminder_preferences(user_id,default_share) values ($1,true)`, owner)
}
