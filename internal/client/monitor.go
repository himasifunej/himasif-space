package client

import (
	"context"
	"net/http"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// MonitorGrants returns the BPH members of a period and who of them holds
// org.monitor (organization admin only).
func (c *Client) MonitorGrants(ctx context.Context, p id.ID) (api.MonitorGrants, error) {
	var out api.MonitorGrants
	err := c.do(ctx, http.MethodGet, periodPath(p)+"/monitors", nil, &out)
	return out, err
}

// GrantMonitor gives org.monitor to a BPH member for the period.
func (c *Client) GrantMonitor(ctx context.Context, p id.ID, req api.MonitorGrantRequest) (api.MonitorGrant, error) {
	var out api.MonitorGrant
	err := c.do(ctx, http.MethodPost, periodPath(p)+"/monitors", req, &out)
	return out, err
}

// RevokeMonitor ends a grant of org.monitor.
func (c *Client) RevokeMonitor(ctx context.Context, p, grant id.ID) error {
	return c.do(ctx, http.MethodDelete, periodPath(p)+"/monitors/"+grant.String(), nil, nil)
}

// MonitorAudit returns the monitoring log of a period, newest first.
func (c *Client) MonitorAudit(ctx context.Context, p id.ID) ([]api.MonitorAuditEvent, error) {
	var out api.List[api.MonitorAuditEvent]
	err := c.do(ctx, http.MethodGet, periodPath(p)+"/monitors/audit", nil, &out)
	return out.Items, err
}

// MonitorOverview returns the monitoring dashboard of a period (holders of
// org.monitor only).
func (c *Client) MonitorOverview(ctx context.Context, p id.ID) (api.MonitorOverview, error) {
	var out api.MonitorOverview
	err := c.do(ctx, http.MethodGet, periodPath(p)+"/monitor", nil, &out)
	return out, err
}

// SpaceView returns one space with the caller's access to it.
func (c *Client) SpaceView(ctx context.Context, sid id.ID) (api.Space, error) {
	var out api.Space
	err := c.do(ctx, http.MethodGet, "/api/spaces/"+sid.String(), nil, &out)
	return out, err
}

// MonitorMeetings returns the cross-division meeting timeline of a period
// (holders of org.monitor only), the server's default range.
func (c *Client) MonitorMeetings(ctx context.Context, p id.ID) ([]api.MonitorMeeting, error) {
	var out api.List[api.MonitorMeeting]
	err := c.do(ctx, http.MethodGet, periodPath(p)+"/monitor/meetings", nil, &out)
	return out.Items, err
}

// MonitorDigest returns the caller's weekly digest settings and a preview.
func (c *Client) MonitorDigest(ctx context.Context) (api.MonitorDigestSettings, error) {
	var out api.MonitorDigestSettings
	err := c.do(ctx, http.MethodGet, "/api/me/monitor-digest", nil, &out)
	return out, err
}

// SaveMonitorDigest turns the weekly digest on or off and picks its channel.
func (c *Client) SaveMonitorDigest(ctx context.Context, req api.MonitorDigestRequest) (api.MonitorDigestSettings, error) {
	var out api.MonitorDigestSettings
	err := c.do(ctx, http.MethodPut, "/api/me/monitor-digest", req, &out)
	return out, err
}
