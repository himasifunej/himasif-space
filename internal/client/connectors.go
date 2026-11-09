package client

import (
	"context"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"net/http"
	"time"
)

func (c *Client) Connectors(ctx context.Context, pid, did id.ID) ([]api.Connector, error) {
	var out api.List[api.Connector]
	err := c.do(ctx, http.MethodGet, divisionPath(pid, did)+"/connectors", nil, &out)
	return out.Items, err
}
func (c *Client) CreateConnector(ctx context.Context, pid, did id.ID, req api.ConnectorRequest) (api.Connector, error) {
	var out api.Connector
	err := c.do(ctx, http.MethodPost, divisionPath(pid, did)+"/connectors", req, &out)
	return out, err
}
func (c *Client) UpdateConnector(ctx context.Context, cid id.ID, req api.ConnectorRequest) (api.Connector, error) {
	var out api.Connector
	err := c.do(ctx, http.MethodPut, "/api/connectors/"+cid.String(), req, &out)
	return out, err
}
func (c *Client) FetchConnector(ctx context.Context, cid id.ID) (api.ConnectorRun, error) {
	c = c.AccountSession()
	httpCopy := http.Client{}
	if c.HTTP != nil {
		httpCopy = *c.HTTP
	}
	httpCopy.Timeout = 60 * time.Second
	c.HTTP = &httpCopy
	var out api.ConnectorRun
	err := c.do(ctx, http.MethodPost, "/api/connectors/"+cid.String()+"/fetch", nil, &out)
	return out, err
}
func (c *Client) ConnectorItems(ctx context.Context, cid id.ID) ([]api.ConnectorItem, error) {
	var out api.List[api.ConnectorItem]
	err := c.do(ctx, http.MethodGet, "/api/connectors/"+cid.String()+"/items", nil, &out)
	return out.Items, err
}
func (c *Client) ConnectorRuns(ctx context.Context, cid id.ID) ([]api.ConnectorRun, error) {
	var out api.List[api.ConnectorRun]
	err := c.do(ctx, http.MethodGet, "/api/connectors/"+cid.String()+"/runs", nil, &out)
	return out.Items, err
}
func (c *Client) DecideConnectorItem(ctx context.Context, cid, iid id.ID, req api.ConnectorDecision) (api.MedhumContent, error) {
	var out api.MedhumContent
	err := c.do(ctx, http.MethodPost, "/api/connectors/"+cid.String()+"/items/"+iid.String()+"/decision", req, &out)
	return out, err
}
func (c *Client) SetConnectorPerson(ctx context.Context, cid id.ID, req api.ConnectorPersonMapping) error {
	return c.do(ctx, http.MethodPut, "/api/connectors/"+cid.String()+"/people", req, nil)
}
func (c *Client) ConnectorEvents(ctx context.Context, cid id.ID) ([]api.ConnectorEvent, error) {
	var out api.List[api.ConnectorEvent]
	err := c.do(ctx, http.MethodGet, "/api/connectors/"+cid.String()+"/events", nil, &out)
	return out.Items, err
}
func (c *Client) MirrorConnector(ctx context.Context, cid id.ID) error {
	c = c.AccountSession()
	httpCopy := http.Client{}
	if c.HTTP != nil {
		httpCopy = *c.HTTP
	}
	httpCopy.Timeout = 60 * time.Second
	c.HTTP = &httpCopy
	return c.do(ctx, http.MethodPost, "/api/connectors/"+cid.String()+"/mirror", nil, nil)
}
