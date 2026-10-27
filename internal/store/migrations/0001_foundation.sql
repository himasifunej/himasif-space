-- Foundation of Space: accounts, sessions, periods, divisions and the audit
-- log. Identities are UUIDv7 made by the application (internal/id); every
-- instant is a timestamptz (stored in UTC); calendar dates without a time
-- of day are dates.

create table users (
    id                uuid primary key,
    -- One person has one account across periods (ACC-01). The organization
    -- admin is a single shared account, apart from personal ones (ACC-12).
    kind              text not null check (kind in ('person', 'organization_admin')),
    email             text not null check (email = lower(email) and email like '%_@_%'),
    display_name      text not null check (length(display_name) between 1 and 120),
    password_hash     text,
    email_verified_at timestamptz,
    disabled_at       timestamptz,
    created_at        timestamptz not null default now(),
    updated_at        timestamptz not null default now()
);
create unique index users_email_key on users (email);
create unique index users_one_organization_admin on users (kind) where kind = 'organization_admin';

create table sessions (
    id           uuid primary key,
    user_id      uuid not null references users (id) on delete cascade,
    -- SHA-256 of the bearer token; the token itself is never stored.
    token_hash   bytea not null unique,
    client       text not null default '' check (client in ('', 'web', 'desktop', 'mobile')),
    created_at   timestamptz not null default now(),
    last_seen_at timestamptz not null default now(),
    expires_at   timestamptz not null,
    revoked_at   timestamptz
);
create index sessions_user_id on sessions (user_id);

-- A kepengurusan, as 2025/2026. Membership, roles, spaces and reports are
-- bound to a period (PER-01); a closed period is read-only (PER-05).
create table periods (
    id             uuid primary key,
    label          text not null unique check (length(label) between 1 and 40),
    starts_on      date not null,
    ends_on        date not null,
    time_zone      text not null default 'Asia/Jakarta',
    status         text not null check (status in ('preparing', 'active', 'closed')),
    predecessor_id uuid references periods (id),
    closed_at      timestamptz,
    created_at     timestamptz not null default now(),
    check (ends_on >= starts_on),
    check ((status = 'closed') = (closed_at is not null))
);

-- A division keeps its identity across periods; which divisions a period
-- has, and who is in them, is per period (Sesi 1B/1C).
create table divisions (
    id         uuid primary key,
    slug       text not null unique check (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    name       text not null check (length(name) between 1 and 80),
    created_at timestamptz not null default now()
);

-- Who did what, when, where and from which channel, with the values before
-- and after (ACC-11, NFR-05). Rows are only ever added.
create table audit_events (
    id            uuid primary key,
    occurred_at   timestamptz not null default now(),
    -- The account that acted. For the shared organization admin this is
    -- that account, not a guessed person behind it (ACC-13).
    actor_user_id uuid references users (id),
    actor_kind    text not null check (actor_kind in ('person', 'organization_admin', 'system', 'bot')),
    action        text not null,
    source        text not null check (source in ('api', 'wa', 'import', 'sync', 'system', 'cli')),
    period_id     uuid references periods (id),
    scope_type    text not null default '',
    scope_id      uuid,
    target_type   text not null default '',
    target_id     uuid,
    before        jsonb,
    after         jsonb,
    request_id    text not null default ''
);
create index audit_events_target on audit_events (target_type, target_id, occurred_at);
create index audit_events_actor on audit_events (actor_user_id, occurred_at);

create function audit_events_append_only() returns trigger language plpgsql as $$
begin
    raise exception 'audit_events is append-only';
end;
$$;
create trigger audit_events_no_update before update or delete on audit_events
    for each row execute function audit_events_append_only();
