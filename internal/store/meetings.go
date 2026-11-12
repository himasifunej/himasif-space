package store

import (
	"context"
	"encoding/json"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/jackc/pgx/v5"
	"time"
)

const meetingColumns = `id,period_id,space_id,owner_division_id,organizer_id,title,description,location,join_url,starts_at,ends_at,time_zone,note_id,cancelled,reminder_minutes,version,created_at,updated_at`

// meetingRead adds the shared timer (stage T2), which is written only by
// SetMeetingTimer.
const meetingRead = meetingColumns + `,timer,timer_version`

func scanMeeting(row pgx.Row) (api.Meeting, error) {
	var m api.Meeting
	var timer []byte
	err := row.Scan(&m.ID, &m.PeriodID, &m.SpaceID, &m.OwnerDivisionID, &m.OrganizerID, &m.Title, &m.Description, &m.Location, &m.JoinURL, &m.StartsAt, &m.EndsAt, &m.TimeZone, &m.NoteID, &m.Cancelled, &m.ReminderMinutes, &m.Version, &m.CreatedAt, &m.UpdatedAt, &timer, &m.TimerVersion)
	if err == nil && len(timer) > 0 && string(timer) != "null" {
		m.Timer = &api.MeetingTimer{}
		err = json.Unmarshal(timer, m.Timer)
	}
	return m, err
}
func (s *Store) Meeting(ctx context.Context, q Querier, mid id.ID, lock string) (api.Meeting, error) {
	return scanMeeting(q.QueryRow(ctx, `select `+meetingRead+` from meetings where id=$1`+lock, mid))
}

// SetMeetingTimer stores the shared timer (nil: none) as version.
func (s *Store) SetMeetingTimer(ctx context.Context, q Querier, mid id.ID, t *api.MeetingTimer, version int64) error {
	var raw []byte
	if t != nil {
		var err error
		if raw, err = json.Marshal(t); err != nil {
			return err
		}
	}
	_, err := q.Exec(ctx, `update meetings set timer=$2,timer_version=$3 where id=$1`, mid, raw, version)
	return err
}
func (s *Store) Meetings(ctx context.Context, q Querier, sid id.ID) ([]api.Meeting, error) {
	rows, err := q.Query(ctx, `select `+meetingRead+` from meetings where space_id=$1 order by starts_at,id`, sid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []api.Meeting{}
	for rows.Next() {
		m, err := scanMeeting(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MyMeetings are the meetings of period that uid organizes or attends,
// starting from from up to to, not cancelled, soonest first.
func (s *Store) MyMeetings(ctx context.Context, q Querier, pid, uid id.ID, from, to time.Time) ([]api.Meeting, error) {
	rows, err := q.Query(ctx, `select `+meetingRead+` from meetings m where m.period_id=$1 and not m.cancelled and m.starts_at>=$3 and m.starts_at<$4 and (m.organizer_id=$2 or exists(select 1 from meeting_participants mp where mp.meeting_id=m.id and mp.user_id=$2)) order by m.starts_at,m.id`, pid, uid, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []api.Meeting{}
	for rows.Next() {
		m, err := scanMeeting(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (s *Store) MeetingParticipants(ctx context.Context, q Querier, mid id.ID) ([]api.MeetingParticipant, error) {
	rows, err := q.Query(ctx, `select mp.user_id,u.display_name,mp.arrived_at,mp.attendance_note,mp.attendance_status,mp.confirmed_by,mp.rsvp,mp.rsvp_note from meeting_participants mp join users u on u.id=mp.user_id where meeting_id=$1 order by lower(u.display_name),u.id`, mid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []api.MeetingParticipant{}
	for rows.Next() {
		var p api.MeetingParticipant
		if err := rows.Scan(&p.UserID, &p.DisplayName, &p.ArrivedAt, &p.AttendanceNote, &p.AttendanceStatus, &p.ConfirmedBy, &p.RSVP, &p.RSVPNote); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) SaveMeeting(ctx context.Context, q Querier, m api.Meeting, participants []id.ID, create bool) error {
	if create {
		_, err := q.Exec(ctx, `insert into meetings (`+meetingColumns+`) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`, m.ID, m.PeriodID, m.SpaceID, m.OwnerDivisionID, m.OrganizerID, m.Title, m.Description, m.Location, m.JoinURL, m.StartsAt, m.EndsAt, m.TimeZone, m.NoteID, m.Cancelled, m.ReminderMinutes, m.Version, m.CreatedAt, m.UpdatedAt)
		if err != nil {
			return err
		}
	} else {
		_, err := q.Exec(ctx, `update meetings set title=$2,description=$3,location=$4,join_url=$5,starts_at=$6,ends_at=$7,time_zone=$8,note_id=$9,cancelled=$10,reminder_minutes=$11,version=$12,updated_at=$13 where id=$1`, m.ID, m.Title, m.Description, m.Location, m.JoinURL, m.StartsAt, m.EndsAt, m.TimeZone, m.NoteID, m.Cancelled, m.ReminderMinutes, m.Version, m.UpdatedAt)
		if err != nil {
			return err
		}
	}
	if participants == nil {
		participants = []id.ID{}
	}
	if _, err := q.Exec(ctx, `delete from meeting_participants where meeting_id=$1 and not(user_id=any($2::uuid[]))`, m.ID, participants); err != nil {
		return err
	}
	for _, uid := range participants {
		if _, err := q.Exec(ctx, `insert into meeting_participants(meeting_id,user_id) values($1,$2) on conflict do nothing`, m.ID, uid); err != nil {
			return err
		}
	}
	return nil
}

// SetMeetingRSVP keeps a participant's own answer; false when they are not
// a participant.
func (s *Store) SetMeetingRSVP(ctx context.Context, q Querier, mid, uid id.ID, status, note string, at time.Time) (bool, error) {
	tag, err := q.Exec(ctx, `update meeting_participants set rsvp=$3,rsvp_note=$4,rsvp_at=$5 where meeting_id=$1 and user_id=$2`, mid, uid, status, note, at)
	return tag.RowsAffected() > 0, err
}
