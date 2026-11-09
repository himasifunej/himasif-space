package appui

import (
	"time"

	"github.com/himasifunej/himasif-space/internal/id"
)

// PendingNoteFile is account-owned device data, not an uploaded attachment.
// UploadID survives restarts and a lost response; it must never change on retry.
type PendingNoteFile struct {
	UploadID  id.ID     `json:"uploadId"`
	NoteID    id.ID     `json:"noteId"`
	SpaceID   id.ID     `json:"spaceId"`
	Name      string    `json:"name"`
	SHA256    string    `json:"sha256"`
	SizeBytes int64     `json:"sizeBytes"`
	CreatedAt time.Time `json:"createdAt"`
	Data      []byte    `json:"-"`
}
type PendingFileStore interface {
	Load(server string, userID, noteID id.ID) (*PendingNoteFile, error)
	Save(server string, userID id.ID, file PendingNoteFile) error
	Delete(server string, userID, noteID, uploadID id.ID) error
}
