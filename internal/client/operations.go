package client

import (
	"context"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func (c *Client) SystemOperations(ctx context.Context) (api.SystemOperations, error) {
	var out api.SystemOperations
	err := c.do(ctx, "GET", "/api/operations", nil, &out)
	return out, err
}
func (c *Client) DivisionOperations(ctx context.Context, pid, did id.ID) (api.DivisionOperations, error) {
	var out api.DivisionOperations
	err := c.do(ctx, "GET", divisionPath(pid, did)+"/operations", nil, &out)
	return out, err
}
