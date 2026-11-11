package server

import (
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
)

func (s *Server) waTaskCommand(r *http.Request, q store.Querier, p *principal, period api.Period, group api.WAGroup, in waInbound, command string, args []string) (whatsapp.SendRequest, error) {
	reply := whatsapp.SendRequest{}
	if len(args) == 0 {
		return reply, badRequest("Isi ID tugas yang ditampilkan oleh !tugas.")
	}
	tid, err := id.Parse(args[0])
	if err != nil {
		return reply, badRequest("ID tugas harus berupa ID lengkap dari !tugas.")
	}
	t, err := s.store.Task(r.Context(), q, tid, store.LockUpdate)
	if errors.Is(err, store.ErrNotFound) {
		return reply, errNotFound
	}
	if err != nil {
		return reply, err
	}
	if t.PeriodID != period.ID || !slices.Contains(t.SpaceIDs, group.SpaceID) {
		return reply, errNotFound
	}
	if err = s.taskAccess(r.Context(), q, p, period, &t); err != nil {
		return reply, err
	}
	switch command {
	case "cek":
		due := ""
		if t.DeadlineDate != nil {
			due = t.DeadlineDate.String()
		} else if t.DeadlineAt != nil {
			due = t.DeadlineAt.Format("2006-01-02T15:04:05Z07:00")
		}
		reply.Text = waText(r.Context(), "task.details", t.Title, t.Assignee.DisplayName, due, waProgress(waLanguage(r.Context()), t.Progress), waMatch(waLanguage(r.Context()), t.MatchStatus), t.ID)
		if link := s.cardLink("task", t.ID); link != "" {
			reply.Text += "\n" + waText(r.Context(), "card.link", link)
		}
	case "done":
		if !t.Permissions.Progress {
			return reply, errForbidden
		}
		if len(args) == 1 {
			if t.Progress == api.TaskCancelled {
				return reply, conflict("Tugas dibatalkan; buka kembali melalui Space dengan alasan.")
			}
			if t.MatchStatus != api.MatchVerified {
				return reply, badRequest("Bukti belum terverifikasi. Gunakan !done <ID> <tautan/catatan hasil> untuk mengajukan hasil; bukti tetap perlu ditinjau.")
			}
			if t.Progress != api.TaskDone {
				before := t.Task
				t.Progress = api.TaskDone
				t.Rank, err = s.store.TaskRank(r.Context(), q, t.PeriodID, t.ID, id.Nil, t.Progress)
				if err != nil {
					return reply, err
				}
				if err = s.saveTaskMutation(r, q, p, &t, "task.move", "Command WhatsApp; bukti sudah terverifikasi.", before); err != nil {
					return reply, err
				}
			}
			reply.Text = waText(r.Context(), "task.done")
		} else {
			result := strings.Join(args[1:], " ")
			req := api.TaskEvidenceRequest{ExpectedVersion: t.Version, Note: result}
			if len(result) > 10000 {
				return reply, badRequest("Catatan hasil terlalu panjang.")
			}
			if strings.HasPrefix(args[1], "http://") || strings.HasPrefix(args[1], "https://") {
				u, e := url.Parse(args[1])
				if e != nil || u.Hostname() == "" || u.User != nil || len(args[1]) > 4096 {
					return reply, badRequest("Tautan hasil tidak valid.")
				}
				req.URL = args[1]
				req.Note = strings.Join(args[2:], " ")
			}
			// Preserve the WA event time. Missing/implausible source timestamps use
			// receipt time, never the later time at which this worker was restarted.
			at := in.ReceivedAt
			if in.OccurredAt != nil && !in.OccurredAt.After(in.ReceivedAt.Add(5*time.Minute)) {
				at = *in.OccurredAt
			}
			var evidence api.TaskEvidence
			if err = s.applyTaskEvidence(r, q, p, &t, req, at, &evidence); err != nil {
				return reply, err
			}
			reply.Text = waText(r.Context(), "task.submitted")
		}
	case "bantu":
		if !t.Permissions.Help {
			return reply, errForbidden
		}
		if t.Progress == api.TaskDone || t.Progress == api.TaskCancelled {
			return reply, conflict("Tugas sudah selesai atau dibatalkan.")
		}
		message := strings.Join(args[1:], " ")
		if len(message) > 2000 {
			return reply, badRequest("Pesan bantuan terlalu panjang.")
		}
		help := api.TaskHelpRequest{ID: id.New(), User: taskPerson(p), Message: message, Status: "pending", CreatedAt: s.now()}
		if err = s.store.AddTaskHelp(r.Context(), q, t.ID, help); err != nil {
			if errors.Is(err, store.ErrConflict) {
				return reply, conflict("Pengajuan bantuan masih menunggu keputusan.")
			}
			return reply, err
		}
		if err = s.taskEvent(r, q, p, &t, "task.help.offer", "WhatsApp", nil, help); err != nil {
			return reply, err
		}
		managers, err := s.taskManagers(r, q, &t)
		if err != nil {
			return reply, err
		}
		if err = s.taskNotify(r, q, &t, "Pengajuan bantuan", t.Title, managers...); err != nil {
			return reply, err
		}
		reply.Text = waText(r.Context(), "help.submitted")
	case "delegasi":
		if !t.Permissions.Manage {
			return reply, errForbidden
		}
		if len(args) < 3 {
			return reply, badRequest("Gunakan !delegasi <ID tugas> <ID akun Space> <alasan>. ID akun menghindari tebakan dari nama/nomor.")
		}
		uid, e := id.Parse(args[1])
		if e != nil {
			return reply, badRequest("ID akun tujuan tidak valid.")
		}
		reason := strings.Join(args[2:], " ")
		if len(reason) > 2000 {
			return reply, badRequest("Alasan terlalu panjang.")
		}
		if err = s.lockTaskAccounts(r.Context(), q, []id.ID{uid, t.Assignee.ID}); err != nil {
			return reply, err
		}
		person, err := s.taskAssignee(r.Context(), q, period.ID, t.OwnerDivisionID, uid)
		if err != nil {
			return reply, err
		}
		req := api.TaskWriteRequest{Title: t.Title, AssigneeID: uid, DeadlineDate: t.DeadlineDate, DeadlineAt: t.DeadlineAt, PostingDate: t.PostingDate, PostingAt: t.PostingAt}
		if err = s.guardMedhumTaskUpdate(r, q, &t, req); err != nil {
			return reply, err
		}
		before := t.Task
		recipients := taskPeople(&t)
		t.Assignee = person
		if err = s.saveTaskMutation(r, q, p, &t, "task.update", reason, before); err != nil {
			return reply, err
		}
		recipients = append(recipients, uid)
		if err = s.taskNotify(r, q, &t, "Penugasan atau tenggat berubah", t.Title, recipients...); err != nil {
			return reply, err
		}
		reply.Text = waText(r.Context(), "task.delegated", person.DisplayName)
	}
	return reply, nil
}
