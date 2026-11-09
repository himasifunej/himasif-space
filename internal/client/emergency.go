package client

import (
	"context"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func (c *Client) CreateEmergencySnapshot(ctx context.Context, sid id.ID, req api.EmergencySnapshotRequest) (api.EmergencySnapshot, error) {
	var out api.EmergencySnapshot
	err := c.do(ctx, "POST", "/api/spaces/"+sid.String()+"/emergency-snapshots", req, &out)
	return out, err
}
func (c *Client) EmergencySnapshotCSV(ctx context.Context, snapshot id.ID) ([]byte, error) {
	return c.download(ctx, "/api/emergency-snapshots/"+snapshot.String()+"/csv", "text/csv", 32<<20)
}
func (c *Client) PreviewEmergency(ctx context.Context, changes []api.EmergencyChange) ([]api.EmergencyReconcileResult, error) {
	var out api.List[api.EmergencyReconcileResult]
	err := c.doAudit(ctx, "POST", "/api/emergency-reconciliation/preview", api.EmergencyReconcileRequest{Changes: changes}, &out)
	return out.Items, err
}
func (c *Client) ApplyEmergency(ctx context.Context, req api.EmergencyReconcileRequest) ([]api.EmergencyReconcileResult, error) {
	var out api.List[api.EmergencyReconcileResult]
	err := c.doAudit(ctx, "POST", "/api/emergency-reconciliation/apply", req, &out)
	return out.Items, err
}

func (c *Client) ResolveEmergency(ctx context.Context, req api.EmergencyReconcileRequest) ([]api.EmergencyReconcileResult, error) {
	var out api.List[api.EmergencyReconcileResult]
	err := c.doAudit(ctx, "POST", "/api/emergency-reconciliation/resolve", req, &out)
	return out.Items, err
}
