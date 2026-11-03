//go:build !js

package main

import (
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func TestNotificationBatchDoesNotAcceptAnUncertainPriorAttempt(t *testing.T) {
	uid := id.New()
	n := api.Delivery{ID: id.New(), AvailableAt: time.Date(2026, 11, 2, 9, 25, 0, 0, time.UTC)}
	key := uid.String() + ":" + n.ID.String() + ":" + n.AvailableAt.Format(time.RFC3339Nano)
	d := &desktopDevice{journal: map[string]string{key: "unknown"}}
	if err := d.NotifyBatch(uid, []api.Delivery{n}); err == nil {
		t.Fatal("uncertain prior attempt must not be reported as accepted")
	}
	d.journal[key] = "accepted"
	if err := d.NotifyBatch(uid, []api.Delivery{n}); err != nil {
		t.Fatalf("confirmed prior acceptance must not be sent again: %v", err)
	}
}
