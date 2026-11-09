-- Riwayat ping of a Space (canvas A, Tahap 5b): the newest pings of one
-- space first. Additive; older servers ignore it.
create index delivery_events_space_ping on delivery_events(space_id, created_at desc, id desc) where source_kind='ping';
