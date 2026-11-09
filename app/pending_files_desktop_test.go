//go:build !js

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/id"
)

func TestPendingFileSurvivesRestartAndScopesAccountAndServer(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	user, other, note := id.New(), id.New(), id.New()
	data := []byte("offline private file")
	sum := sha256.Sum256(data)
	file := appui.PendingNoteFile{UploadID: id.New(), NoteID: note, SpaceID: id.New(), Name: "fixture.txt", SHA256: hex.EncodeToString(sum[:]), SizeBytes: int64(len(data)), Data: data, CreatedAt: time.Now()}
	first := newPendingFileStore()
	if err := first.Save("https://space.test", user, file); err != nil {
		t.Fatal(err)
	}
	restarted := newPendingFileStore()
	got, err := restarted.Load("https://space.test", user, note)
	if err != nil || got == nil || got.UploadID != file.UploadID || string(got.Data) != string(data) {
		t.Fatalf("lost stable upload intent: %+v %v", got, err)
	}
	if got, err := restarted.Load("https://space.test", other, note); err != nil || got != nil {
		t.Fatalf("cross account %+v %v", got, err)
	}
	if got, err := restarted.Load("https://other.test", user, note); err != nil || got != nil {
		t.Fatalf("cross server %+v %v", got, err)
	}
	// A delayed cleanup of an earlier upload must preserve its replacement.
	newer := file
	newer.UploadID = id.New()
	if err := restarted.Save("https://space.test", user, newer); err == nil {
		t.Fatal("existing pending file was overwritten")
	}
	if err := first.Delete("https://space.test", user, note, file.UploadID); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Save("https://space.test", user, newer); err != nil {
		t.Fatal(err)
	}
	if err := first.Delete("https://space.test", user, note, file.UploadID); err == nil {
		t.Fatal("stale cleanup was not reported")
	}
	got, err = restarted.Load("https://space.test", user, note)
	if err != nil || got == nil || got.UploadID != newer.UploadID {
		t.Fatalf("old ack deleted new intent: %+v %v", got, err)
	}
	if err := restarted.Delete("https://space.test", user, note, newer.UploadID); err != nil {
		t.Fatal(err)
	}
	got, err = restarted.Load("https://space.test", user, note)
	if err != nil || got != nil {
		t.Fatalf("pending cleanup failed %+v %v", got, err)
	}
}
