package appui

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"github.com/himasifunej/himasif-space/internal/workspaceui"
)

type taskTestAPI struct {
	*fakeAPI
	TasksAPI
	WorkspaceAPI
	listErr   error
	items     []api.Task
	listCalls int
	listGate  chan struct{}
	moveGate  chan struct{}
	moves     chan api.TaskMoveRequest
	moveErr   error
}

func (f *taskTestAPI) MyTasks(context.Context, id.ID, api.TaskFilter) ([]api.Task, error) {
	f.listCalls++
	if f.listGate != nil {
		<-f.listGate
	}
	return slices.Clone(f.items), f.listErr
}
func (f *taskTestAPI) SpaceTasks(context.Context, id.ID, api.TaskFilter) ([]api.Task, error) {
	return f.MyTasks(context.Background(), id.Nil, api.TaskFilter{})
}
func (f *taskTestAPI) MoveTask(_ context.Context, _ id.ID, r api.TaskMoveRequest) (api.Task, error) {
	f.moves <- r
	if f.moveGate != nil {
		<-f.moveGate
	}
	if f.moveErr != nil {
		return api.Task{}, f.moveErr
	}
	f.items[0].Progress = r.Progress
	f.items[0].Version++
	return f.items[0], nil
}
func taskHarness(t *testing.T) (*harness, *taskTestAPI) {
	f := &taskTestAPI{fakeAPI: newFake(api.AccountPerson), moves: make(chan api.TaskMoveRequest, 4)}
	h := newHarness(t, f.fakeAPI, "")
	h.app.api = f
	h.app.screen = screenShell
	h.app.user = f.user
	h.app.conn = connOnline
	h.app.periodID = f.periods[1].ID
	h.app.periods = f.periods
	deadline := timeutil.Date{Year: 2026, Month: 10, Day: 30}
	f.items = []api.Task{{ID: id.New(), Title: "Poster", PeriodID: h.app.periodID, Version: 1, Progress: api.TaskTodo, MatchStatus: api.MatchNone, Assignee: api.PersonRef{ID: f.user.ID, DisplayName: f.user.DisplayName}, DeadlineDate: &deadline, Permissions: api.TaskPermissions{Progress: true}}}
	h.app.ensureTasks(true)
	h.settle()
	return h, f
}
func TestTaskMoveWaitsForServerAndConflictDoesNotPretendSaved(t *testing.T) {
	h, f := taskHarness(t)
	tid := f.items[0].ID
	f.moveGate = make(chan struct{})
	f.moveErr = &client.Error{Status: 409, Code: api.CodeConflict, Message: "Versi berbeda"}
	h.app.moveTask(workspaceui.MoveRequest{TaskID: tid, Destination: workspaceui.Doing})
	select {
	case r := <-f.moves:
		if r.ExpectedVersion != 1 {
			t.Fatal("move did not carry version")
		}
	case <-time.After(time.Second):
		t.Fatal("move missing")
	}
	if !h.app.tasks.busy || h.app.tasks.items[0].Progress != api.TaskTodo || h.app.tasks.notice != "" {
		t.Fatal("unconfirmed move changed snapshot or claimed saved")
	}
	close(f.moveGate)
	h.settle()
	if h.app.tasks.busy || h.app.tasks.items[0].Progress != api.TaskTodo || h.app.tasks.err == "" {
		t.Fatal("conflict overwrote snapshot or disappeared")
	}
}
func TestTaskMoveSnapshotAndSelectionRetainCanonicalID(t *testing.T) {
	h, f := taskHarness(t)
	tid := f.items[0].ID
	h.app.workspace.Open(tid)
	h.app.moveTask(workspaceui.MoveRequest{TaskID: tid, Destination: workspaceui.Doing})
	h.settle()
	if h.app.tasks.items[0].Progress != api.TaskDoing || h.app.tasks.items[0].Version != 2 || h.app.workspace.SelectedID() != tid {
		t.Fatal("saved move lost canonical identity or inspector")
	}
}
func TestTaskOldScopeResponseCannotPopulateAnotherPeriod(t *testing.T) {
	h, f := taskHarness(t)
	gate := make(chan struct{})
	f.listGate = gate
	h.app.loadTasks()
	h.app.tasksContextChanged()
	h.app.periodID = id.New()
	close(gate)
	// Do not render the new scope: it would correctly start its own request.
	for h.app.inflight > 0 {
		select {
		case apply := <-h.queue:
			apply()
		case <-time.After(time.Second):
			t.Fatal("request did not settle")
		}
	}
	if len(h.app.tasks.items) != 0 || h.app.workspace.HasSnapshot() || h.app.tasks.loaded {
		t.Fatal("old scope leaked into current workspace")
	}
}
func TestTaskSubmissionCannotBeBypassedByDraggingToDone(t *testing.T) {
	h, f := taskHarness(t)
	tid := f.items[0].ID
	h.app.moveTask(workspaceui.MoveRequest{TaskID: tid, Destination: workspaceui.Done})
	h.app.moveTask(workspaceui.MoveRequest{TaskID: tid, Destination: workspaceui.Review})
	if len(f.moves) != 0 || h.app.workspace.SelectedID() != tid || h.app.tasks.notice == "" {
		t.Fatal("drag bypassed evidence submission/review")
	}
	h.app.tasks.items[0].Permissions = api.TaskPermissions{}
	h.app.moveTask(workspaceui.MoveRequest{TaskID: tid, Destination: workspaceui.Doing})
	if len(f.moves) != 0 {
		t.Fatal("read-only user sent move")
	}
}
func TestTaskCalendarDateRemainsDateOnlyAndExplicitClockUsesPeriodZone(t *testing.T) {
	d, at, err := parseTaskDate("2026-10-30", "", "Asia/Jakarta", true)
	if err != nil || d == nil || at != nil || d.String() != "2026-10-30" {
		t.Fatal("date-only gained invented time")
	}
	d, at, err = parseTaskDate("2026-10-30", "16:30", "Asia/Jakarta", true)
	if err != nil || d != nil || at == nil || at.Format(time.RFC3339) != "2026-10-30T09:30:00Z" {
		t.Fatal("explicit deadline converted incorrectly")
	}
	for _, bad := range [][2]string{{"", "10:00"}, {"2026-02-30", ""}, {"2026-10-30", "25:00"}} {
		if _, _, e := parseTaskDate(bad[0], bad[1], "Asia/Jakarta", true); e == nil {
			t.Fatal("invalid deadline accepted")
		}
	}
}
func TestUnavailableTaskServiceNeverBecomesConfirmedEmptyBoard(t *testing.T) {
	h := newHarness(t, newFake(api.AccountPerson), "")
	h.app.periodID = id.New()
	h.app.ensureTasks(true)
	h.app.ensureTasks(true)
	if h.app.tasks.loaded || h.app.workspace.HasSnapshot() || h.app.tasks.err == "" {
		t.Fatal("missing task service became confirmed empty data")
	}
}

