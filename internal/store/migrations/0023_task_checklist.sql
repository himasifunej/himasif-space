-- Checklist items inside one task (canvas A detail). Items belong to the task,
-- never become tasks of their own, and are soft-deleted so history stays.
create table task_checklist_items (
    id uuid primary key,
    task_id uuid not null references tasks(id),
    body text not null check(length(body) between 1 and 500),
    position integer not null,
    done boolean not null default false,
    created_by uuid not null references users(id),
    created_at timestamptz not null,
    updated_at timestamptz not null,
    done_by uuid references users(id),
    done_at timestamptz,
    deleted_at timestamptz
);
create index task_checklist_items_task on task_checklist_items(task_id, position, id) where deleted_at is null;
create trigger task_checklist_realtime after insert or update or delete on task_checklist_items for each row execute function space_task_child_changed();
