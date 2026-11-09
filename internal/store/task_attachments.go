package store

import (
	"context"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/jackc/pgx/v5"
)

// TaskAttachmentPath is where a task attachment is downloaded.
func TaskAttachmentPath(tid, aid id.ID) string {
	return "/api/tasks/" + tid.String() + "/attachments/" + aid.String() + "/file"
}

// TaskAttachments returns a task's active attachments, oldest first.
func (s *Store) TaskAttachments(ctx context.Context, q Querier, tid id.ID) ([]api.TaskAttachment, error) {
	rows, err := q.Query(ctx, `select a.id,a.name,a.content_type,a.size_bytes,a.created_at,u.id,u.display_name
		from task_attachments a join users u on u.id=a.added_by
		where a.task_id=$1 and a.removed_at is null order by a.created_at,a.id`, tid)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (api.TaskAttachment, error) {
		var a api.TaskAttachment
		err := r.Scan(&a.ID, &a.Name, &a.ContentType, &a.SizeBytes, &a.CreatedAt, &a.AddedBy.ID, &a.AddedBy.DisplayName)
		a.URL = TaskAttachmentPath(tid, a.ID)
		return a, err
	})
}

// TaskAttachmentUsage returns how many active attachments a task has and
// how many bytes the active attachments of its period take.
func (s *Store) TaskAttachmentUsage(ctx context.Context, q Querier, tid, pid id.ID) (count int, periodBytes int64, err error) {
	err = q.QueryRow(ctx, `select
		(select count(*) from task_attachments where task_id=$1 and removed_at is null),
		(select coalesce(sum(a.size_bytes),0) from task_attachments a join tasks t on t.id=a.task_id where t.period_id=$2 and a.removed_at is null)`, tid, pid).Scan(&count, &periodBytes)
	return count, periodBytes, err
}

// AddTaskAttachment records an uploaded blob as a task's attachment.
func (s *Store) AddTaskAttachment(ctx context.Context, q Querier, tid id.ID, a api.TaskAttachment, key id.ID, sha string) error {
	_, err := q.Exec(ctx, `insert into task_attachments(id,task_id,name,content_type,size_bytes,storage_key,sha256,added_by,created_at)
		values($1,$2,$3,$4,$5,$6,$7,$8,$9)`, a.ID, tid, a.Name, a.ContentType, a.SizeBytes, key, sha, a.AddedBy.ID, a.CreatedAt)
	return err
}

// TaskAttachmentFile is what a download needs of an active attachment.
type TaskAttachmentFile struct {
	Key                     id.ID
	Name, ContentType, Hash string
	Size                    int64
	CreatedAt               time.Time
	AddedBy                 id.ID
}

// TaskAttachment returns an active attachment of a task, for download or
// removal; ErrNotFound when it is absent or removed.
func (s *Store) TaskAttachment(ctx context.Context, q Querier, tid, aid id.ID) (TaskAttachmentFile, error) {
	var f TaskAttachmentFile
	err := q.QueryRow(ctx, `select storage_key,name,content_type,sha256,size_bytes,created_at,added_by from task_attachments
		where id=$1 and task_id=$2 and removed_at is null`, aid, tid).Scan(&f.Key, &f.Name, &f.ContentType, &f.Hash, &f.Size, &f.CreatedAt, &f.AddedBy)
	if err == pgx.ErrNoRows {
		return f, ErrNotFound
	}
	return f, err
}

// RemoveTaskAttachment soft-deletes an attachment; its blob stays.
func (s *Store) RemoveTaskAttachment(ctx context.Context, q Querier, tid, aid, by id.ID, now time.Time) error {
	tag, err := q.Exec(ctx, `update task_attachments set removed_at=$4,removed_by=$3 where id=$1 and task_id=$2 and removed_at is null`, aid, tid, by, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, err = q.Exec(ctx, `update task_attachment_receipts set removed_at=$2 where id=$1 and removed_at is null`, aid, now)
	return err
}
