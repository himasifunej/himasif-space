// Package broadcast parses the explicit fields of Humas lists. It does not
// infer identities, calendar changes or task completion from conversational text.
//
// Two shapes are read:
//
//   - the Humas monthly template, one content per line:
//     "1. open regis, pj humas @Nama, pj med @Nama SI'24 (19 nov) dark blue",
//     under a month header ("[HALO, WELCOME TO NOVEMBER MEDIATEK]"), with an
//     optional "DADAKAN" section and a "tanggal = DL" note;
//   - the labelled block, one field per line ("PJ Humas: …", "Tanggal: …").
//
// The date written next to a content is its posting date. The design deadline
// is that date minus DesignLeadDays (owner decision 25 Nov 2026).
package broadcast

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/himasifunej/himasif-space/internal/timeutil"
)

const Version = "humas-fields-v2"

// DesignLeadDays is how many days before posting the design must be done.
const DesignLeadDays = 2

type Item struct {
	Position int `json:"position"`
	// Line is the 1-based line of the message where the item starts.
	Line     int    `json:"line,omitempty"`
	Title    string `json:"title"`
	Humas    string `json:"humas"`
	Design   string `json:"design"`
	DateText string `json:"dateText"`
	// PostingDate is set only when the year is known (written on the item or
	// in the header). Otherwise Day and Month carry the date.
	PostingDate *timeutil.Date `json:"postingDate,omitempty"`
	Day, Month  int
	// DesignDeadline is PostingDate minus DesignLeadDays. Without a year,
	// DeadlineDay/DeadlineMonth carry it, except when it depends on whether
	// February is leap (posting on 1 or 2 March).
	DesignDeadline *timeutil.Date `json:"designDeadline,omitempty"`
	DeadlineDay    int            `json:"deadlineDay,omitempty"`
	DeadlineMonth  int            `json:"deadlineMonth,omitempty"`
	// HumasAlias/DesignAlias hold a nickname written in brackets after the
	// name ("@Almas SI'23 (Almas)"). The cohort tag (SI'23) is dropped.
	HumasAlias  string `json:"humasAlias,omitempty"`
	DesignAlias string `json:"designAlias,omitempty"`
	// Color is the free note after the date, e.g. "dark blue".
	Color  string `json:"color,omitempty"`
	Urgent bool   `json:"urgent"`
	// DateIsDL records that the message carried the "tanggal = DL" note.
	DateIsDL bool   `json:"dateIsDL,omitempty"`
	Raw      string `json:"raw"`
	Issue    string `json:"issue"`
}

// DesignDeadline returns the design deadline for a posting date.
func DesignDeadline(posting timeutil.Date) timeutil.Date {
	return posting.AddDays(-DesignLeadDays)
}

// DesignDeadlineIn returns the design deadline once the year is known.
func (i Item) DesignDeadlineIn(year int) (timeutil.Date, bool) {
	if i.PostingDate != nil {
		return DesignDeadline(*i.PostingDate), true
	}
	if i.Month == 0 || i.Day == 0 || year <= 0 {
		return timeutil.Date{}, false
	}
	d, ok := validDate(year, i.Month, i.Day)
	if !ok {
		return timeutil.Date{}, false
	}
	return DesignDeadline(d), true
}

