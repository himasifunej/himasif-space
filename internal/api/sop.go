package api

import (
	"encoding/json"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/sop"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"time"
)

type SOPQuarter struct {
	Label    string        `json:"label"`
	StartsOn timeutil.Date `json:"startsOn"`
	EndsOn   timeutil.Date `json:"endsOn"`
}
type SOPSettings struct {
	Rules                      []string       `json:"rules"`
	FollowupClock              string         `json:"followupClock"`
	WeeklyClock                string         `json:"weeklyClock"`
	WeeklyWeekday              *int           `json:"weeklyWeekday,omitempty"`
	Quarters                   []SOPQuarter   `json:"quarters"`
	UrgentWindowStart          *timeutil.Date `json:"urgentWindowStart,omitempty"`
	UrgentWindowEnd            *timeutil.Date `json:"urgentWindowEnd,omitempty"`
	UrgentLimit                *int           `json:"urgentLimit,omitempty"`
	BotFollowupCountsRequested bool           `json:"botFollowupCountsRequested"`
	BotAgreementNote           string         `json:"botAgreementNote"`
}
type SOPPolicy struct {
	ID                        id.ID       `json:"id"`
	PeriodID                  id.ID       `json:"periodId"`
	DivisionID                id.ID       `json:"divisionId"`
	SpaceID                   id.ID       `json:"spaceId"`
	DocumentVersion           string      `json:"documentVersion"`
	Enabled                   bool        `json:"enabled"`
	Settings                  SOPSettings `json:"settings"`
	BotFollowupCountsApproved bool        `json:"botFollowupCountsApproved"`
	Version                   int64       `json:"version"`
	UpdatedAt                 time.Time   `json:"updatedAt"`
}
type SOPPolicyRequest struct {
	SpaceID         id.ID       `json:"spaceId"`
	DocumentVersion string      `json:"documentVersion"`
	Enabled         bool        `json:"enabled"`
	Settings        SOPSettings `json:"settings"`
	ExpectedVersion int64       `json:"expectedVersion"`
	Reason          string      `json:"reason"`
}
type SOPEvaluatorGrant struct {
	ID          id.ID      `json:"id"`
	UserID      id.ID      `json:"userId"`
	DisplayName string     `json:"displayName"`
	CanReview   bool       `json:"canReview"`
	GrantedAt   time.Time  `json:"grantedAt"`
	RevokedAt   *time.Time `json:"revokedAt,omitempty"`
}
type SOPEvaluatorGrantRequest struct {
	UserID    id.ID  `json:"userId"`
	CanReview bool   `json:"canReview"`
	Reason    string `json:"reason"`
}
type SOPAccess struct {
	DivisionSlug  string `json:"divisionSlug"`
	CanApproveBot bool   `json:"canApproveBot"`
	PeriodID      id.ID  `json:"periodId"`
	DivisionID    id.ID  `json:"divisionId"`
	DivisionName  string `json:"divisionName"`
	CanConfigure  bool   `json:"canConfigure"`
	CanReview     bool   `json:"canReview"`
	ReadOnly      bool   `json:"readOnly"`
}
type SOPSourceChange struct {
	Sequence   int64     `json:"sequence"`
	SourceType string    `json:"sourceType"`
	SourceID   id.ID     `json:"sourceId"`
	RecordedAt time.Time `json:"recordedAt"`
}

