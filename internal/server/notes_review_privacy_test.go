package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func TestNoteLinksFollowTargetAccessAndReminderPrivacy(t *testing.T) {
	o := newTaskOrg(t)
	peer, peerAccount := o.join(o.kadiv, o.period.ID, "mediatek", "note-private-peer@himasif.test", "Peer Mediatek")
	if status := o.call("PUT", o.scope(o.period.ID, "humas")+"/kadiv", o.admin, api.SetKadivRequest{UserID: o.peerID}, nil); status != http.StatusOK {
		t.Fatalf("set Humas kadiv=%d", status)
	}
	sharedSpace := o.spaces["medhum"]
	var note api.Note
	if status := o.call("POST", "/api/spaces/"+sharedSpace.String()+"/notes", o.member, api.NoteCreateRequest{Title: "Rujukan rapat", Content: []api.NoteBlock{para("Tindak lanjut")}}, &note); status != http.StatusCreated {
		t.Fatalf("create shared note=%d", status)
	}
	linksPath := "/api/notes/" + note.ID.String() + "/links"
	historyPath := "/api/notes/" + note.ID.String() + "/history"
	foreignReq := api.TaskWriteRequest{OwnerDivisionID: o.div["humas"].ID, Title: "Internal Humas", ExpectedOutput: "Berkas", AssigneeID: o.peerID, DeadlineDate: taskTestDate(t, "2026-11-05")}
	var foreign api.Task
	if status := o.call("POST", "/api/spaces/"+o.spaces["division-humas"].String()+"/tasks", o.peer, foreignReq, &foreign); status != http.StatusCreated {
		t.Fatalf("create private task=%d", status)
	}
	for _, target := range []id.ID{foreign.ID, id.New()} {
		if status, code := o.errCode("POST", linksPath, o.member, api.NoteLinkRequest{TargetType: api.NoteLinkTask, TargetID: target}); status != http.StatusNotFound || code != api.CodeNotFound {
			t.Fatalf("foreign/absent task link disclosed existence: %d %s", status, code)
		}
	}
	if o.audits("note.link.add") != 0 {
		t.Fatal("rejected targets produced committed link events")
	}
	allowedReq := api.TaskWriteRequest{OwnerDivisionID: o.div["mediatek"].ID, SpaceIDs: []id.ID{sharedSpace}, Title: "Desain bersama", ExpectedOutput: "Berkas", AssigneeID: o.memberID, DeadlineDate: taskTestDate(t, "2026-11-05")}
	var allowed api.Task
	if status := o.call("POST", "/api/spaces/"+o.spaces["division-mediatek"].String()+"/tasks", o.kadiv, allowedReq, &allowed); status != http.StatusCreated {
		t.Fatalf("create readable task=%d", status)
	}
	if status := o.call("POST", linksPath, o.member, api.NoteLinkRequest{TargetType: api.NoteLinkTask, TargetID: allowed.ID}, nil); status != http.StatusCreated {
		t.Fatalf("readable target rejected for a non-creator member=%d", status)
	}
	var reminder api.Reminder
	if status := o.call("POST", "/api/reminders", o.member, dateReq("Personal follow-up", "2026-11-06"), &reminder); status != http.StatusCreated {
		t.Fatalf("private reminder=%d", status)
	}
	var privateLink api.NoteLink
	if status := o.call("POST", linksPath, o.member, api.NoteLinkRequest{TargetType: api.NoteLinkReminder, TargetID: reminder.ID}, &privateLink); status != http.StatusCreated {
		t.Fatalf("link own reminder=%d", status)
	}
	assertLinks := func(token string, targets ...id.ID) {
		t.Helper()
		var links api.List[api.NoteLink]
		if status := o.call("GET", linksPath, token, nil, &links); status != http.StatusOK || len(links.Items) != len(targets) {
			t.Fatalf("visible links=%d %+v, want targets%v", status, links.Items, targets)
		}
		for _, target := range targets {
			found := false
			for _, link := range links.Items {
				found = found || link.TargetID == target
			}
			if !found {
				t.Fatalf("target %s absent from %+v", target, links.Items)
			}
		}
	}
	assertHistoryPrivacy := func(token string, visible bool) {
		t.Helper()
		var history api.List[api.NoteEvent]
		if status := o.call("GET", historyPath, token, nil, &history); status != http.StatusOK {
			t.Fatalf("history=%d", status)
		}
		raw, err := json.Marshal(history)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), reminder.ID.String()) != visible || strings.Contains(string(raw), privateLink.ID.String()) != visible {
			t.Fatalf("history private target visibility incorrect: visible=%v history=%s", visible, raw)
		}
	}
	assertLinks(o.member, allowed.ID, reminder.ID)
	assertHistoryPrivacy(o.member, true)
	assertLinks(peer, allowed.ID)
	assertHistoryPrivacy(peer, false)
	assertLinks(o.peer, allowed.ID)
	reminderPath := "/api/reminders/" + reminder.ID.String()
	if status, _ := o.errCode("GET", reminderPath, peer, nil); status != http.StatusNotFound {
		t.Fatalf("note link granted private reminder access=%d", status)
	}
	if status := o.call("POST", reminderPath+"/recipients", o.member, api.ReminderShareRequest{RecipientID: peerAccount.User.ID}, nil); status != http.StatusCreated {
		t.Fatalf("add recipient=%d", status)
	}
	// A dormant recipient row does not make the link or history public.
	assertLinks(peer, allowed.ID)
	assertHistoryPrivacy(peer, false)
	if status := o.call("PUT", reminderPath+"/sharing", o.member, api.ReminderSharingRequest{Shared: true}, nil); status != http.StatusOK {
		t.Fatalf("enable sharing=%d", status)
	}
	assertLinks(peer, allowed.ID, reminder.ID)
	assertHistoryPrivacy(peer, true)
	if status := o.call("DELETE", reminderPath+"/recipients/"+peerAccount.User.ID.String(), o.member, nil, nil); status != http.StatusNoContent {
		t.Fatalf("revoke recipient=%d", status)
	}
	assertLinks(peer, allowed.ID)
	assertHistoryPrivacy(peer, false)
	// Removing a link records sufficient target context to apply privacy to its
	// historical event too, even though the live link is no longer present.
	if status := o.call("DELETE", linksPath+"/"+privateLink.ID.String(), o.member, nil, nil); status != http.StatusNoContent {
		t.Fatalf("delete private link=%d", status)
	}
	assertHistoryPrivacy(o.member, true)
	assertHistoryPrivacy(peer, false)
	// Taking the task out of Medhum must hide its link from Humas while leaving
	// the source note readable there. Visibility is not cached at link creation.
	allowedReq.SpaceIDs, allowedReq.ExpectedVersion, allowedReq.Reason = nil, allowed.Version, "Kembali menjadi pekerjaan internal"
	if status := o.call("PATCH", "/api/tasks/"+allowed.ID.String(), o.kadiv, allowedReq, nil); status != http.StatusOK {
		t.Fatalf("remove shared task projection=%d", status)
	}
	assertLinks(o.peer)
	assertLinks(peer, allowed.ID)
}

