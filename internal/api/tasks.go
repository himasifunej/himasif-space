package api

import (
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"time"
)

const (
	TaskTodo      = "todo"
	TaskDoing     = "doing"
	TaskSubmitted = "submitted"
	TaskDone      = "done"
	TaskCancelled = "cancelled"
	MatchNone     = "none"
	MatchPending  = "pending"
	MatchVerified = "verified"
	MatchRejected = "rejected"
)

type PersonRef struct {
	ID          id.ID  `json:"id"`
	DisplayName string `json:"displayName"`
}
type TaskPermissions struct {
	Manage   bool `json:"manage"`
	Progress bool `json:"progress"`
	Comment  bool `json:"comment"`
	Help     bool `json:"help"`
	Review   bool `json:"review"`
}

// Task is one canonical object projected into authorized spaces and jobdesk.
// Exactly one of DeadlineDate/DeadlineAt is present; date-only stays date-only.
type Task struct {
	LocalPending    bool    `json:"-"`
	ID              id.ID   `json:"id"`
	PeriodID        id.ID   `json:"periodId"`
	OwnerDivisionID id.ID   `json:"ownerDivisionId"`
	SpaceIDs        []id.ID `json:"spaceIds"`
	// Numbers is the task's number (#T12) in each of its Spaces, by Space
	// ID. Absent from servers before Tahap 6a.
	Numbers          map[string]int64    `json:"numbers,omitempty"`
	Title            string              `json:"title"`
	Description      string              `json:"description"`
	ExpectedOutput   string              `json:"expectedOutput"`
	Category         string              `json:"category"`
	Assignee         PersonRef           `json:"assignee"`
	Participants     []PersonRef         `json:"participants"`
	PlannedStartDate *timeutil.Date      `json:"plannedStartDate,omitempty"`
	PlannedStartAt   *time.Time          `json:"plannedStartAt,omitempty"`
	DeadlineDate     *timeutil.Date      `json:"deadlineDate,omitempty"`
	DeadlineAt       *time.Time          `json:"deadlineAt,omitempty"`
	PostingDate      *timeutil.Date      `json:"postingDate,omitempty"`
	PostingAt        *time.Time          `json:"postingAt,omitempty"`
	TimeZone         string              `json:"timeZone"`
	Progress         string              `json:"progress"`
	MatchStatus      string              `json:"matchStatus"`
	Version          int64               `json:"version"`
	CreatedBy        PersonRef           `json:"createdBy"`
	CreatedAt        time.Time           `json:"createdAt"`
	UpdatedAt        time.Time           `json:"updatedAt"`
	Permissions      TaskPermissions     `json:"permissions"`
	Evidence         []TaskEvidence      `json:"evidence"`
	Comments         []TaskComment       `json:"comments"`
	Checklist        []TaskChecklistItem `json:"checklist"`
	// Attachments are the task's files, oldest first. Absent from servers
	// before Tahap 6b.
	Attachments  []TaskAttachment  `json:"attachments,omitempty"`
	HelpRequests []TaskHelpRequest `json:"helpRequests"`
}

// TaskWriteRequest replaces editable metadata. Owner division is fixed after
// creation. SpaceIDs adds shared spaces; the owner's division space is implicit.
type TaskWriteRequest struct {
	OwnerDivisionID  id.ID          `json:"ownerDivisionId"`
	SpaceIDs         []id.ID        `json:"spaceIds"`
	Title            string         `json:"title"`
	Description      string         `json:"description"`
	ExpectedOutput   string         `json:"expectedOutput"`
	Category         string         `json:"category"`
	AssigneeID       id.ID          `json:"assigneeId"`
	PlannedStartDate *timeutil.Date `json:"plannedStartDate,omitempty"`
	PlannedStartAt   *time.Time     `json:"plannedStartAt,omitempty"`
	DeadlineDate     *timeutil.Date `json:"deadlineDate,omitempty"`
	DeadlineAt       *time.Time     `json:"deadlineAt,omitempty"`
	PostingDate      *timeutil.Date `json:"postingDate,omitempty"`
	PostingAt        *time.Time     `json:"postingAt,omitempty"`
	ExpectedVersion  int64          `json:"expectedVersion,omitempty"`
	Reason           string         `json:"reason,omitempty"`
}

