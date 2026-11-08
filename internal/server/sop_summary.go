package server

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/sop"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// Bounds of a summary, so a read stays light (Tahap 7a).
const (
	sopSummaryMaxDays     = 120
	sopSummaryMaxContents = 2000
)

// sopRange resolves ?quarter=<label> (a quarter of the policy) or
// ?from=&to= into a date range inside the period.
func sopRange(r *http.Request, sc *scope, policy api.SOPPolicy, filter store.SOPFilter) (string, store.SOPFilter, error) {
	label := "Rentang pilihan"
	if quarter := r.URL.Query().Get("quarter"); quarter != "" {
		found := false
		for _, rangeQ := range policy.Settings.Quarters {
			if strings.EqualFold(rangeQ.Label, quarter) {
				from, to := rangeQ.StartsOn, rangeQ.EndsOn
				filter.From, filter.To = &from, &to
				label = rangeQ.Label
				found = true
				break
			}
		}
		if !found {
			return "", filter, badRequest("Quarter belum dikonfigurasi; tidak memakai batas kalender asumsi.")
		}
	}
	if filter.From == nil || filter.To == nil {
		return "", filter, badRequest("Tentukan awal/akhir rentang atau label quarter yang dikonfigurasi.")
	}
	if filter.From.Before(sc.period.StartsOn) || filter.To.Compare(sc.period.EndsOn) > 0 {
		return "", filter, badRequest("Rentang laporan berada di luar periode ini.")
	}
	return label, filter, nil
}

// sopSummary is the Evaluasi SOP chart: per rule of the policy, how many
// Medhum contents of the range were evaluated and on time ("Sesuai"), and
// the states of the findings. Compliant events are never stored (the worker
// records only late, over-limit or insufficient ones), so they are counted
// here by evaluating each content with the worker's own input and rules.
// It reads only: no finding, observation or notice is written.
func (s *Server) sopSummary(w http.ResponseWriter, r *http.Request, p *principal) error {
	filter, err := parseSOPFilter(r)
	if err != nil {
		return err
	}
	var out api.SOPSummary
	err = s.withSOPScope(r, p, "read", nil, func(q store.Querier, ac *sopAccess) error {
		sc := ac.scope
		ctx := r.Context()
		policy, err := s.store.SOPPolicy(ctx, q, sc.period.ID, sc.division.ID, store.LockNone)
		if err != nil {
			return err
		}
		label, f, err := sopRange(r, sc, policy, filter)
		if err != nil {
			return err
		}
		from, to := *f.From, *f.To
		if to.At(12, 0, time.UTC).Sub(from.At(12, 0, time.UTC)) > sopSummaryMaxDays*24*time.Hour {
			return badRequest("Rentang ringkasan paling panjang 120 hari.")
		}
		out = api.SOPSummary{Label: label, StartsOn: from, EndsOn: to, PolicyEnabled: policy.Enabled, Rules: []api.SOPRuleSummary{}}
		index := map[string]int{}
		for _, rule := range policy.Settings.Rules {
			index[rule] = len(out.Rules)
			out.Rules = append(out.Rules, api.SOPRuleSummary{Rule: rule})
		}
		loc, err := timeutil.LoadZone(sc.period.TimeZone)
		if err != nil {
			return err
		}
		contents, err := s.store.MedhumContents(ctx, q, policy.SpaceID)
		if err != nil {
			return err
		}
		evaluated := 0
		for _, c := range contents {
			var day timeutil.Date
			switch {
			case c.PostingAt != nil:
				day = timeutil.DateOf(c.PostingAt.In(loc))
			case c.PostingDate != nil:
				day = *c.PostingDate
			default:
				continue
			}
			if day.Before(from) || day.Compare(to) > 0 {
				continue
			}
			if evaluated++; evaluated > sopSummaryMaxContents {
				return badRequest("Terlalu banyak konten pada rentang ini; persempit rentangnya.")
			}
			in, pending, err := s.sopInput(ctx, q, sc.period, policy, c)
			if err != nil {
				return err
			}
			for _, rule := range policy.Settings.Rules {
				sum := &out.Rules[index[rule]]
				if !slices.Contains(sop.Rules, rule) {
					continue
				}
				o, err := sop.Evaluate(rule, in)
				if err != nil {
					return err
				}
				sum.Evaluated++
				switch {
				case pending:
					sum.Insufficient++
				case o.Measurement == "on_time" || o.Measurement == "within_limit":
					sum.Compliant++
				case o.Measurement == "insufficient":
					sum.Insufficient++
				case o.Measurement == "exempt" || o.Measurement == "not_applicable":
					sum.NotApplicable++
				}
			}
		}
		findings, err := s.store.SOPReportFindings(ctx, q, sc.period.ID, sc.division.ID, f)
		if err != nil {
			return err
		}
		for _, fd := range findings {
			// A finding of a rule no longer in the policy keeps its own row.
			i, ok := index[fd.Rule]
			if !ok {
				i = len(out.Rules)
				index[fd.Rule] = i
				out.Rules = append(out.Rules, api.SOPRuleSummary{Rule: fd.Rule})
			}
			sum := &out.Rules[i]
			switch fd.State {
			case "needs_review":
				sum.NeedsReview++
			case "confirmed":
				sum.Confirmed++
			case "dismissed":
				sum.Dismissed++
			case "corrected":
				sum.Corrected++
			}
		}
		out.Contents = evaluated
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}
