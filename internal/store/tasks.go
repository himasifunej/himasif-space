package store

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"github.com/jackc/pgx/v5"
)

// TaskRow keeps ordering and pre-submission progress private to the server.
type TaskRow struct {
	api.Task
	Rank           int64
	ResumeProgress string
}

// TaskPeriod is a nonlocking peek. Callers must lock the period before Task.
func (s *Store) TaskPeriod(ctx context.Context, q Querier, tid id.ID) (id.ID, error) {
	var pid id.ID
	err := q.QueryRow(ctx, `select period_id from tasks where id=$1`, tid).Scan(&pid)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return pid, err
}

// Task loads the canonical record and its children inside the caller's snapshot.
func (s *Store) Task(ctx context.Context, q Querier, tid id.ID, lock string) (TaskRow, error) {
	var t TaskRow
	var deadline, posting, planned *string
	err := q.QueryRow(ctx, `select t.id,t.period_id,t.owner_division_id,t.title,t.description,t.expected_output,t.category,
 t.assignee_id,a.display_name,t.deadline_date::text,t.deadline_at,t.posting_date::text,t.posting_at,p.time_zone,
 t.progress,t.match_status,t.version,t.created_by,c.display_name,t.created_at,t.updated_at,t.rank,t.resume_progress,t.planned_start_date::text,t.planned_start_at
 from tasks t join users a on a.id=t.assignee_id join users c on c.id=t.created_by join periods p on p.id=t.period_id
 where t.id=$1`+taskLock(lock), tid).Scan(&t.ID, &t.PeriodID, &t.OwnerDivisionID, &t.Title, &t.Description, &t.ExpectedOutput, &t.Category,
		&t.Assignee.ID, &t.Assignee.DisplayName, &deadline, &t.DeadlineAt, &posting, &t.PostingAt, &t.TimeZone,
		&t.Progress, &t.MatchStatus, &t.Version, &t.CreatedBy.ID, &t.CreatedBy.DisplayName, &t.CreatedAt, &t.UpdatedAt, &t.Rank, &t.ResumeProgress, &planned, &t.PlannedStartAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	if err != nil {
		return t, err
	}
	if planned != nil {
		d, e := timeutil.ParseDate(*planned)
		if e != nil {
			return t, e
		}
		t.PlannedStartDate = &d
	}
	if deadline != nil {
		d, e := timeutil.ParseDate(*deadline)
		if e != nil {
			return t, e
		}
		t.DeadlineDate = &d
	}
	if posting != nil {
		d, e := timeutil.ParseDate(*posting)
		if e != nil {
			return t, e
		}
		t.PostingDate = &d
	}
	if t.SpaceIDs, t.Numbers, err = taskSpaceNumbers(ctx, q, tid, ""); err != nil {
		return t, err
	}
	rows, err := q.Query(ctx, `select u.id,u.display_name from task_participants x join users u on u.id=x.user_id where x.task_id=$1 order by u.id`, tid)
	if err != nil {
		return t, err
	}
	t.Participants, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (api.PersonRef, error) {
		var p api.PersonRef
		e := r.Scan(&p.ID, &p.DisplayName)
		return p, e
	})
	if err != nil {
		return t, err
	}
	rows, err = q.Query(ctx, `select c.id,u.id,u.display_name,c.body,c.created_at from task_comments c join users u on u.id=c.author_id where c.task_id=$1 order by c.created_at,c.id`, tid)
	if err != nil {
		return t, err
	}
	t.Comments, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (api.TaskComment, error) {
		var c api.TaskComment
		e := r.Scan(&c.ID, &c.Author.ID, &c.Author.DisplayName, &c.Body, &c.CreatedAt)
		return c, e
	})
	if err != nil {
		return t, err
	}
	rows, err = q.Query(ctx, `select c.id,c.body,c.done,c.done_at,c.created_at,u.id,u.display_name from task_checklist_items c left join users u on u.id=c.done_by where c.task_id=$1 and c.deleted_at is null order by c.position,c.id`, tid)
	if err != nil {
		return t, err
	}
	t.Checklist, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (api.TaskChecklistItem, error) {
		var c api.TaskChecklistItem
		var who *id.ID
		var name *string
		e := r.Scan(&c.ID, &c.Body, &c.Done, &c.DoneAt, &c.CreatedAt, &who, &name)
		if who != nil && name != nil {
			c.DoneBy = &api.PersonRef{ID: *who, DisplayName: *name}
		}
		return c, e
	})
	if err != nil {
		return t, err
	}
	if t.Checklist == nil {
		t.Checklist = []api.TaskChecklistItem{}
	}
	rows, err = q.Query(ctx, `select h.id,u.id,u.display_name,h.message,h.status,h.created_at from task_help_requests h join users u on u.id=h.user_id where h.task_id=$1 order by h.created_at,h.id`, tid)
	if err != nil {
		return t, err
	}
	t.HelpRequests, err = pgx.CollectRows(rows, scanTaskHelp)
	if err != nil {
		return t, err
	}
	rows, err = q.Query(ctx, `select e.id,u.id,u.display_name,e.url,e.note,e.match_status,e.submitted_at,e.recorded_at,e.reviewed_at,r.id,r.display_name,e.review_reason
 from task_evidence e join users u on u.id=e.submitted_by left join users r on r.id=e.reviewed_by where e.task_id=$1 order by e.recorded_at,e.id`, tid)
	if err != nil {
		return t, err
	}
	t.Evidence, err = pgx.CollectRows(rows, scanTaskEvidence)
	return t, err
}
func taskLock(lock string) string {
	if lock == LockUpdate {
		return " for update of t"
	}
	if lock == LockShare {
		return " for share of t"
	}
	return ""
}
func scanTaskHelp(r pgx.CollectableRow) (api.TaskHelpRequest, error) {
	var h api.TaskHelpRequest
	e := r.Scan(&h.ID, &h.User.ID, &h.User.DisplayName, &h.Message, &h.Status, &h.CreatedAt)
	return h, e
}
func scanTaskEvidence(r pgx.CollectableRow) (api.TaskEvidence, error) {
	var e api.TaskEvidence
	var uid *id.ID
	var name *string
	err := r.Scan(&e.ID, &e.SubmittedBy.ID, &e.SubmittedBy.DisplayName, &e.URL, &e.Note, &e.MatchStatus, &e.SubmittedAt, &e.RecordedAt, &e.ReviewedAt, &uid, &name, &e.ReviewReason)
	if uid != nil {
		e.ReviewedBy = &api.PersonRef{ID: *uid, DisplayName: *name}
	}
	return e, err
}

