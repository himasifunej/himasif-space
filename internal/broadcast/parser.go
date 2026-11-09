// Package broadcast parses the explicit fields of Humas lists. It does not
// infer identities, calendar changes or task completion from conversational text.
package broadcast

import (
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const Version = "humas-fields-v1"

type Item struct {
	Position    int            `json:"position"`
	Title       string         `json:"title"`
	Humas       string         `json:"humas"`
	Design      string         `json:"design"`
	DateText    string         `json:"dateText"`
	PostingDate *timeutil.Date `json:"postingDate,omitempty"`
	Day, Month  int
	Urgent      bool   `json:"urgent"`
	Raw         string `json:"raw"`
	Issue       string `json:"issue"`
}

var numbered = regexp.MustCompile(`^\s*([0-9]{1,3})[.)]\s+(.+)$`)
var field = regexp.MustCompile(`(?i)^\s*(pj\s*humas|pj\s*mediatek|pj\s*desain|tanggal\s*posting|tanggal|jadwal\s*posting|dl|judul|konsep)\s*[:=]\s*(.*?)\s*$`)
var numericDate = regexp.MustCompile(`^([0-9]{1,4})[-/]([0-9]{1,2})(?:[-/]([0-9]{2,4}))?$`)
var namedDate = regexp.MustCompile(`(?i)^([0-9]{1,2})\s+([a-z]+)(?:\s+([0-9]{4}))?$`)
var months = map[string]int{"januari": 1, "jan": 1, "februari": 2, "feb": 2, "maret": 3, "mar": 3, "april": 4, "apr": 4, "mei": 5, "juni": 6, "jun": 6, "juli": 7, "jul": 7, "agustus": 8, "agu": 8, "agt": 8, "september": 9, "sep": 9, "oktober": 10, "okt": 10, "november": 11, "nov": 11, "desember": 12, "des": 12}

func Normalize(s string) string {
	s = strings.Trim(strings.TrimSpace(s), "*_`~@")
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}
func cleanLine(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimLeftFunc(s, func(r rune) bool {
		return r == '*' || r == '_' || r == '`' || r == '~' || r == '>' || r == '•' || r == '-' || unicode.IsSpace(r)
	})
	return strings.Trim(strings.TrimSpace(s), "*_`~")
}

// Parse returns every numbered block once any explicit PJ/date field makes the
// message recognizable as a broadcast. Unsupported fields remain in Raw.
func Parse(body string) []Item {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	out := []Item{}
	urgent := false
	recognized := false
	finish := func() {
		if len(out) == 0 {
			return
		}
		i := &out[len(out)-1]
		if i.Title == "" {
			i.Issue += " Judul belum terbaca."
		}
		if i.Humas == "" {
			i.Issue += " PJ Humas belum terbaca."
		}
		if i.DateText == "" {
			i.Issue += " Tanggal posting belum terbaca."
		}
		i.Issue = strings.TrimSpace(i.Issue)
	}
	for _, raw := range lines {
		line := cleanLine(raw)
		if strings.Trim(strings.ToLower(line), " :[]") == "dadakan" {
			urgent = true
			continue
		}
		if m := numbered.FindStringSubmatch(line); m != nil {
			finish()
			if len(out) >= 100 {
				out[len(out)-1].Issue = "Daftar melebihi 100 item; perlu tinjauan pesan lengkap."
				break
			}
			title := cleanLine(m[2])
			if f := field.FindStringSubmatch(title); f != nil && (Normalize(f[1]) == "judul" || Normalize(f[1]) == "konsep") {
				title = f[2]
			}
			out = append(out, Item{Position: len(out) + 1, Title: title, Urgent: urgent, Raw: raw})
			continue
		}
		if len(out) == 0 {
			continue
		}
		item := &out[len(out)-1]
		item.Raw += "\n" + raw
		f := field.FindStringSubmatch(line)
		if f == nil {
			continue
		}
		recognized = true
		key, value := Normalize(f[1]), cleanLine(f[2])
		switch key {
		case "pj humas":
			if item.Humas != "" && Normalize(item.Humas) != Normalize(value) {
				item.Issue += " PJ Humas ganda."
			}
			item.Humas = value
		case "pj mediatek", "pj desain":
			if item.Design != "" && Normalize(item.Design) != Normalize(value) {
				item.Issue += " PJ desain ganda."
			}
			item.Design = value
		case "judul", "konsep":
			if item.Title != "" && Normalize(item.Title) != Normalize(value) {
				item.Issue += " Judul ganda."
			}
			item.Title = value
		default:
			if item.DateText != "" && item.DateText != value {
				item.Issue += " Tanggal ganda."
			}
			item.DateText = value
			item.PostingDate, item.Day, item.Month = parseDate(value)
			if item.Month == 0 {
				item.Issue += " Format tanggal tidak didukung atau ambigu."
			}
		}
	}
	finish()
	if !recognized {
		return nil
	}
	return out
}

// A missing year remains missing. Matching it against an existing calendar is
// allowed only if that calendar yields one candidate in the authorized period.
func parseDate(raw string) (*timeutil.Date, int, int) {
	value := strings.ToLower(strings.TrimSpace(raw))
	day, month, year := 0, 0, 0
	if m := numericDate.FindStringSubmatch(value); m != nil {
		a, _ := strconv.Atoi(m[1])
		b, _ := strconv.Atoi(m[2])
		c, _ := strconv.Atoi(m[3])
		if len(m[1]) == 4 {
			year, month, day = a, b, c
		} else {
			day, month, year = a, b, c
			if year > 0 && year < 100 {
				return nil, 0, 0
			}
		}
	} else if m := namedDate.FindStringSubmatch(value); m != nil {
		day, _ = strconv.Atoi(m[1])
		month = months[m[2]]
		year, _ = strconv.Atoi(m[3])
	} else {
		return nil, 0, 0
	}
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return nil, 0, 0
	}
	checkYear := year
	if checkYear == 0 {
		checkYear = 2000
	}
	date, err := timeutil.ParseDate(strconv.Itoa(checkYear) + "-" + pad(month) + "-" + pad(day))
	if err != nil {
		return nil, 0, 0
	}
	if year == 0 {
		return nil, day, month
	}
	return &date, day, month
}
func pad(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
