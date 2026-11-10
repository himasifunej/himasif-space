-- NIM, linking WhatsApp by direct message, and invitation bounces.
-- Safe for existing data: only adds columns, relaxes one NOT NULL with a
-- check that every existing row already satisfies, and adds triggers.

-- NIM is derived from the account's verified email, never typed: a student
-- account is <NIM>@mail.unej.ac.id (the domain the HIMASIF website uses, see
-- docs/handoff/anggota-catatan-rapat-rencana.md §8). Generated, so existing
-- accounts get it at once and nobody can edit it apart from the email.
alter table users add column nim text generated always as (
    case when email ~ '^[0-9]{8,20}@mail\.unej\.ac\.id$' then split_part(email, '@', 1) end
) stored;
create unique index users_nim on users (nim) where nim is not null;

-- A permanent bounce or complaint reported by Amazon SES for a pending
-- invitation. Sending again clears it.
alter table invitations add column bounced_at timestamptz;
alter table invitations add column bounce_reason text not null default '';

-- Replies to a link code sent to the bot in a direct message have no group.
alter table wa_outbox alter column group_id drop not null;
alter table wa_outbox add constraint wa_outbox_group_or_dm check (group_id is not null or source_kind = 'link_dm');
create or replace function wa_outbox_target_snapshot() returns trigger language plpgsql as $$
begin
 if new.target_name='' then
  if new.group_id is null then
   new.target_name := 'Pesan pribadi';
  else
   select name into new.target_name from wa_groups where id=new.group_id;
  end if;
 end if;
 return new;
end;
$$;

-- A member's Space shows "WhatsApp tertaut" without reloading.
create trigger wa_identities_realtime after insert or update or delete on wa_identities for each row execute function space_period_changed();
create trigger invitations_realtime after insert or update or delete on invitations for each row execute function space_period_changed();