// TaskIDs follows persisted lane order, with UUID as a deterministic tie break.
func (s *Store) TaskIDs(ctx context.Context, q Querier, pid id.ID) ([]id.ID, error) {
	rows, e := q.Query(ctx, `select id from tasks where period_id=$1 order by progress,rank,id`, pid)
	if e != nil {
		return nil, e
	}
	return pgx.CollectRows(rows, pgx.RowTo[id.ID])
}

func (s *Store) InsertTask(ctx context.Context, q Querier, t *TaskRow) error {
	_, err := q.Exec(ctx, `insert into tasks(id,period_id,owner_division_id,title,description,expected_output,category,assignee_id,deadline_date,deadline_at,posting_date,posting_at,progress,match_status,rank,created_by,created_at,updated_at,planned_start_date,planned_start_at)
 values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`, t.ID, t.PeriodID, t.OwnerDivisionID, t.Title, t.Description, t.ExpectedOutput, t.Category, t.Assignee.ID, t.DeadlineDate, t.DeadlineAt, t.PostingDate, t.PostingAt, t.Progress, t.MatchStatus, t.Rank, t.CreatedBy.ID, t.CreatedAt, t.UpdatedAt, t.PlannedStartDate, t.PlannedStartAt)
	if err != nil {
		return err
	}
	return s.TaskSpaces(ctx, q, t)
}

// TaskSpaces makes t's Space links t.SpaceIDs by difference: a kept link
// keeps its number, a removed one is deleted (its number is not reused), a
// new one takes its Space's next number. It fills t.Numbers.
func (s *Store) TaskSpaces(ctx context.Context, q Querier, t *TaskRow) error {
	have, numbers, err := taskSpaceNumbers(ctx, q, t.ID, " for update")
	if err != nil {
		return err
	}
	want := slices.Clone(t.SpaceIDs)
	// Counters are locked in Space order, so two tasks gaining the same
	// Spaces cannot deadlock.
	slices.SortFunc(want, func(a, b id.ID) int { return strings.Compare(a.String(), b.String()) })
	for _, sid := range have {
		if !slices.Contains(want, sid) {
			if _, e := q.Exec(ctx, `delete from task_spaces where task_id=$1 and space_id=$2`, t.ID, sid); e != nil {
				return e
			}
			delete(numbers, sid.String())
		}
	}
	for _, sid := range want {
		if slices.Contains(have, sid) {
			continue
		}
		var n int64
		if e := q.QueryRow(ctx, `insert into space_task_counters(space_id,last_number) values($1,1)
			on conflict(space_id) do update set last_number=space_task_counters.last_number+1 returning last_number`, sid).Scan(&n); e != nil {
			return e
		}
		if _, e := q.Exec(ctx, `insert into task_spaces(task_id,space_id,period_id,number) values($1,$2,$3,$4)`, t.ID, sid, t.PeriodID, n); e != nil {
			return e
		}
		numbers[sid.String()] = n
	}
	t.Numbers = numbers
	return nil
}

