package appui

import (
	"fmt"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/sop"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// drawerHarness opens the board with a new-task draft whose division has
// the given members loaded.
func drawerHarness(t *testing.T, members []api.Member) (*harness, *taskDraft) {
	t.Helper()
	h := newChromeHarness(t, nil)
	h.tt.SetSize(1440, 900)
	d := h.api.divisions[0]
	testAuthorizedSpace(h.app, d)
	h.app.chrome.tab = "Board tugas"
	h.app.router.Push(PathSpaces)
	h.app.tasks = tasksUIState{scope: h.app.selectedSpaceID(), loaded: true,
		members: map[id.ID][]api.Member{d.ID: members}, membersBusy: map[id.ID]bool{}, memberErrors: map[id.ID]string{}}
	draft := &taskDraft{owner: d.ID, shared: map[id.ID]bool{}}
	h.app.tasks.draft = draft
	h.tt.Advance(500 * time.Millisecond)
	h.tt.Frame()
	return h, draft
}

// Tahap 7b: canvas A's "Tugas baru": examples in Judul and Hasil, PJ as
// chips ("Saya" for the account), the rest folded under "Detail lainnya".
func TestNewTaskDrawerFollowsCanvas(t *testing.T) {
	aruna := api.Member{UserID: id.New(), DisplayName: "[DUMMY] Aruna Desain"}
	h, draft := drawerHarness(t, nil)
	me := api.Member{UserID: h.app.user.ID, DisplayName: "Ani Wulandari"}
	h.app.tasks.members[draft.owner] = []api.Member{aruna, me}
	h.tt.Frame()
	// (Placeholders are not texts to the tester; drawer-new-task.png shows them.)
	for _, want := range []string{"PJ *", "Aruna", "Saya", "Jadwal *", "Detail lainnya"} {
		h.want(want)
	}
	h.dontWant("Jadwal posting")
	h.click("PJ: [DUMMY] Aruna Desain")
	h.tt.Frame()
	if draft.assignee != aruna.UserID {
		t.Fatalf("chip did not choose the PJ: %v", draft.assignee)
	}
	h.click("Detail lainnya")
	h.tt.Frame()
	h.want("Deskripsi")
	h.want("Jadwal posting")
	captureNativeUI(t, h, "drawer-new-task")
}

// A filled field keeps "Detail lainnya" open, and a big division shows
// eight chips and "Lainnya…" with everyone.
func TestNewTaskDrawerFoldAndManyMembers(t *testing.T) {
	var members []api.Member
	for i := range 11 {
		members = append(members, api.Member{UserID: id.New(), DisplayName: fmt.Sprintf("Anggota %02d", i+1)})
	}
	h, draft := drawerHarness(t, members)
	draft.category = "Desain"
	h.tt.Frame()
	h.want("Label · pisahkan dengan koma")
	if _, ok := h.tt.Find("PJ: Anggota 09"); ok {
		t.Fatal("more than eight chips")
	}
	h.click("Lainnya…")
	h.tt.Frame()
	h.click("Anggota 02")
	h.tt.Frame()
	if draft.assignee != members[1].UserID || h.app.tasks.assigneeOpen {
		t.Fatal("Lainnya… did not choose the PJ and close")
	}
	// A PJ beyond the eighth (chosen from the scrolled list) takes the last chip.
	draft.assignee = members[10].UserID
	h.tt.Frame()
	if _, ok := h.tt.Find("PJ: Anggota 11"); !ok {
		t.Fatal("the chosen member beyond eight is not shown as a chip")
	}
}

// sopHarness shows Evaluasi SOP with a policy, two findings and a chart.
func sopHarness(t *testing.T, w, hgt int) *harness {
	t.Helper()
	h := newChromeHarness(t, nil)
	h.tt.SetSize(w, hgt)
	did := h.api.divisions[0].ID
	h.app.router.Push(PathEvaluasi)
	h.tt.Frame()
	start := h.app.period().StartsOn
	q := api.SOPQuarter{Label: "Kuartal 4", StartsOn: start, EndsOn: start.AddDays(89)}
	st := &h.app.evaluation
	st.user, st.period, st.division, st.scopesLoaded, st.tab = h.app.user.ID, h.app.periodID, did, true, "Temuan"
	st.scopes = []api.SOPAccess{{DivisionID: did, DivisionName: "Medhum", CanReview: true, CanConfigure: true}}
	st.policy = api.SOPPolicy{Version: 1, Enabled: true, Settings: api.SOPSettings{Rules: []string{sop.DesignH2, sop.FinalH12}, Quarters: []api.SOPQuarter{q}}}
	st.items = []api.SOPFinding{
		{ID: id.New(), ContentTitle: "Desain carousel diserahkan 6 Nov 23.40", EventOn: start.AddDays(36), Rule: sop.DesignH2, PersonName: "Aruna", State: "dismissed"},
		{ID: id.New(), ContentTitle: "Poster showcase lewat tenggat", EventOn: start.AddDays(35), Rule: sop.FinalH12, PersonName: "Aruna", State: "needs_review"},
	}
	st.summaryQuarter, st.summaryKey = q.Label, "fixed"
	st.summary = &api.SOPSummary{Label: q.Label, StartsOn: q.StartsOn, EndsOn: q.EndsOn, Contents: 6, Rules: []api.SOPRuleSummary{
		{Rule: sop.DesignH2, Evaluated: 6, Compliant: 3, Dismissed: 1, NeedsReview: 1, Confirmed: 1},
		{Rule: sop.FinalH12, Evaluated: 6, Compliant: 5, NeedsReview: 1},
	}}
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	return h
}

// Tahap 7a: canvas A's Evaluasi SOP: segments, table with rule names and
// state words, the chart per rule; Kebijakan holds the old tabs.
func TestSOPPageFollowsCanvas(t *testing.T) {
	h := sopHarness(t, 1440, 900)
	for _, want := range []string{"Evaluasi SOP Medhum", "Temuan", "Laporan kuartal", "Kebijakan", "TEMUAN", "ATURAN", "PIHAK", "STATUS",
		"Desain H-2", "Posting H-12", "Bukan pelanggaran", "Perlu tinjauan", "Kuartal 4 · per aturan", "Desain", "Posting", "Sesuai", "Pelanggaran disahkan", "Ekspor CSV"} {
		h.want(want)
	}
	if bars := sopBars(h.app.evaluation.summary); len(bars) != 2 || bars[0].group != "Desain" || bars[0].ok != 4 || bars[0].review != 1 || bars[0].violations != 1 {
		t.Fatalf("bars %+v", bars)
	}
	captureNativeUI(t, h, "sop-desktop")
	h.click("Kebijakan")
	h.tt.Frame()
	for _, tab := range []string{"Aturan", "Konteks", "Pengiriman", "Izin"} {
		h.want(tab)
	}
	if h.app.evaluation.tab != "Aturan" {
		t.Fatalf("Kebijakan opened %q", h.app.evaluation.tab)
	}
	h.click("Laporan kuartal")
	h.tt.Frame()
	if h.app.evaluation.tab != "Laporan" {
		t.Fatalf("Laporan kuartal opened %q", h.app.evaluation.tab)
	}
}

func TestSOPPagePhone(t *testing.T) {
	h := sopHarness(t, 390, 844)
	h.want("Desain carousel diserahkan 6 Nov 23.40")
	h.dontWant("ATURAN")
	captureNativeUI(t, h, "sop-phone")
}

func TestSOPHelpers(t *testing.T) {
	today := timeutil.Date{Year: 2026, Month: time.November, Day: 9}
	p := api.SOPPolicy{Settings: api.SOPSettings{Quarters: []api.SOPQuarter{
		{Label: "Q3", StartsOn: timeutil.Date{Year: 2026, Month: time.July, Day: 1}, EndsOn: timeutil.Date{Year: 2026, Month: time.September, Day: 30}},
		{Label: "Q4", StartsOn: timeutil.Date{Year: 2026, Month: time.October, Day: 1}, EndsOn: timeutil.Date{Year: 2026, Month: time.December, Day: 31}},
	}}}
	if got := sopCurrentQuarter(p, today); got != "Q4" {
		t.Fatalf("quarter %q", got)
	}
	for tab, seg := range map[string]string{"Temuan": "Temuan", "Laporan": "Laporan kuartal", "Izin": "Kebijakan", "Konteks": "Kebijakan"} {
		if sopSegment(tab) != seg {
			t.Errorf("%s → %s", tab, sopSegment(tab))
		}
	}
}
