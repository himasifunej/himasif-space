package client

import (
	"context"
	"net/http"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func periodPath(p id.ID) string { return "/api/periods/" + p.String() }

func (c *Client) CreatePeriod(ctx context.Context, req api.CreatePeriodRequest) (api.Period, error) {
	var out api.Period
	err := c.do(ctx, http.MethodPost, "/api/periods", req, &out)
	return out, err
}

func (c *Client) ActivatePeriod(ctx context.Context, p id.ID) (api.Period, error) {
	var out api.Period
	err := c.do(ctx, http.MethodPost, periodPath(p)+"/activate", nil, &out)
	return out, err
}

func (c *Client) ClosePeriod(ctx context.Context, p id.ID) (api.Period, error) {
	var out api.Period
	err := c.do(ctx, http.MethodPost, periodPath(p)+"/close", nil, &out)
	return out, err
}

func (c *Client) SetKadiv(ctx context.Context, p, d id.ID, req api.SetKadivRequest) (api.Member, error) {
	var out api.Member
	err := c.do(ctx, http.MethodPut, divisionPath(p, d)+"/kadiv", req, &out)
	return out, err
}

// Users is the organization admin's directory for choosing personal accounts.
// A normal division member uses Members rather than this endpoint.
func (c *Client) Users(ctx context.Context) ([]api.User, error) {
	var out api.List[api.User]
	err := c.do(ctx, http.MethodGet, "/api/users", nil, &out)
	return out.Items, err
}

func (c *Client) GetHandover(ctx context.Context, p id.ID) (api.Handover, error) {
	var out api.Handover
	err := c.do(ctx, http.MethodGet, periodPath(p)+"/handover", nil, &out)
	return out, err
}

func (c *Client) SaveHandover(ctx context.Context, p id.ID, req api.HandoverRequest) (api.Handover, error) {
	var out api.Handover
	err := c.do(ctx, http.MethodPut, periodPath(p)+"/handover", req, &out)
	return out, err
}

func (c *Client) ArchiveGrants(ctx context.Context, p, d id.ID) ([]api.ArchiveGrant, error) {
	var out api.List[api.ArchiveGrant]
	err := c.do(ctx, http.MethodGet, divisionPath(p, d)+"/archives", nil, &out)
	return out.Items, err
}

func (c *Client) GrantArchive(ctx context.Context, p, d id.ID, req api.ArchiveRequest) (api.ArchiveGrant, error) {
	var out api.ArchiveGrant
	err := c.do(ctx, http.MethodPost, divisionPath(p, d)+"/archives", req, &out)
	return out, err
}

func (c *Client) RevokeArchive(ctx context.Context, p, d, grant id.ID) error {
	return c.do(ctx, http.MethodDelete, divisionPath(p, d)+"/archives/"+grant.String(), nil, nil)
}

// ArchiveTargets returns eligible archive metadata for the granting scope;
// it neither opens the target Space nor grants access to its contents.
func (c *Client) ArchiveTargets(ctx context.Context, p, d id.ID) ([]api.ArchiveTarget, error) {
	var out api.List[api.ArchiveTarget]
	err := c.do(ctx, http.MethodGet, divisionPath(p, d)+"/archives/targets", nil, &out)
	return out.Items, err
}
