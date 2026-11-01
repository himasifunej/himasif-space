package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/jackc/pgx/v5"
	"net/http"
	"reflect"
	"time"
)

func (s *Server) offlineReceipt(r *http.Request, q store.Querier, p *principal, req api.OfflineMutation, out *api.OfflineResult) (bool, error) {
	data, _ := json.Marshal(req)
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	var oldHash string
	var response []byte
	err := q.QueryRow(r.Context(), `select request_hash,response from offline_mutations where user_id=$1 and mutation_id=$2`, p.User.ID, req.ID).Scan(&oldHash, &response)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if oldHash != hash {
		return false, conflict("Identitas perubahan sudah dipakai untuk isi yang berbeda.")
	}
	return true, json.Unmarshal(response, out)
}
func (s *Server) saveOfflineReceipt(r *http.Request, q store.Querier, p *principal, req api.OfflineMutation, out api.OfflineResult) error {
	data, _ := json.Marshal(req)
	sum := sha256.Sum256(data)
	response, err := json.Marshal(out)
	if err != nil {
		return err
	}
	_, err = q.Exec(r.Context(), `insert into offline_mutations(user_id,mutation_id,request_hash,response,created_at) values($1,$2,$3,$4,$5)`, p.User.ID, req.ID, hex.EncodeToString(sum[:]), response, s.now())
	return err
}
func offlineConflict(message string, current any) error {
	return &apiError{status: http.StatusConflict, code: api.CodeConflict, message: message, data: current}
}
func (s *Server) syncOffline(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.OfflineMutation
	if err := readJSON(r, &req); err != nil {
		return err
	}
	if req.ID == id.Nil || req.TargetID == id.Nil {
		return badRequest("Identitas perubahan dan tujuan wajib diisi.")
	}
	out := api.OfflineResult{MutationID: req.ID}
	var err error
	switch req.Kind {
	case "delivery.response":
		err = s.syncOfflineDelivery(r, p, req, &out)
	case "task.progress":
		if req.Progress != api.TaskTodo && req.Progress != api.TaskDoing {
			return badRequest("Antrean offline hanya mendukung progres belum/sedang dikerjakan. Penyerahan dan tinjauan memerlukan koneksi.")
		}
		r.SetPathValue("taskId", req.TargetID.String())
		err = s.withTask(r, p, true, func(q store.Querier, period api.Period, t *store.TaskRow) error {
			if !t.Permissions.Progress {
				return errForbidden
			}
			done, err := s.offlineReceipt(r, q, p, req, &out)
			if err != nil || done {
				return err
			}
			if t.Progress != req.BaseProgress && t.Progress != req.Progress {
				return offlineConflict("Progres tugas berubah di perangkat lain. Kedua versi dipertahankan.", t.Task)
			}
			if t.MatchStatus != api.MatchNone && t.MatchStatus != api.MatchRejected {
				return offlineConflict("Tugas sudah memiliki penyerahan atau tinjauan. Bukti server dipertahankan.", t.Task)
			}
			if t.Progress != req.Progress {
				before := t.Task
				t.Progress = req.Progress
				t.ResumeProgress = req.Progress
				rank, err := s.store.TaskRank(r.Context(), q, t.PeriodID, t.ID, id.Nil, t.Progress)
				if err != nil {
					return err
				}
				t.Rank = rank
				if err := s.saveTaskMutation(r, q, p, t, "task.offline.progress", "Sinkronisasi perubahan perangkat", before); err != nil {
					return err
				}
			}
			out.Task = &t.Task
			return s.saveOfflineReceipt(r, q, p, req, out)
		})
	case "note.content":
		title, e := validateNoteTitle(req.Title)
		if e != nil {
			return e
		}
		req.Title = title
		if e := api.ValidateNoteContent(req.Content); e != nil {
			return badRequest(e.Error())
		}
		r.SetPathValue("noteId", req.TargetID.String())
		err = s.withNote(r, p, true, func(q store.Querier, period api.Period, n *store.NoteRow) error {
			if !n.Permissions.Manage {
				return errForbidden
			}
			done, err := s.offlineReceipt(r, q, p, req, &out)
			if err != nil || done {
				return err
			}
			if n.Lock != nil && !n.Lock.HeldByCaller && s.now().Before(n.Lock.ExpiresAt) {
				return offlineConflict("Editor lain sedang memegang catatan. Draf offline tetap tersedia.", n.Note)
			}
			localTitleChanged := req.Title != req.BaseTitle
			remoteTitleChanged := n.Title != req.BaseTitle
			localContentChanged := !reflect.DeepEqual(req.Content, req.BaseContent)
			remoteContentChanged := !reflect.DeepEqual(n.Content, req.BaseContent)
			if localTitleChanged && remoteTitleChanged && req.Title != n.Title || localContentChanged && remoteContentChanged && !reflect.DeepEqual(req.Content, n.Content) {
				return offlineConflict("Bidang catatan yang sama berubah pada dua perangkat. Pilih isi secara eksplisit.", n.Note)
			}
			before := n.Note
			if localTitleChanged {
				n.Title = req.Title
			}
			if localContentChanged {
				n.Content = req.Content
			}
			if n.Title != before.Title || !reflect.DeepEqual(n.Content, before.Content) {
				n.Version++
				n.UpdatedAt = s.now().UTC()
				if err := s.store.SaveNote(r.Context(), q, *n); err != nil {
					return err
				}
				if err := s.noteEvent(r, q, p, n, "note.offline.sync", "Penggabungan perubahan perangkat", before, n.Note); err != nil {
					return err
				}
			}
			out.Note = &n.Note
			return s.saveOfflineReceipt(r, q, p, req, out)
		})
	default:
		return badRequest("Jenis perubahan offline tidak didukung.")
	}
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) syncOfflineDelivery(r *http.Request, p *principal, req api.OfflineMutation, out *api.OfflineResult) error {
	if req.Action != "seen" && req.Action != "snooze" {
		return badRequest("Respons offline tidak valid.")
	}
	if req.Action == "snooze" && (req.SnoozeUntil == nil || req.SnoozeUntil.After(s.now().Add(7*24*time.Hour))) {
		return badRequest("Waktu snooze tidak valid.")
	}
	return s.store.Tx(r.Context(), func(q store.Querier) error {
		if _, err := s.lockedAccount(r, q, p); err != nil {
			return err
		}
		done, err := s.offlineReceipt(r, q, p, req, out)
		if err != nil || done {
			return err
		}
		var d api.Delivery
		err = q.QueryRow(r.Context(), `select d.id,e.source_kind,e.source_id,e.space_id,d.channel,e.related_kind,e.related_id,e.scheduled_at from delivery_recipients d join delivery_events e on e.id=d.event_id where d.id=$1 and d.recipient_id=$2 and e.cancelled_at is null and d.status<>'cancelled' for update of d`, req.TargetID, p.User.ID).Scan(&d.ID, &d.SourceKind, &d.SourceID, &d.SpaceID, &d.Channel, &d.RelatedKind, &d.RelatedID, &d.ScheduledAt)
		if err != nil {
			return errNotFound
		}
		if d.Channel != "device" || !s.deliveryVisible(r.Context(), q, p, d) {
			return errNotFound
		}
		if req.Action == "seen" {
			_, err = q.Exec(r.Context(), `update delivery_recipients set seen_at=coalesce(seen_at,$2) where id=$1`, d.ID, s.now())
		} else {
			until := *req.SnoozeUntil
			if until.Before(d.ScheduledAt) {
				until = d.ScheduledAt
			}
			_, err = q.Exec(r.Context(), `update delivery_recipients set status='queued',available_at=$2,snoozed_until=$2,seen_at=null,accepted_at=null where id=$1`, d.ID, until)
		}
		if err != nil {
			return err
		}
		return s.saveOfflineReceipt(r, q, p, req, *out)
	})
}
