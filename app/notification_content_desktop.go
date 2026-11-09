//go:build !js

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"net/url"
	"strings"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// A UUIDv7 prefix mostly holds its timestamp. Hash the entire identity so
// deliveries created together do not replace each other in Windows history.
func windowsNotificationTag(delivery id.ID) string {
	digest := sha256.Sum256(delivery[:])
	return hex.EncodeToString(digest[:8])
}

func escapeXML(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func notificationActionLink(link, action string) string {
	u, err := url.Parse(link)
	if err != nil {
		return link
	}
	q := u.Query()
	q.Set("action", action)
	u.RawQuery = q.Encode()
	return u.String()
}

func windowsNotificationXML(d api.Delivery, link string) string {
	seen := notificationActionLink(link, "seen")
	snooze := notificationActionLink(link, "snooze")
	return `<toast activationType="protocol" launch="` + escapeXML(link) + `"><visual><binding template="ToastGeneric"><text>` + escapeXML(d.Title) + `</text><text>` + escapeXML(d.Body) + `</text></binding></visual><actions><action content="Buka" activationType="protocol" arguments="` + escapeXML(link) + `"/><action content="Sudah lihat" activationType="protocol" arguments="` + escapeXML(seen) + `"/><action content="15 menit lagi" activationType="protocol" arguments="` + escapeXML(snooze) + `"/></actions><audio silent="true"/></toast>`
}
