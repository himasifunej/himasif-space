package api

import (
	"strings"
	"testing"
)

func TestValidateNoteContent(t *testing.T) {
	ok := []NoteBlock{
		{Type: BlockHeading, Level: 2, Text: "Agenda"},
		{Type: BlockParagraph, Spans: []Span{{Text: "halo", Bold: true}, {Text: "tautan", Link: "https://himasif.id"}}},
		{Type: BlockBulletedList, Items: []NoteListItem{{Spans: []Span{{Text: "satu"}}}}},
		{Type: BlockChecklist, Items: []NoteListItem{{Spans: []Span{{Text: "cek"}}, Checked: true}}},
		{Type: BlockTable, Columns: []string{"A", "B"}, Rows: [][][]Span{{{{Text: "1"}}, {{Text: "2"}}}}},
		{Type: BlockDivider},
	}
	if err := ValidateNoteContent(ok); err != nil {
		t.Fatalf("valid document rejected: %v", err)
	}

	bad := []struct {
		name  string
		block NoteBlock
	}{
		{"unknown type", NoteBlock{Type: "marquee"}},
		{"heading level", NoteBlock{Type: BlockHeading, Level: 9, Text: "x"}},
		{"heading empty", NoteBlock{Type: BlockHeading, Level: 1, Text: ""}},
		{"heading with spans", NoteBlock{Type: BlockHeading, Level: 1, Text: "x", Spans: []Span{{Text: "y"}}}},
		{"empty list", NoteBlock{Type: BlockBulletedList, Items: nil}},
		{"table no columns", NoteBlock{Type: BlockTable, Columns: nil}},
		{"table ragged row", NoteBlock{Type: BlockTable, Columns: []string{"A", "B"}, Rows: [][][]Span{{{{Text: "1"}}}}}},
		{"bad link scheme", NoteBlock{Type: BlockParagraph, Spans: []Span{{Text: "x", Link: "ftp://x"}}}},
		{"paragraph with items", NoteBlock{Type: BlockParagraph, Items: []NoteListItem{{Spans: []Span{{Text: "x"}}}}}},
	}
	for _, c := range bad {
		if err := ValidateNoteContent([]NoteBlock{c.block}); err == nil {
			t.Errorf("%s: expected an error", c.name)
		}
	}

	// Too many blocks.
	many := make([]NoteBlock, NoteMaxBlocks+1)
	for i := range many {
		many[i] = NoteBlock{Type: BlockDivider}
	}
	if err := ValidateNoteContent(many); err == nil {
		t.Error("too many blocks accepted")
	}

	// Oversized block text.
	big := NoteBlock{Type: BlockParagraph, Spans: []Span{{Text: strings.Repeat("x", NoteMaxBlockText+1)}}}
	if err := ValidateNoteContent([]NoteBlock{big}); err == nil {
		t.Error("oversized block accepted")
	}
}

func TestValidateNoteContentSerializedSize(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		blocks     int
		valid      bool
	}{
		{"ordinary document", "x", 40, true},
		{"ASCII exceeds total cap", "x", 60, false},
		{"UTF8 bytes exceed character count", "界", 20, false},
		{"JSON escaping counts towards cap", "<", 10, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			blocks := make([]NoteBlock, tc.blocks)
			for i := range blocks {
				blocks[i] = NoteBlock{Type: BlockParagraph, Spans: []Span{{Text: strings.Repeat(tc.text, NoteMaxBlockText)}}}
			}
			err := ValidateNoteContent(blocks)
			if (err == nil) != tc.valid {
				t.Fatalf("serialized size accepted=%v, want%v, err=%v", err == nil, tc.valid, err)
			}
		})
	}
}
