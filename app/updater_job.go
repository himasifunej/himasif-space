//go:build !js

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/himasifunej/himasif-space/internal/update"
)

// updateHelperFlag starts Space as the update helper: a copy of Space.exe
// that waits for Space to close, runs the installer and opens Space again.
const updateHelperFlag = "--space-apply-update"

// updateResultFile is the helper's report, read at the next start.
const updateResultFile = "result.json"

// updateJob is what the helper is told, in a file next to the installer.
type updateJob struct {
	Setup     string           `json:"setup"`
	Installer update.Installer `json:"installer"`
	Version   string           `json:"version"`
	// Dir holds the installer, the helper and the report.
	Dir string `json:"dir"`
	// InstallDir and Exe are where Space is installed and its executable's
	// name; PID is the Space to wait for.
	InstallDir string `json:"installDir"`
	Exe        string `json:"exe"`
	PID        int    `json:"pid"`
}

// updateResult is what the helper reports.
type updateResult struct {
	Version string    `json:"version"`
	OK      bool      `json:"ok"`
	Error   string    `json:"error,omitempty"`
	At      time.Time `json:"at"`
}

func writeUpdateResult(dir string, r updateResult) {
	r.At = time.Now()
	data, _ := json.Marshal(r)
	_ = os.WriteFile(filepath.Join(dir, updateResultFile), data, 0o600)
}
