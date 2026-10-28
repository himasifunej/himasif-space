package api

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Limits of a note document (see the Notes API contract §2).
const (
	NoteMaxBlocks      = 500
	NoteMaxBlockText   = 4000
	NoteMaxTableCols   = 50
	NoteMaxTableRows   = 500
	NoteMaxHeadingText = 500
	NoteMaxLinkLen     = 2000
)

// ValidateNoteContent checks a note document's shape and size, rejecting
// unknown block types and oversized content. It returns a human-readable
// error (in Indonesian) suitable for a 400 response. The rules are shared
// so a client can check the same document before sending it.
func ValidateNoteContent(blocks []NoteBlock) error {
	if len(blocks) > NoteMaxBlocks {
		return fmt.Errorf("note terlalu panjang: maksimal %d blok", NoteMaxBlocks)
	}
	for i, b := range blocks {
		if err := validateBlock(b); err != nil {
			return fmt.Errorf("blok %d: %w", i+1, err)
		}
	}
	return nil
}

func validateBlock(b NoteBlock) error {
	switch b.Type {
	case BlockHeading:
		if b.Level < 1 || b.Level > 3 {
			return errors.New("level heading harus 1 sampai 3")
		}
		if n := utf8.RuneCountInString(b.Text); n < 1 || n > NoteMaxHeadingText {
			return errors.New("teks heading kosong atau terlalu panjang")
		}
		return mustEmpty(b, "heading", "spans", "items", "columns", "rows")
	case BlockParagraph:
		if err := validateSpans(b.Spans); err != nil {
			return err
		}
		return mustEmpty(b, "paragraph", "items", "columns", "rows")
	case BlockBulletedList, BlockNumberedList, BlockChecklist:
		if len(b.Items) == 0 {
			return errors.New("daftar tidak boleh kosong")
		}
		for _, it := range b.Items {
			if err := validateSpans(it.Spans); err != nil {
				return err
			}
		}
		return mustEmpty(b, "list", "spans", "columns", "rows")
	case BlockTable:
		if len(b.Columns) == 0 || len(b.Columns) > NoteMaxTableCols {
			return fmt.Errorf("tabel harus memiliki 1 sampai %d kolom", NoteMaxTableCols)
		}
		if len(b.Rows) > NoteMaxTableRows {
			return fmt.Errorf("tabel maksimal %d baris", NoteMaxTableRows)
		}
		for _, row := range b.Rows {
			if len(row) != len(b.Columns) {
				return errors.New("jumlah sel tiap baris harus sama dengan jumlah kolom")
			}
			for _, cell := range row {
				if err := validateSpans(cell); err != nil {
					return err
				}
			}
		}
		return mustEmpty(b, "table", "spans", "items")
	case BlockDivider:
		return mustEmpty(b, "divider", "spans", "items", "columns", "rows")
	default:
		return fmt.Errorf("jenis blok tidak dikenal: %q", b.Type)
	}
}

// validateSpans checks a run of spans: total text length and link form.
func validateSpans(spans []Span) error {
	total := 0
	for _, s := range spans {
		if strings.ContainsRune(s.Text, 0) {
			return errors.New("teks tidak valid")
		}
		total += utf8.RuneCountInString(s.Text)
		if s.Link != "" {
			if len(s.Link) > NoteMaxLinkLen {
				return errors.New("tautan terlalu panjang")
			}
			if !strings.HasPrefix(s.Link, "http://") && !strings.HasPrefix(s.Link, "https://") {
				return errors.New("tautan harus diawali http:// atau https://")
			}
		}
	}
	if total > NoteMaxBlockText {
		return fmt.Errorf("teks blok terlalu panjang: maksimal %d karakter", NoteMaxBlockText)
	}
	return nil
}

// mustEmpty reports an error if a field that does not belong to a block
// type is set, so a client cannot smuggle unexpected data.
func mustEmpty(b NoteBlock, kind string, fields ...string) error {
	for _, f := range fields {
		set := false
		switch f {
		case "text":
			set = b.Text != ""
		case "spans":
			set = len(b.Spans) > 0
		case "items":
			set = len(b.Items) > 0
		case "columns":
			set = len(b.Columns) > 0
		case "rows":
			set = len(b.Rows) > 0
		}
		if set {
			return fmt.Errorf("blok %s tidak memakai field %q", kind, f)
		}
	}
	return nil
}
