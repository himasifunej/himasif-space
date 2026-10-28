-- A canonical task is linked into spaces; personal jobdesk is a projection.
create table tasks (
    id uuid primary key,
    period_id uuid not null references periods(id),
    owner_division_id uuid not null,
    title text not null check (length(title) between 1 and 240),
    description text not null default '',
    expected_output text not null check (length(expected_output) between 1 and 10000),
    category text not null default '',
    assignee_id uuid not null references users(id),
    deadline_date date,
    deadline_at timestamptz,
    posting_date date,
    posting_at timestamptz,
    progress text not null check (progress in ('todo','doing','submitted','done','cancelled')),
    match_status text not null check (match_status in ('none','pending','verified','rejected')),
    resume_progress text not null default 'todo' check (resume_progress in ('todo','doing')),
    rank bigint not null,
    version bigint not null default 1 check (version > 0),
    created_by uuid not null references users(id),
    created_at timestamptz not null,
    updated_at timestamptz not null,
    unique(id,period_id),
    foreign key(period_id,owner_division_id) references period_divisions(period_id,division_id),
    check ((deadline_date is null) <> (deadline_at is null)),
    check (posting_date is null or posting_at is null)
);
create index tasks_board on tasks(period_id,progress,rank,id);
create index tasks_assignee on tasks(assignee_id,period_id);
create table task_spaces (
    task_id uuid not null,
    space_id uuid not null,
    period_id uuid not null,
    primary key(task_id,space_id),
    foreign key(task_id,period_id) references tasks(id,period_id),
    foreign key(space_id,period_id) references spaces(id,period_id)
);
create index task_spaces_space on task_spaces(space_id,task_id);
create table task_comments (
    id uuid primary key,
    task_id uuid not null references tasks(id),
    author_id uuid not null references users(id),
    body text not null check(length(body) between 1 and 10000),
    created_at timestamptz not null
);
create table task_help_requests (
    id uuid primary key,
    task_id uuid not null references tasks(id),
    user_id uuid not null references users(id),
    message text not null default '',
    status text not null check(status in ('pending','approved','rejected')),
    created_at timestamptz not null,
    decided_at timestamptz,
    decided_by uuid references users(id),
    check ((status = 'pending') = (decided_at is null)),
    check ((decided_at is null) = (decided_by is null))
);
create unique index task_help_one_pending on task_help_requests(task_id,user_id) where status='pending';
create table task_participants (
    task_id uuid not null references tasks(id),
    user_id uuid not null references users(id),
    help_id uuid not null references task_help_requests(id),
    primary key(task_id,user_id)
);
create table task_evidence (
    id uuid primary key,
    task_id uuid not null references tasks(id),
    submitted_by uuid not null references users(id),
    url text not null default '',
    note text not null default '',
    match_status text not null check(match_status in ('pending','verified','rejected')),
    submitted_at timestamptz not null,
    reviewed_at timestamptz,
    reviewed_by uuid references users(id),
    review_reason text not null default '',
    check(length(url)>0 or length(note)>0),
    check ((match_status = 'pending') = (reviewed_at is null)),
    check ((reviewed_at is null) = (reviewed_by is null))
);
create unique index task_evidence_one_pending on task_evidence(task_id) where match_status='pending';
create table task_events (
    id uuid primary key,
    task_id uuid not null references tasks(id),
    actor_id uuid not null references users(id),
    action text not null,
    reason text not null default '',
    created_at timestamptz not null,
    before jsonb,
    after jsonb
);
create index task_events_task on task_events(task_id,created_at,id);
create trigger task_events_no_update before update or delete on task_events
    for each row execute function audit_events_append_only();
create table task_notifications (
    id uuid primary key,
    task_id uuid not null references tasks(id),
    recipient_id uuid not null references users(id),
    title text not null,
    body text not null,
    created_at timestamptz not null,
    read_at timestamptz
);
create index task_notifications_recipient on task_notifications(recipient_id,created_at desc,id);
