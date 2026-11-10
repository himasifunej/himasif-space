package client

import (
	"context"
	"net/http"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Onboarding returns the caller's onboarding progress; periodID, when not
// Nil, is the period whose WhatsApp link counts.
func (c *Client) Onboarding(ctx context.Context, periodID id.ID) (api.Onboarding, error) {
	var out api.Onboarding
	path := "/api/me/onboarding"
	if periodID != id.Nil {
		path += "?periodId=" + periodID.String()
	}
	err := c.do(ctx, http.MethodGet, path, nil, &out)
	return out, err
}

// MarkOnboarding records or clears one onboarding mark.
func (c *Client) MarkOnboarding(ctx context.Context, m api.OnboardingMark) (api.Onboarding, error) {
	var out api.Onboarding
	err := c.do(ctx, http.MethodPost, "/api/me/onboarding", m, &out)
	return out, err
}

// UpdateProfile changes the caller's display name.
func (c *Client) UpdateProfile(ctx context.Context, p api.ProfileUpdate) (api.User, error) {
	var out api.User
	err := c.do(ctx, http.MethodPut, "/api/me/profile", p, &out)
	return out, err
}

// Guides returns the in-app guide the server has.
func (c *Client) Guides(ctx context.Context) (api.GuideList, error) {
	var out api.GuideList
	err := c.do(ctx, http.MethodGet, "/api/guides", nil, &out)
	return out, err
}
