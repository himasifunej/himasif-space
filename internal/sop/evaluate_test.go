package sop

import (
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"testing"
	"time"
)

func date(s string) *timeutil.Date { d, _ := timeutil.ParseDate(s); return &d }
func TestDesignDeadlineAlreadyH2AndLateAttribution(t *testing.T) {
	in := Input{Zone: "Asia/Jakarta", Posting: Deadline{Date: date("2026-11-10")}, DesignDeadline: Deadline{Date: date("2026-11-08")}, DesignDelivered: Evidence{Date: date("2026-11-08"), Verified: true}}
	out, err := Evaluate(DesignH2, in)
	if err != nil || out.Measurement != "on_time" || out.Deadline.Date.String() != "2026-11-08" {
		t.Fatalf("H2 applied twice: %+v %v", out, err)
	}
	in.DesignDelivered.Date = date("2026-11-09")
	out, _ = Evaluate(DesignH2, in)
	if out.Measurement != "insufficient" {
		t.Fatal("unattributed lateness turned into fault")
	}
	in.ConceptMature = Evidence{Date: date("2026-11-03"), Verified: true}
	in.Followup = Evidence{Date: date("2026-11-06"), Verified: true}
	out, _ = Evaluate(DesignH2, in)
	if out.Measurement != "late" {
		t.Fatalf("missing preliminary late observation: %+v", out)
	}
}
func TestHourRuleRequiresRealClockAndVerifiedEvidence(t *testing.T) {
	at := time.Date(2026, 11, 9, 1, 0, 0, 0, time.UTC)
	in := Input{Zone: "Asia/Jakarta", Posting: Deadline{Date: date("2026-11-10")}, FinalApproved: Evidence{At: &at, Verified: true}}
	out, _ := Evaluate(FinalH12, in)
	if out.Measurement != "insufficient" {
		t.Fatal("invented posting clock")
	}
	clock := "12:00"
	in.PostingClock = &clock
	out, _ = Evaluate(FinalH12, in)
	if out.Measurement != "on_time" {
		t.Fatalf("verified timestamp ignored %+v", out)
	}
	in.FinalApproved.Verified = false
	out, _ = Evaluate(FinalH12, in)
	if out.Measurement != "insufficient" {
		t.Fatal("unverified evidence accepted")
	}
}
