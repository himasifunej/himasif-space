// Package emergency reads the editable change columns of Space's emergency
// spreadsheet. Canonical baseline data always comes from the server snapshot.
package emergency

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
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
