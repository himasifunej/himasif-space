-- Child writes do not always bump their parent version (comments, history,
-- attachments, notification reads). Invalidate the same canonical snapshots
-- on COMMIT without changing versions or exposing any child content.
create function space_task_child_changed() returns trigger language plpgsql as $$
declare target uuid; pid uuid;
begin
 if TG_OP='DELETE' then target := old.task_id; else target := new.task_id; end if;
 select period_id into pid from tasks where id=target;
 if pid is not null then perform pg_notify('space_changes',pid::text); end if;
 return null;
end;
$$;
create trigger task_comments_realtime after insert or update or delete on task_comments for each row execute function space_task_child_changed();
create trigger task_help_realtime after insert or update or delete on task_help_requests for each row execute function space_task_child_changed();
create trigger task_participants_realtime after insert or update or delete on task_participants for each row execute function space_task_child_changed();
create trigger task_evidence_realtime after insert or update or delete on task_evidence for each row execute function space_task_child_changed();
create trigger task_events_realtime after insert on task_events for each row execute function space_task_child_changed();
create trigger task_notifications_realtime after insert or update or delete on task_notifications for each row execute function space_task_child_changed();

create function space_note_child_changed() returns trigger language plpgsql as $$
declare target uuid; pid uuid;
begin
 if TG_OP='DELETE' then target := old.note_id; else target := new.note_id; end if;
 select period_id into pid from notes where id=target;
 if pid is not null then perform pg_notify('space_changes',pid::text); end if;
 return null;
end;
$$;
create trigger note_comments_realtime after insert or update or delete on note_comments for each row execute function space_note_child_changed();
create trigger note_attachments_realtime after insert or update or delete on note_attachments for each row execute function space_note_child_changed();
create trigger note_links_realtime after insert or update or delete on note_links for each row execute function space_note_child_changed();
create trigger note_events_realtime after insert on note_events for each row execute function space_note_child_changed();

create function space_meeting_child_changed() returns trigger language plpgsql as $$
declare target uuid; pid uuid;
begin
 if TG_OP='DELETE' then target := old.meeting_id; else target := new.meeting_id; end if;
 select period_id into pid from meetings where id=target;
 if pid is not null then perform pg_notify('space_changes',pid::text); end if;
 return null;
end;
$$;
create trigger meeting_participants_realtime after insert or update or delete on meeting_participants for each row execute function space_meeting_child_changed();

-- Account/session changes must wake streams even if no workspace was edited.
-- The zero UUID is a broadcast invalidation, containing no account identifier.
create function space_account_changed() returns trigger language plpgsql as $$
begin
 perform pg_notify('space_changes','00000000-0000-0000-0000-000000000000');
 return null;
end;
$$;
create trigger users_realtime after update or delete on users for each row execute function space_account_changed();
create trigger sessions_realtime after delete on sessions for each row execute function space_account_changed();
create trigger sessions_revoked_realtime after update of revoked_at,expires_at on sessions for each row
 when (old.revoked_at is distinct from new.revoked_at or old.expires_at is distinct from new.expires_at)
 execute function space_account_changed();
