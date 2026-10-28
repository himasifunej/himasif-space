package appui

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// RemindersAPI is account-scoped: neither a Space nor a period grants access.
type RemindersAPI interface {
	Reminders(context.Context) ([]api.Reminder, error)
	GetReminder(context.Context, id.ID) (api.Reminder, error)
	CreateReminder(context.Context, api.ReminderRequest) (api.Reminder, error)
	UpdateReminder(context.Context, id.ID, api.ReminderRequest) (api.Reminder, error)
	DeleteReminder(context.Context, id.ID) error
	CompleteReminder(context.Context, id.ID, api.ReminderCompleteRequest) (api.Reminder, error)
	SetReminderSharing(context.Context, id.ID, api.ReminderSharingRequest) (api.Reminder, error)
	ReminderRecipients(context.Context, id.ID) ([]api.ReminderShare, error)
	AddReminderRecipient(context.Context, id.ID, api.ReminderShareRequest) (api.Reminder, error)
	RemoveReminderRecipient(context.Context, id.ID, id.ID) error
	ReminderHistory(context.Context, id.ID) ([]api.ReminderAuditEntry, error)
	ReminderPreferences(context.Context) (api.ReminderPreferences, error)
	SetReminderPreferences(context.Context, api.ReminderPreferences) (api.ReminderPreferences, error)
}

var _ RemindersAPI = (*client.Client)(nil)

type reminderDraft struct {
	reminderID                      id.ID
	title, notes, date, clock, zone string
	source                          *api.Reminder
}

func newReminderDraft(r api.Reminder) *reminderDraft {
	date, clock := draftDate(r.DueOn, r.DueAt, r.TimeZone)
	return &reminderDraft{reminderID: r.ID, title: r.Title, notes: r.Notes, date: date, clock: clock, zone: r.TimeZone, source: &r}
}

func (d *reminderDraft) unchanged() bool {
	if d == nil || d.source == nil {
		return false
	}
	date, clock := draftDate(d.source.DueOn, d.source.DueAt, d.source.TimeZone)
	return d.title == d.source.Title && d.notes == d.source.Notes && d.date == date && d.clock == clock && d.zone == d.source.TimeZone
}

type remindersUIState struct {
	seq, detailSeq, candidatesSeq       uint64
	loaded, loading, busy               bool
	items                               []api.Reminder
	err, notice, filter                 string
	draft                               *reminderDraft
	selected                            id.ID
	prefs                               api.ReminderPreferences
	prefsLoaded                         bool
	prefsErr                            string
	recipients                          []api.ReminderShare
	recipientsLoaded, detailsLoading    bool
	detailsErr                          string
	history                             []api.ReminderAuditEntry
	candidates                          []api.Member
	candidatesLoading, candidatesLoaded bool
	candidatesErr, recipientFilter      string
	recipientOpen, deleteConfirm        bool
	candidatesPeriod                    id.ID
	candidatesAccess                    uint64
}

func (a *App) resetReminders() {
	a.reminders = remindersUIState{seq: a.reminders.seq + 1}
}

func (a *App) reminderService() (RemindersAPI, bool) {
	if c, ok := a.api.(*client.Client); ok {
		return c.AccountSession(), true
	}
	s, ok := a.api.(RemindersAPI)
	return s, ok
}

func (a *App) ensureReminders() {
	// Directory choices may change with period/access; the reminders themselves
	// remain attached to the account, including explicit grants from old periods.
	if a.reminders.candidatesPeriod != a.periodID || a.reminders.candidatesAccess != a.access.seq {
		a.reminders.candidatesSeq++
		a.reminders.candidatesPeriod = a.periodID
		a.reminders.candidatesAccess = a.access.seq
		a.reminders.candidates = nil
		a.reminders.candidatesLoaded, a.reminders.candidatesLoading = false, false
		a.reminders.candidatesErr = ""
		a.reminders.recipientOpen = false
	}
	if !a.isAdmin() && !a.reminders.loaded && !a.reminders.loading && a.reminders.err == "" {
		a.loadReminders()
	}
}

func (a *App) loadReminders() {
	service, ok := a.reminderService()
	if !ok {
		a.reminders.err = "Layanan reminder pribadi belum tersedia."
		return
	}
	if a.isAdmin() || a.reminders.loading || a.reminders.busy || a.screen != screenShell {
		return
	}
	gen, seq := a.gen, a.reminders.seq
	a.reminders.loading, a.reminders.err = true, ""
	a.async(func() func() {
		items, err := service.Reminders(context.Background())
		prefs, prefsErr := service.ReminderPreferences(context.Background())
		return func() {
			if gen != a.gen || seq != a.reminders.seq {
				return
			}
			a.reminders.loading = false
			if err != nil {
				a.reminderFailure(err)
				return
			}
			if prefsErr != nil && client.IsUnauthenticated(prefsErr) {
				a.failed(prefsErr)
				return
			}
			a.conn = connOnline
			a.reminders.items, a.reminders.loaded = items, true
			a.reminders.prefsLoaded = prefsErr == nil
			a.reminders.prefsErr = ""
			if prefsErr == nil {
				a.reminders.prefs = prefs
			} else {
				a.reminders.prefsErr = a.failed(prefsErr)
			}
			if a.reminders.selected != id.Nil && a.reminderByID(a.reminders.selected) == nil {
				a.clearReminderSelection()
			} else if a.reminders.selected != id.Nil {
				a.loadReminderDetails()
			}
		}
	})
}

