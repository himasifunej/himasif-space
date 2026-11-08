-- Interface preferences that follow an account across devices (canvas A,
-- Tahap 5c): for now the Space tab bars, {"<spaceId>": {"order": [...],
-- "hidden": [...]}}. Additive; older servers ignore it.
create table user_ui_preferences (
    user_id    uuid primary key references users (id),
    space_tabs jsonb not null default '{}'::jsonb,
    updated_at timestamptz not null
);
