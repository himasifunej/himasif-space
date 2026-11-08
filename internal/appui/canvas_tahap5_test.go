package appui

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
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

// pingHistoryTestAPI serves a Space's Riwayat ping.
type pingHistoryTestAPI struct {
	*fakeAPI
	pings []api.PingRecord
	calls int
}

func (f *pingHistoryTestAPI) Pings(context.Context, id.ID, time.Time, int, string) ([]api.PingRecord, error) {
	f.calls++
	return f.pings, nil
}

func TestPingTabShowsHistoryWithRecipientStatus(t *testing.T) {
	h, _ := newSpaceTabsHarness(t, &fakePreferences{})
	seen := time.Now()
	f := &pingHistoryTestAPI{fakeAPI: h.api, pings: []api.PingRecord{{
		ID: id.New(), Message: "Mohon perbarui progres tugas hari ini.", SentAt: time.Now(), TimeZone: "Asia/Jakarta",
		Sender: &api.PersonRef{ID: id.New(), DisplayName: "Demo Mediatek"}, RelatedKind: "task",
		Recipients: []api.PingRecipientStatus{
			{Recipient: api.PersonRef{ID: id.New(), DisplayName: "Aruna"}, Channel: "device", Status: "accepted", SeenAt: &seen},
			{Recipient: api.PersonRef{ID: id.New(), DisplayName: "Nara"}, Channel: "whatsapp", Status: "failed"},
		},
	}}}
	h.app.api = f
	h.app.chrome.tab = "Ping"
	h.tt.Frame()
	h.settle()
	h.tt.Frame()
	// Tahap 7c (canvas Ping C): one line per ping, seen count, Belum melihat.
	h.want("Demo · Mohon perbarui progres tugas hari ini.")
	h.want("Tugas · 1/2 dilihat · ada yang gagal")
	h.want("Belum melihat")
	h.want("1 ping belum dilihat · WA gagal")
	h.want("Ping ulang 1")
	h.dontWant("Riwayat ping belum tersedia")
	if f.calls != 1 {
		t.Fatalf("history loaded %d times", f.calls)
	}
	// Sending a ping reloads the history.
	h.app.pingHistory.loaded = false
	h.tt.Frame()
	h.settle()
	if f.calls != 2 {
		t.Fatalf("history not reloaded: %d", f.calls)
	}
}

func TestPingRecipientLabels(t *testing.T) {
	for _, tc := range []struct {
		r    api.PingRecipientStatus
		want string
	}{
		{api.PingRecipientStatus{Channel: "device", Status: "queued"}, "perangkat · antre"},
		{api.PingRecipientStatus{Channel: "whatsapp", Status: "accepted"}, "WA · terkirim"},
		{api.PingRecipientStatus{Channel: "whatsapp", Status: "unknown"}, "WA · tidak diketahui"},
	} {
		if got := pingRecipientLabel(tc.r); got != tc.want {
			t.Errorf("%+v = %q, want %q", tc.r, got, tc.want)
		}
	}
}

// uiPrefsTestAPI keeps an account's tab bars as the server would.
type uiPrefsTestAPI struct {
	*fakeAPI
	stored api.UIPreferences
	puts   []api.UIPreferences
	err    error
}

func (f *uiPrefsTestAPI) UIPreferences(context.Context) (api.UIPreferences, error) {
	return f.stored, f.err
}

func (f *uiPrefsTestAPI) SetUIPreferences(_ context.Context, p api.UIPreferences) (api.UIPreferences, error) {
	if f.err != nil {
		return api.UIPreferences{}, f.err
	}
	f.puts = append(f.puts, p)
	f.stored = p
	return p, nil
}

// useUIPrefs points the harness at f and forgets what the first frames
// learnt from the plain fake.
func useUIPrefs(h *harness, f *uiPrefsTestAPI) {
	h.app.api = f
	h.app.uiPrefs = uiPreferencesSync{}
	h.tt.Frame()
	h.settle()
	h.tt.Frame()
}

func TestSpaceTabsComeFromTheAccount(t *testing.T) {
	store := &fakePreferences{}
	h, sp := newSpaceTabsHarness(t, store)
	order := []string{"board", "beranda", "agenda", "catatan", "rapat", "tinjauan", "ping", "anggota", "bot"}
	f := &uiPrefsTestAPI{fakeAPI: h.api, stored: api.UIPreferences{SpaceTabs: map[string]api.SpaceTabPreference{sp.ID.String(): {Order: order, Hidden: []string{"rapat"}}}}}
	useUIPrefs(h, f)
	want := []string{"Board tugas", "Beranda", "Agenda", "Catatan", "Tinjauan", "Ping", "Anggota", "Bot WhatsApp"}
	if got := spaceTabTitles(h.app.spaceTabsVisible(&sp)); !slices.Equal(got, want) {
		t.Fatalf("tabs %q, want the account's %q", got, want)
	}
	if store.saves == 0 {
		t.Fatal("the account's bars were not kept on the device")
	}
	if len(f.puts) != 0 {
		t.Fatalf("reading the account's bars wrote them back: %+v", f.puts)
	}
	h.click("Atur tab")
	h.want("Urutan dan tab yang tampil disimpan untuk akunmu dan ikut ke perangkat lain. Beranda selalu tampil.")
	// A change on this device goes to the account.
	h.click("Tampilkan Rapat")
	h.tt.Frame()
	h.settle()
	if len(f.puts) != 1 || len(f.puts[0].SpaceTabs[sp.ID.String()].Hidden) != 0 || f.puts[0].SpaceTabs[sp.ID.String()].Order[0] != "board" {
		t.Fatalf("change not sent: %+v", f.puts)
	}
}

func TestSpaceTabsDeviceBarsSeedAnEmptyAccount(t *testing.T) {
	h, sp := newSpaceTabsHarness(t, &fakePreferences{})
	key := h.app.user.ID.String() + "/" + sp.ID.String()
	layouts := SpaceTabLayouts{key: {Hidden: []string{"ping"}}, id.New().String() + "/" + sp.ID.String(): {Hidden: []string{"rapat"}}}
	h.app.chrome.preferences.SpaceTabs = &layouts
	f := &uiPrefsTestAPI{fakeAPI: h.api, stored: api.UIPreferences{SpaceTabs: map[string]api.SpaceTabPreference{}}}
	useUIPrefs(h, f)
	if len(f.puts) != 1 || len(f.puts[0].SpaceTabs) != 1 || f.puts[0].SpaceTabs[sp.ID.String()].Hidden[0] != "ping" {
		t.Fatalf("device bars not sent once, or another account's sent: %+v", f.puts)
	}
}

func TestSpaceTabsStayOnDeviceWithOldServer(t *testing.T) {
	h, sp := newSpaceTabsHarness(t, &fakePreferences{})
	f := &uiPrefsTestAPI{fakeAPI: h.api, err: &client.Error{Status: 404, Code: "not_found", Message: "tidak ada"}}
	useUIPrefs(h, f)
	if !h.app.uiPrefs.off {
		t.Fatal("a server without the endpoint did not turn account sync off")
	}
	h.click("Atur tab")
	h.want("Urutan dan tab yang tampil disimpan untuk akunmu di perangkat ini. Beranda selalu tampil.")
	h.click("Tampilkan Ping")
	h.tt.Frame()
	h.settle()
	if got := spaceTabTitles(h.app.spaceTabsVisible(&sp)); slices.Contains(got, "Ping") {
		t.Fatalf("device change lost: %q", got)
	}
}
