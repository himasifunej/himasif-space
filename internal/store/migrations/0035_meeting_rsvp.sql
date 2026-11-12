-- Kehadiranmu (0.4.6-rc.5, canvas 3747dbbac0d0 desain 5): a participant
-- says beforehand whether they come, come late or cannot, with a reason for
-- the organizer. It is their answer, not the attendance the organizer
-- records afterwards. Additive: older servers ignore the columns.
alter table meeting_participants
    add column rsvp text not null default '' check (rsvp in ('', 'yes', 'late', 'no')),
    add column rsvp_note text not null default '' check (length(rsvp_note) <= 300),
    add column rsvp_at timestamptz;
