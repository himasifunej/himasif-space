// Package timeutil holds Space's rules for time.
//
// Instants are stored in UTC and shown in the zone of the space they belong
// to, WIB (Asia/Jakarta) by default. A calendar date without a time of day,
// such as a posting date imported from a spreadsheet, is a Date: it is never
// given a made-up hour (PRD MED-04, MED-05, NFR-10).
package timeutil

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// DefaultZoneName is the zone of spaces that have not chosen another.
const DefaultZoneName = "Asia/Jakarta"

// wib is UTC+7 with no daylight saving time, used when the system has no
// time zone database (as some minimal containers and browsers).
var wib = time.FixedZone("WIB", 7*60*60)

// LoadZone returns the named zone, with WIB as the fallback for the
// default zone when the system lacks a zone database.
func LoadZone(name string) (*time.Location, error) {
	if name == "" {
		name = DefaultZoneName
	}
	loc, err := time.LoadLocation(name)
	if err != nil && name == DefaultZoneName {
		return wib, nil
	}
	return loc, err
}

// Date is a calendar date with no time of day and no zone.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// DateOf returns the calendar date of t in t's zone.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{y, m, d}
}

// ParseDate reads a date written as YYYY-MM-DD.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return Date{}, fmt.Errorf("tanggal %q tidak valid, gunakan format YYYY-MM-DD", s)
	}
	return DateOf(t), nil
}

// String writes the date as YYYY-MM-DD.
func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

// IsZero reports whether d is the zero date.
func (d Date) IsZero() bool { return d == Date{} }

// AddDays returns the date n days after d (before it for negative n),
// across months and years.
func (d Date) AddDays(n int) Date {
	return DateOf(time.Date(d.Year, d.Month, d.Day+n, 0, 0, 0, 0, time.UTC))
}

// Before reports whether d comes before e.
func (d Date) Before(e Date) bool { return d.Compare(e) < 0 }

// Compare returns -1, 0 or +1 as d is before, equal to or after e.
func (d Date) Compare(e Date) int {
	a, b := d.utc(), e.utc()
	return a.Compare(b)
}

// At returns the instant of the given time of day on d in loc.
func (d Date) At(hour, min int, loc *time.Location) time.Time {
	return time.Date(d.Year, d.Month, d.Day, hour, min, 0, 0, loc)
}

func (d Date) utc() time.Time { return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC) }

// MarshalText writes the date as YYYY-MM-DD, for JSON.
func (d Date) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

// UnmarshalText reads a date written as YYYY-MM-DD.
func (d *Date) UnmarshalText(b []byte) error {
	v, err := ParseDate(string(b))
	if err != nil {
		return err
	}
	*d = v
	return nil
}

// Value stores the date in a database date column.
func (d Date) Value() (driver.Value, error) { return d.String(), nil }

// Scan reads the date from a database date column.
func (d *Date) Scan(src any) error {
	switch v := src.(type) {
	case time.Time:
		*d = DateOf(v.UTC())
		return nil
	case string:
		return d.UnmarshalText([]byte(v))
	case []byte:
		return d.UnmarshalText(v)
	}
	return fmt.Errorf("timeutil: cannot scan %T into Date", src)
}
