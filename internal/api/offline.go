package api

import (
	"github.com/himasifunej/himasif-space/internal/id"
	"time"
)

// OfflineMutation is an immutable user intent, retained on the device until
// the server commits it. Base values permit a conservative three-way merge.
type OfflineMutation struct {
	ID           id.ID       `json:"id"`
	Kind         string      `json:"kind"`
	TargetID     id.ID       `json:"targetId"`
	BaseProgress string      `json:"baseProgress,omitempty"`
	Progress     string      `json:"progress,omitempty"`
	BaseTitle    string      `json:"baseTitle,omitempty"`
	Title        string      `json:"title,omitempty"`
	BaseContent  []NoteBlock `json:"baseContent,omitempty"`
	Content      []NoteBlock `json:"content,omitempty"`
	Action       string      `json:"action,omitempty"`
	SnoozeUntil  *time.Time  `json:"snoozeUntil,omitempty"`
}

type OfflineResult struct {
	MutationID id.ID `json:"mutationId"`
	Task       *Task `json:"task,omitempty"`
	Note       *Note `json:"note,omitempty"`
}
