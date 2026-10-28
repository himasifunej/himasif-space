package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// taskAccess is computed from current space bindings, independently of being
// named on the task. A historical assignment never grants access by itself.
func (s *Server) taskAccess(ctx context.Context, q store.Querier, p *principal, period api.Period, t *store.TaskRow) error {
	spaces, err := s.store.Spaces(ctx, q, period.ID)
	if err != nil {
		return err
	}
	spaces = slices.DeleteFunc(spaces, func(sp store.Space) bool { return !slices.Contains(t.SpaceIDs, sp.ID) })
	views, err := s.spaceViews(ctx, q, p, period, spaces)
	if err != nil {
		return err
	}
	readable, own, comment, help := false, false, false, false
	for _, v := range views {
		if !slices.Contains(v.Capabilities, api.CapSpaceRead) {
			continue
		}
		readable = true
		own = own || slices.Contains(v.Capabilities, api.CapTasksOwn)
		comment = comment || slices.Contains(v.Capabilities, api.CapCommentsWrite)
		help = help || slices.Contains(v.Capabilities, api.CapHelpRequest)
	}
	if !readable {
		return errForbidden
	}
	// Shared capabilities are a union, but management/review always belongs to
	// this task's owner division. A Humas kadiv cannot edit a Mediatek task.
	r := &http.Request{}
	r.SetPathValue("periodId", period.ID.String())
	r.SetPathValue("divisionId", t.OwnerDivisionID.String())
	sc, err := s.loadScope(ctx, q, r, p, store.LockNone)
	if err != nil {
		return err
	}
	participant := t.Assignee.ID == p.User.ID || slices.ContainsFunc(t.Participants, func(x api.PersonRef) bool { return x.ID == p.User.ID })
	open := period.Status != api.PeriodClosed
	manage := open && slices.Contains(sc.caps, api.CapTasksManage)
	t.Permissions = api.TaskPermissions{Manage: manage, Progress: open && (manage || own && participant), Comment: open && comment, Help: open && help && !participant, Review: open && slices.Contains(sc.caps, api.CapResultsReview)}
	return nil
}

// Lock order matches membership, 45 and period-close writes: period first,
// task and account revalidation. Exclusive period locks also serialize rank
// maintenance and prevent authority revocation between check and commit.
func (s *Server) withTask(r *http.Request, p *principal, write bool, fn func(store.Querier, api.Period, *store.TaskRow) error) error {
	return s.withTaskAccounts(r, p, write, nil, fn)
}

func (s *Server) withTaskAccounts(r *http.Request, p *principal, write bool, extra []id.ID, fn func(store.Querier, api.Period, *store.TaskRow) error) error {
	if write && p.admin() {
		return errForbidden
	}
	tid, err := pathID(r, "taskId")
	if err != nil {
		return err
	}
	pid, err := s.store.TaskPeriod(r.Context(), s.store.DB(), tid)
	if err != nil {
		return notFound(err, errNotFound)
	}
	return s.store.Tx(r.Context(), func(q store.Querier) error {
		lock := store.LockShare
		if write {
			lock = store.LockUpdate
		}
		period, err := s.store.Period(r.Context(), q, pid, lock)
		if err != nil {
			return err
		}
		tl := store.LockNone
		if write {
			tl = store.LockUpdate
		}
		t, err := s.store.Task(r.Context(), q, tid, tl)
		if err != nil {
			return err
		}
		if write {
			accounts := append(slices.Clone(extra), p.User.ID)
			accounts = append(accounts, taskPeople(&t)...)
			managers, err := s.taskManagers(r, q, &t)
			if err != nil {
				return err
			}
			accounts = append(accounts, managers...)
			// Help decisions may lock the applicant as well. Use a global
			// account order even for tasks in different writable periods.
			// Include recipients: inserting a notification also locks the
			// referenced account through its foreign-key check.
			for _, h := range t.HelpRequests {
				if h.Status == "pending" {
					accounts = append(accounts, h.User.ID)
				}
			}
			if err = s.lockTaskAccounts(r.Context(), q, accounts); err != nil {
				return err
			}
			if _, err = s.lockedAccount(r, q, p); err != nil {
				return err
			}
		}
		if err = s.taskAccess(r.Context(), q, p, period, &t); err != nil {
			return err
		}
		if write && period.Status == api.PeriodClosed {
			return errPeriodClosed
		}
		return fn(q, period, &t)
	})
}

