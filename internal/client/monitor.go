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
