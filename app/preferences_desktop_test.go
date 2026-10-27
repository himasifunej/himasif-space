//go:build !js

package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/himasifunej/himasif-space/internal/appui"
)

func TestPreferencesMissingUsesDefaultsWithoutWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "himasif-space", "appearance.json")
	store := &desktopPreferences{path: path}
	got, err := store.Load()
	if err != nil || got != appui.DefaultPreferences() {
		t.Fatalf("Load() = %+v, %v; want defaults, nil", got, err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load created the config directory: %v", err)
	}
}

func TestPreferencesReplaceAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "himasif-space", "appearance.json")
	store := &desktopPreferences{path: path}
	if err := store.Save(appui.DefaultPreferences()); err != nil {
		t.Fatal(err)
	}
	want := appui.Preferences{Theme: "system", Accent: "#123abc", Compact: true, SidebarCollapsed: true}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := (&desktopPreferences{path: path}).Load()
	if err != nil || got != want {
		t.Fatalf("Load() = %+v, %v; want %+v, nil", got, err, want)
	}
	if runtime.GOOS != "windows" {
		for name, mode := range map[string]os.FileMode{path: 0o600, filepath.Dir(path): 0o700} {
			info, err := os.Stat(name)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != mode {
				t.Fatalf("%s mode = %o; want %o", name, info.Mode().Perm(), mode)
			}
		}
	}
	assertNoPreferenceTemps(t, filepath.Dir(path))
}

func TestPreferencesCorruptConfigReturnsErrorAndKeepsFile(t *testing.T) {
	for _, contents := range []string{"{", "null", "[]", `{"compact":"yes"}`} {
		t.Run(contents, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "appearance.json")
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := (&desktopPreferences{path: path}).Load()
			if err == nil || got != appui.DefaultPreferences() {
				t.Fatalf("Load() = %+v, %v; want defaults with error", got, err)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != contents {
				t.Fatalf("Load changed corrupt file: %q, %v", data, err)
			}
		})
	}
}

func TestPreferencesUnavailablePathReturnsErrors(t *testing.T) {
	barrier := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(barrier, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &desktopPreferences{path: filepath.Join(barrier, "appearance.json")}
	if _, err := store.Load(); err == nil {
		t.Fatal("Load hid an unreadable config path")
	}
	if err := store.Save(appui.DefaultPreferences()); err == nil {
		t.Fatal("Save hid an unwritable config path")
	}
}

func TestPreferencesFailedReplaceCleansTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appearance.json")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(path, "keep")
	if err := os.WriteFile(marker, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (&desktopPreferences{path: path}).Save(appui.DefaultPreferences()); err == nil {
		t.Fatal("Save should reject replacing a directory")
	}
	data, err := os.ReadFile(marker)
	if err != nil || string(data) != "unchanged" {
		t.Fatalf("Failed Save changed existing data: %q, %v", data, err)
	}
	assertNoPreferenceTemps(t, dir)
}

func assertNoPreferenceTemps(t *testing.T, dir string) {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(dir, ".appearance-*.json"))
	if err != nil || len(names) != 0 {
		t.Fatalf("temporary appearance files remain: %v, %v", names, err)
	}
}
