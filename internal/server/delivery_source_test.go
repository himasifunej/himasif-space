package server

import (
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// QA rc.1 finding 9: deliveries of one source share one visibility check;
// a ping about another task does not.
func TestDeliverySourceKey(t *testing.T) {
	task, other, space := id.New(), id.New(), id.New()
	a := api.Delivery{ID: id.New(), SourceKind: "task", SourceID: task, SpaceID: &space, Channel: "device"}
	b := api.Delivery{ID: id.New(), SourceKind: "task", SourceID: task, SpaceID: &space, Channel: "whatsapp"}
	if deliverySourceOf(a) != deliverySourceOf(b) {
		t.Fatal("two channels of one task checked twice")
	}
	ping := api.Delivery{SourceKind: "ping", SourceID: id.New(), RelatedKind: "task", RelatedID: &task}
	ping2 := ping
	ping2.RelatedID = &other
	if deliverySourceOf(ping) == deliverySourceOf(ping2) {
		t.Fatal("pings about different tasks share a check")
	}
}