func TestUnreadableNoteAndSubresourcesDoNotDiscloseExistence(t *testing.T) {
	o := newTaskOrg(t)
	var note api.Note
	if status := o.call("POST", "/api/spaces/"+o.spaces["division-mediatek"].String()+"/notes", o.member, api.NoteCreateRequest{Title: "Internal", Content: []api.NoteBlock{para("Internal")}}, &note); status != http.StatusCreated {
		t.Fatalf("create internal note=%d", status)
	}
	for _, suffix := range []string{"", "/history", "/links", "/draft"} {
		for _, target := range []id.ID{note.ID, id.New()} {
			if status, code := o.errCode("GET", "/api/notes/"+target.String()+suffix, o.peer, nil); status != http.StatusNotFound || code != api.CodeNotFound {
				t.Errorf("hidden/absent note%s=%d %s, want404", suffix, status, code)
			}
		}
	}
}

func TestNoteDocumentSizeLimitAppliesToEveryWrite(t *testing.T) {
	o := newTaskOrg(t)
	sid := o.spaces["division-mediatek"]
	var note api.Note
	if status := o.call("POST", "/api/spaces/"+sid.String()+"/notes", o.member, api.NoteCreateRequest{Title: "Small", Content: []api.NoteBlock{para("Small")}}, &note); status != http.StatusCreated {
		t.Fatalf("create note=%d", status)
	}
	if status := o.call("POST", "/api/notes/"+note.ID.String()+"/lock", o.member, nil, nil); status != http.StatusOK {
		t.Fatalf("lock note=%d", status)
	}
	var template api.NoteTemplate
	if status := o.call("POST", "/api/spaces/"+sid.String()+"/note-templates", o.member, api.NoteTemplateRequest{Name: "Small", Content: []api.NoteBlock{para("Small")}}, &template); status != http.StatusCreated {
		t.Fatalf("create template=%d", status)
	}
	large := make([]api.NoteBlock, 60)
	for i := range large {
		large[i] = para(strings.Repeat("x", api.NoteMaxBlockText))
	}
	for _, tc := range []struct {
		method, path string
		body         any
	}{
		{"POST", "/api/spaces/" + sid.String() + "/notes", api.NoteCreateRequest{Title: "Oversized", Content: large}},
		{"PUT", "/api/notes/" + note.ID.String(), api.NoteWriteRequest{Title: "Oversized", Content: large, ExpectedVersion: note.Version}},
		{"PUT", "/api/notes/" + note.ID.String() + "/draft", api.NoteDraftRequest{Title: "Oversized", Content: large, BaseVersion: note.Version}},
		{"POST", "/api/spaces/" + sid.String() + "/note-templates", api.NoteTemplateRequest{Name: "Oversized", Content: large}},
		{"PUT", "/api/note-templates/" + template.ID.String(), api.NoteTemplateRequest{Name: "Oversized", Content: large}},
	} {
		if status, _ := o.errCode(tc.method, tc.path, o.member, tc.body); status != http.StatusBadRequest {
			t.Errorf("oversized content via %s %s=%d", tc.method, tc.path, status)
		}
	}
	var current api.Note
	if status := o.call("GET", "/api/notes/"+note.ID.String(), o.member, nil, &current); status != http.StatusOK || current.Version != note.Version || current.Title != "Small" {
		t.Fatalf("oversized write changed original=%d %+v", status, current)
	}
}

