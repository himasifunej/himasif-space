package client

import (
	"context"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func waPath(cid id.ID) string { return "/api/whatsapp/" + cid.String() }
func (c *Client) WAConnections(ctx context.Context, pid, did id.ID) ([]api.WAConnection, error) {
	var out api.List[api.WAConnection]
	err := c.do(ctx, "GET", divisionPath(pid, did)+"/whatsapp", nil, &out)
	return out.Items, err
}
func (c *Client) CreateWAConnection(ctx context.Context, pid, did id.ID, req api.WAConnectionRequest) (api.WAConnection, error) {
	var out api.WAConnection
	err := c.do(ctx, "POST", divisionPath(pid, did)+"/whatsapp", req, &out)
	return out, err
}
func (c *Client) UpdateWAConnection(ctx context.Context, cid id.ID, req api.WAConnectionRequest) (api.WAConnection, error) {
	var out api.WAConnection
	err := c.do(ctx, "PUT", waPath(cid), req, &out)
	return out, err
}
func (c *Client) WAState(ctx context.Context, cid id.ID) (api.WASessionState, error) {
	var out api.WASessionState
	err := c.do(ctx, "GET", waPath(cid)+"/state", nil, &out)
	return out, err
}
func (c *Client) WAAction(ctx context.Context, cid id.ID, req api.WAActionRequest) error {
	return c.do(ctx, "POST", waPath(cid)+"/action", req, nil)
}
func (c *Client) WAAvailableGroups(ctx context.Context, cid id.ID) ([]api.WAAvailableGroup, error) {
	var out api.List[api.WAAvailableGroup]
	err := c.do(ctx, "GET", waPath(cid)+"/available-groups", nil, &out)
	return out.Items, err
}
func (c *Client) SaveWAGroup(ctx context.Context, cid id.ID, req api.WAGroupRequest) error {
	return c.do(ctx, "PUT", waPath(cid)+"/groups", req, nil)
}
func (c *Client) CreateWALink(ctx context.Context, pid, did id.ID) (api.WALinkChallenge, error) {
	var out api.WALinkChallenge
	err := c.do(ctx, "POST", divisionPath(pid, did)+"/whatsapp-link", nil, &out)
	return out, err
}
func (c *Client) WACommands(ctx context.Context, cid id.ID) ([]api.WACustomCommand, error) {
	var out []api.WACustomCommand
	err := c.do(ctx, "GET", waPath(cid)+"/commands", nil, &out)
	return out, err
}
func (c *Client) SaveWACommand(ctx context.Context, cid, commandID id.ID, req api.WACustomCommandRequest) (api.WACustomCommand, error) {
	var out api.WACustomCommand
	err := c.do(ctx, "PUT", waPath(cid)+"/commands/"+commandID.String(), req, &out)
	return out, err
}
func (c *Client) DeleteWACommand(ctx context.Context, cid, commandID id.ID, version int64) error {
	return c.do(ctx, "DELETE", waPath(cid)+"/commands/"+commandID.String(), map[string]int64{"expectedVersion": version}, nil)
}
func (c *Client) WAOutbox(ctx context.Context, cid id.ID) ([]api.WAOutboxItem, error) {
	var out []api.WAOutboxItem
	err := c.do(ctx, "GET", waPath(cid)+"/outbox", nil, &out)
	return out, err
}

// PreviewWA renders a draft of the bot's texts as the bot would send them,
// with example data (Tahap 7d). It stores and sends nothing.
func (c *Client) PreviewWA(ctx context.Context, pid, did id.ID, req api.WAPreviewRequest) (api.WAPreview, error) {
	var out api.WAPreview
	err := c.do(ctx, "POST", divisionPath(pid, did)+"/whatsapp/preview", req, &out)
	return out, err
}
