alter table wa_outbox add column accepted_at timestamptz;
alter table wa_outbox add column target_name text not null default '';
create function wa_outbox_target_snapshot() returns trigger language plpgsql as $$
begin
 if new.target_name='' then
  select name into new.target_name from wa_groups where id=new.group_id;
 end if;
 return new;
end;
$$;
create trigger wa_outbox_target_snapshot before insert on wa_outbox
 for each row execute function wa_outbox_target_snapshot();
-- SOP review is separate from matching task results and never writes legacy
-- violations. Grants below open evaluation data only, not another Space/Note.
create table sop_policies (
 id uuid primary key,
 period_id uuid not null,
 division_id uuid not null,
 space_id uuid not null,
 document_version text not null,
 enabled boolean not null default false,
 settings jsonb not null,
 version bigint not null default 1,
 updated_by uuid not null references users(id),
 updated_at timestamptz not null,
 unique(period_id,division_id),
 foreign key(period_id,division_id) references period_divisions(period_id,division_id),
 foreign key(space_id,period_id) references spaces(id,period_id)
);
create table sop_evaluator_grants (
 id uuid primary key,
 period_id uuid not null,
 division_id uuid not null,
 user_id uuid not null references users(id),
 can_review boolean not null default false,
 granted_by uuid not null references users(id),
 granted_at timestamptz not null,
 revoked_at timestamptz,
 foreign key(period_id,division_id) references period_divisions(period_id,division_id)
);
create unique index sop_evaluator_active on sop_evaluator_grants(period_id,division_id,user_id) where revoked_at is null;
create table sop_contexts (
 content_id uuid not null references medhum_contents(id),
 division_id uuid not null references divisions(id),
 data jsonb not null,
 version bigint not null default 1,
 updated_by uuid not null references users(id),
 updated_at timestamptz not null,
 primary key(content_id,division_id)
);
create table sop_bot_agreements (
 id uuid primary key,
 policy_id uuid not null references sop_policies(id),
 policy_version bigint not null,
 evaluator_id uuid not null references users(id),
 approved boolean not null,
 reason text not null,
 created_at timestamptz not null
);
create trigger sop_bot_agreements_append_only before update or delete on sop_bot_agreements
 for each row execute function audit_events_append_only();
create table sop_findings (
 id uuid primary key,
 period_id uuid not null,
 division_id uuid not null,
 content_id uuid not null references medhum_contents(id),
 rule text not null,
 event_on date not null,
 policy_id uuid not null references sop_policies(id),
 person_id uuid references users(id),
 state text not null default 'needs_review' check(state in ('needs_review','confirmed','dismissed','corrected')),
 latest_observation_id uuid,
 decision_observation_id uuid,
 needs_recheck boolean not null default false,
 version bigint not null default 1,
 created_at timestamptz not null,
 updated_at timestamptz not null,
 unique(content_id,division_id,rule),
 foreign key(period_id,division_id) references period_divisions(period_id,division_id)
);
create table sop_observations (
 id uuid primary key,
 finding_id uuid not null references sop_findings(id),
 policy_version bigint not null,
 document_version text not null,
 fingerprint text not null,
 measurement text not null check(measurement in ('late','insufficient','on_time','exempt','within_limit','exceeds_limit','not_applicable','manual')),
 data jsonb not null,
 occurred_at timestamptz not null,
 unique(id,finding_id)
);
create index sop_observations_source on sop_observations(finding_id,fingerprint);
alter table sop_findings add foreign key(latest_observation_id,id) references sop_observations(id,finding_id);
alter table sop_findings add foreign key(decision_observation_id,id) references sop_observations(id,finding_id);
create trigger sop_observations_append_only before update or delete on sop_observations
 for each row execute function audit_events_append_only();
create table sop_decisions (
 id uuid primary key,
 finding_id uuid not null references sop_findings(id),
 observation_id uuid not null,
 actor_id uuid not null references users(id),
 action text not null check(action in ('confirm','dismiss','reopen','correct')),
 reason text not null,
 before_data jsonb not null,
 after_data jsonb not null,
 created_at timestamptz not null,
 foreign key(observation_id,finding_id) references sop_observations(id,finding_id)
);
create trigger sop_decisions_append_only before update or delete on sop_decisions
 for each row execute function audit_events_append_only();
