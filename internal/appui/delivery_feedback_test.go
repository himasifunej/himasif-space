package appui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

type deliveryFeedbackAPI struct {
	*fakeAPI
	items     []api.Delivery
	responses []string
}

func (f *deliveryFeedbackAPI) Deliveries(context.Context) ([]api.Delivery, error) {
	return f.items, nil
}
func (f *deliveryFeedbackAPI) RespondDelivery(_ context.Context, _ id.ID, r api.DeliveryResponse) error {
	f.responses = append(f.responses, r.Action)
	return nil
}
func (f *deliveryFeedbackAPI) SendPing(context.Context, id.ID, api.PingRequest) error { return nil }

type failingNotificationSink struct{}

func (failingNotificationSink) Notify(id.ID, api.Delivery) error {
	return errors.New("izin Windows ditolak")
}

func TestDeliveryErrorSurvivesSuccessfulRefresh(t *testing.T) {
	f := &deliveryFeedbackAPI{fakeAPI: newFake(api.AccountPerson)}
	f.items = []api.Delivery{{ID: id.New(), Channel: "device", Status: "queued", AvailableAt: time.Now().Add(-time.Minute)}}
	h := newHarness(t, f.fakeAPI, "")
	h.app.api = f
	h.app.screen = screenShell
	h.app.user = f.user
	h.app.Notifications = failingNotificationSink{}
	h.app.PollDeliveries()
	h.settle()
	if !strings.Contains(h.app.deliveries.notificationErr, "izin Windows ditolak") || len(f.responses) != 1 || f.responses[0] != "unknown" {
		t.Fatalf("notification failure was not retained and reported: %q %v", h.app.deliveries.notificationErr, f.responses)
	}
	h.app.PollDeliveries()
	h.settle()
	if !strings.Contains(h.app.deliveries.notificationErr, "izin Windows ditolak") {
		t.Fatal("a successful inbox refresh erased the device notification failure")
	}
}
