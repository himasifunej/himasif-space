package sheet

import (
	"strconv"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/id"
)

// The workbook's layout. Tab Meta holds the sheet's clock (=NOW(), which
// Google recalculates every minute when the sheet is set to): it tells a
// newer published copy from an older one. Tab Tugas has the columns the
// team reads and edits, then the IDs Space matches rows by, hidden.
const (
	TabMeta  = "Meta"
	TabTasks = "Tugas"
	// TemplateVersion is the layout's version, in Meta.
	TemplateVersion = "1"
	// ClockKey names the clock's row in Meta.
	ClockKey = "jam_sheet"
)

// Columns of Tugas: what is shown, then the hidden IDs.
var (
	shownColumns = []string{"Judul", "PJ", "Space", "Tenggat", "Progres", "Dilaporkan oleh", "Waktu laporan", "Catatan"}
	idColumns    = []string{"task_id", "task_version", "base_progress"}
)

// Header aliases: the spike's template and the team's own renames still
// read.
var headerAliases = map[string]string{
	"judul": "Judul", "pj": "PJ", "space": "Space", "divisi": "Space", "tenggat": "Tenggat", "deadline": "Tenggat",
	"progres": "Progres", "progress": "Progres", "dilaporkan oleh": "Dilaporkan oleh", "pelapor": "Dilaporkan oleh",
	"waktu laporan": "Waktu laporan", "catatan": "Catatan", "task_id": "task_id", "task_version": "task_version", "base_progress": "base_progress",
}

// Progress values in the sheet: the dropdown's words, and the codes.
var progressWords = map[string]string{
	"belum mulai": "todo", "belum": "todo", "todo": "todo",
	"dikerjakan": "doing", "sedang dikerjakan": "doing", "doing": "doing",
	"selesai": "done", "done": "done",
	"diserahkan": "submitted", "submitted": "submitted",
	"dibatalkan": "cancelled", "cancelled": "cancelled",
}

// ProgressWord is how the sheet writes a progress.
func ProgressWord(p string) string {
	switch p {
	case "todo":
		return "Belum mulai"
	case "doing":
		return "Dikerjakan"
	case "done":
		return "Selesai"
	case "submitted":
		return "Diserahkan"
	case "cancelled":
		return "Dibatalkan"
	}
	return p
}

// Choices of the Progres dropdown: what a sheet may set. Submitting and
// cancelling need Space's own flows.
var progressChoices = []string{"Belum mulai", "Dikerjakan", "Selesai"}

// Task is a task as Space writes it into the sheet, from the device cache.
type Task struct {
	ID       id.ID
	Version  int64
	Progress string
	Title    string
	Assignee string
	Space    string
	Deadline string
}

// safeText keeps a cell text: spreadsheet formulas stay text, line breaks
// and tabs become spaces.
func safeText(s string) string {
	s = strings.NewReplacer("\t", " ", "\r", " ", "\n", " ").Replace(s)
	if s != "" && strings.ContainsRune("=+-@", rune(s[0])) {
		s = "'" + s
	}
	return s
}

func taskCells(t Task) []string {
	return []string{safeText(t.Title), safeText(t.Assignee), safeText(t.Space), safeText(t.Deadline), ProgressWord(t.Progress), "", "", "",
		t.ID.String(), strconv.FormatInt(t.Version, 10), t.Progress}
}

// Workbook writes the sheet with tasks: the template, filled. Import it into
// the linked sheet (File → Impor → Ganti spreadsheet), or paste TSV.
func Workbook(tasks []Task, made time.Time) ([]byte, error) {
	meta := WriteTab{Name: TabMeta, Widths: []float64{18, 70}, Rows: [][]Cell{
		{{Text: "space_darurat", Style: styleHead}, {Text: TemplateVersion, Style: styleHead}},
		{{Text: ClockKey}, {Formula: "NOW()", Style: styleClock}},
		{{Text: "petunjuk"}, {Text: "Atur File → Setelan: zona waktu (GMT+07:00) Jakarta dan Penghitungan \"Saat ada perubahan dan setiap menit\". Jangan ubah tab ini, nama tab, atau kolom abu-abu di Tugas."}},
		{{Text: "isi"}, {Text: "Tugas dari cache perangkat Space, " + made.Format("2006-01-02 15:04 MST") + "."}},
	}}
	head := []Cell{}
	for _, h := range shownColumns {
		head = append(head, Cell{Text: h, Style: styleHead})
	}
	for _, h := range idColumns {
		head = append(head, Cell{Text: h, Style: styleIDHd})
	}
	rows := [][]Cell{head}
	for _, t := range tasks {
		row := []Cell{}
		for i, v := range taskCells(t) {
			style := styleText
			if i >= len(shownColumns) {
				style = styleID
			}
			row = append(row, Cell{Text: v, Style: style})
		}
		rows = append(rows, row)
	}
	tasksTab := WriteTab{Name: TabTasks, Rows: rows, Widths: []float64{40, 18, 16, 18, 14, 18, 18, 36, 38, 12, 12},
		Hidden: []int{8, 9, 10}, Lists: map[int][]string{4: progressChoices}, ListRows: max(len(rows)+200, 500)}
	return WriteXLSX([]WriteTab{meta, tasksTab})
}

// TSV is Tugas as text to paste at A1 of the tab: the header, then a row
// per task, the hidden IDs last.
func TSV(tasks []Task) string {
	var b strings.Builder
	b.WriteString(strings.Join(append(append([]string{}, shownColumns...), idColumns...), "\t"))
	for _, t := range tasks {
		b.WriteString("\n")
		b.WriteString(strings.Join(taskCells(t), "\t"))
	}
	return b.String()
}
