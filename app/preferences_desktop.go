//go:build !js

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/himasifunej/himasif-space/internal/appui"
)

type desktopPreferences struct {
	mu      sync.Mutex
	path    string
	initErr error
}

func newPreferencesStore() appui.PreferencesStore {
	dir, err := os.UserConfigDir()
	if err != nil {
		return &desktopPreferences{initErr: fmt.Errorf("locate appearance config: %w", err)}
	}
	return &desktopPreferences{path: filepath.Join(dir, "himasif-space", "appearance.json")}
}

func (s *desktopPreferences) Load() (appui.Preferences, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defaults := appui.DefaultPreferences()
	if s.initErr != nil {
		return defaults, s.initErr
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return defaults, nil
	}
	if err != nil {
		return defaults, fmt.Errorf("read appearance config: %w", err)
	}
	var preferences *appui.Preferences
	if err := json.Unmarshal(data, &preferences); err != nil {
		return defaults, fmt.Errorf("decode appearance config: %w", err)
	}
	if preferences == nil {
		return defaults, errors.New("decode appearance config: expected a JSON object")
	}
	return appui.NormalizePreferences(*preferences), nil
}

func (s *desktopPreferences) Save(preferences appui.Preferences) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initErr != nil {
		return s.initErr
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create appearance directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("protect appearance directory: %w", err)
	}
	data, err := json.MarshalIndent(appui.NormalizePreferences(preferences), "", "  ")
	if err != nil {
		return fmt.Errorf("encode appearance config: %w", err)
	}
	file, err := os.CreateTemp(dir, ".appearance-*.json")
	if err != nil {
		return fmt.Errorf("create appearance config: %w", err)
	}
	name := file.Name()
	defer os.Remove(name)
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write appearance config: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync appearance config: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close appearance config: %w", err)
	}
	if err := os.Rename(name, s.path); err != nil {
		return fmt.Errorf("replace appearance config: %w", err)
	}
	return nil
}
