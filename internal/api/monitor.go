package api

import (
	"time"

	"github.com/himasifunej/himasif-space/internal/id"
)

// CapOrgMonitor is the organization monitoring of BPH (OPEN-15): granted by
// the organization admin to one account for one period, never inherited by
// a role or a successor, and revoked when the period closes. It opens every
// division of the period read-mostly; personal reminders and drafts stay
// closed.
const CapOrgMonitor = "org.monitor"

// RoleMonitor names a monitor's binding to a division it is not a member of.
const RoleMonitor = "monitor"

// AccessMonitor is the access to a space through org.monitor.
const AccessMonitor = "monitor"

// Reasons a monitoring grant ended.
const (
	MonitorRevokedAdmin  = "admin"
	MonitorRevokedClosed = "period_closed"
)

// MonitorGrant is org.monitor held by an account in a period.
type MonitorGrant struct {
	ID            id.ID      `json:"id"`
	PeriodID      id.ID      `json:"periodId"`
	UserID        id.ID      `json:"userId"`
	DisplayName   string     `json:"displayName"`
	GrantedBy     id.ID      `json:"grantedBy"`
	GrantedAt     time.Time  `json:"grantedAt"`
	RevokedAt     *time.Time `json:"revokedAt,omitempty"`
	RevokedReason string     `json:"revokedReason,omitempty"`
}

// MonitorCandidate is a BPH member of a period, with their grant if any.
type MonitorCandidate struct {
	UserID      id.ID         `json:"userId"`
	DisplayName string        `json:"displayName"`
	Role        string        `json:"role"`
	Grant       *MonitorGrant `json:"grant,omitempty"`
}

// MonitorGrants is the admin's view of monitoring in a period.
type MonitorGrants struct {
	PeriodID   id.ID              `json:"periodId"`
	Candidates []MonitorCandidate `json:"candidates"`
	Grants     []MonitorGrant     `json:"grants"`
}

// MonitorGrantRequest grants org.monitor to a BPH member.
type MonitorGrantRequest struct {
	UserID id.ID `json:"userId"`
}

// MonitorAuditEvent is one entry of the monitoring log the admin reads:
// grants, revocations, and every read or action of a monitor in a division.
type MonitorAuditEvent struct {
	ID           id.ID     `json:"id"`
	OccurredAt   time.Time `json:"occurredAt"`
	Action       string    `json:"action"`
	ActorID      *id.ID    `json:"actorId,omitempty"`
	ActorName    string    `json:"actorName"`
	SubjectID    *id.ID    `json:"subjectId,omitempty"`
	SubjectName  string    `json:"subjectName,omitempty"`
	DivisionID   *id.ID    `json:"divisionId,omitempty"`
	DivisionName string    `json:"divisionName,omitempty"`
	Detail       string    `json:"detail,omitempty"`
}
