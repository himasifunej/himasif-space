package appui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

type SelectedFile struct {
	Name string
	Data []byte
}
type NotesFilesAPI interface {
	UploadNoteAttachment(context.Context, id.ID, id.ID, string, []byte) (api.NoteAttachment, error)
	DownloadNoteAttachment(context.Context, id.ID, api.NoteAttachment) ([]byte, error)
}
type noteFileUpload struct {
	PendingNoteFile
	Saved bool
}
type noteFileDownload struct {
	Attachment api.NoteAttachment
	Data       []byte
}

func (a *App) pickNoteFile() {
	n := a.notes.note
	if !a.canPrepareNoteFile() || a.PickFile == nil {
		return
	}
	gen, seq, detail, nid, space := a.gen, a.notes.seq, a.notes.detailSeq, n.ID, a.notes.space
	limit := min(n.Permissions.MaxAttachmentBytes, int64(100<<20))
	if limit <= 0 {
		return
	}
	a.notes.busy = true
	a.notes.err = ""
	a.PickFile(limit, func(file SelectedFile, err error) {
		a.update(func() {
			if gen != a.gen || seq != a.notes.seq || detail != a.notes.detailSeq || nid != a.notes.selected || space != a.selectedSpaceID() {
				return
			}
			a.notes.busy = false
			if err != nil {
				a.notes.err = err.Error()
				return
			}
			if file.Name == "" {
				return
			}
			a.notes.fileUpload = &noteFileUpload{PendingNoteFile: PendingNoteFile{UploadID: id.New(), NoteID: nid, SpaceID: space, Name: file.Name, Data: file.Data, SizeBytes: int64(len(file.Data)), CreatedAt: a.noteNow().UTC()}}
			a.savePendingNoteFile()

		})
	})
}
func (a *App) uploadNoteFile() {
	draft := a.notes.fileUpload
	n := a.notes.note
	if draft == nil || !draft.Saved || n == nil || !n.Permissions.UploadFiles {
		return
	}
	service, ok := a.notesService()
	if !ok {
		return
	}
	files, ok := service.(NotesFilesAPI)
	if !ok {
		return
	}
	nid := n.ID
	pendingStore, pendingServer, pendingUser := a.PendingFiles, a.ServerURL, a.user.ID
	a.writeNote(func(NotesAPI) (func(), error) {
		attachment, err := files.UploadNoteAttachment(context.Background(), nid, draft.UploadID, draft.Name, draft.Data)
		var cleanupErr error
		if err == nil && pendingStore != nil {
			cleanupErr = pendingStore.Delete(pendingServer, pendingUser, nid, draft.UploadID)
		}
		return func() {
			if !slices.ContainsFunc(a.notes.note.Attachments, func(existing api.NoteAttachment) bool { return existing.ID == attachment.ID }) {
				a.notes.note.Attachments = append(a.notes.note.Attachments, attachment)
			}
			if cleanupErr == nil {
				a.notes.fileUpload = nil
			}
			a.notes.notice = "Berkas tersimpan di server; checksum cocok."
			if cleanupErr != nil {
				a.notes.pendingFileErr = "Berkas sudah cloud, tetapi salinan pending perangkat belum terhapus: " + cleanupErr.Error()
			}

		}, err
	})
}
func (a *App) downloadNoteFile(attachment api.NoteAttachment) {
	service, ok := a.notesService()
	if !ok || a.notes.busy || a.notes.detailLoading || a.offlineMode() {
		return
	}
	files, ok := service.(NotesFilesAPI)
	if !ok {
		return
	}
	gen, seq, detail, nid, space := a.gen, a.notes.seq, a.notes.detailSeq, a.notes.selected, a.notes.space
	a.notes.busy = true
	a.notes.err = ""
	a.async(func() func() {
		data, err := files.DownloadNoteAttachment(context.Background(), nid, attachment)
		return func() {
			if gen != a.gen || seq != a.notes.seq || detail != a.notes.detailSeq || nid != a.notes.selected || space != a.selectedSpaceID() {
				return
			}
			a.notes.busy = false
			if err != nil {
				a.noteFailure(err)
				return
			}
			a.notes.fileDownload = &noteFileDownload{Attachment: attachment, Data: data}
			a.notes.notice = "Checksum unduhan cocok; pilih Simpan berkas."
		}
	})
}
func (a *App) noteFileActions(c *ui.Context) {
	n := a.notes.note
	if n == nil {
		return
	}
	disabled := a.notesWriteBlocked() || a.notes.detailLoading
	if a.notes.pendingFileErr != "" {
		ui.Text(c, a.notes.pendingFileErr)
	}
	if n.Permissions.Manage && a.PickFile != nil || a.notes.fileUpload != nil {
		if a.notes.fileUpload == nil {
			if spaceButton(c, "Pilih berkas untuk diunggah", "plus", false).Disabled(!a.canPrepareNoteFile()).Clicked() {
				a.pickNoteFile()
			}
			muted(c, fmt.Sprintf("Batas unggah server: %d byte. Akses berkas mengikuti izin catatan.", n.Permissions.MaxAttachmentBytes))
		} else {
			draft := a.notes.fileUpload
			ui.Text(c, fmt.Sprintf("Belum cloud · %s · %d byte", draft.Name, len(draft.Data)))
			if draft.Saved {
				muted(c, "Tersimpan di perangkat, belum cloud. Buka kembali catatan ini pada akun/server yang sama untuk memulihkan unggahan setelah restart.")
			} else {
				muted(c, "Salinan perangkat belum tersimpan. Jangan tutup aplikasi sebelum penyimpanan berhasil.")
				if spaceButton(c, "Simpan ulang salinan perangkat", "archive", false).Disabled(a.notes.busy).Clicked() {
					a.savePendingNoteFile()
				}
			}
			if spaceButton(c, "Unggah / periksa ulang unggahan", "archive", false).Disabled(disabled || !draft.Saved || !n.Permissions.UploadFiles).Clicked() {
				a.uploadNoteFile()
			}
			if spaceButton(c, "Batalkan pilihan berkas", "close", false).Disabled(a.notes.busy).Clicked() {
				a.discardPendingNoteFile()
			}
		}
	}
	if download := a.notes.fileDownload; download != nil && a.ExportFile != nil {
		if spaceButton(c, "Simpan berkas · "+download.Attachment.Name, "archive", false).Disabled(a.notes.busy).Clicked() {
			gen, seq, detail := a.gen, a.notes.seq, a.notes.detailSeq
			save := a.ExportFile
			a.notes.busy = true
			a.async(func() func() {
				path, err := save(download.Attachment.Name, download.Attachment.ContentType, download.Data)
				return func() {
					if gen != a.gen || seq != a.notes.seq || detail != a.notes.detailSeq {
						return
					}
					a.notes.busy = false
					if err != nil {
						a.notes.err = err.Error()
						return
					}
					if path != "" {
						a.notes.notice = path
						a.notes.fileDownload = nil
					}
				}
			})
		}
	}
}

