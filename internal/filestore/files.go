// Package filestore keeps immutable private blobs outside the web document
// root. Database metadata, never a public filesystem URL, grants access.
package filestore

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/himasifunej/himasif-space/internal/id"
)

type Blob struct {
	Key                 id.ID
	Size                int64
	SHA256, ContentType string
}

func Put(root string, source io.Reader, limit int64) (blob Blob, err error) {
	if root == "" {
		return blob, fmt.Errorf("private attachment storage is not configured")
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return blob, err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return blob, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return blob, fmt.Errorf("attachment storage must be a private directory (0700)")
	}
	f, err := os.CreateTemp(root, ".upload-*")
	if err != nil {
		return blob, err
	}
	temp := f.Name()
	defer os.Remove(temp)
	defer func() {
		if err != nil {
			_ = f.Close()
		}
	}()
	if err = f.Chmod(0600); err != nil {
		return blob, err
	}
	h := sha256.New()
	prefix := &firstBytes{limit: 512}
	blob.Size, err = io.Copy(io.MultiWriter(f, h, prefix), io.LimitReader(source, limit+1))
	if err != nil {
		return blob, err
	}
	if blob.Size > limit {
		return blob, fmt.Errorf("attachment exceeds %d bytes", limit)
	}
	if err = f.Sync(); err != nil {
		return blob, err
	}
	if err = f.Close(); err != nil {
		return blob, err
	}
	blob.Key = id.New()
	blob.SHA256 = hex.EncodeToString(h.Sum(nil))
	blob.ContentType = http.DetectContentType(prefix.data)
	if err = os.Rename(temp, path(root, blob.Key)); err != nil {
		return blob, err
	}
	// Persist the rename before a database transaction is allowed to reference
	// this key. A crash can leave an unreferenced blob, never a partial live one.
	dir, err := os.Open(root)
	if err != nil {
		_ = Remove(root, blob.Key)
		return blob, err
	}
	err = dir.Sync()
	_ = dir.Close()
	if err != nil {
		_ = Remove(root, blob.Key)
		return blob, err
	}
	return blob, nil
}
func path(root string, key id.ID) string { return filepath.Join(root, key.String()+".blob") }
func Open(root string, key id.ID) (*os.File, error) {
	if root == "" {
		return nil, fmt.Errorf("private attachment storage is not configured")
	}
	directory, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	return directory.Open(key.String() + ".blob")
}
func Remove(root string, key id.ID) error { return os.Remove(path(root, key)) }

type firstBytes struct {
	data  []byte
	limit int
}

func (w *firstBytes) Write(p []byte) (int, error) {
	remaining := w.limit - len(w.data)
	if remaining > 0 {
		w.data = append(w.data, p[:min(remaining, len(p))]...)
	}
	return len(p), nil
}
