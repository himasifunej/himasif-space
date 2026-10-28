package client

import (
	"context"
	"net/http"

	"github.com/himasifunej/himasif-space/internal/api"
)

// AccountSession snapshots the bearer on the UI thread before asynchronous
// work begins. Switching accounts cannot redirect a queued credential operation
// onto the next account's session. The HTTP transport remains shared.
func (c *Client) AccountSession() *Client {
	return &Client{BaseURL: c.BaseURL, HTTP: c.HTTP, Token: c.token()}
}

// InspectInvitation selects the new-account or existing-account invitation flow.
func (c *Client) InspectInvitation(ctx context.Context, req api.InvitationTokenRequest) (api.InvitationPreview, error) {
	var out api.InvitationPreview
	err := c.do(ctx, http.MethodPost, "/api/auth/invitations/inspect", req, &out)
	return out, err
}

// VerifyInvitationEmail asks for the separate proof sent to the invited email.
func (c *Client) VerifyInvitationEmail(ctx context.Context, req api.InvitationTokenRequest) error {
	return c.do(ctx, http.MethodPost, "/api/auth/invitations/verify", req, nil)
}

// AcceptInvitation creates a verified account or binds the signed-in account.
// It does not create or change the client's session.
func (c *Client) AcceptInvitation(ctx context.Context, req api.InvitationAcceptRequest) (api.InvitationAcceptResponse, error) {
	var out api.InvitationAcceptResponse
	err := c.do(ctx, http.MethodPost, "/api/auth/invitations/accept", req, &out)
	return out, err
}

// RequestRecovery has the same successful response for known and unknown emails.
func (c *Client) RequestRecovery(ctx context.Context, req api.RecoveryRequest) error {
	return c.do(ctx, http.MethodPost, "/api/auth/recovery", req, nil)
}

// ResetPassword consumes a recovery proof and revokes every session of its account.
func (c *Client) ResetPassword(ctx context.Context, req api.RecoveryResetRequest) error {
	return c.do(ctx, http.MethodPost, "/api/auth/recovery/reset", req, nil)
}

// ChangePassword keeps this session and revokes the account's other sessions.
func (c *Client) ChangePassword(ctx context.Context, req api.PasswordChangeRequest) error {
	return c.do(ctx, http.MethodPost, "/api/auth/password", req, nil)
}

// RequestAdminEmail sends a proof to the organization's new recovery email.
func (c *Client) RequestAdminEmail(ctx context.Context, req api.AdminEmailRequest) error {
	return c.do(ctx, http.MethodPost, "/api/auth/admin-email", req, nil)
}

// AdminHandover changes custody of the shared admin and ends every session.
func (c *Client) AdminHandover(ctx context.Context, req api.AdminHandoverRequest) error {
	return c.do(ctx, http.MethodPost, "/api/auth/admin-handover", req, nil)
}
