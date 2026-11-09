-- Cutoff and suppression preserve the original receipt rather than inventing
-- cancellation, acceptance, or a human read after restoring an old database.
alter table runtime_control add column outbound_not_before timestamptz;
alter table runtime_control add column activation_id uuid;
alter table delivery_events add column suppressed_at timestamptz;
alter table delivery_events add column suppression_reason text not null default '';
alter table wa_outbox add column suppressed_at timestamptz;
alter table wa_outbox add column suppression_reason text not null default '';

-- All WA producers, including reports and followups, share this fence.
create function wa_restore_cutoff() returns trigger language plpgsql as $$
declare cutoff timestamptz;
begin
 select outbound_not_before into cutoff from runtime_control where singleton;
 if cutoff is not null and new.available_at<cutoff then
  new.suppressed_at=now();
  new.suppression_reason='Scheduled before restore activation cutoff';
 end if;
 return new;
end;
$$;
create trigger wa_restore_cutoff before insert on wa_outbox
 for each row execute function wa_restore_cutoff();
