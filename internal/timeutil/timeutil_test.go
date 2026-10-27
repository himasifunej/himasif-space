package timeutil

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAddDaysAcrossMonthsAndYears(t *testing.T) {
	cases := []struct {
		from string
		n    int
		want string
	}{
		{"2026-10-20", -2, "2026-10-18"},
		{"2026-11-01", -2, "2026-10-30"},
		{"2027-01-01", -2, "2026-12-30"},
		{"2028-03-01", -1, "2028-02-29"},
		{"2026-11-30", 1, "2026-12-01"},
	}
	for _, c := range cases {
		d, err := ParseDate(c.from)
		if err != nil {
			t.Fatal(err)
		}
		if got := d.AddDays(c.n).String(); got != c.want {
			t.Errorf("%s %+d = %s, want %s", c.from, c.n, got, c.want)
		}
	}
}

func TestDateJSONRoundTrip(t *testing.T) {
	type wrap struct {
		On Date `json:"on"`
	}
	b, err := json.Marshal(wrap{On: Date{2025, time.December, 1}})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"on":"2025-12-01"}` {
		t.Fatalf("marshal = %s", b)
	}
	var w wrap
	if err := json.Unmarshal(b, &w); err != nil {
		t.Fatal(err)
	}
	if w.On != (Date{2025, time.December, 1}) {
		t.Fatalf("unmarshal = %v", w.On)
	}
	if err := json.Unmarshal([]byte(`{"on":"1 Desember"}`), &w); err == nil {
		t.Fatal("expected an error for a malformed date")
	}
}

func TestAtUsesZoneWithoutShiftingTheDate(t *testing.T) {
	loc, err := LoadZone("")
	if err != nil {
		t.Fatal(err)
	}
	d := Date{2026, time.October, 18}
	at := d.At(6, 0, loc)
	if got := at.UTC().Format(time.RFC3339); got != "2026-10-17T23:00:00Z" {
		t.Fatalf("06.00 WIB in UTC = %s", got)
	}
	if DateOf(at) != d {
		t.Fatalf("DateOf(At) = %v, want %v", DateOf(at), d)
	}
}