create table sop_followups (
 id uuid primary key,
 content_id uuid not null references medhum_contents(id),
 policy_id uuid not null references sop_policies(id),
 policy_version bigint not null,
 calendar_version bigint not null,
 calendar_key text not null,
 scheduled_at timestamptz not null,
 outbox_id uuid unique references wa_outbox(id),
 snapshot jsonb not null,
 created_at timestamptz not null,
 unique(content_id,scheduled_at,policy_version,calendar_key)
);
create table sop_report_runs (
 id uuid primary key,
 period_id uuid not null references periods(id),
 division_id uuid not null references divisions(id),
 policy_id uuid not null references sop_policies(id),
 policy_version bigint not null,
 scheduled_at timestamptz not null,
 event_key text not null unique,
 range_start date not null,
 range_end date not null,
 snapshot jsonb not null,
 outbox_id uuid unique references wa_outbox(id),
 created_at timestamptz not null,
 check(range_end>=range_start)
);
create index sop_decisions_latest on sop_decisions(finding_id,created_at desc,id desc);

create table sop_source_changes (
 sequence bigint generated always as identity primary key,
 finding_id uuid not null references sop_findings(id),
 source_type text not null,
 source_id uuid not null,
 occurred_at timestamptz not null
);
create index sop_source_changes_finding on sop_source_changes(finding_id,sequence);
create trigger sop_source_changes_append_only before update or delete on sop_source_changes
 for each row execute function audit_events_append_only();

-- Mark stale decisions in the same transaction as source changes. The
-- background evaluator later records a new observation; it never withdraws
-- or confirms the human decision automatically.
create function sop_mark_content_changed(cid uuid,kind text,source uuid) returns void language sql as $$
 insert into sop_source_changes(finding_id,source_type,source_id,occurred_at)
 select id,kind,source,now() from sop_findings where content_id=cid and decision_observation_id is not null;
 update sop_findings set needs_recheck=true,version=version+1,updated_at=now()
 where content_id=cid and decision_observation_id is not null and not needs_recheck;
$$;
create function sop_content_source_changed() returns trigger language plpgsql as $$
begin
 if row(new.title,new.posting_date,new.posting_at,new.design_date,new.design_at,new.humas_pj_id,new.design_pj_id,new.concept_url,new.concept_submitted_at,new.published_at)
 is distinct from row(old.title,old.posting_date,old.posting_at,old.design_date,old.design_at,old.humas_pj_id,old.design_pj_id,old.concept_url,old.concept_submitted_at,old.published_at) then
  perform sop_mark_content_changed(new.id,'calendar',new.id);
 end if;
 return new;
end;
$$;
create trigger sop_content_changed after update on medhum_contents
 for each row execute function sop_content_source_changed();
create function sop_task_source_changed() returns trigger language plpgsql as $$
declare cid uuid;
begin
 if row(new.assignee_id,new.deadline_date,new.deadline_at,new.match_status,new.progress='cancelled')
 is distinct from row(old.assignee_id,old.deadline_date,old.deadline_at,old.match_status,old.progress='cancelled') then
  for cid in select id from medhum_contents where design_task_id=new.id or humas_task_id=new.id loop
   perform sop_mark_content_changed(cid,'task',new.id);
  end loop;
 end if;
 return new;
end;
$$;
create trigger sop_task_changed after update on tasks
 for each row execute function sop_task_source_changed();
create function sop_evidence_source_changed() returns trigger language plpgsql as $$
declare cid uuid;
begin
 if tg_op='UPDATE' and row(new.submitted_at,new.url,new.note,new.match_status,new.submitted_by,new.reviewed_at,new.reviewed_by,new.review_reason)
 is not distinct from row(old.submitted_at,old.url,old.note,old.match_status,old.submitted_by,old.reviewed_at,old.reviewed_by,old.review_reason) then
  return new;
 end if;
 for cid in select id from medhum_contents where design_task_id=new.task_id loop
  perform sop_mark_content_changed(cid,'evidence',new.id);
 end loop;
 return new;
end;
$$;
create trigger sop_evidence_changed after insert or update on task_evidence
 for each row execute function sop_evidence_source_changed();
create function sop_context_source_changed() returns trigger language plpgsql as $$
begin
 perform sop_mark_content_changed(new.content_id,'context',new.content_id);
 return new;
end;
$$;
create trigger sop_context_changed after insert or update on sop_contexts
 for each row execute function sop_context_source_changed();
create function sop_message_source_changed() returns trigger language plpgsql as $$
declare cid uuid;
begin
 if tg_op='UPDATE' and row(new.state,new.candidates,new.content_id) is not distinct from row(old.state,old.candidates,old.content_id) then
  return new;
 end if;
 if new.kind='correction' then
  foreach cid in array new.candidates loop
   perform sop_mark_content_changed(cid,'message',new.id);
  end loop;
 end if;
 return new;
end;
$$;
create trigger sop_message_changed after insert or update on wa_content_items
 for each row execute function sop_message_source_changed();
