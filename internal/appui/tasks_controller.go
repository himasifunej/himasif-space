package appui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"github.com/himasifunej/himasif-space/internal/workspaceui"
)

// TasksAPI persists all task intents. No projection owns a second task copy.
type TasksAPI interface {
	SpaceTasks(context.Context, id.ID, api.TaskFilter) ([]api.Task, error)
	MyTasks(context.Context, id.ID, api.TaskFilter) ([]api.Task, error)
	GetTask(context.Context, id.ID) (api.Task, error)
	CreateTask(context.Context, id.ID, api.TaskWriteRequest) (api.Task, error)
	UpdateTask(context.Context, id.ID, api.TaskWriteRequest) (api.Task, error)
	MoveTask(context.Context, id.ID, api.TaskMoveRequest) (api.Task, error)
	AddTaskComment(context.Context, id.ID, api.TaskCommentRequest) (api.TaskComment, error)
	RequestTaskHelp(context.Context, id.ID, api.TaskHelpCreateRequest) (api.TaskHelpRequest, error)
	DecideTaskHelp(context.Context, id.ID, id.ID, api.TaskHelpDecisionRequest) (api.TaskHelpRequest, error)
	SubmitTaskEvidence(context.Context, id.ID, api.TaskEvidenceRequest) (api.TaskEvidence, error)
	ReviewTaskEvidence(context.Context, id.ID, id.ID, api.TaskEvidenceReviewRequest) (api.TaskEvidence, error)
	TaskHistory(context.Context, id.ID) ([]api.TaskEvent, error)
	Notifications(context.Context) ([]api.Notification, error)
	ReadNotification(context.Context, id.ID) error
}

var _ TasksAPI = (*client.Client)(nil)

type taskDraft struct {
	taskID, owner, assignee                                                                    id.ID
	version                                                                                    int64
	title, description, output, category, deadline, deadlineTime, posting, postingTime, reason string
	shared                                                                                     map[id.ID]bool
}
type tasksUIState struct {
	seq                                                      uint64
	scope                                                    id.ID
	personal, loaded, loading, busy                          bool
	items                                                    []api.Task
	err, notice                                              string
	draft                                                    *taskDraft
	members                                                  map[id.ID][]api.Member
	membersBusy                                              map[id.ID]bool
	memberErrors                                             map[id.ID]string
	ownerOpen, assigneeOpen, createOpen                      bool
	filter, comment, help, evidenceURL, evidenceNote, reason string
	inspected, pendingOpen                                   id.ID
	weekOffset                                               int
	showCancelled                                            bool
	inspectorSeq, historySeq                                 uint64
	history                                                  []api.TaskEvent
	historyLoaded, historyBusy                               bool
	historyErr                                               string
	notifications                                            []api.Notification
	notificationsLoaded, notificationsBusy                   bool
	notificationsErr                                         string
}

