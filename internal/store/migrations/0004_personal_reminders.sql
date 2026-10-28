-- Sesi 2A (reminder pribadi): the reminders a person keeps for themselves,
-- their read-only shares, and each person's default for new reminders.
--
-- A reminder is owned by one account and is private by default (PRI-02). A
-- share grants one other account read-only access; it is attached to that
-- account, never to a position, so a replacement kadiv, the 45, the admin,
-- the BPH or a later period's successor inherit nothing (PRI-03, PRI-04).
-- The owner's default for new reminders never reopens the old ones
-- (PRI-05). The history of every change is in audit_events, in the same
-- transaction as the change, readable by the owner (PRI-05, AC-09).
--
-- A reminder is never a division's task: there is no space or division
-- here, and nothing grants a recipient the right to change it, finish it,
-- change its owner or turn it into a task (PRI-07). Space, archive and
-- directory endpoints never read these tables (PRI-08).

-- The time a reminder is due. Space distinguishes a calendar date with no
-- time of day from an instant (PRD MED-04, MED-05, NFR-10): a date-only
-- reminder keeps due_on and no due_at; an instant reminder keeps due_at
-- (stored UTC) and no due_on. time_zone is the zone the owner set it in,
-- for showing the date or instant, Asia/Jakarta by default.
create table reminders (
    id           uuid primary key,
    owner_id     uuid not null references users (id),
    title        text not null check (length(title) between 1 and 200),
    notes        text not null default '' check (length(notes) <= 4000),
    due_kind     text not null check (due_kind in ('date', 'instant')),
    due_on       date,
    due_at       timestamptz,
    time_zone    text not null default 'Asia/Jakarta' check (length(time_zone) between 1 and 60),
    -- Private by default; the owner may toggle sharing on any reminder at
    -- any time (PRI-02). shared is a convenience flag the owner controls;
    -- the recipients are the rows of reminder_shares.
    shared       boolean not null default false,
    completed_at timestamptz,
    created_at   timestamptz not null default now(),
    updated_at   timestamptz not null default now(),
    -- Exactly one of due_on / due_at, matching due_kind.
    check ((due_kind = 'date') = (due_on is not null)),
    check ((due_kind = 'instant') = (due_at is not null))
);
create index reminders_owner on reminders (owner_id, id);

-- A read-only share of a reminder to another account (PRI-05). recipient_id
-- is the account chosen at the time of sharing, not a position: it does not
-- follow a role. The owner may not share a reminder with themselves.
create table reminder_shares (
    reminder_id  uuid not null references reminders (id) on delete cascade,
    recipient_id uuid not null references users (id),
    shared_by    uuid not null references users (id),
    created_at   timestamptz not null default now(),
    primary key (reminder_id, recipient_id),
    check (recipient_id <> shared_by)
);
create index reminder_shares_recipient on reminder_shares (recipient_id, reminder_id);

-- Each person's default for whether a new reminder starts shared (PRI-02).
-- Changing it does not touch any existing reminder (PRI-05): it is read
-- only when a reminder is created.
create table reminder_preferences (
    user_id       uuid primary key references users (id),
    default_share boolean not null default false,
    updated_at    timestamptz not null default now()
);
