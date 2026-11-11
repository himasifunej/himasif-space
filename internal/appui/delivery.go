package appui

import (
	"context"
	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"slices"
	"time"
)

// NotificationSink reports acceptance by the OS API, never a claim that the
// human saw the toast. It runs off the UI thread. Notification identity is
// durable on the device before invoking a platform channel.
type NotificationSink interface {
	Notify(id.ID, api.Delivery) error
}
type NotificationBatchSink interface {
	NotifyBatch(id.ID, []api.Delivery) error
}
type DeliveryAPI interface {
	Deliveries(context.Context) ([]api.Delivery, error)
	RespondDelivery(context.Context, id.ID, api.DeliveryResponse) error
	SendPing(context.Context, id.ID, api.PingRequest) error
}
type deliveriesState struct {
	items           []api.Delivery
	loading         bool
	last            time.Time
	err             string
	notificationErr string
	offered         map[string]bool
	navigation      *deliveryNavigation
}

func (a *App) deliveryService() (DeliveryAPI, bool) {
	if c, ok := a.api.(*client.Client); ok {
		return c.AccountSession(), true
	}
	s, ok := a.api.(DeliveryAPI)
	return s, ok
}
func (a *App) tickDeliveries(now time.Time) {
	if a.screen == screenShell && !a.deliveries.loading && (a.deliveries.last.IsZero() || now.Sub(a.deliveries.last) >= 15*time.Second) {
		a.PollDeliveries()
	}
}

// PollDeliveries must be invoked through Window.Update, including while hidden.
func (a *App) PollDeliveries() {
	if a.screen != screenShell || a.deliveries.loading {
		return
	}
	svc, ok := a.deliveryService()
	if !ok {
		return
	}
	gen, uid := a.gen, a.user.ID
	a.deliveries.loading = true
	a.deliveries.last = time.Now()
	a.async(func() func() {
		items, err := svc.Deliveries(context.Background())
		if cl, ok := svc.(*client.Client); ok && err == nil {
			items = overlayDeliveryResponses(cl, items)
		}
		return func() {
			if gen != a.gen || uid != a.user.ID {
				return
			}
			a.deliveries.loading = false
			if err != nil {
				a.deliveries.err = a.failed(err)
				return
			}
			a.deliveries.items = items
			a.deliveries.err = ""
			if a.deliveries.offered == nil {
				a.deliveries.offered = map[string]bool{}
			}
			// A single catch-up toast represents the batch; individual objects remain
			// available in the inbox. Opening the app after a month never rings 200 bells.
			pending := []api.Delivery{}
			for _, d := range items {
				key := d.ID.String() + ":" + d.AvailableAt.Format(time.RFC3339Nano)
				if d.Channel == "device" && (d.Status == "queued" || d.Status == "accepted") && d.SeenAt == nil && !d.AvailableAt.After(time.Now()) && d.AvailableAt.After(time.Now().Add(-24*time.Hour)) && !a.deliveries.offered[key] {
					a.deliveries.offered[key] = true
					pending = append(pending, d)
				}
			}
			if len(pending) > 0 && a.Notifications != nil {
				sink := a.Notifications
				a.async(func() func() {
					var err error
					if batch, ok := sink.(NotificationBatchSink); ok {
						err = batch.NotifyBatch(uid, pending)
					} else {
						err = sink.Notify(uid, pending[0])
					}
					action := "accepted"
					if err != nil {
						action = "unknown"
					}
					for _, d := range pending {
						_ = svc.RespondDelivery(context.Background(), d.ID, api.DeliveryResponse{Action: action})
					}
					return func() {
						if gen != a.gen {
							return
						}
						if err != nil {
							a.deliveries.notificationErr = "Notifikasi perangkat belum diterima: " + err.Error()
						} else {
							a.deliveries.notificationErr = ""
						}
					}
				})
			}
		}
	})
}
func (a *App) RespondNotification(did id.ID, action string, minutes int) {
	if a.offlineMode() && (action == "seen" || action == "snooze") {
		a.queueDeliveryResponse(did, action, minutes)
		return
	}

	svc, ok := a.deliveryService()
	if !ok {
		return
	}
	gen := a.gen
	a.async(func() func() {
		err := svc.RespondDelivery(context.Background(), did, api.DeliveryResponse{Action: action, SnoozeMinutes: minutes})
		return func() {
			if gen != a.gen {
				return
			}
			if err != nil {
				a.deliveries.err = a.failed(err)
				return
			}
			a.PollDeliveries()
		}
	})
}

