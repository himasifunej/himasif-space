-- Berkas sebagai bukti (0.4.5, keputusan owner 11 Nov 2026): a submission
-- carries files uploaded to the task, as well as or instead of a link or a
-- note. Additive: older servers ignore evidence_id and still write a link
-- or a note with every submission.
alter table task_attachments add column evidence_id uuid references task_evidence(id);
create index task_attachments_evidence on task_attachments(evidence_id) where evidence_id is not null;

-- A submission of files alone has neither a link nor a note; the server
-- checks that it has one of the three. The check was unnamed: find it by
-- what it says.
do $$
declare c text;
begin
    for c in select conname from pg_constraint
        where conrelid = 'task_evidence'::regclass and contype = 'c'
          and pg_get_constraintdef(oid) like '%length(url)%length(note)%'
    loop
        execute format('alter table task_evidence drop constraint %I', c);
    end loop;
end $$;
