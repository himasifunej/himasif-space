package appui

import (
	"testing"
	"unicode/utf8"
)

// Every version's notes fit a signed manifest (update.MaxNotes, 600), one
// bullet a line, and read back as the same lines.
func TestChangelogFitsTheManifest(t *testing.T) {
	if len(Changelog) == 0 || Changelog[0].Version != "0.4.5" {
		t.Fatalf("newest first, 0.4.5 on top: %+v", Changelog)
	}
	for _, r := range Changelog {
		notes := FormatNotes(r.Notes)
		if n := utf8.RuneCountInString(notes); n > 600 || len(r.Notes) == 0 {
			t.Errorf("%s: %d characters, %d lines", r.Version, n, len(r.Notes))
		}
		back := NoteLines(notes)
		if len(back) != len(r.Notes) {
			t.Fatalf("%s: %q", r.Version, back)
		}
		for i := range back {
			if back[i] != r.Notes[i] {
				t.Errorf("%s line %d: %q, want %q", r.Version, i, back[i], r.Notes[i])
			}
		}
	}
	if r, ok := ChangelogFor("0.4.5-rc2"); !ok || r.Version != "0.4.5" {
		t.Fatal("a release candidate takes its release's notes")
	}
	if _, ok := ChangelogFor("9.9.9"); ok {
		t.Fatal("an unknown version has notes")
	}
	// An older manifest's one sentence is one line.
	if got := NoteLines("Perbaikan kecil."); len(got) != 1 || got[0] != "Perbaikan kecil." {
		t.Fatalf("%q", got)
	}
}

// A notice's date from an older server reads as the app writes dates.
func TestNoticeDateInIndonesian(t *testing.T) {
	if got := NoticeDate("05 Dec 2026 08:00 WIB"); got != "5 Des 2026 · 08.00 WIB" {
		t.Fatalf("%q", got)
	}
	if got := NoticeDate("Cek grup divisi ya."); got != "Cek grup divisi ya." {
		t.Fatalf("%q", got)
	}
}
