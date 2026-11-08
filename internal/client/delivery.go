package client

import (
	"context"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func (c *Client) Deliveries(ctx context.Context) ([]api.Delivery, error) {
	var out api.List[api.Delivery]
	err := c.do(ctx, http.MethodGet, "/api/deliveries", nil, &out)
	return out.Items, err
}
func (c *Client) RespondDelivery(ctx context.Context, did id.ID, req api.DeliveryResponse) error {
	return c.do(ctx, http.MethodPost, "/api/deliveries/"+did.String()+"/response", req, nil)
}
func (c *Client) SendPing(ctx context.Context, sid id.ID, req api.PingRequest) error {
	return c.do(ctx, http.MethodPost, "/api/spaces/"+sid.String()+"/pings", req, nil)
}
func (c *Client) DivisionNotificationPolicy(ctx context.Context, pid, did id.ID) (api.NotificationPolicy, error) {
	var out api.NotificationPolicy
	err := c.do(ctx, http.MethodGet, divisionPath(pid, did)+"/notification-policy", nil, &out)
	return out, err
}
func (c *Client) SetDivisionNotificationPolicy(ctx context.Context, pid, did id.ID, req api.NotificationPolicy) (api.NotificationPolicy, error) {
	var out api.NotificationPolicy
	err := c.do(ctx, http.MethodPut, divisionPath(pid, did)+"/notification-policy", req, &out)
	return out, err
}
func (c *Client) TaskNotificationPolicy(ctx context.Context, tid id.ID) (api.NotificationPolicy, error) {
	var out api.NotificationPolicy
	err := c.do(ctx, http.MethodGet, "/api/tasks/"+tid.String()+"/notification-policy", nil, &out)
	return out, err
}
func (c *Client) SetTaskNotificationPolicy(ctx context.Context, tid id.ID, req api.NotificationPolicy) (api.NotificationPolicy, error) {
	var out api.NotificationPolicy
	err := c.do(ctx, http.MethodPut, "/api/tasks/"+tid.String()+"/notification-policy", req, &out)
	return out, err
}
func (c *Client) ResetTaskNotificationPolicy(ctx context.Context, tid id.ID) error {
	return c.do(ctx, http.MethodDelete, "/api/tasks/"+tid.String()+"/notification-policy", nil, nil)
}

// Pings lists a Space's sent pings, newest first; before (zero for the
// newest) pages back; filter is "", "unseen" or "failed" (Tahap 7c).
func (c *Client) Pings(ctx context.Context, sid id.ID, before time.Time, limit int, filter string) ([]api.PingRecord, error) {
	q := url.Values{}
	if filter != "" {
		q.Set("filter", filter)
	}
	if !before.IsZero() {
		q.Set("before", before.UTC().Format(time.RFC3339Nano))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	path := "/api/spaces/" + sid.String() + "/pings"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var out api.List[api.PingRecord]
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	return out.Items, err
}
