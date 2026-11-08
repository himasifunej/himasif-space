package appui

import (
	"fmt"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
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
