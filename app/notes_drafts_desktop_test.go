//go:build !js

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/id"
)

func draftFixture(noteID id.ID, title string) appui.LocalNoteDraft {
	return appui.LocalNoteDraft{SpaceID: id.New(), NoteID: noteID, Title: title,
		Content:     []api.NoteBlock{{Type: api.BlockParagraph, Spans: []api.Span{{Text: "Belum disimpan ke cloud", Bold: true}}}},
		BaseVersion: 3, SavedAt: time.Date(2026, 10, 28, 11, 0, 0, 123000000, time.UTC)}
}

func draftFile(root string, userID, noteID id.ID) string {
	return filepath.Join(root, userID.String(), noteID.String()+".json")
}

func TestNoteDraftMissingDoesNotCreateStorage(t *testing.T) {
	root := filepath.Join(t.TempDir(), "note-drafts")
	store := &desktopNoteDrafts{root: root}
	user, note := id.New(), id.New()
	if got, err := store.Load(user, note); err != nil || got != nil {
		t.Fatalf("missing draft = %+v, %v", got, err)
	}
	if err := store.Delete(user, note); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reading/discarding a missing draft created storage: %v", err)
	}
}

func TestNoteDraftSurvivesRestartAndKeepsAccountAndNoteBoundaries(t *testing.T) {
	root := filepath.Join(t.TempDir(), "note-drafts")
	store := &desktopNoteDrafts{root: root}
	a, b, note, anotherNote := id.New(), id.New(), id.New(), id.New()
	want := draftFixture(note, "Perangkat A")
	want.PendingLinks = []api.NoteLinkRequest{{TargetType: api.NoteLinkTask, TargetID: id.New()}, {TargetType: api.NoteLinkReminder, TargetID: id.New()}}
	otherAccount := draftFixture(note, "Akun lain")
	otherNote := draftFixture(anotherNote, "Note lain")
	for _, saved := range []struct {
		user  id.ID
		note  id.ID
		draft appui.LocalNoteDraft
	}{{a, note, want}, {b, note, otherAccount}, {a, anotherNote, otherNote}} {
		if err := store.Save(saved.user, saved.note, saved.draft); err != nil {
			t.Fatal(err)
		}
	}
	// Replacing the same draft updates one durable copy, rather than leaving
	// temporary files or relying on the previous store instance's memory.
	want.Title = "Draf perangkat diperbarui"
	if err := store.Save(a, note, want); err != nil {
		t.Fatal(err)
	}
	restarted := &desktopNoteDrafts{root: root}
	if got, err := restarted.Load(a, note); err != nil || !reflect.DeepEqual(got, &want) {
		t.Fatalf("restart recovered %+v, %v; want %+v", got, err, want)
	}
	if err := restarted.Delete(a, note); err != nil {
		t.Fatal(err)
	}
	if got, err := restarted.Load(a, note); err != nil || got != nil {
		t.Fatalf("discarded draft = %+v %v", got, err)
	}
	for _, untouched := range []struct {
		user id.ID
		note id.ID
		want appui.LocalNoteDraft
	}{{b, note, otherAccount}, {a, anotherNote, otherNote}} {
		if got, err := restarted.Load(untouched.user, untouched.note); err != nil || !reflect.DeepEqual(got, &untouched.want) {
			t.Fatalf("discard touched another namespace: %+v %v", got, err)
		}
	}
	if runtime.GOOS != "windows" {
		for path, mode := range map[string]os.FileMode{root: 0o700, filepath.Join(root, a.String()): 0o700, draftFile(root, a, anotherNote): 0o600} {
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != mode {
				t.Fatalf("permissions for %s: info=%v err=%v want=%o", path, info, err, mode)
			}
		}
	}
	assertNoNoteDraftTemps(t, root)
}

func TestNoteDraftCorruptionOrMismatchedIdentityIsAnExplicitReadFailure(t *testing.T) {
	root := filepath.Join(t.TempDir(), "note-drafts")
	user, other, note := id.New(), id.New(), id.New()
	good := draftFixture(note, "Keep original")
	store := &desktopNoteDrafts{root: root}
	if err := store.Save(user, note, good); err != nil {
		t.Fatal(err)
	}
	marshal := func(owner id.ID, d appui.LocalNoteDraft) []byte {
		t.Helper()
		data, err := json.Marshal(savedNoteDraft{UserID: owner, Draft: d})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	valid := marshal(user, good)
	wrongNote := good
	wrongNote.NoteID = id.New()
	wrongVersion := good
	wrongVersion.BaseVersion = 0
	wrongTime := good
	wrongTime.SavedAt = time.Time{}
	wrongSpace := good
	wrongSpace.SpaceID = id.Nil
	unknownBlock := good
	unknownBlock.Content = []api.NoteBlock{{Type: "unknown"}}
	unknownLink := good
	unknownLink.PendingLinks = []api.NoteLinkRequest{{TargetType: "unknown", TargetID: id.New()}}
	missingTarget := good
	missingTarget.PendingLinks = []api.NoteLinkRequest{{TargetType: api.NoteLinkTask}}
	for _, test := range []struct {
		name string
		data []byte
	}{
		{"null", []byte("null")}, {"array", []byte("[]")}, {"truncated", []byte("{")},
		{"trailing", append(bytes.Clone(valid), []byte("{}")...)},
		{"unknown field", bytes.Replace(valid, []byte(`"draft":`), []byte(`"password":"not-permitted","draft":`), 1)},
		{"wrong account", marshal(other, good)}, {"wrong note", marshal(user, wrongNote)},
		{"invalid version", marshal(user, wrongVersion)}, {"missing saved time", marshal(user, wrongTime)},
		{"missing Space", marshal(user, wrongSpace)}, {"invalid content", marshal(user, unknownBlock)},
		{"unknown pending link", marshal(user, unknownLink)}, {"missing pending target", marshal(user, missingTarget)},
		{"oversized file", []byte(strings.Repeat("x", maxLocalNoteDraftBytes+1))},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := draftFile(root, user, note)
			if err := os.WriteFile(path, test.data, 0o600); err != nil {
				t.Fatal(err)
			}
			if got, err := store.Load(user, note); err == nil || got != nil {
				t.Fatalf("damaged/private draft became a successful empty read: %+v %v", got, err)
			}
			data, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(data, test.data) {
				t.Fatal("reading damaged data changed it")
			}
		})
	}
}