var (
	numbered = regexp.MustCompile(`^\s*([0-9]{1,3})\s*[.)]\s*(.+)$`)
	// Labelled block lines ("PJ Humas: Ana"). A separator is required.
	field = regexp.MustCompile(`(?i)^\s*(pj\s*humas|pic\s*humas|pj\s*mediatek|pj\s*med|pic\s*mediatek|pic\s*med|pj\s*desain|pj\s*design|tanggal\s*posting|tanggal|tgl|jadwal\s*posting|dl|judul|konsep)\s*[:=]\s*(.*?)\s*$`)
	// Inline PIC labels inside one template line.
	inlinePIC = regexp.MustCompile(`(?i)(?:^|[\s,;|/–—-])((?:pj|pic)\s*(?:humas|hms|mediatek|medtek|mdtk|med|desain|design))\b\s*[:=]?\s*`)
	// Inline labelled dates ("tgl 19 nov", "DL: 20/11").
	inlineDate  = regexp.MustCompile(`(?i)(?:^|[\s,;|(])(?:tanggal\s*posting|tanggal|tgl|dl|deadline|posting)\s*[:=]?\s*([0-9]{1,4}(?:\s*[-/.]\s*[0-9]{1,2}(?:\s*[-/.]\s*[0-9]{2,4})?|\s+[a-z]+(?:\s+[0-9]{4})?)?)`)
	parenthesis = regexp.MustCompile(`\(([^()]*)\)`)
	bareDate    = regexp.MustCompile(`(?i)\b([0-9]{1,2})\s+([a-z]{3,9})(?:\s+([0-9]{4}))?\b`)
	dateNote    = regexp.MustCompile(`(?i)\b(tanggal|tgl)\s*[:=]\s*(dl|deadline)\b`)
	cohortTag   = regexp.MustCompile(`(?i)\s*\b[a-z]{2,3}\s*['’‘` + "`" + `]\s*[0-9]{2}\b`)
	numericDate = regexp.MustCompile(`^([0-9]{1,4})\s*[-/.]\s*([0-9]{1,2})(?:\s*[-/.]\s*([0-9]{2,4}))?$`)
	namedDate   = regexp.MustCompile(`(?i)^([0-9]{1,2})\s*([a-z]+)\.?(?:\s+([0-9]{4}))?$`)
	dayOnly     = regexp.MustCompile(`^([0-9]{1,2})$`)
	yearWord    = regexp.MustCompile(`\b(20[0-9]{2})\b`)
	wordRe      = regexp.MustCompile(`[a-z]+`)
)

var months = map[string]int{
	"januari": 1, "january": 1, "jan": 1,
	"februari": 2, "february": 2, "feb": 2, "pebruari": 2,
	"maret": 3, "march": 3, "mar": 3, "mrt": 3,
	"april": 4, "apr": 4,
	"mei": 5, "may": 5,
	"juni": 6, "june": 6, "jun": 6,
	"juli": 7, "july": 7, "jul": 7,
	"agustus": 8, "august": 8, "agu": 8, "agt": 8, "agst": 8, "aug": 8,
	"september": 9, "sept": 9, "sep": 9,
	"oktober": 10, "october": 10, "okt": 10, "oct": 10,
	"november": 11, "nopember": 11, "nov": 11, "nop": 11,
	"desember": 12, "december": 12, "des": 12, "dec": 12,
}

// Month names long enough to be trusted when found in free header text.
var headerMonths = map[string]int{
	"januari": 1, "january": 1, "februari": 2, "february": 2, "pebruari": 2,
	"maret": 3, "march": 3, "april": 4, "mei": 5, "may": 5, "juni": 6, "june": 6,
	"juli": 7, "july": 7, "agustus": 8, "august": 8, "september": 9,
	"oktober": 10, "october": 10, "november": 11, "nopember": 11,
	"desember": 12, "december": 12,
}

func Normalize(s string) string {
	s = strings.Trim(strings.TrimSpace(s), "*_`~@")
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// stripDecor removes emoji, variation selectors and joiners, and turns keycap
// digits ("1️⃣") into "1.".
func stripDecor(s string) string {
	s = strings.ReplaceAll(s, "️⃣", ".")
	s = strings.ReplaceAll(s, "⃣", ".")
	return strings.Map(func(r rune) rune {
		switch {
		case r == '‍' || r == '​' || r == '️' || r == '︎':
			return -1
		case r >= 0x1F000 || unicode.Is(unicode.So, r) || unicode.Is(unicode.Sk, r) || unicode.Is(unicode.Cs, r):
			return -1
		case r == ' ':
			return ' '
		}
		return r
	}, s)
}

func cleanLine(s string) string {
	s = strings.TrimSpace(stripDecor(s))
	s = strings.TrimLeftFunc(s, func(r rune) bool {
		return r == '*' || r == '_' || r == '`' || r == '~' || r == '>' || r == '•' || r == '-' || r == '·' || unicode.IsSpace(r)
	})
	return strings.Trim(strings.TrimSpace(s), "*_`~")
}

func trimValue(s string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(s), ",;|–—-:*_~ "))
}

// cleanName drops the leading "@", a cohort tag ("SI'24") and a bracketed
// alias, which it returns separately.
func cleanName(s string) (name, alias string) {
	s = trimValue(s)
	if m := parenthesis.FindStringSubmatchIndex(s); m != nil {
		alias = trimValue(s[m[2]:m[3]])
		s = trimValue(s[:m[0]] + " " + s[m[1]:])
	}
	s = trimValue(cohortTag.ReplaceAllString(s, ""))
	s = strings.TrimSpace(strings.TrimPrefix(s, "@"))
	s = strings.Join(strings.Fields(s), " ")
	if Normalize(alias) == Normalize(s) {
		alias = ""
	}
	return s, alias
}