func (a *App) reminderByID(rid id.ID) *api.Reminder {
	for i := range a.reminders.items {
		if a.reminders.items[i].ID == rid {
			return &a.reminders.items[i]
		}
	}
	return nil
}

func (a *App) clearReminderSelection() {
	a.reminders.detailSeq++
	a.reminders.selected, a.reminders.draft = id.Nil, nil
	a.reminders.recipients, a.reminders.history = nil, nil
	a.reminders.detailsLoading, a.reminders.recipientsLoaded = false, false
	a.reminders.detailsErr = ""
	a.reminders.deleteConfirm, a.reminders.recipientOpen = false, false
}

func (a *App) closeReminder() {
	if !a.reminders.busy {
		a.clearReminderSelection()
	}
}

func (a *App) beginReminder() {
	if a.reminderWriteBlocked() || !a.reminders.prefsLoaded {
		return
	}
	a.clearReminderSelection()
	a.chrome.appearance = false
	a.reminders.err, a.reminders.notice = "", ""
	a.reminders.draft = &reminderDraft{zone: timeutil.DefaultZoneName}
}

func (a *App) openReminder(rid id.ID) {
	if a.reminders.busy {
		return
	}
	r := a.reminderByID(rid)
	if r == nil {
		return
	}
	a.clearReminderSelection()
	a.chrome.appearance = false
	a.reminders.selected = rid
	if r.Relation == api.ReminderOwner && r.OwnerID == a.user.ID {
		a.reminders.draft = newReminderDraft(*r)
	}
	a.loadReminderDetails()
}

func (a *App) loadReminderDetails() {
	service, ok := a.reminderService()
	rid := a.reminders.selected
	if !ok || rid == id.Nil || a.reminders.busy || a.reminders.detailsLoading {
		return
	}
	a.reminders.detailSeq++
	gen, seq, detail := a.gen, a.reminders.seq, a.reminders.detailSeq
	a.reminders.detailsLoading, a.reminders.recipientsLoaded = true, false
	a.reminders.detailsErr = ""
	a.async(func() func() {
		r, err := service.GetReminder(context.Background(), rid)
		var recipients []api.ReminderShare
		var history []api.ReminderAuditEntry
		if err == nil && r.Relation == api.ReminderOwner {
			recipients, err = service.ReminderRecipients(context.Background(), rid)
			if err == nil {
				history, err = service.ReminderHistory(context.Background(), rid)
			}
		}
		return func() {
			if gen != a.gen || seq != a.reminders.seq || detail != a.reminders.detailSeq || rid != a.reminders.selected {
				return
			}
			a.reminders.detailsLoading = false
			if err != nil {
				if client.IsUnauthenticated(err) {
					a.failed(err)
					return
				}
				var e *client.Error
				if errors.As(err, &e) && (e.Status == http.StatusForbidden || e.Status == http.StatusNotFound) {
					a.forgetReminder(rid)
					a.clearReminderSelection()
					a.reminders.err = "Reminder tidak lagi dapat diakses. Muat ulang daftar."
					return
				}
				a.reminders.detailsErr = a.failed(err)
				return
			}
			a.conn = connOnline
			a.replaceReminder(r)
			a.reminders.recipients, a.reminders.history = recipients, history
			a.reminders.recipientsLoaded = r.Relation == api.ReminderOwner
			if r.Relation != api.ReminderOwner || r.OwnerID != a.user.ID {
				a.reminders.draft = nil
			} else if a.reminders.draft == nil || a.reminders.draft.unchanged() {
				// Opening starts from the fresh detail, not a possibly stale list.
				// Subsequent refreshes retain deliberate unsaved edits.
				a.reminders.draft = newReminderDraft(r)
			}
		}
	})
}

func (a *App) reminderWriteBlocked() bool {
	return a.screen != screenShell || a.isAdmin() || !a.reminders.loaded || a.reminders.loading || a.reminders.busy || a.reminders.detailsLoading || a.reminders.detailsErr != "" || a.conn != connOnline
}

