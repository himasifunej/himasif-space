package broadcast

import (
	"encoding/json"
	"strings"
	"testing"
)

// ownerTemplate follows the Humas template the owner shared on 25 Nov 2026,
// with every person's name replaced. Titles and layout are kept.
const ownerTemplate = `[HALO, WELCOME TO NOVEMBER MEDIATEK]

Kami dari humas izin menyampaikan list konsep postingan bulan NOVEMBER yaa!

1. open regis caketum, pj humas @dimas, pj med @Sari Putri SI'24 (19 nov) dark blue
2. mingper 15, pj humas @lala, pj med @Bima SI'23 (Bimbim) (23 nov) light b

DADAKAN

note : tanggal = DL
terima kasih, semangattt!`

type want struct {
	title, humas, design, alias, color string
	day, month, dlDay, dlMonth         int
	urgent                             bool
}

func check(t *testing.T, got Item, w want) {
	t.Helper()
	if got.Title != w.title || got.Humas != w.humas || got.Design != w.design || got.DesignAlias != w.alias || got.Color != w.color {
		t.Errorf("fields: got %q/%q/%q/%q/%q, want %+v", got.Title, got.Humas, got.Design, got.DesignAlias, got.Color, w)
	}
	if got.Day != w.day || got.Month != w.month || got.DeadlineDay != w.dlDay || got.DeadlineMonth != w.dlMonth {
		t.Errorf("%q dates: posting %d/%d deadline %d/%d, want %d/%d and %d/%d", got.Title, got.Day, got.Month, got.DeadlineDay, got.DeadlineMonth, w.day, w.month, w.dlDay, w.dlMonth)
	}
	if got.Urgent != w.urgent {
		t.Errorf("%q urgent %v", got.Title, got.Urgent)
	}
	if got.Issue != "" {
		t.Errorf("%q unexpected issue %q", got.Title, got.Issue)
	}
}

func TestOwnerTemplate(t *testing.T) {
	items := Parse(ownerTemplate)
	if len(items) != 2 {
		t.Fatalf("items %+v", items)
	}
	check(t, items[0], want{title: "open regis caketum", humas: "dimas", design: "Sari Putri", color: "dark blue", day: 19, month: 11, dlDay: 17, dlMonth: 11})
	check(t, items[1], want{title: "mingper 15", humas: "lala", design: "Bima", alias: "Bimbim", color: "light b", day: 23, month: 11, dlDay: 21, dlMonth: 11})
	for _, it := range items {
		if !it.DateIsDL {
			t.Errorf("%q missed the tanggal = DL note", it.Title)
		}
		if it.PostingDate != nil || it.DesignDeadline != nil {
			t.Errorf("%q: year guessed", it.Title)
		}
		if it.Line == 0 || !strings.Contains(it.Raw, it.Title) || strings.Contains(it.Raw, "terima kasih") {
			t.Errorf("%q raw/line %d %q", it.Title, it.Line, it.Raw)
		}
	}
	if items[0].Line != 5 || items[1].Line != 6 {
		t.Errorf("lines %d %d", items[0].Line, items[1].Line)
	}
}

func TestDadakanSectionAndYearFromHeader(t *testing.T) {
	msg := `*[HALO, WELCOME TO DESEMBER 2026 MEDIATEK]* 🎉
1️⃣ Hari Guru 🏫 , PJ HUMAS @nina , PJ MED @rafi (5 des) Dark Blue
2) rekap ukm — pic humas @tono — pj mediatek @Wulan (2 JAN)

DADAKAN:
- ucapan duka, pj humas @nina, pj med @rafi (10)
poster lomba, pj hms @tono, pj med @wulan (12 desember)`
	items := Parse(msg)
	if len(items) != 4 {
		t.Fatalf("items %d %+v", len(items), items)
	}
	check(t, items[0], want{title: "Hari Guru", humas: "nina", design: "rafi", color: "Dark Blue", day: 5, month: 12, dlDay: 3, dlMonth: 12})
	if items[0].PostingDate == nil || items[0].PostingDate.String() != "2026-12-05" || items[0].DesignDeadline.String() != "2026-12-03" {
		t.Errorf("year from header: %+v", items[0])
	}
	check(t, items[1], want{title: "rekap ukm", humas: "tono", design: "Wulan", day: 2, month: 1, dlDay: 31, dlMonth: 12})
	if items[1].PostingDate == nil || items[1].PostingDate.String() != "2027-01-02" || items[1].DesignDeadline.String() != "2026-12-31" {
		t.Errorf("January post in a December list: %+v %+v", items[1].PostingDate, items[1].DesignDeadline)
	}
	check(t, items[2], want{title: "ucapan duka", humas: "nina", design: "rafi", day: 10, month: 12, dlDay: 8, dlMonth: 12, urgent: true})
	check(t, items[3], want{title: "poster lomba", humas: "tono", design: "wulan", day: 12, month: 12, dlDay: 10, dlMonth: 12, urgent: true})
	if items[0].DateIsDL {
		t.Error("DL note invented")
	}
}