// headerMonth finds the list's month (and year) in the text before the items.
func headerMonth(line string) (int, int) {
	lower := strings.ToLower(line)
	month := 0
	for _, w := range wordRe.FindAllString(lower, -1) {
		if m, ok := headerMonths[w]; ok {
			month = m
			break
		}
	}
	if month == 0 && strings.HasPrefix(strings.TrimSpace(lower), "[") {
		for _, w := range wordRe.FindAllString(lower, -1) {
			if m, ok := months[w]; ok && len(w) >= 3 {
				month = m
				break
			}
		}
	}
	year := 0
	if month != 0 {
		if m := yearWord.FindStringSubmatch(lower); m != nil {
			year, _ = strconv.Atoi(m[1])
		}
	}
	return month, year
}

func isDadakan(line string) bool {
	w := strings.Trim(strings.ToLower(line), " :[]()*_~!.-")
	return w == "dadakan" || w == "konten dadakan" || w == "postingan dadakan" || w == "list dadakan"
}

type parsedDate struct {
	day, month, year int
	ok               bool
}

// readDate reads one date value. Day-only values take fallbackMonth.
func readDate(raw string, fallbackMonth int) parsedDate {
	value := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(raw)), " "))
	value = strings.TrimSuffix(value, ".")
	var d parsedDate
	if m := numericDate.FindStringSubmatch(value); m != nil {
		a, _ := strconv.Atoi(m[1])
		b, _ := strconv.Atoi(m[2])
		c, _ := strconv.Atoi(m[3])
		if len(m[1]) == 4 {
			d.year, d.month, d.day = a, b, c
		} else {
			d.day, d.month, d.year = a, b, c
			if m[3] != "" && len(m[3]) != 4 {
				return parsedDate{}
			}
		}
	} else if m := namedDate.FindStringSubmatch(value); m != nil {
		month, ok := months[m[2]]
		if !ok {
			return parsedDate{}
		}
		d.day, _ = strconv.Atoi(m[1])
		d.month = month
		d.year, _ = strconv.Atoi(m[3])
	} else if m := dayOnly.FindStringSubmatch(value); m != nil && fallbackMonth != 0 {
		d.day, _ = strconv.Atoi(m[1])
		d.month = fallbackMonth
	} else {
		return parsedDate{}
	}
	if d.month < 1 || d.month > 12 || d.day < 1 || d.day > 31 {
		return parsedDate{}
	}
	checkYear := d.year
	if checkYear == 0 {
		checkYear = 2000 // leap, so 29 Feb stays possible
	}
	if _, ok := validDate(checkYear, d.month, d.day); !ok {
		return parsedDate{}
	}
	d.ok = true
	return d
}

func validDate(year, month, day int) (timeutil.Date, bool) {
	date, err := timeutil.ParseDate(pad4(year) + "-" + pad(month) + "-" + pad(day))
	return date, err == nil
}

// looksLikeDate tells whether a value was meant as a date even if it is not
// a valid one, so it is reported instead of being taken as a name or title.
func looksLikeDate(raw string) bool {
	v := strings.ToLower(strings.TrimSpace(raw))
	if numericDate.MatchString(v) || dayOnly.MatchString(v) {
		return true
	}
	if m := namedDate.FindStringSubmatch(v); m != nil {
		_, ok := months[m[2]]
		return ok
	}
	return false
}

type state struct {
	month, year int // from the header
	dateIsDL    bool
}

func (st state) setDate(item *Item, text string) {
	item.DateText = text
	d := readDate(text, st.month)
	item.PostingDate, item.Day, item.Month = nil, 0, 0
	item.DesignDeadline, item.DeadlineDay, item.DeadlineMonth = nil, 0, 0
	if !d.ok {
		if dayOnly.MatchString(strings.TrimSpace(text)) && st.month == 0 {
			item.Issue += " Tanggal tanpa bulan dan bulan di judul broadcast belum terbaca."
		} else {
			item.Issue += " Format tanggal tidak didukung atau tanggal tidak ada di kalender."
		}
		return
	}
	item.Day, item.Month = d.day, d.month
	year := d.year
	if year == 0 && st.year != 0 {
		// A December list may hold January posts, and the other way round.
		year = st.year
		if st.month != 0 && st.month-d.month > 6 {
			year++
		} else if st.month != 0 && d.month-st.month > 6 {
			year--
		}
	}
	if year != 0 {
		date, ok := validDate(year, d.month, d.day)
		if !ok {
			item.Day, item.Month = 0, 0
			item.Issue += " Tanggal tidak ada di kalender tahun itu."
			return
		}
		item.PostingDate = &date
		dl := DesignDeadline(date)
		item.DesignDeadline = &dl
		item.DeadlineDay, item.DeadlineMonth = dl.Day, int(dl.Month)
		return
	}
	// Without a year the deadline is still known unless it falls in a
	// February whose length is unknown.
	if d.month == 3 && d.day <= DesignLeadDays {
		return
	}
	ref, _ := validDate(2001, d.month, d.day)
	if d.month == 2 && d.day == 29 {
		ref, _ = validDate(2000, d.month, d.day)
	}
	dl := DesignDeadline(ref)
	item.DeadlineDay, item.DeadlineMonth = dl.Day, int(dl.Month)
}

