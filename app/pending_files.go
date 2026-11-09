package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/id"
)

const pendingFileMaxBytes = 100 << 20
const pendingAccountMaxBytes = 256 << 20
const pendingAccountMaxCount = 32

type savedPendingFile struct {
	Server string                `json:"server"`
	UserID id.ID                 `json:"userId"`
	File   appui.PendingNoteFile `json:"file"`
}

func pendingServerKey(server string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(server, "/"))
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("alamat server untuk berkas pending tidak valid")
	}
	sum := sha256.Sum256([]byte(parsed.String()))
	return hex.EncodeToString(sum[:]), nil
}
func encodePendingFile(server string, user id.ID, file appui.PendingNoteFile) ([]byte, error) {
	key, err := pendingServerKey(server)
	if err != nil {
		return nil, err
	}
	if user == id.Nil || file.NoteID == id.Nil || file.SpaceID == id.Nil || file.UploadID == id.Nil {
		return nil, errors.New("identitas berkas pending tidak lengkap")
	}
	if file.Name == "" || !utf8.ValidString(file.Name) || utf8.RuneCountInString(file.Name) > 300 || strings.ContainsAny(file.Name, "/\\\r\n\x00") || file.Name == "." || file.Name == ".." {
		return nil, errors.New("nama berkas pending tidak valid")
	}
	if file.SizeBytes < 0 || file.SizeBytes > pendingFileMaxBytes || file.CreatedAt.IsZero() {
		return nil, errors.New("ukuran/waktu berkas pending tidak valid")
	}
	if len(file.SHA256) != 64 {
		return nil, errors.New("checksum berkas pending tidak valid")
	}
	return json.Marshal(savedPendingFile{Server: key, UserID: user, File: file})
}
func decodePendingFile(server string, user, note id.ID, metadata []byte) (*appui.PendingNoteFile, error) {
	if len(metadata) > 8192 {
		return nil, errors.New("metadata berkas pending terlalu besar")
	}
	var saved savedPendingFile
	if err := json.Unmarshal(metadata, &saved); err != nil {
		return nil, err
	}
	key, err := pendingServerKey(server)
	if err != nil {
		return nil, err
	}
	if saved.Server != key || saved.UserID != user || saved.File.NoteID != note {
		return nil, errors.New("berkas pending bukan milik server/akun/catatan ini")
	}
	if _, err = encodePendingFile(server, user, saved.File); err != nil {
		return nil, err
	}
	return &saved.File, nil
}
func verifyPendingFile(file *appui.PendingNoteFile) error {
	if int64(len(file.Data)) != file.SizeBytes {
		return errors.New("ukuran berkas pending tidak cocok")
	}
	sum := sha256.Sum256(file.Data)
	if hex.EncodeToString(sum[:]) != file.SHA256 {
		return fmt.Errorf("checksum berkas pending tidak cocok; salinan perangkat tidak utuh")
	}
	return nil
}
