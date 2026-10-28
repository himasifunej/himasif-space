package server

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// reminder env: a couple of personal accounts to own and receive reminders.
func dateReq(title, on string) api.ReminderRequest {
	d, _ := timeutil.ParseDate(on)
	return api.ReminderRequest{Title: title, DueKind: api.DueDate, DueOn: &d}
}

// AC-09 / PRI-01, PRI-02: a member keeps a private reminder of their own; it
// is not a division's task and no one else sees it by default.
func TestReminderCreatePrivateByDefault(t *testing.T) {
	o := newOrg(t)
	kTok, _ := o.kadiv("mediatek", "km@himasif.test")
	aTok, _ := o.join(kTok, o.period.ID, "mediatek", "a@himasif.test", "A")
	bTok, _ := o.join(kTok, o.period.ID, "mediatek", "b@himasif.test", "B")

	var rem api.Reminder
	if c := o.call("POST", "/api/reminders", aTok, dateReq("Kumpulkan desain", "2026-11-10"), &rem); c != 201 {
		t.Fatalf("create = %d", c)
	}
	if rem.Relation != api.ReminderOwner || rem.Shared || rem.DueKind != api.DueDate || rem.DueOn == nil ||
		rem.DueOn.String() != "2026-11-10" || rem.DueAt != nil || rem.TimeZone != timeutil.DefaultZoneName || rem.Completed {
		t.Fatalf("new reminder %+v", rem)
	}
	// The owner lists it; B does not.
	var mine api.List[api.Reminder]
	o.call("GET", "/api/reminders", aTok, nil, &mine)
	if len(mine.Items) != 1 || mine.Items[0].ID != rem.ID {
		t.Fatalf("owner list %+v", mine)
	}
	var theirs api.List[api.Reminder]
	o.call("GET", "/api/reminders", bTok, nil, &theirs)
	if len(theirs.Items) != 0 {
		t.Fatalf("private reminder leaked to B %+v", theirs)
	}
	// B cannot read it directly: it is as good as absent (PRI-08).
	if c, code := o.errCode("GET", "/api/reminders/"+rem.ID.String(), bTok, nil); c != 404 || code != api.CodeNotFound {
		t.Fatalf("B reads private = %d %s", c, code)
	}
}

// PRI-04: the admin account keeps no personal reminders.
func TestReminderAdminHasNone(t *testing.T) {
	o := newOrg(t)
	if c, code := o.errCode("POST", "/api/reminders", o.admin, dateReq("x", "2026-11-10")); c != 403 || code != api.CodeForbidden {
		t.Fatalf("admin create = %d %s", c, code)
	}
	if c, _ := o.errCode("GET", "/api/reminders", o.admin, nil); c != 403 {
		t.Fatalf("admin list = %d", c)
	}
}

// MED-04/MED-05: a date-only reminder and an instant reminder are distinct;
// a date is never given a made-up hour, an instant keeps its moment (UTC).
func TestReminderDateVersusInstant(t *testing.T) {
	o := newOrg(t)
	kTok, _ := o.kadiv("mediatek", "km@himasif.test")
	aTok, _ := o.join(kTok, o.period.ID, "mediatek", "a@himasif.test", "A")

	// A date-only reminder may not carry a time.
	d, _ := timeutil.ParseDate("2026-11-10")
	at := time.Date(2026, 11, 10, 6, 0, 0, 0, time.UTC)
	if c, _ := o.errCode("POST", "/api/reminders", aTok, api.ReminderRequest{Title: "x", DueKind: api.DueDate, DueOn: &d, DueAt: &at}); c != 400 {
		t.Fatalf("date with time = %d", c)
	}
	// An instant reminder keeps the exact moment.
	var rem api.Reminder
	if c := o.call("POST", "/api/reminders", aTok, api.ReminderRequest{Title: "Rapat", DueKind: api.DueInstant, DueAt: &at, TimeZone: "Asia/Jakarta"}, &rem); c != 201 {
		t.Fatalf("instant = %d", c)
	}
	if rem.DueKind != api.DueInstant || rem.DueAt == nil || !rem.DueAt.Equal(at) || rem.DueOn != nil {
		t.Fatalf("instant reminder %+v", rem)
	}
	// An instant reminder may not carry a date-only due.
	if c, _ := o.errCode("POST", "/api/reminders", aTok, api.ReminderRequest{Title: "x", DueKind: api.DueInstant, DueOn: &d}); c != 400 {
		t.Fatalf("instant with dueOn = %d", c)
	}
	// An unknown zone is refused.
	if c, _ := o.errCode("POST", "/api/reminders", aTok, api.ReminderRequest{Title: "x", DueKind: api.DueDate, DueOn: &d, TimeZone: "Mars/Olympus"}); c != 400 {
		t.Fatalf("bad zone = %d", c)
	}
}