func TestNoteTemplateWritesRevalidatePrincipalAfterWaiting(t *testing.T) {
	for _, action := range []string{"create", "update", "delete"} {
		for _, revoke := range []string{"logout", "disable"} {
			t.Run(action+"/"+revoke, func(t *testing.T) {
				o := newTaskOrg(t)
				sid := o.spaces["division-mediatek"]
				var template api.NoteTemplate
				if status := o.call("POST", "/api/spaces/"+sid.String()+"/note-templates", o.member, api.NoteTemplateRequest{Name: "Original", Content: []api.NoteBlock{para("Original")}}, &template); status != http.StatusCreated {
					t.Fatalf("template fixture=%d", status)
				}
				beforeAudit := o.audits("note.template." + action)
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				conn, err := regressionConnection(ctx, o.st)
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close(context.Background())
				tx, err := conn.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(context.Background())
				if _, err := o.st.UserForUpdate(ctx, tx, o.memberID); err != nil {
					t.Fatal(err)
				}
				method, path := "POST", "/api/spaces/"+sid.String()+"/note-templates"
				var body any = api.NoteTemplateRequest{Name: "Unauthorized change", Content: []api.NoteBlock{para("Changed")}}
				if action == "update" {
					method, path = "PUT", "/api/note-templates/"+template.ID.String()
				} else if action == "delete" {
					method, path, body = "DELETE", "/api/note-templates/"+template.ID.String(), nil
				}
				done := accessCallAsync(ctx, o.env, method, path, o.member, body)
				waitAccessQueryBlocked(ctx, t, o.st, "", "", done)
				now := time.Now()
				if revoke == "disable" {
					if _, err := o.st.SetDisabled(ctx, tx, o.memberID, true, now); err != nil {
						t.Fatal(err)
					}
				} else if _, err := o.st.RevokeUserSessions(ctx, tx, o.memberID, nil, now); err != nil {
					t.Fatal(err)
				}
				if err := tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				select {
				case result := <-done:
					if result.err != nil || result.status != http.StatusUnauthorized {
						t.Fatalf("stale %s/%s status=%d err=%v", action, revoke, result.status, result.err)
					}
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				persisted, _, _, err := o.st.Template(ctx, o.st.DB(), template.ID)
				if err != nil || persisted.Name != "Original" || o.audits("note.template."+action) != beforeAudit {
					t.Fatalf("revoked template write committed: %+v err=%v", persisted, err)
				}
				var count int
				if err := o.st.DB().QueryRow(ctx, "select count(*) from note_templates").Scan(&count); err != nil || count != 1 {
					t.Fatalf("revoked write changed template count=%d err=%v", count, err)
				}
			})
		}
	}
}

// A legacy history event without target context must not disclose a link ID
// which can no longer be classified against the target's current permissions.
func TestNoteLinkHistoryRequiresClassifiableTarget(t *testing.T) {
	e := api.NoteEvent{Action: "note.link.remove", Before: map[string]any{"linkId": id.New()}}
	if _, ok := noteEventLink(e); ok {
		t.Fatal("legacy removal event incorrectly treated as a visible target")
	}
}
