-- Linking WhatsApp when the number already belongs to another account: the
-- code is kept open with the conflict, and the sender of that number may
-- answer PINDAH to move the link (whatsapp_link.go). Space shows the state.
-- Only adds nullable/defaulted columns and a trigger: safe for existing rows.
alter table wa_link_challenges add column attempted_at timestamptz;
alter table wa_link_challenges add column conflict_user_id uuid references users(id);
alter table wa_link_challenges add column conflict_jids text[] not null default '{}';
alter table wa_link_challenges add column conflict_until timestamptz;
create trigger wa_link_challenges_realtime after insert or update or delete on wa_link_challenges for each row execute function space_period_changed();
