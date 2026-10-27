package api

import (
 "time"
 "github.com/himasifunej/himasif-space/internal/id"
 "github.com/himasifunej/himasif-space/internal/timeutil"
)

const (
 CodeConflict = "conflict"
 CodePeriodClosed = "period_closed"
 CodeInvalidToken = "invalid_token"
 RoleMember = "member"
 RoleCompanion = "companion"
 RoleKadiv = "kadiv"
 Role45 = "45"
 CapSpaceRead = "space.read"
 CapMembersRead = "members.read"
 CapTasksOwn = "tasks.own"
 CapCommentsWrite = "comments.write"
 CapHelpRequest = "help.request"
 CapNotesRead = "notes.read"
 CapTasksManage = "tasks.manage"
 CapResultsReview = "results.review"
 CapPingSend = "ping.send"
 CapWhatsAppManage = "whatsapp.manage"
 CapInvitationsManage = "invitations.manage"
 CapMembersManage = "members.manage"
 CapRolesManage = "roles.manage"
 CapArchivesManage = "archives.manage"
 Cap45Manage = "acting45.manage"
 CapPeriodsManage = "periods.manage"
 CapKadivManage = "kadiv.manage"
)

type Access struct {
 DivisionID id.ID `json:"divisionId"`
 Roles []string `json:"roles"`
 CustomRoleIDs []id.ID `json:"customRoleIds"`
 Capabilities []string `json:"capabilities"`
 PeriodStatus string `json:"periodStatus"`
 ReadOnly bool `json:"readOnly"`
}
type Space struct {
 ID id.ID `json:"id"`
 PeriodID id.ID `json:"periodId"`
 Code string `json:"code"`
 Name string `json:"name"`
 Kind string `json:"kind"`
 Divisions []Division `json:"divisions"`
 Roles []Access `json:"roles"`
 Capabilities []string `json:"capabilities"`
 PeriodStatus string `json:"periodStatus"`
 ReadOnly bool `json:"readOnly"`
 Access string `json:"access"`
}
type Member struct {
 ID id.ID `json:"id"`
 UserID id.ID `json:"userId"`
 PeriodID id.ID `json:"periodId"`
 DivisionID id.ID `json:"divisionId"`
 DisplayName string `json:"displayName"`
 Role string `json:"role"`
 Roles []string `json:"roles"`
 Groups []string `json:"groups"`
 CustomRoleIDs []id.ID `json:"customRoleIds"`
 Capabilities []string `json:"capabilities"`
}
type Invitation struct {
 ID id.ID `json:"id"`
 PeriodID id.ID `json:"periodId"`
 DivisionID id.ID `json:"divisionId"`
 Email string `json:"email"`
 Status string `json:"status"`
 ExpiresAt time.Time `json:"expiresAt"`
 CreatedAt time.Time `json:"createdAt"`
}
type InviteRequest struct { Email string `json:"email"` }
type InvitationVerifyRequest struct { Token string `json:"token"` }
type InvitationAcceptRequest struct {
 Token string `json:"token"`
 VerificationToken string `json:"verificationToken,omitempty"`
 DisplayName string `json:"displayName,omitempty"`
 Password string `json:"password,omitempty"`
}
type InvitationAcceptResponse struct {
 User User `json:"user"`
 Membership Member `json:"membership"`
}
type RecoveryRequest struct { Email string `json:"email"` }
type RecoveryResetRequest struct {
 Token string `json:"token"`
 Password string `json:"password"`
}
type CreatePeriodRequest struct {
 Label string `json:"label"`
 StartsOn timeutil.Date `json:"startsOn"`
 EndsOn timeutil.Date `json:"endsOn"`
 TimeZone string `json:"timeZone"`
 DivisionIDs []id.ID `json:"divisionIds"`
}
type SetKadivRequest struct { UserID id.ID `json:"userId"` }
type UpdateMemberRequest struct {
 Role string `json:"role"`
 Groups []string `json:"groups"`
 CustomRoleIDs []id.ID `json:"customRoleIds"`
}
type RoleRequest struct {
 Name string `json:"name"`
 Capabilities []string `json:"capabilities"`
}
type CustomRole struct {
 ID id.ID `json:"id"`
 PeriodID id.ID `json:"periodId"`
 DivisionID id.ID `json:"divisionId"`
 Name string `json:"name"`
 Capabilities []string `json:"capabilities"`
}
type Appoint45Request struct {
 UserID id.ID `json:"userId"`
 Capabilities []string `json:"capabilities,omitempty"`
}
type Appointment45 struct {
 ID id.ID `json:"id"`
 PeriodID id.ID `json:"periodId"`
 DivisionID id.ID `json:"divisionId"`
 UserID id.ID `json:"userId"`
 Capabilities []string `json:"capabilities"`
 AppointedAt time.Time `json:"appointedAt"`
}
type ArchiveRequest struct {
 UserID id.ID `json:"userId"`
 SpaceID id.ID `json:"spaceId"`
}
type ArchiveGrant struct {
 ID id.ID `json:"id"`
 PeriodID id.ID `json:"periodId"`
 DivisionID id.ID `json:"divisionId"`
 UserID id.ID `json:"userId"`
 SpaceID id.ID `json:"spaceId"`
 GrantedBy id.ID `json:"grantedBy"`
 CreatedAt time.Time `json:"createdAt"`
}
type Handover struct {
 SuccessorPeriodID id.ID `json:"successorPeriodId"`
 IncomingLeadership string `json:"incomingLeadership"`
 AdminCustodian string `json:"adminCustodian"`
 BotManagement string `json:"botManagement"`
 ArchivePlan string `json:"archivePlan"`
 RemainingWork string `json:"remainingWork"`
}
// A change of the admin recovery email is proved separately before rotation.
type AdminHandoverRequest struct {
 CurrentPassword string `json:"currentPassword"`
 Password string `json:"password"`
 Email string `json:"email"`
 VerificationToken string `json:"verificationToken"`
 Custodian string `json:"custodian"`
}
type AdminEmailRequest struct {
 CurrentPassword string `json:"currentPassword"`
 Email string `json:"email"`
}
