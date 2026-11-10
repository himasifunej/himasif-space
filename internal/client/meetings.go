package client

import (
	"context"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"net/http"
)

func (c *Client) SpaceMeetings(ctx context.Context, sid id.ID) ([]api.Meeting, error) {
	var out api.List[api.Meeting]
	err := c.do(ctx, http.MethodGet, "/api/spaces/"+sid.String()+"/meetings", nil, &out)
	return out.Items, err
}
func (c *Client) GetMeeting(ctx context.Context, mid id.ID) (api.Meeting, error) {
	var out api.Meeting
	err := c.do(ctx, http.MethodGet, "/api/meetings/"+mid.String(), nil, &out)
	return out, err
}
func (c *Client) CreateMeeting(ctx context.Context, sid id.ID, req api.MeetingRequest) (api.Meeting, error) {
	var out api.Meeting
	err := c.do(ctx, http.MethodPost, "/api/spaces/"+sid.String()+"/meetings", req, &out)
	return out, err
}
func (c *Client) UpdateMeeting(ctx context.Context, mid id.ID, req api.MeetingRequest) (api.Meeting, error) {
	var out api.Meeting
	err := c.do(ctx, http.MethodPut, "/api/meetings/"+mid.String(), req, &out)
	return out, err
}
func (c *Client) RecordAttendance(ctx context.Context, mid, uid id.ID, req api.AttendanceRequest) (api.Meeting, error) {
	var out api.Meeting
	err := c.do(ctx, http.MethodPut, "/api/meetings/"+mid.String()+"/attendance/"+uid.String(), req, &out)
	return out, err
}
func (c *Client) MeetingCandidates(ctx context.Context, sid id.ID) ([]api.PersonRef, error) {
	var out api.List[api.PersonRef]
	err := c.do(ctx, http.MethodGet, "/api/spaces/"+sid.String()+"/meeting-candidates", nil, &out)
	return out.Items, err
}

// MeetingTimer runs one action of the shared meeting timer (stage T2).
func (c *Client) MeetingTimer(ctx context.Context, mid id.ID, req api.MeetingTimerRequest) (api.Meeting, error) {
	var out api.Meeting
	err := c.do(ctx, http.MethodPost, "/api/meetings/"+mid.String()+"/timer", req, &out)
	return out, err
}
