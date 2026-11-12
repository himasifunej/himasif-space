package appui

import (
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// cancelHarness is the board with one task the user manages, its moves
// recorded.
func cancelHarness(t *testing.T) (*harness, *taskTestAPI, id.ID) {
	t.Helper()
	h, _ := drawerHarness(t, nil)
	h.app.tasks.draft = nil
	f := &taskTestAPI{fakeAPI: h.api, moves: make(chan api.TaskMoveRequest, 4)}
	sp := h.app.selectedSpace()
	f.items = []api.Task{{ID: id.New(), Title: "Poster Dies Natalis", PeriodID: h.app.periodID, SpaceIDs: []id.ID{sp.ID}, Version: 3, Progress: api.TaskTodo, MatchStatus: api.MatchNone,
		Assignee: api.PersonRef{ID: h.app.user.ID, DisplayName: h.app.user.DisplayName}, Permissions: api.TaskPermissions{Manage: true, Progress: true}}}
	h.app.api = f
	h.app.tasks.items = append([]api.Task(nil), f.items...)
	h.app.projectTasks()
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	return h, f, f.items[0].ID
}

func (h *harness) openCancel(tid id.ID) {
	h.app.taskAction(tid, "cancel")
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
}

// cancelEnabled is the dialog button's own condition.
func (h *harness) cancelEnabled() bool {
	return cancelReasonText(h.app.tasks.cancelChoice, h.app.tasks.cancelReason) != ""
}

// One tap on a reason is enough; the note joins it; Lainnya needs the note;
// the reason chosen is picked first next time.
func TestCancelReasonChips(t *testing.T) {
	h, f, tid := cancelHarness(t)
	h.openCancel(tid)
	for _, r := range cancelReasons {
		h.want(r)
	}
	h.want("Catatan (opsional)")
	if h.cancelEnabled() {
		t.Fatal("cancel enabled before a reason")
	}
	captureNativeUI(t, h, "cancel-reason-empty")
	h.click("Batalkan tugas") // disabled: nothing is sent
	select {
	case r := <-f.moves:
		t.Fatalf("cancelled without a reason: %+v", r)
	default:
	}
	h.click("Alasan: Digabung ke tugas lain")
	h.tt.Frame()
	if !h.cancelEnabled() {
		t.Fatal("a reason chip did not enable cancel")
	}
	h.app.tasks.cancelReason = "ke #T12"
	h.tt.Frame()
	h.want("Tercatat di riwayat: Digabung ke tugas lain: ke #T12")
	captureNativeUI(t, h, "cancel-reason-chosen")
	h.click("Batalkan tugas")
	select {
	case r := <-f.moves:
		if r.Progress != api.TaskCancelled || r.Reason != "Digabung ke tugas lain: ke #T12" || r.ExpectedVersion != 3 {
			t.Fatalf("move %+v", r)
		}
	case <-time.After(time.Second):
		t.Fatal("no move")
	}
	h.settle()

	// Next time the same reason is already picked, the note empty.
	f.items[0].Progress = api.TaskTodo
	h.app.tasks.items = append([]api.Task(nil), f.items...)
	h.openCancel(tid)
	if h.app.tasks.cancelChoice != "Digabung ke tugas lain" || h.app.tasks.cancelReason != "" || !h.cancelEnabled() {
		t.Fatalf("not remembered: %q %q", h.app.tasks.cancelChoice, h.app.tasks.cancelReason)
	}

	// Lainnya: the note becomes required.
	h.click("Alasan: Lainnya")
	h.tt.Frame()
	h.want("Tulis alasannya *")
	if h.cancelEnabled() {
		t.Fatal("Lainnya without a note enabled cancel")
	}
	captureNativeUI(t, h, "cancel-reason-other")
}

func TestCancelReasonText(t *testing.T) {
	for _, c := range []struct{ choice, note, want string }{
		{"", "apa saja", ""},
		{"Salah input / duplikat", "", "Salah input / duplikat"},
		{"Salah input / duplikat", "  dobel dengan #T3 ", "Salah input / duplikat: dobel dengan #T3"},
		{cancelOther, " ", ""},
		{cancelOther, "acara diundur", "Lainnya: acara diundur"},
	} {
		if got := cancelReasonText(c.choice, c.note); got != c.want {
			t.Errorf("%q %q: %q, want %q", c.choice, c.note, got, c.want)
		}
	}
}
