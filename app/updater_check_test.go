//go:build !js

package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/update"
)

// A release candidate without a chosen channel takes rc; a choice wins.
func TestUpdateChannelDefault(t *testing.T) {
	for _, c := range []struct{ chosen, version, want string }{
		{"", "0.4.5-rc2", update.ChannelRC},
		{"", "0.4.5", update.ChannelStable},
		{"", "0.4.6-dev", update.ChannelStable},
		{update.ChannelStable, "0.4.5-rc2", update.ChannelStable},
		{update.ChannelRC, "0.4.5", update.ChannelRC},
	} {
		if got := updateChannel(c.chosen, c.version); got != c.want {
			t.Errorf("updateChannel(%q, %q) = %q, want %q", c.chosen, c.version, got, c.want)
		}
	}
	if f := updateFeeds(update.ChannelRC); len(f) != 2 || f[1] != update.ChannelRC {
		t.Fatalf("rc feeds %v", f)
	}
}

func noFeed(ch string) feedResult {
	return feedResult{err: fmt.Errorf("%w: %s.json", update.ErrNoFeed, ch)}
}

// The owner's rc2: stable.json never published. Stable alone is "nothing
// yet", not an error; an rc client still finds rc.json; a real failure is
// never up to date.
func TestSummarizeCheck(t *testing.T) {
	out := summarizeCheck("0.4.5-rc2", []feedResult{noFeed("stable")})
	if out.offer != nil || out.err != nil || !out.noFeed {
		t.Fatalf("stable 404: %+v", out)
	}
	rc := update.Manifest{App: update.App, Channel: update.ChannelRC, Version: "0.4.5-rc4"}
	out = summarizeCheck("0.4.5-rc2", []feedResult{noFeed("stable"), {manifest: rc}})
	if out.offer == nil || out.offer.Version != "0.4.5-rc4" || out.err != nil {
		t.Fatalf("rc client: %+v", out)
	}
	out = summarizeCheck("0.4.5-rc4", []feedResult{noFeed("stable"), {manifest: rc}})
	if out.offer != nil || out.err != nil || out.noFeed {
		t.Fatalf("rc up to date: %+v", out)
	}
	down := errors.New("update: dial tcp: no such host")
	out = summarizeCheck("0.4.5-rc2", []feedResult{{err: down}})
	if out.offer != nil || out.err == nil || out.noFeed {
		t.Fatalf("failed check: %+v", out)
	}
	out = summarizeCheck("0.4.5-rc2", []feedResult{{err: down}, {manifest: rc}})
	if out.offer == nil {
		t.Fatalf("a newer rc found despite stable failing: %+v", out)
	}
}

// The status never says "terbaru" after a failed check.
func TestIdleStatus(t *testing.T) {
	at := time.Date(2026, 11, 11, 18, 50, 0, 0, time.Local)
	if s := idleStatus(update.ChannelStable, at, true, false); strings.Contains(s, "terbaru") || !strings.Contains(s, "gagal") {
		t.Fatalf("failed: %q", s)
	}
	if s := idleStatus(update.ChannelStable, at, false, true); s != "Belum ada versi stabil baru, diperiksa 18.50" {
		t.Fatalf("no stable: %q", s)
	}
	if s := idleStatus(update.ChannelRC, at, false, true); strings.Contains(s, "terbaru") {
		t.Fatalf("no rc: %q", s)
	}
	if s := idleStatus(update.ChannelStable, at, false, false); s != "terbaru, diperiksa 18.50" {
		t.Fatalf("up to date: %q", s)
	}
	if s := idleStatus(update.ChannelStable, time.Time{}, true, false); s != "belum diperiksa" {
		t.Fatalf("never: %q", s)
	}
}
