package sheet

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/id"
)

// Row is a row of Tugas as the sheet has it now.
type Row struct {
	Line         int    `json:"line"` // the sheet's row number
	TaskID       id.ID  `json:"taskId"`
	Version      int64  `json:"version"`
	BaseProgress string `json:"base"`
	Progress     string `json:"progress"`
	Title        string `json:"title"`
	Assignee     string `json:"assignee,omitempty"`
	ReportedBy   string `json:"reportedBy,omitempty"`
	ReportedAt   string `json:"reportedAt,omitempty"`
	Note         string `json:"note,omitempty"`
}

// Changed reports a row the team changed: another progress, or a note.
func (r Row) Changed() bool { return r.Progress != r.BaseProgress || r.Note != "" }

// Issue is a row Space could not read, and why.
type Issue struct {
	Line   int    `json:"line"`
	Title  string `json:"title"`
	Reason string `json:"reason"`
}

// Snapshot is one published copy of the sheet.
type Snapshot struct {
	// Clock is Meta's =NOW() at the copy, the sheet's wall time read in
	// the zone it was given; ClockText is the cell as published.
	Clock     time.Time `json:"clock"`
	ClockText string    `json:"clockText"`
	Rows      []Row     `json:"rows"`
	// NoID are rows without an ID: a task typed in the sheet, for review.
	NoID   []Row   `json:"noId,omitempty"`
	Issues []Issue `json:"issues,omitempty"`
	Digest string  `json:"digest"`
}

// Parse errors: the whole copy is set aside, nothing local changes.
var (
	ErrNotWorkbook = errors.New("respons bukan workbook xlsx")
	ErrNoClock     = errors.New("tab Meta tanpa jam sheet (jam_sheet)")
	ErrNoTasks     = errors.New("tab Tugas tidak ada atau kolomnya berubah")
)

// Field limits, as the server's reconciliation keeps them.
const (
	maxReportedBy = 200
	maxReportedAt = 100
	maxNote       = 4000
	maxTitle      = 500
)

// Parse reads a published workbook. loc is the sheet's zone (Meta's clock
// carries none).
func Parse(data []byte, loc *time.Location) (Snapshot, error) {
	if !bytes.HasPrefix(data, []byte("PK")) {
		return Snapshot{}, ErrNotWorkbook
	}
	tabs, err := ReadXLSX(data)
	if err != nil {
		return Snapshot{}, fmt.Errorf("%w: %v", ErrNotWorkbook, err)
	}
	var s Snapshot
	meta := tabs[TabMeta]
	if meta == nil {
		return Snapshot{}, ErrNoClock
	}
	for _, r := range meta.Rows {
		if len(r) > 1 && strings.TrimSpace(r[0]) == ClockKey {
			s.ClockText = r[1]
			s.Clock, err = serialTime(r[1], loc)
			if err != nil {
				return Snapshot{}, ErrNoClock
			}
		}
	}
	if s.Clock.IsZero() {
		return Snapshot{}, ErrNoClock
	}
	tab := tabs[TabTasks]
	if tab == nil || len(tab.Rows) == 0 {
		return Snapshot{}, ErrNoTasks
	}
	col := map[string]int{}
	for i, h := range tab.Rows[0] {
		if name, ok := headerAliases[strings.ToLower(strings.TrimSpace(h))]; ok {
			if _, dup := col[name]; !dup {
				col[name] = i
			}
		}
	}
	for _, need := range []string{"task_id", "task_version", "base_progress", "Progres", "Judul"} {
		if _, ok := col[need]; !ok {
			return Snapshot{}, fmt.Errorf("%w: kolom %s", ErrNoTasks, need)
		}
	}
	seen := map[id.ID]int{}
	for i, cells := range tab.Rows[1:] {
		line := i + 2
		cell := func(name string) string {
			c, ok := col[name]
			if !ok || c >= len(cells) {
				return ""
			}
			return strings.TrimSpace(cells[c])
		}
		if strings.Join(cells, "") == "" {
			continue
		}
		r := Row{Line: line, Title: cell("Judul"), Assignee: cell("PJ"), ReportedBy: cell("Dilaporkan oleh"), ReportedAt: cell("Waktu laporan"), Note: cell("Catatan")}
		reject := func(reason string) { s.Issues = append(s.Issues, Issue{Line: line, Title: r.Title, Reason: reason}) }
		if len(r.ReportedBy) > maxReportedBy || len(r.ReportedAt) > maxReportedAt || len(r.Note) > maxNote || len(r.Title) > maxTitle {
			reject("isian terlalu panjang")
			continue
		}
		progress, ok := progressWords[strings.ToLower(cell("Progres"))]
		raw := cell("task_id")
		if raw == "" {
			if r.Title != "" || cell("Progres") != "" {
				r.Progress = progress
				s.NoID = append(s.NoID, r)
			}
			continue
		}
		tid, err := id.Parse(raw)
		if err != nil || tid == id.Nil {
			reject("ID tugas tidak terbaca; kolom abu-abu berubah")
			continue
		}
		r.TaskID = tid
		r.Version, err = parseVersion(cell("task_version"))
		if err != nil {
			reject("versi tugas tidak terbaca; kolom abu-abu berubah")
			continue
		}
		base, baseOK := progressWords[strings.ToLower(cell("base_progress"))]
		if !baseOK {
			reject("progres awal tidak terbaca; kolom abu-abu berubah")
			continue
		}
		r.BaseProgress = base
		switch {
		case cell("Progres") == "":
			reject("Progres kosong; pilih dari daftar")
			continue
		case !ok:
			reject("Progres \"" + cell("Progres") + "\" tidak dikenal; pilih dari daftar")
			continue
		}
		r.Progress = progress
		if first, dup := seen[tid]; dup {
			// The same task twice: neither copy is believed.
			reject("tugas yang sama muncul dua kali (juga baris " + strconv.Itoa(s.Rows[first].Line) + ")")
			s.Issues = append(s.Issues, Issue{Line: s.Rows[first].Line, Title: s.Rows[first].Title, Reason: "tugas yang sama muncul dua kali (juga baris " + strconv.Itoa(line) + ")"})
			s.Rows[first].TaskID = id.Nil
			continue
		}
		seen[tid] = len(s.Rows)
		s.Rows = append(s.Rows, r)
	}
	kept := s.Rows[:0]
	for _, r := range s.Rows {
		if r.TaskID != id.Nil {
			kept = append(kept, r)
		}
	}
	s.Rows = kept
	b, _ := json.Marshal([]any{s.Rows, s.NoID, s.Issues})
	sum := sha256.Sum256(b)
	s.Digest = hex.EncodeToString(sum[:8])
	return s, nil
}

func parseVersion(s string) (int64, error) {
	if v, err := strconv.ParseInt(s, 10, 64); err == nil && v >= 0 {
		return v, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 || f != math.Trunc(f) || f > 1e15 {
		return 0, errors.New("versi")
	}
	return int64(f), nil
}

// serialTime reads Meta's clock: a spreadsheet serial day number (days
// since 1899-12-30), as xlsx keeps dates.
func serialTime(v string, loc *time.Location) (time.Time, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || f < 40000 || f > 80000 {
		return time.Time{}, errors.New("jam sheet")
	}
	days := math.Floor(f)
	secs := math.Round((f - days) * 86400)
	t := time.Date(1899, 12, 30, 0, 0, 0, 0, loc).AddDate(0, 0, int(days)).Add(time.Duration(secs) * time.Second)
	return t, nil
}
