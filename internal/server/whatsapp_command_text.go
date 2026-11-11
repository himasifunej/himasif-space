package server

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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
	out := strings.NewReplacer(
		"{sender}", v.Sender,
		"{date}", date,
		"{time}", at.Format("15.04"),
		"{group}", v.Group,
		"{name}", v.Bot,
		"{args}", v.Args,
	).Replace(response)
	// A response naming {args} many times could grow far past a chat
	// message: what the filling adds is cut at waCommandTextMax runes. A
	// response saved longer than that, which fills nothing in, is sent
	// whole as before.
	limit := max(waCommandTextMax, utf8.RuneCountInString(response))
	if out != response && utf8.RuneCountInString(out) > limit {
		out = string([]rune(out)[:limit])
	}
	return out
}

// waCommandTextMax is the longest filled response sent, in runes.
const waCommandTextMax = 4096
