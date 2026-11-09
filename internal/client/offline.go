package client

import (
	"context"
	"github.com/himasifunej/himasif-space/internal/api"
	"net/http"
)

func (c *Client) SyncOffline(ctx context.Context, req api.OfflineMutation) (api.OfflineResult, error) {
	var out api.OfflineResult
	err := c.do(ctx, http.MethodPost, "/api/sync", req, &out)
	return out, err
}
