package sheet

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/id"
)

var jakarta = time.FixedZone("WIB", 7*3600)

// published builds a copy as Google publishes it: Meta's clock as a serial
// number, Tugas with the given rows (cells as the team left them).
func published(t *testing.T, serial string, header []string, rows ...[]string) []byte {
	t.Helper()
	meta := WriteTab{Name: TabMeta, Rows: [][]Cell{{{Text: "space_darurat"}, {Text: "1"}}, {{Text: ClockKey}, {Text: serial}}}}
	tab := WriteTab{Name: TabTasks, Hidden: []int{8, 9, 10}}
	head := []Cell{}
	for _, h := range header {
		head = append(head, Cell{Text: h})
	}
	tab.Rows = append(tab.Rows, head)
	for _, r := range rows {
		row := []Cell{}
		for _, v := range r {
			row = append(row, Cell{Text: v})
		}
		tab.Rows = append(tab.Rows, row)
	}
	b, err := WriteXLSX([]WriteTab{meta, tab})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

var header = append(append([]string{}, shownColumns...), idColumns...)

func row(tid id.ID, title, progress, base string, extra ...string) []string {
	r := []string{title, "Anggota A", "Divisi 1", "2026-11-20", progress, "", "", "", tid.String(), "3", base}
	for i, v := range extra {
		r[5+i] = v
	}
	return r
}

func TestWorkbookRoundTripKeepsHiddenIDsAndDropdown(t *testing.T) {
	tasks := []Task{{ID: id.New(), Version: 4, Progress: "doing", Title: "=HYPERLINK(\"x\")", Assignee: "Anggota A", Space: "Divisi 1", Deadline: "20 Nov 2026"}}
	b, err := Workbook(tasks, time.Date(2026, 11, 12, 9, 0, 0, 0, jakarta))
	if err != nil {
		t.Fatal(err)
	}
	tabs, err := ReadXLSX(b)
	if err != nil {
		t.Fatal(err)
	}
	tab := tabs[TabTasks]
	if tab == nil || len(tab.Rows) != 2 {
		t.Fatalf("Tugas: %+v", tab)
	}
	if !tab.Hidden[8] || !tab.Hidden[9] || !tab.Hidden[10] || tab.Hidden[4] {
		t.Fatalf("hidden columns: %v", tab.Hidden)
	}
	got := tab.Rows[1]
	if got[0] != "'=HYPERLINK(\"x\")" || got[4] != "Dikerjakan" || got[8] != tasks[0].ID.String() || got[9] != "4" || got[10] != "doing" {
		t.Fatalf("row: %q", got)
	}
	if !strings.Contains(string(mustUnzip(t, b, "xl/worksheets/sheet2.xml")), `<formula1>"Belum mulai,Dikerjakan,Selesai"</formula1>`) {
		t.Fatal("no Progres dropdown")
	}
	if !strings.Contains(string(mustUnzip(t, b, "xl/worksheets/sheet1.xml")), "<f>NOW()</f>") {
		t.Fatal("no sheet clock")
	}
	tsv := TSV(tasks)
	if lines := strings.Split(tsv, "\n"); len(lines) != 2 || strings.Count(lines[1], "\t") != 10 || !strings.HasSuffix(lines[1], "\t4\tdoing") {
		t.Fatalf("TSV: %q", tsv)
	}
}

func mustUnzip(t *testing.T, b []byte, name string) []byte {
	t.Helper()
	z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	f, err := z.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestParseReadsRowsAndSetsAsideWhatItCannotBelieve(t *testing.T) {
	a, b, c, d := id.New(), id.New(), id.New(), id.New()
	data := published(t, "46338.35602", header,
		row(a, "Poster", "Dikerjakan", "todo", "Raka", "13.38", "sudah draf"),
		row(b, "Kas", "done", "doing"),
		[]string{"Tugas baru", "", "", "", "Belum mulai"},
		row(c, "Proposal", "Hampir", "todo"),
		row(d, "Satu", "Belum mulai", "todo"),
		row(d, "Dua", "Dikerjakan", "todo"),
		[]string{"Rusak", "", "", "", "Dikerjakan", "", "", "", "bukan-id", "3", "todo"},
		[]string{"Panjang", "", "", "", "Dikerjakan", "", "", strings.Repeat("x", 4001), id.New().String(), "3", "todo"},
		[]string{},
	)
	s, err := Parse(data, jakarta)
	if err != nil {
		t.Fatal(err)
	}
	if s.Clock.Format("2006-01-02 15:04:05") != "2026-11-12 08:32:40" {
		t.Fatalf("clock %v", s.Clock)
	}
	if len(s.Rows) != 2 || s.Rows[0].TaskID != a || s.Rows[0].Progress != "doing" || s.Rows[0].ReportedBy != "Raka" || s.Rows[1].Progress != "done" {
		t.Fatalf("rows %+v", s.Rows)
	}
	if len(s.NoID) != 1 || s.NoID[0].Title != "Tugas baru" {
		t.Fatalf("no ID %+v", s.NoID)
	}
	reasons := fmt.Sprint(s.Issues)
	for _, want := range []string{"tidak dikenal", "dua kali", "ID tugas tidak terbaca", "terlalu panjang"} {
		if !strings.Contains(reasons, want) {
			t.Fatalf("no issue %q in %s", want, reasons)
		}
	}
	if len(s.Issues) != 5 {
		t.Fatalf("issues %v", s.Issues)
	}
}

func TestParseRejectsCopiesWithoutClockOrLayout(t *testing.T) {
	if _, err := Parse([]byte("<!doctype html><html>"), jakarta); !errors.Is(err, ErrNotWorkbook) {
		t.Fatalf("html: %v", err)
	}
	if _, err := Parse(published(t, "", header), jakarta); !errors.Is(err, ErrNoClock) {
		t.Fatalf("no clock: %v", err)
	}
	if _, err := Parse(published(t, "46338.5", []string{"Judul", "Progres"}), jakarta); !errors.Is(err, ErrNoTasks) {
		t.Fatalf("no ID columns: %v", err)
	}
	// The spike's template (owner's sheet) reads: IDs first, Divisi, Deadline.
	spike := []string{"space_row_id", "task_id", "task_version", "base_progress", "Judul", "PJ", "Divisi", "Deadline", "Progres", "Dilaporkan oleh", "Waktu laporan", "Catatan"}
	tid := id.New()
	s, err := Parse(published(t, "46338.5", spike, []string{id.New().String(), tid.String(), "3", "todo", "[DUMMY] Poster", "Anggota A", "Divisi 1", "2026-11-20", "todo"}), jakarta)
	if err != nil || len(s.Rows) != 1 || s.Rows[0].TaskID != tid {
		t.Fatalf("spike layout: %v %+v", err, s)
	}
}

func snap(serial string, rows ...Row) Snapshot {
	c, _ := serialTime(serial, jakarta)
	return Snapshot{Clock: c, Rows: rows}
}

func TestFollowKeepsTheNewestCopyAndSetsOlderOnesAside(t *testing.T) {
	a := id.New()
	var f Follow
	fresh := snap("46338.36", Row{TaskID: a, BaseProgress: "todo", Progress: "doing", Title: "Poster"})
	if out := f.Offer(fresh); out.Kind != OutApplied || len(out.Changes) != 1 || out.Changes[0].From != "todo" || out.Changes[0].To != "doing" {
		t.Fatalf("first: %+v", out)
	}
	// Google's cache then serves the copy of 28 minutes earlier: the
	// progress must not go back.
	old := snap("46338.34", Row{TaskID: a, BaseProgress: "todo", Progress: "todo", Title: "Poster"})
	if out := f.Offer(old); out.Kind != OutStale {
		t.Fatalf("older copy: %+v", out)
	}
	if out := f.Offer(fresh); out.Kind != OutStale {
		t.Fatalf("same clock again: %+v", out)
	}
	if r, _ := f.Row(a); r.Progress != "doing" {
		t.Fatalf("row went back to %q", r.Progress)
	}
	// A newer copy without the row: it stays, marked missing.
	if out := f.Offer(snap("46338.37")); out.Kind != OutApplied || out.Gone != 1 {
		t.Fatalf("row gone: %+v", out)
	}
	if r, ok := f.Row(a); !ok || r.Progress != "doing" || !f.Missing[a.String()] {
		t.Fatalf("deleted row was dropped: %+v %v", r, f.Missing)
	}
	if len(f.Changed()) != 1 {
		t.Fatalf("changed %v", f.Changed())
	}
}

func TestFollowHoldsMassChanges(t *testing.T) {
	var f Follow
	var rows []Row
	for i := 0; i < 40; i++ {
		rows = append(rows, Row{TaskID: id.New(), BaseProgress: "todo", Progress: "todo"})
	}
	f.Offer(snap("46338.30", rows...))
	changed := append([]Row{}, rows...)
	for i := 0; i <= MassChange; i++ {
		changed[i].Progress = "done"
	}
	if out := f.Offer(snap("46338.31", changed...)); out.Kind != OutHeld || out.Held != MassChange+1 || f.Held == nil {
		t.Fatalf("31 changes: %+v", out)
	}
	if r, _ := f.Row(rows[0].TaskID); r.Progress != "todo" {
		t.Fatal("held copy was applied")
	}
	// The sheet is emptied: also held, not applied.
	if out := f.Offer(snap("46338.32")); out.Kind != OutHeld || out.Held != 40 {
		t.Fatalf("emptied: %+v", out)
	}
	// A later copy with a few changes goes through and drops the held one.
	few := append([]Row{}, rows...)
	few[0].Progress = "doing"
	if out := f.Offer(snap("46338.33", few...)); out.Kind != OutApplied || f.Held != nil {
		t.Fatalf("few: %+v", out)
	}
	f.Offer(snap("46338.34", changed...))
	if out := f.AcceptHeld(); out.Kind != OutApplied || len(out.Changes) != MassChange+1 {
		t.Fatalf("accept: %+v", out)
	}
}

func TestFetchSortsOutWhatGoogleSends(t *testing.T) {
	good := published(t, "46338.36", header, row(id.New(), "Poster", "Dikerjakan", "todo"))
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Write(good)
	})
	mux.HandleFunc("/html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("<!doctype html><title>Sign in</title>"))
	})
	mux.HandleFunc("/html-octet", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html><body>moved</body></html>")) })
	mux.HandleFunc("/rate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1200")
		w.WriteHeader(429)
	})
	mux.HandleFunc("/down", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) })
	mux.HandleFunc("/gone", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	mux.HandleFunc("/junk", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("PK not a zip")) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	s, err := Fetch(context.Background(), srv.Client(), srv.URL+"/ok", jakarta)
	if err != nil || len(s.Rows) != 1 {
		t.Fatalf("ok: %v %+v", err, s)
	}
	for path, kind := range map[string]string{"/html": FailHTML, "/html-octet": FailHTML, "/rate": FailRate, "/down": FailServer, "/gone": FailHTTP, "/junk": FailFormat} {
		_, err := Fetch(context.Background(), srv.Client(), srv.URL+path, jakarta)
		var fe *FetchError
		if !errors.As(err, &fe) || fe.Kind != kind || fe.Error() == "" {
			t.Fatalf("%s: %v", path, err)
		}
		if path == "/rate" && fe.RetryAfter != 20*time.Minute {
			t.Fatalf("retry-after %v", fe.RetryAfter)
		}
	}
	srv.Close()
	if _, err := Fetch(context.Background(), srv.Client(), srv.URL+"/ok", jakarta); err == nil {
		t.Fatal("closed server fetched")
	}
}

