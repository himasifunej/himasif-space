package emergency

import (
	"bytes"
	"encoding/csv"
	"github.com/himasifunej/himasif-space/internal/id"
	"testing"
)

func fixtureCSV(t *testing.T, duplicate bool) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	writer.Write(columns)
	row := make([]string, len(columns))
	for i, key := range columns {
		switch key {
		case "snapshot_id", "row_id", "task_id":
			row[i] = id.New().String()
		case "new_progress":
			row[i] = "doing"
		case "reported_at":
			row[i] = "2026-10-29 12:00 WIB"
		case "reported_by":
			row[i] = "operator statement"
		case "title":
			row[i] = "edited baseline must not authorize"
		}
	}
	writer.Write(row)
	if duplicate {
		writer.Write(row)
	}
	writer.Flush()
	return buffer.Bytes()
}
func TestChangesOnlyIdentityAndIntentAreImported(t *testing.T) {
	changes, err := ParseChanges(append([]byte{0xef, 0xbb, 0xbf}, fixtureCSV(t, false)...))
	if err != nil || len(changes) != 1 || changes[0].Progress != "doing" || changes[0].ReportedBy != "operator statement" {
		t.Fatalf("changes %+v %v", changes, err)
	}
	if _, err = ParseChanges(fixtureCSV(t, true)); err == nil {
		t.Fatal("duplicate row intent accepted")
	}
	if _, err = ParseChanges([]byte("wrong,headers\n1,2\n")); err == nil {
		t.Fatal("wrong export accepted")
	}
}

func TestParseSnapshotRowsReadsTheServersCSV(t *testing.T) {
	sid, rid, tid := id.New(), id.New(), id.New()
	data := "\xef\xbb\xbfsnapshot_id,row_id,task_id,captured_at_utc,task_version,title,expected_output,assignee,deadline_kind,deadline_value,time_zone,base_progress,base_match_status,new_progress,reported_at,reported_by,change_note\n" +
		sid.String() + "," + rid.String() + "," + tid.String() + ",2026-11-12T06:00:00Z,7,\"Poster, A3\",,Raka,date,2026-11-20,Asia/Jakarta,doing,none,,,,\n"
	rows, err := ParseSnapshotRows([]byte(data))
	if err != nil || len(rows) != 1 || rows[0] != (SnapshotRow{SnapshotID: sid, RowID: rid, TaskID: tid, Version: 7, Progress: "doing"}) {
		t.Fatalf("%v %+v", err, rows)
	}
	if _, err := ParseSnapshotRows([]byte("title\nx\n")); err == nil {
		t.Fatal("a CSV without IDs was read")
	}
}
