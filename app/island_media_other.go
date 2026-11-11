//go:build !windows && !js

package main

func islandMediaSupported() bool { return false }

type islandMediaSession struct{}

func newIslandMediaSession() *islandMediaSession      { return &islandMediaSession{} }
func (*islandMediaSession) read() (islandMedia, bool) { return islandMedia{}, false }
func (*islandMediaSession) command(string)            {}
func (*islandMediaSession) close()                    {}