type deliveryNavigation struct {
	ready    bool
	delivery api.Delivery
	period   id.ID
	spaces   []id.ID
}

func (a *App) OpenNotification(d api.Delivery) {
	if d.RelatedID != nil {
		d.SourceKind = d.RelatedKind
		d.SourceID = *d.RelatedID
	}
	a.chrome.notificationsOpen = false
	cl, ok := a.api.(*client.Client)
	if !ok {
		return
	}
	service := cl.AccountSession()
	navigation := &deliveryNavigation{delivery: d}
	a.deliveries.navigation = navigation
	gen, currentPeriod := a.gen, a.periodID
	periods := slices.Clone(a.periods)
	knownSpaces := slices.Clone(a.access.spaces)
	a.async(func() func() {
		var err error
		var reminder api.Reminder
		var targetPeriod id.ID
		var targetSpaces []id.ID
		switch d.SourceKind {
		case "task":
			var task api.Task
			task, err = service.GetTask(context.Background(), d.SourceID)
			targetPeriod = task.PeriodID
			targetSpaces = task.SpaceIDs
		case "meeting":
			var meeting api.Meeting
			meeting, err = service.GetMeeting(context.Background(), d.SourceID)
			targetPeriod = meeting.PeriodID
			targetSpaces = []id.ID{meeting.SpaceID}
		case "reminder":
			reminder, err = service.GetReminder(context.Background(), d.SourceID)
		default:
			if d.SpaceID != nil {
				targetSpaces = []id.ID{*d.SpaceID}
				targetPeriod = currentPeriod
				found := false
				for _, sp := range knownSpaces {
					if sp.ID == *d.SpaceID {
						targetPeriod = sp.PeriodID
						found = true
						break
					}
				}
				if !found {
					for _, period := range periods {
						spaces, e := service.Spaces(context.Background(), period.ID)
						if client.IsUnauthenticated(e) {
							err = e
							break
						}
						if e != nil {
							continue
						}
						for _, sp := range spaces {
							if sp.ID == *d.SpaceID {
								targetPeriod = sp.PeriodID
								found = true
								break
							}
						}
						if found {
							break
						}
					}
				}
				if !found && err == nil {
					err = &client.Error{Status: 404, Code: api.CodeNotFound, Message: "Ruang notifikasi tidak lagi tersedia."}
				}
			}
		}
		return func() {
			if gen != a.gen || a.deliveries.navigation != navigation {
				return
			}
			if err != nil {
				a.deliveries.err = a.failed(err)
				a.deliveries.navigation = nil
				return
			}
			if d.SourceKind == "reminder" {
				a.navigate(PathReminder)
				a.ensureReminders()
				a.replaceReminder(reminder)
				a.openReminder(reminder.ID)
				a.deliveries.navigation = nil
				return
			}
			navigation.period = targetPeriod
			navigation.spaces = targetSpaces
			navigation.ready = true
			a.applyNotificationNavigation()
		}
	})
}
func (a *App) applyNotificationNavigation() {
	nav := a.deliveries.navigation
	if nav == nil || !nav.ready {
		return
	}
	if nav.period != id.Nil && nav.period != a.periodID {
		if a.periodsLoading {
			return
		}
		if !slices.ContainsFunc(a.periods, func(p api.Period) bool { return p.ID == nav.period }) {
			a.deliveries.err = "Periode objek notifikasi tidak tersedia."
			a.deliveries.navigation = nil
			return
		}
		a.changePeriod(nav.period)
		return
	}
	if a.access.loading {
		return
	}
	for _, sp := range a.access.spaces {
		if slices.Contains(nav.spaces, sp.ID) {
			a.selectSpace(sp.ID)
			a.navigate(PathSpaces)
			switch nav.delivery.SourceKind {
			case "task":
				a.chrome.tab = "Board tugas"
				a.ensureTasks(false)
				a.tasks.pendingOpen = nav.delivery.SourceID
				a.workspace.Open(nav.delivery.SourceID)
			case "meeting":
				a.chrome.tab = "Rapat"
				a.ensureMeetings()
				a.selectMeeting(nav.delivery.SourceID)
			}
			a.deliveries.navigation = nil
			return
		}
	}
	if a.access.err == "" {
		a.deliveries.err = "Ruang objek notifikasi tidak lagi dapat diakses."
		a.deliveries.navigation = nil
	}
}
func (a *App) deliveryNotifications(c *ui.Context) {
	if a.deliveries.notificationErr != "" {
		ui.Text(c, a.deliveries.notificationErr).FontSize(12).TextColor(c.Theme().Danger)
	}
	if a.deliveries.err != "" {
		muted(c, a.deliveries.err)
	}
	if len(a.deliveries.items) == 0 {
		muted(c, "Belum ada pengingat terjadwal.")
	}
	for _, d := range a.deliveries.items {
		d := d
		ui.Column(c.Key(d.ID)).FillWidth().Gap(5).Padding(8).Children(func() {
			ui.Text(c, d.Title).Bold()
			muted(c, d.Body)
			muted(c, deliveryStatusLabel(d.Status)+" · "+taskTimestamp(d.ScheduledAt, d.TimeZone))
			if d.LastError != "" {
				muted(c, d.LastError)
			}
			ui.Row(c).Wrap().Gap(4).Children(func() {
				if spaceButton(c, "Buka", "", false).Clicked() {
					a.OpenNotification(d)
				}
				if spaceButton(c, "Sudah lihat", "", false).Disabled(d.Channel != "device").Clicked() {
					a.RespondNotification(d.ID, "seen", 0)
				}
				if spaceButton(c, "15 menit lagi", "", false).Disabled(d.Channel != "device").Clicked() {
					a.RespondNotification(d.ID, "snooze", 15)
				}
			})
		})
	}
}
func (a *App) IsCurrentAccount(uid id.ID) bool { return a.screen == screenShell && a.user.ID == uid }
func (a *App) OpenDeviceSettings() {
	a.chrome.settingsTab = "Perangkat"
	a.chrome.settingsDetail = true
	a.navigate(PathPengaturan)
}

