package api

import (
	"github.com/himasifunej/himasif-space/internal/id"
	"time"
)

type EmergencySnapshotRequest struct {
	IncludeCompleted bool   `json:"includeCompleted"`
	PersonID         *id.ID `json:"personId,omitempty"`
}
type EmergencySnapshot struct {
	ID         id.ID     `json:"id"`
	SpaceID    id.ID     `json:"spaceId"`
	SpaceName  string    `json:"spaceName"`
	PeriodID   id.ID     `json:"periodId"`
	CapturedAt time.Time `json:"capturedAt"`
	CapturedBy PersonRef `json:"capturedBy"`
	RowCount   int       `json:"rowCount"`
}
type EmergencyTask struct {
	RowID          id.ID     `json:"rowId"`
	TaskID         id.ID     `json:"taskId"`
	Version        int64     `json:"version"`
	Title          string    `json:"title"`
	ExpectedOutput string    `json:"expectedOutput"`
	Assignee       PersonRef `json:"assignee"`
	DeadlineKind   string    `json:"deadlineKind"`
	DeadlineValue  string    `json:"deadlineValue"`
	TimeZone       string    `json:"timeZone"`
	Progress       string    `json:"progress"`
	MatchStatus    string    `json:"matchStatus"`
}
type EmergencyChange struct {
	TaskID                 id.ID  `json:"taskId"`
	SnapshotID             id.ID  `json:"snapshotId"`
	RowID                  id.ID  `json:"rowId"`
	Progress               string `json:"progress"`
	ReportedAt             string `json:"reportedAt"`
	ReportedBy             string `json:"reportedBy"`
	Note                   string `json:"note"`
	ExpectedCurrentVersion int64  `json:"expectedCurrentVersion"`
}
type EmergencyReconcileRequest struct {
	Resolution string            `json:"resolution,omitempty"`
	Changes    []EmergencyChange `json:"changes"`
	Reason     string            `json:"reason"`
}
type EmergencyReconcileResult struct {
	RowID           id.ID              `json:"rowId"`
	Status          string             `json:"status"`
	Message         string             `json:"message"`
	Snapshot        *EmergencySnapshot `json:"snapshot,omitempty"`
	Baseline        *EmergencyTask     `json:"baseline,omitempty"`
	Current         *EmergencyTask     `json:"current,omitempty"`
	AppliedProgress string             `json:"appliedProgress"`
	AppliedAt       *time.Time         `json:"appliedAt,omitempty"`
	AppliedBy       string             `json:"appliedBy"`
}
