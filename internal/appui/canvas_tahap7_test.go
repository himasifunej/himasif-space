package appui

import (
	"context"
	"fmt"
	"slices"
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
	h.want("Label")
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

// pingPagesAPI serves pages of pings and records sends.
type pingPagesAPI struct {
	*fakeAPI
	pages   [][]api.PingRecord
	calls   []string
	sent    []api.PingRequest
	filters []string
}

func (f *pingPagesAPI) Pings(_ context.Context, _ id.ID, before time.Time, _ int, filter string) ([]api.PingRecord, error) {
	f.calls = append(f.calls, before.Format(time.RFC3339))
	f.filters = append(f.filters, filter)
	if before.IsZero() {
		return f.pages[0], nil
	}
	return f.pages[1], nil
}

func (f *pingPagesAPI) SendPing(_ context.Context, _ id.ID, r api.PingRequest) error {
	f.sent = append(f.sent, r)
	return nil
}

func (f *pingPagesAPI) Deliveries(context.Context) ([]api.Delivery, error) { return nil, nil }
func (f *pingPagesAPI) RespondDelivery(context.Context, id.ID, api.DeliveryResponse) error {
	return nil
}

func pingRecord(msg string, at time.Time, seen bool) api.PingRecord {
	r := api.PingRecipientStatus{Recipient: api.PersonRef{ID: id.New(), DisplayName: "Aruna Desain"}, Channel: "device", Status: "accepted"}
	if seen {
		r.SeenAt = &at
	}
	return api.PingRecord{ID: id.New(), Message: msg, SentAt: at, TimeZone: "Asia/Jakarta", Sender: &api.PersonRef{ID: id.New(), DisplayName: "Demo Mediatek"}, Recipients: []api.PingRecipientStatus{r}}
}

// Tahap 7c: pages of 50 go back with before= the oldest ping shown and
// forward again; the filter is sent to the server; Ping ulang asks first
// and sends to who has not seen it.
func TestPingPagesFilterAndResend(t *testing.T) {
	h, _ := newSpaceTabsHarness(t, &fakePreferences{})
	now := time.Now()
	var first []api.PingRecord
	for i := range pingHistoryLimit {
		first = append(first, pingRecord(fmt.Sprintf("Ping %02d", i), now.Add(-time.Duration(i)*time.Minute), i > 0))
	}
	f := &pingPagesAPI{fakeAPI: h.api, pages: [][]api.PingRecord{first, {pingRecord("Ping lama", now.Add(-72*time.Hour), true)}}}
	h.app.api = f
	h.app.pingHistory = pingHistoryState{}
	h.app.chrome.tab = "Ping"
	h.tt.Frame()
	h.settle()
	h.tt.Frame()
	h.want("Halaman 1 · 50 per halaman")
	h.click("Lebih lama")
	h.settle()
	h.tt.Frame()
	if len(f.calls) != 2 || f.calls[1] != first[pingHistoryLimit-1].SentAt.Format(time.RFC3339) {
		t.Fatalf("older page cursor %v", f.calls)
	}
	h.want("Halaman 2 · 50 per halaman")
	h.want("Demo · Ping lama")
	h.click("Lebih baru")
	h.settle()
	h.tt.Frame()
	if f.calls[len(f.calls)-1] != (time.Time{}).Format(time.RFC3339) {
		t.Fatalf("newer page cursor %v", f.calls)
	}
	h.click("Belum dilihat")
	h.settle()
	if f.filters[len(f.filters)-1] != "unseen" {
		t.Fatalf("filter not sent: %v", f.filters)
	}
	h.tt.Frame()
	h.click("Ping ulang 1")
	h.tt.Frame()
	h.want("Ping ulang 1 orang?")
	h.click("Ya, kirim")
	h.settle()
	if len(f.sent) != 1 || f.sent[0].Message != "Ping 00" || len(f.sent[0].Recipients) != 1 || f.sent[0].Channels[0] != "device" {
		t.Fatalf("resend %+v", f.sent)
	}
	captureNativeUI(t, h, "ping-c-desktop")
	h.tt.SetSize(390, 844)
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	h.want("Lebih lama")
	captureNativeUI(t, h, "ping-c-phone")
}

func TestPingDayLabels(t *testing.T) {
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	for at, want := range map[time.Time]string{now: "Hari ini · Sen 9 Nov", now.Add(-24 * time.Hour): "Kemarin · Min 8 Nov", now.Add(-72 * time.Hour): "Jum 6 Nov"} {
		if got := pingDayLabel(at, now, "UTC"); got != want {
			t.Errorf("%v = %q, want %q", at, got, want)
		}
	}
}

// waHarness opens Bot WhatsApp for a kadiv with the given connection
// status, group and linked members.
func waHarness(t *testing.T, w, hgt int, status string, group bool, linked []bool) *harness {
	t.Helper()
	h, sp := newSpaceTabsHarness(t, &fakePreferences{})
	h.tt.SetSize(w, hgt)
	caps := append(slices.Clone(sp.Capabilities), api.CapWhatsAppManage, api.CapTasksManage)
	sp.Capabilities = caps
	sp.Roles[0].Capabilities = caps
	h.app.access.spaces = []api.Space{sp}
	did := sp.Divisions[0].ID
	h.app.chrome.tab = "Bot WhatsApp"
	h.tt.Frame()
	st := &h.app.whatsapp
	st.division, st.loaded, st.loading = did, true, false
	if status != "" {
		conn := &api.WAConnection{ID: id.New(), DisplayName: "Bot Mediatek", Language: "id", Status: status, Templates: map[string]string{"reminder": "{greeting}\n{body}"}}
		if group {
			conn.Groups = []api.WAGroup{{Name: "Mediatek 2025/2026", SpaceID: sp.ID}}
		}
		st.connection = conn
		st.session.Status = status
		st.templates[0] = waFriendly(conn.Templates["reminder"])
	}
	var members []api.Member
	for i, l := range linked {
		v := l
		members = append(members, api.Member{UserID: id.New(), DisplayName: []string{"Demo Mediatek", "Aruna Desain", "Nara Tekno", "Raka Video", "Sinta Pendamping"}[i], WhatsAppLinked: &v})
	}
	h.app.access.members = memberManagementState{division: did, loaded: true, members: members}
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	return h
}

// Tahap 7d: canvas Bot WhatsApp A: one status sentence for each state,
// four steps, the members' numbers; texts without "{body}" jargon.
func TestBotWhatsAppStates(t *testing.T) {
	h := waHarness(t, 1440, 900, "", false, nil)
	h.want("Bot belum terhubung")
	h.want("Langkah penyiapan")
	h.want("0 dari 4 selesai")
	h.want("Hubungkan WhatsApp")

	h = waHarness(t, 1440, 900, "connected", true, []bool{true, true, false, false, true})
	h.want("Bot aktif, 2 anggota belum menautkan")
	h.want("Nara dan Raka belum menerima pengingat pribadi.")
	h.want("3/5 tertaut")
	h.want("3 dari 5 anggota sudah menautkan.")
	h.want("Kirim cara menautkan")
	captureNativeUI(t, h, "wa-a-partial")

	h = waHarness(t, 1440, 900, "connected", true, []bool{true, true, true, true, true})
	h.want("Bot berjalan normal")
	h.want("4 dari 4 selesai")
	h.dontWant("Kirim cara menautkan")
}

func TestBotWhatsAppAdvancedAndPreview(t *testing.T) {
	h := waHarness(t, 1440, 1400, "connected", true, []bool{true, true, true, true, true})
	h.click("Pengaturan lanjutan")
	h.tt.Frame()
	for _, want := range []string{"Nama bot", "Sapaan", "Pengingat tenggat", "Balasan perintah di grup", "Ringkasan mingguan SOP"} {
		h.want(want)
	}
	if got := h.app.whatsapp.templates[0]; got != "[Sapaan]\n[Isi pesan]" {
		t.Fatalf("template shown as %q", got)
	}
	if got := h.app.waTemplatesForServer()["reminder"]; got != "{greeting}\n{body}" {
		t.Fatalf("template saved as %q", got)
	}
	// A preview shows the server's text as a WhatsApp message.
	pv := "reminder|tpl-reminder"
	key := fmt.Sprintf("%v", h.app.waPreviewDraft(pv))
	h.app.whatsapp.previews = map[string]*waPreviewState{pv: {asked: key, pending: key, pendingAt: time.Now().Add(-time.Second), text: "Halo Mediatek!\nPengingat: #T12 Poster open recruitment"}}
	h.app.whatsapp.previewOpen = pv
	h.tt.Frame()
	h.want("Contoh di grup WhatsApp")
	h.want("Halo Mediatek!\nPengingat: #T12 Poster open recruitment")
	h.want("Tutup pratinjau")
	captureNativeUI(t, h, "wa-a-preview")
}

func TestBotWhatsAppPhone(t *testing.T) {
	h := waHarness(t, 390, 844, "connected", true, []bool{true, false})
	h.want("Bot aktif, 1 anggota belum menautkan")
	h.want("Langkah penyiapan")
	captureNativeUI(t, h, "wa-a-phone")
}

func TestWAFriendlyTokens(t *testing.T) {
	in := "{greeting} {name}: {title}\n{body}"
	if got := waServerTemplate(waFriendly(in)); got != in {
		t.Fatalf("round trip %q", got)
	}
	if waFriendly(in) != "[Sapaan] [Nama bot]: [Judul]\n[Isi pesan]" {
		t.Fatalf("friendly %q", waFriendly(in))
	}
}

// The Tampilan popover offers canvas A's four accents; the full panel all
// five and a custom colour.
func TestAppearancePopoverFourAccents(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.click("Tampilan")
	for _, a := range quickAccents {
		if _, ok := h.tt.Find("Aksen " + a); !ok {
			t.Fatalf("popover lacks %s", a)
		}
	}
	if _, ok := h.tt.Find("Aksen lavender"); ok {
		t.Fatal("popover shows a fifth accent")
	}
	captureNativeUI(t, h, "appearance-popover")
	h.click("Semua pengaturan tampilan…")
	h.tt.Frame()
	if _, ok := h.tt.Find("Aksen lavender"); !ok {
		t.Fatal("full panel lacks lavender")
	}
}
