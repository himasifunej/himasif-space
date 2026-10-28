package client

import (
	"context"
	"net/http"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func divisionPath(p, d id.ID) string {
	return "/api/periods/" + p.String() + "/divisions/" + d.String()
}
func (c *Client) Spaces(ctx context.Context, p id.ID) ([]api.Space, error) {
	var r api.List[api.Space]
	err := c.do(ctx, http.MethodGet, "/api/periods/"+p.String()+"/spaces", nil, &r)
	return r.Items, err
}
func (c *Client) DivisionAccess(ctx context.Context, p, d id.ID) (api.Access, error) {
	var r api.Access
	err := c.do(ctx, http.MethodGet, divisionPath(p, d)+"/access", nil, &r)
	return r, err
}
func (c *Client) Members(ctx context.Context, p, d id.ID) ([]api.Member, error) {
	var r api.List[api.Member]
	err := c.do(ctx, http.MethodGet, divisionPath(p, d)+"/members", nil, &r)
	return r.Items, err
}
func (c *Client) Invitations(ctx context.Context, p, d id.ID) ([]api.Invitation, error) {
	var r api.List[api.Invitation]
	err := c.do(ctx, http.MethodGet, divisionPath(p, d)+"/invitations", nil, &r)
	return r.Items, err
}
func (c *Client) Invite(ctx context.Context, p, d id.ID, req api.InviteRequest) (api.Invitation, error) {
	var r api.Invitation
	err := c.do(ctx, http.MethodPost, divisionPath(p, d)+"/invitations", req, &r)
	return r, err
}
func (c *Client) ResendInvitation(ctx context.Context, p, d, i id.ID) (api.Invitation, error) {
	var r api.Invitation
	err := c.do(ctx, http.MethodPost, divisionPath(p, d)+"/invitations/"+i.String()+"/resend", nil, &r)
	return r, err
}
func (c *Client) RevokeInvitation(ctx context.Context, p, d, i id.ID) error {
	return c.do(ctx, http.MethodDelete, divisionPath(p, d)+"/invitations/"+i.String(), nil, nil)
}
func (c *Client) UpdateMember(ctx context.Context, p, d, u id.ID, req api.UpdateMemberRequest) (api.Member, error) {
	var r api.Member
	err := c.do(ctx, http.MethodPatch, divisionPath(p, d)+"/members/"+u.String(), req, &r)
	return r, err
}
func (c *Client) RemoveMember(ctx context.Context, p, d, u id.ID) error {
	return c.do(ctx, http.MethodDelete, divisionPath(p, d)+"/members/"+u.String(), nil, nil)
}
func (c *Client) Roles(ctx context.Context, p, d id.ID) ([]api.CustomRole, error) {
	var r api.List[api.CustomRole]
	err := c.do(ctx, http.MethodGet, divisionPath(p, d)+"/roles", nil, &r)
	return r.Items, err
}
func (c *Client) CreateRole(ctx context.Context, p, d id.ID, req api.RoleRequest) (api.CustomRole, error) {
	var r api.CustomRole
	err := c.do(ctx, http.MethodPost, divisionPath(p, d)+"/roles", req, &r)
	return r, err
}
func (c *Client) UpdateRole(ctx context.Context, p, d, rid id.ID, req api.RoleRequest) (api.CustomRole, error) {
	var r api.CustomRole
	err := c.do(ctx, http.MethodPut, divisionPath(p, d)+"/roles/"+rid.String(), req, &r)
	return r, err
}
func (c *Client) DeleteRole(ctx context.Context, p, d, rid id.ID) error {
	return c.do(ctx, http.MethodDelete, divisionPath(p, d)+"/roles/"+rid.String(), nil, nil)
}
func (c *Client) Appoint45(ctx context.Context, p, d id.ID, req api.Appoint45Request) (api.Appointment45, error) {
	var r api.Appointment45
	err := c.do(ctx, http.MethodPut, divisionPath(p, d)+"/45", req, &r)
	return r, err
}
func (c *Client) Remove45(ctx context.Context, p, d id.ID) error {
	return c.do(ctx, http.MethodDelete, divisionPath(p, d)+"/45", nil, nil)
}
