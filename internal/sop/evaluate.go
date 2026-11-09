// Package sop compares explicit, versioned evidence. Its output is a
// preliminary observation; only an authorized human can confirm a violation.
package sop

import (
	"fmt"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"slices"
	"time"
)

const EngineVersion = "medhum-time-v1"

const (
	ConceptH7   = "7.5"
	DesignH2    = "7.11"
	Urgent24    = "7.12"
	FollowupH4  = "7.14"
	FinalH12    = "7.19"
	UrgentLimit = "7.6"
)

var Rules = []string{ConceptH7, DesignH2, Urgent24, FollowupH4, FinalH12, UrgentLimit}

type Evidence struct {
	At       *time.Time     `json:"at,omitempty"`
	Date     *timeutil.Date `json:"date,omitempty"`
	Verified bool           `json:"verified"`
	Source   string         `json:"source"`
	SourceID string         `json:"sourceId"`
	URL      string         `json:"url"`
	Note     string         `json:"note"`
}
type Deadline struct {
	At   *time.Time     `json:"at,omitempty"`
	Date *timeutil.Date `json:"date,omitempty"`
}
type Input struct {
	UrgentCountIssue      string   `json:"urgentCountIssue"`
	Zone                  string   `json:"zone"`
	Posting               Deadline `json:"posting"`
	DesignDeadline        Deadline `json:"designDeadline"`
	PostingClock          *string  `json:"postingClock,omitempty"`
	DesignClock           *string  `json:"designClock,omitempty"`
	ConceptMature         Evidence `json:"conceptMature"`
	ConceptDelivered      Evidence `json:"conceptDelivered"`
	DesignDelivered       Evidence `json:"designDelivered"`
	Followup              Evidence `json:"followup"`
	FinalApproved         Evidence `json:"finalApproved"`
	UrgentConfirmed       Evidence `json:"urgentConfirmed"`
	Urgent                *bool    `json:"urgent,omitempty"`
	SpecialAgreement      Evidence `json:"specialAgreement"`
	SpecialAgreementRules []string `json:"specialAgreementRules"`
	UrgentException       Evidence `json:"urgentException"`
	UrgentCount           *int     `json:"urgentCount,omitempty"`
	UrgentLimit           *int     `json:"urgentLimit,omitempty"`
}
type Observation struct {
	Rule        string     `json:"rule"`
	Measurement string     `json:"measurement"`
	Deadline    Deadline   `json:"deadline"`
	Evidence    []Evidence `json:"evidence"`
	Reason      string     `json:"reason"`
}