// taskSpaceNumbers returns a task's Spaces and its number in each.
func taskSpaceNumbers(ctx context.Context, q Querier, tid id.ID, lock string) ([]id.ID, map[string]int64, error) {
	rows, err := q.Query(ctx, `select space_id,number from task_spaces where task_id=$1 order by space_id`+lock, tid)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	ids := []id.ID{}
	numbers := map[string]int64{}
	for rows.Next() {
		var sid id.ID
		var n int64
		if err := rows.Scan(&sid, &n); err != nil {
			return nil, nil, err
		}
		ids = append(ids, sid)
		numbers[sid.String()] = n
	}
	return ids, numbers, rows.Err()
}

// SaveTask requires a period/task write lock held by the caller. Optimistic
// concurrency is checked before any child mutation; only this task is bumped.
func (s *Store) SaveTask(ctx context.Context, q Querier, t TaskRow) error {
	_, e := q.Exec(ctx, `update tasks set title=$2,description=$3,expected_output=$4,category=$5,assignee_id=$6,deadline_date=$7,deadline_at=$8,
 posting_date=$9,posting_at=$10,progress=$11,match_status=$12,resume_progress=$13,rank=$14,version=$15,updated_at=$16,planned_start_date=$17,planned_start_at=$18 where id=$1`,
		t.ID, t.Title, t.Description, t.ExpectedOutput, t.Category, t.Assignee.ID, t.DeadlineDate, t.DeadlineAt, t.PostingDate, t.PostingAt, t.Progress, t.MatchStatus, t.ResumeProgress, t.Rank, t.Version, t.UpdatedAt, t.PlannedStartDate, t.PlannedStartAt)
	return e
}

// TaskRank inserts before a task, or at the end. Rebalancing changes only ranks
// and is performed under the exclusive period lock, never task versions.
func (s *Store) TaskRank(ctx context.Context, q Querier, pid, tid, before id.ID, lane string) (int64, error) {
	rows, e := q.Query(ctx, `select id from tasks where period_id=$1 and progress=$2 and id<>$3 order by rank,id`, pid, lane, tid)
	if e != nil {
		return 0, e
	}
	ids, e := pgx.CollectRows(rows, pgx.RowTo[id.ID])
	if e != nil {
		return 0, e
	}
	pos := len(ids)
	if before != id.Nil {
		pos = -1
		for i, x := range ids {
			if x == before {
				pos = i
				break
			}
		}
		if pos < 0 {
			return 0, ErrNotFound
		}
	}
	// Compact integer ordering avoids precision drift after repeated drags.
	for i, x := range ids {
		rank := int64(i+1) * 1024
		if i >= pos {
			rank += 1024
		}
		if _, e = q.Exec(ctx, `update tasks set rank=$2 where id=$1`, x, rank); e != nil {
			return 0, e
		}
	}
	return int64(pos+1) * 1024, nil
}

func (s *Store) AddTaskChecklistItem(ctx context.Context, q Querier, tid, author id.ID, c api.TaskChecklistItem) error {
	_, e := q.Exec(ctx, `insert into task_checklist_items(id,task_id,body,position,created_by,created_at,updated_at)
 values($1,$2,$3,(select coalesce(max(position),0)+1 from task_checklist_items where task_id=$2),$4,$5,$5)`, c.ID, tid, c.Body, author, c.CreatedAt)
	return e
}