func (a *App) ownsSelectedReminder() bool {
	r := a.reminderByID(a.reminders.selected)
	return r != nil && r.Relation == api.ReminderOwner && r.OwnerID == a.user.ID
}

func (a *App) reminderFailure(err error) {
	if client.IsUnauthenticated(err) {
		a.failed(err)
		return
	}
	var e *client.Error
	if errors.As(err, &e) && e.Status == http.StatusForbidden {
		a.clearReminderSelection()
		a.reminders.items, a.reminders.loaded = nil, false
	}
	a.reminders.err = a.failed(err)
}

func (a *App) replaceReminder(r api.Reminder) {
	for i := range a.reminders.items {
		if a.reminders.items[i].ID == r.ID {
			a.reminders.items[i] = r
			return
		}
	}
	a.reminders.items = append(a.reminders.items, r)
}

func (a *App) forgetReminder(rid id.ID) {
	a.reminders.items = slices.DeleteFunc(a.reminders.items, func(r api.Reminder) bool { return r.ID == rid })
}

// mutateReminder freezes the bearer before queuing work, and publishes only
// acknowledged changes. A session reset invalidates the complete callback.
func (a *App) mutateReminder(work func(RemindersAPI) (api.Reminder, error), after func(api.Reminder)) {
	service, ok := a.reminderService()
	if !ok || a.reminderWriteBlocked() {
		return
	}
	gen, seq := a.gen, a.reminders.seq
	a.reminders.busy, a.reminders.err = true, ""
	a.reminders.notice = ""
	a.async(func() func() {
		r, err := work(service)
		return func() {
			if gen != a.gen || seq != a.reminders.seq {
				return
			}
			a.reminders.busy = false
			if err != nil {
				a.reminderFailure(err)
				return
			}
			a.conn = connOnline
			after(r)
			a.reminders.notice = "Perubahan reminder tersimpan di server."
			a.loadReminders()
		}
	})
}

func (a *App) saveReminder() {
	d := a.reminders.draft
	if d == nil || (d.reminderID != id.Nil && !a.ownsSelectedReminder()) || (d.reminderID == id.Nil && !a.reminders.prefsLoaded) {
		return
	}
	zone := strings.TrimSpace(d.zone)
	if _, err := timeutil.LoadZone(zone); err != nil {
		a.reminders.err = "Zona waktu tidak valid. Gunakan nama seperti Asia/Jakarta."
		return
	}
	date, at, err := parseTaskDate(d.date, d.clock, zone, true)
	if err != nil {
		a.reminders.err = "Tanggal reminder: " + err.Error()
		return
	}
	if strings.TrimSpace(d.title) == "" {
		a.reminders.err = "Judul reminder wajib diisi."
		return
	}
	req := api.ReminderRequest{Title: strings.TrimSpace(d.title), Notes: d.notes, DueKind: api.DueDate, DueOn: date, DueAt: at, TimeZone: zone}
	if at != nil {
		req.DueKind = api.DueInstant
	}
	// The editor shows minutes; editing text must not truncate an existing
	// instant's seconds or precision, which may have been set by another client.
	if old := d.source; old != nil && old.DueAt != nil {
		oldDate, oldClock := draftDate(old.DueOn, old.DueAt, old.TimeZone)
		if d.date == oldDate && d.clock == oldClock && zone == old.TimeZone {
			req.DueKind, req.DueOn, req.DueAt = api.DueInstant, nil, old.DueAt
		}
	}
	rid := d.reminderID
	a.mutateReminder(func(s RemindersAPI) (api.Reminder, error) {
		if rid == id.Nil {
			return s.CreateReminder(context.Background(), req)
		}
		return s.UpdateReminder(context.Background(), rid, req)
	}, func(r api.Reminder) {
		a.replaceReminder(r)
		a.reminders.selected, d.reminderID = r.ID, r.ID
		d.source = &r
	})
}

func (a *App) completeReminder() {
	if !a.ownsSelectedReminder() {
		return
	}
	r := *a.reminderByID(a.reminders.selected)
	a.mutateReminder(func(s RemindersAPI) (api.Reminder, error) {
		return s.CompleteReminder(context.Background(), r.ID, api.ReminderCompleteRequest{Completed: !r.Completed})
	}, a.replaceReminder)
}

func (a *App) setReminderSharing(shared bool) {
	if !a.ownsSelectedReminder() {
		return
	}
	rid := a.reminders.selected
	a.mutateReminder(func(s RemindersAPI) (api.Reminder, error) {
		return s.SetReminderSharing(context.Background(), rid, api.ReminderSharingRequest{Shared: shared})
	}, a.replaceReminder)
}

func (a *App) deleteReminder() {
	if !a.ownsSelectedReminder() || !a.reminders.deleteConfirm {
		return
	}
	rid := a.reminders.selected
	a.mutateReminder(func(s RemindersAPI) (api.Reminder, error) {
		return api.Reminder{}, s.DeleteReminder(context.Background(), rid)
	}, func(api.Reminder) {
		a.forgetReminder(rid)
		a.clearReminderSelection()
	})
}

