-- Preserve append-only note history while making deletion hide the object.
alter table notes add column deleted_at timestamptz;

-- An editor belongs to one authenticated session, not every device signed in
-- to the same account. Existing account-only leases must be reacquired.
alter table note_locks add column session_id uuid;
update note_locks set expires_at = least(expires_at, now()) where session_id is null;

-- Keep recovery copies independent across sessions. No session FK: logging
-- out, revoking or pruning an old session must not erase its recovery draft.
-- Existing drafts retain their contents and remain recoverable by their owner.
alter table note_drafts add column id uuid not null default gen_random_uuid();
alter table note_drafts add column session_id uuid;
alter table note_drafts drop constraint note_drafts_pkey;
alter table note_drafts add primary key (id);
create unique index note_drafts_session on note_drafts (note_id, user_id, session_id)
    where session_id is not null;
create index note_drafts_recovery on note_drafts (note_id, user_id, saved_at desc, id);
