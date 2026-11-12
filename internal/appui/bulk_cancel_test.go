package appui

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

// bulkTaskAPI moves each task by its ID; fail answers some with an error.
type bulkTaskAPI struct {
	*taskTestAPI
	mu    sync.Mutex
	fail  map[id.ID]error
	moved []id.ID
	reqs  []api.TaskMoveRequest
}

func (f *bulkTaskAPI) MoveTask(_ context.Context, tid id.ID, r api.TaskMoveRequest) (api.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reqs = append(f.reqs, r)
	if err := f.fail[tid]; err != nil {
		return api.Task{}, err
	}
	for i := range f.items {
		if f.items[i].ID == tid {
			if f.items[i].Version != r.ExpectedVersion {
				return api.Task{}, &client.Error{Status: 409, Message: "Tugas sudah berubah."}
			}
			f.items[i].Progress, f.items[i].Version = r.Progress, f.items[i].Version+1
			f.moved = append(f.moved, tid)
			return f.items[i], nil
		}
	}
	return api.Task{}, &client.Error{Status: 404, Message: "Tidak ditemukan."}
}
func (f *bulkTaskAPI) SpaceTasks(context.Context, id.ID, api.TaskFilter) ([]api.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.items), nil
}

// bulkHarness is a board of ten tasks: nine the user may cancel, one
// (the last) they only work on.
func bulkHarness(t *testing.T) (*harness, *bulkTaskAPI) {
	t.Helper()
	h, _ := drawerHarness(t, nil)
	h.app.tasks.draft = nil
	f := &bulkTaskAPI{taskTestAPI: &taskTestAPI{fakeAPI: h.api}}
	sp := h.app.selectedSpace()
	me := api.PersonRef{ID: h.app.user.ID, DisplayName: h.app.user.DisplayName}
	titles := []string{"Poster Dies Natalis", "Caption Mingper", "Video Sempro", "Rekap Kas", "Undangan Rapat", "Banner Expo", "Notulen Rapat", "Dokumentasi PKKMB", "Feed Hari Guru", "Laporan Divisi"}
	for i, title := range titles {
		lane := []string{api.TaskTodo, api.TaskDoing, api.TaskDone}[i%3]
		perm := api.TaskPermissions{Manage: true, Progress: true}
		if i == len(titles)-1 {
			perm = api.TaskPermissions{Progress: true}
		}
		f.items = append(f.items, api.Task{ID: id.New(), Title: title, PeriodID: h.app.periodID, SpaceIDs: []id.ID{sp.ID}, Version: int64(i + 1), Progress: lane, MatchStatus: api.MatchNone, Assignee: me, Permissions: perm})
	}
	h.app.api = f
	h.tt.SetSize(1440, 1000)
	h.app.tasks.items = slices.Clone(f.items)
	h.app.projectTasks()
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	return h, f
}

// Picks nine, one reason for all, one fails: the summary names it and it
// stays picked; the others are cancelled with the same reason.
func TestBulkCancelPartialFailure(t *testing.T) {
	h, f := bulkHarness(t)
	h.want("Pilih")
	h.click("Pilih")
	h.want("Belum ada yang dipilih")
	h.want("Klik kartu untuk memilih · Esc untuk keluar")
	for _, task := range f.items[:9] {
		h.click("Pilih tugas " + task.Title)
	}
	// The task the user only works on is shown but cannot be picked.
	h.click(f.items[9].Title + ", tidak dapat kamu batalkan")
	if got := h.app.workspace.Selected(); len(got) != 9 || slices.Contains(got, f.items[9].ID) {
		t.Fatalf("selected %d: %v", len(got), got)
	}
	h.want("9 dipilih")
	captureNativeUI(t, h, "bulk-cancel-selecting")

	h.click("Batalkan (9)")
	h.tt.Frame()
	h.want("Batalkan 9 tugas?")
	for _, r := range cancelReasons {
		h.want(r)
	}
	h.click("Alasan: Diganti rencana")
	h.tt.Frame()
	captureNativeUI(t, h, "bulk-cancel-dialog")

	stale := f.items[3]
	f.fail = map[id.ID]error{stale.ID: &client.Error{Status: 409, Message: "Tugas sudah berubah."}}
	h.click("Batalkan 9 tugas")
	h.settle()
	h.settle()
	if len(f.reqs) != 9 || len(f.moved) != 8 {
		t.Fatalf("requests %d, moved %d", len(f.reqs), len(f.moved))
	}
	for _, r := range f.reqs {
		if r.Progress != api.TaskCancelled || r.Reason != "Diganti rencana" {
			t.Fatalf("request %+v", r)
		}
	}
	want := "8 dibatalkan, 1 gagal: “Rekap Kas” (sudah diubah orang lain, muat ulang lalu coba lagi)"
	if h.app.tasks.bulkFailed != want {
		t.Fatalf("summary %q", h.app.tasks.bulkFailed)
	}
	h.want(want)
	if !h.app.workspace.Selecting() || !slices.Equal(h.app.workspace.Selected(), []id.ID{stale.ID}) {
		t.Fatalf("after: selecting %v, selected %v", h.app.workspace.Selecting(), h.app.workspace.Selected())
	}
	h.want("1 dipilih")
	h.want("Batalkan (1)")
	captureNativeUI(t, h, "bulk-cancel-partial")
	if h.app.lastCancelReason != "Diganti rencana" {
		t.Fatalf("reason not remembered: %q", h.app.lastCancelReason)
	}

	// Retry the one left: the reason is picked already; it succeeds and
	// select mode closes.
	delete(f.fail, stale.ID)
	h.click("Batalkan (1)")
	h.tt.Frame()
	h.want("Batalkan 1 tugas?")
	h.click("Batalkan 1 tugas")
	h.settle()
	h.settle()
	if len(f.moved) != 9 || h.app.workspace.Selecting() {
		t.Fatalf("retry: moved %d, selecting %v", len(f.moved), h.app.workspace.Selecting())
	}
	h.dontWant("Klik kartu untuk memilih · Esc untuk keluar")
}

