package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/himasifunej/himasif-space/internal/id"
)

// Limits of a note document (see the Notes API contract §2).
const (
	NoteMaxBlocks       = 500
	NoteMaxBlockText    = 4000
	NoteMaxTableCols    = 50
	NoteMaxTableRows    = 500
	NoteMaxHeadingText  = 500
	NoteMaxLinkLen      = 2000
	NoteMaxContentBytes = 200 * 1024
	// NoteMaxAgendaMinutes caps the planned minutes of one agenda item.
	NoteMaxAgendaMinutes = 480
)

// ValidateNoteContent checks a note document's shape and size, rejecting
// unknown block types and oversized content. It returns a human-readable
// error (in Indonesian) suitable for a 400 response. The server uses it;
// clients use ValidateNoteContentPreserving.
func ValidateNoteContent(blocks []NoteBlock) error {
	return validateNoteContent(blocks, false)
}

// ValidateNoteContentPreserving is ValidateNoteContent for a client: it
// also accepts blocks of a type this build does not know that came from the
// server (NoteBlock.Raw), which the client writes back unchanged and the
// server checks. They still count towards the block and size limits.
func ValidateNoteContentPreserving(blocks []NoteBlock) error {
	return validateNoteContent(blocks, true)
}

func validateNoteContent(blocks []NoteBlock, keep bool) error {
	count := len(blocks)
	for _, b := range blocks {
		if b.Type == BlockToggle {
			count += len(b.Children)
		}
	}
	if count > NoteMaxBlocks {
		return fmt.Errorf("note terlalu panjang: maksimal %d blok", NoteMaxBlocks)
	}
	for i, b := range blocks {
		if err := validateBlock(b, keep, true); err != nil {
			return fmt.Errorf("blok %d: %w", i+1, err)
		}
	}
	raw, err := json.Marshal(blocks)
	if err != nil {
		return errors.New("isi note tidak dapat diserialisasi")
	}
	if len(raw) > NoteMaxContentBytes {
		return fmt.Errorf("isi note terlalu besar: maksimal %d KB", NoteMaxContentBytes/1024)
	}
	return nil
}

// validateBlock checks one block. keep accepts kept unknown blocks (see
// ValidateNoteContentPreserving); top is false for a toggle's children.
func validateBlock(b NoteBlock, keep, top bool) error {
	if keep && b.Unsupported() && len(b.Raw) > 0 {
		return nil
	}
	switch b.Type {
	case BlockHeading:
		if b.Level < 1 || b.Level > 3 {
			return errors.New("level heading harus 1 sampai 3")
		}
		if n := utf8.RuneCountInString(b.Text); n < 1 || n > NoteMaxHeadingText {
			return errors.New("teks heading kosong atau terlalu panjang")
		}
		if err := validateMinutes(b.Minutes); err != nil {
			return err
		}
		return mustEmpty(b, "heading", "spans", "items", "columns", "rows", "variant", "children")
	case BlockParagraph:
		if err := validateSpans(b.Spans); err != nil {
			return err
		}
		return mustEmpty(b, "paragraph", "items", "columns", "rows", "variant", "minutes", "children")
	case BlockCallout:
		if b.Variant != CalloutInfo && b.Variant != CalloutDecision {
			return errors.New("callout harus info atau decision")
		}
		if err := validateSpans(b.Spans); err != nil {
			return err
		}
		return mustEmpty(b, "callout", "text", "items", "columns", "rows", "minutes", "children")
	case BlockToggle:
		if !top {
			return errors.New("toggle tidak boleh berada di dalam toggle")
		}
		if utf8.RuneCountInString(b.Text) > NoteMaxHeadingText || strings.ContainsRune(b.Text, 0) {
			return errors.New("judul toggle terlalu panjang atau tidak valid")
		}
		for i, child := range b.Children {
			if err := validateBlock(child, keep, false); err != nil {
				return fmt.Errorf("isi toggle %d: %w", i+1, err)
			}
		}
		return mustEmpty(b, "toggle", "spans", "items", "columns", "rows", "variant", "minutes")
	case BlockBulletedList, BlockNumberedList, BlockChecklist:
		if len(b.Items) == 0 {
			return errors.New("daftar tidak boleh kosong")
		}
		for _, it := range b.Items {
			if err := validateSpans(it.Spans); err != nil {
				return err
			}
			if err := validateItem(b.Type, it); err != nil {
				return err
			}
		}
		return mustEmpty(b, "list", "spans", "columns", "rows", "variant", "minutes", "children")
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
		return mustEmpty(b, "table", "spans", "items", "variant", "minutes", "children")
	case BlockDivider:
		return mustEmpty(b, "divider", "spans", "items", "columns", "rows", "variant", "minutes", "children")
	default:
		return fmt.Errorf("jenis blok tidak dikenal: %q", b.Type)
	}
}

// validateItem checks the fields of a list item that belong to one kind of
// list: action item fields on checklists, agenda minutes on numbered lists.
func validateItem(kind string, it NoteListItem) error {
	if kind != BlockChecklist && (it.AssigneeID != nil || it.DueOn != nil || it.TaskID != nil) {
		return errors.New("PJ, tenggat dan tugas hanya untuk butir checklist")
	}
	if it.AssigneeID != nil && *it.AssigneeID == id.Nil {
		return errors.New("PJ butir checklist kosong")
	}
	if it.TaskID != nil && *it.TaskID == id.Nil {
		return errors.New("tugas butir checklist kosong")
	}
	if it.DueOn != nil && it.DueOn.IsZero() {
		return errors.New("tenggat butir checklist tidak valid")
	}
	if kind != BlockNumberedList && it.Minutes != 0 {
		return errors.New("durasi agenda hanya untuk daftar bernomor atau judul bagian")
	}
	return validateMinutes(it.Minutes)
}

// validateMinutes checks the planned minutes of an agenda item; 0 is none.
func validateMinutes(m int) error {
	if m < 0 || m > NoteMaxAgendaMinutes {
		return fmt.Errorf("durasi agenda harus 0 sampai %d menit", NoteMaxAgendaMinutes)
	}
	return nil
}

// validateSpans checks a run of spans: total text length, link form and
// mentions.
func validateSpans(spans []Span) error {
	total := 0
	for _, s := range spans {
		if strings.ContainsRune(s.Text, 0) {
			return errors.New("teks tidak valid")
		}
		if m := s.Mention; m != nil {
			if m.Kind != MentionUser && m.Kind != MentionTask {
				return errors.New("mention harus ke anggota atau tugas")
			}
			if m.ID == id.Nil {
				return errors.New("mention tanpa tujuan")
			}
			if strings.TrimSpace(s.Text) == "" {
				return errors.New("mention harus memiliki teks nama")
			}
			if s.Link != "" {
				return errors.New("mention tidak boleh sekaligus tautan")
			}
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
		case "variant":
			set = b.Variant != ""
		case "minutes":
			set = b.Minutes != 0
		case "children":
			set = len(b.Children) > 0
		}
		if set {
			return fmt.Errorf("blok %s tidak memakai field %q", kind, f)
		}
	}
	return nil
}