func (a *App) tasksContextChanged() {
	seq := a.tasks.seq + 1
	a.tasks = tasksUIState{seq: seq}
	a.workspace = workspaceui.New()
}
func (a *App) taskService() (TasksAPI, bool) {
	if c, ok := a.api.(*client.Client); ok {
		return c.AccountSession(), true
	}
	s, ok := a.api.(TasksAPI)
	return s, ok
}
func (a *App) ensureTasks(personal bool) {
	scope := a.selectedSpaceID()
	if personal {
		scope = a.periodID
	}
	if a.tasks.scope != scope || a.tasks.personal != personal {
		a.tasksContextChanged()
		a.tasks.scope = scope
		a.tasks.personal = personal
	}
	if scope != id.Nil && !a.tasks.loaded && !a.tasks.loading && a.tasks.err == "" {
		a.loadTasks()
	}
}
func (a *App) loadTasks() {
	service, ok := a.taskService()
	if !ok {
		a.tasks.err = "Layanan tugas belum tersedia."
		return
	}
	if a.tasks.scope == id.Nil || a.tasks.loading {
		return
	}
	gen, seq, scope, personal := a.gen, a.tasks.seq, a.tasks.scope, a.tasks.personal
	a.tasks.loading = true
	a.tasks.err = ""
	a.async(func() func() {
		var tasks []api.Task
		var err error
		if personal {
			tasks, err = service.MyTasks(context.Background(), scope, api.TaskFilter{})
		} else {
			tasks, err = service.SpaceTasks(context.Background(), scope, api.TaskFilter{})
		}
		return func() {
			if gen != a.gen || seq != a.tasks.seq || scope != a.tasks.scope {
				return
			}
			a.tasks.loading = false
			if err != nil {
				a.tasks.err = a.taskFailure(err)
				return
			}
			a.conn = connOnline
			a.tasks.items = tasks
			a.tasks.loaded = true
			a.projectTasks()
			if a.tasks.pendingOpen != id.Nil {
				a.workspace.Open(a.tasks.pendingOpen)
				a.tasks.pendingOpen = id.Nil
			}
		}
	})
}
func (a *App) taskFailure(err error) string {
	var e *client.Error
	if errors.As(err, &e) {
		if e.Status == http.StatusForbidden || e.Status == http.StatusNotFound {
			a.tasks.items = nil
			a.tasks.loaded = false
			a.tasks.draft = nil
			a.tasks.inspected = id.Nil
			a.workspace = workspaceui.New()
			if e.Status == http.StatusForbidden {
				a.loadSpaces()
			}
		}
		if e.Status == http.StatusConflict {
			return "Tugas sudah berubah. Muat ulang data sebelum melanjutkan; isianmu masih ada untuk diperiksa."
		}
	}
	return a.failed(err)
}
func taskLane(t api.Task) workspaceui.Lane {
	switch t.Progress {
	case api.TaskDoing:
		return workspaceui.Doing
	case api.TaskSubmitted:
		return workspaceui.Review
	case api.TaskDone:
		return workspaceui.Done
	case api.TaskCancelled:
		return workspaceui.Cancelled
	}
	return workspaceui.Todo
}
func laneProgress(l workspaceui.Lane) string {
	switch l {
	case workspaceui.Doing:
		return api.TaskDoing
	case workspaceui.Review:
		return api.TaskSubmitted
	case workspaceui.Done:
		return api.TaskDone
	case workspaceui.Cancelled:
		return api.TaskCancelled
	}
	return api.TaskTodo
}
func (a *App) taskByID(tid id.ID) *api.Task {
	for i := range a.tasks.items {
		if a.tasks.items[i].ID == tid {
			return &a.tasks.items[i]
		}
	}
	return nil
}
func taskDate(date *timeutil.Date, at *time.Time, zone string) string {
	if date != nil {
		return fmt.Sprintf("%d %s %d · tanpa jam", date.Day, monthName(date.Month), date.Year)
	}
	if at != nil {
		loc, err := timeutil.LoadZone(zone)
		if err != nil {
			loc = time.UTC
		}
		return at.In(loc).Format("02 Jan 2006 · 15:04 MST")
	}
	return "—"
}
func (a *App) taskSpaceNames(t api.Task) string {
	var names []string
	for _, sid := range t.SpaceIDs {
		for _, s := range a.access.spaces {
			if s.ID == sid {
				names = append(names, s.Name)
			}
		}
	}
	return strings.Join(names, " · ")
}
func (a *App) projectTasks() {
	if !a.tasks.loaded {
		a.workspace = workspaceui.New()
		return
	}
	tasks := make([]workspaceui.Task, 0, len(a.tasks.items))
	q := strings.ToLower(strings.TrimSpace(a.tasks.filter))
	for _, t := range a.tasks.items {
		if t.Progress == api.TaskCancelled && !a.tasks.showCancelled {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(t.Title+" "+t.Category+" "+t.Assignee.DisplayName), q) {
			continue
		}
		tasks = append(tasks, workspaceui.Task{ID: t.ID, Title: t.Title, Description: t.Description, ExpectedOutput: t.ExpectedOutput, Category: t.Category, Assignee: t.Assignee.DisplayName, Deadline: taskDate(t.DeadlineDate, t.DeadlineAt, t.TimeZone), Posting: taskDate(t.PostingDate, t.PostingAt, t.TimeZone), Space: a.taskSpaceNames(t), Lane: taskLane(t), ReadOnly: !t.Permissions.Manage && (!t.Permissions.Progress || t.Progress == api.TaskDone || t.Progress == api.TaskCancelled)})
	}
	_ = a.workspace.SetTasks(tasks)
	a.workspace.OnMove = a.moveTask
	a.workspace.Details = a.taskDetails
	if a.tasks.busy || a.tasks.loading || a.conn == connOffline {
		a.workspace.OnMove = nil
	}
}
func (a *App) taskWriteBlocked() bool {
	return a.tasks.busy || a.tasks.loading || a.conn == connOffline
}
func (a *App) mutateTask(action string, work func(TasksAPI) error, after func()) {
	service, ok := a.taskService()
	if !ok || a.taskWriteBlocked() {
		return
	}
	gen, seq := a.gen, a.tasks.seq
	inspectorSeq, draft := a.tasks.inspectorSeq, a.tasks.draft
	a.tasks.busy = true
	a.tasks.err = ""
	a.tasks.notice = ""
	a.projectTasks()
	a.async(func() func() {
		err := work(service)
		return func() {
			if gen != a.gen || seq != a.tasks.seq {
				return
			}
			a.tasks.busy = false
			if err != nil {
				a.tasks.err = a.taskFailure(err)
				a.projectTasks()
				return
			}
			a.conn = connOnline
			a.tasks.notice = action + " tersimpan di server."
			a.tasks.historyLoaded = false
			a.tasks.historySeq++
			a.tasks.historyBusy = false
			a.tasks.historyErr = ""
			if after != nil && inspectorSeq == a.tasks.inspectorSeq && draft == a.tasks.draft {
				after()
			}
			a.tasks.notificationsLoaded = false
			a.loadTasks()
		}
	})
}
func (a *App) moveTask(m workspaceui.MoveRequest) {
	t := a.taskByID(m.TaskID)
	if t == nil || (!t.Permissions.Progress && !t.Permissions.Manage) || (t.Progress == api.TaskDone || t.Progress == api.TaskCancelled) && !t.Permissions.Manage {
		return
	}
	if m.Destination == workspaceui.Review && t.Progress != api.TaskSubmitted {
		a.workspace.Open(t.ID)
		a.tasks.notice = "Tambahkan tautan atau catatan hasil di panel kanan untuk menyerahkan tugas."
		return
	}
	if m.Destination == workspaceui.Done && t.Progress != api.TaskDone {
		a.workspace.Open(t.ID)
		a.tasks.notice = "Selesaikan melalui pencocokan bukti pada panel kanan."
		return
	}
	reason := strings.TrimSpace(a.tasks.reason)
	if (t.Progress == api.TaskDone && m.Destination != workspaceui.Done || t.Progress == api.TaskCancelled && m.Destination != workspaceui.Cancelled) && reason == "" {
		a.workspace.Open(t.ID)
		a.tasks.notice = "Tulis alasan pada panel kanan sebelum membuka kembali tugas."
		return
	}
	req := api.TaskMoveRequest{Progress: laneProgress(m.Destination), BeforeID: m.BeforeID, ExpectedVersion: t.Version, Reason: reason}
	tid := t.ID
	a.mutateTask("Perpindahan tugas", func(s TasksAPI) error { _, err := s.MoveTask(context.Background(), tid, req); return err }, nil)
}
func (a *App) manageableDivisions() []api.Division {
	s := a.selectedSpace()
	if s == nil || s.ReadOnly || a.isAdmin() {
		return nil
	}
	var ds []api.Division
	for _, d := range s.Divisions {
		if a.spacePermissionsForDivision(d.ID, api.CapTasksManage) {
			ds = append(ds, d)
		}
	}
	return ds
}
func (a *App) beginTask() {
	ds := a.manageableDivisions()
	if len(ds) == 0 {
		return
	}
	a.tasks.draft = &taskDraft{owner: ds[0].ID, shared: map[id.ID]bool{}}
	if s := a.selectedSpace(); s != nil && s.Kind == api.SpaceShared {
		a.tasks.draft.shared[s.ID] = true
	}
	a.tasks.err = ""
	a.tasks.notice = ""
	a.workspace.Close()
	a.chrome.tab = "Board tugas"
	a.chrome.appearance = false
	a.loadTaskMembers(ds[0].ID)
}
func (a *App) editTask(t api.Task) {
	d := &taskDraft{taskID: t.ID, owner: t.OwnerDivisionID, assignee: t.Assignee.ID, version: t.Version, title: t.Title, description: t.Description, output: t.ExpectedOutput, category: t.Category, shared: map[id.ID]bool{}}
	for _, sid := range t.SpaceIDs {
		d.shared[sid] = true
	}
	d.deadline, d.deadlineTime = draftDate(t.DeadlineDate, t.DeadlineAt, t.TimeZone)
	d.posting, d.postingTime = draftDate(t.PostingDate, t.PostingAt, t.TimeZone)
	a.tasks.draft = d
	a.loadTaskMembers(d.owner)
}
func draftDate(d *timeutil.Date, at *time.Time, zone string) (string, string) {
	if d != nil {
		return d.String(), ""
	}
	if at != nil {
		loc, _ := timeutil.LoadZone(zone)
		if loc == nil {
			loc = time.UTC
		}
		return at.In(loc).Format(time.DateOnly), at.In(loc).Format("15:04")
	}
	return "", ""
}
func parseTaskDate(date, clock, zone string, required bool) (*timeutil.Date, *time.Time, error) {
	date, clock = strings.TrimSpace(date), strings.TrimSpace(clock)
	if date == "" {
		if required || clock != "" {
			return nil, nil, fmt.Errorf("tanggal wajib diisi")
		}
		return nil, nil, nil
	}
	d, err := timeutil.ParseDate(date)
	if err != nil {
		return nil, nil, err
	}
	if clock == "" {
		return &d, nil, nil
	}
	loc, err := timeutil.LoadZone(zone)
	if err != nil {
		return nil, nil, err
	}
	at, err := time.ParseInLocation("2006-01-02 15:04", date+" "+clock, loc)
	if err != nil {
		return nil, nil, fmt.Errorf("jam harus HH:MM (contoh 16:30)")
	}
	utc := at.UTC()
	return nil, &utc, nil
}
func (a *App) saveTask() {
	d := a.tasks.draft
	if d == nil {
		return
	}
	zone := timeutil.DefaultZoneName
	if p := a.period(); p != nil {
		zone = p.TimeZone
	}
	dd, dt, err := parseTaskDate(d.deadline, d.deadlineTime, zone, true)
	if err != nil {
		a.tasks.err = "Tenggat: " + err.Error()
		return
	}
	pd, pt, err := parseTaskDate(d.posting, d.postingTime, zone, false)
	if err != nil {
		a.tasks.err = "Posting: " + err.Error()
		return
	}
	if strings.TrimSpace(d.title) == "" || strings.TrimSpace(d.output) == "" || d.assignee == id.Nil {
		a.tasks.err = "Isi judul, hasil yang diharapkan, dan penanggung jawab utama."
		return
	}
	req := api.TaskWriteRequest{OwnerDivisionID: d.owner, Title: d.title, Description: d.description, ExpectedOutput: d.output, Category: d.category, AssigneeID: d.assignee, DeadlineDate: dd, DeadlineAt: dt, PostingDate: pd, PostingAt: pt, Reason: d.reason, ExpectedVersion: d.version}
	for sid, on := range d.shared {
		if on {
			req.SpaceIDs = append(req.SpaceIDs, sid)
		}
	}
	tid, scope := d.taskID, a.selectedSpaceID()
	var saved api.Task
	a.mutateTask("Tugas", func(s TasksAPI) error {
		var e error
		if tid == id.Nil {
			saved, e = s.CreateTask(context.Background(), scope, req)
		} else {
			saved, e = s.UpdateTask(context.Background(), tid, req)
		}
		return e
	}, func() {
		a.tasks.draft = nil
		a.tasks.inspected = id.Nil
		a.tasks.notice = "Tugas “" + saved.Title + "” tersimpan di server."
	})
}
func (a *App) loadTaskMembers(did id.ID) {
	if a.tasks.members == nil {
		a.tasks.members = map[id.ID][]api.Member{}
		a.tasks.membersBusy = map[id.ID]bool{}
		a.tasks.memberErrors = map[id.ID]string{}
	}
	if _, ok := a.tasks.members[did]; ok || a.tasks.membersBusy[did] {
		return
	}
	service, ok := a.workspaceService()
	if !ok {
		return
	}
	gen, seq, p := a.gen, a.tasks.seq, a.periodID
	a.tasks.membersBusy[did] = true
	a.async(func() func() {
		members, err := service.Members(context.Background(), p, did)
		return func() {
			if gen != a.gen || seq != a.tasks.seq {
				return
			}
			a.tasks.membersBusy[did] = false
			if err != nil {
				a.tasks.memberErrors[did] = a.taskFailure(err)
				return
			}
			a.tasks.members[did] = members
		}
	})
}
func (a *App) loadTaskHistory(tid id.ID) {
	s, ok := a.taskService()
	if !ok || a.tasks.historyBusy || a.tasks.historyLoaded {
		return
	}
	gen, seq := a.gen, a.tasks.seq
	a.tasks.historySeq++
	hseq := a.tasks.historySeq
	a.tasks.historyBusy = true
	a.async(func() func() {
		events, err := s.TaskHistory(context.Background(), tid)
		return func() {
			if gen != a.gen || seq != a.tasks.seq || tid != a.tasks.inspected || hseq != a.tasks.historySeq {
				return
			}
			a.tasks.historyBusy = false
			if err != nil {
				a.tasks.historyErr = a.taskFailure(err)
				return
			}
			a.tasks.history = events
			a.tasks.historyLoaded = true
		}
	})
}
func (a *App) loadTaskNotifications() {
	s, ok := a.taskService()
	if !ok {
		a.tasks.notificationsErr = "Layanan notifikasi belum tersedia."
		return
	}
	if a.tasks.notificationsBusy {
		return
	}
	gen, seq := a.gen, a.tasks.seq
	a.tasks.notificationsBusy = true
	a.tasks.notificationsErr = ""
	a.async(func() func() {
		items, err := s.Notifications(context.Background())
		return func() {
			if gen != a.gen || seq != a.tasks.seq {
				return
			}
			a.tasks.notificationsBusy = false
			if err != nil {
				a.tasks.notifications = nil
				a.tasks.notificationsErr = a.failed(err)
				return
			}
			a.tasks.notifications = items
			a.tasks.notificationsLoaded = true
		}
	})
}
