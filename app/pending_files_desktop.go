//go:build !js

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/id"
)

type desktopPendingFiles struct{ mu sync.Mutex }

func newPendingFileStore() appui.PendingFileStore { return &desktopPendingFiles{} }
func (s *desktopPendingFiles) account(server string, user id.ID, create bool) (*os.Root, error) {
	key, err := pendingServerKey(server)
	if err != nil {
		return nil, err
	}
	if user == id.Nil {
		return nil, errors.New("akun berkas pending tidak valid")
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	// Reuse the established private directory and symlink checks used by drafts.
	draft := &desktopNoteDrafts{root: filepath.Join(config, "himasif-space", "pending-files", key)}
	return draft.accountRoot(user, create)
}
func pendingMetadataName(note id.ID) string { return note.String() + ".pending.json" }
func readPendingMetadata(account *os.Root, server string, user, note id.ID) (*appui.PendingNoteFile, error) {
	name := pendingMetadataName(note)
	info, err := account.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("metadata berkas pending harus berkas biasa")
	}
	f, err := account.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 8193))
	if err != nil {
		return nil, err
	}
	return decodePendingFile(server, user, note, data)
}
func (s *desktopPendingFiles) Load(server string, user, note id.ID) (*appui.PendingNoteFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if note == id.Nil {
		return nil, errors.New("catatan pending tidak valid")
	}
	account, err := s.account(server, user, false)
	if err != nil || account == nil {
		return nil, err
	}
	defer account.Close()
	file, err := readPendingMetadata(account, server, user, note)
	if err != nil || file == nil {
		return file, err
	}
	name := file.NoteID.String() + "-" + file.UploadID.String() + ".blob"
	info, err := account.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() != file.SizeBytes {
		return nil, errors.New("berkas pending tidak utuh")
	}
	f, err := account.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	file.Data, err = io.ReadAll(io.LimitReader(f, pendingFileMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if err = verifyPendingFile(file); err != nil {
		return nil, err
	}
	return file, nil
}
func writePendingAtomic(root *os.Root, name string, data []byte) error {
	temporary := ".pending-" + id.New().String() + ".tmp"
	f, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	defer f.Close()
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return root.Rename(temporary, name)
}
func (s *desktopPendingFiles) Save(server string, user id.ID, file appui.PendingNoteFile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	metadata, err := encodePendingFile(server, user, file)
	if err != nil {
		return err
	}
	if err = verifyPendingFile(&file); err != nil {
		return err
	}
	account, err := s.account(server, user, true)
	if err != nil {
		return err
	}
	defer account.Close()
	existing, err := readPendingMetadata(account, server, user, file.NoteID)
	if err != nil {
		return err
	}
	if existing != nil && (existing.UploadID != file.UploadID || existing.SHA256 != file.SHA256 || existing.Name != file.Name) {
		return errors.New("catatan ini masih memiliki berkas pending lain; pulihkan atau buang terlebih dahulu")
	}
	dir, err := account.Open(".")
	if err != nil {
		return err
	}
	entries, err := dir.ReadDir(-1)
	_ = dir.Close()
	if err != nil {
		return err
	}
	var count int
	var total int64
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".pending.json") {
			count++
		}
		if strings.HasSuffix(entry.Name(), ".blob") {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
	}
	if existing != nil {
		total -= existing.SizeBytes
		count--
	}
	if count >= pendingAccountMaxCount || total+file.SizeBytes > pendingAccountMaxBytes {
		return fmt.Errorf("batas perangkat tercapai: maksimal %d berkas pending / %d byte per akun dan server", pendingAccountMaxCount, pendingAccountMaxBytes)
	}
	if err = writePendingAtomic(account, file.NoteID.String()+"-"+file.UploadID.String()+".blob", file.Data); err != nil {
		return err
	}
	if err = writePendingAtomic(account, pendingMetadataName(file.NoteID), metadata); err != nil {
		return err
	}
	// Renamed blobs are complete even on a platform without directory fsync.
	// On systems that support it, also persist both directory entries.
	if directory, openErr := account.Open("."); openErr == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}
func (s *desktopPendingFiles) Delete(server string, user, note, upload id.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	account, err := s.account(server, user, false)
	if err != nil || account == nil {
		return err
	}
	defer account.Close()
	file, err := readPendingMetadata(account, server, user, note)
	if err != nil || file == nil {
		return err
	}
	if file.UploadID != upload {
		return errors.New("berkas pending berubah; salinan baru tidak dihapus")
	}
	if err = account.Remove(pendingMetadataName(note)); err != nil {
		return err
	}
	if err = account.Remove(note.String() + "-" + upload.String() + ".blob"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
