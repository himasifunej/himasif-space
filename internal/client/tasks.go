package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func taskPath(t id.ID) string { return "/api/tasks/" + t.String() }
func taskFilterPath(path string, f api.TaskFilter) string {
	q := url.Values{}
	if f.AssigneeID != id.Nil {
		q.Set("assigneeId", f.AssigneeID.String())
	}
	if f.Progress != "" {
		q.Set("progress", f.Progress)
	}
	if f.DueBefore != nil {
		q.Set("dueBefore", f.DueBefore.String())
	}
	if len(q) > 0 {
		return path + "?" + q.Encode()
	}
	return path
}
func (c *Client) SpaceTasks(ctx context.Context, s id.ID, f api.TaskFilter) ([]api.Task, error) {
	var r api.List[api.Task]
	err := c.do(ctx, http.MethodGet, taskFilterPath("/api/spaces/"+s.String()+"/tasks", f), nil, &r)
	return r.Items, err
}
func (c *Client) MyTasks(ctx context.Context, p id.ID, f api.TaskFilter) ([]api.Task, error) {
	var r api.List[api.Task]
	err := c.do(ctx, http.MethodGet, taskFilterPath("/api/periods/"+p.String()+"/tasks/mine", f), nil, &r)
	return r.Items, err
}
func (c *Client) GetTask(ctx context.Context, t id.ID) (api.Task, error) {
	var r api.Task
	err := c.do(ctx, http.MethodGet, taskPath(t), nil, &r)
	return r, err
}
func (c *Client) CreateTask(ctx context.Context, s id.ID, req api.TaskWriteRequest) (api.Task, error) {
	var r api.Task
	err := c.do(ctx, http.MethodPost, "/api/spaces/"+s.String()+"/tasks", req, &r)
	return r, err
}
func (c *Client) UpdateTask(ctx context.Context, t id.ID, req api.TaskWriteRequest) (api.Task, error) {
	var r api.Task
	err := c.do(ctx, http.MethodPatch, taskPath(t), req, &r)
	return r, err
}
func (c *Client) MoveTask(ctx context.Context, t id.ID, req api.TaskMoveRequest) (api.Task, error) {
	var r api.Task
	err := c.do(ctx, http.MethodPost, taskPath(t)+"/move", req, &r)
	return r, err
}
func (c *Client) AddTaskComment(ctx context.Context, t id.ID, req api.TaskCommentRequest) (api.TaskComment, error) {
	var r api.TaskComment
	err := c.do(ctx, http.MethodPost, taskPath(t)+"/comments", req, &r)
	return r, err
}
func (c *Client) RequestTaskHelp(ctx context.Context, t id.ID, req api.TaskHelpCreateRequest) (api.TaskHelpRequest, error) {
	var r api.TaskHelpRequest
	err := c.do(ctx, http.MethodPost, taskPath(t)+"/help", req, &r)
	return r, err
}
func (c *Client) DecideTaskHelp(ctx context.Context, t, h id.ID, req api.TaskHelpDecisionRequest) (api.TaskHelpRequest, error) {
	var r api.TaskHelpRequest
	err := c.do(ctx, http.MethodPost, taskPath(t)+"/help/"+h.String()+"/decide", req, &r)
	return r, err
}
func (c *Client) SubmitTaskEvidence(ctx context.Context, t id.ID, req api.TaskEvidenceRequest) (api.TaskEvidence, error) {
	var r api.TaskEvidence
	err := c.do(ctx, http.MethodPost, taskPath(t)+"/evidence", req, &r)
	return r, err
}
func (c *Client) ReviewTaskEvidence(ctx context.Context, t, e id.ID, req api.TaskEvidenceReviewRequest) (api.TaskEvidence, error) {
	var r api.TaskEvidence
	err := c.do(ctx, http.MethodPost, taskPath(t)+"/evidence/"+e.String()+"/review", req, &r)
	return r, err
}
func (c *Client) TaskHistory(ctx context.Context, t id.ID) ([]api.TaskEvent, error) {
	var r api.List[api.TaskEvent]
	err := c.do(ctx, http.MethodGet, taskPath(t)+"/history", nil, &r)
	return r.Items, err
}
func (c *Client) Notifications(ctx context.Context) ([]api.Notification, error) {
	var r api.List[api.Notification]
	err := c.do(ctx, http.MethodGet, "/api/notifications", nil, &r)
	return r.Items, err
}
func (c *Client) ReadNotification(ctx context.Context, n id.ID) error {
	return c.do(ctx, http.MethodPost, "/api/notifications/"+n.String()+"/read", nil, nil)
}
