package appui

import (
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// On a wide window Space saya uses the width (owner's rc.2 feedback: the
// table stopped at about 1080 and left the right empty): the table grows
// to pageMaxWidth with "Minggu ini" beside it from pageAsideMin, and the
// other list pages grow as far. A laptop keeps the single column.
// SPACE_UI_CAPTURE_DIR keeps the pages at 1920.
func TestWidePagesUseTheWidth(t *testing.T) {
	h := redesignHarness(t)
	h.app.access.spaces = append(h.app.access.spaces, api.Space{ID: id.New(), Name: "Medhum", Kind: api.SpaceShared})
	h.app.access.loading, h.app.access.err = false, ""
	now := time.Now()
	h.app.untuk = untukState{period: h.app.periodID, loaded: true, fetched: now, meetings: []api.Meeting{
		{ID: id.New(), SpaceID: h.app.access.spaces[0].ID, Title: "Gladi LPJ", StartsAt: now.Add(26 * time.Hour)},
		{ID: id.New(), SpaceID: h.app.access.spaces[1].ID, Title: "Koordinasi Medhum", StartsAt: now.Add(50 * time.Hour)},
		{ID: id.New(), SpaceID: h.app.access.spaces[0].ID, Title: "Rapat bulanan", StartsAt: now.Add(10 * 24 * time.Hour)},
	}}
	h.app.selectSpace(id.Nil)
	h.tt.SetSize(1920, 1080)
	h.app.navigate(PathSpaces)
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	h.want("Minggu ini")
	h.want("Koordinasi Medhum")
	if h.tt.HasText("Rapat bulanan") {
		t.Fatal("a meeting past 7 days in Minggu ini")
	}
	search, _ := h.tt.Find("Cari Space")
	week, _ := h.tt.Find("Minggu ini")
	if week.X+week.W < 1600 || search.W < 1000 {
		t.Fatalf("wide: search %+v, week %+v", search, week)
	}
	captureNativeUI(t, h, "wide-space-saya")
	h.click("Buka Medhum: Koordinasi Medhum")
	if h.app.selectedSpaceID() != h.app.access.spaces[1].ID {
		t.Fatal("the week's meeting did not open its Space")
	}

	h.app.selectSpace(id.Nil)
	h.app.navigate(PathUntukku)
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	captureNativeUI(t, h, "wide-untukku")

	h.tt.SetSize(1440, 900)
	h.app.navigate(PathSpaces)
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	if h.tt.HasText("Minggu ini") {
		t.Fatal("Minggu ini on a laptop")
	}
	if search, _ := h.tt.Find("Cari Space"); search.W < 1100 {
		t.Fatalf("laptop: search %+v", search)
	}
}
