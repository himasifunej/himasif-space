package appui

import (
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// LocalNoteDraft is a device recovery copy, not a committed cloud note.
// Storage is separated by account and note; no authentication data belongs here.
type LocalNoteDraft struct {
	SpaceID      id.ID                 `json:"spaceId"`
	NoteID       id.ID                 `json:"noteId"`
	Title        string                `json:"title"`
	Content      []api.NoteBlock       `json:"content"`
	BaseVersion  int64                 `json:"baseVersion"`
	SavedAt      time.Time             `json:"savedAt"`
	PendingLinks []api.NoteLinkRequest `json:"pendingLinks,omitempty"`
}

type NoteDraftStore interface {
	Load(userID, noteID id.ID) (*LocalNoteDraft, error)
	Save(userID, noteID id.ID, draft LocalNoteDraft) error
	Delete(userID, noteID id.ID) error
}
