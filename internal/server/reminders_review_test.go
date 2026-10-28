package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
)

// Authentication can finish before logout or disable commits. Every reminder
// write must revalidate that principal inside its transaction, just as account
// and task writes do; ownership alone does not revive a revoked session.
func TestReminderWritesRejectRevokedPrincipal(t *testing.T) {
	for _, revoke := range []string{"logout", "disable"} {
		t.Run(revoke, func(t *testing.T) {
			o := newOrg(t)
			ownerToken, owner := o.join(o.admin, o.period.ID, "humas", "reminder-owner@himasif.test", "Owner")
			_, recipient := o.join(o.admin, o.period.ID, "humas", "reminder-recipient@himasif.test", "Recipient")
			var reminder api.Reminder
			if status := o.call("POST", "/api/reminders", ownerToken, dateReq("Original", "2026-11-10"), &reminder); status != http.StatusCreated {
				t.Fatalf("create fixture=%d", status)
			}
			path := "/api/reminders/" + reminder.ID.String()
			if status := o.call("POST", path+"/recipients", ownerToken, api.ReminderShareRequest{RecipientID: recipient.User.ID}, nil); status != http.StatusCreated {
				t.Fatalf("share fixture=%d", status)
			}
			capture := httptest.NewRequest("GET", "/api/me", nil)
			capture.Header.Set("Authorization", "Bearer "+ownerToken)
			old, err := o.srv.principal(capture)
			if err != nil {
				t.Fatal(err)
			}
			if revoke == "logout" {
				if status := o.call("POST", "/api/auth/logout", ownerToken, nil, nil); status != http.StatusNoContent {
					t.Fatalf("logout=%d", status)
				}
			} else if status := o.call("POST", "/api/users/"+owner.User.ID.String()+"/disable", o.admin, nil, nil); status != http.StatusOK {
				t.Fatalf("disable=%d", status)
			}
			beforeAudit := o.audits("reminder.create") + o.audits("reminder.update") + o.audits("reminder.complete") + o.audits("reminder.sharing") + o.audits("reminder.share.add") + o.audits("reminder.share.remove") + o.audits("reminder.preferences.set") + o.audits("reminder.delete")
			cases := []struct {
				name, method, path string
				body               any
				action             func(http.ResponseWriter, *http.Request, *principal) error
			}{
				{"create", "POST", "/api/reminders", dateReq("Unauthorized new reminder", "2026-11-11"), o.srv.createReminder},
				{"update", "PUT", path, dateReq("Unauthorized change", "2026-11-12"), o.srv.updateReminder},
				{"complete", "POST", path + "/complete", api.ReminderCompleteRequest{Completed: true}, o.srv.completeReminder},
				{"sharing", "PUT", path + "/sharing", api.ReminderSharingRequest{Shared: true}, o.srv.setReminderSharing},
				{"remove-recipient", "DELETE", path + "/recipients/" + recipient.User.ID.String(), nil, o.srv.unshareReminder},
				{"add-recipient", "POST", path + "/recipients", api.ReminderShareRequest{RecipientID: recipient.User.ID}, o.srv.shareReminder},
				{"preferences", "PUT", "/api/reminders/preferences", api.ReminderPreferences{DefaultShare: true}, o.srv.putReminderPreferences},
				{"delete", "DELETE", path, nil, o.srv.deleteReminder},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					body, err := json.Marshal(tc.body)
					if err != nil {
						t.Fatal(err)
					}
					req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(body))
					req.SetPathValue("reminderId", reminder.ID.String())
					req.SetPathValue("recipientId", recipient.User.ID.String())
					rec := httptest.NewRecorder()
					if err := tc.action(rec, req, old); err != nil {
						o.srv.answerError(rec, req, old, err)
					}
					if rec.Code != http.StatusUnauthorized {
						t.Errorf("%s request with revoked session=%d %s; want401", tc.name, rec.Code, rec.Body.String())
					}
				})
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var reminders, shares, audits int
			if err := o.st.DB().QueryRow(ctx, `select
 (select count(*) from reminders where owner_id=$1),
 (select count(*) from reminder_shares where reminder_id=$2),
 (select count(*) from audit_events where actor_user_id=$1 and action like 'reminder.%')`, owner.User.ID, reminder.ID).Scan(&reminders, &shares, &audits); err != nil {
				t.Fatal(err)
			}
			if reminders != 1 || shares != 1 || audits != beforeAudit {
				t.Errorf("revoked requests changed persisted state: reminders=%d shares=%d audits=%d; want1,1,%d", reminders, shares, audits, beforeAudit)
			}
			persisted, err := o.st.Reminder(ctx, o.st.DB(), reminder.ID, store.LockNone)
			if err != nil || persisted.Title != "Original" || persisted.Shared || persisted.CompletedAt != nil {
				t.Errorf("revoked requests changed original reminder: %+v err=%v", persisted, err)
			}
		})
	}
}