// UpdateTaskChecklistItem changes the body or done state of a live item of
// the task. It reports ErrNotFound for another task's item or a deleted one.
func (s *Store) UpdateTaskChecklistItem(ctx context.Context, q Querier, tid, item, actor id.ID, body *string, done *bool, at time.Time) error {
	tag, e := q.Exec(ctx, `update task_checklist_items set body=coalesce($4,body),
 done=coalesce($5,done),
 done_by=case when $5::boolean is null then done_by when $5 then $3 else null end,
 done_at=case when $5::boolean is null then done_at when $5 then $6 else null end,
 updated_at=$6 where id=$2 and task_id=$1 and deleted_at is null`, tid, item, actor, body, done, at)
	if e != nil {
		return e
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteTaskChecklistItem(ctx context.Context, q Querier, tid, item id.ID, at time.Time) error {
	tag, e := q.Exec(ctx, `update task_checklist_items set deleted_at=$3,updated_at=$3 where id=$2 and task_id=$1 and deleted_at is null`, tid, item, at)
	if e != nil {
		return e
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) AddTaskComment(ctx context.Context, q Querier, tid id.ID, c api.TaskComment) error {
	_, e := q.Exec(ctx, `insert into task_comments(id,task_id,author_id,body,created_at) values($1,$2,$3,$4,$5)`, c.ID, tid, c.Author.ID, c.Body, c.CreatedAt)
	return e
}
func (s *Store) AddTaskHelp(ctx context.Context, q Querier, tid id.ID, h api.TaskHelpRequest) error {
	_, e := q.Exec(ctx, `insert into task_help_requests(id,task_id,user_id,message,status,created_at) values($1,$2,$3,$4,$5,$6)`, h.ID, tid, h.User.ID, h.Message, h.Status, h.CreatedAt)
	if isUniqueViolation(e) {
		return ErrConflict
	}
	return e
}
func (s *Store) DecideTaskHelp(ctx context.Context, q Querier, tid id.ID, h api.TaskHelpRequest, actor id.ID, now time.Time) error {
	_, e := q.Exec(ctx, `update task_help_requests set status=$3,decided_by=$4,decided_at=$5 where id=$1 and task_id=$2`, h.ID, tid, h.Status, actor, now)
	if e != nil {
		return e
	}
	if h.Status == "approved" {
		_, e = q.Exec(ctx, `insert into task_participants(task_id,user_id,help_id) values($1,$2,$3) on conflict(task_id,user_id) do nothing`, tid, h.User.ID, h.ID)
	}
	return e
}
func (s *Store) AddTaskEvidence(ctx context.Context, q Querier, tid id.ID, e api.TaskEvidence) error {
	if e.RecordedAt.IsZero() {
		e.RecordedAt = e.SubmittedAt
	}
	_, err := q.Exec(ctx, `insert into task_evidence(id,task_id,submitted_by,url,note,match_status,submitted_at,recorded_at) values($1,$2,$3,$4,$5,$6,$7,$8)`, e.ID, tid, e.SubmittedBy.ID, e.URL, e.Note, e.MatchStatus, e.SubmittedAt, e.RecordedAt)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}
func (s *Store) ReviewTaskEvidence(ctx context.Context, q Querier, tid id.ID, e api.TaskEvidence) error {
	_, err := q.Exec(ctx, `update task_evidence set match_status=$3,reviewed_at=$4,reviewed_by=$5,review_reason=$6 where id=$1 and task_id=$2`, e.ID, tid, e.MatchStatus, e.ReviewedAt, e.ReviewedBy.ID, e.ReviewReason)
	return err
}

func (s *Store) AddTaskEvent(ctx context.Context, q Querier, tid id.ID, e api.TaskEvent) error {
	before, err := jsonOrNil(e.Before)
	if err != nil {
		return err
	}
	after, err := jsonOrNil(e.After)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `insert into task_events(id,task_id,actor_id,action,reason,created_at,before,after) values($1,$2,$3,$4,$5,$6,$7,$8)`, e.ID, tid, e.Actor.ID, e.Action, e.Reason, e.CreatedAt, before, after)
	return err
}
func (s *Store) TaskHistory(ctx context.Context, q Querier, tid id.ID) ([]api.TaskEvent, error) {
	rows, e := q.Query(ctx, `select e.id,u.id,u.display_name,e.action,e.reason,e.created_at,e.before,e.after from task_events e join users u on u.id=e.actor_id where e.task_id=$1 order by e.created_at,e.id`, tid)
	if e != nil {
		return nil, e
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (api.TaskEvent, error) {
		var e api.TaskEvent
		var before, after []byte
		err := r.Scan(&e.ID, &e.Actor.ID, &e.Actor.DisplayName, &e.Action, &e.Reason, &e.CreatedAt, &before, &after)
		if err == nil && before != nil {
			err = json.Unmarshal(before, &e.Before)
		}
		if err == nil && after != nil {
			err = json.Unmarshal(after, &e.After)
		}
		return e, err
	})
}

func (s *Store) AddTaskNotification(ctx context.Context, q Querier, uid id.ID, n api.Notification) error {
	_, e := q.Exec(ctx, `insert into task_notifications(id,task_id,recipient_id,title,body,created_at) values($1,$2,$3,$4,$5,$6)`, n.ID, n.TaskID, uid, n.Title, n.Body, n.CreatedAt)
	return e
}
func (s *Store) TaskNotifications(ctx context.Context, q Querier, uid id.ID) ([]api.Notification, error) {
	rows, e := q.Query(ctx, `select id,task_id,title,body,created_at,read_at from task_notifications where recipient_id=$1 order by created_at desc,id desc`, uid)
	if e != nil {
		return nil, e
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (api.Notification, error) {
		var n api.Notification
		e := r.Scan(&n.ID, &n.TaskID, &n.Title, &n.Body, &n.CreatedAt, &n.ReadAt)
		return n, e
	})
}
func (s *Store) ReadTaskNotification(ctx context.Context, q Querier, nid, uid id.ID, now time.Time) error {
	tag, e := q.Exec(ctx, `update task_notifications set read_at=coalesce(read_at,$3) where id=$1 and recipient_id=$2`, nid, uid, now)
	if e == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return e
}
