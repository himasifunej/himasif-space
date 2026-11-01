package server

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/integrations"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/jackc/pgx/v5"
)

func (s *Server) createEmergencySnapshot(w http.ResponseWriter, r *http.Request, p *principal) error {
	if p.admin() {
		return errForbidden
	}
	sid, err := pathID(r, "spaceId")
	if err != nil {
		return err
	}
	var req api.EmergencySnapshotRequest
	if err = readJSON(r, &req); err != nil {
		return err
	}
	var out api.EmergencySnapshot
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		space, err := s.store.SpaceByID(r.Context(), q, sid)
		if err != nil {
			return err
		}
		if _, err = s.store.Period(r.Context(), q, space.PeriodID, store.LockShare); err != nil {
			return err
		}
		if _, err = s.lockedAccount(r, q, p); err != nil {
			return err
		}
		view, err := s.spaceView(r.Context(), q, p, sid)
		if err != nil {
			return err
		}
		if !slices.Contains(view.Capabilities, api.CapSpaceRead) {
			return errForbidden
		}
		rows, err := q.Query(r.Context(), `select t.id,t.version,t.title,t.expected_output,u.id,u.display_name,t.deadline_date::text,t.deadline_at,p.time_zone,t.progress,t.match_status from tasks t join task_spaces ts on ts.task_id=t.id join periods p on p.id=t.period_id join users u on u.id=t.assignee_id where ts.space_id=$1 and ($2 or t.progress not in ('done','cancelled')) and ($3::uuid is null or t.assignee_id=$3) order by t.id limit 5001`, sid, req.IncludeCompleted, req.PersonID)
		if err != nil {
			return err
		}
		items := []api.EmergencyTask{}
		for rows.Next() {
			var item api.EmergencyTask
			var on *string
			var at *time.Time
			if err = rows.Scan(&item.TaskID, &item.Version, &item.Title, &item.ExpectedOutput, &item.Assignee.ID, &item.Assignee.DisplayName, &on, &at, &item.TimeZone, &item.Progress, &item.MatchStatus); err != nil {
				rows.Close()
				return err
			}
			item.RowID = id.New()
			item.DeadlineKind = "date"
			if on != nil {
				item.DeadlineValue = *on
			} else if at != nil {
				item.DeadlineKind = "instant"
				item.DeadlineValue = at.UTC().Format(time.RFC3339Nano)
			}
			items = append(items, item)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if len(items) > 5000 {
			return badRequest("Snapshot melebihi 5000 tugas; pilih PJ tertentu atau keluarkan tugas selesai.")
		}
		// Guard the eventual CSV download before publishing an unusably large
		// snapshot. No row is silently truncated to fit a transport limit.
		if _, err = emergencyCSV(api.EmergencySnapshot{}, items); err != nil {
			return err
		}
		out = api.EmergencySnapshot{ID: id.New(), SpaceID: sid, SpaceName: view.Name, PeriodID: space.PeriodID, CapturedAt: s.now().UTC(), CapturedBy: taskPerson(p), RowCount: len(items)}
		data, err := json.Marshal(out)
		if err != nil {
			return err
		}
		if _, err = q.Exec(r.Context(), `insert into emergency_snapshots(id,space_id,period_id,captured_by,captured_at,data) values($1,$2,$3,$4,$5,$6)`, out.ID, sid, space.PeriodID, p.User.ID, out.CapturedAt, data); err != nil {
			return err
		}
		for _, item := range items {
			data, err = json.Marshal(item)
			if err != nil {
				return err
			}
			if _, err = q.Exec(r.Context(), `insert into emergency_snapshot_rows(id,snapshot_id,task_id,data) values($1,$2,$3,$4)`, item.RowID, out.ID, item.TaskID, data); err != nil {
				return err
			}
		}
		return s.record(r, q, p, store.AuditEvent{Action: "emergency.snapshot.create", TargetType: "emergency_snapshot", TargetID: &out.ID, PeriodID: &space.PeriodID, ScopeType: "space", ScopeID: &sid, After: out})
	})
	if err != nil {
		return err
	}
	writeJSON(w, 201, out)
	return nil
}
func emergencyCSV(snapshot api.EmergencySnapshot, items []api.EmergencyTask) ([]byte, error) {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	if err := writer.Write([]string{"snapshot_id", "row_id", "task_id", "captured_at_utc", "task_version", "title", "expected_output", "assignee", "deadline_kind", "deadline_value", "time_zone", "base_progress", "base_match_status", "new_progress", "reported_at", "reported_by", "change_note"}); err != nil {
		return nil, err
	}
	for _, item := range items {
		row := []string{snapshot.ID.String(), item.RowID.String(), item.TaskID.String(), snapshot.CapturedAt.Format(time.RFC3339Nano), fmt.Sprint(item.Version), item.Title, item.ExpectedOutput, item.Assignee.DisplayName, item.DeadlineKind, item.DeadlineValue, item.TimeZone, item.Progress, item.MatchStatus, "", "", "", ""}
		for i := range row {
			row[i] = spreadsheetCell(row[i])
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return nil, err
		}
		if buffer.Len() > 32<<20 {
			return nil, badRequest("CSV melebihi 32 MiB; pilih snapshot dengan PJ atau status yang lebih terbatas.")
		}
	}
	writer.Flush()
	return buffer.Bytes(), writer.Error()
}
func (s *Server) emergencySnapshotAccess(r *http.Request, q store.Querier, p *principal, snapshotID id.ID) (api.EmergencySnapshot, error) {
	var out api.EmergencySnapshot
	if p.admin() {
		return out, errForbidden
	}
	var data []byte
	err := q.QueryRow(r.Context(), `select data from emergency_snapshots where id=$1`, snapshotID).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, errNotFound
	}
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	view, err := s.spaceView(r.Context(), q, p, out.SpaceID)
	if err != nil {
		return out, err
	}
	if !slices.Contains(view.Capabilities, api.CapSpaceRead) {
		return out, errForbidden
	}
	return out, nil
}
func (s *Server) downloadEmergencySnapshot(w http.ResponseWriter, r *http.Request, p *principal) error {
	snapshotID, err := pathID(r, "snapshotId")
	if err != nil {
		return err
	}
	var output []byte
	err = s.store.Tx(r.Context(), func(q store.Querier) error {
		snapshot, err := s.emergencySnapshotAccess(r, q, p, snapshotID)
		if err != nil {
			return err
		}
		if _, err = s.store.Period(r.Context(), q, snapshot.PeriodID, store.LockShare); err != nil {
			return err
		}
		// Recheck access after obtaining the period lock, including revocations
		// which committed while the request waited for a writer.
		if _, err = s.emergencySnapshotAccess(r, q, p, snapshotID); err != nil {
			return err
		}
		rows, err := q.Query(r.Context(), `select e.data,exists(select 1 from task_spaces ts where ts.task_id=e.task_id and ts.space_id=$2) from emergency_snapshot_rows e where e.snapshot_id=$1 order by e.id`, snapshotID, snapshot.SpaceID)
		if err != nil {
			return err
		}
		defer rows.Close()
		items := []api.EmergencyTask{}
		for rows.Next() {
			var data []byte
			var visible bool
			if err = rows.Scan(&data, &visible); err != nil {
				return err
			}
			if !visible {
				return conflict("Hubungan tugas/Space berubah; buat snapshot baru sebelum mengunduh.")
			}
			var item api.EmergencyTask
			if err = json.Unmarshal(data, &item); err != nil {
				return err
			}
			items = append(items, item)
		}
		if err = rows.Err(); err != nil {
			return err
		}
		output, err = emergencyCSV(snapshot, items)
		return err
	})
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="space-emergency.csv"`)
	_, err = w.Write(output)
	return err
}

func emergencyTask(t *store.TaskRow) api.EmergencyTask {
	out := api.EmergencyTask{TaskID: t.ID, Version: t.Version, Title: t.Title, ExpectedOutput: t.ExpectedOutput, Assignee: t.Assignee, TimeZone: t.TimeZone, Progress: t.Progress, MatchStatus: t.MatchStatus, DeadlineKind: "date"}
	if t.DeadlineDate != nil {
		out.DeadlineValue = t.DeadlineDate.String()
	} else if t.DeadlineAt != nil {
		out.DeadlineKind = "instant"
		out.DeadlineValue = t.DeadlineAt.UTC().Format(time.RFC3339Nano)
	}
	return out
}
func emergencyIntentHash(change api.EmergencyChange) string {
	change.ExpectedCurrentVersion = 0
	return integrations.Hash(change)
}
func (s *Server) reconcileEmergencyRow(r *http.Request, p *principal, change api.EmergencyChange, reason string, apply bool, resolution string) (api.EmergencyReconcileResult, error) {
	out := api.EmergencyReconcileResult{RowID: change.RowID}
	var tid id.ID
	err := s.store.DB().QueryRow(r.Context(), `select task_id from emergency_snapshot_rows where id=$1 and snapshot_id=$2 and task_id=$3`, change.RowID, change.SnapshotID, change.TaskID).Scan(&tid)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, errNotFound
	}
	if err != nil {
		return out, err
	}
	rr := r.Clone(r.Context())
	rr.SetPathValue("taskId", tid.String())
	err = s.withTask(rr, p, apply || resolution != "", func(q store.Querier, period api.Period, t *store.TaskRow) error {
		snapshot, err := s.emergencySnapshotAccess(r, q, p, change.SnapshotID)
		if err != nil {
			return err
		}
		if snapshot.PeriodID != period.ID || !slices.Contains(t.SpaceIDs, snapshot.SpaceID) {
			return errNotFound
		}
		var data []byte
		if err = q.QueryRow(r.Context(), `select data from emergency_snapshot_rows where id=$1 and snapshot_id=$2 and task_id=$3`, change.RowID, snapshot.ID, t.ID).Scan(&data); err != nil {
			return err
		}
		var baseline api.EmergencyTask
		if err = json.Unmarshal(data, &baseline); err != nil {
			return err
		}
		current := emergencyTask(t)
		out.Snapshot = &snapshot
		out.Baseline = &baseline
		out.Current = &current
		var oldHash string
		var receipt []byte
		err = q.QueryRow(r.Context(), `select request_hash,data from emergency_reconciliations where row_id=$1`, change.RowID).Scan(&oldHash, &receipt)
		if err == nil {
			if oldHash != emergencyIntentHash(change) {
				out.Status = "conflict"
				out.Message = "Baris snapshot ini sudah direkonsiliasi dengan isi berbeda. Periksa riwayat dan buat snapshot baru."
				return nil
			}
			var saved api.EmergencyReconcileResult
			if err = json.Unmarshal(receipt, &saved); err != nil {
				return err
			}
			out.Status = saved.Status
			out.Message = "Perubahan/tinjauan ini sudah dicatat; tidak diproses dua kali."
			out.AppliedProgress = saved.AppliedProgress
			out.AppliedAt = saved.AppliedAt
			out.AppliedBy = saved.AppliedBy
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if resolution != "" {
			if !t.Permissions.Progress {
				return errForbidden
			}
			if change.ExpectedCurrentVersion != t.Version {
				return conflict("Tugas berubah sesudah pratinjau. Periksa ulang sebelum mencatat penyelesaian tinjauan.")
			}
			now := s.now().UTC()
			out.Status = resolution
			out.Message = "Tinjauan dicatat tanpa mengubah tugas atau mengesahkan bukti/SOP."
			out.AppliedAt = &now
			out.AppliedBy = p.User.DisplayName
			return s.saveEmergencyReconciliation(rr, q, p, t, change, reason, out, t.Task)
		}
		switch {
		case !t.Permissions.Progress:
			out.Status = "forbidden"
			out.Message = "Akun ini tidak lagi berwenang memperbarui progres tugas."
		case change.Progress != "todo" && change.Progress != "doing":
			out.Status = "manual"
			out.Message = "Penyerahan, kecocokan hasil, PJ/deadline dan perubahan lain harus ditinjau pada alur tugas yang sesuai."
		case t.Progress != api.TaskTodo && t.Progress != api.TaskDoing:
			out.Status = "manual"
			out.Message = "Tugas selesai/dibatalkan harus dibuka kembali melalui alur tugas yang berwenang."
		case t.Progress == change.Progress:
			out.Status = "unchanged"
			out.Message = "Progres server sudah sesuai; tidak ada pembaruan otomatis. Catatan tambahan tetap perlu ditinjau."
		case t.Version != baseline.Version || t.Progress != baseline.Progress || t.Assignee.ID != baseline.Assignee.ID || t.MatchStatus != baseline.MatchStatus:
			out.Status = "conflict"
			out.Message = "Tugas berubah setelah snapshot. Kedua versi ditampilkan; periksa tugas terkini sebelum memutuskan."
		case t.MatchStatus != api.MatchNone && t.MatchStatus != api.MatchRejected:
			out.Status = "manual"
			out.Message = "Tugas memiliki bukti/tinjauan hasil; data server dipertahankan."
		default:
			out.Status = "ready"
			out.Message = "Progres dapat direkonsiliasi setelah dipilih dan dikonfirmasi."
		}
		if !apply || out.Status != "ready" {
			return nil
		}
		if change.ExpectedCurrentVersion != t.Version {
			return conflict("Versi pratinjau berubah; periksa ulang sebelum menerapkan.")
		}
		before := t.Task
		t.Progress = change.Progress
		t.ResumeProgress = change.Progress
		t.Rank, err = s.store.TaskRank(r.Context(), q, t.PeriodID, t.ID, id.Nil, t.Progress)
		if err != nil {
			return err
		}
		if err = s.saveTaskMutation(rr, q, p, t, "task.emergency.progress", "Rekonsiliasi snapshot "+snapshot.ID.String()+": "+reason, before); err != nil {
			return err
		}
		now := s.now().UTC()
		out.Status = "applied"
		out.Message = "Progres diterapkan dengan waktu pencatatan server."
		out.AppliedProgress = t.Progress
		out.AppliedAt = &now
		out.AppliedBy = p.User.DisplayName
		current = emergencyTask(t)
		out.Current = &current
		return s.saveEmergencyReconciliation(rr, q, p, t, change, reason, out, before)
	})
	return out, err
}
func (s *Server) reconcileEmergency(w http.ResponseWriter, r *http.Request, p *principal) error {
	if p.admin() {
		return errForbidden
	}
	var req api.EmergencyReconcileRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if len(req.Changes) == 0 || len(req.Changes) > 100 {
		return badRequest("Pilih 1 sampai 100 baris perubahan per rekonsiliasi.")
	}
	apply := r.PathValue("action") == "apply"
	resolution := ""
	if r.PathValue("action") == "resolve" {
		if !slices.Contains([]string{"manual_reviewed", "ignored"}, req.Resolution) {
			return badRequest("Hasil tinjauan darurat tidak valid.")
		}
		resolution = req.Resolution
	}
	if r.PathValue("action") != "preview" && !apply && resolution == "" {
		return errNotFound
	}
	if apply || resolution != "" {
		var err error
		req.Reason, err = text(req.Reason, "alasan rekonsiliasi", 1, 4000)
		if err != nil {
			return err
		}
	}
	seen := map[id.ID]bool{}
	for _, change := range req.Changes {
		if change.RowID == id.Nil || change.SnapshotID == id.Nil || change.TaskID == id.Nil || seen[change.RowID] {
			return badRequest("Identitas baris/snapshot tidak valid atau berulang.")
		}
		seen[change.RowID] = true
		if len(change.Progress) > 64 || len(change.ReportedAt) > 100 || len(change.ReportedBy) > 200 || len(change.Note) > 4000 {
			return badRequest("Catatan perubahan darurat melebihi batas ukuran.")
		}
		if strings.TrimSpace(change.ReportedAt) == "" || strings.TrimSpace(change.ReportedBy) == "" {
			return badRequest("Catat pelapor dan waktu laporan darurat pada setiap baris yang diubah.")
		}
	}
	results := []api.EmergencyReconcileResult{}
	for _, change := range req.Changes {
		result, err := s.reconcileEmergencyRow(r, p, change, req.Reason, apply, resolution)
		if err != nil {
			var ae *apiError
			if errors.As(err, &ae) && ae.status != http.StatusUnauthorized {
				result.Status = "conflict"
				if ae.status == 403 {
					result.Status = "forbidden"
				}
				if ae.status == 404 {
					result.Status = "missing"
				}
				result.Message = ae.message
			} else if errors.Is(err, store.ErrNotFound) || errors.Is(err, pgx.ErrNoRows) {
				result.Status = "missing"
				result.Message = "Baris/tugas tidak tersedia pada akses saat ini."
			} else {
				return err
			}
		}
		results = append(results, result)
	}
	writeJSON(w, 200, api.List[api.EmergencyReconcileResult]{Items: results})
	return nil
}

func (s *Server) saveEmergencyReconciliation(r *http.Request, q store.Querier, p *principal, t *store.TaskRow, change api.EmergencyChange, reason string, out api.EmergencyReconcileResult, before any) error {
	data, err := json.Marshal(out)
	if err != nil {
		return err
	}
	if _, err = q.Exec(r.Context(), `insert into emergency_reconciliations(row_id,request_hash,actor_id,data,created_at) values($1,$2,$3,$4,$5)`, change.RowID, emergencyIntentHash(change), p.User.ID, data, s.now()); err != nil {
		return err
	}
	return s.record(r, q, p, store.AuditEvent{Source: "import", Action: "emergency.change." + out.Status, TargetType: "task", TargetID: &t.ID, PeriodID: &t.PeriodID, ScopeType: "division", ScopeID: &t.OwnerDivisionID, Before: before, After: map[string]any{"change": change, "result": out, "reason": reason, "reportedTimeIsNotVerifiedEvidence": true}})
}
