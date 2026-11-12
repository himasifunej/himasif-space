package sheet

import (
	"context"
	"os"
	"testing"
	"time"
)

// SPACE_SHEET_E2E is a published link to read once, read-only: the
// owner's test sheet. Nothing is written to Google.
func TestEndToEndPublishedSheet(t *testing.T) {
	raw := os.Getenv("SPACE_SHEET_E2E")
	if raw == "" {
		t.Skip("SPACE_SHEET_E2E not set")
	}
	u, err := PublishURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("Asia/Jakarta")
	s, err := Fetch(context.Background(), nil, u, loc)
	if err != nil {
		t.Fatal(err)
	}
	var f Follow
	out := f.Offer(s)
	t.Logf("clock %s (%s), rows %d, no ID %d, issues %d, outcome %s, %s", s.Clock.Format(time.RFC3339), s.ClockText, len(s.Rows), len(s.NoID), len(s.Issues), out.Kind, Lag(s.Clock, time.Now()))
	for _, r := range s.Rows {
		t.Logf("row %d %s %q base=%s now=%s", r.Line, r.TaskID, r.Title, r.BaseProgress, r.Progress)
	}
	for _, i := range s.Issues {
		t.Logf("issue %+v", i)
	}
	if len(s.Rows) == 0 {
		t.Fatal("no rows read")
	}
}
