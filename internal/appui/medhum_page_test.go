package appui

import (
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

func TestMedhumPageHasCanvasTabsAndStages(t *testing.T) {
	h := newChromeHarness(t, nil)
	hum, med := h.api.divisions[1], h.api.divisions[0]
	caps := []string{api.CapSpaceRead, api.CapMembersRead, api.CapTasksManage}
	shared := api.Space{ID: id.New(), PeriodID: h.app.periodID, Name: "Medhum", Code: "medhum", Kind: api.SpaceShared, Divisions: []api.Division{hum, med}, Roles: []api.Access{{DivisionID: hum.ID, Capabilities: caps}}, Capabilities: caps}
	h.app.access.spaces = []api.Space{shared}
	h.app.selectSpace(shared.ID)
	h.app.chrome.tab = defaultSpaceTab(shared)
	h.app.router.Push(PathSpaces)
	now := time.Now()
	submitted := now.Add(-time.Hour)
	post := timeutil.DateOf(now.AddDate(0, 0, 3))
	humas := id.New()
	concept := api.MedhumContent{ID: id.New(), SpaceID: shared.ID, Title: "Poster open recruitment", PostingDate: &post, HumasPJID: humas, HumasPJName: "Humas A", CanEditCalendar: true}
	review := api.MedhumContent{ID: id.New(), SpaceID: shared.ID, Title: "Carousel wisuda", PostingDate: &post, HumasPJID: humas, HumasPJName: "Humas A", ConceptSubmittedAt: &submitted}
	h.app.medhum = medhumState{space: shared.ID, loaded: true, items: []api.MedhumContent{concept, review}, humans: []api.Member{{UserID: humas, DisplayName: "Humas A"}}}
	h.app.waContent = waContentUIState{space: shared.ID, user: h.app.user.ID, loaded: true, items: []api.WAContentItem{{ID: id.New(), Kind: "drive", State: "pending", ContentID: &review.ID}}}
	if s, _ := h.app.medhumStageOf(concept); s != medhumConcept {
		t.Fatalf("content without a concept is in stage %d", s)
	}
	if s, _ := h.app.medhumStageOf(review); s != medhumReview {
		t.Fatalf("content with pending Drive evidence is in stage %d", s)
	}
	for _, size := range [][2]int{{1440, 900}, {390, 844}} {
		h.tt.SetSize(size[0], size[1])
		h.app.medhum.tab = ""
		h.tt.Frame()
		for _, label := range []string{"Daftar konten", "Kalender posting", "Inbox broadcast · 1", "Sumber dan sinkron", "Aturan jam", "Konsep belum masuk", "Poster open recruitment", "Konten baru"} {
			h.want(label)
		}
		// Phones scroll the sub-tabs sideways; open them directly there.
		open := func(label, tab string) {
			if size[0] < mobileMaxWidth {
				h.app.medhum.tab = tab
			} else {
				h.click(label)
			}
			h.tt.Frame()
		}
		open("Kalender posting", medhumTabCal)
		h.want("Follow-up H-4")
		open("Aturan jam", medhumTabPolicy)
		h.want("Jam posting bawaan bila kosong")
		h.click("Jam posting bawaan: Belum ditetapkan")
		h.tt.Frame()
		h.want("Jam analog")
		h.click("Selesai")
		h.tt.Frame()
		if h.app.chrome.whens["medhum-posting-clock"].open {
			t.Fatal("clock dialog stayed open")
		}
		open("Daftar konten", medhumTabList)
		h.click("Buka konten Poster open recruitment")
		h.tt.Frame()
		h.want("Simpan konten")
		h.want("Jadwal posting")
		h.click("Tutup konten")
		h.tt.Frame()
		if h.app.medhum.draft != nil {
			t.Fatalf("content dialog stayed open at %v", size)
		}
	}
}
