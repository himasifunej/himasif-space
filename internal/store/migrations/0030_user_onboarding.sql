-- Onboarding progress of an account (canvas a036cb358772, option A): the
-- steps recorded once, as {"<key>": "<time>"} (welcome, profile, tasks,
-- tour, checklistHidden, handover). Whether WhatsApp is linked is derived
-- from wa_identities. Additive; older servers ignore it.
create table user_onboarding (
    user_id    uuid primary key references users (id),
    marks      jsonb not null default '{}'::jsonb,
    updated_at timestamptz not null
);

-- Accounts that already work in Space skip the welcome, the tour and the
-- checklist; they can still replay the tour and read the guide.
insert into user_onboarding (user_id, marks, updated_at)
select id, jsonb_build_object('welcome', to_jsonb(now()), 'tour', to_jsonb(now()), 'checklistHidden', to_jsonb(now()), 'handover', to_jsonb(now())), now()
from users where kind = 'person'
on conflict (user_id) do nothing;