// The endpoint promises one due-ordered list, with all completed reminders at
// the back, regardless of whether the caller owns or receives each reminder.
func TestReminderListOrdersOwnedAndReceivedTogether(t *testing.T) {
	o := newOrg(t)
	ownerToken, _ := o.join(o.admin, o.period.ID, "humas", "list-owner@himasif.test", "Owner")
	viewerToken, viewer := o.join(o.admin, o.period.ID, "humas", "list-viewer@himasif.test", "Viewer")
	create := func(token string, req api.ReminderRequest) api.Reminder {
		t.Helper()
		var reminder api.Reminder
		if status := o.call("POST", "/api/reminders", token, req, &reminder); status != http.StatusCreated {
			t.Fatalf("create %q=%d", req.Title, status)
		}
		return reminder
	}
	done := create(viewerToken, dateReq("Completed, earliest date", "2026-10-01"))
	if status := o.call("POST", "/api/reminders/"+done.ID.String()+"/complete", viewerToken, api.ReminderCompleteRequest{Completed: true}, nil); status != http.StatusOK {
		t.Fatalf("complete=%d", status)
	}
	later := create(viewerToken, dateReq("Owned, later date", "2026-11-05"))
	at := time.Date(2026, 11, 3, 9, 30, 0, 0, time.UTC)
	instant := create(viewerToken, api.ReminderRequest{Title: "Owned, instant", DueKind: api.DueInstant, DueAt: &at, TimeZone: "Asia/Jakarta"})
	shared := create(ownerToken, dateReq("Received, nearest date", "2026-11-01"))
	path := "/api/reminders/" + shared.ID.String()
	if status := o.call("POST", path+"/recipients", ownerToken, api.ReminderShareRequest{RecipientID: viewer.User.ID}, nil); status != http.StatusCreated {
		t.Fatalf("share=%d", status)
	}
	if status := o.call("PUT", path+"/sharing", ownerToken, api.ReminderSharingRequest{Shared: true}, nil); status != http.StatusOK {
		t.Fatalf("enable sharing=%d", status)
	}
	var list api.List[api.Reminder]
	if status := o.call("GET", "/api/reminders", viewerToken, nil, &list); status != http.StatusOK {
		t.Fatalf("list=%d", status)
	}
	var got []id.ID
	for _, reminder := range list.Items {
		got = append(got, reminder.ID)
	}
	if want := []id.ID{shared.ID, instant.ID, later.ID, done.ID}; !slices.Equal(got, want) {
		t.Fatalf("combined order=%v; want nearest received, owned instant, owned date, completed: %v", got, want)
	}
	if list.Items[0].DueOn == nil || list.Items[0].DueAt != nil || list.Items[1].DueOn != nil || list.Items[1].DueAt == nil || !list.Items[1].DueAt.Equal(at) {
		t.Fatalf("ordering changed date/instant representations: %+v", list.Items)
	}
}

// Holding the lower account first makes the old recipient-first lock order
// produce a cycle: B->A waits on A, while A->B holds B and waits on its A FK.
// With globally ordered account locks both requests wait on A and then succeed.
func TestReminderReciprocalSharingUsesConsistentAccountLocks(t *testing.T) {
	o := newOrg(t)
	tokenA, accountA := o.join(o.admin, o.period.ID, "humas", "share-a@himasif.test", "A")
	tokenB, accountB := o.join(o.admin, o.period.ID, "humas", "share-b@himasif.test", "B")
	uidA, uidB := accountA.User.ID, accountB.User.ID
	if strings.Compare(uidA.String(), uidB.String()) > 0 {
		uidA, uidB, tokenA, tokenB = uidB, uidA, tokenB, tokenA
	}
	var reminderA, reminderB api.Reminder
	if status := o.call("POST", "/api/reminders", tokenA, dateReq("A reminder", "2026-11-01"), &reminderA); status != http.StatusCreated {
		t.Fatalf("create A=%d", status)
	}
	if status := o.call("POST", "/api/reminders", tokenB, dateReq("B reminder", "2026-11-02"), &reminderB); status != http.StatusCreated {
		t.Fatalf("create B=%d", status)
	}
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
	if _, err := o.st.UserForUpdate(ctx, tx, uidA); err != nil {
		t.Fatal(err)
	}
	first := accessCallAsync(ctx, o.env, "POST", "/api/reminders/"+reminderB.ID.String()+"/recipients", tokenB, api.ReminderShareRequest{RecipientID: uidA})
	waitAccessQueryBlocked(ctx, t, o.st, "from users where id = $1 for update", "", first)
	second := accessCallAsync(ctx, o.env, "POST", "/api/reminders/"+reminderA.ID.String()+"/recipients", tokenA, api.ReminderShareRequest{RecipientID: uidB})
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting int
		if err := o.st.DB().QueryRow(ctx, `select count(*) from pg_stat_activity where datname=current_database()
 and pid<>pg_backend_pid() and wait_event_type='Lock'`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting == 2 {
			break
		}
		select {
		case result := <-first:
			t.Fatalf("first share completed before both requests waited: %+v", result)
		case result := <-second:
			t.Fatalf("second share completed before both requests waited: %+v", result)
		case <-ctx.Done():
			t.Fatal("both reciprocal shares did not reach their lock waits: ", ctx.Err())
		case <-ticker.C:
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for _, done := range []<-chan accessHTTPResult{first, second} {
		select {
		case result := <-done:
			if result.err != nil || result.status != http.StatusCreated {
				t.Errorf("reciprocal share status=%d err=%v; want201", result.status, result.err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	var shares int
	if err := o.st.DB().QueryRow(ctx, "select count(*) from reminder_shares where reminder_id in ($1,$2)", reminderA.ID, reminderB.ID).Scan(&shares); err != nil {
		t.Fatal(err)
	}
	if shares != 2 {
		t.Fatalf("reciprocal sharing persisted %d grants; want2", shares)
	}
}
