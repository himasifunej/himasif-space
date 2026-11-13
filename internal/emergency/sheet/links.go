// Package sheet is Space's emergency Google Sheet (note "Space — Darurat
// via Google Sheets", spike of 12 Nov 2026): the workbook Space writes from
// its offline cache, the sheet's links, and reading what the team changed
// from the sheet published to the web. Space reads the sheet; it never
// writes to Google.
package sheet

import (
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	publishPath = regexp.MustCompile(`^/spreadsheets/d/e/([A-Za-z0-9_-]{20,})/(pub|pubhtml)(/.*)?$`)
	editPath    = regexp.MustCompile(`^/spreadsheets/d/([A-Za-z0-9_-]{20,})(/.*)?$`)
)

// ErrPublishLink and ErrEditLink say which link a pasted one should be.
var (
	ErrPublishLink = errors.New("Tempel link dari File → Bagikan → Publikasikan ke web: bentuknya docs.google.com/spreadsheets/d/e/…/pub.")
	ErrEditLink    = errors.New("Tempel link dari tombol Bagikan sheet: bentuknya docs.google.com/spreadsheets/d/…/edit, bukan link publish.")
)

func googleURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Host != "docs.google.com" || u.User != nil {
		return nil, errors.New("bukan link Google Sheets")
	}
	return u, nil
}

// PublishURL is the published workbook a pasted publish link stands for,
// as xlsx: one request brings every tab, the hidden ID columns included.
func PublishURL(raw string) (string, error) {
	u, err := googleURL(raw)
	if err != nil {
		return "", ErrPublishLink
	}
	m := publishPath.FindStringSubmatch(u.Path)
	if m == nil {
		return "", ErrPublishLink
	}
	return "https://docs.google.com/spreadsheets/d/e/" + m[1] + "/pub?output=xlsx", nil
}

// EditURL is a pasted edit link, kept to the sheet's own address: what the
// team opens, and what Salin link and the QR hold.
func EditURL(raw string) (string, error) {
	u, err := googleURL(raw)
	if err != nil {
		return "", ErrEditLink
	}
	m := editPath.FindStringSubmatch(u.Path)
	if m == nil || m[1] == "e" {
		return "", ErrEditLink
	}
	return "https://docs.google.com/spreadsheets/d/" + m[1] + "/edit?usp=sharing", nil
}

// RowURL opens the edit link at a row of the Tugas tab, when the tab's
// gid is known; else the sheet.
func RowURL(edit, gid string, line int) string {
	if gid == "" || line <= 0 {
		return edit
	}
	base, _, _ := strings.Cut(edit, "?")
	return base + "#gid=" + gid + "&range=A" + strconv.Itoa(line)
}

// WhatsAppURL opens WhatsApp with text ready to send to a group.
func WhatsAppURL(text string) string {
	return "https://wa.me/?text=" + url.QueryEscape(text)
}

// ShareText is the message Bagikan ke WhatsApp sends with the edit link.
func ShareText(edit string) string {
	return "Space sedang gangguan. Catat progres tugas di sheet darurat ini (tanpa login): " + edit + "\nIsi kolom Progres dan Dilaporkan oleh. Jangan ubah kolom abu-abu."
}
