-- Restore quarantine is durable: changing an environment file back to live
-- cannot activate a database that was explicitly marked as an isolated copy.
create table runtime_control (
 singleton boolean primary key default true check(singleton),
 mode text not null default 'live' check(mode in ('live','isolated_restore')),
 restore_id uuid,
 restored_at timestamptz,
 reason text not null default '',
 check((mode='live') or (restore_id is not null and restored_at is not null and length(reason)>0))
);
insert into runtime_control(singleton,mode) values(true,'live');
