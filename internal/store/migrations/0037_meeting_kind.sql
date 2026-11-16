-- Jenis rapat (PRD MTG-06; pemantauan BPH, 16 Nov 2026): rapat divisi,
-- Medhum, BPH atau pengurus. Additive: older servers ignore the column and
-- their inserts take the default. Meetings already in a shared space are
-- Medhum's.
alter table meetings add column kind text not null default 'division'
    check (kind in ('division', 'medhum', 'bph', 'pengurus'));
update meetings m set kind = 'medhum' from spaces s where s.id = m.space_id and s.kind = 'shared';
create index meetings_period_schedule on meetings (period_id, starts_at, id);