func Evaluate(rule string, in Input) (Observation, error) {
	out := Observation{Rule: rule, Measurement: "insufficient", Evidence: []Evidence{}, Reason: "Bukti atau parameter belum cukup; bukan keputusan pelanggaran."}
	loc, err := timeutil.LoadZone(in.Zone)
	if err != nil {
		return out, err
	}
	if (in.Posting.At != nil && in.Posting.Date != nil) || (in.DesignDeadline.At != nil && in.DesignDeadline.Date != nil) {
		return out, fmt.Errorf("jadwal memiliki dua presisi")
	}
	if in.SpecialAgreement.Verified && in.SpecialAgreement.Note != "" && slices.Contains(in.SpecialAgreementRules, rule) {
		out.Measurement = "exempt"
		out.Evidence = []Evidence{in.SpecialAgreement}
		out.Reason = "Kesepakatan khusus tercatat; penerapannya tetap dapat ditinjau manusia."
		return out, nil
	}
	var evidence Evidence
	switch rule {
	case ConceptH7:
		out.Deadline = daysBefore(in.Posting, 7, loc)
		// A delivered concept is not proof of a mature request by itself.
		evidence = in.ConceptMature
		out.Evidence = append(out.Evidence, in.ConceptDelivered)
	case DesignH2:
		// The content model has already derived H-2. Never subtract it again.
		out.Deadline = withClock(in.DesignDeadline, in.DesignClock, loc)
		evidence = in.DesignDelivered
	case FollowupH4:
		out.Deadline = daysBefore(in.Posting, 4, loc)
		evidence = in.Followup
	case FinalH12, Urgent24:
		if rule == Urgent24 {
			if in.Urgent == nil {
				out.Reason = "Klasifikasi dadakan belum dikonfirmasi."
				return out, nil
			}
			if !*in.Urgent {
				out.Measurement = "not_applicable"
				out.Reason = "Konten tidak diklasifikasikan dadakan."
				return out, nil
			}
			evidence = in.UrgentConfirmed
		} else {
			evidence = in.FinalApproved
		}
		post := withClock(in.Posting, in.PostingClock, loc)
		if post.At == nil {
			out.Reason = "Jadwal posting hanya tanggal; jam belum ditetapkan untuk aturan berbasis jam."
			return out, nil
		}
		hours := 12
		if rule == Urgent24 {
			hours = 24
		}
		at := post.At.Add(-time.Duration(hours) * time.Hour)
		out.Deadline.At = &at
	case UrgentLimit:
		if in.Urgent == nil {
			out.Reason = "Klasifikasi dadakan belum dikonfirmasi."
			return out, nil
		}
		if !*in.Urgent {
			out.Measurement = "not_applicable"
			out.Reason = "Konten tidak diklasifikasikan dadakan."
			return out, nil
		}
		if in.UrgentException.Verified && in.UrgentException.Note != "" {
			out.Measurement = "exempt"
			out.Evidence = []Evidence{in.UrgentException}
			out.Reason = "Pengecualian dadakan dikonfirmasi manusia dengan bukti."
			return out, nil
		}
		if in.UrgentCount == nil || in.UrgentLimit == nil {
			out.Reason = "Rentang penghitungan atau batas dadakan belum ditetapkan."
			if in.UrgentCountIssue != "" {
				out.Reason = in.UrgentCountIssue
			}
			return out, nil
		}
		out.Measurement = "within_limit"
		if *in.UrgentCount > *in.UrgentLimit {
			out.Measurement = "exceeds_limit"
		}
		out.Reason = fmt.Sprintf("Tercatat %d konten dadakan nonpengecualian dalam rentang konfigurasi, batas %d; atribusi tetap perlu tinjauan.", *in.UrgentCount, *in.UrgentLimit)
		return out, nil
	default:
		return out, fmt.Errorf("aturan SOP tidak didukung")
	}
	out.Evidence = append(out.Evidence, evidence)
	out.Measurement = compare(out.Deadline, evidence, loc)
	if rule == DesignH2 && out.Measurement == "late" {
		mature := compare(daysBefore(in.Posting, 7, loc), in.ConceptMature, loc)
		followup := compare(daysBefore(in.Posting, 4, loc), in.Followup, loc)
		out.Evidence = append(out.Evidence, in.ConceptMature, in.Followup)
		if mature != "on_time" || followup != "on_time" {
			out.Measurement = "insufficient"
			out.Reason = "Waktu desain melewati tenggat, tetapi kematangan konsep/follow-up/pengecualian belum membuktikan atribusi SOP. Perlu tinjauan."
			return out, nil
		}
	}
	switch out.Measurement {
	case "on_time":
		out.Reason = "Bukti kejadian yang terverifikasi tercatat sebelum atau pada tenggat; waktu klik selesai tidak dipakai sebagai waktu penyerahan."
	case "late":
		out.Reason = "Waktu kejadian yang dapat dibandingkan melewati tenggat; ini temuan awal yang perlu tinjauan, bukan pelanggaran final."
	default:
		out.Reason = "Bukti belum terverifikasi, waktu tidak cukup presisi, atau tenggat belum tersedia; tidak menyimpulkan pelanggaran."
	}
	return out, nil
}

func daysBefore(post Deadline, days int, loc *time.Location) Deadline {
	if post.At != nil {
		at := post.At.In(loc).AddDate(0, 0, -days)
		return Deadline{At: &at}
	}
	if post.Date != nil {
		date := post.Date.AddDays(-days)
		return Deadline{Date: &date}
	}
	return Deadline{}
}
func withClock(d Deadline, clock *string, loc *time.Location) Deadline {
	if d.At != nil || d.Date == nil || clock == nil {
		return d
	}
	parsed, err := time.Parse("15:04", *clock)
	if err != nil || len(*clock) != 5 {
		return d
	}
	at := d.Date.At(parsed.Hour(), parsed.Minute(), loc)
	return Deadline{At: &at}
}
func compare(deadline Deadline, evidence Evidence, loc *time.Location) string {
	if !evidence.Verified || (evidence.At == nil) == (evidence.Date == nil) {
		return "insufficient"
	}
	if deadline.At != nil {
		if evidence.At != nil {
			if evidence.At.After(*deadline.At) {
				return "late"
			}
			return "on_time"
		}
		day := timeutil.DateOf(deadline.At.In(loc))
		switch evidence.Date.Compare(day) {
		case -1:
			return "on_time"
		case 1:
			return "late"
		default:
			return "insufficient"
		}
	}
	if deadline.Date == nil {
		return "insufficient"
	}
	date := evidence.Date
	if date == nil {
		day := timeutil.DateOf(evidence.At.In(loc))
		date = &day
	}
	if date.Compare(*deadline.Date) > 0 {
		return "late"
	}
	return "on_time"
}
