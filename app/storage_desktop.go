//go:build !js

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Windows reports PATH_NOT_FOUND both for missing ancestors and for a file
// used as a directory. Check the nearest existing ancestor before treating a
// read as an ordinary first launch with no saved preferences/draft.
func validateMissingStoragePath(path string) error {
	for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
		info, err := os.Stat(parent)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("storage parent is not a directory: %s", parent)
			}
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if next := filepath.Dir(parent); next == parent {
			return err
		}
	}
}
