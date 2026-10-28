package api

import (
	"time"

	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// Error codes of Sesi 1B–1C.
const (
	CodeConflict     = "conflict"
	CodePeriodClosed = "period_closed"
	CodeInvalidToken = "invalid_token"
)

// Built-in roles. kadiv comes from the admin's assignment, 45 from the
// original kadiv's appointment; organization_admin is the shared account.
const (
	RoleMember            = "member"
	RoleCompanion         = "companion"
	RoleKadiv             = "kadiv"
	Role45                = "45"
	RoleOrganizationAdmin = "organization_admin"
)

// Mediatek's groupings; they grant no capability (ACC-06).
const (
	GroupTekno  = "tekno"
	GroupDesain = "desain"
	GroupVideo  = "video"
)

// Operational capabilities.
const (
	CapSpaceRead      = "space.read"
	CapMembersRead    = "members.read"
	CapTasksOwn       = "tasks.own"
	CapCommentsWrite  = "comments.write"
	CapHelpRequest    = "help.request"
	CapNotesRead      = "notes.read"
	CapNotesManage    = "notes.manage"
	CapTasksManage    = "tasks.manage"
	CapResultsReview  = "results.review"
	CapPingSend       = "ping.send"
	CapWhatsAppManage = "whatsapp.manage"
)

// Administrative capabilities, never held by a 45 nor given by a custom
// role (ACC-09, ACC-11).
const (
	CapInvitationsManage = "invitations.manage"
	CapMembersManage     = "members.manage"
	CapRolesManage       = "roles.manage"
	CapArchivesManage    = "archives.manage"
	Cap45Manage          = "acting45.manage"
	CapPeriodsManage     = "periods.manage"
	CapKadivManage       = "kadiv.manage"
)

// Kinds of space and of access to one.
const (
	SpaceDivision = "division"
	SpaceShared   = "shared"

	AccessMember  = "member"
	AccessArchive = "archive"
	AccessAdmin   = "admin"
)

// Invitation statuses.
const (
	InvitationPending  = "pending"
	InvitationExpired  = "expired"
	InvitationRevoked  = "revoked"
	InvitationAccepted = "accepted"
)

// Access is what the caller may do in one division of a period. With
// ReadOnly (a closed period) Capabilities holds only reading ones.
type Access struct {
	DivisionID    id.ID    `json:"divisionId"`
	Roles         []string `json:"roles"`
	CustomRoleIDs []id.ID  `json:"customRoleIds"`
	Capabilities  []string `json:"capabilities"`
	PeriodStatus  string   `json:"periodStatus"`
	ReadOnly      bool     `json:"readOnly"`
}

// Space is a space the caller may open. Divisions names the divisions of
// the space; Roles holds only the caller's own bindings.
type Space struct {
	ID           id.ID      `json:"id"`
	PeriodID     id.ID      `json:"periodId"`
	Code         string     `json:"code"`
	Name         string     `json:"name"`
	Kind         string     `json:"kind"`
	Divisions    []Division `json:"divisions"`
	Roles        []Access   `json:"roles"`
	Capabilities []string   `json:"capabilities"`
	PeriodStatus string     `json:"periodStatus"`
	ReadOnly     bool       `json:"readOnly"`
	Access       string     `json:"access"`
}

// Member is an active membership of a division in a period. Emails of
// members are not listed.
type Member struct {
	ID            id.ID    `json:"id"`
	UserID        id.ID    `json:"userId"`
	PeriodID      id.ID    `json:"periodId"`
	DivisionID    id.ID    `json:"divisionId"`
	DisplayName   string   `json:"displayName"`
	Role          string   `json:"role"`
	Roles         []string `json:"roles"`
	Groups        []string `json:"groups"`
	CustomRoleIDs []id.ID  `json:"customRoleIds"`
	Capabilities  []string `json:"capabilities"`
}

// Invitation is an invitation as its managers see it, without its token.
type Invitation struct {
	ID         id.ID     `json:"id"`
	PeriodID   id.ID     `json:"periodId"`
	DivisionID id.ID     `json:"divisionId"`
	Email      string    `json:"email"`
	Status     string    `json:"status"`
	ExpiresAt  time.Time `json:"expiresAt"`
	CreatedAt  time.Time `json:"createdAt"`
}

type InviteRequest struct {
	Email string `json:"email"`
}

// InvitationTokenRequest carries the token of an invitation link.
type InvitationTokenRequest struct {
	Token string `json:"token"`
}

// InvitationPreview is what the holder of an invitation link sees before
// accepting it. AccountExists picks the flow: sign in, or verify the email
// and create an account.
type InvitationPreview struct {
	Email         string    `json:"email"`
	PeriodID      id.ID     `json:"periodId"`
	PeriodLabel   string    `json:"periodLabel"`
	DivisionID    id.ID     `json:"divisionId"`
	DivisionName  string    `json:"divisionName"`
	ExpiresAt     time.Time `json:"expiresAt"`
	AccountExists bool      `json:"accountExists"`
}

// InvitationAcceptRequest accepts an invitation. A new account sends the
// email proof, its name and password without a session; an existing one
// sends only the token, with the session of the invited account.
type InvitationAcceptRequest struct {
	Token             string `json:"token"`
	VerificationToken string `json:"verificationToken,omitempty"`
	DisplayName       string `json:"displayName,omitempty"`
	Password          string `json:"password,omitempty"`
}

type InvitationAcceptResponse struct {
	User       User   `json:"user"`
	Membership Member `json:"membership"`
}

type RecoveryRequest struct {
	Email string `json:"email"`
}

type RecoveryResetRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// PasswordChangeRequest changes the caller's password; the caller's other
// sessions end.
type PasswordChangeRequest struct {
	CurrentPassword string `json:"currentPassword"`
	Password        string `json:"password"`
}

type CreatePeriodRequest struct {
	Label    string        `json:"label"`
	StartsOn timeutil.Date `json:"startsOn"`
	EndsOn   timeutil.Date `json:"endsOn"`
	TimeZone string        `json:"timeZone"`
	// DivisionIDs empty uses every division.
	DivisionIDs []id.ID `json:"divisionIds"`
}

type SetKadivRequest struct {
	UserID id.ID `json:"userId"`
}

// UpdateMemberRequest replaces a member's operational configuration; every
// field is required.
type UpdateMemberRequest struct {
	Role          *string  `json:"role"`
	Groups        []string `json:"groups"`
	CustomRoleIDs []id.ID  `json:"customRoleIds"`
}

type RoleRequest struct {
	Name         string   `json:"name"`
	Capabilities []string `json:"capabilities"`
}

type CustomRole struct {
	ID           id.ID    `json:"id"`
	PeriodID     id.ID    `json:"periodId"`
	DivisionID   id.ID    `json:"divisionId"`
	Name         string   `json:"name"`
	Capabilities []string `json:"capabilities"`
}

// Appoint45Request appoints the 45. Capabilities, when given, narrows the
// default (every operational capability of a kadiv beyond a member's).
type Appoint45Request struct {
	UserID       id.ID    `json:"userId"`
	Capabilities []string `json:"capabilities,omitempty"`
}

type Appointment45 struct {
	ID           id.ID     `json:"id"`
	PeriodID     id.ID     `json:"periodId"`
	DivisionID   id.ID     `json:"divisionId"`
	UserID       id.ID     `json:"userId"`
	Capabilities []string  `json:"capabilities"`
	AppointedAt  time.Time `json:"appointedAt"`
}

type ArchiveRequest struct {
	UserID  id.ID `json:"userId"`
	SpaceID id.ID `json:"spaceId"`
}

// ArchiveGrant is read-only access of a member of the scope (PeriodID,
// DivisionID) to one space of a closed period.
type ArchiveGrant struct {
	ID         id.ID     `json:"id"`
	PeriodID   id.ID     `json:"periodId"`
	DivisionID id.ID     `json:"divisionId"`
	UserID     id.ID     `json:"userId"`
	SpaceID    id.ID     `json:"spaceId"`
	GrantedBy  id.ID     `json:"grantedBy"`
	CreatedAt  time.Time `json:"createdAt"`
}

// HandoverRequest sets the handover checklist of a period. RemainingWork
// is required: the work left and how it is closed or forwarded, or why
// none is left.
type HandoverRequest struct {
	SuccessorPeriodID  *id.ID `json:"successorPeriodId"`
	IncomingLeadership string `json:"incomingLeadership"`
	AdminCustodian     string `json:"adminCustodian"`
	BotManagement      string `json:"botManagement"`
	ArchivePlan        string `json:"archivePlan"`
	RemainingWork      string `json:"remainingWork"`
}

// Handover is the handover checklist of a period; UpdatedAt is nil until
// it is first written.
type Handover struct {
	PeriodID           id.ID      `json:"periodId"`
	SuccessorPeriodID  *id.ID     `json:"successorPeriodId"`
	IncomingLeadership string     `json:"incomingLeadership"`
	AdminCustodian     string     `json:"adminCustodian"`
	BotManagement      string     `json:"botManagement"`
	ArchivePlan        string     `json:"archivePlan"`
	RemainingWork      string     `json:"remainingWork"`
	UpdatedAt          *time.Time `json:"updatedAt"`
}

// AdminEmailRequest asks for a proof of a new recovery email of the
// organization admin, sent to that email.
type AdminEmailRequest struct {
	CurrentPassword string `json:"currentPassword"`
	Email           string `json:"email"`
}

// AdminHandoverRequest hands the organization admin account over: a new
// password and, optionally, a new email proved by VerificationToken. Every
// session of the account ends, this one included.
type AdminHandoverRequest struct {
	CurrentPassword   string `json:"currentPassword"`
	Password          string `json:"password"`
	Email             string `json:"email,omitempty"`
	VerificationToken string `json:"verificationToken,omitempty"`
	Custodian         string `json:"custodian"`
}
