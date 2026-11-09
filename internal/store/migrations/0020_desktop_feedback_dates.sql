-- Unknown meeting end stays unknown; existing schedules retain their end.
alter table meetings alter column ends_at drop not null;

-- Planned work is independent of posting/publication and the required deadline.
alter table tasks add column planned_start_date date;
alter table tasks add column planned_start_at timestamptz;
alter table tasks add constraint tasks_planned_start_kind check (planned_start_date is null or planned_start_at is null);
