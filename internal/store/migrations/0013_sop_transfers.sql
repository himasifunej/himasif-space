-- A transfer withdraws the source attribution and opens target review. Neither
-- the original finding nor either division's immutable evidence is moved.
create table sop_transfers (
 id uuid primary key,
 source_finding_id uuid not null references sop_findings(id),
 target_finding_id uuid not null references sop_findings(id),
 source_observation_id uuid not null,
 target_observation_id uuid not null,
 actor_id uuid not null references users(id),
 reason text not null,
 created_at timestamptz not null,
 check(source_finding_id<>target_finding_id),
 foreign key(source_observation_id,source_finding_id) references sop_observations(id,finding_id),
 foreign key(target_observation_id,target_finding_id) references sop_observations(id,finding_id)
);
create index sop_transfers_source on sop_transfers(source_finding_id,created_at,id);
create index sop_transfers_target on sop_transfers(target_finding_id,created_at,id);
create trigger sop_transfers_append_only before update or delete on sop_transfers
 for each row execute function audit_events_append_only();
