-- PostgreSQL emits NOTIFY only on COMMIT, including writes made by workers,
-- imports and other server instances. Payloads contain no task/note/user data.
create function space_period_changed() returns trigger language plpgsql as $$
begin
 if TG_OP='DELETE' then
  perform pg_notify('space_changes',old.period_id::text);
 else
  perform pg_notify('space_changes',new.period_id::text);
 end if;
 return null;
end;
$$;
create trigger tasks_realtime after insert or update or delete on tasks for each row execute function space_period_changed();
create trigger task_spaces_realtime after insert or update or delete on task_spaces for each row execute function space_period_changed();
create trigger meetings_realtime after insert or update or delete on meetings for each row execute function space_period_changed();
create trigger notes_realtime after insert or update or delete on notes for each row execute function space_period_changed();
create trigger memberships_realtime after insert or update or delete on memberships for each row execute function space_period_changed();
