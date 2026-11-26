-- ACC bersama tugas desain Medhum (PRD MED-10–15, register R-07): one ACC
-- from the content's PJ Humas, a Humas manager or a Mediatek manager. The
-- side the reviewer approved from is kept with the review, so the content
-- can show "ACC oleh … · Humas". Additive: older servers never write it and
-- ignore the column; '' means "not a Medhum design review".
alter table task_evidence add column review_side text not null default ''
    check (review_side in ('', 'humas', 'mediatek'));