// AC-09 / PRI-03, PRI-05, PRI-07: sharing is to an account, the owner sees
// and revokes recipients, and the recipient only reads.
func TestReminderSharingIsPerAccountAndReadOnly(t *testing.T) {
	o := newOrg(t)
	kTok, _ := o.kadiv("mediatek", "km@himasif.test")
	aTok, aAcc := o.join(kTok, o.period.ID, "mediatek", "a@himasif.test", "A")
	bTok, bAcc := o.join(kTok, o.period.ID, "mediatek", "b@himasif.test", "B")

	var rem api.Reminder
	o.call("POST", "/api/reminders", aTok, dateReq("Follow-up H-4", "2026-11-12"), &rem)

	// Share with B's account, then turn sharing on.
	var shared api.Reminder
	if c := o.call("POST", "/api/reminders/"+rem.ID.String()+"/recipients", aTok, api.ReminderShareRequest{RecipientID: bAcc.User.ID}, &shared); c != 201 {
		t.Fatalf("share = %d", c)
	}
	if len(shared.Recipients) != 1 || shared.Recipients[0].RecipientID != bAcc.User.ID || shared.Recipients[0].DisplayName != "B" {
		t.Fatalf("recipients %+v", shared.Recipients)
	}
	if c := o.call("PUT", "/api/reminders/"+rem.ID.String()+"/sharing", aTok, api.ReminderSharingRequest{Shared: true}, nil); c != 200 {
		t.Fatalf("enable sharing = %d", c)
	}
	// B now sees it, read-only, without the recipient list.
	var bList api.List[api.Reminder]
	o.call("GET", "/api/reminders", bTok, nil, &bList)
	if len(bList.Items) != 1 || bList.Items[0].Relation != api.ReminderRecipient || bList.Items[0].Recipients != nil || bList.Items[0].Shared {
		t.Fatalf("recipient view %+v", bList.Items)
	}
	// B may not change, complete, delete or re-share it (PRI-07).
	rid := rem.ID.String()
	for _, tc := range []struct {
		method, path string
		body         any
	}{
		{"PUT", "/api/reminders/" + rid, dateReq("ubah", "2026-11-13")},
		{"POST", "/api/reminders/" + rid + "/complete", api.ReminderCompleteRequest{Completed: true}},
		{"PUT", "/api/reminders/" + rid + "/sharing", api.ReminderSharingRequest{Shared: false}},
		{"POST", "/api/reminders/" + rid + "/recipients", api.ReminderShareRequest{RecipientID: aAcc.User.ID}},
		{"DELETE", "/api/reminders/" + rid, nil},
		{"GET", "/api/reminders/" + rid + "/recipients", nil},
		{"GET", "/api/reminders/" + rid + "/history", nil},
	} {
		if c, code := o.errCode(tc.method, tc.path, bTok, tc.body); c != 403 || code != api.CodeForbidden {
			t.Errorf("recipient %s %s = %d %s, want 403", tc.method, tc.path, c, code)
		}
	}
	// The owner revokes B; B loses access at once.
	if c := o.call("DELETE", "/api/reminders/"+rid+"/recipients/"+bAcc.User.ID.String(), aTok, nil, nil); c != 204 {
		t.Fatalf("revoke = %d", c)
	}
	o.call("GET", "/api/reminders", bTok, nil, &bList)
	if len(bList.Items) != 0 {
		t.Fatalf("B still sees revoked reminder %+v", bList.Items)
	}
	if c, _ := o.errCode("GET", "/api/reminders/"+rid, bTok, nil); c != 404 {
		t.Fatalf("B reads revoked = %d", c)
	}
	// Cannot share with oneself; recipient must be a real active person.
	if c, _ := o.errCode("POST", "/api/reminders/"+rid+"/recipients", aTok, api.ReminderShareRequest{RecipientID: aAcc.User.ID}); c != 400 {
		t.Fatalf("self-share = %d", c)
	}
	if c, _ := o.errCode("POST", "/api/reminders/"+rid+"/recipients", aTok, api.ReminderShareRequest{RecipientID: id.New()}); c != 400 {
		t.Fatalf("share to stranger = %d", c)
	}
}

