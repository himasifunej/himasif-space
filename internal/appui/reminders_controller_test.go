package appui

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// This fixture tests client intent and publication timing. Authorization and
// persistence are exercised separately by the real server/Postgres tests.
type reminderFixture struct {
	*fakeAPI
	RemindersAPI
	WorkspaceAPI
	reminderMu                        sync.Mutex
	items                             []api.Reminder
	prefs                             api.ReminderPreferences
	recipients                        map[id.ID][]api.ReminderShare
	listErr, writeErr                 error
	listCalls, writeCalls             int
	createRequests, updateRequests    []api.ReminderRequest
	listGate, detailGate, writeGate   chan struct{}
	listStarted, detailStarted, wrote chan struct{}
	memberCalls                       []id.ID
	members                           map[id.ID][]api.Member
}

func signalReminderCall(ch chan struct{}) {
	if ch != nil {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (f *reminderFixture) Reminders(context.Context) ([]api.Reminder, error) {
	f.reminderMu.Lock()
	f.listCalls++
	gate, started := f.listGate, f.listStarted
	f.reminderMu.Unlock()
	signalReminderCall(started)
	if gate != nil {
		<-gate
	}
	f.reminderMu.Lock()
	defer f.reminderMu.Unlock()
	return slices.Clone(f.items), f.listErr
}

func (f *reminderFixture) GetReminder(_ context.Context, rid id.ID) (api.Reminder, error) {
	f.reminderMu.Lock()
	gate, started := f.detailGate, f.detailStarted
	f.reminderMu.Unlock()
	signalReminderCall(started)
	if gate != nil {
		<-gate
	}
	f.reminderMu.Lock()
	defer f.reminderMu.Unlock()
	for _, r := range f.items {
		if r.ID == rid {
			return r, nil
		}
	}
	return api.Reminder{}, &client.Error{Status: 404, Code: api.CodeNotFound}
}

func (f *reminderFixture) ReminderPreferences(context.Context) (api.ReminderPreferences, error) {
	f.reminderMu.Lock()
	defer f.reminderMu.Unlock()
	return f.prefs, nil
}

func (f *reminderFixture) waitWrite() error {
	f.reminderMu.Lock()
	f.writeCalls++
	gate, wrote, err := f.writeGate, f.wrote, f.writeErr
	f.reminderMu.Unlock()
	signalReminderCall(wrote)
	if gate != nil {
		<-gate
	}
	return err
}

func (f *reminderFixture) SetReminderPreferences(_ context.Context, p api.ReminderPreferences) (api.ReminderPreferences, error) {
	if err := f.waitWrite(); err != nil {
		return api.ReminderPreferences{}, err
	}
	f.reminderMu.Lock()
	defer f.reminderMu.Unlock()
	f.prefs = p
	return p, nil
}

func (f *reminderFixture) CreateReminder(_ context.Context, req api.ReminderRequest) (api.Reminder, error) {
	if err := f.waitWrite(); err != nil {
		return api.Reminder{}, err
	}
	f.reminderMu.Lock()
	defer f.reminderMu.Unlock()
	f.createRequests = append(f.createRequests, req)
	r := api.Reminder{ID: id.New(), OwnerID: f.user.ID, Title: req.Title, Notes: req.Notes, DueKind: req.DueKind, DueOn: req.DueOn, DueAt: req.DueAt, TimeZone: req.TimeZone, Shared: f.prefs.DefaultShare, Relation: api.ReminderOwner}
	f.items = append(f.items, r)
	return r, nil
}

func (f *reminderFixture) UpdateReminder(_ context.Context, rid id.ID, req api.ReminderRequest) (api.Reminder, error) {
	if err := f.waitWrite(); err != nil {
		return api.Reminder{}, err
	}
	f.reminderMu.Lock()
	defer f.reminderMu.Unlock()
	f.updateRequests = append(f.updateRequests, req)
	for i := range f.items {
		if f.items[i].ID == rid {
			r := &f.items[i]
			r.Title, r.Notes, r.DueKind, r.DueOn, r.DueAt, r.TimeZone = req.Title, req.Notes, req.DueKind, req.DueOn, req.DueAt, req.TimeZone
			return *r, nil
		}
	}
	return api.Reminder{}, &client.Error{Status: 404, Code: api.CodeNotFound}
}

func (f *reminderFixture) CompleteReminder(_ context.Context, rid id.ID, req api.ReminderCompleteRequest) (api.Reminder, error) {
	if err := f.waitWrite(); err != nil {
		return api.Reminder{}, err
	}
	f.reminderMu.Lock()
	defer f.reminderMu.Unlock()
	for i := range f.items {
		if f.items[i].ID == rid {
			f.items[i].Completed = req.Completed
			return f.items[i], nil
		}
	}
	return api.Reminder{}, &client.Error{Status: 404, Code: api.CodeNotFound}
}

func (f *reminderFixture) SetReminderSharing(_ context.Context, rid id.ID, req api.ReminderSharingRequest) (api.Reminder, error) {
	if err := f.waitWrite(); err != nil {
		return api.Reminder{}, err
	}
	f.reminderMu.Lock()
	defer f.reminderMu.Unlock()
	for i := range f.items {
		if f.items[i].ID == rid {
			f.items[i].Shared = req.Shared
			return f.items[i], nil
		}
	}
	return api.Reminder{}, &client.Error{Status: 404, Code: api.CodeNotFound}
}

func (f *reminderFixture) DeleteReminder(_ context.Context, rid id.ID) error {
	if err := f.waitWrite(); err != nil {
		return err
	}
	f.reminderMu.Lock()
	defer f.reminderMu.Unlock()
	f.items = slices.DeleteFunc(f.items, func(r api.Reminder) bool { return r.ID == rid })
	return nil
}

func (f *reminderFixture) ReminderRecipients(_ context.Context, rid id.ID) ([]api.ReminderShare, error) {
	f.reminderMu.Lock()
	defer f.reminderMu.Unlock()
	return slices.Clone(f.recipients[rid]), nil
}

func (f *reminderFixture) RemoveReminderRecipient(_ context.Context, rid, recipient id.ID) error {
	if err := f.waitWrite(); err != nil {
		return err
	}
	f.reminderMu.Lock()
	defer f.reminderMu.Unlock()
	f.recipients[rid] = slices.DeleteFunc(f.recipients[rid], func(r api.ReminderShare) bool { return r.RecipientID == recipient })
	return nil
}

func (f *reminderFixture) AddReminderRecipient(_ context.Context, rid id.ID, req api.ReminderShareRequest) (api.Reminder, error) {
	if err := f.waitWrite(); err != nil {
		return api.Reminder{}, err
	}
	f.reminderMu.Lock()
	defer f.reminderMu.Unlock()
	f.recipients[rid] = append(f.recipients[rid], api.ReminderShare{RecipientID: req.RecipientID, DisplayName: "Penerima"})
	for _, r := range f.items {
		if r.ID == rid {
			return r, nil
		}
	}
	return api.Reminder{}, &client.Error{Status: 404, Code: api.CodeNotFound}
}

func (f *reminderFixture) ReminderHistory(context.Context, id.ID) ([]api.ReminderAuditEntry, error) {
	return []api.ReminderAuditEntry{{Action: "created", OccurredAt: time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)}}, nil
}

func (f *reminderFixture) Spaces(context.Context, id.ID) ([]api.Space, error) {
	return nil, nil
}

func (f *reminderFixture) Members(_ context.Context, _ id.ID, division id.ID) ([]api.Member, error) {
	f.reminderMu.Lock()
	defer f.reminderMu.Unlock()
	f.memberCalls = append(f.memberCalls, division)
	return slices.Clone(f.members[division]), nil
}

// Drain without rendering, so these controller tests do not depend on a page's
// animation, polling or lazy-loading behavior.
func settleReminders(t *testing.T, h *harness) {
	t.Helper()
	for h.app.inflight > 0 {
		select {
		case apply := <-h.queue:
			apply()
		case <-time.After(5 * time.Second):
			t.Fatalf("%d reminder calls did not settle", h.app.inflight)
		}
	}
}

func reminderHarness(t *testing.T) (*harness, *reminderFixture) {
	f := &reminderFixture{fakeAPI: newFake(api.AccountPerson), recipients: map[id.ID][]api.ReminderShare{}, members: map[id.ID][]api.Member{}}
	h := newHarness(t, f.fakeAPI, "")
	h.app.api, h.app.user, h.app.screen, h.app.conn = f, f.user, screenShell, connOnline
	h.app.periodID, h.app.periods = f.periods[1].ID, f.periods
	date := timeutil.Date{Year: 2026, Month: 10, Day: 30}
	f.items = []api.Reminder{{ID: id.New(), OwnerID: f.user.ID, Title: "Ingat dokumen pribadi", DueKind: api.DueDate, DueOn: &date, TimeZone: "Asia/Jakarta", Relation: api.ReminderOwner}}
	h.app.ensureReminders()
	settleReminders(t, h)
	return h, f
}

func TestReminderPreferenceOnlyChangesFutureReminderIntent(t *testing.T) {
	h, f := reminderHarness(t)
	old := h.app.reminders.items[0].ID
	if h.app.reminders.prefs.DefaultShare || h.app.reminders.items[0].Shared {
		t.Fatal("private reminder or preference did not start private")
	}
	h.app.setReminderDefaultShare(true)
	settleReminders(t, h)
	if !h.app.reminders.prefs.DefaultShare || h.app.reminderByID(old).Shared {
		t.Fatal("future preference changed an existing reminder")
	}
	h.app.beginReminder()
	d := h.app.reminders.draft
	d.title, d.date = "Reminder baru", "2026-11-02"
	h.app.saveReminder()
	settleReminders(t, h)
	if len(f.createRequests) != 1 || f.createRequests[0].DueKind != api.DueDate || f.createRequests[0].DueOn == nil || f.createRequests[0].DueAt != nil {
		t.Fatal("date-only reminder was converted into a fabricated time")
	}
	if h.app.reminderByID(old).Shared || !h.app.reminderByID(h.app.reminders.selected).Shared {
		t.Fatal("new preference was applied to the wrong reminder")
	}
}

func TestReminderSaveWaitsForAcknowledgementAndKeepsFailedDraft(t *testing.T) {
	h, f := reminderHarness(t)
	h.app.beginReminder()
	draft := h.app.reminders.draft
	draft.title, draft.date, draft.clock = "Belajar mandiri", "2026-11-02", "16:30"
	f.writeGate, f.wrote = make(chan struct{}), make(chan struct{}, 1)
	f.writeErr = &client.Error{Status: 422, Code: api.CodeBadRequest, Message: "Tidak dapat menyimpan"}
	h.app.saveReminder()
	select {
	case <-f.wrote:
	case <-time.After(time.Second):
		t.Fatal("save was not dispatched")
	}
	if !h.app.reminders.busy || h.app.reminders.notice != "" || len(h.app.reminders.items) != 1 {
		t.Fatal("pending write pretended to save a reminder")
	}
	close(f.writeGate)
	settleReminders(t, h)
	if h.app.reminders.busy || h.app.reminders.draft != draft || h.app.reminders.err == "" || h.app.reminders.notice != "" {
		t.Fatal("failure lost the draft or reported success")
	}
	f.writeErr, f.writeGate = nil, nil
	h.app.saveReminder()
	settleReminders(t, h)
	if len(f.createRequests) != 1 {
		t.Fatal("retry did not issue a single create")
	}
	req := f.createRequests[0]
	want := time.Date(2026, 11, 2, 9, 30, 0, 0, time.UTC)
	if req.DueKind != api.DueInstant || req.DueOn != nil || req.DueAt == nil || !req.DueAt.Equal(want) || req.TimeZone != "Asia/Jakarta" {
		t.Fatalf("explicit local time was not preserved: %+v", req)
	}
}

func TestReminderRecipientCannotDispatchOwnerWrites(t *testing.T) {
	h, f := reminderHarness(t)
	rid := f.items[0].ID
	f.items[0].OwnerID, f.items[0].Relation = id.New(), api.ReminderRecipient
	h.app.loadReminders()
	settleReminders(t, h)
	h.app.openReminder(rid)
	settleReminders(t, h)
	if h.app.reminders.draft != nil || h.app.reminders.recipientsLoaded || len(h.app.reminders.history) != 0 {
		t.Fatal("recipient obtained owner-only editing or sharing details")
	}
	// Even an obsolete or injected draft cannot bypass the controller check.
	h.app.reminders.draft = &reminderDraft{reminderID: rid, title: "Changed", date: "2026-11-02", zone: "Asia/Jakarta"}
	h.app.reminders.deleteConfirm = true
	h.app.saveReminder()
	h.app.completeReminder()
	h.app.setReminderSharing(true)
	h.app.deleteReminder()
	h.app.addReminderRecipient(id.New())
	h.app.removeReminderRecipient(id.New())
	settleReminders(t, h)
	if f.writeCalls != 0 || h.app.reminderByID(rid).Title != "Ingat dokumen pribadi" {
		t.Fatal("read-only recipient dispatched an owner write")
	}
}

func TestReminderEditingTextPreservesTheOriginalInstant(t *testing.T) {
	h, f := reminderHarness(t)
	rid := f.items[0].ID
	at := time.Date(2026, 11, 2, 9, 30, 42, 123000000, time.UTC)
	f.items[0].DueKind, f.items[0].DueOn, f.items[0].DueAt = api.DueInstant, nil, &at
	h.app.loadReminders()
	settleReminders(t, h)
	h.app.openReminder(rid)
	settleReminders(t, h)
	h.app.reminders.draft.title = "Judul diperjelas"
	h.app.saveReminder()
	settleReminders(t, h)
	if len(f.updateRequests) != 1 || f.updateRequests[0].DueAt == nil || !f.updateRequests[0].DueAt.Equal(at) || f.updateRequests[0].DueOn != nil {
		t.Fatal("editing the title rounded the stored deadline's seconds or precision")
	}
	if h.app.reminderByID(rid).Title != "Judul diperjelas" || h.app.reminderByID(rid).ID != rid {
		t.Fatal("acknowledged update did not preserve the reminder's identity")
	}
	h.app.reminders.draft.clock = "16:31"
	h.app.saveReminder()
	settleReminders(t, h)
	want := time.Date(2026, 11, 2, 9, 31, 0, 0, time.UTC)
	if len(f.updateRequests) != 2 || f.updateRequests[1].DueAt == nil || !f.updateRequests[1].DueAt.Equal(want) {
		t.Fatal("an explicit clock edit kept the old instant instead of the new local time")
	}
	h.app.reminders.draft.date, h.app.reminders.draft.clock = "2026-11-03", ""
	h.app.saveReminder()
	settleReminders(t, h)
	if len(f.updateRequests) != 3 || f.updateRequests[2].DueAt != nil || f.updateRequests[2].DueKind != api.DueDate || f.updateRequests[2].DueOn == nil || f.updateRequests[2].DueOn.String() != "2026-11-03" {
		t.Fatal("clearing the clock did not deliberately convert the reminder to a date-only deadline")
	}
}

func TestReminderDetailsRefreshesCleanDraftButPreservesUnsavedEdits(t *testing.T) {
	h, f := reminderHarness(t)
	rid := f.items[0].ID
	freshDate := timeutil.Date{Year: 2026, Month: 11, Day: 2}
	// The list is an older snapshot. The detail response must become the
	// editor's baseline before an unrelated edit can overwrite fresh fields.
	f.items[0].Title, f.items[0].Notes, f.items[0].DueOn = "Judul terbaru", "Catatan terbaru", &freshDate
	h.app.openReminder(rid)
	settleReminders(t, h)
	draft := h.app.reminders.draft
	if draft == nil || draft.title != "Judul terbaru" || draft.notes != "Catatan terbaru" || draft.date != "2026-11-02" {
		t.Fatal("detail GET refreshed the list but left a stale editor baseline")
	}

	draft.title = "Perubahan lokal belum disimpan"
	newerDate := timeutil.Date{Year: 2026, Month: 11, Day: 3}
	f.items[0].Title, f.items[0].Notes, f.items[0].DueOn = "Judul dari perangkat lain", "Catatan dari perangkat lain", &newerDate
	h.app.loadReminderDetails()
	settleReminders(t, h)
	if h.app.reminderByID(rid).Title != "Judul dari perangkat lain" || h.app.reminderByID(rid).DueOn.String() != "2026-11-03" {
		t.Fatal("detail refresh did not publish the fresh canonical reminder")
	}
	if h.app.reminders.draft != draft || draft.title != "Perubahan lokal belum disimpan" || draft.notes != "Catatan terbaru" || draft.date != "2026-11-02" {
		t.Fatal("background detail refresh replaced actual unsaved edits")
	}

	// Closing discards the local edit. A subsequent clean editor may follow
	// fresh server values again, rather than becoming permanently detached.
	h.app.closeReminder()
	h.app.openReminder(rid)
	settleReminders(t, h)
	if h.app.reminders.draft.title != "Judul dari perangkat lain" {
		t.Fatal("reopened editor did not adopt the current server baseline")
	}
	f.items[0].Title = "Judul terkini"
	h.app.loadReminderDetails()
	settleReminders(t, h)
	if h.app.reminders.draft.title != "Judul terkini" || h.app.reminders.draft.notes != "Catatan dari perangkat lain" || h.app.reminders.draft.date != "2026-11-03" {
		t.Fatal("a clean detail refresh stopped following server changes")
	}
}

func TestReminderOwnerCanRevokeDormantGrantAndCompleteOrDelete(t *testing.T) {
	h, f := reminderHarness(t)
	rid, recipient := f.items[0].ID, id.New()
	// Normal GET intentionally omits recipients when sharing is off. The
	// dedicated owner endpoint must still supply the dormant grant for revoke.
	f.recipients[rid] = []api.ReminderShare{{RecipientID: recipient, DisplayName: "Rekan"}}
	h.app.openReminder(rid)
	settleReminders(t, h)
	if h.app.reminderByID(rid).Shared || !h.app.reminders.recipientsLoaded || len(h.app.reminders.recipients) != 1 {
		t.Fatal("sharing off hid the owner's dormant recipient grants")
	}
	h.app.removeReminderRecipient(recipient)
	settleReminders(t, h)
	if len(h.app.reminders.recipients) != 0 || h.app.reminderByID(rid).Shared {
		t.Fatal("revoke failed or enabled sharing implicitly")
	}
	h.app.completeReminder()
	settleReminders(t, h)
	if !h.app.reminderByID(rid).Completed {
		t.Fatal("completion acknowledgement was not shown")
	}
	h.app.deleteReminder()
	settleReminders(t, h)
	if h.app.reminderByID(rid) == nil {
		t.Fatal("delete bypassed its explicit confirmation")
	}
	h.app.reminders.deleteConfirm = true
	h.app.deleteReminder()
	settleReminders(t, h)
	if h.app.reminderByID(rid) != nil || h.app.reminders.selected != id.Nil || h.app.reminders.draft != nil {
		t.Fatal("acknowledged delete retained the reminder or editor")
	}
}

func TestReminderDataSurvivesPeriodChangeButRecipientDirectoryDoesNot(t *testing.T) {
	h, f := reminderHarness(t)
	rid := f.items[0].ID
	h.app.openReminder(rid)
	settleReminders(t, h)
	h.app.reminders.candidatesLoaded = true
	h.app.reminders.candidates = []api.Member{{UserID: id.New(), DisplayName: "Old period"}}
	h.app.changePeriod(f.periods[0].ID)
	settleReminders(t, h)
	h.app.ensureReminders()
	if !h.app.reminders.loaded || h.app.reminderByID(rid) == nil || h.app.reminders.selected != rid || h.app.reminders.draft == nil {
		t.Fatal("period change discarded an account-owned reminder")
	}
	if h.app.reminders.candidatesLoaded || len(h.app.reminders.candidates) != 0 {
		t.Fatal("old period's candidate directory survived the switch")
	}
}

func TestReminderRepliesCannotSurviveAccountReset(t *testing.T) {
	for _, request := range []string{"list", "detail", "write"} {
		t.Run(request, func(t *testing.T) {
			h, f := reminderHarness(t)
			rid := f.items[0].ID
			gate, started := make(chan struct{}), make(chan struct{}, 1)
			switch request {
			case "list":
				f.listGate, f.listStarted = gate, started
				h.app.loadReminders()
			case "detail":
				f.detailGate, f.detailStarted = gate, started
				h.app.openReminder(rid)
			case "write":
				h.app.openReminder(rid)
				settleReminders(t, h)
				f.writeGate, f.wrote = gate, started
				h.app.completeReminder()
			}
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("gated request was not dispatched")
			}
			h.app.toLogin("")
			close(gate)
			settleReminders(t, h)
			if h.app.screen != screenLogin || h.app.reminders.loaded || len(h.app.reminders.items) != 0 || h.app.reminders.selected != id.Nil || h.app.reminders.draft != nil || h.app.reminders.notice != "" || len(h.app.reminders.recipients) != 0 {
				t.Fatal("old account's reminder reply populated the new session")
			}
		})
	}
}

