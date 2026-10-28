package server

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
)

func TestNoteDeletionHidesEveryReadButPreservesAppendOnlyHistory(t *testing.T) {
	o := newOrg(t)
	token, _ := o.kadiv("mediatek", "delete-note@himasif.test")
	sid := o.spaceID(token, "division-mediatek")
	var note api.Note
	if code := o.call("POST", "/api/spaces/"+sid.String()+"/notes", token, api.NoteCreateRequest{Title: "Delete safely", Content: []api.NoteBlock{para("preserved in audit")}}, &note); code != http.StatusCreated {
		t.Fatalf("create = %d", code)
	}
	path := "/api/notes/" + note.ID.String()
	if code := o.call("DELETE", path, token, nil, nil); code != http.StatusNoContent {
		t.Fatalf("delete = %d; append-only history must not block deletion", code)
	}
	for _, suffix := range []string{"", "/history", "/draft", "/drafts", "/links"} {
		if code, reason := o.errCode("GET", path+suffix, token, nil); code != http.StatusNotFound || reason != api.CodeNotFound {
			t.Fatalf("deleted note %s = %d %s", suffix, code, reason)
		}
	}
	if code, _ := o.errCode("DELETE", path, token, nil); code != http.StatusNotFound {
		t.Fatalf("repeat delete = %d", code)
	}
	var listed api.List[api.Note]
	if code := o.call("GET", "/api/spaces/"+sid.String()+"/notes", token, nil, &listed); code != http.StatusOK || len(listed.Items) != 0 {
		t.Fatalf("deleted note remained listed: code=%d notes=%+v", code, listed.Items)
	}
	var tombstones, events int
	q := o.st.DB()
	if err := q.QueryRow(context.Background(), `select count(*) from notes where id=$1 and deleted_at is not null`, note.ID).Scan(&tombstones); err != nil {
		t.Fatal(err)
	}
	if err := q.QueryRow(context.Background(), `select count(*) from note_events where note_id=$1`, note.ID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if tombstones != 1 || events != 2 {
		t.Fatalf("tombstones=%d preserved events=%d", tombstones, events)
	}
	if _, err := q.Exec(context.Background(), `update note_events set reason='rewrite' where note_id=$1`, note.ID); err == nil {
		t.Fatal("note history lost its append-only protection")
	}
}

func TestNoteEditorLeaseAndRecoveryDraftsAreIndependentAcrossSessions(t *testing.T) {
	o := newOrg(t)
	first, userID := o.kadiv("mediatek", "two-devices@himasif.test")
	code, secondLogin, _ := o.login("two-devices@himasif.test", "kata-sandi-anggota")
	if code != http.StatusOK {
		t.Fatalf("second session login = %d", code)
	}
	second := secondLogin.Token
	reader, _ := o.join(first, o.period.ID, "mediatek", "other-draft-reader@himasif.test", "Other account")
	sid := o.spaceID(first, "division-mediatek")
	var note api.Note
	if code := o.call("POST", "/api/spaces/"+sid.String()+"/notes", first, api.NoteCreateRequest{Title: "Session note", Content: []api.NoteBlock{para("official")}}, &note); code != http.StatusCreated {
		t.Fatalf("create = %d", code)
	}
	path := "/api/notes/" + note.ID.String()
	if code := o.call("POST", path+"/lock", first, nil, nil); code != http.StatusOK {
		t.Fatalf("first lock = %d", code)
	}
	var rejected api.ErrorResponse
	if code := o.call("POST", path+"/lock", second, nil, &rejected); code != http.StatusConflict || rejected.Error.Code != api.CodeConflict {
		t.Fatalf("same-account second session acquired a live lease: %d %+v", code, rejected)
	}
	encoded, err := json.Marshal(rejected.Error.Data)
	if err != nil {
		t.Fatal(err)
	}
	var lock api.NoteLock
	if err := json.Unmarshal(encoded, &lock); err != nil || lock.Editor.ID != userID || lock.ExpiresAt.IsZero() || lock.HeldByCaller {
		t.Fatalf("lock conflict omitted safe structured editor metadata: %s err=%v", encoded, err)
	}
	if strings.Contains(strings.ToLower(string(encoded)), "session") || strings.Contains(string(encoded), first) || strings.Contains(string(encoded), second) {
		t.Fatal("conflict exposed an authentication session")
	}
	var secondView api.Note
	if code := o.call("GET", path, second, nil, &secondView); code != http.StatusOK || secondView.Lock == nil || secondView.Lock.HeldByCaller || secondView.Permissions.CanEdit {
		t.Fatalf("same-account second session displayed editable: %d %+v", code, secondView.Permissions)
	}
	// A second device cannot release the first device's lease or save content.
	if code := o.call("DELETE", path+"/lock", second, nil, nil); code != http.StatusNoContent {
		t.Fatalf("idempotent foreign-session release = %d", code)
	}
	if code, _ := o.errCode("PUT", path, second, api.NoteWriteRequest{Title: "Wrong device", Content: []api.NoteBlock{para("wrong")}, ExpectedVersion: 1}); code != http.StatusConflict {
		t.Fatalf("non-holder content save = %d", code)
	}
	draftReq := func(title string) api.NoteDraftRequest {
		return api.NoteDraftRequest{Title: title, Content: []api.NoteBlock{para(title)}, BaseVersion: 1}
	}
	var a, b api.NoteDraft
	if code := o.call("PUT", path+"/draft", first, draftReq("Device A draft"), &a); code != http.StatusOK || a.ID == id.Nil || !a.CurrentSession {
		t.Fatalf("device A draft = %d %+v", code, a)
	}
	if code := o.call("PUT", path+"/draft", second, draftReq("Device B draft"), &b); code != http.StatusOK || b.ID == id.Nil || b.ID == a.ID || !b.CurrentSession {
		t.Fatalf("device B overwrote A's draft = %d %+v", code, b)
	}
	var again api.NoteDraft
	if code := o.call("GET", path+"/draft", first, nil, &again); code != http.StatusOK || again.ID != a.ID || again.Title != a.Title || !again.CurrentSession {
		t.Fatalf("GET draft did not prefer current session: %d %+v", code, again)
	}
	var ownCopies api.List[api.NoteDraft]
	if code := o.call("GET", path+"/drafts", first, nil, &ownCopies); code != http.StatusOK || len(ownCopies.Items) != 2 {
		t.Fatalf("recoverable copies = %d %+v", code, ownCopies.Items)
	}
	var otherCopies api.List[api.NoteDraft]
	if code := o.call("GET", path+"/drafts", reader, nil, &otherCopies); code != http.StatusOK || len(otherCopies.Items) != 0 {
		t.Fatalf("other account obtained recovery drafts = %d %+v", code, otherCopies.Items)
	}
	if code, _ := o.errCode("DELETE", path+"/drafts/"+a.ID.String(), reader, nil); code != http.StatusNotFound {
		t.Fatalf("other account discarded recovery copy = %d", code)
	}
	if code := o.call("POST", path+"/lock/takeover", second, api.NoteLockRequest{Force: true}, nil); code != http.StatusOK {
		t.Fatalf("explicit same-account takeover = %d", code)
	}
	if code, _ := o.errCode("PUT", path, first, api.NoteWriteRequest{Title: "Old editor", Content: []api.NoteBlock{para("old")}, ExpectedVersion: 1}); code != http.StatusConflict {
		t.Fatalf("old session still saved after takeover = %d", code)
	}
	if code := o.call("PUT", path, second, api.NoteWriteRequest{Title: "Official device B", Content: []api.NoteBlock{para("saved")}, ExpectedVersion: 1}, &note); code != http.StatusOK || note.Version != 2 {
		t.Fatalf("holder save = %d %+v", code, note)
	}
	if code := o.call("GET", path+"/drafts", second, nil, &ownCopies); code != http.StatusOK || len(ownCopies.Items) != 1 || ownCopies.Items[0].ID != a.ID || ownCopies.Items[0].CurrentSession || !ownCopies.Items[0].Stale {
		t.Fatalf("official save discarded other-device recovery: %d %+v", code, ownCopies.Items)
	}
	if code := o.call("GET", path+"/draft", second, nil, &again); code != http.StatusOK || again.ID != a.ID || again.CurrentSession {
		t.Fatalf("reconnect fallback did not recover the remaining own draft: %d %+v", code, again)
	}
	// DELETE /draft is session-local even when GET /draft used a fallback.
	if code := o.call("DELETE", path+"/draft", second, nil, nil); code != http.StatusNoContent {
		t.Fatalf("discard current draft = %d", code)
	}
	if code := o.call("GET", path+"/drafts", second, nil, &ownCopies); code != http.StatusOK || len(ownCopies.Items) != 1 {
		t.Fatalf("discard current erased a fallback copy: %d %+v", code, ownCopies.Items)
	}
	if code := o.call("DELETE", path+"/drafts/"+a.ID.String(), second, nil, nil); code != http.StatusNoContent {
		t.Fatalf("explicit discard own recovery copy = %d", code)
	}
	if code := o.call("GET", path+"/drafts", first, nil, &ownCopies); code != http.StatusOK || len(ownCopies.Items) != 0 {
		t.Fatalf("explicit discard did not remove selected copy: %d %+v", code, ownCopies.Items)
	}
}

func TestClosedNoteStillAllowsOwnRecoveryReads(t *testing.T) {
	o := newOrg(t)
	token, _ := o.kadiv("mediatek", "closed-draft@himasif.test")
	sid := o.spaceID(token, "division-mediatek")
	var note api.Note
	o.call("POST", "/api/spaces/"+sid.String()+"/notes", token, api.NoteCreateRequest{Title: "Close later", Content: []api.NoteBlock{para("official")}}, &note)
	path := "/api/notes/" + note.ID.String()
	req := api.NoteDraftRequest{Title: "Recover me", Content: []api.NoteBlock{para("unsaved")}, BaseVersion: 1}
	var saved api.NoteDraft
	if code := o.call("PUT", path+"/draft", token, req, &saved); code != http.StatusOK {
		t.Fatalf("draft = %d", code)
	}
	if code := o.call("POST", "/api/periods/"+o.period.ID.String()+"/close", o.admin, nil, nil); code != http.StatusOK {
		t.Fatalf("close = %d", code)
	}
	var recovered api.NoteDraft
	if code := o.call("GET", path+"/draft", token, nil, &recovered); code != http.StatusOK || recovered.ID != saved.ID || recovered.Title != req.Title {
		t.Fatalf("closed-period recovery = %d %+v", code, recovered)
	}
	var copies api.List[api.NoteDraft]
	if code := o.call("GET", path+"/drafts", token, nil, &copies); code != http.StatusOK || len(copies.Items) != 1 {
		t.Fatalf("closed-period recovery list = %d %+v", code, copies.Items)
	}
	for _, change := range []struct {
		method, suffix string
		body           any
	}{
		{"PUT", "/draft", req}, {"DELETE", "/draft", nil}, {"DELETE", "/drafts/" + saved.ID.String(), nil},
	} {
		if code, reason := o.errCode(change.method, path+change.suffix, token, change.body); code != http.StatusConflict || reason != api.CodePeriodClosed {
			t.Fatalf("closed-period draft mutation %s%s = %d %s", change.method, change.suffix, code, reason)
		}
	}
}

func TestNotesReviewMigrationPreservesLegacyRecoveryAndIsIdempotent(t *testing.T) {
	o := newOrg(t)
	token, userID := o.kadiv("mediatek", "legacy-draft@himasif.test")
	sid := o.spaceID(token, "division-mediatek")
	var note api.Note
	if code := o.call("POST", "/api/spaces/"+sid.String()+"/notes", token, api.NoteCreateRequest{Title: "Legacy", Content: []api.NoteBlock{para("official")}}, &note); code != http.StatusCreated {
		t.Fatalf("create = %d", code)
	}
	ctx, q := context.Background(), o.st.DB()
	// This fixture owns a disposable database. Reproduce exactly the old 0005
	// shape, add old recovery data, then run only the new migration through the
	// normal ledger runner. No deployed or preview database is touched.
	if err := o.st.Tx(ctx, func(tx store.Querier) error {
		_, err := tx.Exec(ctx, `drop index note_drafts_session, note_drafts_recovery;
			alter table note_drafts drop constraint note_drafts_pkey;
			alter table note_drafts drop column id, drop column session_id;
			alter table note_drafts add primary key (note_id,user_id);
			alter table note_locks drop column session_id;
			alter table notes drop column deleted_at;
			delete from schema_migrations where name='0006_notes_review'`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	legacyAt := time.Date(2026, 10, 28, 5, 0, 0, 0, time.UTC)
	if _, err := q.Exec(ctx, `insert into note_drafts(note_id,user_id,title,content,base_version,saved_at)
		values($1,$2,'Legacy recovery','[{"type":"paragraph","spans":[{"text":"keep me"}]}]',1,$3)`, note.ID, userID, legacyAt); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Exec(ctx, `insert into note_locks(note_id,editor_id,acquired_at,expires_at) values($1,$2,now(),now()+interval '1 hour')`, note.ID, userID); err != nil {
		t.Fatal(err)
	}
	snapshot := func() string {
		t.Helper()
		var value string
		if err := q.QueryRow(ctx, `select jsonb_build_object('title',title,'content',content,'baseVersion',base_version,'savedAt',saved_at)::text
			from note_drafts where note_id=$1 and user_id=$2`, note.ID, userID).Scan(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	before := snapshot()
	applied, err := o.st.Migrate(ctx)
	if err != nil || !slices.Equal(applied, []string{"0006_notes_review"}) {
		t.Fatalf("upgrade applied=%v err=%v", applied, err)
	}
	if after := snapshot(); after != before {
		t.Fatalf("legacy draft changed: before=%s after=%s", before, after)
	}
	var expired bool
	if err := q.QueryRow(ctx, `select expires_at<=now() and session_id is null from note_locks where note_id=$1`, note.ID).Scan(&expired); err != nil || !expired {
		t.Fatalf("legacy account-only lease remained live: expired=%v err=%v", expired, err)
	}
	path := "/api/notes/" + note.ID.String()
	var recovery api.NoteDraft
	if code := o.call("GET", path+"/draft", token, nil, &recovery); code != http.StatusOK || recovery.ID == id.Nil || recovery.CurrentSession || recovery.Title != "Legacy recovery" || !recovery.SavedAt.Equal(legacyAt) {
		t.Fatalf("legacy draft not recoverable: %d %+v", code, recovery)
	}
	if code := o.call("POST", path+"/lock", token, nil, nil); code != http.StatusOK {
		t.Fatalf("reacquire upgraded lease = %d", code)
	}
	if code := o.call("PUT", path, token, api.NoteWriteRequest{Title: "New official", Content: []api.NoteBlock{para("new")}, ExpectedVersion: 1}, nil); code != http.StatusOK {
		t.Fatalf("save after upgrade = %d", code)
	}
	var copies api.List[api.NoteDraft]
	if code := o.call("GET", path+"/drafts", token, nil, &copies); code != http.StatusOK || len(copies.Items) != 1 || copies.Items[0].ID != recovery.ID || !copies.Items[0].Stale {
		t.Fatalf("new save erased legacy recovery: %d %+v", code, copies.Items)
	}
	applied, err = o.st.Migrate(ctx)
	if err != nil || len(applied) != 0 || snapshot() != before {
		t.Fatalf("repeat upgrade changed data: applied=%v err=%v", applied, err)
	}
}
