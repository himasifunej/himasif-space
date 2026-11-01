//go:build !js

package main

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"net/url"
	"strings"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func TestNotificationLinkKeepsLongContentOutOfShellArguments(t *testing.T) {
	account, space, related := id.New(), id.New(), id.New()
	d := api.Delivery{ID: id.New(), SourceKind: "ping", SourceID: id.New(), RelatedKind: "task", RelatedID: &related, SpaceID: &space,
		Title: "Judul privat", Body: strings.Repeat("Isi privat & <rapat> ", 1000), LastError: "Jangan bawa error ke shell"}
	link := notificationLink(account, d, "open")
	if len(link) > 2048 {
		t.Fatalf("navigation URL grows with content: %d", len(link))
	}
	u, err := url.Parse(link)
	if err != nil || u.Query().Get("account") != account.String() {
		t.Fatalf("bad account hint: %v", err)
	}
	data, err := base64.RawURLEncoding.DecodeString(u.Query().Get("payload"))
	if err != nil {
		t.Fatal(err)
	}
	var hint api.Delivery
	if err := json.Unmarshal(data, &hint); err != nil {
		t.Fatal(err)
	}
	if hint.ID != d.ID || hint.SourceID != d.SourceID || hint.SourceKind != d.SourceKind || hint.RelatedKind != d.RelatedKind || *hint.RelatedID != related || *hint.SpaceID != space {
		t.Fatal("notification lost navigation identity")
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"title", "body", "lastError", "eventId", "status", "scheduledAt"} {
		if _, found := fields[key]; found {
			t.Fatalf("protocol payload exposes %s", key)
		}
	}
}

func TestWindowsToastTagsUseWholeDeliveryIdentity(t *testing.T) {
	// Same timestamp/version prefix, different suffix: the old [:16] tags collide.
	a, _ := id.Parse("019a8c00-1234-7123-8000-000000000001")
	b, _ := id.Parse("019a8c00-1234-7123-8000-000000000002")
	if a.String()[:16] != b.String()[:16] {
		t.Fatal("invalid collision fixture")
	}
	if windowsNotificationTag(a) == windowsNotificationTag(b) || windowsNotificationTag(a) != windowsNotificationTag(a) || len(windowsNotificationTag(a)) != 16 {
		t.Fatal("toast tags collide or cannot address a delivery consistently")
	}
}

func TestWindowsToastXMLKeepsTextAndDistinctAccountBoundActions(t *testing.T) {
	d := api.Delivery{ID: id.New(), SourceKind: "task", SourceID: id.New(), Title: `Rapat <A> & "B"`, Body: "Isi 'privat' & agenda"}
	account := id.New()
	link := notificationLink(account, d, "open")
	var toast struct {
		Launch  string   `xml:"launch,attr"`
		Texts   []string `xml:"visual>binding>text"`
		Actions []struct {
			Args       string `xml:"arguments,attr"`
			Activation string `xml:"activationType,attr"`
		} `xml:"actions>action"`
		Audio struct {
			Silent string `xml:"silent,attr"`
		} `xml:"audio"`
	}
	if err := xml.Unmarshal([]byte(windowsNotificationXML(d, link)), &toast); err != nil {
		t.Fatal(err)
	}
	if toast.Launch != link || len(toast.Texts) != 2 || toast.Texts[0] != d.Title || toast.Texts[1] != d.Body || toast.Audio.Silent != "true" || len(toast.Actions) != 3 {
		t.Fatal("toast XML lost escaped text, silence or actions")
	}
	for i, action := range []string{"open", "seen", "snooze"} {
		u, err := url.Parse(toast.Actions[i].Args)
		if err != nil || u.Query().Get("action") != action || u.Query().Get("account") != account.String() || toast.Actions[i].Activation != "protocol" {
			t.Fatalf("bad action %s: %v", action, err)
		}
	}
}
