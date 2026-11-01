create table emergency_snapshots (
 id uuid primary key,
 space_id uuid not null references spaces(id),
 period_id uuid not null references periods(id),
 captured_by uuid not null references users(id),
 captured_at timestamptz not null,
 data jsonb not null
);
create table emergency_snapshot_rows (
 id uuid primary key,
 snapshot_id uuid not null references emergency_snapshots(id),
 task_id uuid not null references tasks(id),
 data jsonb not null,
 unique(snapshot_id,task_id)
);
create index emergency_snapshot_rows_snapshot on emergency_snapshot_rows(snapshot_id,id);
create table emergency_reconciliations (
 row_id uuid primary key references emergency_snapshot_rows(id),
 request_hash text not null,
 actor_id uuid not null references users(id),
 data jsonb not null,
 created_at timestamptz not null
);
create trigger emergency_snapshots_append_only before update or delete on emergency_snapshots
 for each row execute function audit_events_append_only();
create trigger emergency_snapshot_rows_append_only before update or delete on emergency_snapshot_rows
 for each row execute function audit_events_append_only();
create trigger emergency_reconciliations_append_only before update or delete on emergency_reconciliations
 for each row execute function audit_events_append_only();
