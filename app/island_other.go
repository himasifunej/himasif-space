//go:build !windows && !js

package main

func islandSupported() bool                                            { return false }
func islandEnvironmentAllowed() bool                                   { return false }
func islandPrepareWindow(uintptr)                                      {}
func islandWindowShape(uintptr, [4]float64, int, int)                  {}
func islandClearBackdrop(uintptr)                                      {}
func islandObstructed() bool                                           { return false }
func islandPosition(uintptr, uintptr, bool, int, int) bool             { return false }
func islandPointer(uintptr, float64, float64) (float64, float64, bool) { return 0, 0, false }