// Move changes canonical progress and ordering. BeforeID must be a readable
// task in the same period/lane. Version conflicts never silently overwrite.
type TaskMoveRequest struct {
	Progress        string `json:"progress"`
	BeforeID        id.ID  `json:"beforeId,omitempty"`
	ExpectedVersion int64  `json:"expectedVersion"`
	Reason          string `json:"reason,omitempty"`
}
type TaskCommentRequest struct {
	Body string `json:"body"`
}

// TaskChecklistItem is one step inside a task. Ticking it does not change the
// task's progress or evidence.
type TaskChecklistItem struct {
	ID        id.ID      `json:"id"`
	Body      string     `json:"body"`
	Done      bool       `json:"done"`
	DoneBy    *PersonRef `json:"doneBy,omitempty"`
	DoneAt    *time.Time `json:"doneAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}
type TaskChecklistRequest struct {
	Body *string `json:"body,omitempty"`
	Done *bool   `json:"done,omitempty"`
}
type TaskComment struct {
	ID        id.ID     `json:"id"`
	Author    PersonRef `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}
type TaskHelpCreateRequest struct {
	Message string `json:"message"`
}
type TaskHelpDecisionRequest struct {
	Approved        bool  `json:"approved"`
	ExpectedVersion int64 `json:"expectedVersion"`
}
type TaskHelpRequest struct {
	ID        id.ID     `json:"id"`
	User      PersonRef `json:"user"`
	Message   string    `json:"message"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}
type TaskEvidenceRequest struct {
	URL             string `json:"url"`
	Note            string `json:"note"`
	ExpectedVersion int64  `json:"expectedVersion"`
}
type TaskEvidenceReviewRequest struct {
	Matched         bool   `json:"matched"`
	Reason          string `json:"reason"`
	ExpectedVersion int64  `json:"expectedVersion"`
}
type TaskEvidence struct {
	ID           id.ID      `json:"id"`
	SubmittedBy  PersonRef  `json:"submittedBy"`
	URL          string     `json:"url"`
	Note         string     `json:"note"`
	MatchStatus  string     `json:"matchStatus"`
	SubmittedAt  time.Time  `json:"submittedAt"`
	RecordedAt   time.Time  `json:"recordedAt"`
	ReviewedAt   *time.Time `json:"reviewedAt,omitempty"`
	ReviewedBy   *PersonRef `json:"reviewedBy,omitempty"`
	ReviewReason string     `json:"reviewReason,omitempty"`
}
type TaskEvent struct {
	ID        id.ID     `json:"id"`
	Actor     PersonRef `json:"actor"`
	Action    string    `json:"action"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"createdAt"`
	Before    any       `json:"before,omitempty"`
	After     any       `json:"after,omitempty"`
}
type Notification struct {
	ID        id.ID      `json:"id"`
	TaskID    id.ID      `json:"taskId"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	CreatedAt time.Time  `json:"createdAt"`
	ReadAt    *time.Time `json:"readAt,omitempty"`
}

// TaskFilter is a projection filter; it never grants access to a task.
type TaskFilter struct {
	AssigneeID id.ID
	Progress   string
	DueBefore  *timeutil.Date
}

// TaskAttachment is a file attached to a task (Tahap 6b). URL is the
// download path; the file itself is never public.
type TaskAttachment struct {
	ID          id.ID     `json:"id"`
	Name        string    `json:"name"`
	ContentType string    `json:"contentType"`
	SizeBytes   int64     `json:"sizeBytes"`
	URL         string    `json:"url"`
	AddedBy     PersonRef `json:"addedBy"`
	CreatedAt   time.Time `json:"createdAt"`
}
