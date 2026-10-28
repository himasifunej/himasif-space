package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Keep the recovery payload bounded independently of the storage backend. The
// shared validator handles document shape; the local bound also applies when
// reading a damaged file or an older client's localStorage value.
const maxLocalNoteDraftBytes = 256 * 1024

type savedNoteDraft struct {
	UserID id.ID                `json:"userId"`
	Draft  appui.LocalNoteDraft `json:"draft"`
}

func validateNoteDraftIdentity(userID, noteID id.ID) error {
	if userID == id.Nil || noteID == id.Nil {
		return errors.New("akun dan identitas note wajib tersedia untuk draf perangkat")
	}
	return nil
}

func validateLocalNoteDraft(userID, noteID id.ID, draft appui.LocalNoteDraft) error {
	if err := validateNoteDraftIdentity(userID, noteID); err != nil {
		return err
	}
	if draft.NoteID != noteID || draft.SpaceID == id.Nil {
		return errors.New("identitas note atau Space pada draf perangkat tidak sesuai")
	}
	if draft.BaseVersion < 1 || draft.SavedAt.IsZero() || draft.SavedAt.Year() < 1 || draft.SavedAt.Year() > 9999 {
		return errors.New("versi atau waktu penyimpanan draf perangkat tidak valid")
	}
	if !utf8.ValidString(draft.Title) || utf8.RuneCountInString(draft.Title) > 200 || strings.ContainsRune(draft.Title, 0) {
		return errors.New("judul draf perangkat tidak valid atau melebihi 200 karakter")
	}
	if err := api.ValidateNoteContent(draft.Content); err != nil {
		return fmt.Errorf("isi draf perangkat: %w", err)
	}
	content, err := json.Marshal(draft.Content)
	if err != nil {
		return fmt.Errorf("encode isi draf perangkat: %w", err)
	}
	if len(content) > 200*1024 {
		return errors.New("isi draf perangkat melebihi 200 KB")
	}
	for _, link := range draft.PendingLinks {
		if link.TargetID == id.Nil || (link.TargetType != api.NoteLinkTask && link.TargetType != api.NoteLinkReminder) {
			return errors.New("rujukan tugas atau reminder tertunda pada draf perangkat tidak valid")
		}
	}
	return nil
}

func encodeLocalNoteDraft(userID, noteID id.ID, draft appui.LocalNoteDraft) ([]byte, error) {
	if err := validateLocalNoteDraft(userID, noteID, draft); err != nil {
		return nil, err
	}
	data, err := json.Marshal(savedNoteDraft{UserID: userID, Draft: draft})
	if err != nil {
		return nil, fmt.Errorf("encode draf perangkat: %w", err)
	}
	if len(data) > maxLocalNoteDraftBytes {
		return nil, errors.New("draf perangkat terlalu besar")
	}
	return data, nil
}

func decodeLocalNoteDraft(userID, noteID id.ID, data []byte) (*appui.LocalNoteDraft, error) {
	if err := validateNoteDraftIdentity(userID, noteID); err != nil {
		return nil, err
	}
	if len(data) > maxLocalNoteDraftBytes {
		return nil, errors.New("draf perangkat terlalu besar")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var saved *savedNoteDraft
	if err := decoder.Decode(&saved); err != nil {
		return nil, fmt.Errorf("decode draf perangkat: %w", err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, errors.New("draf perangkat memiliki data tambahan yang tidak valid")
	}
	if saved == nil || saved.UserID != userID {
		return nil, errors.New("draf perangkat bukan milik akun ini atau formatnya tidak valid")
	}
	if err := validateLocalNoteDraft(userID, noteID, saved.Draft); err != nil {
		return nil, err
	}
	return &saved.Draft, nil
}
