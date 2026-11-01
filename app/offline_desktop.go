//go:build !js

package main

import (
	"errors"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type diskOffline struct {
	mu   sync.Mutex
	root *os.Root
}

func configureOffline(c *client.Client) {
	config, err := os.UserConfigDir()
	if err != nil {
		return
	}
	dir := filepath.Join(config, "himasif-space", "offline")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return
	}
	info, err := os.Lstat(dir)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		return
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return
	}
	c.Offline = client.NewOfflineRepository(&diskOffline{root: root})
}
func validOfflineKey(key string) bool {
	return key != "" && !strings.ContainsAny(key, "/\\.") && len(key) < 200
}
func (s *diskOffline) Read(key string) ([]byte, error) {
	if !validOfflineKey(key) {
		return nil, errors.New("Kunci cache tidak valid.")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.root.Open(key)
	if errors.Is(err, os.ErrNotExist) {
		return nil, client.ErrOfflineMissing
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 40<<20))
	if len(data) >= 40<<20 {
		return nil, errors.New("Cache melebihi batas ukuran.")
	}
	return data, err
}
func (s *diskOffline) Write(key string, data []byte) error {
	if !validOfflineKey(key) {
		return errors.New("Kunci cache tidak valid.")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	name := "pending-" + id.New().String()
	defer s.root.Remove(name)
	f, err := s.root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return s.root.Rename(name, key)
}
func (s *diskOffline) Delete(key string) error {
	if !validOfflineKey(key) {
		return errors.New("Kunci cache tidak valid.")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.root.Remove(key)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
