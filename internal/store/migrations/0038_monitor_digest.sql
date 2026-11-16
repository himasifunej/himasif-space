-- Ringkasan mingguan BPH (PRD MON-08): sent Monday 07.00 WIB to holders
-- of org.monitor, by WhatsApp direct message, email or both, unless they
-- turned it off. Additive: older servers ignore the tables, and the relaxed
-- outbox check still accepts every row they write.

-- A person's choice; without a row the digest goes by both channels.
create table monitor_digest_preferences (
    user_id    uuid primary key references users (id),
    enabled    boolean not null default true,
    channel    text not null default 'both' check (channel in ('whatsapp', 'email', 'both')),
    updated_at timestamptz not null
);

-- One digest per person per week, so restarts and other instances never
-- send it twice.
create table monitor_digest_runs (
    user_id    uuid not null references users (id),
    period_id  uuid not null references periods (id),
    week_start date not null,
    channels   text[] not null,
    created_at timestamptz not null,
    email_sent_at timestamptz,
    email_error   text not null default '',
    primary key (user_id, week_start)
);

-- The digest is a direct message, like the link replies.
alter table wa_outbox drop constraint wa_outbox_group_or_dm;
alter table wa_outbox add constraint wa_outbox_group_or_dm check (group_id is not null or source_kind in ('link_dm', 'monitor_digest'));