func overlayDeliveryResponses(cl *client.Client, items []api.Delivery) []api.Delivery {
	pending, err := cl.PendingOffline()
	if err != nil {
		return items
	}
	for i := range items {
		for _, p := range pending {
			if p.Mutation.Kind != "delivery.response" || p.Mutation.TargetID != items[i].ID || (p.State != "local" && p.State != "sending") {
				continue
			}
			if p.Mutation.Action == "seen" {
				at := p.CreatedAt
				items[i].SeenAt = &at
			} else if p.Mutation.SnoozeUntil != nil {
				items[i].AvailableAt = *p.Mutation.SnoozeUntil
				items[i].SnoozedUntil = p.Mutation.SnoozeUntil
				items[i].SeenAt = nil
				items[i].Status = "queued"
			}
		}
	}
	return items
}
func (a *App) queueDeliveryResponse(did id.ID, action string, minutes int) {
	cl := a.offlineClient()
	if cl == nil {
		return
	}
	mutation := api.OfflineMutation{ID: id.New(), Kind: "delivery.response", TargetID: did, Action: action}
	if action == "snooze" {
		if minutes < 1 || minutes > 10080 {
			return
		}
		until := time.Now().Add(time.Duration(minutes) * time.Minute)
		for _, d := range a.deliveries.items {
			if d.ID == did && until.Before(d.ScheduledAt) {
				until = d.ScheduledAt
			}
		}
		mutation.SnoozeUntil = &until
	}
	gen := a.gen
	a.async(func() func() {
		err := cl.QueueOffline(mutation)
		return func() {
			if gen != a.gen {
				return
			}
			if err != nil {
				a.deliveries.err = err.Error()
				return
			}
			a.deliveries.items = overlayDeliveryResponses(cl, a.deliveries.items)
			a.refreshOffline(false)
		}
	})
}

func deliveryStatusLabel(status string) string {
	switch status {
	case "queued":
		return "Dalam antrean"
	case "accepted":
		return "Diterima kanal perangkat"
	case "failed":
		return "Gagal"
	case "unknown":
		return "Penerimaan belum dapat dipastikan"
	case "cancelled":
		return "Dibatalkan"
	}
	return status
}
