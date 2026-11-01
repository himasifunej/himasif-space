package server

import (
	"errors"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"github.com/jackc/pgx/v5"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"time"
)

func (s *Server) medhumPermissions(r *http.Request, q store.Querier, p *principal, c *api.MedhumContent) error {
	if c.HumasPJID == id.Nil {
		return badRequest("Pilih PJ Humas.")
	}
	if err := q.QueryRow(r.Context(), `select display_name from users where id=$1`, c.HumasPJID).Scan(&c.HumasPJName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return badRequest("Akun PJ Humas tidak ditemukan.")
		}
		return err
	}
	if c.DesignPJID != nil {
		if err := q.QueryRow(r.Context(), `select display_name from users where id=$1`, *c.DesignPJID).Scan(&c.DesignPJName); err != nil {
			return err
		}
	}
	view, err := s.spaceView(r.Context(), q, p, c.SpaceID)
	if err != nil {
		return err
	}
	if !slices.Contains(view.Capabilities, api.CapSpaceRead) {
		return errNotFound
	}
	if view.ReadOnly || p.admin() {
		return nil
	}
	for _, role := range view.Roles {
		if !slices.Contains(role.Capabilities, api.CapTasksManage) {
			continue
		}
		if role.DivisionID == c.HumasDivisionID {
			c.CanEditCalendar = true
		}
		if role.DivisionID == c.DesignDivisionID {
			c.CanAssignDesign = true
		}
	}
	return nil
}
func (s *Server) medhumDivisions(r *http.Request, q store.Querier, sid id.ID) (store.Space, id.ID, id.ID, error) {
	sp, err := s.store.SpaceByID(r.Context(), q, sid)
	if err != nil {
		return sp, id.Nil, id.Nil, err
	}
	if sp.Code != "medhum" || sp.Kind != api.SpaceShared {
		return sp, id.Nil, id.Nil, badRequest("Konten Medhum hanya tersedia pada ruang Medhum.")
	}
	var humas, design id.ID
	for _, d := range sp.Divisions {
		switch d.Slug {
		case "humas":
			humas = d.ID
		case "mediatek":
			design = d.ID
		}
	}
	if humas == id.Nil || design == id.Nil {
		return sp, humas, design, badRequest("Divisi Humas dan Mediatek belum tersedia pada ruang ini.")
	}
	return sp, humas, design, nil
}
func validateMedhum(c *api.MedhumContent, zone string) error {
	var err error
	c.Title, err = text(c.Title, "judul konten", 1, 220)
	if err != nil {
		return err
	}
	c.Caption, err = text(c.Caption, "caption", 0, 20000)
	if err != nil {
		return err
	}
	if (c.PostingDate == nil) == (c.PostingAt == nil) {
		return badRequest("Pilih jadwal posting tanggal saja atau waktu persis.")
	}
	if c.ConceptURL != "" {
		u, err := url.Parse(c.ConceptURL)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || len(c.ConceptURL) > 2000 {
			return badRequest("Tautan konsep tidak valid.")
		}
	}
	if c.PostingDate != nil {
		if _, err := timeutil.ParseDate(c.PostingDate.String()); err != nil {
			return badRequest("Tanggal posting tidak valid.")
		}
		d := c.PostingDate.AddDays(-2)
		c.DesignDate = &d
		c.DesignAt = nil
	} else {
		if c.PostingAt.IsZero() {
			return badRequest("Waktu posting tidak valid.")
		}
		loc, err := time.LoadLocation(zone)
		if err != nil {
			return err
		}
		at := c.PostingAt.In(loc).AddDate(0, 0, -2).UTC()
		c.DesignAt = &at
		c.DesignDate = nil
	}
	if c.HumasPJID == id.Nil {
		return badRequest("Pilih PJ Humas.")
	}
	return nil
}
func (s *Server) withMedhum(r *http.Request, p *principal, write bool, extra []id.ID, fn func(store.Querier, api.Period, *api.MedhumContent) error) error {
	cid, err := pathID(r, "contentId")
	if err != nil {
		return err
	}
	peek, err := s.store.Medhum(r.Context(), s.store.DB(), cid, store.LockNone)
	if errors.Is(err, pgx.ErrNoRows) {
		return errNotFound
	}
	if err != nil {
		return err
	}
	return s.store.Tx(r.Context(), func(q store.Querier) error {
		lock, rowLock := store.LockShare, store.LockNone
		if write {
			lock, rowLock = store.LockUpdate, store.LockUpdate
		}
		period, err := s.store.Period(r.Context(), q, peek.PeriodID, lock)
		if err != nil {
			return err
		}
		c, err := s.store.Medhum(r.Context(), q, cid, rowLock)
		if err != nil {
			return err
		}
		if err := s.medhumPermissions(r, q, p, &c); err != nil {
			return err
		}
		if write {
			if p.admin() {
				return errForbidden
			}
			if period.Status == api.PeriodClosed {
				return errPeriodClosed
			}
			accounts := append(slices.Clone(extra), p.User.ID, c.HumasPJID)
			if c.DesignPJID != nil {
				accounts = append(accounts, *c.DesignPJID)
			}
			if err := s.lockTaskAccounts(r.Context(), q, accounts); err != nil {
				return err
			}
			if _, err := s.lockedAccount(r, q, p); err != nil {
				return err
			}
		}
		return fn(q, period, &c)
	})
}
func (s *Server) listMedhum(w http.ResponseWriter, r *http.Request, p *principal) error {
	sid, err := pathID(r, "spaceId")
	if err != nil {
		return err
	}
	out := []api.MedhumContent{}
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		sp, _, _, err := s.medhumDivisions(r, q, sid)
		if err != nil {
			return err
		}
		if _, err := s.store.Period(r.Context(), q, sp.PeriodID, store.LockShare); err != nil {
			return err
		}
		if _, err := s.spaceView(r.Context(), q, p, sid); err != nil {
			return err
		}
		rows, err := s.store.MedhumContents(r.Context(), q, sid)
		if err != nil {
			return err
		}
		for _, c := range rows {
			if err := s.medhumPermissions(r, q, p, &c); err != nil {
				return err
			}
			out = append(out, c)
		}
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, api.List[api.MedhumContent]{Items: out})
	return nil
}
func (s *Server) getMedhum(w http.ResponseWriter, r *http.Request, p *principal) error {
	var out api.MedhumContent
	err := s.withMedhum(r, p, false, nil, func(q store.Querier, period api.Period, c *api.MedhumContent) error { out = *c; return nil })
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}
func (s *Server) createMedhum(w http.ResponseWriter, r *http.Request, p *principal) error {
	if p.admin() {
		return errForbidden
	}
	sid, err := pathID(r, "spaceId")
	if err != nil {
		return err
	}
	var req api.MedhumCreateRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	var out api.MedhumContent
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		sp, humas, design, err := s.medhumDivisions(r, q, sid)
		if err != nil {
			return err
		}
		period, err := s.store.Period(r.Context(), q, sp.PeriodID, store.LockUpdate)
		if err != nil {
			return err
		}
		if period.Status == api.PeriodClosed {
			return errPeriodClosed
		}
		if err := s.lockTaskAccounts(r.Context(), q, []id.ID{p.User.ID, req.HumasPJID}); err != nil {
			return err
		}
		if _, err := s.lockedAccount(r, q, p); err != nil {
			return err
		}
		now := s.now().UTC()
		out = api.MedhumContent{ID: id.New(), PeriodID: sp.PeriodID, SpaceID: sid, HumasDivisionID: humas, DesignDivisionID: design, Title: req.Title, PostingDate: req.PostingDate, PostingAt: req.PostingAt, HumasPJID: req.HumasPJID, ConceptURL: req.ConceptURL, Caption: req.Caption, Version: 1, CreatedBy: p.User.ID, CreatedAt: now, UpdatedAt: now}
		if err := s.medhumPermissions(r, q, p, &out); err != nil {
			return err
		}
		if !out.CanEditCalendar {
			return errForbidden
		}
		if err := validateMedhum(&out, period.TimeZone); err != nil {
			return err
		}
		if err := s.syncMedhumTasks(r, q, p, period, &out); err != nil {
			return err
		}
		if err := s.store.SaveMedhum(r.Context(), q, out, true); err != nil {
			return err
		}
		return s.record(r, q, p, store.AuditEvent{Action: "medhum.create", TargetType: "medhum_content", TargetID: &out.ID, PeriodID: &out.PeriodID, After: out})
	})
	if err != nil {
		return err
	}
	writeJSON(w, 201, out)
	return nil
}
func (s *Server) updateMedhum(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.MedhumUpdateRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	extras := []id.ID{}
	if req.HumasPJID != nil {
		extras = append(extras, *req.HumasPJID)
	}
	if req.DesignPJID != nil {
		extras = append(extras, *req.DesignPJID)
	}
	var out api.MedhumContent
	err := s.withMedhum(r, p, true, extras, func(q store.Querier, period api.Period, c *api.MedhumContent) error {
		if req.ExpectedVersion != c.Version {
			return conflict("Konten telah berubah. Muat ulang sebelum menyimpan.")
		}
		calendar := req.Title != nil || req.PostingDate != nil || req.PostingAt != nil || req.HumasPJID != nil || req.ConceptURL != nil || req.Caption != nil || req.TrelloCardID != nil || req.PublishedAt != nil
		if calendar && !c.CanEditCalendar || req.DesignPJID != nil && !c.CanAssignDesign {
			return errForbidden
		}
		before := *c
		if req.Title != nil {
			c.Title = *req.Title
		}
		if req.PostingDate != nil {
			c.PostingDate = req.PostingDate
			c.PostingAt = nil
		}
		if req.PostingAt != nil {
			c.PostingAt = req.PostingAt
			c.PostingDate = nil
		}
		if req.HumasPJID != nil {
			c.HumasPJID = *req.HumasPJID
		}
		if req.DesignPJID != nil {
			if *req.DesignPJID == id.Nil {
				return badRequest("Pilih PJ desain aktif; tugas yang sudah diterbitkan tidak dilepas diam-diam.")
			}
			c.DesignPJID = req.DesignPJID
		}
		if req.ConceptURL != nil {
			c.ConceptURL = *req.ConceptURL
		}
		if req.Caption != nil {
			c.Caption = *req.Caption
		}
		if req.TrelloCardID != nil {
			c.TrelloCardID = *req.TrelloCardID
		}
		if req.PublishedAt != nil {
			if req.PublishedAt.After(s.now()) {
				return badRequest("Waktu publikasi harus waktu kejadian aktual.")
			}
			c.PublishedAt = req.PublishedAt
		}
		if err := validateMedhum(c, period.TimeZone); err != nil {
			return err
		}
		if err := s.syncMedhumTasks(r, q, p, period, c); err != nil {
			return err
		}
		c.Version++
		c.UpdatedAt = s.now().UTC()
		if err := s.store.SaveMedhum(r.Context(), q, *c, false); err != nil {
			return err
		}
		out = *c
		return s.record(r, q, p, store.AuditEvent{Action: "medhum.update", TargetType: "medhum_content", TargetID: &c.ID, PeriodID: &c.PeriodID, Before: before, After: map[string]any{"content": out, "reason": req.Reason}})
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}
func (s *Server) syncMedhumTasks(r *http.Request, q store.Querier, p *principal, period api.Period, c *api.MedhumContent) error {
	if err := s.syncMedhumTask(r, q, p, period, c, false); err != nil {
		return err
	}
	if c.DesignPJID != nil {
		return s.syncMedhumTask(r, q, p, period, c, true)
	}
	return nil
}
func (s *Server) syncMedhumTask(r *http.Request, q store.Querier, p *principal, period api.Period, c *api.MedhumContent, design bool) error {
	did, uid, tid := c.HumasDivisionID, c.HumasPJID, c.HumasTaskID
	title, category, output := "Publikasi · "+c.Title, "medhum-humas", "Koordinasikan konsep, follow-up, caption dan publikasi konten. Penyerahan konsep tidak menyelesaikan seluruh tanggung jawab Humas."
	deadlineDate, deadlineAt := c.PostingDate, c.PostingAt
	if design {
		did = c.DesignDivisionID
		uid = *c.DesignPJID
		tid = c.DesignTaskID
		title = "Desain · " + c.Title
		category = "medhum-design"
		output = "Serahkan hasil desain untuk dicocokkan kadiv. Penyerahan bukan publikasi atau approval mutu."
		deadlineDate, deadlineAt = c.DesignDate, c.DesignAt
	}
	person, err := s.taskAssignee(r.Context(), q, period.ID, did, uid)
	if err != nil {
		return err
	}
	if tid == nil {
		links, err := s.taskLinks(r.Context(), q, period.ID, did, c.SpaceID, nil)
		if err != nil {
			return err
		}
		now := s.now().UTC()
		t := store.TaskRow{Task: api.Task{ID: id.New(), PeriodID: period.ID, OwnerDivisionID: did, SpaceIDs: links, Title: title, ExpectedOutput: output, Category: category, Assignee: person, DeadlineDate: deadlineDate, DeadlineAt: deadlineAt, PostingDate: c.PostingDate, PostingAt: c.PostingAt, TimeZone: period.TimeZone, Progress: api.TaskTodo, MatchStatus: api.MatchNone, Version: 1, CreatedBy: taskPerson(p), CreatedAt: now, UpdatedAt: now}, ResumeProgress: api.TaskTodo}
		if t.Rank, err = s.store.TaskRank(r.Context(), q, period.ID, t.ID, id.Nil, api.TaskTodo); err != nil {
			return err
		}
		if err := s.store.InsertTask(r.Context(), q, t); err != nil {
			return err
		}
		if err := s.taskEvent(r, q, p, &t, "medhum.task.create", "Tugas kanonis konten Medhum", nil, t.Task); err != nil {
			return err
		}
		if design {
			c.DesignTaskID = &t.ID
		} else {
			c.HumasTaskID = &t.ID
		}
		return s.taskNotify(r, q, &t, "Tugas Medhum baru", title, uid)
	}
	t, err := s.store.Task(r.Context(), q, *tid, store.LockUpdate)
	if err != nil {
		return err
	}
	before := t.Task
	t.Title = title
	t.Assignee = person
	t.DeadlineDate = deadlineDate
	t.DeadlineAt = deadlineAt
	t.PostingDate = c.PostingDate
	t.PostingAt = c.PostingAt
	if reflect.DeepEqual(before, t.Task) {
		return nil
	}
	if err := s.saveTaskMutation(r, q, p, &t, "medhum.calendar.update", "Mengikuti jadwal posting/PJ konten; status dan bukti dipertahankan", before); err != nil {
		return err
	}
	return s.taskNotify(r, q, &t, "Jadwal atau PJ Medhum berubah", title, before.Assignee.ID, uid)
}

// Linked task calendar fields have a single owner: the content model. Normal
// task edits may still update their operational details without bypassing it.
func (s *Server) guardMedhumTaskUpdate(r *http.Request, q store.Querier, t *store.TaskRow, req api.TaskWriteRequest) error {
	var linked bool
	if err := q.QueryRow(r.Context(), `select exists(select 1 from medhum_contents where humas_task_id=$1 or design_task_id=$1)`, t.ID).Scan(&linked); err != nil {
		return err
	}
	if linked && (req.Title != t.Title || req.AssigneeID != t.Assignee.ID || !reflect.DeepEqual(req.DeadlineDate, t.DeadlineDate) || !reflect.DeepEqual(req.DeadlineAt, t.DeadlineAt) || !reflect.DeepEqual(req.PostingDate, t.PostingDate) || !reflect.DeepEqual(req.PostingAt, t.PostingAt)) {
		return conflict("Jadwal dan PJ tugas Medhum dikelola melalui kalender konten sesuai wewenang Humas/Mediatek.")
	}
	return nil
}
