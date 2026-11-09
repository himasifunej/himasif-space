-- Nomor tugas urut per Space (canvas A, Tahap 6a): #T12 is a task's number
-- in one Space; a task in two Spaces has two numbers. Numbers are never
-- reused. Servers before this migration insert task_spaces without a number
-- and fail on the not-null constraint: stop the old API before migrating.
alter table task_spaces add column number bigint;
create table space_task_counters (
    space_id    uuid primary key references spaces(id),
    last_number bigint not null check (last_number >= 0)
);
-- Backfill: by when the task was created, then id, per Space.
update task_spaces ts set number = n.rn
from (select ts2.task_id, ts2.space_id,
             row_number() over (partition by ts2.space_id order by t.created_at, t.id) rn
      from task_spaces ts2 join tasks t on t.id = ts2.task_id) n
where ts.task_id = n.task_id and ts.space_id = n.space_id;
insert into space_task_counters(space_id, last_number)
select space_id, max(number) from task_spaces group by space_id;
alter table task_spaces alter column number set not null;
alter table task_spaces add constraint task_spaces_number unique (space_id, number);