type SOPHistory struct {
	Transfers     []SOPTransfer     `json:"transfers"`
	SourceChanges []SOPSourceChange `json:"sourceChanges"`
	Decisions     []SOPDecision     `json:"decisions"`
	Observations  []SOPObservation  `json:"observations"`
}
type SOPPerson struct {
	ID          id.ID  `json:"id"`
	DisplayName string `json:"displayName"`
	Active      bool   `json:"active"`
}
type SOPEvaluatorCandidate struct {
	ID           id.ID  `json:"id"`
	DisplayName  string `json:"displayName"`
	DivisionName string `json:"divisionName"`
	CanReview    bool   `json:"canReview"`
}
type SOPContentRef struct {
	ID          id.ID          `json:"id"`
	Title       string         `json:"title"`
	PostingDate *timeutil.Date `json:"postingDate,omitempty"`
	PostingAt   *time.Time     `json:"postingAt,omitempty"`
	PersonID    *id.ID         `json:"personId,omitempty"`
	PersonName  string         `json:"personName"`
}
type SOPContext struct {
	UrgentExceptionCategory string       `json:"urgentExceptionCategory"`
	ContentID               id.ID        `json:"contentId"`
	DivisionID              id.ID        `json:"divisionId"`
	ConceptMature           sop.Evidence `json:"conceptMature"`
	Followup                sop.Evidence `json:"followup"`
	FinalApproved           sop.Evidence `json:"finalApproved"`
	UrgentConfirmed         sop.Evidence `json:"urgentConfirmed"`
	Urgent                  *bool        `json:"urgent,omitempty"`
	UrgentException         sop.Evidence `json:"urgentException"`
	SpecialAgreement        sop.Evidence `json:"specialAgreement"`
	SpecialAgreementRules   []string     `json:"specialAgreementRules"`
	Version                 int64        `json:"version"`
}
type SOPContextRequest struct {
	Context         SOPContext `json:"context"`
	ExpectedVersion int64      `json:"expectedVersion"`
	Reason          string     `json:"reason"`
}
type SOPObservation struct {
	EngineVersion    string          `json:"engineVersion"`
	EventOn          timeutil.Date   `json:"eventOn"`
	ContentTitle     string          `json:"contentTitle"`
	ProposedPersonID *id.ID          `json:"proposedPersonId,omitempty"`
	ID               id.ID           `json:"id"`
	DocumentVersion  string          `json:"documentVersion"`
	PolicyVersion    int64           `json:"policyVersion"`
	Observation      sop.Observation `json:"observation"`
	Input            sop.Input       `json:"input"`
	RecordedAt       time.Time       `json:"recordedAt"`
}
type SOPFinding struct {
	Decision              *SOPObservation `json:"decision,omitempty"`
	DecisionReason        string          `json:"decisionReason"`
	DecidedBy             string          `json:"decidedBy"`
	DecidedAt             *time.Time      `json:"decidedAt,omitempty"`
	ID                    id.ID           `json:"id"`
	PeriodID              id.ID           `json:"periodId"`
	DivisionID            id.ID           `json:"divisionId"`
	ContentID             id.ID           `json:"contentId"`
	ContentTitle          string          `json:"contentTitle"`
	Rule                  string          `json:"rule"`
	EventOn               timeutil.Date   `json:"eventOn"`
	PersonID              *id.ID          `json:"personId,omitempty"`
	PersonName            string          `json:"personName"`
	State                 string          `json:"state"`
	NeedsRecheck          bool            `json:"needsRecheck"`
	Latest                SOPObservation  `json:"latest"`
	DecisionObservationID *id.ID          `json:"decisionObservationId,omitempty"`
	Version               int64           `json:"version"`
	CreatedAt             time.Time       `json:"createdAt"`
	UpdatedAt             time.Time       `json:"updatedAt"`
}
type SOPFindingPage struct {
	SummaryOnly bool         `json:"summaryOnly"`
	Items       []SOPFinding `json:"items"`
	NextCursor  string       `json:"nextCursor"`
	HasMore     bool         `json:"hasMore"`
}
type SOPDecisionRequest struct {
	Outcome         string `json:"outcome,omitempty"`
	Action          string `json:"action"`
	ExpectedVersion int64  `json:"expectedVersion"`
	ObservationID   id.ID  `json:"observationId"`
	PersonID        *id.ID `json:"personId,omitempty"`
	Reason          string `json:"reason"`
}
type SOPManualFindingRequest struct {
	ContentID              id.ID        `json:"contentId"`
	Rule                   string       `json:"rule"`
	Evidence               sop.Evidence `json:"evidence"`
	PersonID               *id.ID       `json:"personId,omitempty"`
	ExpectedPolicyVersion  int64        `json:"expectedPolicyVersion"`
	ExpectedFindingVersion int64        `json:"expectedFindingVersion"`
	Reason                 string       `json:"reason"`
}
type SOPDecision struct {
	ID        id.ID           `json:"id"`
	ActorName string          `json:"actorName"`
	Action    string          `json:"action"`
	Reason    string          `json:"reason"`
	Before    json.RawMessage `json:"before"`
	After     json.RawMessage `json:"after"`
	CreatedAt time.Time       `json:"createdAt"`
}
type SOPReportSummary struct {
	Total                int `json:"total"`
	NeedsReview          int `json:"needsReview"`
	Confirmed            int `json:"confirmed"`
	Dismissed            int `json:"dismissed"`
	NeedsRecheck         int `json:"needsRecheck"`
	InsufficientEvidence int `json:"insufficientEvidence"`
}
type SOPReport struct {
	PeriodID        id.ID            `json:"periodId"`
	DivisionID      id.ID            `json:"divisionId"`
	DivisionName    string           `json:"divisionName"`
	Label           string           `json:"label"`
	StartsOn        timeutil.Date    `json:"startsOn"`
	EndsOn          timeutil.Date    `json:"endsOn"`
	GeneratedAt     time.Time        `json:"generatedAt"`
	PolicyVersion   int64            `json:"policyVersion"`
	DocumentVersion string           `json:"documentVersion"`
	Summary         SOPReportSummary `json:"summary"`
	Findings        []SOPFinding     `json:"findings"`
}