// Preparing a private device copy is allowed offline with the last downloaded
// note. Upload always requires current server authorization through writeNote.
func (a *App) canPrepareNoteFile() bool {
	n := a.notes.note
	return n != nil && n.Permissions.Manage && n.Permissions.MaxAttachmentBytes > 0 && a.PendingFiles != nil && !a.notes.busy && !a.notes.detailLoading && a.notes.fileUpload == nil && !a.isAdmin()
}
func (a *App) savePendingNoteFile() {
	draft := a.notes.fileUpload
	storage := a.PendingFiles
	if draft == nil || storage == nil || a.notes.busy {
		return
	}
	gen, seq, detail := a.gen, a.notes.seq, a.notes.detailSeq
	server, user := a.ServerURL, a.user.ID
	a.notes.busy = true
	a.notes.pendingFileErr = ""
	snapshot := draft.PendingNoteFile
	a.async(func() func() {
		if snapshot.SHA256 == "" {
			sum := sha256.Sum256(snapshot.Data)
			snapshot.SHA256 = hex.EncodeToString(sum[:])
		}
		err := storage.Save(server, user, snapshot)
		return func() {
			if gen != a.gen || seq != a.notes.seq || detail != a.notes.detailSeq || a.notes.fileUpload != draft {
				return
			}
			a.notes.busy = false
			draft.SHA256 = snapshot.SHA256
			if err != nil {
				a.notes.pendingFileErr = "Berkas belum tersimpan pada perangkat: " + err.Error()
				return
			}
			draft.Saved = true
			a.notes.notice = "Salinan perangkat tersimpan, belum cloud. Unggah saat koneksi dan izin tersedia."
		}
	})
}
func (a *App) discardPendingNoteFile() {
	draft := a.notes.fileUpload
	storage := a.PendingFiles
	if draft == nil || a.notes.busy {
		return
	}
	if storage == nil {
		a.notes.fileUpload = nil
		return
	}
	gen, seq, detail := a.gen, a.notes.seq, a.notes.detailSeq
	server, user := a.ServerURL, a.user.ID
	a.notes.busy = true
	a.async(func() func() {
		err := storage.Delete(server, user, draft.NoteID, draft.UploadID)
		return func() {
			if gen != a.gen || seq != a.notes.seq || detail != a.notes.detailSeq || a.notes.fileUpload != draft {
				return
			}
			a.notes.busy = false
			if err != nil {
				a.notes.pendingFileErr = err.Error()
				return
			}
			a.notes.fileUpload = nil
			a.notes.pendingFileErr = ""
			a.notes.notice = "Salinan pending perangkat dibuang. Berkas cloud tidak dihapus oleh tindakan ini."
		}
	})
}
