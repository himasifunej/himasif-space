package server

import (
	"context"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/sop"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// Tahap 7a: the Evaluasi SOP chart counts, per rule of the policy, the
// Medhum contents of a quarter as evaluated with the worker's rules, and
// the findings' states; reading it writes nothing.
func TestSOPSummary(t *testing.T) {
	o := newTaskOrg(t)
	ctx := context.Background()
	_, humasPJ := o.join(o.admin, o.period.ID, "humas", "sop-humas@himasif.test", "Humas PJ")
	hk, _ := o.org.kadiv("humas", "sop-humas-kadiv@himasif.test")
	start := o.period.StartsOn
	end := start.AddDays(89)
	path := o.scope(o.period.ID, "mediatek") + "/sop"
	summaryPath := path + "/summary?quarter=Q1"

	// No policy yet: the chart cannot be drawn.
	if c := o.call("GET", summaryPath, o.kadiv, nil, nil); c == 200 {
		t.Fatal("summary without a policy")
	}
	policy := api.SOPPolicyRequest{DocumentVersion: "SOP Medhum v1", Enabled: true, Reason: "uji ringkasan",
		Settings: api.SOPSettings{Rules: []string{sop.DesignH2, sop.FinalH12}, Quarters: []api.SOPQuarter{{Label: "Q1", StartsOn: start, EndsOn: end}}}}
	var saved api.SOPPolicy
	if c := o.call("PUT", path+"/policy", o.kadiv, policy, &saved); c != 200 {
		t.Fatalf("policy = %d %+v", c, saved)
	}

	// One content posted inside the quarter, one outside.
	posting := start.AddDays(10)
	later := end.AddDays(5)
	contents := "/api/spaces/" + o.spaces["medhum"].String() + "/contents"
	for _, day := range []timeutil.Date{posting, later} {
		d := day
		if c := o.call("POST", contents, hk, api.MedhumCreateRequest{Title: "Konten " + d.String(), PostingDate: &d, HumasPJID: humasPJ.User.ID}, nil); c != 201 {
			t.Fatalf("content %s = %d", d, c)
		}
	}
	counts := func() (int, int) {
		var f, ob int
		if err := o.st.DB().QueryRow(ctx, `select (select count(*) from sop_findings),(select count(*) from sop_observations)`).Scan(&f, &ob); err != nil {
			t.Fatal(err)
		}
		return f, ob
	}
	f0, o0 := counts()

	var sum api.SOPSummary
	if c := o.call("GET", summaryPath, o.kadiv, nil, &sum); c != 200 {
		t.Fatalf("summary = %d %+v", c, sum)
	}
	if sum.Label != "Q1" || sum.Contents != 1 || !sum.PolicyEnabled || len(sum.Rules) != 2 {
		t.Fatalf("summary %+v", sum)
	}
	for _, r := range sum.Rules {
		if r.Evaluated != 1 || r.Compliant+r.Insufficient+r.NotApplicable > r.Evaluated {
			t.Fatalf("rule %s: %+v", r.Rule, r)
		}
	}
	if f1, o1 := counts(); f1 != f0 || o1 != o0 {
		t.Fatalf("summary wrote: findings %d→%d observations %d→%d", f0, f1, o0, o1)
	}

	// A finding's state is counted under its rule.
	fid, oid := id.New(), id.New()
	tx := []struct {
		sql  string
		args []any
	}{
		{`insert into sop_findings(id,period_id,division_id,content_id,rule,event_on,policy_id,created_at,updated_at,state)
			select $1,$2,$3,c.id,$4,$5,$6,now(),now(),'confirmed' from medhum_contents c where c.posting_date=$5 limit 1`,
			[]any{fid, o.period.ID, o.div["mediatek"].ID, sop.DesignH2, posting, saved.ID}},
		{`insert into sop_observations(id,finding_id,policy_version,document_version,fingerprint,measurement,data,occurred_at)
			values($1,$2,1,'SOP Medhum v1','uji','late','{}'::jsonb,now())`, []any{oid, fid}},
		{`update sop_findings set latest_observation_id=$1 where id=$2`, []any{oid, fid}},
	}
	for _, step := range tx {
		if _, err := o.st.DB().Exec(ctx, step.sql, step.args...); err != nil {
			t.Fatal(err)
		}
	}
	sum = api.SOPSummary{}
	o.call("GET", summaryPath, o.kadiv, nil, &sum)
	for _, r := range sum.Rules {
		if r.Rule == sop.DesignH2 && r.Confirmed != 1 {
			t.Fatalf("confirmed finding not counted: %+v", r)
		}
	}

	// Bounds and access.
	if c := o.call("GET", path+"/summary?from="+start.String()+"&to="+start.AddDays(150).String(), o.kadiv, nil, nil); c != 400 {
		t.Fatalf("150-day range = %d", c)
	}
	if c := o.call("GET", path+"/summary?quarter=Q9", o.kadiv, nil, nil); c != 400 {
		t.Fatalf("unknown quarter = %d", c)
	}
	if c, _ := o.errCode("GET", summaryPath, o.peer, nil); c != 403 && c != 404 {
		t.Fatalf("other division read the summary = %d", c)
	}
}
