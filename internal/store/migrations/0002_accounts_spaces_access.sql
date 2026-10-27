-- Sesi 1B–1C: invitations and email proofs, account recovery, the
-- divisions and spaces of each period, memberships with their roles and
-- groups, custom roles, the 45 appointment, archive grants and handover
-- notes. Authorization is decided by the server from these rows; nothing
-- here is copied between periods (PER-02).

-- Which divisions a period has. A division keeps its identity across
-- periods; its people, roles and work do not.
create table period_divisions (
    period_id   uuid not null references periods (id),
    division_id uuid not null references divisions (id),
    created_at  timestamptz not null default now(),
    primary key (period_id, division_id)
);

-- A space of a period: one per division, and Medhum shared by Humas and
-- Mediatek. code is stable within the period ("division-<slug>", "medhum").
create table spaces (
    id         uuid primary key,
    period_id  uuid not null references periods (id),
    code       text not null check (code ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    name       text not null check (length(name) between 1 and 80),
    kind       text not null check (kind in ('division', 'shared')),
    created_at timestamptz not null default now(),
    unique (period_id, code),
    unique (id, period_id)
);

-- The divisions of a space. Belonging to one of them opens the space; it
-- does not open the internal spaces of the other divisions.
create table space_divisions (
    space_id    uuid not null,
    period_id   uuid not null,
    division_id uuid not null,
    primary key (space_id, division_id),
    -- A space's divisions are of the space's own period.
    foreign key (space_id, period_id) references spaces (id, period_id),
    foreign key (period_id, division_id) references period_divisions (period_id, division_id)
);

-- A person's membership of a division in a period. role is the built-in
-- role; 45 is an appointment on top of it (acting45). A removed membership
-- keeps its row for history and grants nothing.
create table memberships (
    id          uuid primary key,
    period_id   uuid not null,
    division_id uuid not null,
    user_id     uuid not null references users (id),
    role        text not null check (role in ('member', 'companion', 'kadiv')),
    -- Mediatek's tekno/desain/video groupings; they grant nothing (ACC-06).
    groups      text[] not null default '{}' check (groups <@ array['tekno', 'desain', 'video']::text[]),
    source      text not null check (source in ('invitation', 'kadiv_assignment')),
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    removed_at  timestamptz,
    foreign key (period_id, division_id) references period_divisions (period_id, division_id)
);
create unique index memberships_one_active on memberships (period_id, division_id, user_id) where removed_at is null;
create unique index memberships_one_kadiv on memberships (period_id, division_id) where role = 'kadiv' and removed_at is null;
create index memberships_user on memberships (user_id, period_id) where removed_at is null;

-- Custom roles of a division in a period, with operational capabilities
-- only; the server refuses administrative ones (ACC-07, ACC-11).
create table custom_roles (
    id           uuid primary key,
    period_id    uuid not null,
    division_id  uuid not null,
    name         text not null check (length(name) between 1 and 60),
    capabilities text[] not null default '{}',
    created_at   timestamptz not null default now(),
    updated_at   timestamptz not null default now(),
    foreign key (period_id, division_id) references period_divisions (period_id, division_id)
);
create unique index custom_roles_name on custom_roles (period_id, division_id, lower(name));

create table membership_custom_roles (
    membership_id uuid not null references memberships (id),
    role_id       uuid not null references custom_roles (id) on delete cascade,
    primary key (membership_id, role_id)
);

-- The 45 of a division: at most one active per division and period, with
-- no expiry, until the original kadiv removes it (ACC-08). The index makes
-- a second appointment fail even when two are made at once.
create table acting45 (
    id           uuid primary key,
    period_id    uuid not null,
    division_id  uuid not null,
    user_id      uuid not null references users (id),
    capabilities text[] not null,
    appointed_by uuid not null references users (id),
    appointed_at timestamptz not null default now(),
    removed_at   timestamptz,
    removed_by   uuid references users (id),
    foreign key (period_id, division_id) references period_divisions (period_id, division_id),
    check ((removed_at is null) = (removed_by is null))
);
create unique index acting45_one_active on acting45 (period_id, division_id) where removed_at is null;

-- An invitation of an email to a division in a period (ACC-02–04). Only
-- the SHA-256 of its token is kept. Its status is derived: accepted,
-- revoked, expired (expires_at passed) or pending.
create table invitations (
    id          uuid primary key,
    period_id   uuid not null,
    division_id uuid not null,
    email       text not null check (email = lower(email) and email like '%_@_%'),
    token_hash  bytea not null unique,
    invited_by  uuid not null references users (id),
    created_at  timestamptz not null default now(),
    sent_at     timestamptz not null,
    expires_at  timestamptz not null,
    revoked_at  timestamptz,
    revoked_by  uuid references users (id),
    accepted_at timestamptz,
    accepted_by uuid references users (id),
    foreign key (period_id, division_id) references period_divisions (period_id, division_id),
    check (revoked_at is null or accepted_at is null),
    check ((accepted_at is null) = (accepted_by is null))
);
-- One open invitation per email and scope; an expired one is resent.
create unique index invitations_one_open on invitations (period_id, division_id, email)
    where revoked_at is null and accepted_at is null;

alter table memberships add column invitation_id uuid references invitations (id);

-- Single-use tokens sent by email: the proof that a new account owns the
-- invited address, account recovery, and the proof of a new recovery
-- address of the organization admin. Only their SHA-256 is kept.
create table auth_tokens (
    id            uuid primary key,
    purpose       text not null check (purpose in ('invitation_email', 'recovery', 'admin_email')),
    token_hash    bytea not null unique,
    email         text not null check (email = lower(email)),
    user_id       uuid references users (id),
    invitation_id uuid references invitations (id),
    created_at    timestamptz not null default now(),
    expires_at    timestamptz not null,
    used_at       timestamptz,
    check (purpose <> 'invitation_email' or invitation_id is not null),
    check (purpose = 'invitation_email' or user_id is not null)
);
create index auth_tokens_user on auth_tokens (user_id) where used_at is null;
create index auth_tokens_invitation on auth_tokens (invitation_id) where used_at is null;

-- Read-only access of a member of a scope (period, division) to one space
-- of a closed period, given by that scope's kadiv or the admin (PER-04).
-- It counts only while the grantee is an active member of the scope.
create table archive_grants (
    id          uuid primary key,
    period_id   uuid not null,
    division_id uuid not null,
    user_id     uuid not null references users (id),
    space_id    uuid not null references spaces (id),
    granted_by  uuid not null references users (id),
    created_at  timestamptz not null default now(),
    revoked_at  timestamptz,
    revoked_by  uuid references users (id),
    foreign key (period_id, division_id) references period_divisions (period_id, division_id),
    check ((revoked_at is null) = (revoked_by is null))
);
create unique index archive_grants_one_active on archive_grants (period_id, division_id, user_id, space_id) where revoked_at is null;
create index archive_grants_user on archive_grants (user_id, space_id) where revoked_at is null;

-- The handover checklist of a period (PER-07): notes, not a copy of work.
create table period_handovers (
    period_id           uuid primary key references periods (id),
    successor_period_id uuid references periods (id),
    incoming_leadership text not null default '',
    admin_custodian     text not null default '',
    bot_management      text not null default '',
    archive_plan        text not null default '',
    remaining_work      text not null,
    updated_at          timestamptz not null default now(),
    updated_by          uuid not null references users (id),
    check (successor_period_id is null or successor_period_id <> period_id)
);
