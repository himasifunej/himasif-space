-- Source event time and the time Space recorded/reviewed it are distinct.
alter table task_evidence add column recorded_at timestamptz;
update task_evidence set recorded_at=submitted_at;
alter table task_evidence alter column recorded_at set default now();
alter table task_evidence alter column recorded_at set not null;
create table wa_parser_policies (
 space_id uuid primary key references spaces(id),
 broadcast_enabled boolean not null default false,
 drive_enabled boolean not null default false,
 template_confirmed boolean not null default false,
 version bigint not null default 1,
 updated_by uuid not null references users(id),
 updated_at timestamptz not null
);
create table wa_content_items (
 id uuid primary key,
 inbound_id uuid not null references wa_inbound(id),
 space_id uuid not null references spaces(id),
 kind text not null check(kind in ('concept','drive','correction')),
 position integer not null,
 parser_version text not null,
 parsed jsonb not null,
 candidates uuid[] not null default '{}',
 content_id uuid references medhum_contents(id),
 evidence_id uuid references task_evidence(id),
 sender_user_id uuid references users(id),
 state text not null default 'pending' check(state in ('pending','applied','ignored','corrected','retracted')),
 issue text not null default '',
 source_at timestamptz not null,
 matched_at timestamptz,
 reviewed_at timestamptz,
 reviewed_by uuid references users(id),
 version bigint not null default 1,
 unique(inbound_id,kind,position)
);
create index wa_content_review_queue on wa_content_items(space_id,state,source_at,id);
create table wa_concept_receipts (
 id uuid primary key,
 content_id uuid not null references medhum_contents(id),
 humas_pj_id uuid not null references users(id),
 fact_key text not null,
 first_item_id uuid not null references wa_content_items(id),
 first_at timestamptz not null,
 active boolean not null default true,
 invalidated_by uuid references users(id),
 invalidated_at timestamptz,
 invalidation_reason text not null default '',
 unique(content_id,fact_key)
);
create table wa_content_reviews (
 id uuid primary key,
 item_id uuid not null references wa_content_items(id),
 actor_id uuid references users(id),
 action text not null,
 reason text not null default '',
 before_data jsonb,
 after_data jsonb,
 occurred_at timestamptz not null
);
create table wa_concept_item_receipts (
 item_id uuid primary key references wa_content_items(id),
 receipt_id uuid not null references wa_concept_receipts(id)
);
create trigger wa_content_reviews_append_only before update or delete on wa_content_reviews
 for each row execute function audit_events_append_only();
-- Reposted Drive evidence for the same task is the same source fact. Keep the
-- first source timestamp and receipt even after a later manual correction.
create table wa_drive_receipts (
 content_id uuid not null references medhum_contents(id),
 resource_id text not null,
 item_id uuid not null references wa_content_items(id),
 evidence_id uuid not null references task_evidence(id),
 first_at timestamptz not null,
 primary key(content_id,resource_id)
);
