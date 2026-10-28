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
	OnMove func(MoveRequest)
	// Details supplies a caller-owned inspector body, inside the right panel.
	Details         func(*ui.Context, id.ID)
	tasks           []Task
	hasSnapshot     bool
	selected        id.ID
	menuID          id.ID
	menuOpen        bool
	boardScroll     ui.ScrollState
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
	return nil
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
