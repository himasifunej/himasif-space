//go:build !js

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/id"
)

type desktopNoteDrafts struct {
	root    string
	initErr error
	mu      sync.Mutex
}

func newNoteDraftStore() appui.NoteDraftStore {
	config, err := os.UserConfigDir()
	if err != nil {
		return &desktopNoteDrafts{initErr: fmt.Errorf("lokasi draf perangkat: %w", err)}
	}
	return &desktopNoteDrafts{root: filepath.Join(config, "himasif-space", "note-drafts")}
}

// accountRoot confines subsequent reads/writes to one account's directory,
// including symlink resolution. Loading a missing draft never creates files.
func (s *desktopNoteDrafts) accountRoot(userID id.ID, create bool) (*os.Root, error) {
	if s.initErr != nil {
		return nil, s.initErr
	}
	if s.root == "" || !filepath.IsAbs(s.root) {
		return nil, errors.New("lokasi draf perangkat tidak valid")
	}
	if create {
		if err := os.MkdirAll(s.root, 0o700); err != nil {
			return nil, fmt.Errorf("buat direktori draf perangkat: %w", err)
		}
	}
	info, err := os.Lstat(s.root)
	if errors.Is(err, os.ErrNotExist) && !create {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("baca direktori draf perangkat: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("direktori draf perangkat harus direktori biasa")
	}
	if create {
		if err := os.Chmod(s.root, 0o700); err != nil {
			return nil, fmt.Errorf("lindungi direktori draf perangkat: %w", err)
		}
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return nil, fmt.Errorf("buka direktori draf perangkat: %w", err)
	}
	defer root.Close()
	name := userID.String()
	if create {
		if err := root.Mkdir(name, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("buat direktori draf akun: %w", err)
		}
	}
	info, err = root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) && !create {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("baca direktori draf akun: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("direktori draf akun tidak boleh berupa tautan atau berkas")
	}
	account, err := root.OpenRoot(name)
	if err != nil {
		return nil, fmt.Errorf("buka direktori draf akun: %w", err)
	}
	if create {
		if err := account.Chmod(".", 0o700); err != nil {
			account.Close()
			return nil, fmt.Errorf("lindungi direktori draf akun: %w", err)
		}
	}
	return account, nil
}

func (s *desktopNoteDrafts) Load(userID, noteID id.ID) (*appui.LocalNoteDraft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateNoteDraftIdentity(userID, noteID); err != nil {
		return nil, err
	}
	account, err := s.accountRoot(userID, false)
	if err != nil || account == nil {
		return nil, err
	}
	defer account.Close()
	name := noteID.String() + ".json"
	info, err := account.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("periksa draf perangkat: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("draf perangkat harus berkas biasa")
	}
	f, err := account.Open(name)
	if err != nil {
		return nil, fmt.Errorf("buka draf perangkat: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxLocalNoteDraftBytes+1))
	if err != nil {
		return nil, fmt.Errorf("baca draf perangkat: %w", err)
	}
	return decodeLocalNoteDraft(userID, noteID, data)
}

func (s *desktopNoteDrafts) Save(userID, noteID id.ID, draft appui.LocalNoteDraft) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Validate before touching the last good copy or creating any directories.
	data, err := encodeLocalNoteDraft(userID, noteID, draft)
	if err != nil {
		return err
	}
	account, err := s.accountRoot(userID, true)
	if err != nil {
		return err
	}
	defer account.Close()
	name := noteID.String() + ".json"
	if info, err := account.Lstat(name); err == nil && !info.Mode().IsRegular() {
		return errors.New("draf perangkat tujuan harus berkas biasa")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("periksa tujuan draf perangkat: %w", err)
	}
	temporary := ".draft-" + id.New().String() + ".tmp"
	f, err := account.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("buat draf perangkat sementara: %w", err)
	}
	defer account.Remove(temporary)
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("tulis draf perangkat: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sinkronkan berkas draf perangkat: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("tutup draf perangkat: %w", err)
	}
	if err := account.Rename(temporary, name); err != nil {
		return fmt.Errorf("ganti draf perangkat: %w", err)
	}
	return nil
}

func (s *desktopNoteDrafts) Delete(userID, noteID id.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateNoteDraftIdentity(userID, noteID); err != nil {
		return err
	}
	account, err := s.accountRoot(userID, false)
	if err != nil || account == nil {
		return err
	}
	defer account.Close()
	name := noteID.String() + ".json"
	if info, err := account.Lstat(name); err == nil && !info.Mode().IsRegular() {
		return errors.New("draf perangkat yang akan dibuang harus berkas biasa")
	} else if errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("periksa draf perangkat: %w", err)
	}
	if err := account.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("buang draf perangkat: %w", err)
	}
	return nil
}
