package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
)

// waTestInbound stores a message as the ingest does and runs the command
// worker until it is idle; groupJID "" is a private chat with the bot.
func waTestInbound(t *testing.T, o *taskOrg, cid id.ID, groupID *id.ID, groupJID, sender, text string) (id.ID, string, string) {
	t.Helper()
	ctx := context.Background()
	body, state, reason := text, "queued", ""
	if groupJID == "" {
		body, state, reason = waDirectInbound(o.srv, whatsapp.Event{Type: "message", SenderJID: sender, Text: text}, true)
	}
	iid := id.New()
	if _, err := o.st.DB().Exec(ctx, `insert into wa_inbound(id,connection_id,group_id,group_jid,sender_jid,message_id,envelope_id,revision,kind,body,received_at,state,error) values($1,$2,$3,$4,$5,$6,$6,'r','message',$7,now(),$8,$9)`,
		iid, cid, groupID, groupJID, sender, iid.String(), body, state, reason); err != nil {
		t.Fatal(err)
	}
	for {
		found, err := o.srv.processWACommand(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			break
		}
	}
	var st string
	o.st.DB().QueryRow(ctx, `select state from wa_inbound where id=$1`, iid).Scan(&st)
	var reply whatsapp.SendRequest
	var payload []byte
	if o.st.DB().QueryRow(ctx, `select payload from wa_outbox where source_id=$1 order by created_at limit 1`, iid).Scan(&payload) == nil {
		json.Unmarshal(payload, &reply)
	}
	return iid, st, reply.Text
}

// A number that already belongs to another account is not taken silently:
// the reply names that account and offers PINDAH; only the same number may
// answer it. Linking replies are plain text, without the bot's name or
// greeting, in a private chat and in a group alike.
func TestWALinkMoveWithPindah(t *testing.T) {
	o := newTaskOrg(t)
	ctx := context.Background()
	scope := o.scope(o.period.ID, "mediatek")
	cid := id.New()
	if _, err := o.st.DB().Exec(ctx, `insert into wa_connections(id,period_id,division_id,display_name,greeting,enabled,status,account_jid,created_by,updated_at) values($1,$2,$3,'Space Showcase Mediatek','HELLOOOO',true,'connected','6281111111111@s.whatsapp.net',$4,now())`,
		cid, o.period.ID, o.div["mediatek"].ID, o.memberID); err != nil {
		t.Fatal(err)
	}
	code := func(token string) string {
		var l api.WALinkChallenge
		if c := o.call("POST", scope+"/whatsapp-link", token, nil, &l); c != 201 {
			t.Fatalf("code = %d", c)
		}
		return l.Code
	}
	clean := func(reply string) {
		t.Helper()
		if strings.Contains(reply, "HELLOOOO") || strings.Contains(reply, "Space Showcase Mediatek") {
			t.Fatalf("reply carries bot name/greeting: %q", reply)
		}
	}
	phone := "6282222222222@s.whatsapp.net"
	_, _, reply := waTestInbound(t, o, cid, nil, "", phone, code(o.member))
	clean(reply)
	if !strings.HasPrefix(reply, "Tertaut ✓") {
		t.Fatalf("link reply %q", reply)
	}

	// The kadiv's code from the same number: a conflict with a way out.
	_, state, reply := waTestInbound(t, o, cid, nil, "", phone, code(o.kadiv))
	clean(reply)
	if state != "applied" || !strings.Contains(reply, "sudah tertaut ke akun Space *PJ Mediatek*") || !strings.Contains(reply, "balas *PINDAH*") {
		t.Fatalf("conflict state=%s reply=%q", state, reply)
	}
	var status api.WALinkStatus
	o.call("GET", scope+"/whatsapp-link", o.kadiv, nil, &status)
	if status.Linked || status.Pending == nil || status.Pending.ConflictAccount != "PJ Mediatek" || status.Pending.MoveUntil == nil {
		t.Fatalf("kadiv status %+v pending %+v", status, status.Pending)
	}

	// PINDAH from a number that did not send the code moves nothing.
	_, state, reply = waTestInbound(t, o, cid, nil, "", "6287777777777@s.whatsapp.net", "PINDAH")
	if state != "ignored" {
		// The bridge drops it; were it to arrive, the server refuses it.
		if state != "failed" || !strings.Contains(reply, "Tidak ada permintaan pindah") {
			t.Fatalf("stranger PINDAH state=%s reply=%q", state, reply)
		}
	}
	_, state, reply = waTestInbound(t, o, cid, nil, "", phone, "pindah")
	clean(reply)
	if state != "applied" || !strings.HasPrefix(reply, "Dipindahkan ✓") || !strings.Contains(reply, "Akun PJ Mediatek tidak lagi") {
		t.Fatalf("move state=%s reply=%q", state, reply)
	}
	var owner id.ID
	o.st.DB().QueryRow(ctx, `select user_id from wa_identities where period_id=$1 and jid=$2`, o.period.ID, phone).Scan(&owner)
	var kadiv api.User
	o.call("GET", "/api/me", o.kadiv, nil, &kadiv)
	if owner != kadiv.ID {
		t.Fatalf("number owner %v, want kadiv %v", owner, kadiv.ID)
	}
	status = api.WALinkStatus{}
	o.call("GET", scope+"/whatsapp-link", o.member, nil, &status)
	if status.Linked {
		t.Fatal("old account still linked")
	}
	// A second PINDAH has nothing left to move.
	_, state, _ = waTestInbound(t, o, cid, nil, "", phone, "PINDAH")
	if state != "failed" {
		t.Fatalf("second PINDAH state %s", state)
	}

	// In a group: the same plain reply, and !pindah is how to answer.
	gid := id.New()
	gjid := "120363000000000001@g.us"
	if _, err := o.st.DB().Exec(ctx, `insert into wa_groups(id,connection_id,space_id,period_id,jid,name,commands_enabled) values($1,$2,$3,$4,$5,'Grup Mediatek',true)`,
		gid, cid, o.spaces["division-mediatek"], o.period.ID, gjid); err != nil {
		t.Fatal(err)
	}
	_, state, reply = waTestInbound(t, o, cid, &gid, gjid, phone, "!link "+code(o.member))
	clean(reply)
	if state != "applied" || !strings.Contains(reply, "kirim *!pindah* di sini") || strings.Contains(reply, "NIM") {
		t.Fatalf("group conflict state=%s reply=%q", state, reply)
	}
	_, state, reply = waTestInbound(t, o, cid, &gid, gjid, phone, "!pindah")
	clean(reply)
	if state != "applied" || !strings.HasPrefix(reply, "Dipindahkan ✓") {
		t.Fatalf("group move state=%s reply=%q", state, reply)
	}
}
