package filestore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestImmutableBlobAndLimitCleanup(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private")
	content := []byte("private note attachment")
	a, err := Put(root, bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Put(root, bytes.NewReader(content), 100)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	if a.Key == b.Key || a.SHA256 != hex.EncodeToString(sum[:]) || a.Size != int64(len(content)) {
		t.Fatalf("bad blob identity/checksum: %+v %+v", a, b)
	}
	f, err := Open(root, a.Key)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := io.ReadAll(f)
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("content %q: %v", got, err)
	}
	info, _ := f.Stat()
	if info.Mode().Perm() != 0600 {
		t.Fatal("not private")
	}
	if _, err = Put(root, bytes.NewReader(content), 2); err == nil {
		t.Fatal("oversize accepted")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 2 {
		t.Fatalf("partial upload leaked: %d", len(entries))
	}
}
func TestRejectPublicOrSymlinkStorage(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "public")
	os.Mkdir(root, 0755)
	if _, err := Put(root, bytes.NewBufferString("x"), 10); err == nil {
		t.Fatal("public storage accepted")
	}
	link := filepath.Join(parent, "link")
	os.Symlink(root, link)
	if _, err := Put(link, bytes.NewBufferString("x"), 10); err == nil {
		t.Fatal("symlink accepted")
	}
}
