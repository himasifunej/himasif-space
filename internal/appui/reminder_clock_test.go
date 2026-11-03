package appui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestReminderClockSelectionAndClear(t *testing.T) {
	a := &App{}
	value := ""
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { a.reminderClockField(c, &value) })
	}, 400, 600)
	if err := tt.Click("Pilih jam reminder"); err != nil {
		t.Fatal(err)
	}
	if dir := os.Getenv("SPACE_UI_CAPTURE_DIR"); dir != "" {
		f, err := os.Create(filepath.Join(dir, "reminder-clock.png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	face, ok := tt.Find("Jam analog reminder")
	if !ok {
		t.Fatal("clock face missing")
	}
	tt.ClickAt(face.X+face.W/2, face.Y+20)
	if value != "00:00" {
		t.Fatalf("12 AM selected %q", value)
	}
	if err := tt.Click("Ubah AM atau PM"); err != nil {
		t.Fatal(err)
	}
	if value != "12:00" {
		t.Fatalf("PM selected %q", value)
	}
	if err := tt.Click("Pilih menit pada jam analog"); err != nil {
		t.Fatal(err)
	}
	tt.ClickAt(face.X+face.W-20, face.Y+face.H/2)
	if value != "12:15" {
		t.Fatalf("15 minutes selected %q", value)
	}
	tt.Press(face.X+face.W-20, face.Y+face.H/2)
	tt.Move(face.X+face.W/2, face.Y+face.H-20)
	tt.Release(face.X+face.W/2, face.Y+face.H-20)
	if value != "12:30" {
		t.Fatalf("dragging to 30 minutes selected %q", value)
	}
	if err := tt.Click("Tanpa jam"); err != nil {
		t.Fatal(err)
	}
	if value != "" || a.reminders.clockOpen {
		t.Fatalf("date-only selection failed: %q", value)
	}
}
