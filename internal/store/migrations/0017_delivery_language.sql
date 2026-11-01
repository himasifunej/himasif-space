-- Generated delivery text has an explicit message key and literal arguments.
-- User-authored ping text never enters a translation-by-string-replacement path.
alter table delivery_events add column message_key text not null default '';
alter table delivery_events add column message_args jsonb not null default '[]'::jsonb;
alter table delivery_events add check(jsonb_typeof(message_args)='array');
