create table wa_connections (
 id uuid primary key,
 period_id uuid not null,
 division_id uuid not null,
 display_name text not null,
 language text not null default 'id',
 greeting text not null default '',
 templates jsonb not null default '{}',
 enabled boolean not null default false,
 status text not null default 'disconnected',
 account_jid text not null default '',
 event_cursor bigint not null default 0,
 last_event_at timestamptz,
 last_error text not null default '',
 version bigint not null default 1,
 created_by uuid not null references users(id),
 updated_at timestamptz not null,
 unique(period_id,division_id),
 foreign key(period_id,division_id) references period_divisions(period_id,division_id)
);
create table wa_groups (
 id uuid primary key,
 connection_id uuid not null references wa_connections(id),
 space_id uuid not null unique references spaces(id),
 jid text not null,
 name text not null,
 commands_enabled boolean not null default true,
 unique(connection_id,jid)
);
create table wa_identities (
 period_id uuid not null,
 division_id uuid not null,
 user_id uuid not null references users(id),
 jid text not null,
 verified_by uuid not null references users(id),
 verified_at timestamptz not null,
 primary key(period_id,jid),
 foreign key(period_id,division_id) references period_divisions(period_id,division_id)
);
create table wa_link_challenges (
 id uuid primary key,
 period_id uuid not null,
 division_id uuid not null,
 user_id uuid not null references users(id),
 token_hash bytea not null unique,
 expires_at timestamptz not null,
 used_at timestamptz,
 foreign key(period_id,division_id) references period_divisions(period_id,division_id)
);
create table wa_bridge_events (
 connection_id uuid not null references wa_connections(id),
 sequence bigint not null,
 data jsonb not null,
 created_at timestamptz not null,
 primary key(connection_id,sequence)
);
create table wa_inbound (
 id uuid primary key,
 connection_id uuid not null references wa_connections(id),
 group_id uuid references wa_groups(id),
 group_jid text not null,
 sender_jid text not null,
 sender_alt_jid text not null default '',
 message_id text not null,
 envelope_id text not null,
 revision text not null,
 kind text not null,
 body text not null,
 quoted_message_id text not null default '',
 mentions jsonb not null default '[]',
 media jsonb,
 occurred_at timestamptz,
 received_at timestamptz not null,
 state text not null default 'queued' check(state in ('queued','processing','applied','review','ignored','failed')),
 error text not null default '',
 unique(connection_id,envelope_id,revision)
);
create index wa_inbound_pending on wa_inbound(received_at,id) where state='queued';
create table wa_outbox (
 id uuid primary key,
 event_key text not null unique,
 connection_id uuid not null references wa_connections(id),
 group_id uuid not null references wa_groups(id),
 source_kind text not null,
 source_id uuid,
 actor_id uuid references users(id),
 payload jsonb not null,
 status text not null default 'queued' check(status in ('queued','sending','accepted','delivered','read','failed','unknown','cancelled')),
 attempts integer not null default 0,
 available_at timestamptz not null,
 lease_until timestamptz,
 message_id text not null default '',
 last_error text not null default '',
 created_at timestamptz not null,
 updated_at timestamptz not null
);
create index wa_outbox_queue on wa_outbox(available_at,id) where status in ('queued','unknown','sending');
create table wa_custom_commands (
 id uuid primary key,
 connection_id uuid not null references wa_connections(id),
 trigger text not null,
 response text not null default '',
 asset_id text not null default '',
 media_type text not null default '',
 group_ids uuid[] not null default '{}',
 version bigint not null default 1,
 updated_by uuid not null references users(id),
 updated_at timestamptz not null,
 unique(connection_id,trigger)
);
create table wa_assets (
 id uuid primary key,
 connection_id uuid not null references wa_connections(id),
 bridge_asset_id text not null,
 name text not null,
 mime text not null,
 size bigint not null,
 source_message_id uuid references wa_inbound(id),
 created_by uuid references users(id),
 created_at timestamptz not null,
 unique(connection_id,bridge_asset_id)
);
create table wa_events (
 id uuid primary key,
 connection_id uuid not null references wa_connections(id),
 actor_id uuid references users(id),
 action text not null,
 detail jsonb,
 occurred_at timestamptz not null
);
create trigger wa_events_append_only before update or delete on wa_events
 for each row execute function audit_events_append_only();
alter table wa_connections add unique(id,period_id);
alter table wa_groups add column period_id uuid not null;
alter table wa_groups add foreign key(connection_id,period_id) references wa_connections(id,period_id);
alter table wa_groups add unique(period_id,jid);
