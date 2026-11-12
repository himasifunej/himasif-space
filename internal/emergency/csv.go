// Package emergency reads the editable change columns of Space's emergency
// spreadsheet. Canonical baseline data always comes from the server snapshot.
package emergency

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

var columns = []string{"snapshot_id", "row_id", "task_id", "captured_at_utc", "task_version", "title", "expected_output", "assignee", "deadline_kind", "deadline_value", "time_zone", "base_progress", "base_match_status", "new_progress", "reported_at", "reported_by", "change_note"}

func ParseChanges(data []byte) ([]api.EmergencyChange, error) {
	if len(data) > 32<<20 || !utf8.Valid(data) {
		return nil, errors.New("CSV harus UTF-8 dan maksimal 32 MiB")
	}
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})))
	header, err := reader.Read()
	if err != nil {
		return nil, errors.New("header CSV darurat tidak dapat dibaca")
	}
	indexes := map[string]int{}
	for i, key := range header {
		key = strings.TrimSpace(key)
		if _, duplicate := indexes[key]; duplicate {
			return nil, errors.New("header CSV berulang")
		}
		indexes[key] = i
	}
	if len(header) != len(columns) {
		return nil, errors.New("gunakan seluruh kolom dari ekspor Space; jangan menghapus/menambah kolom")
	}
	for _, key := range columns {
		if _, ok := indexes[key]; !ok {
			return nil, fmt.Errorf("kolom CSV wajib tidak ditemukan: %s", key)
		}
	}
	changes := []api.EmergencyChange{}
	seen := map[id.ID]bool{}
	for rowNumber := 2; ; rowNumber++ {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("baris %d CSV tidak valid", rowNumber)
		}
		if rowNumber > 5001 {
			return nil, errors.New("CSV melebihi 5000 baris tugas")
		}
		cell := func(key string) string { return strings.TrimSpace(row[indexes[key]]) }
		progress, at, by, note := cell("new_progress"), cell("reported_at"), cell("reported_by"), cell("change_note")
		if progress == "" && at == "" && by == "" && note == "" {
			continue
		}
		snapshot, err := id.Parse(cell("snapshot_id"))
		if err != nil {
			return nil, fmt.Errorf("snapshot_id baris %d tidak valid", rowNumber)
		}
		rid, err := id.Parse(cell("row_id"))
		if err != nil || seen[rid] {
			return nil, fmt.Errorf("row_id baris %d tidak valid atau berulang", rowNumber)
		}
		seen[rid] = true
		tid, err := id.Parse(cell("task_id"))
		if err != nil {
			return nil, fmt.Errorf("task_id baris %d tidak valid", rowNumber)
		}
		if snapshot == id.Nil || rid == id.Nil || tid == id.Nil {
			return nil, fmt.Errorf("identitas kosong pada baris %d", rowNumber)
		}
		if at == "" || by == "" {
			return nil, fmt.Errorf("isi reported_at dan reported_by pada baris perubahan %d", rowNumber)
		}
		if len(progress) > 64 || len(at) > 100 || len(by) > 200 || len(note) > 4000 {
			return nil, fmt.Errorf("isi perubahan baris %d terlalu panjang", rowNumber)
		}
		changes = append(changes, api.EmergencyChange{SnapshotID: snapshot, RowID: rid, TaskID: tid, Progress: progress, ReportedAt: at, ReportedBy: by, Note: note})
		if len(changes) > 100 {
			return nil, errors.New("maksimal 100 baris perubahan per impor; pisahkan perubahan menjadi beberapa berkas tanpa mengubah ID aslinya")
		}
	}
	if len(changes) == 0 {
		return nil, errors.New("CSV tidak memiliki catatan perubahan darurat")
	}
	return changes, nil
}

// SnapshotRow is a row of a server snapshot's CSV: the IDs a change must
// name, and the task's version and progress at the snapshot.
type SnapshotRow struct {
	SnapshotID, RowID, TaskID id.ID
	Version                   int64
	Progress                  string
}

// ParseSnapshotRows reads the rows of a snapshot CSV as the server writes
// it, to pair them with a sheet's rows after recovery.
func ParseSnapshotRows(data []byte) ([]SnapshotRow, error) {
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})))
	header, err := reader.Read()
	if err != nil {
		return nil, errors.New("header snapshot tidak dapat dibaca")
	}
	indexes := map[string]int{}
	for i, key := range header {
		indexes[strings.TrimSpace(key)] = i
	}
	for _, key := range []string{"snapshot_id", "row_id", "task_id", "task_version", "base_progress"} {
		if _, ok := indexes[key]; !ok {
			return nil, fmt.Errorf("kolom snapshot tidak ditemukan: %s", key)
		}
	}
	var out []SnapshotRow
	for {
		row, err := reader.Read()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, errors.New("CSV snapshot tidak valid")
		}
		var r SnapshotRow
		var e1, e2, e3, e4 error
		r.SnapshotID, e1 = id.Parse(row[indexes["snapshot_id"]])
		r.RowID, e2 = id.Parse(row[indexes["row_id"]])
		r.TaskID, e3 = id.Parse(row[indexes["task_id"]])
		r.Version, e4 = strconv.ParseInt(row[indexes["task_version"]], 10, 64)
		if err := errors.Join(e1, e2, e3, e4); err != nil {
			return nil, errors.New("identitas snapshot tidak valid")
		}
		r.Progress = row[indexes["base_progress"]]
		out = append(out, r)
	}
}
