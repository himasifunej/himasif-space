package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

func TestTaskClientProjectionAndVersionContract(t *testing.T) {
	sid, tid, uid := id.New(), id.New(), id.New()
	date := timeutil.Date{Year: 2026, Month: 10, Day: 30}
	seen := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen++
		if r.Header.Get("Authorization") != "Bearer task-session" {
			t.Error("missing session")
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /api/spaces/" + sid.String() + "/tasks":
			if r.URL.Query().Get("assigneeId") != uid.String() || r.URL.Query().Get("dueBefore") != "2026-10-30" || r.URL.Query().Get("progress") != api.TaskDoing {
				t.Error("projection filters lost")
			}
			writeJSON(w, 200, api.List[api.Task]{Items: []api.Task{{ID: tid, Version: 7}}})
		case "POST /api/tasks/" + tid.String() + "/move":
			var req api.TaskMoveRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			if req.ExpectedVersion != 7 || req.Progress != api.TaskDoing || req.BeforeID != sid {
				t.Error("move identity/order/version lost")
			}
			writeJSON(w, 200, api.Task{ID: tid, Version: 8})
		case "POST /api/spaces/" + sid.String() + "/tasks":
			var raw map[string]json.RawMessage
			_ = json.NewDecoder(r.Body).Decode(&raw)
			if string(raw["deadlineDate"]) != "\"2026-10-30\"" || raw["deadlineAt"] != nil {
				t.Error("date-only contract changed")
			}
			writeJSON(w, 201, api.Task{ID: tid})
		default:
			t.Error("unexpected path", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c := New(server.URL)
	c.SetToken("task-session")
	got, err := c.SpaceTasks(context.Background(), sid, api.TaskFilter{AssigneeID: uid, Progress: api.TaskDoing, DueBefore: &date})
	if err != nil || len(got) != 1 || got[0].ID != tid {
		t.Fatal("projection", err)
	}
	moved, err := c.MoveTask(context.Background(), tid, api.TaskMoveRequest{Progress: api.TaskDoing, ExpectedVersion: 7, BeforeID: sid})
	if err != nil || moved.Version != 8 {
		t.Fatal("move", err)
	}
	if _, err = c.CreateTask(context.Background(), sid, api.TaskWriteRequest{DeadlineDate: &date}); err != nil {
		t.Fatal(err)
	}
	if seen != 3 {
		t.Fatal("requests missing")
	}
}