func TestNextDelayBacksOff(t *testing.T) {
	for _, c := range []struct {
		failures int
		err      error
		jitter   float64
		want     time.Duration
	}{
		{0, nil, 0, 3 * time.Minute},
		{0, nil, .5, 3*time.Minute + 30*time.Second},
		{1, &FetchError{Kind: FailHTML}, 0, 6 * time.Minute},
		{2, &FetchError{Kind: FailServer}, 0, 12 * time.Minute},
		{5, &FetchError{Kind: FailNetwork}, 0, 15 * time.Minute},
		{1, &FetchError{Kind: FailRate, RetryAfter: 20 * time.Minute}, 0, 20 * time.Minute},
		{1, &FetchError{Kind: FailRate, RetryAfter: 2 * time.Hour}, 0, 30 * time.Minute},
	} {
		if got := NextDelay(c.failures, c.err, c.jitter); got != c.want {
			t.Fatalf("%+v: %v", c, got)
		}
	}
}

func TestLinks(t *testing.T) {
	pub, err := PublishURL(" https://docs.google.com/spreadsheets/d/e/2PACX-1vRzZyKg1JpJV7OMNS9AIkbn5ZAzw/pubhtml?gid=0&single=true ")
	if err != nil || pub != "https://docs.google.com/spreadsheets/d/e/2PACX-1vRzZyKg1JpJV7OMNS9AIkbn5ZAzw/pub?output=xlsx" {
		t.Fatalf("publish: %q %v", pub, err)
	}
	edit, err := EditURL("https://docs.google.com/spreadsheets/d/1HIJLuUccYpo_yl_xwLEzJ_6vk2xeChyDct1CL2ucWSc/edit?gid=863666945#gid=863666945")
	if err != nil || edit != "https://docs.google.com/spreadsheets/d/1HIJLuUccYpo_yl_xwLEzJ_6vk2xeChyDct1CL2ucWSc/edit?usp=sharing" {
		t.Fatalf("edit: %q %v", edit, err)
	}
	for _, bad := range []string{"https://docs.google.com/spreadsheets/d/e/2PACX-1vRzZyKg1JpJV7OMNS9AIkbn5ZAzw/pub", "http://docs.google.com/spreadsheets/d/1HIJLuUccYpo_yl_xwLEzJ/edit", "https://evil.example/spreadsheets/d/1HIJLuUccYpo_yl_xwLEzJ/edit"} {
		if _, err := EditURL(bad); err == nil {
			t.Fatalf("edit accepted %q", bad)
		}
	}
	if _, err := PublishURL(edit); err == nil {
		t.Fatal("publish accepted an edit link")
	}
	if got := RowURL(edit, "748986563", 14); got != "https://docs.google.com/spreadsheets/d/1HIJLuUccYpo_yl_xwLEzJ_6vk2xeChyDct1CL2ucWSc/edit#gid=748986563&range=A14" {
		t.Fatalf("row: %q", got)
	}
	if !strings.HasPrefix(WhatsAppURL(ShareText(edit)), "https://wa.me/?text=Space+sedang") {
		t.Fatal(WhatsAppURL(ShareText(edit)))
	}
}