func TestRejectedNoteDraftSavePreservesTheLastGoodCopy(t *testing.T) {
	root := filepath.Join(t.TempDir(), "note-drafts")
	user, note := id.New(), id.New()
	good := draftFixture(note, "Last good copy")
	store := &desktopNoteDrafts{root: root}
	if err := store.Save(user, note, good); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(draftFile(root, user, note))
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*appui.LocalNoteDraft){
		func(d *appui.LocalNoteDraft) { d.NoteID = id.New() },
		func(d *appui.LocalNoteDraft) { d.SpaceID = id.Nil },
		func(d *appui.LocalNoteDraft) { d.BaseVersion = -1 },
		func(d *appui.LocalNoteDraft) { d.SavedAt = time.Time{} },
		func(d *appui.LocalNoteDraft) { d.Title = strings.Repeat("a", 201) },
		func(d *appui.LocalNoteDraft) { d.Title = string([]byte{0xff}) },
		func(d *appui.LocalNoteDraft) {
			d.PendingLinks = []api.NoteLinkRequest{{TargetType: "unknown", TargetID: id.New()}}
		},
		func(d *appui.LocalNoteDraft) {
			d.PendingLinks = []api.NoteLinkRequest{{TargetType: api.NoteLinkReminder}}
		},
		func(d *appui.LocalNoteDraft) {
			d.Content = nil
			for i := 0; i < 70; i++ {
				d.Content = append(d.Content, api.NoteBlock{Type: api.BlockParagraph, Spans: []api.Span{{Text: strings.Repeat("x", 3900)}}})
			}
		},
	} {
		bad := good
		mutate(&bad)
		if err := store.Save(user, note, bad); err == nil {
			t.Fatal("invalid recovery copy was saved")
		}
		after, err := os.ReadFile(draftFile(root, user, note))
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("rejected save replaced the previous recovery data")
		}
	}
	assertNoNoteDraftTemps(t, root)
}

func TestNoteDraftInvalidNamespacesAndUnavailablePathsReturnErrors(t *testing.T) {
	user, note := id.New(), id.New()
	draft := draftFixture(note, "Keep")
	for _, pair := range [][2]id.ID{{id.Nil, note}, {user, id.Nil}} {
		store := &desktopNoteDrafts{root: filepath.Join(t.TempDir(), "drafts")}
		if _, err := store.Load(pair[0], pair[1]); err == nil {
			t.Fatal("Load accepted a missing namespace identity")
		}
		if err := store.Save(pair[0], pair[1], draft); err == nil {
			t.Fatal("Save accepted a missing namespace identity")
		}
		if err := store.Delete(pair[0], pair[1]); err == nil {
			t.Fatal("Delete accepted a missing namespace identity")
		}
	}
	file := filepath.Join(t.TempDir(), "barrier")
	if err := os.WriteFile(file, []byte("untouched"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, store := range []*desktopNoteDrafts{
		{root: file}, {root: filepath.Join(file, "drafts")}, {root: "relative-drafts"},
		{root: filepath.Join(t.TempDir(), "drafts"), initErr: errors.New("config unavailable")},
	} {
		if _, err := store.Load(user, note); err == nil {
			t.Fatal("Load hid storage unavailability")
		}
		if err := store.Save(user, note, draft); err == nil {
			t.Fatal("Save hid storage unavailability")
		}
		if err := store.Delete(user, note); err == nil {
			t.Fatal("Delete hid storage unavailability")
		}
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != "untouched" {
		t.Fatal("failed storage access changed the barrier file")
	}
}

func TestNoteDraftRejectsSymlinksOutsideAccountNamespace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks on Windows needs optional host privileges")
	}
	root := filepath.Join(t.TempDir(), "drafts")
	user, note := id.New(), id.New()
	store := &desktopNoteDrafts{root: root}
	draft := draftFixture(note, "Keep private")
	if err := store.Save(user, note, draft); err != nil {
		t.Fatal(err)
	}
	path := draftFile(root, user, note)
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte("outside must be untouched"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(user, note); err == nil {
		t.Fatal("Load followed a recovery-file symlink")
	}
	if err := store.Save(user, note, draft); err == nil {
		t.Fatal("Save followed a recovery-file symlink")
	}
	if err := store.Delete(user, note); err == nil {
		t.Fatal("Delete accepted a recovery-file symlink")
	}
	if data, err := os.ReadFile(outside); err != nil || string(data) != "outside must be untouched" {
		t.Fatal("draft operation changed an external file")
	}
	accountAlias := id.New()
	if err := os.Symlink(filepath.Join(root, user.String()), filepath.Join(root, accountAlias.String())); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(accountAlias, note); err == nil {
		t.Fatal("Load accepted a different account's directory alias")
	}
}

func assertNoNoteDraftTemps(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(entry.Name(), ".draft-") {
			t.Errorf("temporary draft remains after save: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
