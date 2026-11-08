package appui

import (
	"context"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Tahap 6a: a task shows its number in the open Space, else in its
// division Space; a server without numbers keeps the short ID code.
func TestTaskCodeUsesSpaceNumber(t *testing.T) {
	h := newChromeHarness(t, nil)
	div := api.Space{ID: id.New(), Name: "Mediatek", Kind: api.SpaceDivision}
	medhum := api.Space{ID: id.New(), Name: "Medhum", Kind: api.SpaceShared}
	h.app.access.spaces = []api.Space{div, medhum}
	task := api.Task{ID: id.New(), Title: "Poster", SpaceIDs: []id.ID{medhum.ID, div.ID}, Numbers: map[string]int64{div.ID.String(): 12, medhum.ID.String(): 3}}
	h.app.selectSpace(medhum.ID)
	if got := h.app.taskCode(task); got != "#T3" {
		t.Fatalf("in Medhum = %q", got)
	}
	h.app.selectSpace(id.Nil)
	if got := h.app.taskCode(task); got != "#T12" {
		t.Fatalf("outside a Space = %q, want the division number", got)
	}
	old := api.Task{ID: task.ID}
	if got := h.app.taskCode(old); got != taskRef(task.ID) {
		t.Fatalf("old server = %q", got)
	}
	h.app.selectSpace(div.ID)
	for q, want := range map[string]bool{"#t12": true, "t12": true, "12": true, "012": true, "#t3": false, "1": false, "poster": true, "#t": false, "t1x": false} {
		if got := h.app.taskMatches(task, q); got != want {
			t.Errorf("match %q = %v, want %v", q, got, want)
		}
	}
}

// taskFilesTestAPI keeps a task's files as the server would.
type taskFilesTestAPI struct {
	*taskTestAPI
	uploads, removals []id.ID
}

func (f *taskFilesTestAPI) UploadTaskAttachment(_ context.Context, tid, aid id.ID, name string, data []byte) (api.TaskAttachment, error) {
	f.uploads = append(f.uploads, aid)
	a := api.TaskAttachment{ID: aid, Name: name, SizeBytes: int64(len(data)), ContentType: "application/pdf", AddedBy: api.PersonRef{ID: f.user.ID, DisplayName: f.user.DisplayName}, CreatedAt: time.Now()}
	f.items[0].Attachments = append(f.items[0].Attachments, a)
	return a, nil
}

func (f *taskFilesTestAPI) DownloadTaskAttachment(_ context.Context, _ id.ID, a api.TaskAttachment) ([]byte, error) {
	return []byte("%PDF-1.4 isi"), nil
}

func (f *taskFilesTestAPI) RemoveTaskAttachment(_ context.Context, _, aid id.ID) error {
	f.removals = append(f.removals, aid)
	f.items[0].Attachments = nil
	return nil
}

// Tahap 6b: the PJ attaches a file from the detail, downloads it to save,
// and removes it; the card counts it.
func TestTaskDetailAttachesDownloadsAndRemovesFiles(t *testing.T) {
	h := newChromeHarness(t, nil)
	h.tt.SetSize(1440, 900)
	testAuthorizedSpace(h.app, h.api.divisions[0])
	h.app.chrome.tab = "Board tugas"
	h.app.router.Push(PathSpaces)
	task := api.Task{ID: id.New(), Title: "Siapkan poster", Progress: api.TaskDoing, MatchStatus: api.MatchNone, SpaceIDs: []id.ID{h.app.selectedSpaceID()},
		Assignee: api.PersonRef{ID: h.app.user.ID, DisplayName: "PJ"}, Permissions: api.TaskPermissions{Progress: true, Comment: true}}
	f := &taskFilesTestAPI{taskTestAPI: &taskTestAPI{fakeAPI: h.api, items: []api.Task{task}}}
	h.app.api = f
	var saved string
	h.app.PickFile = func(_ int64, done func(SelectedFile, error)) {
		done(SelectedFile{Name: "brief.pdf", Data: []byte("%PDF-1.4 isi")}, nil)
	}
	h.app.ExportFile = func(name, _ string, _ []byte) (string, error) { saved = name; return "/unduhan/" + name, nil }
	h.app.tasks = tasksUIState{scope: h.app.selectedSpaceID(), loaded: true, items: []api.Task{task}}
	h.tt.Advance(400 * time.Millisecond)
	h.click("Tugas " + task.Title)
	h.want("Berkas · 0")
	h.click("Lampirkan berkas")
	drainUpdates(h) // the picker answers through a.update, outside inflight
	h.settle()
	h.tt.Frame()
	if len(f.uploads) != 1 {
		t.Fatalf("uploads %v; files=%+v err=%q", f.uploads, h.app.tasks.files, h.app.tasks.err)
	}
	h.want("brief.pdf")
	h.want("Berkas · 1")
	captureNativeUI(t, h, "task-files")
	h.dontWant("Belum terunggah: brief.pdf · 12 B")
	h.click("Unduh brief.pdf")
	h.settle()
	h.tt.Frame()
	h.click("Simpan berkas · brief.pdf")
	h.settle()
	if saved != "brief.pdf" {
		t.Fatalf("saved %q", saved)
	}
	h.click("Hapus brief.pdf")
	h.settle()
	h.tt.Frame()
	if len(f.removals) != 1 {
		t.Fatalf("removals %v", f.removals)
	}
	h.want("Berkas · 0")
}

func TestTaskFileSizeLabels(t *testing.T) {
	for n, want := range map[int64]string{820: "820 B", 14 * 1024: "14 KB", 3*1024*1024 + 200*1024: "3,2 MB"} {
		if got := taskFileSize(n); got != want {
			t.Errorf("%d = %q, want %q", n, got, want)
		}
	}
}

// drainUpdates runs the UI updates queued without an inflight call.
func drainUpdates(h *harness) {
	for {
		select {
		case fn := <-h.queue:
			fn()
		default:
			return
		}
	}
}