// parseInline reads one template line (number already removed). It reports
// whether the line had a PIC label or a labelled date.
func (st state) parseInline(item *Item, text string) bool {
	rest := text
	explicit := false
	dateText := ""
	dateStart, dateEnd := -1, -1
	// 1. A bracketed date, the template's own form: "(19 nov)", "(19)".
	for _, m := range parenthesis.FindAllStringSubmatchIndex(rest, -1) {
		inner := strings.TrimSpace(rest[m[2]:m[3]])
		inner = strings.TrimSpace(trimDateLabel(inner))
		if looksLikeDate(inner) {
			dateText, dateStart, dateEnd = inner, m[0], m[1]
			break
		}
	}
	// 2. A labelled date: "tgl 19 nov", "DL: 20/11".
	if dateStart < 0 {
		if m := inlineDate.FindStringSubmatchIndex(rest); m != nil && m[3] > m[2] {
			dateText, dateStart, dateEnd = strings.TrimSpace(rest[m[2]:m[3]]), m[0], m[1]
			explicit = true
		}
	}
	// 3. A bare day + month name after the title: "…, 19 nov, …".
	if dateStart < 0 {
		for _, m := range bareDate.FindAllStringSubmatchIndex(rest, -1) {
			if _, ok := months[strings.ToLower(rest[m[4]:m[5]])]; ok && m[0] > 0 {
				dateText, dateStart, dateEnd = rest[m[0]:m[1]], m[0], m[1]
				break
			}
		}
	}
	tail := ""
	if dateStart >= 0 {
		before, after := rest[:dateStart], rest[dateEnd:]
		if inlinePIC.MatchString(after) {
			rest = before + ", " + after
		} else {
			rest, tail = before, after
		}
	}
	// Split the remainder at PIC labels: title, then each labelled value.
	labels := inlinePIC.FindAllStringSubmatchIndex(rest, -1)
	if len(labels) == 0 {
		item.Title = trimValue(rest)
	} else {
		explicit = true
		item.Title = trimValue(rest[:labels[0][0]])
		for n, m := range labels {
			end := len(rest)
			if n+1 < len(labels) {
				end = labels[n+1][0]
			}
			st.setPIC(item, rest[m[2]:m[3]], rest[m[1]:end])
		}
	}
	if dateText != "" {
		st.setDate(item, dateText)
	}
	if c := trimValue(tail); c != "" {
		item.Color = strings.Join(strings.Fields(c), " ")
	}
	return explicit
}

// trimDateLabel removes "tgl"/"DL" inside brackets: "(DL 19 nov)".
func trimDateLabel(s string) string {
	lower := strings.ToLower(s)
	for _, p := range []string{"tanggal posting", "tanggal", "tgl", "deadline", "dl", "posting"} {
		if strings.HasPrefix(lower, p) {
			return strings.TrimLeft(s[len(p):], " :=.")
		}
	}
	return s
}

func picKind(label string) string {
	l := strings.ToLower(strings.Join(strings.Fields(label), ""))
	if strings.HasSuffix(l, "humas") || strings.HasSuffix(l, "hms") {
		return "humas"
	}
	return "design"
}

func (st state) setPIC(item *Item, label, value string) {
	name, alias := cleanName(value)
	if picKind(label) == "humas" {
		if item.Humas != "" && Normalize(item.Humas) != Normalize(name) {
			item.Issue += " PJ Humas ganda."
		}
		item.Humas, item.HumasAlias = name, alias
		return
	}
	if item.Design != "" && Normalize(item.Design) != Normalize(name) {
		item.Issue += " PJ desain ganda."
	}
	item.Design, item.DesignAlias = name, alias
}

