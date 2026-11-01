-- Heartbeats contain service progress only: no payloads, addresses, QR, user
-- names, source credentials or private reminder counts.
create table worker_health (
 instance_id uuid not null,
 worker text not null,
 runtime_mode text not null,
 state text not null,
 progressed_at timestamptz not null,
 succeeded_at timestamptz,
 failed_at timestamptz,
 primary key(instance_id,worker)
);
create index worker_health_latest on worker_health(runtime_mode,worker,progressed_at desc);
alter table wa_connections add column last_polled_at timestamptz;
