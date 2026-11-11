//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/himasifunej/himasif-space/internal/update"
)

// installerUpdatesSupported reports whether Space runs from where its
// installer put it, with its uninstaller beside it: a portable copy or a
// development build is not updated by an installer.
func installerUpdatesSupported() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(filepath.Dir(exe), "Uninstall.exe"))
	return err == nil
}

// startUpdateHelper copies Space.exe next to the installer and starts the
// copy as the helper, which outlives Space: the installer replaces
// Space.exe, so the helper cannot be the installed one.
func startUpdateHelper(job updateJob) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	job.InstallDir, job.Exe, job.PID = filepath.Dir(exe), filepath.Base(exe), os.Getpid()
	if strings.ContainsAny(job.InstallDir, "\"\r\n") {
		return errors.New("folder Space tidak didukung pemasang")
	}
	helper := filepath.Join(job.Dir, "space-update-helper.exe")
	if err := copyFile(exe, helper); err != nil {
		return err
	}
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	jobFile := filepath.Join(job.Dir, "job.json")
	if err := os.WriteFile(jobFile, data, 0o600); err != nil {
		return err
	}
	cmd := exec.Command(helper, updateHelperFlag, jobFile)
	cmd.Dir = job.Dir
	return cmd.Start()
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o700)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// runUpdateHelper is main's first step: as the helper, it applies the job
// and reports true, and Space does not start.
func runUpdateHelper() bool {
	if len(os.Args) != 3 || os.Args[1] != updateHelperFlag {
		return false
	}
	data, err := os.ReadFile(os.Args[2])
	if err != nil {
		return true
	}
	var job updateJob
	if json.Unmarshal(data, &job) != nil || job.Dir == "" || job.InstallDir == "" || job.Exe == "" {
		return true
	}
	err = applyUpdate(job)
	r := updateResult{Version: job.Version, OK: err == nil}
	if err != nil {
		r.Error = err.Error()
	}
	writeUpdateResult(job.Dir, r)
	// Space opens again either way: the new version, or the old one with
	// the report of what failed.
	space := exec.Command(filepath.Join(job.InstallDir, job.Exe))
	if job.Background {
		space.Args = append(space.Args, "--background")
	}
	space.Dir = job.InstallDir
	_ = space.Start()
	return true
}

// applyUpdate waits for Space to close, checks the installer once more
// and runs it silently into Space's folder.
func applyUpdate(job updateJob) error {
	if p, err := os.FindProcess(job.PID); err == nil {
		exited := make(chan struct{})
		go func() { _, _ = p.Wait(); close(exited) }()
		select {
		case <-exited:
		case <-time.After(90 * time.Second):
			// The installer closes it too (CloseApp under /S).
		}
	}
	if err := update.VerifyFile(job.Setup, job.Installer); err != nil {
		return fmt.Errorf("installer tidak cocok dengan manifest, tidak dijalankan")
	}
	// NSIS takes /D= last and unquoted, spaces and all.
	cmd := exec.Command(job.Setup)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `"` + job.Setup + `" /S /D=` + job.InstallDir, HideWindow: true}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("installer berhenti: %v", err)
	}
	return nil
}