func (s *Server) lockTaskAccounts(ctx context.Context, q store.Querier, users []id.ID) error {
	slices.SortFunc(users, func(a, b id.ID) int { return strings.Compare(a.String(), b.String()) })
	for _, uid := range slices.Compact(users) {
		if _, err := s.store.UserForUpdate(ctx, q, uid); err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
	}
	return nil
}
func taskVersion(t *store.TaskRow, v int64) error {
	if v < 1 {
		return badRequest("Versi tugas wajib diisi.")
	}
	if t.Version != v {
		return conflict("Tugas telah berubah. Muat ulang sebelum menyimpan.")
	}
	return nil
}
func taskProgress(v string) bool {
	return slices.Contains([]string{api.TaskTodo, api.TaskDoing, api.TaskSubmitted, api.TaskDone, api.TaskCancelled}, v)
}
func taskPerson(p *principal) api.PersonRef {
	return api.PersonRef{ID: p.User.ID, DisplayName: p.User.DisplayName}
}

func taskSameInstant(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func validateTaskWrite(req *api.TaskWriteRequest) error {
	var err error
	if req.OwnerDivisionID == id.Nil || req.AssigneeID == id.Nil {
		return badRequest("Pilih divisi pemilik dan PJ utama.")
	}
	if req.Title, err = text(req.Title, "judul tugas", 1, 240); err != nil {
		return err
	}
	if req.Description, err = text(req.Description, "deskripsi", 0, 20000); err != nil {
		return err
	}
	if req.ExpectedOutput, err = text(req.ExpectedOutput, "hasil yang diminta", 1, 10000); err != nil {
		return err
	}
	if req.Category, err = text(req.Category, "kategori", 0, 120); err != nil {
		return err
	}
	if req.Reason, err = text(req.Reason, "alasan", 0, 2000); err != nil {
		return err
	}
	if (req.DeadlineDate == nil) == (req.DeadlineAt == nil) {
		return badRequest("Pilih satu tenggat: tanggal saja atau tanggal dan jam.")
	}
	if req.PostingDate != nil && req.PostingAt != nil {
		return badRequest("Pilih satu jadwal posting: tanggal saja atau tanggal dan jam.")
	}
	for _, d := range []*timeutil.Date{req.DeadlineDate, req.PostingDate} {
		if d != nil {
			if _, err = timeutil.ParseDate(d.String()); err != nil {
				return badRequest("Tanggal tidak valid.")
			}
		}
	}
	for _, at := range []*time.Time{req.DeadlineAt, req.PostingAt} {
		if at != nil {
			if at.IsZero() || at.Year() < 1 || at.Year() > 9999 {
				return badRequest("Waktu tidak valid.")
			}
			*at = at.UTC()
		}
	}
	return nil
}

// taskLinks validates all links before changing any row. The owner space is
// implicit and accepted idempotently when round-tripping a Task DTO.
func (s *Server) taskLinks(ctx context.Context, q store.Querier, pid, did, source id.ID, extra []id.ID) ([]id.ID, error) {
	spaces, err := s.store.Spaces(ctx, q, pid)
	if err != nil {
		return nil, err
	}
	var owner id.ID
	byID := map[id.ID]store.Space{}
	for _, sp := range spaces {
		byID[sp.ID] = sp
		if sp.Kind == api.SpaceDivision && sp.HasDivision(did) {
			owner = sp.ID
		}
	}
	if owner == id.Nil {
		return nil, badRequest("Divisi pemilik tidak ada pada periode ini.")
	}
	ids := []id.ID{owner}
	extra = slices.Clone(extra)
	if source != id.Nil {
		extra = append(extra, source)
	}
	for _, sid := range extra {
		if sid == owner {
			continue
		}
		sp, ok := byID[sid]
		if !ok || sp.Kind != api.SpaceShared || !sp.HasDivision(did) {
			return nil, badRequest("Space bersama harus satu periode dan memuat divisi pemilik.")
		}
		if !slices.Contains(ids, sid) {
			ids = append(ids, sid)
		}
	}
	return ids, nil
}
func (s *Server) taskAssignee(ctx context.Context, q store.Querier, pid, did, uid id.ID) (api.PersonRef, error) {
	u, err := s.store.UserForUpdate(ctx, q, uid)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return api.PersonRef{}, err
	}
	if err != nil || u.Kind != api.AccountPerson || u.DisabledAt != nil || u.EmailVerifiedAt == nil {
		return api.PersonRef{}, badRequest("PJ harus akun pribadi aktif dan terverifikasi.")
	}
	if _, err = s.store.ActiveMembership(ctx, q, pid, did, uid); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return api.PersonRef{}, badRequest("PJ harus anggota aktif divisi pemilik.")
		}
		return api.PersonRef{}, err
	}
	return api.PersonRef{ID: u.ID, DisplayName: u.DisplayName}, nil
}
func (s *Server) taskEvent(r *http.Request, q store.Querier, p *principal, t *store.TaskRow, action, reason string, before, after any) error {
	e := api.TaskEvent{ID: id.New(), Actor: taskPerson(p), Action: action, Reason: reason, CreatedAt: s.now().UTC().Truncate(time.Microsecond), Before: before, After: after}
	if err := s.store.AddTaskEvent(r.Context(), q, t.ID, e); err != nil {
		return err
	}
	return s.record(r, q, p, store.AuditEvent{Action: action, TargetType: "task", TargetID: &t.ID, PeriodID: &t.PeriodID, ScopeType: "division", ScopeID: &t.OwnerDivisionID, Before: before, After: map[string]any{"value": after, "reason": reason}, OccurredAt: e.CreatedAt})
}
func (s *Server) taskNotify(r *http.Request, q store.Querier, t *store.TaskRow, title, body string, users ...id.ID) error {
	seen := map[id.ID]bool{}
	for _, uid := range users {
		if uid == id.Nil || seen[uid] {
			continue
		}
		seen[uid] = true
		n := api.Notification{ID: id.New(), TaskID: t.ID, Title: title, Body: body, CreatedAt: s.now().UTC().Truncate(time.Microsecond)}
		if err := s.store.AddTaskNotification(r.Context(), q, uid, n); err != nil {
			return err
		}
	}
	return nil
}
func taskPeople(t *store.TaskRow) []id.ID {
	out := []id.ID{t.Assignee.ID}
	for _, p := range t.Participants {
		out = append(out, p.ID)
	}
	return out
}
func (s *Server) taskManagers(r *http.Request, q store.Querier, t *store.TaskRow) ([]id.ID, error) {
	ms, err := s.store.DivisionMemberships(r.Context(), q, t.PeriodID, t.OwnerDivisionID)
	if err != nil {
		return nil, err
	}
	roles, err := s.store.CustomRoles(r.Context(), q, t.PeriodID, t.OwnerDivisionID)
	if err != nil {
		return nil, err
	}
	byID := rolesByID(roles)
	out := []id.ID{}
	for _, m := range ms {
		caps := bindingOf(m, byID).Capabilities()
		if slices.Contains(caps, api.CapTasksManage) || slices.Contains(caps, api.CapResultsReview) {
			out = append(out, m.UserID)
		}
	}
	return out, nil
}
func (s *Server) saveTaskMutation(r *http.Request, q store.Querier, p *principal, t *store.TaskRow, action, reason string, before any) error {
	t.Version++
	t.UpdatedAt = s.now().UTC().Truncate(time.Microsecond)
	if err := s.store.SaveTask(r.Context(), q, *t); err != nil {
		return err
	}
	return s.taskEvent(r, q, p, t, action, reason, before, t.Task)
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request, p *principal) error {
	if p.admin() {
		return errForbidden
	}
	var req api.TaskWriteRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if err := validateTaskWrite(&req); err != nil {
		return err
	}
	sid, err := pathID(r, "spaceId")
	if err != nil {
		return err
	}
	sp, err := s.store.SpaceByID(r.Context(), s.store.DB(), sid)
	if err != nil {
		return notFound(err, errNotFound)
	}
	var out api.Task
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		period, err := s.store.Period(r.Context(), q, sp.PeriodID, store.LockUpdate)
		if err != nil {
			return err
		}
		if err = s.lockTaskAccounts(r.Context(), q, []id.ID{p.User.ID, req.AssigneeID}); err != nil {
			return err
		}
		if _, err = s.lockedAccount(r, q, p); err != nil {
			return err
		}
		if _, err = s.spaceView(r.Context(), q, p, sid); err != nil {
			return err
		}
		rr := r.Clone(r.Context())
		rr.SetPathValue("periodId", period.ID.String())
		rr.SetPathValue("divisionId", req.OwnerDivisionID.String())
		sc, err := s.loadScope(r.Context(), q, rr, p, store.LockNone)
		if err != nil {
			return err
		}
		if err = sc.require(api.CapTasksManage); err != nil {
			return err
		}
		links, err := s.taskLinks(r.Context(), q, period.ID, req.OwnerDivisionID, sid, req.SpaceIDs)
		if err != nil {
			return err
		}
		assignee, err := s.taskAssignee(r.Context(), q, period.ID, req.OwnerDivisionID, req.AssigneeID)
		if err != nil {
			return err
		}
		now := s.now().UTC().Truncate(time.Microsecond)
		t := store.TaskRow{Task: api.Task{ID: id.New(), PeriodID: period.ID, OwnerDivisionID: req.OwnerDivisionID, SpaceIDs: links, Title: req.Title, Description: req.Description, ExpectedOutput: req.ExpectedOutput, Category: req.Category, Assignee: assignee, DeadlineDate: req.DeadlineDate, DeadlineAt: req.DeadlineAt, PostingDate: req.PostingDate, PostingAt: req.PostingAt, TimeZone: period.TimeZone, Progress: api.TaskTodo, MatchStatus: api.MatchNone, Version: 1, CreatedBy: taskPerson(p), CreatedAt: now, UpdatedAt: now, Participants: []api.PersonRef{}, Evidence: []api.TaskEvidence{}, Comments: []api.TaskComment{}, HelpRequests: []api.TaskHelpRequest{}}, ResumeProgress: api.TaskTodo}
		if t.Rank, err = s.store.TaskRank(r.Context(), q, period.ID, t.ID, id.Nil, api.TaskTodo); err != nil {
			return err
		}
		if err = s.store.InsertTask(r.Context(), q, t); err != nil {
			return err
		}
		if err = s.taskAccess(r.Context(), q, p, period, &t); err != nil {
			return err
		}
		if err = s.taskEvent(r, q, p, &t, "task.create", req.Reason, nil, t.Task); err != nil {
			return err
		}
		if err = s.taskNotify(r, q, &t, "Tugas baru", t.Title, t.Assignee.ID); err != nil {
			return err
		}
		out = t.Task
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}

