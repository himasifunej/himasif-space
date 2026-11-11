//go:build !js

package main

import (
	"errors"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/update"
)

// updateChannel is the device's channel: the one chosen, else rc on a
// release candidate (whose testers would otherwise wait for a stable
// version that may not exist yet), else stable.
func updateChannel(chosen, version string) string {
	switch chosen {
	case update.ChannelRC, update.ChannelStable:
		return chosen
	}
	if strings.Contains(version, "-rc") {
		return update.ChannelRC
	}
	return update.ChannelStable
}

// updateFeeds are the feeds a channel reads: stable always, and rc too on
// the rc channel, which still takes a newer stable release.
func updateFeeds(channel string) []string {
	if channel == update.ChannelRC {
		return []string{update.ChannelStable, update.ChannelRC}
	}
	return []string{update.ChannelStable}
}

// feedResult is one feed read: its manifest, or why there is none.
type feedResult struct {
	manifest update.Manifest
	err      error
}

// checkOutcome is what a check found: a newer version (offer), that
// nothing is published on any feed read (noFeed), or that a feed could not
// be read (err), which never counts as up to date.
type checkOutcome struct {
	offer  *update.Manifest
	noFeed bool
	err    error
}

// summarizeCheck sums the feeds read up. A feed that answers 404 publishes
// nothing: no error, and no claim either way. Another failure is the
// check's error unless a newer version was found anyway.
func summarizeCheck(current string, results []feedResult) checkOutcome {
	var found []update.Manifest
	var out checkOutcome
	allNoFeed := len(results) > 0
	for _, r := range results {
		switch {
		case r.err == nil:
			found = append(found, r.manifest)
			allNoFeed = false
		case errors.Is(r.err, update.ErrNoFeed):
		default:
			allNoFeed = false
			if out.err == nil {
				out.err = r.err
			}
		}
	}
	if m, ok := update.Newest(current, found...); ok {
		return checkOutcome{offer: &m}
	}
	if out.err == nil {
		out.noFeed = allNoFeed
	}
	return out
}

// idleStatus is the updater's state when nothing is offered: a failed
// check never reads as up to date.
func idleStatus(channel string, checked time.Time, failed bool, noFeed bool) string {
	switch {
	case checked.IsZero():
		return "belum diperiksa"
	case failed:
		return "pemeriksaan terakhir gagal, " + checked.Format("15.04")
	case noFeed && channel == update.ChannelStable:
		return "Belum ada versi stabil baru, diperiksa " + checked.Format("15.04")
	case noFeed:
		return "Belum ada versi baru di Uji coba, diperiksa " + checked.Format("15.04")
	}
	return "terbaru, diperiksa " + checked.Format("15.04")
}
