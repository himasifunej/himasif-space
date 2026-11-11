package server

import (
	"strconv"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// waCommandValues are what a custom command's response may name: the
// app shows them as [Nama pengirim], [Tanggal] and so on, and saves the
// placeholders below.
type waCommandValues struct {
	Sender, Group, Bot, Args string
	At                       time.Time
	Zone                     string
}

// renderWACommandText fills a custom command's response. A response
// without placeholders, as every one saved before they existed, is sent
// as it is.
func renderWACommandText(response string, v waCommandValues) string {
	if !strings.Contains(response, "{") {
		return response
	}
	at := v.At
	if loc, err := timeutil.LoadZone(v.Zone); err == nil && loc != nil {
		at = at.In(loc)
	}
	date := strconv.Itoa(at.Day()) + " " + waMonths[at.Month()-1] + " " + strconv.Itoa(at.Year())
	return strings.NewReplacer(
		"{sender}", v.Sender,
		"{date}", date,
		"{time}", at.Format("15.04"),
		"{group}", v.Group,
		"{name}", v.Bot,
		"{args}", v.Args,
	).Replace(response)
}
