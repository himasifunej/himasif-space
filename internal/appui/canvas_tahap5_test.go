package appui

import (
	"context"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// reviewTestAPI records the evidence reviews a Tinjauan row sends.
type reviewTestAPI struct {
	*taskTestAPI
	reviews []api.TaskEvidenceReviewRequest
}

func (f *reviewTestAPI) ReviewTaskEvidence(_ context.Context, task, evidence id.ID, r api.TaskEvidenceReviewRequest) (api.TaskEvidence, error) {
	f.reviews = append(f.reviews, r)
	for i := range f.items {
		if f.items[i].ID == task {
			status := api.MatchRejected
			if r.Matched {
				status = api.MatchVerified
			}
			f.items[i].MatchStatus = status
			f.items[i].Evidence[0].MatchStatus = status
			f.items[i].Version++
		}
	}
	return api.TaskEvidence{ID: evidence}, nil
}

// reviewHarness opens Tinjauan of a Space with one pending result.
func reviewHarness(t *testing.T, review bool) (*harness, *reviewTestAPI) {
	t.Helper()
	h, base := taskHarness(t)
	f := &reviewTestAPI{taskTestAPI: base}
	h.app.api = f
	h.tt.SetSize(1440, 900)
	d := f.divisions[0]
	caps := []string{api.CapSpaceRead, api.CapMembersRead, api.CapTasksOwn, api.CapResultsReview}
	sp := api.Space{ID: id.New(), PeriodID: h.app.periodID, Name: d.Name, Kind: api.SpaceDivision, Divisions: []api.Division{d}, Roles: []api.Access{{DivisionID: d.ID, Roles: []string{api.RoleKadiv}, Capabilities: caps}}, Capabilities: caps}
	h.app.access.spaces = []api.Space{sp}
	f.items[0].Progress = api.TaskSubmitted
	f.items[0].MatchStatus = api.MatchPending
	f.items[0].Permissions.Review = review
	f.items[0].Evidence = []api.TaskEvidence{{ID: id.New(), SubmittedBy: api.PersonRef{ID: f.user.ID, DisplayName: "Aruna"}, URL: "https://contoh.test/poster", MatchStatus: api.MatchPending, SubmittedAt: time.Now()}}
	h.app.selectSpace(sp.ID)
	h.app.chrome.tab = "Tinjauan"
	h.app.router.Push(PathSpaces)
	h.tt.Frame()
	h.settle()
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	return h, f
}

func TestReviewRowMatchesWithDefaultReason(t *testing.T) {
	h, f := reviewHarness(t, true)
	h.want("Tinjauan hasil")
	captureNativeUI(t, h, "review-row")
	h.click("Cocok Poster")
	h.settle()
	h.tt.Frame()
	if len(f.reviews) != 1 || !f.reviews[0].Matched || f.reviews[0].Reason != reviewDefaultReason || f.reviews[0].ExpectedVersion != 1 {
		t.Fatalf("reviews = %+v", f.reviews)
	}
	h.want("Tidak ada bukti yang perlu ditinjau")
}

func TestReviewRowRejectNeedsReason(t *testing.T) {
	h, f := reviewHarness(t, true)
	h.click("Tidak cocok Poster")
	h.tt.Frame()
	h.want("Alasan tidak cocok *")
	captureNativeUI(t, h, "review-row-reject")
	h.click("Tandai tidak cocok")
	if len(f.reviews) != 0 {
		t.Fatalf("rejected without a reason: %+v", f.reviews)
	}
	h.click("Alasan tidak cocok *")
	h.tt.Type("Bukti untuk tugas lain")
	h.tt.Frame()
	h.click("Tandai tidak cocok")
	h.settle()
	h.tt.Frame()
	if len(f.reviews) != 1 || f.reviews[0].Matched || f.reviews[0].Reason != "Bukti untuk tugas lain" {
		t.Fatalf("reviews = %+v", f.reviews)
	}
}

func TestReviewRowHidesActionsWithoutPermission(t *testing.T) {
	h, _ := reviewHarness(t, false)
	h.want("Poster")
	for _, label := range []string{"Cocok Poster", "Tidak cocok Poster"} {
		if _, ok := h.tt.Find(label); ok {
			t.Fatalf("a reader without review rights got %q", label)
		}
	}
	if _, ok := h.tt.Find("Buka tugas Poster"); !ok {
		t.Fatal("no Buka tugas")
	}
}

func TestMembersTableShowsWhatsAppStatus(t *testing.T) {
	yes, no := true, false
	if memberWhatsApp(api.Member{WhatsAppLinked: &yes}) != "Tertaut" || memberWhatsApp(api.Member{WhatsAppLinked: &no}) != "Belum" || memberWhatsApp(api.Member{}) != "—" {
		t.Fatal("whatsapp cell labels")
	}
	h := newChromeHarness(t, nil)
	h.tt.SetSize(1440, 900)
	d := h.api.divisions[0]
	testAuthorizedSpace(h.app, d)
	caps := []string{api.CapSpaceRead, api.CapMembersRead}
	roles := []string{api.RoleMember}
	h.app.access.spaces[0].Roles = []api.Access{{DivisionID: d.ID, Roles: roles, Capabilities: caps}}
	h.app.access.members = memberManagementState{division: d.ID, loaded: true, access: api.Access{DivisionID: d.ID, Roles: roles, Capabilities: caps}, members: []api.Member{
		{UserID: id.New(), DisplayName: "Aruna Desain", Role: api.RoleMember, WhatsAppLinked: &yes},
		{UserID: id.New(), DisplayName: "Nara Tekno", Role: api.RoleMember, WhatsAppLinked: &no},
	}}
	h.app.router.Push(PathSpaces)
	h.app.chrome.tab = "Anggota"
	h.tt.Frame()
	h.want("Tertaut")
	h.want("Belum")
	h.tt.SetSize(390, 844)
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	h.want("— · — · WA tertaut")
}
