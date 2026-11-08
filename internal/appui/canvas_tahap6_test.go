package appui

import (
	"testing"

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
