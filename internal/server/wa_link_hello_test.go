package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
)

// The first link of an account queues one test message to that number in
// a private chat, after the reply; linking again sends no second one.
func TestWALinkHelloOnce(t *testing.T) {
	o := newTaskOrg(t)
	ctx := context.Background()
	scope := o.scope(o.period.ID, "mediatek")
	cid := id.New()
	if _, err := o.st.DB().Exec(ctx, `insert into wa_connections(id,period_id,division_id,display_name,enabled,status,account_jid,created_by,updated_at) values($1,$2,$3,'Bot Mediatek',true,'connected','6281111111111@s.whatsapp.net',$4,now())`,
		cid, o.period.ID, o.div["mediatek"].ID, o.memberID); err != nil {
		t.Fatal(err)
	}
	code := func() string {
		var l api.WALinkChallenge
		if c := o.call("POST", scope+"/whatsapp-link", o.member, nil, &l); c != 201 {
			t.Fatalf("code = %d", c)
		}
		return l.Code
	}
	hellos := func() []whatsapp.SendRequest {
		t.Helper()
		rows, err := o.st.DB().Query(ctx, `select payload from wa_outbox where event_key=$1 and source_kind='link_dm' and group_id is null and available_at > created_at`, "link-hello:"+o.memberID.String())
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []whatsapp.SendRequest
		for rows.Next() {
			var raw []byte
			var r whatsapp.SendRequest
			if err := rows.Scan(&raw); err != nil {
				t.Fatal(err)
			}
			json.Unmarshal(raw, &r)
			out = append(out, r)
		}
		return out
	}
	phone := "6282222222222@s.whatsapp.net"
	if _, state, _ := waTestInbound(t, o, cid, nil, "", phone, code()); state != "applied" {
		t.Fatalf("link state %s", state)
	}
	h := hellos()
	if len(h) != 1 || h[0].ToJID != phone || h[0].Text != waLinkHelloText || h[0].GroupJID != "" {
		t.Fatalf("hello %+v", h)
	}
	var on api.Onboarding
	o.call("GET", "/api/me/onboarding", o.member, nil, &on)
	if !on.Done(api.OnboardWAHello) {
		t.Fatal("hello not recorded")
	}
	// Clients cannot set or clear the server's mark.
	if c, _ := o.errCode("POST", "/api/me/onboarding", o.member, api.OnboardingMark{Key: api.OnboardWAHello, Clear: true}); c != 400 {
		t.Fatalf("client cleared waHello: %d", c)
	}
	// Unlink, link again: no second test message.
	if c := o.call("DELETE", scope+"/whatsapp-link", o.member, nil, nil); c != 204 && c != 200 {
		t.Fatalf("unlink = %d", c)
	}
	if _, state, _ := waTestInbound(t, o, cid, nil, "", phone, code()); state != "applied" {
		t.Fatalf("relink state %s", state)
	}
	if len(hellos()) != 1 {
		t.Fatal("second hello queued")
	}
}