// PRI-02, PRI-05: the owner's default for new reminders can start them
// shared, and changing it does not reopen the old ones.
func TestReminderDefaultShareDoesNotReopenOld(t *testing.T) {
	o := newOrg(t)
	kTok, _ := o.kadiv("mediatek", "km@himasif.test")
	aTok, _ := o.join(kTok, o.period.ID, "mediatek", "a@himasif.test", "A")

	// An old reminder, created private.
	var old api.Reminder
	o.call("POST", "/api/reminders", aTok, dateReq("Lama", "2026-11-01"), &old)
	if old.Shared {
		t.Fatalf("old reminder started shared")
	}
	// Turn the default on.
	var pref api.ReminderPreferences
	if c := o.call("PUT", "/api/reminders/preferences", aTok, api.ReminderPreferences{DefaultShare: true}, &pref); c != 200 || !pref.DefaultShare {
		t.Fatalf("set default = %d %+v", c, pref)
	}
	o.call("GET", "/api/reminders/preferences", aTok, nil, &pref)
	if !pref.DefaultShare {
		t.Fatalf("default not persisted %+v", pref)
	}
	// A new reminder starts shared; the old one is untouched (PRI-05).
	var fresh api.Reminder
	o.call("POST", "/api/reminders", aTok, dateReq("Baru", "2026-11-02"), &fresh)
	if !fresh.Shared {
		t.Fatalf("new reminder not shared %+v", fresh)
	}
	var again api.Reminder
	o.call("GET", "/api/reminders/"+old.ID.String(), aTok, nil, &again)
	if again.Shared {
		t.Fatalf("old reminder reopened by default change %+v", again)
	}
}

// AC-09: the owner reads the history of changes to their reminder; it is
// recorded in the audit log in the same transaction.
func TestReminderHistoryForOwner(t *testing.T) {
	o := newOrg(t)
	kTok, _ := o.kadiv("mediatek", "km@himasif.test")
	aTok, _ := o.join(kTok, o.period.ID, "mediatek", "a@himasif.test", "A")

	var rem api.Reminder
	o.call("POST", "/api/reminders", aTok, dateReq("Rekap", "2026-11-05"), &rem)
	rid := rem.ID.String()
	o.call("PUT", "/api/reminders/"+rid, aTok, dateReq("Rekap mingguan", "2026-11-06"), nil)
	o.call("POST", "/api/reminders/"+rid+"/complete", aTok, api.ReminderCompleteRequest{Completed: true}, nil)

	var hist api.List[api.ReminderAuditEntry]
	if c := o.call("GET", "/api/reminders/"+rid+"/history", aTok, nil, &hist); c != 200 {
		t.Fatalf("history = %d", c)
	}
	var actions []string
	for _, e := range hist.Items {
		actions = append(actions, e.Action)
	}
	slices.Sort(actions)
	if !slices.Equal(actions, []string{"reminder.complete", "reminder.create", "reminder.update"}) {
		t.Fatalf("history actions %v", actions)
	}
	// Completing it set the completed flag.
	var done api.Reminder
	o.call("GET", "/api/reminders/"+rid, aTok, nil, &done)
	if !done.Completed || done.CompletedAt == nil {
		t.Fatalf("not completed %+v", done)
	}
	// Reopen it.
	o.call("POST", "/api/reminders/"+rid+"/complete", aTok, api.ReminderCompleteRequest{Completed: false}, &done)
	if done.Completed {
		t.Fatalf("still completed %+v", done)
	}
}

// PRI-08: a private reminder never leaks through the Space endpoints; those
// endpoints carry tasks and directories, never reminders.
func TestReminderDoesNotLeakThroughSpaces(t *testing.T) {
	o := newOrg(t)
	kTok, _ := o.kadiv("mediatek", "km@himasif.test")
	aTok, _ := o.join(kTok, o.period.ID, "mediatek", "a@himasif.test", "A")
	o.call("POST", "/api/reminders", aTok, dateReq("Privat", "2026-11-09"), nil)

	// The division directory and spaces still show no reminder data; there
	// is simply no field for it, and a member reading them learns nothing.
	var sps api.List[api.Space]
	if c := o.call("GET", "/api/periods/"+o.period.ID.String()+"/spaces", kTok, nil, &sps); c != 200 {
		t.Fatalf("spaces = %d", c)
	}
	var ms api.List[api.Member]
	if c := o.call("GET", o.scope(o.period.ID, "mediatek")+"/members", kTok, nil, &ms); c != 200 {
		t.Fatalf("members = %d", c)
	}
	// The kadiv has no reminder of A through any of their own listings.
	var kList api.List[api.Reminder]
	o.call("GET", "/api/reminders", kTok, nil, &kList)
	if len(kList.Items) != 0 {
		t.Fatalf("kadiv sees A's private reminder %+v", kList.Items)
	}
	_ = http.StatusOK
}
