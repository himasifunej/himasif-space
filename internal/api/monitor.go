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

// MonitorOverview is the monitoring dashboard of a period (canvas
// "Pemantauan organisasi"): every space with its work, and the people with
// the most running tasks. It is a snapshot at GeneratedAt.
type MonitorOverview struct {
	PeriodID    id.ID                 `json:"periodId"`
	GeneratedAt time.Time             `json:"generatedAt"`
	Totals      MonitorTotals         `json:"totals"`
	Spaces      []MonitorSpaceSummary `json:"spaces"`
	Workload    []MonitorWorkload     `json:"workload"`
	// HeavyLoad is how many running tasks make one person's load heavy.
	HeavyLoad int `json:"heavyLoad"`
}

// MonitorTotals adds up the spaces of a period.
type MonitorTotals struct {
	Running          int `json:"running"`
	RunningSpaces    int `json:"runningSpaces"`
	Overdue          int `json:"overdue"`
	AwaitingReview   int `json:"awaitingReview"`
	MeetingsThisWeek int `json:"meetingsThisWeek"`
	MeetingsNoNotes  int `json:"meetingsNoNotes"`
}

// MonitorSpaceSummary is one row of the dashboard.
type MonitorSpaceSummary struct {
	SpaceID        id.ID      `json:"spaceId"`
	Name           string     `json:"name"`
	Kind           string     `json:"kind"`
	Divisions      []Division `json:"divisions"`
	Running        int        `json:"running"`
	Done           int        `json:"done"`
	Overdue        int        `json:"overdue"`
	AwaitingReview int        `json:"awaitingReview"`
	Heavy          int        `json:"heavy"`
	// Progress is done over done and running, 0–100.
	Progress       int        `json:"progress"`
	LastMeeting    string     `json:"lastMeeting,omitempty"`
	LastMeetingAt  *time.Time `json:"lastMeetingAt,omitempty"`
	SOPFindings    int        `json:"sopFindings"`
	SOPNeedsReview int        `json:"sopNeedsReview"`
	// Own says the caller is a member of this space, not monitoring it.
	Own bool `json:"own,omitempty"`
}

// MonitorWorkload is one person's running tasks across the period.
type MonitorWorkload struct {
	UserID      id.ID    `json:"userId"`
	DisplayName string   `json:"displayName"`
	Divisions   []string `json:"divisions"`
	Running     int      `json:"running"`
}

// Channels of the weekly digest of BPH.
const (
	DigestWhatsApp = "whatsapp"
	DigestEmail    = "email"
	DigestBoth     = "both"
)

// MonitorDigestSettings is a monitor's weekly digest (MON-08): whether it
// is sent, by which channel, and what last week's would say.
type MonitorDigestSettings struct {
	Enabled bool   `json:"enabled"`
	Channel string `json:"channel"`
	// Monitor says the caller holds org.monitor in a period now; only then
	// is a digest sent.
	Monitor bool `json:"monitor"`
	// Preview is last week's digest as WhatsApp would show it; empty for
	// non-monitors.
	Preview string `json:"preview,omitempty"`
	// WhatsAppLinked says a WhatsApp number is linked to the account in
	// the period, so a WhatsApp digest can reach it.
	WhatsAppLinked bool `json:"whatsAppLinked"`
}

// MonitorDigestRequest changes a monitor's digest.
type MonitorDigestRequest struct {
	Enabled bool   `json:"enabled"`
	Channel string `json:"channel"`
}
