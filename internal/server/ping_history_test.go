package server

import (
	"context"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Riwayat ping (Tahap 5b): the sender's side sees every ping of the Space
// with all recipients; a member sees only pings sent to them, and only
// their own delivery row; another division's member cannot read it.
func TestPingHistory(t *testing.T) {
	o := newOrg(t)
	kTok, kID := o.kadiv("mediatek", "km@himasif.test")
	aTok, aAcc := o.join(kTok, o.period.ID, "mediatek", "a@himasif.test", "Aruna")
	bTok, bAcc := o.join(kTok, o.period.ID, "mediatek", "b@himasif.test", "Bima")
	hTok, _ := o.join(o.admin, o.period.ID, "humas", "h@himasif.test", "Humas")

	var sps api.List[api.Space]
	o.call("GET", "/api/periods/"+o.period.ID.String()+"/spaces", kTok, nil, &sps)
	var sp api.Space
	for _, s := range sps.Items {
		if s.Code == "division-mediatek" {
			sp = s
		}
	}
	if sp.ID == id.Nil {
		t.Fatal("no mediatek space")
	}
	path := "/api/spaces/" + sp.ID.String() + "/pings"
	if c := o.call("POST", path, kTok, api.PingRequest{Message: "Ping pertama", Recipients: []id.ID{aAcc.User.ID, bAcc.User.ID}}, nil); c != 201 {
		t.Fatalf("first ping = %d", c)
	}
	if c := o.call("POST", path, kTok, api.PingRequest{Message: "Ping kedua", Recipients: []id.ID{bAcc.User.ID}}, nil); c != 201 {
		t.Fatalf("second ping = %d", c)
	}

	var all api.List[api.PingRecord]
	if c := o.call("GET", path, kTok, nil, &all); c != 200 || len(all.Items) != 2 {
		t.Fatalf("kadiv history = %d %+v", c, all)
	}
	if all.Items[0].Message != "Ping kedua" || all.Items[1].Message != "Ping pertama" {
		t.Fatalf("not newest first: %+v", all.Items)
	}
	first := all.Items[1]
	if first.Sender == nil || first.Sender.ID != kID || len(first.Recipients) != 2 || first.Recipients[0].Channel != "device" || first.Recipients[0].Status == "" {
		t.Fatalf("first ping = %+v", first)
	}

	var page api.List[api.PingRecord]
	if c := o.call("GET", path+"?limit=1", kTok, nil, &page); c != 200 || len(page.Items) != 1 || page.Items[0].Message != "Ping kedua" {
		t.Fatalf("limit=1 = %d %+v", c, page)
	}

	var mine api.List[api.PingRecord]
	if c := o.call("GET", path, aTok, nil, &mine); c != 200 || len(mine.Items) != 1 || mine.Items[0].Message != "Ping pertama" {
		t.Fatalf("aruna history = %d %+v", c, mine)
	}
	if r := mine.Items[0].Recipients; len(r) != 1 || r[0].Recipient.ID != aAcc.User.ID {
		t.Fatalf("aruna sees other recipients: %+v", r)
	}
	mine = api.List[api.PingRecord]{}
	if c := o.call("GET", path, bTok, nil, &mine); c != 200 || len(mine.Items) != 2 {
		t.Fatalf("bima history = %d %+v", c, mine)
	}

	if c, _ := o.errCode("GET", path, hTok, nil); c != 403 && c != 404 {
		t.Fatalf("humas reads mediatek pings = %d", c)
	}
	if c, _ := o.errCode("GET", path+"?limit=0", kTok, nil); c != 400 {
		t.Fatalf("limit=0 = %d", c)
	}

	// Tahap 7c filters. Bima saw the second ping; Aruna's first-ping
	// delivery failed. Unseen = someone has not seen it on any channel.
	ctx := context.Background()
	if _, err := o.st.DB().Exec(ctx, `update delivery_recipients d set seen_at=now() from delivery_events e
		where e.id=d.event_id and e.source_kind='ping' and e.body='Ping kedua' and d.recipient_id=$1`, bAcc.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := o.st.DB().Exec(ctx, `update delivery_recipients d set status='failed' from delivery_events e
		where e.id=d.event_id and e.source_kind='ping' and e.body='Ping pertama' and d.recipient_id=$1`, aAcc.User.ID); err != nil {
		t.Fatal(err)
	}
	var unseen, failedList api.List[api.PingRecord]
	if c := o.call("GET", path+"?filter=unseen", kTok, nil, &unseen); c != 200 || len(unseen.Items) != 1 || unseen.Items[0].Message != "Ping pertama" {
		t.Fatalf("unseen = %d %+v", c, unseen)
	}
	if c := o.call("GET", path+"?filter=failed", kTok, nil, &failedList); c != 200 || len(failedList.Items) != 1 || failedList.Items[0].Message != "Ping pertama" {
		t.Fatalf("failed = %d %+v", c, failedList)
	}
	// Bima's own view: he saw "Ping kedua", and did not see "Ping pertama".
	var bimaUnseen api.List[api.PingRecord]
	if c := o.call("GET", path+"?filter=unseen", bTok, nil, &bimaUnseen); c != 200 || len(bimaUnseen.Items) != 1 || bimaUnseen.Items[0].Message != "Ping pertama" {
		t.Fatalf("bima unseen = %d %+v", c, bimaUnseen)
	}
	if c, _ := o.errCode("GET", path+"?filter=semua", kTok, nil); c != 400 {
		t.Fatalf("unknown filter = %d", c)
	}
}