func TestDayOnlyNeedsHeaderMonth(t *testing.T) {
	items := Parse("1. poster, pj humas @ana, pj med @budi (19)")
	if len(items) != 1 || items[0].Month != 0 || !strings.Contains(items[0].Issue, "bulan") || !strings.HasPrefix(items[0].Issue, "Baris 1:") {
		t.Fatalf("day without month: %+v", items)
	}
	items = Parse("list konsep bulan Oktober\n1. poster, pj humas @ana, pj med @budi (19)")
	if len(items) != 1 || items[0].Day != 19 || items[0].Month != 10 || items[0].Issue != "" {
		t.Fatalf("month from intro line: %+v", items)
	}
}

func TestUnreadableLinesAreReported(t *testing.T) {
	msg := `[WELCOME TO NOVEMBER]
1. poster A, pj humas @ana, pj med @budi (31 nov)
2. poster B, pj med @budi (20 nov)
3. poster C pj humas @ana`
	items := Parse(msg)
	if len(items) != 3 {
		t.Fatalf("items %+v", items)
	}
	for i, want := range []string{"Baris 2: Format tanggal", "Baris 3: PJ Humas belum terbaca.", "Baris 4: Tanggal posting belum terbaca."} {
		if !strings.HasPrefix(items[i].Issue, want) {
			t.Errorf("item %d issue %q, want prefix %q", i+1, items[i].Issue, want)
		}
	}
	if items[0].Month != 0 || items[0].PostingDate != nil {
		t.Error("invalid date kept")
	}
	if items[2].Title != "poster C" {
		t.Errorf("title %q", items[2].Title)
	}
}

func TestMarchDeadlineWithoutYearWaitsForYear(t *testing.T) {
	items := Parse("1. poster, pj humas @ana (1 maret)")
	if len(items) != 1 || items[0].DeadlineMonth != 0 || items[0].Issue != "" {
		t.Fatalf("%+v", items)
	}
	for year, want := range map[int]string{2027: "2027-02-27", 2028: "2028-02-28"} {
		d, ok := items[0].DesignDeadlineIn(year)
		if !ok || d.String() != want {
			t.Errorf("%d: %v %v", year, d, ok)
		}
	}
}

func TestOtherDateForms(t *testing.T) {
	for msg, wantMonth := range map[string]int{
		"1. a, pj humas @x, tgl 19 nov":          11,
		"1. a, pj humas @x, DL: 19/11":           11,
		"1. a, pj humas @x, 19 sept, navy":       9,
		"1. a, pj humas @x (DL 3 agt)":           8,
		"1. a, pj humas @x (2026-04-07)":         4,
		"1. a pj humas @x pj desain @y (7 mei.)": 5,
	} {
		items := Parse(msg)
		if len(items) != 1 || items[0].Month != wantMonth || items[0].Humas != "x" || items[0].Title != "a" {
			t.Errorf("%q: %+v", msg, items)
		}
	}
	items := Parse("1. a, pj humas @x, 19 sept, navy")
	if items[0].Color != "" {
		// the color note is read only after a bracketed date
		t.Logf("color %q", items[0].Color)
	}
	if items := Parse("1. a, pj humas @x (19/11/26)"); items[0].Month != 0 || items[0].Issue == "" {
		t.Errorf("two-digit year guessed: %+v", items)
	}
}

// Synthetic fixtures test the declared grammar, not acceptance of an unseen live template.
func TestExplicitListPreservesMissingYearAndAmbiguity(t *testing.T) {
	items := Parse("1. Poster A\nPJ Humas: Ana\nTanggal posting: 8 November\n2. Poster B\nPJ Humas: Budi\nPJ Humas: Citra\nTanggal: 31/02/2026")
	if len(items) != 2 {
		t.Fatalf("items %+v", items)
	}
	if items[0].PostingDate != nil || items[0].Day != 8 || items[0].Month != 11 || items[0].DeadlineDay != 6 {
		t.Fatalf("missing year guessed: %+v", items[0])
	}
	if items[1].Issue == "" || items[1].PostingDate != nil {
		t.Fatal("ambiguous/invalid item treated as safe")
	}
	if got := Parse("1. Hai teman\n2. Sudah selesai"); len(got) != 0 {
		t.Fatal("conversation parsed as broadcast")
	}
	if got := Parse("rapat besok (19 nov) ya\n1. bawa laptop (20 nov)"); len(got) != 0 {
		t.Fatalf("dates without PJ parsed as broadcast: %+v", got)
	}
}

func TestPICWithoutColonOnItsOwnLine(t *testing.T) {
	items := Parse("1. Poster A\npj humas @ana\npj med @budi SI'25\nTanggal: 8 Nov 2026")
	if len(items) != 1 || items[0].Humas != "ana" || items[0].Design != "budi" || items[0].DesignDeadline == nil || items[0].DesignDeadline.String() != "2026-11-06" {
		t.Fatalf("%+v", items)
	}
}

func TestJSONKeepsOldFields(t *testing.T) {
	b, _ := json.Marshal(Parse(ownerTemplate)[0])
	for _, k := range []string{`"title"`, `"humas"`, `"design"`, `"dateText"`, `"Day"`, `"Month"`, `"urgent"`, `"raw"`, `"issue"`, `"deadlineDay"`, `"color"`} {
		if !strings.Contains(string(b), k) {
			t.Errorf("json misses %s: %s", k, b)
		}
	}
}
