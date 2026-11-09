-- Lampiran berkas di tugas (canvas A, Tahap 6b). Blobs live in the same
-- private filestore as note attachments (0014) and are never deleted here:
-- removal is a soft delete, as for notes. Additive; older servers ignore it.
create table task_attachments (
    id           uuid primary key,
    task_id      uuid not null references tasks(id),
    name         text not null check (length(name) between 1 and 300),
    content_type text not null,
    size_bytes   bigint not null check (size_bytes > 0),
    storage_key  uuid not null unique,
    sha256       text not null check (length(sha256) = 64),
    added_by     uuid not null references users(id),
    created_at   timestamptz not null,
    removed_at   timestamptz,
    removed_by   uuid references users(id),
    check ((removed_at is null) = (removed_by is null))
);
create index task_attachments_task on task_attachments(task_id, created_at, id) where removed_at is null;
-- Retained after removal, so a retry of an old upload cannot resurrect a
-- deliberately removed file (the same rule as attachment_upload_receipts).
create table task_attachment_receipts (
    id          uuid primary key,
    task_id     uuid not null references tasks(id),
    actor_id    uuid not null references users(id),
    storage_key uuid not null unique,
    sha256      text not null,
    metadata    jsonb not null,
    created_at  timestamptz not null,
    removed_at  timestamptz
);
create trigger task_attachments_realtime after insert or update or delete on task_attachments
    for each row execute function space_task_child_changed();
