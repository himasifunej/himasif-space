alter table note_attachments add column storage_key uuid unique;
alter table note_attachments add column sha256 text not null default '';
alter table note_attachments add check(storage_key is null or (status='ready' and length(sha256)=64));
-- Retained after unlinking an attachment, so a retry of an old upload cannot
-- resurrect a deliberately removed file. Blobs are immutable and retained
-- until an operator's explicit retention/backup procedure permits collection.
create table attachment_upload_receipts (
 id uuid primary key,
 note_id uuid not null references notes(id),
 actor_id uuid not null references users(id),
 storage_key uuid not null unique,
 sha256 text not null,
 metadata jsonb not null,
 created_at timestamptz not null,
 removed_at timestamptz
);
