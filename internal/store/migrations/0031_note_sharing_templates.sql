-- Catatan C6 and C5 (docs/handoff/anggota-catatan-rapat-rencana.md).
-- Additive: older servers ignore the new column and table.

-- Tags of a note, lowercase words chosen by its writers.
alter table notes add column tags text[] not null default '{}';

-- "Publik lewat tautan": anyone holding the link reads the note without
-- signing in. Only the SHA-256 of the random token is kept, as for
-- invitations and !link codes. Revoking (or making a new link) sets
-- revoked_at; one live link per note.
create table note_public_links (
    id             uuid primary key,
    note_id        uuid not null references notes (id) on delete cascade,
    token_hash     bytea not null unique,
    created_by     uuid not null references users (id),
    created_at     timestamptz not null,
    revoked_at     timestamptz,
    revoked_by     uuid references users (id),
    last_viewed_at timestamptz,
    view_count     integer not null default 0
);
create unique index note_public_links_live on note_public_links (note_id) where revoked_at is null;
create trigger note_public_links_realtime after insert or update or delete on note_public_links
    for each row execute function space_note_child_changed();

-- Built-in templates C5 adds: Evaluasi and Rapat singkat.
alter table notes drop constraint if exists notes_template_kind_check;
alter table notes add constraint notes_template_kind_check
    check (template_kind in ('meeting', 'schedule', 'free', 'custom', 'evaluation', 'brief'));