func (a *App) setReminderDefaultShare(shared bool) {
	service, ok := a.reminderService()
	if !ok || !a.reminders.prefsLoaded || a.reminderWriteBlocked() {
		return
	}
	gen, seq := a.gen, a.reminders.seq
	pref := api.ReminderPreferences{DefaultShare: shared}
	a.reminders.busy, a.reminders.err, a.reminders.notice = true, "", ""
	a.async(func() func() {
		p, err := service.SetReminderPreferences(context.Background(), pref)
		return func() {
			if gen != a.gen || seq != a.reminders.seq {
				return
			}
			a.reminders.busy = false
			if err != nil {
				a.reminderFailure(err)
				return
			}
			a.conn = connOnline
			a.reminders.prefs = p
			a.reminders.notice = "Default privasi tersimpan untuk reminder baru. Reminder lama tidak berubah."
		}
	})
}

func (a *App) addReminderRecipient(recipient id.ID) {
	if !a.ownsSelectedReminder() || !a.reminders.recipientsLoaded || !a.reminders.candidatesLoaded || a.reminders.candidatesPeriod != a.periodID || a.reminders.candidatesAccess != a.access.seq || recipient == a.user.ID {
		return
	}
	if !slices.ContainsFunc(a.reminders.candidates, func(m api.Member) bool { return m.UserID == recipient }) || slices.ContainsFunc(a.reminders.recipients, func(r api.ReminderShare) bool { return r.RecipientID == recipient }) {
		return
	}
	rid := a.reminders.selected
	a.mutateReminder(func(s RemindersAPI) (api.Reminder, error) {
		return s.AddReminderRecipient(context.Background(), rid, api.ReminderShareRequest{RecipientID: recipient})
	}, func(r api.Reminder) {
		a.replaceReminder(r)
		a.reminders.recipientOpen = false
	})
}

func (a *App) removeReminderRecipient(recipient id.ID) {
	if !a.ownsSelectedReminder() || !a.reminders.recipientsLoaded || !slices.ContainsFunc(a.reminders.recipients, func(r api.ReminderShare) bool { return r.RecipientID == recipient }) {
		return
	}
	rid := a.reminders.selected
	a.mutateReminder(func(s RemindersAPI) (api.Reminder, error) {
		return api.Reminder{}, s.RemoveReminderRecipient(context.Background(), rid, recipient)
	}, func(api.Reminder) {})
}

func (a *App) loadReminderCandidates() {
	service, ok := a.workspaceService()
	if !ok {
		a.reminders.candidatesErr = "Direktori anggota belum tersedia."
		return
	}
	if a.reminders.candidatesLoading || a.access.loading || a.periodID == id.Nil || !a.ownsSelectedReminder() {
		return
	}
	divisions := map[id.ID]bool{}
	for _, s := range a.access.spaces {
		for _, access := range s.Roles {
			if slices.Contains(access.Capabilities, api.CapMembersRead) {
				divisions[access.DivisionID] = true
			}
		}
	}
	a.reminders.candidatesSeq++
	gen, seq, candidateSeq, accessSeq, period, self := a.gen, a.reminders.seq, a.reminders.candidatesSeq, a.access.seq, a.periodID, a.user.ID
	a.reminders.candidatesPeriod, a.reminders.candidatesAccess = period, accessSeq
	a.reminders.candidatesLoading, a.reminders.candidatesErr = true, ""
	a.async(func() func() {
		var members []api.Member
		var err error
		seen := map[id.ID]bool{self: true, id.Nil: true}
		for division := range divisions {
			var found []api.Member
			found, err = service.Members(context.Background(), period, division)
			if err != nil {
				break
			}
			for _, m := range found {
				if !seen[m.UserID] {
					seen[m.UserID] = true
					members = append(members, m)
				}
			}
		}
		slices.SortFunc(members, func(a, b api.Member) int {
			return strings.Compare(a.DisplayName+a.UserID.String(), b.DisplayName+b.UserID.String())
		})
		return func() {
			if gen != a.gen || seq != a.reminders.seq || candidateSeq != a.reminders.candidatesSeq || period != a.periodID || accessSeq != a.access.seq {
				return
			}
			a.reminders.candidatesLoading = false
			if err != nil {
				if client.IsUnauthenticated(err) {
					a.failed(err)
					return
				}
				a.reminders.candidates, a.reminders.candidatesLoaded = nil, false
				a.reminders.candidatesErr = a.failed(err)
				return
			}
			a.conn = connOnline
			a.reminders.candidates, a.reminders.candidatesLoaded = members, true
		}
	})
}
