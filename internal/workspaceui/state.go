// Package workspaceui presents a caller-owned task snapshot as a native board.
package workspaceui

import (
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Lane is a presentation group, independent of backend progress and evidence
// matching states. The caller maps its domain state to these groups.
type Lane uint8

const (
	Todo Lane = iota
	Doing
	Review
	Done
	Cancelled
)

func (l Lane) String() string {
	switch l {
	case Todo:
		return "Belum mulai"
	case Doing:
		return "Dikerjakan"
	case Review:
		return "Perlu tinjauan"
	case Done:
		return "Selesai"
	case Cancelled:
		return "Dibatalkan"
	default:
		return "Tidak diketahui"
	}
}

// Task holds display values; dates and names are already formatted by the
// caller. Evidence is the submitted output/matching information, not a design
// revision workflow. Snapshot order determines card order within each lane.
type Task struct {
	ID                                     id.ID
	Title, Description, Category, Assignee string
	Deadline, Posting, Space               string
	ExpectedOutput, Evidence               string
	Lane                                   Lane
	ReadOnly                               bool
	// Color is the card's accent, drawn on its left edge only.
	Color ui.Color
	// Labels are the task's tags as chips; when empty the card shows
	// Category as one chip.
	Labels []Label
	// Due is the short deadline on the card ("5 Nov"); Late marks it past.
	Due  string
	Late bool
	// Ref is a short reference shown on the card, such as "#3F2A".
	Ref           string
	AssigneeColor ui.Color
	// Checklist and attachment counts shown on the card's last line.
	ChecklistDone, ChecklistTotal, Attachments int
	// Remind, when set, is the card menu's "Ingatkan …" row (as
	// "Ingatkan Rani"); choosing it sends OnAction the action "remind".
	Remind string
	// Selectable lets the card be picked in select mode, for OnBulkCancel:
	// the caller sets it only on tasks the user may cancel.
	Selectable bool
}

// Label is one colored tag chip on a card.
type Label struct {
	Name  string
	Color ui.Color
}

// MoveRequest asks the caller to move a task before BeforeID in Destination.
// A nil BeforeID appends it. This intent is not a confirmation of persistence.
type MoveRequest struct {
	TaskID      id.ID
	Destination Lane
	BeforeID    id.ID
}

type taskDrag struct{ ID id.ID }

// State owns local presentation state only. A nil OnMove makes the board
// read-only. The caller persists moves and supplies a new snapshot on success.
type State struct {
	OnMove         func(MoveRequest)
	InspectorWidth float32
	LaneColors     [5]ui.Color
	// OnAction receives card menu actions: "edit", "duplicate", "remind",
	// "cancel".
	OnAction func(id.ID, string)
	// Details supplies a caller-owned inspector body, inside the right panel.
	Details func(*ui.Context, id.ID)
	// Subtitle replaces the line under the board title.
	Subtitle string
	// Toolbar builds controls at the right of the board title.
	Toolbar func(*ui.Context)
	// OnAdd, when set, shows "Tambah tugas" under each open lane.
	OnAdd func(Lane)
	// OnBulkCancel, when set, offers select mode ("Pilih", or Ctrl/Shift+
	// click on a card) and the selection bar's "Batalkan (N)", which hands
	// it the picked tasks in board order. The caller confirms.
	OnBulkCancel    func([]id.ID)
	selecting       bool
	picked          map[id.ID]bool
	tasks           []Task
	hasSnapshot     bool
	selected        id.ID
	menuID          id.ID
	menuOpen        bool
	boardScroll     ui.ScrollState
	boardWidth      float32
	lanes           [5]ui.ScrollState
	inspectorScroll ui.ScrollState
}

func New() *State { return &State{} }

// HasSnapshot reports whether a valid task snapshot was supplied, including
// an explicitly empty snapshot. A new or nil state has no confirmed task data.
func (s *State) HasSnapshot() bool { return s != nil && s.hasSnapshot }

// SetTasks validates then copies the whole snapshot. Invalid snapshots leave
// the previous view intact. No caller-owned slice is retained.
func (s *State) SetTasks(tasks []Task) error {
	seen := make(map[id.ID]bool, len(tasks))
	for _, task := range tasks {
		if task.ID == id.Nil {
			return fmt.Errorf("workspaceui: task has no identity")
		}
		if seen[task.ID] {
			return fmt.Errorf("workspaceui: duplicate task %s", task.ID)
		}
		if task.Lane > Cancelled {
			return fmt.Errorf("workspaceui: invalid lane %d", task.Lane)
		}
		seen[task.ID] = true
	}
	s.tasks = slices.Clone(tasks)
	s.hasSnapshot = true
	if !seen[s.selected] {
		s.Close()
	}
	if !seen[s.menuID] {
		s.menuOpen = false
		s.menuID = id.Nil
	}
	// A task gone, or no longer the user's to cancel, leaves the selection.
	for tid := range s.picked {
		if task, ok := s.task(tid); !ok || !task.Selectable {
			delete(s.picked, tid)
		}
	}
	return nil
}

// Selecting reports whether the board is in select mode.
func (s *State) Selecting() bool { return s != nil && s.selecting }

// SetSelecting enters or leaves select mode; leaving it clears the selection.
func (s *State) SetSelecting(on bool) {
	s.selecting = on
	if !on {
		s.picked = nil
	}
}

// Selected is the picked tasks, in board order.
func (s *State) Selected() []id.ID {
	var out []id.ID
	for _, task := range s.tasks {
		if s.picked[task.ID] {
			out = append(out, task.ID)
		}
	}
	return out
}

// Select picks exactly the selectable tasks among ids, entering select
// mode when any is picked.
func (s *State) Select(ids []id.ID) {
	s.picked = nil
	for _, tid := range ids {
		s.toggle(tid)
	}
	if len(s.picked) > 0 {
		s.selecting = true
	}
}

// toggle picks or unpicks a selectable task.
func (s *State) toggle(tid id.ID) bool {
	task, ok := s.task(tid)
	if !ok || !task.Selectable {
		return false
	}
	if s.picked == nil {
		s.picked = map[id.ID]bool{}
	}
	if s.picked[tid] {
		delete(s.picked, tid)
	} else {
		s.picked[tid] = true
	}
	return true
}

// anySelectable reports whether select mode has anything to pick.
func (s *State) anySelectable() bool {
	for _, task := range s.tasks {
		if task.Selectable {
			return true
		}
	}
	return false
}

func (s *State) Open(taskID id.ID) {
	if _, ok := s.task(taskID); ok {
		s.selected = taskID
		s.inspectorScroll.Y = 0
	}
}
func (s *State) Close()            { s.selected = id.Nil }
func (s *State) SelectedID() id.ID { return s.selected }

func (s *State) task(taskID id.ID) (Task, bool) {
	for _, task := range s.tasks {
		if task.ID == taskID {
			return task, true
		}
	}
	return Task{}, false
}

func (s *State) requestMove(taskID id.ID, lane Lane, before id.ID) {
	if s.OnMove == nil || lane > Cancelled || taskID == before {
		return
	}
	from, ok := s.task(taskID)
	if !ok || from.ReadOnly {
		return
	}
	if before != id.Nil {
		target, exists := s.task(before)
		if !exists || target.Lane != lane {
			return
		}
	}
	// Suppress operations which preserve the supplied snapshot's order.
	if from.Lane == lane {
		next := id.Nil
		found := false
		for _, task := range s.tasks {
			if task.Lane != lane {
				continue
			}
			if found {
				next = task.ID
				break
			}
			found = task.ID == taskID
		}
		if next == before {
			return
		}
	}
	s.OnMove(MoveRequest{taskID, lane, before})
}
