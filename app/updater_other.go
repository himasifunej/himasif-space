//go:build !windows && !js

package main

import "errors"

// Elsewhere Space is not installed by the NSIS installer: updates are
// announced, and installed by hand.
func installerUpdatesSupported() bool { return false }

func startUpdateHelper(updateJob) error {
	return errors.New("pembaruan otomatis hanya untuk Windows")
}

// runUpdateHelper is the helper's entry point, on Windows only.
func runUpdateHelper() bool { return false }
