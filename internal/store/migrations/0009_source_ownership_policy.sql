-- A binding records both the source and the app baseline. Following updates is
-- an explicit per-binding choice, never implied by matching a title.
alter table connector_bindings add column follow_updates boolean not null default false;
alter table connector_bindings add column accepted_data jsonb;
alter table connector_bindings add column accepted_content jsonb;
create table medhum_policies (
 space_id uuid primary key references spaces(id),
 posting_clock text check(posting_clock is null or posting_clock ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
 design_clock text check(design_clock is null or design_clock ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
 version bigint not null default 1,
 updated_by uuid not null references users(id),
 updated_at timestamptz not null
);
create table connector_mirror_runs (
 id uuid primary key,
 connector_id uuid not null references connector_configs(id),
 started_at timestamptz not null,
 finished_at timestamptz,
 status text not null check(status in ('running','succeeded','failed','unknown')),
 item_count integer not null default 0,
 error text not null default ''
);
create unique index connector_one_mirror on connector_mirror_runs(connector_id) where status='running';
