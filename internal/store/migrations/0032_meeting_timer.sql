-- Stage T2: the shared meeting timer, on the meeting (not in the note's
-- content). timer is api.MeetingTimer as JSON, null while none runs;
-- timer_version counts its changes, apart from the meeting's version so the
-- timer never conflicts with editing the meeting. The meetings realtime
-- trigger already tells clients of a change.
alter table meetings add column timer jsonb;
alter table meetings add column timer_version bigint not null default 0;
