package server

import (
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
	"net/http"
	"strings"
)

func (s *Server) waReportCommand(r *http.Request, q store.Querier, p *principal, period api.Period, group api.WAGroup, command string, args []string) (whatsapp.SendRequest, error) {
	reply := whatsapp.SendRequest{}
	if command == "summary" {
		var active, pending, done int
		err := q.QueryRow(r.Context(), `select count(*) filter(where t.progress in ('todo','doing')),count(*) filter(where t.match_status='pending'),count(*) filter(where t.progress='done') from tasks t join task_spaces ts on ts.task_id=t.id where ts.space_id=$1`, group.SpaceID).Scan(&active, &pending, &done)
		if err != nil {
			return reply, err
		}
		reply.Text = waText(r.Context(), "tasks.summary", active, pending, done)
		return reply, nil
	}
	sp, err := s.store.SpaceByID(r.Context(), q, group.SpaceID)
	if err != nil {
		return reply, err
	}
	if sp.Kind != api.SpaceDivision || sp.Code == "medhum" {
		return reply, badRequest("Rekap/keputusan SOP tidak diumumkan ke Medhum. Gunakan grup divisi atau laporan di Space sesuai izin.")
	}
	access, err := s.loadSOPAccess(r, q, p, store.LockNone)
	if err != nil {
		return reply, err
	}
	if !access.read {
		return reply, errForbidden
	}
	policy, err := s.store.SOPPolicy(r.Context(), q, period.ID, access.scope.division.ID, store.LockNone)
	if err != nil {
		return reply, err
	}
	loc, err := timeutil.LoadZone(period.TimeZone)
	if err != nil {
		return reply, err
	}
	to := timeutil.DateOf(s.now().In(loc))
	from := to.AddDays(-6)
	label := waText(r.Context(), "report.week")
	if command == "eval" {
		quarter := strings.TrimSpace(strings.Join(args, " "))
		found := false
		for _, rangeQ := range policy.Settings.Quarters {
			if strings.EqualFold(rangeQ.Label, quarter) {
				from, to = rangeQ.StartsOn, rangeQ.EndsOn
				label = rangeQ.Label
				found = true
				break
			}
		}
		if !found {
			return reply, badRequest("Gunakan !eval <label quarter yang sudah dikonfigurasi>. Tidak ada batas quarter asumsi.")
		}
	} else {
		if from.Before(period.StartsOn) {
			from = period.StartsOn
		}
		if to.Compare(period.EndsOn) > 0 {
			to = period.EndsOn
		}
		if to.Before(from) {
			return reply, badRequest("Rentang rekap tidak berada dalam periode ini.")
		}
	}
	report, err := s.makeSOPReport(r.Context(), q, access.scope, policy, label, store.SOPFilter{From: &from, To: &to})
	if err != nil {
		return reply, err
	}
	reply.Text = sopReportTextLanguage(report, waLanguage(r.Context()))
	return reply, nil
}