func TestReminderUnavailableIsNotAnEmptySuccessOrRenderRetryLoop(t *testing.T) {
	h, f := reminderHarness(t)
	f.listErr = errOffline
	h.app.resetReminders()
	h.app.ensureReminders()
	settleReminders(t, h)
	for range 3 {
		h.app.ensureReminders()
	}
	settleReminders(t, h)
	if h.app.reminders.loaded || h.app.reminders.err == "" || f.listCalls != 2 || h.app.conn != connOffline {
		t.Fatal("failed load looked empty, lost its offline state, or retried implicitly")
	}
	f.listErr = nil
	h.app.loadReminders()
	settleReminders(t, h)
	if !h.app.reminders.loaded || h.app.reminders.err != "" || h.app.conn != connOnline {
		t.Fatal("explicit retry did not restore the server snapshot")
	}
}

func TestReminderRecipientChoicesUseOnlyAuthorizedDirectories(t *testing.T) {
	h, f := reminderHarness(t)
	rid := f.items[0].ID
	allowed, denied, recipient := id.New(), id.New(), id.New()
	h.app.access.spaces = []api.Space{
		{ID: id.New(), Roles: []api.Access{{DivisionID: allowed, Capabilities: []string{api.CapMembersRead}}, {DivisionID: denied, Capabilities: []string{api.CapSpaceRead}}}},
		{ID: id.New(), Roles: []api.Access{{DivisionID: allowed, Capabilities: []string{api.CapMembersRead}}}},
	}
	f.members[allowed] = []api.Member{{UserID: f.user.ID, DisplayName: "Self"}, {UserID: recipient, DisplayName: "Rekan"}, {UserID: recipient, DisplayName: "Duplicate"}, {UserID: id.Nil, DisplayName: "Invalid"}}
	f.members[denied] = []api.Member{{UserID: id.New(), DisplayName: "Unauthorized"}}
	h.app.openReminder(rid)
	settleReminders(t, h)
	h.app.loadReminderCandidates()
	settleReminders(t, h)
	if len(f.memberCalls) != 1 || f.memberCalls[0] != allowed || !h.app.reminders.candidatesLoaded || len(h.app.reminders.candidates) != 1 || h.app.reminders.candidates[0].UserID != recipient {
		t.Fatal("recipient picker broadened access, duplicated directories, or included self")
	}
	h.app.addReminderRecipient(id.New())
	settleReminders(t, h)
	if f.writeCalls != 0 {
		t.Fatal("an account outside the authorized picker was granted access")
	}
	h.app.addReminderRecipient(recipient)
	settleReminders(t, h)
	if len(h.app.reminders.recipients) != 1 || h.app.reminderByID(rid).Shared {
		t.Fatal("explicit account grant failed or enabled sharing implicitly")
	}
}
