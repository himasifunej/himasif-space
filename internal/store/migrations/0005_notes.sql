-- Sesi 2B (Notes dan template): notes ter-scope ke satu Space (divisi atau
-- Medhum), isi dokumen blok (PRD NTE-01), template copy-on-create (NTE-02,
-- NTE-09), satu editor aktif dengan kunci bermasa berlaku (NTE-06, NTE-08),
-- versi/konflik eksplisit (AC-34), draf pemulihan, komentar, lampiran, dan
-- tautan ke tugas/reminder tanpa membuka isi note (NTE-05, NTE-07).
--
-- Otorisasi diputuskan server dari keanggotaan Space: baca lewat notes.read,
-- kelola lewat notes.manage pada divisi pemilik. Periode tertutup hanya-baca.
-- Note internal divisi terbatas di ruangnya; note Medhum di ruang bersama.

-- A note. content is the block document (NTE-01). owner_division_id fixes
-- the division that manages it (one of the space's divisions), like a task.
-- version rises on each content/title change for explicit conflicts.
create table notes (
    id                uuid primary key,
    space_id          uuid not null,
    period_id         uuid not null,
    owner_division_id uuid not null,
    title             text not null check (length(title) between 1 and 200),
    content           jsonb not null,
    template_kind     text not null check (template_kind in ('meeting', 'schedule', 'free', 'custom')),
    -- Reserved for the meeting object (NTE-03); null until the meetings
    -- module (Sesi 2C). This backend stores it but creates no meeting.
    meeting_id        uuid,
    version           bigint not null default 1 check (version > 0),
    created_by        uuid not null references users (id),
    created_at        timestamptz not null,
    updated_at        timestamptz not null,
    unique (id, period_id),
    foreign key (space_id, period_id) references spaces (id, period_id),
    foreign key (period_id, owner_division_id) references period_divisions (period_id, division_id)
);
create index notes_space on notes (space_id, updated_at desc, id);

-- One active editor per note (NTE-06). The lock has an expiry so a
-- disconnected editor never holds it forever (NTE-08); a takeover replaces
-- the row but never touches note_drafts.
create table note_locks (
    note_id     uuid primary key references notes (id) on delete cascade,
    editor_id   uuid not null references users (id),
    acquired_at timestamptz not null,
    expires_at  timestamptz not null
);

-- A per-(note, user) working draft for recovery after a disconnect
-- (NTE-08). It never overwrites the official note until a valid PUT.
create table note_drafts (
    note_id      uuid not null references notes (id) on delete cascade,
    user_id      uuid not null references users (id),
    title        text not null,
    content      jsonb not null,
    base_version bigint not null,
    saved_at     timestamptz not null,
    primary key (note_id, user_id)
);

create table note_comments (
    id         uuid primary key,
    note_id    uuid not null references notes (id) on delete cascade,
    author_id  uuid not null references users (id),
    body       text not null check (length(body) between 1 and 10000),
    created_at timestamptz not null
);
create index note_comments_note on note_comments (note_id, created_at, id);

-- Attachment metadata. Binary upload is out of scope this session; an
-- attachment with no url is pending until uploaded (NTE-09).
create table note_attachments (
    id           uuid primary key,
    note_id      uuid not null references notes (id) on delete cascade,
    name         text not null check (length(name) between 1 and 300),
    url          text not null default '',
    content_type text not null default '',
    size_bytes   bigint not null default 0 check (size_bytes >= 0),
    status       text not null check (status in ('pending', 'ready')),
    added_by     uuid not null references users (id),
    created_at   timestamptz not null,
    check ((status = 'ready') = (length(url) > 0))
);
create index note_attachments_note on note_attachments (note_id, created_at, id);

-- A reference from a note to a task or reminder it gave rise to (NTE-05).
-- It is only a pointer: it does not open or share the note's content
-- (NTE-07). target_id is not a foreign key because it may point at either
-- table; the server checks the target exists and the caller's authority.
create table note_links (
    id          uuid primary key,
    note_id     uuid not null references notes (id) on delete cascade,
    target_type text not null check (target_type in ('task', 'reminder')),
    target_id   uuid not null,
    created_by  uuid not null references users (id),
    created_at  timestamptz not null,
    unique (note_id, target_type, target_id)
);
create index note_links_note on note_links (note_id, created_at, id);

-- Append-only history of a note's changes (AC-33, AC-34).
create table note_events (
    id         uuid primary key,
    note_id    uuid not null references notes (id),
    actor_id   uuid not null references users (id),
    action     text not null,
    reason     text not null default '',
    created_at timestamptz not null,
    before     jsonb,
    after      jsonb
);
create index note_events_note on note_events (note_id, created_at, id);
create trigger note_events_no_update before update or delete on note_events
    for each row execute function audit_events_append_only();

-- Saved templates of a division (NTE-02). Built-in templates (meeting,
-- schedule, free) are not rows: the server supplies them. Changing a saved
-- template never rewrites notes already created from it (NTE-09): notes
-- copy content on create.
create table note_templates (
    id          uuid primary key,
    space_id    uuid not null,
    period_id   uuid not null,
    division_id uuid not null,
    name        text not null check (length(name) between 1 and 120),
    content     jsonb not null,
    created_by  uuid not null references users (id),
    created_at  timestamptz not null,
    updated_at  timestamptz not null,
    foreign key (space_id, period_id) references spaces (id, period_id),
    foreign key (period_id, division_id) references period_divisions (period_id, division_id)
);
create index note_templates_space on note_templates (space_id, lower(name), id);
