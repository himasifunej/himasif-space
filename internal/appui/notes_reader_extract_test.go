package appui

import (
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Owner decision 9 Nov: a note's reader may turn one whole block into a
// personal reminder, and into a task only with the right to create tasks
// in that Space; what a reader makes is never linked to the note (the
// server allows note links only to the note's managers).
func TestNoteReaderMakesReminderFromBlockWithoutLink(t *testing.T) {
	h, f := reminderHarness(t)
	a := h.app
	division := id.New()
	space := api.Space{ID: id.New(), PeriodID: a.periodID, Roles: []api.Access{{DivisionID: division, Capabilities: []string{api.CapNotesRead}}}}
	a.access.spaces, a.access.selected = []api.Space{space}, space.ID
	a.notes.space = space.ID
	a.notes.note = &api.Note{ID: id.New(), SpaceID: space.ID, OwnerDivisionID: division, Title: "Rapat"}
	a.notes.selected = a.notes.note.ID

	if a.noteCanExtractTask() {
		t.Fatal("a reader without task rights may make a task")
	}
	a.beginNoteConversion(api.NoteLinkTask, "Siapkan poster")
	if a.notes.conversion != nil {
		t.Fatal("a reader without task rights opened the task form")
	}
	a.beginNoteConversion(api.NoteLinkReminder, "Siapkan poster")
	c := a.notes.conversion
	if c == nil || !c.reader {
		t.Fatal("a reader could not start a reminder from a block")
	}
	before := len(f.createRequests)
	c.title, c.date = "Siapkan poster", "2026-11-20"
	a.saveNoteConversion()
	settleReminders(t, h)
	if len(f.createRequests) != before+1 || f.createRequests[len(f.createRequests)-1].Notes != "Siapkan poster" {
		t.Fatalf("reminder not created: %+v err=%q", f.createRequests, c.err)
	}
	if c.target == id.Nil || c.linkPending || len(a.notes.pendingLinks) != 0 {
		t.Fatalf("reader conversion target=%v linkPending=%v pending=%v", c.target, c.linkPending, a.notes.pendingLinks)
	}
	// Saving again does not create a second reminder.
	a.saveNoteConversion()
	settleReminders(t, h)
	if len(f.createRequests) != before+1 {
		t.Fatal("saving again made a second reminder")
	}

	// With the right to create tasks the reader may start a task too.
	a.notes.conversion = nil
	a.access.spaces[0].Roles[0].Capabilities = append(a.access.spaces[0].Roles[0].Capabilities, api.CapTasksManage)
	if !a.noteCanExtractTask() {
		t.Fatal("task rights not recognised")
	}
	a.beginNoteConversion(api.NoteLinkTask, "Siapkan poster")
	if a.notes.conversion == nil || a.notes.conversion.kind != api.NoteLinkTask || !a.notes.conversion.reader {
		t.Fatal("a reader with task rights could not start a task")
	}
	settleReminders(t, h)
}