// Parse returns every content once any explicit PJ/date field makes the
// message recognizable as a broadcast. Unsupported fields remain in Raw.
// Lines that start a content but miss a field carry an Issue that names the
// line and what is missing.
func Parse(body string) []Item {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	out := []Item{}
	st := state{}
	urgent := false
	recognized := false
	// inline reports whether the current item came from a one-line template
	// entry; such an item does not take following free text into Raw.
	inline := false
	for _, raw := range lines {
		if dateNote.MatchString(stripDecor(raw)) {
			st.dateIsDL = true
		}
	}
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
		if i.Issue != "" {
			i.Issue = "Baris " + strconv.Itoa(i.Line) + ": " + i.Issue
		}
	}
	start := func(n int, raw string) *Item {
		finish()
		out = append(out, Item{Position: len(out) + 1, Line: n, Urgent: urgent, DateIsDL: st.dateIsDL, Raw: raw})
		return &out[len(out)-1]
	}
	for index, raw := range lines {
		n := index + 1
		line := cleanLine(raw)
		if line == "" {
			continue
		}
		if isDadakan(line) {
			urgent = true
			continue
		}
		if dateNote.MatchString(line) {
			continue
		}
		if len(out) == 0 && st.month == 0 && !numbered.MatchString(line) && !inlinePIC.MatchString(line) {
			if m, y := headerMonth(line); m != 0 {
				st.month, st.year = m, y
				continue
			}
		}
		if m := numbered.FindStringSubmatch(line); m != nil {
			if len(out) >= 100 {
				out[len(out)-1].Issue = "Daftar melebihi 100 item; perlu tinjauan pesan lengkap."
				return out
			}
			item := start(n, raw)
			text := cleanLine(m[2])
			if f := field.FindStringSubmatch(text); f != nil && (Normalize(f[1]) == "judul" || Normalize(f[1]) == "konsep") {
				text = f[2]
			}
			if st.parseInline(item, text) {
				recognized = true
			}
			inline = item.Humas != "" || item.Design != "" || item.DateText != ""
			continue
		}
		if f := field.FindStringSubmatch(line); f != nil && len(out) > 0 {
			item := &out[len(out)-1]
			item.Raw += "\n" + raw
			recognized = true
			key, value := Normalize(f[1]), cleanLine(f[2])
			switch key {
			case "pj humas", "pic humas", "pj mediatek", "pj med", "pic mediatek", "pic med", "pj desain", "pj design":
				st.setPIC(item, key, value)
			case "judul", "konsep":
				if item.Title != "" && Normalize(item.Title) != Normalize(value) {
					item.Issue += " Judul ganda."
				}
				item.Title = value
			default:
				if item.DateText != "" && item.DateText != value {
					item.Issue += " Tanggal ganda."
				}
				st.setDate(item, value)
			}
			continue
		}
		// An unnumbered template line: it carries a PIC label after a title,
		// or, inside the list or the DADAKAN section, a bracketed date.
		loc := inlinePIC.FindStringIndex(line)
		if loc != nil && loc[0] == 0 && len(out) > 0 {
			// "pj humas @Nama" on its own line, without ":", belongs to the
			// current item.
			item := &out[len(out)-1]
			item.Raw += "\n" + raw
			recognized = true
			st.parseInline(item, item.Title+" "+line)
			continue
		}
		if loc != nil && loc[0] > 0 {
			item := start(n, raw)
			if st.parseInline(item, line) {
				recognized = true
			}
			inline = true
			continue
		}
		if (len(out) > 0 || urgent) && hasBracketDate(line, st.month) {
			item := start(n, raw)
			st.parseInline(item, line)
			inline = true
			continue
		}
		if len(out) > 0 && !inline {
			out[len(out)-1].Raw += "\n" + raw
		}
	}
	finish()
	if !recognized {
		return nil
	}
	return out
}

func hasBracketDate(line string, month int) bool {
	for _, m := range parenthesis.FindAllStringSubmatch(line, -1) {
		inner := strings.TrimSpace(trimDateLabel(strings.TrimSpace(m[1])))
		if readDate(inner, month).ok && !strings.HasPrefix(strings.TrimSpace(line), "(") {
			return true
		}
	}
	return false
}

func pad(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func pad4(n int) string {
	s := strconv.Itoa(n)
	for len(s) < 4 {
		s = "0" + s
	}
	return s
}
