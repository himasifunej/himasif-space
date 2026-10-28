package client

import (
	"context"
	"net/http"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func reminderPath(r id.ID) string { return "/api/reminders/" + r.String() }

// Reminders returns the caller's reminders and those explicitly shared with
// their account. Each record retains its owner/recipient relation.
func (c *Client) Reminders(ctx context.Context) ([]api.Reminder, error) {
	var r api.List[api.Reminder]
	err := c.do(ctx, http.MethodGet, "/api/reminders", nil, &r)
	return r.Items, err
}

func (c *Client) GetReminder(ctx context.Context, rid id.ID) (api.Reminder, error) {
	var r api.Reminder
	err := c.do(ctx, http.MethodGet, reminderPath(rid), nil, &r)
	return r, err
}

func (c *Client) CreateReminder(ctx context.Context, req api.ReminderRequest) (api.Reminder, error) {
	var r api.Reminder
	err := c.do(ctx, http.MethodPost, "/api/reminders", req, &r)
	return r, err
}

func (c *Client) UpdateReminder(ctx context.Context, rid id.ID, req api.ReminderRequest) (api.Reminder, error) {
	var r api.Reminder
	err := c.do(ctx, http.MethodPut, reminderPath(rid), req, &r)
	return r, err
}

func (c *Client) DeleteReminder(ctx context.Context, rid id.ID) error {
	return c.do(ctx, http.MethodDelete, reminderPath(rid), nil, nil)
}

func (c *Client) CompleteReminder(ctx context.Context, rid id.ID, req api.ReminderCompleteRequest) (api.Reminder, error) {
	var r api.Reminder
	err := c.do(ctx, http.MethodPost, reminderPath(rid)+"/complete", req, &r)
	return r, err
}

func (c *Client) SetReminderSharing(ctx context.Context, rid id.ID, req api.ReminderSharingRequest) (api.Reminder, error) {
	var r api.Reminder
	err := c.do(ctx, http.MethodPut, reminderPath(rid)+"/sharing", req, &r)
	return r, err
}

// ReminderRecipients includes saved recipients even while sharing is off.
// The normal reminder DTO omits that list when the reminder is private.
func (c *Client) ReminderRecipients(ctx context.Context, rid id.ID) ([]api.ReminderShare, error) {
	var r api.List[api.ReminderShare]
	err := c.do(ctx, http.MethodGet, reminderPath(rid)+"/recipients", nil, &r)
	return r.Items, err
}

func (c *Client) AddReminderRecipient(ctx context.Context, rid id.ID, req api.ReminderShareRequest) (api.Reminder, error) {
	var r api.Reminder
	err := c.do(ctx, http.MethodPost, reminderPath(rid)+"/recipients", req, &r)
	return r, err
}

func (c *Client) RemoveReminderRecipient(ctx context.Context, rid, recipientID id.ID) error {
	return c.do(ctx, http.MethodDelete, reminderPath(rid)+"/recipients/"+recipientID.String(), nil, nil)
}

func (c *Client) ReminderHistory(ctx context.Context, rid id.ID) ([]api.ReminderAuditEntry, error) {
	var r api.List[api.ReminderAuditEntry]
	err := c.do(ctx, http.MethodGet, reminderPath(rid)+"/history", nil, &r)
	return r.Items, err
}

func (c *Client) ReminderPreferences(ctx context.Context) (api.ReminderPreferences, error) {
	var r api.ReminderPreferences
	err := c.do(ctx, http.MethodGet, "/api/reminders/preferences", nil, &r)
	return r, err
}

func (c *Client) SetReminderPreferences(ctx context.Context, req api.ReminderPreferences) (api.ReminderPreferences, error) {
	var r api.ReminderPreferences
	err := c.do(ctx, http.MethodPut, "/api/reminders/preferences", req, &r)
	return r, err
}
