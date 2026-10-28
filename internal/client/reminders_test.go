package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// The server rejects unknown fields. In particular, creating or editing a
// reminder must not send sharing/recipient/role fields from a displayed DTO.
func TestReminderClientWritesUseStrictBodiesAndKeepDateSemantics(t *testing.T) {
	rid, recipient := id.New(), id.New()
	date := timeutil.Date{Year: 2026, Month: time.November, Day: 10}
	at := time.Date(2026, time.November, 11, 3, 30, 0, 0, time.UTC)
	ctx := context.Background()
	tests := []struct {
		name, method, path, body string
		status                   int
		call                     func(*Client) error
	}{
		{"create date", "POST", "/api/reminders", `{"title":"Rapat","notes":"Bawa catatan","dueKind":"date","dueOn":"2026-11-10","timeZone":"Asia/Jakarta"}`, 201,
			func(c *Client) error {
				r, err := c.CreateReminder(ctx, api.ReminderRequest{Title: "Rapat", Notes: "Bawa catatan", DueKind: api.DueDate, DueOn: &date, TimeZone: "Asia/Jakarta"})
				if err == nil && (r.ID != rid || r.DueOn == nil || *r.DueOn != date || r.DueAt != nil || r.Shared) {
					t.Error("create lost date-only or private-default response")
				}
				return err
			}},
		{"replace with instant", "PUT", "/api/reminders/" + rid.String(), `{"title":"Rapat","notes":"","dueKind":"instant","dueAt":"2026-11-11T03:30:00Z","timeZone":"Asia/Jakarta"}`, 200,
			func(c *Client) error {
				_, err := c.UpdateReminder(ctx, rid, api.ReminderRequest{Title: "Rapat", DueKind: api.DueInstant, DueAt: &at, TimeZone: "Asia/Jakarta"})
				return err
			}},
		{"complete", "POST", "/api/reminders/" + rid.String() + "/complete", `{"completed":true}`, 200,
			func(c *Client) error {
				_, err := c.CompleteReminder(ctx, rid, api.ReminderCompleteRequest{Completed: true})
				return err
			}},
		{"reopen", "POST", "/api/reminders/" + rid.String() + "/complete", `{"completed":false}`, 200,
			func(c *Client) error {
				_, err := c.CompleteReminder(ctx, rid, api.ReminderCompleteRequest{})
				return err
			}},
		{"sharing off", "PUT", "/api/reminders/" + rid.String() + "/sharing", `{"shared":false}`, 200,
			func(c *Client) error {
				_, err := c.SetReminderSharing(ctx, rid, api.ReminderSharingRequest{})
				return err
			}},
		{"explicit recipient", "POST", "/api/reminders/" + rid.String() + "/recipients", `{"recipientId":"` + recipient.String() + `"}`, 201,
			func(c *Client) error {
				_, err := c.AddReminderRecipient(ctx, rid, api.ReminderShareRequest{RecipientID: recipient})
				return err
			}},
		{"revoke recipient", "DELETE", "/api/reminders/" + rid.String() + "/recipients/" + recipient.String(), "", 204,
			func(c *Client) error { return c.RemoveReminderRecipient(ctx, rid, recipient) }},
		{"private default", "PUT", "/api/reminders/preferences", `{"defaultShare":false}`, 200,
			func(c *Client) error {
				pref, err := c.SetReminderPreferences(ctx, api.ReminderPreferences{})
				if err == nil && pref.DefaultShare {
					t.Error("private default changed")
				}
				return err
			}},
		{"delete", "DELETE", "/api/reminders/" + rid.String(), "", 204,
			func(c *Client) error { return c.DeleteReminder(ctx, rid) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method || r.URL.RequestURI() != tt.path || r.Header.Get("Authorization") != "Bearer personal-session" {
					t.Errorf("request = %s %s, authorization present=%v", r.Method, r.URL.RequestURI(), r.Header.Get("Authorization") != "")
				}
				got, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if tt.body == "" {
					if len(got) != 0 {
						t.Errorf("body for DELETE = %s", got)
					}
				} else {
					var actual, expected any
					if err := json.Unmarshal(got, &actual); err != nil {
						t.Error(err)
					}
					if err := json.Unmarshal([]byte(tt.body), &expected); err != nil {
						t.Error(err)
					}
					if !reflect.DeepEqual(actual, expected) {
						t.Errorf("body = %s; want %s", got, tt.body)
					}
				}
				if tt.status == http.StatusNoContent {
					w.WriteHeader(tt.status)
					return
				}
				if tt.path == "/api/reminders/preferences" {
					writeJSON(w, tt.status, api.ReminderPreferences{})
					return
				}
				writeJSON(w, tt.status, api.Reminder{ID: rid, DueKind: api.DueDate, DueOn: &date, TimeZone: "Asia/Jakarta", Relation: api.ReminderOwner})
			}))
			defer server.Close()
			c := New(server.URL)
			c.SetToken("personal-session")
			if err := tt.call(c); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReminderClientReadsPreserveAccessRelationAndDormantRecipients(t *testing.T) {
	rid, recipient := id.New(), id.New()
	at := time.Date(2026, time.November, 11, 3, 30, 0, 0, time.UTC)
	readOnly := api.Reminder{ID: rid, DueKind: api.DueInstant, DueAt: &at, TimeZone: "Asia/Jakarta", Relation: api.ReminderRecipient}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		switch r.URL.RequestURI() {
		case "/api/reminders":
			writeJSON(w, 200, api.List[api.Reminder]{Items: []api.Reminder{readOnly}})
		case "/api/reminders/" + rid.String():
			writeJSON(w, 200, readOnly)
		case "/api/reminders/" + rid.String() + "/recipients":
			writeJSON(w, 200, api.List[api.ReminderShare]{Items: []api.ReminderShare{{RecipientID: recipient, DisplayName: "Rekan"}}})
		case "/api/reminders/" + rid.String() + "/history":
			writeJSON(w, 200, api.List[api.ReminderAuditEntry]{Items: []api.ReminderAuditEntry{{Action: "reminder.sharing", OccurredAt: at, After: map[string]any{"shared": false}}}})
		case "/api/reminders/preferences":
			writeJSON(w, 200, api.ReminderPreferences{DefaultShare: true})
		default:
			t.Errorf("unexpected path: %s", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c, ctx := New(server.URL), context.Background()
	items, err := c.Reminders(ctx)
	if err != nil || len(items) != 1 || items[0].Relation != api.ReminderRecipient || items[0].Recipients != nil {
		t.Fatalf("list: %+v, %v", items, err)
	}
	r, err := c.GetReminder(ctx, rid)
	if err != nil || r.DueAt == nil || !r.DueAt.Equal(at) || r.DueOn != nil || r.Relation != api.ReminderRecipient {
		t.Fatalf("get lost instant/access: %+v, %v", r, err)
	}
	shares, err := c.ReminderRecipients(ctx, rid)
	if err != nil || len(shares) != 1 || shares[0].RecipientID != recipient || shares[0].DisplayName != "Rekan" {
		t.Fatalf("recipients: %+v, %v", shares, err)
	}
	history, err := c.ReminderHistory(ctx, rid)
	if err != nil || len(history) != 1 || history[0].Action != "reminder.sharing" || history[0].After.(map[string]any)["shared"] != false {
		t.Fatalf("history: %+v, %v", history, err)
	}
	pref, err := c.ReminderPreferences(ctx)
	if err != nil || !pref.DefaultShare {
		t.Fatalf("preferences: %+v, %v", pref, err)
	}
}

func TestReminderClientDoesNotConvertHiddenRecordIntoSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":{"code":"not_found","message":"Reminder tidak ditemukan."}}`)
	}))
	defer server.Close()
	r, err := New(server.URL).GetReminder(context.Background(), id.New())
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Status != 404 || apiErr.Message != "Reminder tidak ditemukan." || r.ID != id.Nil {
		t.Fatalf("hidden reminder response: %+v, %v", r, err)
	}
}
