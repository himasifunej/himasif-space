-- Meetings retain their identity across schedule changes and cancellation.
create table meetings (
 id uuid primary key,
 period_id uuid not null references periods(id),
 space_id uuid not null,
 owner_division_id uuid not null,
 organizer_id uuid not null references users(id),
 title text not null check(length(title) between 1 and 240),
 description text not null default '',
 location text not null default '',
 join_url text not null default '',
 starts_at timestamptz not null,
 ends_at timestamptz not null,
 time_zone text not null default 'Asia/Jakarta',
 note_id uuid references notes(id),
 cancelled boolean not null default false,
 reminder_minutes integer[] not null default '{1440,60,15}',
 version bigint not null default 1,
 created_at timestamptz not null,
 updated_at timestamptz not null,
 foreign key(space_id,period_id) references spaces(id,period_id),
 foreign key(period_id,owner_division_id) references period_divisions(period_id,division_id),
 check(ends_at>starts_at)
);
create index meetings_space_schedule on meetings(space_id,starts_at,id);
create table meeting_participants (
 meeting_id uuid not null references meetings(id) on delete cascade,
 user_id uuid not null references users(id),
 arrived_at timestamptz,
 attendance_note text not null default '',
 attendance_status text not null default 'unrecorded' check(attendance_status in ('unrecorded','present','excused','absent')),
 confirmed_by uuid references users(id),
 primary key(meeting_id,user_id)
);
create table delivery_events (
 id uuid primary key,
 event_key text not null unique,
 source_kind text not null check(source_kind in ('task','meeting','reminder','ping')),
 source_id uuid not null,
 source_version bigint not null,
 space_id uuid references spaces(id),
 title text not null,
 body text not null default '',
 time_zone text not null,
 scheduled_at timestamptz not null,
 created_at timestamptz not null,
 cancelled_at timestamptz
);
create table delivery_recipients (
 id uuid primary key,
 event_id uuid not null references delivery_events(id),
 recipient_id uuid not null references users(id),
 channel text not null check(channel in ('device','whatsapp')),
 status text not null default 'queued' check(status in ('queued','accepted','failed','unknown','cancelled')),
 attempts integer not null default 0,
 available_at timestamptz not null,
 accepted_at timestamptz,
 seen_at timestamptz,
 snoozed_until timestamptz,
 last_error text not null default '',
 unique(event_id,recipient_id,channel)
);
create index delivery_pending on delivery_recipients(recipient_id,available_at) where status='queued';
create table notification_policies (
 period_id uuid not null,
 division_id uuid not null,
 task_days integer[] not null default '{2,1,0}',
 local_hour integer not null default 6 check(local_hour between 0 and 23),
 local_minute integer not null default 0 check(local_minute between 0 and 59),
 overdue_days integer not null default 1 check(overdue_days between 1 and 30),
 primary key(period_id,division_id),
 foreign key(period_id,division_id) references period_divisions(period_id,division_id)
);
create table task_notification_policies (
 task_id uuid primary key references tasks(id),
 task_days integer[] not null,
 local_hour integer not null check(local_hour between 0 and 23),
 local_minute integer not null check(local_minute between 0 and 59),
 overdue_days integer not null check(overdue_days between 1 and 30)
);
-- Idempotency receipts are account scoped and committed with each mutation.
create table offline_mutations (
 user_id uuid not null references users(id),
 mutation_id uuid not null,
 request_hash text not null,
 response jsonb not null,
 created_at timestamptz not null,
 primary key(user_id,mutation_id)
);

-- Notification generations change only when scheduling semantics change,
-- rather than on every progress/comment/title revision.
alter table tasks add column notification_revision bigint not null default 1;
alter table reminders add column notification_revision bigint not null default 1;
create function task_notification_generation() returns trigger language plpgsql as $$
begin
 if row(new.deadline_date,new.deadline_at,new.assignee_id) is distinct from row(old.deadline_date,old.deadline_at,old.assignee_id)
 or (old.progress not in ('todo','doing') and new.progress in ('todo','doing')) then
  new.notification_revision=old.notification_revision+1;
 end if;
 return new;
end;
$$;
create trigger task_notification_generation before update on tasks for each row execute function task_notification_generation();
create function reminder_notification_generation() returns trigger language plpgsql as $$
begin
 if row(new.due_on,new.due_at,new.time_zone,new.completed_at) is distinct from row(old.due_on,old.due_at,old.time_zone,old.completed_at) then
  new.notification_revision=old.notification_revision+1;
 end if;
 return new;
end;
$$;
create trigger reminder_notification_generation before update on reminders for each row execute function reminder_notification_generation();
alter table delivery_events add column related_kind text not null default '';
alter table delivery_events add column related_id uuid;
create unique index meetings_one_notulensi on meetings(note_id) where note_id is not null;
