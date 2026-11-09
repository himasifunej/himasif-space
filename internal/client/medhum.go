package client

import (
	"context"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"net/http"
)

func (c *Client) MedhumContents(ctx context.Context, sid id.ID) ([]api.MedhumContent, error) {
	var out api.List[api.MedhumContent]
	err := c.do(ctx, http.MethodGet, "/api/spaces/"+sid.String()+"/contents", nil, &out)
	return out.Items, err
}
func (c *Client) GetMedhum(ctx context.Context, cid id.ID) (api.MedhumContent, error) {
	var out api.MedhumContent
	err := c.do(ctx, http.MethodGet, "/api/contents/"+cid.String(), nil, &out)
	return out, err
}
func (c *Client) CreateMedhum(ctx context.Context, sid id.ID, req api.MedhumCreateRequest) (api.MedhumContent, error) {
	var out api.MedhumContent
	err := c.do(ctx, http.MethodPost, "/api/spaces/"+sid.String()+"/contents", req, &out)
	return out, err
}
func (c *Client) UpdateMedhum(ctx context.Context, cid id.ID, req api.MedhumUpdateRequest) (api.MedhumContent, error) {
	var out api.MedhumContent
	err := c.do(ctx, http.MethodPatch, "/api/contents/"+cid.String(), req, &out)
	return out, err
}
func (c *Client) MedhumPolicy(ctx context.Context, sid id.ID) (api.MedhumPolicy, error) {
	var out api.MedhumPolicy
	err := c.do(ctx, http.MethodGet, "/api/spaces/"+sid.String()+"/medhum-policy", nil, &out)
	return out, err
}
func (c *Client) SetMedhumPolicy(ctx context.Context, sid id.ID, req api.MedhumPolicyRequest) (api.MedhumPolicy, error) {
	var out api.MedhumPolicy
	err := c.do(ctx, http.MethodPatch, "/api/spaces/"+sid.String()+"/medhum-policy", req, &out)
	return out, err
}
