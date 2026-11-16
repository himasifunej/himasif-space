-- Pemantauan organisasi oleh BPH (OPEN-15, keputusan owner 16 Nov 2026).
-- The organization admin grants org.monitor to one account for one period;
-- a successor in the same post does not inherit it (PRI-04). Closing the
-- period revokes every grant of it (revoked_reason 'period_closed').
-- Additive: older servers ignore the table.
create table monitor_grants (
    id             uuid primary key,
    period_id      uuid not null references periods (id),
    user_id        uuid not null references users (id),
    granted_by     uuid not null references users (id),
    granted_at     timestamptz not null,
    revoked_at     timestamptz,
    revoked_by     uuid references users (id),
    revoked_reason text not null default '' check (revoked_reason in ('', 'admin', 'period_closed')),
    check ((revoked_at is null) = (revoked_reason = ''))
);
create unique index monitor_grants_one_active on monitor_grants (period_id, user_id) where revoked_at is null;
create index monitor_grants_user on monitor_grants (user_id, period_id) where revoked_at is null;
-- The admin's log of monitoring reads and actions.
create index audit_events_monitor on audit_events (period_id, occurred_at desc) where action like 'monitor.%';
