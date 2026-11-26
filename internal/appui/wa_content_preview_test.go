package appui

import (
	"strings"
	"testing"

	"github.com/himasifunej/himasif-space/internal/broadcast"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// QA rc.1 finding 2: the preview shows everything the parser read.
func TestBroadcastPreviewShowsParsedFields(t *testing.T) {
	post := timeutil.Date{Year: 2026, Month: 12, Day: 10}
	dl := timeutil.Date{Year: 2026, Month: 12, Day: 8}
	head, lines := broadcastPreviewLines(broadcast.Item{Title: "Poster Dies Natalis", Humas: "Almas", HumasAlias: "Mas", Design: "Rafi", PostingDate: &post, DesignDeadline: &dl, Color: "dark blue", Urgent: true, Line: 4})
	all := head + "\n" + strings.Join(lines, "\n")
	for _, want := range []string{"DADAKAN", "Posting 10 Desember", "Deadline desain 8 Desember", "Almas (Mas)", "Rafi", "Warna: dark blue", "Baris 4"} {
		if !strings.Contains(all, want) {
			t.Errorf("preview misses %q:\n%s", want, all)
		}
	}
	// Without a year the day and month still show.
	_, lines = broadcastPreviewLines(broadcast.Item{Title: "Reels", Day: 3, Month: 1, DeadlineDay: 1, DeadlineMonth: 1})
	if !strings.Contains(lines[0], "Posting 3 Januari · Deadline desain 1 Januari") || !strings.Contains(lines[1], "PJ Humas: —") {
		t.Errorf("yearless preview = %q", lines)
	}
}