// SOPDelivery is a narrow read model; it grants no access to WA operations.
type SOPDelivery struct {
	SuppressedAt      *time.Time      `json:"suppressedAt,omitempty"`
	SuppressionReason string          `json:"suppressionReason,omitempty"`
	ID                id.ID           `json:"id"`
	Kind              string          `json:"kind"`
	Title             string          `json:"title"`
	PolicyVersion     int64           `json:"policyVersion"`
	ScheduledAt       time.Time       `json:"scheduledAt"`
	CreatedAt         time.Time       `json:"createdAt"`
	Status            string          `json:"status"`
	TargetName        string          `json:"targetName"`
	Attempts          int             `json:"attempts"`
	Error             string          `json:"error"`
	AcceptedAt        *time.Time      `json:"acceptedAt,omitempty"`
	UpdatedAt         *time.Time      `json:"updatedAt,omitempty"`
	Snapshot          json.RawMessage `json:"snapshot,omitempty"`
}
type SOPDeliveryPage struct {
	Items      []SOPDelivery `json:"items"`
	HasMore    bool          `json:"hasMore"`
	NextCursor string        `json:"nextCursor"`
}

type SOPTransferRequest struct {
	ExpectedVersion      int64  `json:"expectedVersion"`
	ObservationID        id.ID  `json:"observationId"`
	TargetDivisionID     id.ID  `json:"targetDivisionId"`
	TargetRule           string `json:"targetRule"`
	TargetPersonID       *id.ID `json:"targetPersonId,omitempty"`
	TargetPolicyVersion  int64  `json:"targetPolicyVersion"`
	TargetFindingVersion int64  `json:"targetFindingVersion"`
	Reason               string `json:"reason"`
}
type SOPTransferPreview struct {
	PolicyVersion   int64       `json:"policyVersion"`
	DocumentVersion string      `json:"documentVersion"`
	Finding         *SOPFinding `json:"finding,omitempty"`
	People          []SOPPerson `json:"people"`
}
type SOPTransfer struct {
	ID               id.ID     `json:"id"`
	SourceFindingID  id.ID     `json:"sourceFindingId"`
	TargetFindingID  id.ID     `json:"targetFindingId"`
	SourceDivisionID id.ID     `json:"sourceDivisionId"`
	TargetDivisionID id.ID     `json:"targetDivisionId"`
	SourceRule       string    `json:"sourceRule"`
	TargetRule       string    `json:"targetRule"`
	ActorName        string    `json:"actorName"`
	Reason           string    `json:"reason"`
	CreatedAt        time.Time `json:"createdAt"`
}

type SOPScheduleReadiness struct {
	Kind        string     `json:"kind"`
	ScheduledAt *time.Time `json:"scheduledAt,omitempty"`
	Ready       bool       `json:"ready"`
	Blockers    []string   `json:"blockers"`
	Warnings    []string   `json:"warnings"`
}
type SOPReadiness struct {
	GeneratedAt   time.Time             `json:"generatedAt"`
	PolicyVersion int64                 `json:"policyVersion"`
	Weekly        SOPScheduleReadiness  `json:"weekly"`
	Followup      *SOPScheduleReadiness `json:"followup,omitempty"`
}

// SOPRuleSummary is one rule of an Evaluasi SOP summary (Tahap 7a).
// Compliant counts contents on time or within the limit; the finding
// states count the findings of the range.
type SOPRuleSummary struct {
	Rule          string `json:"rule"`
	Evaluated     int    `json:"evaluated"`
	Compliant     int    `json:"compliant"`
	Insufficient  int    `json:"insufficient"`
	NotApplicable int    `json:"notApplicable"`
	NeedsReview   int    `json:"needsReview"`
	Confirmed     int    `json:"confirmed"`
	Dismissed     int    `json:"dismissed"`
	Corrected     int    `json:"corrected"`
}

// SOPSummary is the Evaluasi SOP chart of a range (a quarter of the policy).
type SOPSummary struct {
	Label         string           `json:"label"`
	StartsOn      timeutil.Date    `json:"startsOn"`
	EndsOn        timeutil.Date    `json:"endsOn"`
	PolicyEnabled bool             `json:"policyEnabled"`
	Contents      int              `json:"contents"`
	Rules         []SOPRuleSummary `json:"rules"`
}
