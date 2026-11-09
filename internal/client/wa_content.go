package client

import (
	"context"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/broadcast"
	"github.com/himasifunej/himasif-space/internal/id"
)

func waContentPath(sid id.ID) string { return "/api/spaces/" + sid.String() }
func (c *Client) WAParserPolicy(ctx context.Context, sid id.ID) (api.WAParserPolicy, error) {
	var out api.WAParserPolicy
	err := c.do(ctx, "GET", waContentPath(sid)+"/wa-parser-policy", nil, &out)
	return out, err
}
func (c *Client) SetWAParserPolicy(ctx context.Context, sid id.ID, req api.WAParserPolicy) (api.WAParserPolicy, error) {
	var out api.WAParserPolicy
	err := c.do(ctx, "PUT", waContentPath(sid)+"/wa-parser-policy", req, &out)
	return out, err
}
func (c *Client) PreviewWABroadcast(ctx context.Context, sid id.ID, body string) ([]broadcast.Item, error) {
	var out api.List[broadcast.Item]
	err := c.do(ctx, "POST", waContentPath(sid)+"/wa-broadcast-preview", map[string]string{"body": body}, &out)
	return out.Items, err
}
func (c *Client) WAContentInbox(ctx context.Context, sid id.ID) ([]api.WAContentItem, error) {
	var out api.List[api.WAContentItem]
	err := c.do(ctx, "GET", waContentPath(sid)+"/wa-content-inbox", nil, &out)
	return out.Items, err
}
func (c *Client) DecideWAContent(ctx context.Context, sid, iid id.ID, req api.WAContentDecision) (api.WAContentItem, error) {
	var out api.WAContentItem
	err := c.do(ctx, "POST", waContentPath(sid)+"/wa-content-inbox/"+iid.String()+"/decision", req, &out)
	return out, err
}
func (c *Client) WAContentHistory(ctx context.Context, sid, iid id.ID) ([]api.WAContentReview, error) {
	var out api.List[api.WAContentReview]
	err := c.do(ctx, "GET", waContentPath(sid)+"/wa-content-inbox/"+iid.String()+"/history", nil, &out)
	return out.Items, err
}