func (s *Server) getTask(w http.ResponseWriter, r *http.Request, p *principal) error {
	var out api.Task
	err := s.withTask(r, p, false, func(_ store.Querier, _ api.Period, t *store.TaskRow) error { out = t.Task; return nil })
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}

func (s *Server) updateTask(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.TaskWriteRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if err := validateTaskWrite(&req); err != nil {
		return err
	}
	var out api.Task
	err := s.withTaskAccounts(r, p, true, []id.ID{req.AssigneeID}, func(q store.Querier, period api.Period, t *store.TaskRow) error {
		if !t.Permissions.Manage {
			return errForbidden
		}
		if err := taskVersion(t, req.ExpectedVersion); err != nil {
			return err
		}
		if req.OwnerDivisionID != t.OwnerDivisionID {
			return badRequest("Divisi pemilik tugas tidak dapat dipindahkan.")
		}
		changed := req.AssigneeID != t.Assignee.ID || !reflect.DeepEqual(req.DeadlineDate, t.DeadlineDate) || !taskSameInstant(req.DeadlineAt, t.DeadlineAt)
		if changed && req.Reason == "" {
			return badRequest("Isi alasan perubahan PJ atau tenggat.")
		}
		links, err := s.taskLinks(r.Context(), q, t.PeriodID, t.OwnerDivisionID, id.Nil, req.SpaceIDs)
		if err != nil {
			return err
		}
		assignee, err := s.taskAssignee(r.Context(), q, t.PeriodID, t.OwnerDivisionID, req.AssigneeID)
		if err != nil {
			return err
		}
		before := t.Task
		recipients := taskPeople(t)
		t.Title, t.Description, t.ExpectedOutput, t.Category = req.Title, req.Description, req.ExpectedOutput, req.Category
		t.SpaceIDs = links
		t.Assignee = assignee
		t.DeadlineDate, t.DeadlineAt, t.PostingDate, t.PostingAt = req.DeadlineDate, req.DeadlineAt, req.PostingDate, req.PostingAt
		if err = s.store.TaskSpaces(r.Context(), q, *t); err != nil {
			return err
		}
		if err = s.taskAccess(r.Context(), q, p, period, t); err != nil {
			return err
		}
		if err = s.saveTaskMutation(r, q, p, t, "task.update", req.Reason, before); err != nil {
			return err
		}
		if changed {
			recipients = append(recipients, taskPeople(t)...)
			if err = s.taskNotify(r, q, t, "Penugasan atau tenggat berubah", t.Title, recipients...); err != nil {
				return err
			}
		}
		out = t.Task
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}

func (s *Server) moveTask(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.TaskMoveRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if !taskProgress(req.Progress) {
		return badRequest("Status tugas tidak valid.")
	}
	reason, err := text(req.Reason, "alasan", 0, 2000)
	if err != nil {
		return err
	}
	var out api.Task
	err = s.withTask(r, p, true, func(q store.Querier, period api.Period, t *store.TaskRow) error {
		if !t.Permissions.Progress {
			return errForbidden
		}
		if err := taskVersion(t, req.ExpectedVersion); err != nil {
			return err
		}
		reopening := t.MatchStatus == api.MatchVerified && req.Progress != api.TaskDone
		if req.Progress == api.TaskCancelled || t.Progress == api.TaskCancelled && req.Progress != api.TaskCancelled || reopening {
			if !t.Permissions.Manage {
				return errForbidden
			}
			if reason == "" {
				return badRequest("Isi alasan pembatalan atau pembukaan kembali tugas.")
			}
		}
		if req.Progress == api.TaskDone && t.MatchStatus != api.MatchVerified {
			return conflict("Bukti perlu dicocokkan sebelum tugas dapat selesai.")
		}
		if req.Progress == api.TaskSubmitted && t.MatchStatus != api.MatchPending {
			return badRequest("Kirim bukti hasil sebelum memindahkan tugas ke perlu tinjauan.")
		}
		if req.BeforeID != id.Nil {
			if req.BeforeID == t.ID {
				return badRequest("Urutan tujuan tidak valid.")
			}
			other, err := s.store.Task(r.Context(), q, req.BeforeID, store.LockNone)
			if err != nil {
				return notFound(err, errNotFound)
			}
			if other.PeriodID != t.PeriodID || other.Progress != req.Progress {
				return badRequest("Tujuan urutan harus satu periode dan kolom.")
			}
			if err = s.taskAccess(r.Context(), q, p, period, &other); err != nil {
				return err
			}
		}
		before := t.Task
		if reopening {
			t.MatchStatus = api.MatchNone
		}
		t.Progress = req.Progress
		if (t.Progress == api.TaskTodo || t.Progress == api.TaskDoing) && t.MatchStatus != api.MatchPending {
			t.ResumeProgress = t.Progress
		}
		rank, err := s.store.TaskRank(r.Context(), q, t.PeriodID, t.ID, req.BeforeID, t.Progress)
		if err != nil {
			return err
		}
		t.Rank = rank
		if err = s.saveTaskMutation(r, q, p, t, "task.move", reason, before); err != nil {
			return err
		}
		out = t.Task
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}

type taskFilter struct {
	assignee id.ID
	progress string
	due      *timeutil.Date
}

func parseTaskFilter(r *http.Request) (taskFilter, error) {
	var f taskFilter
	var err error
	if v := r.URL.Query().Get("assigneeId"); v != "" {
		if f.assignee, err = id.Parse(v); err != nil {
			return f, badRequest("Filter PJ tidak valid.")
		}
	}
	f.progress = r.URL.Query().Get("progress")
	if f.progress != "" && !taskProgress(f.progress) {
		return f, badRequest("Filter status tidak valid.")
	}
	if v := r.URL.Query().Get("dueBefore"); v != "" {
		d, e := timeutil.ParseDate(v)
		if e != nil {
			return f, badRequest("Filter tanggal tidak valid.")
		}
		f.due = &d
	}
	return f, nil
}
func (f taskFilter) matches(t *store.TaskRow) bool {
	if f.assignee != id.Nil && t.Assignee.ID != f.assignee {
		return false
	}
	if f.progress != "" && t.Progress != f.progress {
		return false
	}
	if f.due != nil {
		if t.DeadlineDate != nil {
			return t.DeadlineDate.Before(*f.due)
		}
		loc, e := timeutil.LoadZone(t.TimeZone)
		if e != nil {
			return false
		}
		return timeutil.DateOf(t.DeadlineAt.In(loc)).Before(*f.due)
	}
	return true
}
func (s *Server) listSpaceTasks(w http.ResponseWriter, r *http.Request, p *principal) error {
	return s.listTasks(w, r, p, false)
}
func (s *Server) listMyTasks(w http.ResponseWriter, r *http.Request, p *principal) error {
	return s.listTasks(w, r, p, true)
}
func (s *Server) listTasks(w http.ResponseWriter, r *http.Request, p *principal, mine bool) error {
	f, err := parseTaskFilter(r)
	if err != nil {
		return err
	}
	var pid, sid id.ID
	if mine {
		pid, err = pathID(r, "periodId")
	} else {
		sid, err = pathID(r, "spaceId")
		if err == nil {
			var sp store.Space
			sp, err = s.store.SpaceByID(r.Context(), s.store.DB(), sid)
			pid = sp.PeriodID
		}
	}
	if err != nil {
		return notFound(err, errNotFound)
	}
	out := []api.Task{}
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		period, err := s.store.Period(r.Context(), q, pid, store.LockShare)
		if err != nil {
			return notFound(err, errNotFound)
		}
		if !mine {
			if _, err = s.spaceView(r.Context(), q, p, sid); err != nil {
				return err
			}
		}
		ids, err := s.store.TaskIDs(r.Context(), q, pid)
		if err != nil {
			return err
		}
		for _, tid := range ids {
			t, err := s.store.Task(r.Context(), q, tid, store.LockNone)
			if err != nil {
				return err
			}
			if !mine && !slices.Contains(t.SpaceIDs, sid) {
				continue
			}
			if mine && t.Assignee.ID != p.User.ID && !slices.ContainsFunc(t.Participants, func(x api.PersonRef) bool { return x.ID == p.User.ID }) {
				continue
			}
			if !f.matches(&t) {
				continue
			}
			if err = s.taskAccess(r.Context(), q, p, period, &t); errors.Is(err, errForbidden) {
				continue
			} else if err != nil {
				return err
			}
			out = append(out, t.Task)
		}
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, api.List[api.Task]{Items: out})
	return nil
}

func (s *Server) addTaskComment(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.TaskCommentRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	body, err := text(req.Body, "komentar", 1, 10000)
	if err != nil {
		return err
	}
	var out api.TaskComment
	err = s.withTask(r, p, true, func(q store.Querier, _ api.Period, t *store.TaskRow) error {
		if !t.Permissions.Comment {
			return errForbidden
		}
		out = api.TaskComment{ID: id.New(), Author: taskPerson(p), Body: body, CreatedAt: s.now().UTC().Truncate(time.Microsecond)}
		if err := s.store.AddTaskComment(r.Context(), q, t.ID, out); err != nil {
			return err
		}
		return s.taskEvent(r, q, p, t, "task.comment", "", nil, out)
	})
	if err != nil {
		return err
	}
	writeJSON(w, 201, out)
	return nil
}
func (s *Server) offerTaskHelp(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.TaskHelpCreateRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	message, err := text(req.Message, "pesan bantuan", 0, 2000)
	if err != nil {
		return err
	}
	var out api.TaskHelpRequest
	err = s.withTask(r, p, true, func(q store.Querier, _ api.Period, t *store.TaskRow) error {
		if !t.Permissions.Help {
			return errForbidden
		}
		if t.Progress == api.TaskCancelled || t.Progress == api.TaskDone {
			return conflict("Tugas ini sudah selesai atau dibatalkan.")
		}
		out = api.TaskHelpRequest{ID: id.New(), User: taskPerson(p), Message: message, Status: "pending", CreatedAt: s.now().UTC().Truncate(time.Microsecond)}
		if err := s.store.AddTaskHelp(r.Context(), q, t.ID, out); err != nil {
			if errors.Is(err, store.ErrConflict) {
				return conflict("Pengajuan bantuan masih menunggu keputusan.")
			}
			return err
		}
		if err := s.taskEvent(r, q, p, t, "task.help.offer", "", nil, out); err != nil {
			return err
		}
		users, err := s.taskManagers(r, q, t)
		if err != nil {
			return err
		}
		return s.taskNotify(r, q, t, "Pengajuan bantuan", t.Title, users...)
	})
	if err != nil {
		return err
	}
	writeJSON(w, 201, out)
	return nil
}
func (s *Server) decideTaskHelp(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.TaskHelpDecisionRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	hid, err := pathID(r, "helpId")
	if err != nil {
		return err
	}
	var out api.TaskHelpRequest
	err = s.withTask(r, p, true, func(q store.Querier, period api.Period, t *store.TaskRow) error {
		if !t.Permissions.Manage {
			return errForbidden
		}
		if err := taskVersion(t, req.ExpectedVersion); err != nil {
			return err
		}
		i := slices.IndexFunc(t.HelpRequests, func(x api.TaskHelpRequest) bool { return x.ID == hid })
		if i < 0 {
			return errNotFound
		}
		out = t.HelpRequests[i]
		if out.Status != "pending" {
			return conflict("Pengajuan sudah diputuskan.")
		}
		if req.Approved {
			if t.Progress == api.TaskCancelled || t.Progress == api.TaskDone {
				return conflict("Tugas ini sudah selesai atau dibatalkan.")
			}
			u, err := s.store.UserForUpdate(r.Context(), q, out.User.ID)
			if err != nil {
				return err
			}
			if u.DisabledAt != nil || u.Kind != api.AccountPerson || u.EmailVerifiedAt == nil {
				return badRequest("Calon pelaksana tidak aktif.")
			}
			copy := *t
			candidate := &principal{User: u}
			if err = s.taskAccess(r.Context(), q, candidate, period, &copy); err != nil {
				return err
			}
			if !copy.Permissions.Help {
				return errForbidden
			}
		}
		before := t.Task
		out.Status = "rejected"
		if req.Approved {
			out.Status = "approved"
			t.Participants = append(t.Participants, out.User)
		}
		t.HelpRequests = append([]api.TaskHelpRequest(nil), t.HelpRequests...)
		t.HelpRequests[i] = out
		if err := s.store.DecideTaskHelp(r.Context(), q, t.ID, out, p.User.ID, s.now().UTC().Truncate(time.Microsecond)); err != nil {
			return err
		}
		if err := s.saveTaskMutation(r, q, p, t, "task.help.decide", "", before); err != nil {
			return err
		}
		return s.taskNotify(r, q, t, "Keputusan pengajuan bantuan", t.Title, out.User.ID, t.Assignee.ID)
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}

func (s *Server) submitTaskEvidence(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.TaskEvidenceRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	var err error
	if req.URL, err = text(req.URL, "tautan hasil", 0, 4096); err != nil {
		return err
	}
	if req.Note, err = text(req.Note, "catatan hasil", 0, 10000); err != nil {
		return err
	}
	if req.URL == "" && req.Note == "" {
		return badRequest("Isi tautan atau catatan hasil.")
	}
	if req.URL != "" {
		u, e := url.Parse(req.URL)
		if e != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil || strings.ContainsAny(req.URL, "\r\n") {
			return badRequest("Tautan hasil harus berupa URL http atau https.")
		}
	}
	var out api.TaskEvidence
	err = s.withTask(r, p, true, func(q store.Querier, _ api.Period, t *store.TaskRow) error {
		if !t.Permissions.Progress {
			return errForbidden
		}
		if err := taskVersion(t, req.ExpectedVersion); err != nil {
			return err
		}
		if t.Progress == api.TaskCancelled {
			return conflict("Buka kembali tugas sebelum mengirim hasil.")
		}
		if t.MatchStatus == api.MatchPending {
			return conflict("Bukti sebelumnya masih perlu tinjauan.")
		}
		if t.MatchStatus == api.MatchVerified {
			if !t.Permissions.Manage {
				return errForbidden
			}
			if req.Note == "" {
				return badRequest("Isi catatan alasan koreksi bukti terverifikasi.")
			}
		}
		before := t.Task
		if (t.Progress == api.TaskTodo || t.Progress == api.TaskDoing) && t.MatchStatus != api.MatchPending {
			t.ResumeProgress = t.Progress
		}
		out = api.TaskEvidence{ID: id.New(), SubmittedBy: taskPerson(p), URL: req.URL, Note: req.Note, MatchStatus: api.MatchPending, SubmittedAt: s.now().UTC().Truncate(time.Microsecond)}
		if err := s.store.AddTaskEvidence(r.Context(), q, t.ID, out); err != nil {
			return err
		}
		t.Progress, t.MatchStatus = api.TaskSubmitted, api.MatchPending
		t.Evidence = append(t.Evidence, out)
		rank, err := s.store.TaskRank(r.Context(), q, t.PeriodID, t.ID, id.Nil, t.Progress)
		if err != nil {
			return err
		}
		t.Rank = rank
		if err = s.saveTaskMutation(r, q, p, t, "task.evidence.submit", req.Note, before); err != nil {
			return err
		}
		users, err := s.taskManagers(r, q, t)
		if err != nil {
			return err
		}
		users = append(users, taskPeople(t)...)
		return s.taskNotify(r, q, t, "Hasil dikirim — perlu tinjauan", t.Title, users...)
	})
	if err != nil {
		return err
	}
	writeJSON(w, 201, out)
	return nil
}

func (s *Server) reviewTaskEvidence(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.TaskEvidenceReviewRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	reason, err := text(req.Reason, "alasan pencocokan bukti", 1, 2000)
	if err != nil {
		return err
	}
	eid, err := pathID(r, "evidenceId")
	if err != nil {
		return err
	}
	var out api.TaskEvidence
	err = s.withTask(r, p, true, func(q store.Querier, _ api.Period, t *store.TaskRow) error {
		if !t.Permissions.Review {
			return errForbidden
		}
		if err := taskVersion(t, req.ExpectedVersion); err != nil {
			return err
		}
		i := slices.IndexFunc(t.Evidence, func(x api.TaskEvidence) bool { return x.ID == eid })
		if i < 0 {
			return errNotFound
		}
		out = t.Evidence[i]
		// A final decision can be corrected by an authorized reviewer, but never
		// against superseded evidence while newer evidence exists.
		if i != len(t.Evidence)-1 {
			return conflict("Bukti telah digantikan oleh penyerahan yang lebih baru.")
		}
		if t.Progress == api.TaskCancelled {
			return conflict("Buka kembali tugas sebelum meninjau bukti.")
		}
		before := t.Task
		now, person := s.now().UTC().Truncate(time.Microsecond), taskPerson(p)
		out.ReviewedAt = &now
		out.ReviewedBy = &person
		out.ReviewReason = reason
		out.MatchStatus = api.MatchRejected
		t.Progress = t.ResumeProgress
		if req.Matched {
			out.MatchStatus = api.MatchVerified
			t.Progress = api.TaskDone
		}
		t.MatchStatus = out.MatchStatus
		t.Evidence = append([]api.TaskEvidence(nil), t.Evidence...)
		t.Evidence[i] = out
		if err := s.store.ReviewTaskEvidence(r.Context(), q, t.ID, out); err != nil {
			return err
		}
		rank, err := s.store.TaskRank(r.Context(), q, t.PeriodID, t.ID, id.Nil, t.Progress)
		if err != nil {
			return err
		}
		t.Rank = rank
		if err = s.saveTaskMutation(r, q, p, t, "task.evidence.review", reason, before); err != nil {
			return err
		}
		return s.taskNotify(r, q, t, "Pencocokan hasil diperbarui", t.Title, taskPeople(t)...)
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}

func (s *Server) taskHistory(w http.ResponseWriter, r *http.Request, p *principal) error {
	out := []api.TaskEvent{}
	err := s.withTask(r, p, false, func(q store.Querier, _ api.Period, t *store.TaskRow) error {
		var err error
		out, err = s.store.TaskHistory(r.Context(), q, t.ID)
		return err
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, api.List[api.TaskEvent]{Items: nonNil(out)})
	return nil
}
func (s *Server) listTaskNotifications(w http.ResponseWriter, r *http.Request, p *principal) error {
	ns, err := s.store.TaskNotifications(r.Context(), s.store.DB(), p.User.ID)
	if err != nil {
		return err
	}
	out := []api.Notification{}
	for _, n := range ns {
		rr := r.Clone(r.Context())
		rr.SetPathValue("taskId", n.TaskID.String())
		err = s.withTask(rr, p, false, func(_ store.Querier, _ api.Period, _ *store.TaskRow) error { out = append(out, n); return nil })
		if err != nil && !errors.Is(err, errForbidden) && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, errNotFound) {
			return err
		}
	}
	writeJSON(w, 200, api.List[api.Notification]{Items: out})
	return nil
}
func (s *Server) readTaskNotification(w http.ResponseWriter, r *http.Request, p *principal) error {
	nid, err := pathID(r, "notificationId")
	if err != nil {
		return err
	}
	ns, err := s.store.TaskNotifications(r.Context(), s.store.DB(), p.User.ID)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(ns, func(n api.Notification) bool { return n.ID == nid })
	if i < 0 {
		return errNotFound
	}
	rr := r.Clone(r.Context())
	rr.SetPathValue("taskId", ns[i].TaskID.String())
	// Marking a private delivery read is not a mutation of a closed task.
	err = s.withTask(rr, p, false, func(q store.Querier, _ api.Period, t *store.TaskRow) error {
		if _, err := s.lockedAccount(r, q, p); err != nil {
			return err
		}
		now := s.now().UTC().Truncate(time.Microsecond)
		if err := s.store.ReadTaskNotification(r.Context(), q, nid, p.User.ID, now); err != nil {
			return err
		}
		return s.record(r, q, p, store.AuditEvent{Action: "notification.read", TargetType: "notification", TargetID: &nid,
			PeriodID: &t.PeriodID, ScopeType: "division", ScopeID: &t.OwnerDivisionID,
			After: map[string]any{"taskId": t.ID}, OccurredAt: now})
	})
	if err != nil {
		return err
	}
	w.WriteHeader(204)
	return nil
}