func TestTaskCancelledCanBeInspectedAndACompletionDoesNotEraseAnotherDraft(t *testing.T) {
	h, f := taskHarness(t)
	f.items[0].Progress = api.TaskCancelled
	h.app.tasks.items = f.items
	h.app.tasks.showCancelled = true
	h.app.projectTasks()
	h.app.workspace.Open(f.items[0].ID)
	if h.app.workspace.SelectedID() != f.items[0].ID {
		t.Fatal("cancelled task not inspectable")
	}
	h.app.workspace.Close()
	// Completion of an operation for task A must not clear inputs for task B.
	gate := make(chan struct{})
	h.app.tasks.comment = "A"
	h.app.tasks.inspectorSeq = 1
	h.app.mutateTask("Komentar", func(TasksAPI) error { <-gate; return nil }, func() { h.app.tasks.comment = "" })
	h.app.tasks.inspectorSeq++
	h.app.tasks.comment = "draft B"
	close(gate)
	h.settle()
	if h.app.tasks.comment != "draft B" {
		t.Fatal("older task completion erased the new inspector draft")
	}
}

// The real creation form must expose its primary action after scrolling, even
// with both large text fields and a shared-space option in a small window.
func TestTaskCreateSaveRemainsReachable(t *testing.T) {
	h, f := taskHarness(t)
	s := testAuthorizedSpace(h.app, f.divisions[0])
	h.app.access.spaces[0].Roles[0].Capabilities = append(h.app.access.spaces[0].Roles[0].Capabilities, api.CapTasksManage)
	shared := api.Space{ID: id.New(), PeriodID: h.app.periodID, Name: "Medhum", Kind: api.SpaceShared, Divisions: []api.Division{f.divisions[0]}}
	h.app.access.spaces = append(h.app.access.spaces, shared)
	h.app.tasks = tasksUIState{scope: s.ID, loaded: true, draft: &taskDraft{owner: f.divisions[0].ID, shared: map[id.ID]bool{}}}
	h.app.chrome.tab = "Board tugas"
	h.app.router.Replace(PathSpaces)
	for _, size := range [][2]int{{1440, 900}, {880, 640}} {
		h.tt.SetSize(size[0], size[1])
		h.tt.Advance(400 * time.Millisecond)
		h.tt.Frame()
		h.tt.Scroll(float32(size[0]-15), float32(size[1]-180), 0, 10000)
		h.tt.Advance(time.Second)
		h.tt.Frame()
		r, ok := h.tt.Find("Simpan tugas")
		if !ok || r.W <= 0 || r.H <= 0 || r.Y < 0 || r.Y+r.H > float32(size[1]-28) {
			t.Fatalf("%dx%d save action clipped after full scroll: bounds=%+v found=%v", size[0], size[1], r, ok)
		}
	}
}

func (f *taskTestAPI) Spaces(context.Context, id.ID) ([]api.Space, error) { return nil, nil }
func TestMissingTasksEndpointDoesNotRetryThroughSpaceRefresh(t *testing.T) {
	h, f := taskHarness(t)
	f.listErr = &client.Error{Status: 404, Code: api.CodeNotFound, Message: "Layanan tugas tidak ditemukan."}
	h.app.loadTasks()
	for i := 0; i < 5; i++ {
		h.settle()
	}
	if f.listCalls != 2 || h.app.tasks.loaded || h.app.workspace.HasSnapshot() || h.app.tasks.err == "" {
		t.Fatalf("missing endpoint retried or became confirmed empty: calls=%d loaded=%v snapshot=%v err=%q", f.listCalls, h.app.tasks.loaded, h.app.workspace.HasSnapshot(), h.app.tasks.err)
	}
}
func TestOpeningCanonicalTaskClearsTheActiveProjectionFilter(t *testing.T) {
	h, f := taskHarness(t)
	s := testAuthorizedSpace(h.app, f.divisions[0])
	f.items[0].SpaceIDs = []id.ID{s.ID}
	h.app.tasks = tasksUIState{scope: s.ID, loaded: true, items: slices.Clone(f.items), filter: "no matching task"}
	h.app.projectTasks()
	if !h.app.openCanonicalTask(f.items[0]) || h.app.tasks.filter != "" || h.app.workspace.SelectedID() != f.items[0].ID {
		t.Fatal("navigation retained a filter hiding its target task")
	}
}