// Different causes are each named; nothing cancelled still says so.
func TestBulkCancelSummary(t *testing.T) {
	for _, c := range []struct {
		done   int
		failed []cancelFailure
		want   string
	}{
		{3, nil, "3 tugas dibatalkan."},
		{8, []cancelFailure{{title: "Poster", cause: "kamu tidak berhak membatalkannya"}}, "8 dibatalkan, 1 gagal: “Poster” (kamu tidak berhak membatalkannya)"},
		{0, []cancelFailure{{title: "A", cause: "x"}, {title: "B", cause: "y"}}, "0 dibatalkan, 2 gagal: “A” (x); “B” (y)"},
	} {
		if got := bulkCancelSummary(c.done, c.failed); got != c.want {
			t.Errorf("%d %v: %q, want %q", c.done, c.failed, got, c.want)
		}
	}
	for status, want := range map[int]string{409: "sudah diubah orang lain, muat ulang lalu coba lagi", 403: "kamu tidak berhak membatalkannya", 404: "tugas tidak ditemukan", 400: "Isi alasan"} {
		if got := bulkCancelCause(&client.Error{Status: status, Message: "Isi alasan."}); got != want {
			t.Errorf("%d: %q", status, got)
		}
	}
	if got := bulkCancelCause(fmt.Errorf("boom")); got != "Terjadi kesalahan. Coba lagi" {
		t.Errorf("plain error: %q", got)
	}
}

// Only tasks the user may cancel, as the server rules it, are pickable.
func TestTaskCancellable(t *testing.T) {
	both := api.TaskPermissions{Manage: true, Progress: true}
	for _, c := range []struct {
		t    api.Task
		want bool
	}{
		{api.Task{Progress: api.TaskTodo, Permissions: both}, true},
		{api.Task{Progress: api.TaskDone, Permissions: both}, true},
		{api.Task{Progress: api.TaskCancelled, Permissions: both}, false},
		{api.Task{Progress: api.TaskTodo, Permissions: api.TaskPermissions{Progress: true}}, false},
		{api.Task{Progress: api.TaskTodo, Permissions: api.TaskPermissions{Manage: true}}, false},
		{api.Task{Progress: api.TaskTodo, Permissions: both, LocalPending: true}, false},
	} {
		if got := taskCancellable(c.t); got != c.want {
			t.Errorf("%+v: %v", c.t, got)
		}
	}
}

// Esc closes the dialog first, then leaves select mode; the dialog's
// "Kembali" keeps the selection.
func TestBulkCancelEscape(t *testing.T) {
	h, f := bulkHarness(t)
	h.click("Pilih")
	h.click("Pilih tugas " + f.items[0].Title)
	h.click("Pilih tugas " + f.items[1].Title)
	h.click("Batalkan (2)")
	h.tt.Frame()
	h.want("Batalkan 2 tugas?")
	h.click("Kembali")
	h.tt.Frame()
	h.dontWant("Batalkan 2 tugas?")
	if len(h.app.workspace.Selected()) != 2 {
		t.Fatal("Kembali dropped the selection")
	}
	h.click("Batalkan (2)")
	h.tt.Frame()
	h.tt.Key(0, ui.KeyEscape)
	h.tt.Frame()
	if h.app.tasks.cancelOpen {
		t.Fatal("Esc did not close the dialog")
	}
	if !h.app.workspace.Selecting() || len(h.app.workspace.Selected()) != 2 {
		t.Fatal("Esc on the dialog also left select mode")
	}
	h.tt.Key(0, ui.KeyEscape)
	h.tt.Frame()
	if h.app.workspace.Selecting() {
		t.Fatal("Esc did not leave select mode")
	}
	if len(f.reqs) != 0 {
		t.Fatalf("sent %d", len(f.reqs))
	}
}
