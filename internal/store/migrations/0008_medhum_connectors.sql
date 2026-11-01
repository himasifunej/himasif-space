-- Content owns the posting calendar; two canonical tasks represent the
-- independent Humas and Mediatek responsibilities. Dates are never re-subtracted.
create table medhum_contents (
 id uuid primary key,
 period_id uuid not null references periods(id),
 space_id uuid not null,
 humas_division_id uuid not null,
 design_division_id uuid not null,
 title text not null check(length(title) between 1 and 240),
 posting_date date,
 posting_at timestamptz,
 design_date date,
 design_at timestamptz,
 humas_pj_id uuid not null references users(id),
 design_pj_id uuid references users(id),
 humas_task_id uuid references tasks(id),
 design_task_id uuid references tasks(id),
 concept_url text not null default '',
 caption text not null default '',
 trello_card_id text not null default '',
 concept_submitted_at timestamptz,
 published_at timestamptz,
 version bigint not null default 1,
 created_by uuid not null references users(id),
 created_at timestamptz not null,
 updated_at timestamptz not null,
 foreign key(space_id,period_id) references spaces(id,period_id),
 foreign key(period_id,humas_division_id) references period_divisions(period_id,division_id),
 foreign key(period_id,design_division_id) references period_divisions(period_id,division_id),
 check ((posting_date is null) <> (posting_at is null)),
 check ((design_date is null) <> (design_at is null)),
 check(humas_division_id<>design_division_id)
);
create index medhum_calendar on medhum_contents(space_id,posting_date,posting_at,id);
create table connector_configs (
 id uuid primary key,
 period_id uuid not null,
 division_id uuid not null,
 space_id uuid not null,
 kind text not null check(kind in ('sheets','trello')),
 name text not null,
 config jsonb not null,
 enabled boolean not null default false,
 mode text not null default 'read_only' check(mode in ('read_only','mirror')),
 interval_seconds integer not null default 900 check(interval_seconds between 300 and 86400),
 next_run_at timestamptz not null,
 last_success_at timestamptz,
 last_error text not null default '',
 version bigint not null default 1,
 created_by uuid not null references users(id),
 updated_at timestamptz not null,
 foreign key(period_id,division_id) references period_divisions(period_id,division_id),
 foreign key(space_id,period_id) references spaces(id,period_id)
);
create table connector_runs (
 id uuid primary key,
 connector_id uuid not null references connector_configs(id),
 started_at timestamptz not null,
 finished_at timestamptz,
 status text not null check(status in ('running','succeeded','failed','unchanged','needs_review')),
 snapshot_hash text not null default '',
 item_count integer not null default 0,
 error text not null default ''
);
create index connector_run_history on connector_runs(connector_id,started_at desc,id);
create table connector_items (
 id uuid primary key,
 connector_id uuid not null references connector_configs(id),
 external_key text not null,
 fingerprint text not null,
 source_row integer,
 data jsonb not null,
 state text not null default 'pending' check(state in ('pending','applied','ignored','conflict','missing')),
 issue text not null default '',
 content_id uuid references medhum_contents(id),
 first_seen_at timestamptz not null,
 last_seen_at timestamptz not null,
 unique(connector_id,external_key)
);
create table connector_bindings (
 connector_id uuid not null references connector_configs(id),
 external_key text not null,
 content_id uuid not null references medhum_contents(id),
 accepted_fingerprint text not null,
 decided_by uuid not null references users(id),
 decided_at timestamptz not null,
 primary key(connector_id,external_key)
);
create table connector_people (
 connector_id uuid not null references connector_configs(id),
 source_name text not null,
 division_id uuid not null references divisions(id),
 user_id uuid not null references users(id),
 primary key(connector_id,source_name,division_id)
);
create table connector_events (
 id uuid primary key,
 connector_id uuid not null references connector_configs(id),
 actor_id uuid references users(id),
 action text not null,
 before jsonb,
 after jsonb,
 occurred_at timestamptz not null
);
create trigger connector_events_append_only before update or delete on connector_events
 for each row execute function audit_events_append_only();
create unique index connector_one_fetch on connector_runs(connector_id) where status='running';
