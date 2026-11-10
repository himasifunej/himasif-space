//go:build !windows && !js

package main

func islandSupported() bool                                { return false }
func islandEnvironmentAllowed() bool                       { return false }
func islandPrepareWindow(uintptr)                          {}
func islandWindowShape(uintptr, int, int, float64)         {}
func islandGlass(uintptr, bool, bool)                      {}
func islandPosition(uintptr, uintptr, bool, int, int) bool { return false }
func islandPointer(uintptr, [3]float64) (bool, bool)       { return false, false }
