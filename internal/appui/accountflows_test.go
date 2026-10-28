package appui

import (
	"context"
	"strings"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

type accountTestAPI struct {
	*fakeAPI
	preview    api.InvitationPreview
	accepts    []api.InvitationAcceptRequest
	recoveries []api.RecoveryRequest
	resets     []api.RecoveryResetRequest
	changes    []api.PasswordChangeRequest
	handovers  []api.AdminHandoverRequest
	proofs     int
	inspects   int
	resultErr  error
	gate       chan struct{}
}

func (f *accountTestAPI) InspectInvitation(context.Context, api.InvitationTokenRequest) (api.InvitationPreview, error) {
	f.mu.Lock()
	f.inspects++
	f.mu.Unlock()
	if f.gate != nil {
		<-f.gate
	}
	return f.preview, f.resultErr
}
func (f *accountTestAPI) VerifyInvitationEmail(context.Context, api.InvitationTokenRequest) error {
	f.proofs++
	return f.resultErr
}
func (f *accountTestAPI) AcceptInvitation(_ context.Context, r api.InvitationAcceptRequest) (api.InvitationAcceptResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.accepts = append(f.accepts, r)
	if f.resultErr == nil && r.Password != "" {
		f.password = r.Password
	}
	return api.InvitationAcceptResponse{User: f.user}, f.resultErr
}
func (f *accountTestAPI) RequestRecovery(_ context.Context, r api.RecoveryRequest) error {
	f.recoveries = append(f.recoveries, r)
	return f.resultErr
}
func (f *accountTestAPI) ResetPassword(_ context.Context, r api.RecoveryResetRequest) error {
	f.resets = append(f.resets, r)
	return f.resultErr
}
func (f *accountTestAPI) ChangePassword(_ context.Context, r api.PasswordChangeRequest) error {
	if f.gate != nil {
		<-f.gate
	}
	f.changes = append(f.changes, r)
	return f.resultErr
}
func (f *accountTestAPI) RequestAdminEmail(context.Context, api.AdminEmailRequest) error {
	return f.resultErr
}
func (f *accountTestAPI) AdminHandover(_ context.Context, r api.AdminHandoverRequest) error {
	f.handovers = append(f.handovers, r)
	return f.resultErr
}

func accountHarness(t *testing.T, kind string, signedIn bool) (*harness, *accountTestAPI) {
	f := &accountTestAPI{fakeAPI: newFake(kind)}
	f.preview = api.InvitationPreview{Email: f.user.Email, DivisionName: "Mediatek", PeriodLabel: "2026/2027"}
	saved := ""
	if signedIn {
		saved = "saved"
		f.valid[saved] = true
	}
	h := newHarness(t, f.fakeAPI, saved)
	h.app.api = f
	h.app.Start()
	h.settle()
	return h, f
}

func TestParseAccountLinksWithoutEchoingSecrets(t *testing.T) {
	for _, raw := range []string{"#/undangan?token=invite", "https://app.himasif.id/#/undangan?token=invite&verification=email-proof", "#/pemulihan?token=recovery", "#/organisasi/serah-terima-admin?verification=proof"} {
		if _, err := parseAccountLink(raw); err != nil {
			t.Errorf("valid account link rejected: %v", err)
		}
	}
	for _, raw := range []string{"https://example.test/?token=SECRET#/undangan?token=secret", "javascript:SECRET", "#/undangan?token=SECRET&token=other", "#/undangan?token=SECRET&extra=yes", "#/pemulihan?verification=SECRET", "#/undangan?token=%0ASECRET", "#/other?token=SECRET", "https://user:SECRET@example.test/#/undangan?token=secret"} {
		_, err := parseAccountLink(raw)
		if err == nil {
			t.Error("unsafe link accepted")
		} else if strings.Contains(err.Error(), "SECRET") {
			t.Error("error exposed link secret")
		}
	}
}

func TestNewInvitationRequiresProofThenReallyLogsIn(t *testing.T) {
	h, f := accountHarness(t, api.AccountPerson, false)
	if err := h.app.HandleAccountLink("#/undangan?token=invite"); err != nil {
		t.Fatal(err)
	}
	h.app.inspectInvitation()
	h.settle()
	h.app.account.name = "Ani"
	h.app.account.password, h.app.account.confirm = "new-password", "new-password"
	h.app.acceptInvitation()
	if len(f.accepts) != 0 {
		t.Fatal("new account accepted without email proof")
	}
	h.app.verifyInvitation()
	h.settle()
	if f.proofs != 1 {
		t.Fatal("verification endpoint not called")
	}
	if err := h.app.HandleAccountLink("#/undangan?token=invite&verification=proof"); err != nil {
		t.Fatal(err)
	}
	h.app.inspectInvitation()
	h.settle()
	h.app.account.name = "Ani"
	h.app.account.password, h.app.account.confirm = "new-password", "new-password"
	h.app.acceptInvitation()
	h.settle()
	if len(f.accepts) != 1 || f.accepts[0].VerificationToken != "proof" || f.accepts[0].DisplayName != "Ani" {
		t.Fatal("new-account proof/body not submitted")
	}
	if len(f.logins) != 1 || h.app.screen != screenShell || h.tokens.get() == "" {
		t.Fatal("acceptance was not followed by real authenticated login")
	}
	if h.app.account.token != "" || h.app.account.verification != "" || h.app.account.password != "" {
		t.Fatal("invitation credentials retained after acceptance")
	}
}

func TestExistingInvitationResumesAndSendsTokenOnly(t *testing.T) {
	h, f := accountHarness(t, api.AccountPerson, false)
	f.preview.AccountExists = true
	_ = h.app.HandleAccountLink("#/undangan?token=invite&verification=unused-proof")
	h.app.inspectInvitation()
	h.settle()
	h.app.invitationLogin()
	h.settle()
	if !h.app.account.awaitingLogin || h.app.email != f.user.Email {
		t.Fatal("invitation not kept for matching login")
	}
	h.app.password = f.password
	h.app.login()
	h.settle()
	// enterShell is wired to resumeAccountFlow by the owner of app.go.
	if h.app.account.awaitingLogin {
		h.app.resumeAccountFlow()
	}
	h.app.acceptInvitation()
	h.settle()
	if len(f.accepts) != 1 {
		t.Fatal("existing account did not accept")
	}
	r := f.accepts[0]
	if r.Token != "invite" || r.Password != "" || r.DisplayName != "" || r.VerificationToken != "" {
		t.Fatal("existing-account request contained registration credentials")
	}
}

func TestRecoverySuccessForgetsSessionAndProof(t *testing.T) {
	h, f := accountHarness(t, api.AccountPerson, true)
	_ = h.app.HandleAccountLink("#/pemulihan?token=recovery")
	h.app.account.password, h.app.account.confirm = "new-password", "new-password"
	h.app.resetAccountPassword()
	h.settle()
	if len(f.resets) != 1 || h.app.screen != screenLogin || h.tokens.get() != "" || h.app.user.ID != (id.ID{}) {
		t.Fatal("reset did not discard stale signed-in session")
	}
	if h.app.account.token != "" || h.app.account.password != "" {
		t.Fatal("reset proof/password survived success")
	}
}

func TestRecoveryResponseDoesNotRevealRegistration(t *testing.T) {
	h, f := accountHarness(t, api.AccountPerson, false)
	h.app.openRecovery()
	h.app.account.email = "unknown@example.test"
	h.app.requestRecovery()
	h.settle()
	unknown := h.app.account.message
	h.app.account.email = f.user.Email
	h.app.requestRecovery()
	h.settle()
	if unknown == "" || h.app.account.message != unknown || len(f.recoveries) != 2 {
		t.Fatal("recovery response leaks registration or was not requested")
	}
}

func TestAccountReplacementDropsLateCallback(t *testing.T) {
	h, f := accountHarness(t, api.AccountPerson, false)
	f.gate = make(chan struct{})
	_ = h.app.HandleAccountLink("#/undangan?token=old-secret")
	h.app.inspectInvitation()
	h.app.openRecovery()
	h.app.account.email = "new@example.test"
	close(f.gate)
	h.settle()
	if h.app.account.mode != "recovery" || h.app.account.preview != nil || h.app.account.token != "" || h.app.account.email != "new@example.test" {
		t.Fatal("late invitation response contaminated replacement flow")
	}
}

func TestCredentialResultCannotFollowAnotherSession(t *testing.T) {
	h, f := accountHarness(t, api.AccountPerson, true)
	f.gate = make(chan struct{})
	h.app.account.currentPassword = "old-password"
	h.app.account.newPassword, h.app.account.confirmPassword = "new-password", "new-password"
	h.app.changeAccountPassword()
	h.app.clearAccountFlow()
	h.app.toLogin("")
	h.app.account.message = "Different session"
	close(f.gate)
	h.settle()
	if h.app.account.message != "Different session" || h.app.account.currentPassword != "" || h.app.account.newPassword != "" {
		t.Fatal("credential callback affected another session")
	}
}

func TestRejectedAccountOperationDoesNotPretendSuccess(t *testing.T) {
	h, f := accountHarness(t, api.AccountPerson, true)
	f.resultErr = &client.Error{Status: 401, Code: api.CodeUnauthenticated, Message: "Kata sandi saat ini salah."}
	h.app.account.currentPassword = "wrong"
	h.app.account.newPassword, h.app.account.confirmPassword = "new-password", "new-password"
	h.app.changeAccountPassword()
	h.settle()
	if h.app.account.err != "Kata sandi saat ini salah." || h.app.account.message != "" || h.app.account.currentPassword != "" || h.app.account.newPassword != "" {
		t.Fatal("failed change claimed success or retained passwords")
	}
}

func TestAdminHandoverNeedsProofAndEndsCurrentSession(t *testing.T) {
	h, f := accountHarness(t, api.AccountOrganizationAdmin, true)
	h.app.account.custodian = "Pengurus baru"
	h.app.account.currentPassword = "old-password"
	h.app.account.newPassword, h.app.account.confirmPassword = "new-password", "new-password"
	h.app.account.adminEmail = "new@example.test"
	if h.app.prepareAdminHandover() {
		t.Fatal("email change accepted without proof")
	}
	h.app.account.verification = "email-proof"
	if !h.app.prepareAdminHandover() {
		t.Fatal("valid handover cannot be reviewed")
	}
	h.app.submitAdminHandover()
	h.settle()
	if len(f.handovers) != 1 || f.handovers[0].Custodian != "Pengurus baru" || f.handovers[0].VerificationToken != "email-proof" {
		t.Fatal("handover request missing reauth/proof/custodian")
	}
	if h.app.screen != screenLogin || h.tokens.get() != "" || h.app.account.verification != "" {
		t.Fatal("handover kept current admin session/proof")
	}
}

func TestIncomingInvitationWaitsForCoreLogin(t *testing.T) {
	h, f := accountHarness(t, api.AccountPerson, false)
	h.app.busy = true
	if err := h.app.HandleAccountLink("#/undangan?token=invite"); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	f.mu.Lock()
	calls := f.inspects
	f.mu.Unlock()
	if calls != 0 || h.app.account.busy || h.app.inflight != 0 || h.tt.HasText("Undangan ke Space") {
		t.Fatal("account link activated while core login was still pending")
	}
	if !h.app.account.active || h.app.account.token != "invite" {
		t.Fatal("pending link was discarded instead of waiting for login")
	}
	h.app.busy = false
	h.tt.Frame()
	h.settle()
	f.mu.Lock()
	calls = f.inspects
	f.mu.Unlock()
	if calls != 1 || h.app.account.preview == nil || !h.tt.HasText("Undangan ke Space") {
		t.Fatal("invitation did not inspect and render after core login finished")
	}
}
