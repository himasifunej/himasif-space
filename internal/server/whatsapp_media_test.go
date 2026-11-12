package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
)

func TestWAMediaKindsAndGIFLimits(t *testing.T) {
	for command, want := range map[string]string{"sticker": "sticker", "setsticker": "sticker", "setgif": "gif"} {
		if kind, ok := waMediaCommand(command); !ok || kind.Type != want {
			t.Errorf("%s converts %q (%v), want %q", command, kind.Type, ok, want)
		}
	}
	// !gif is left to custom commands; only !setgif is reserved.
	for _, command := range []string{"gif", "tugas", "damn"} {
		if _, ok := waMediaCommand(command); ok {
			t.Errorf("%s converts media", command)
		}
	}
	if err := validateWACommand(&api.WACustomCommandRequest{Trigger: "setgif", Response: "x"}); err == nil {
		t.Error("setgif accepted as a custom trigger")
	}
	if err := validateWACommand(&api.WACustomCommandRequest{Trigger: "gif", AssetID: strings.Repeat("a", 64), MediaType: "gif"}); err != nil {
		t.Errorf("gif media refused: %v", err)
	}
	ok := whatsapp.Asset{AssetID: strings.Repeat("a", 64), MIME: "video/mp4", Size: 200_000, DurationMS: 3_000}
	if err := validateWAGIF(ok); err != nil {
		t.Fatalf("valid GIF refused: %v", err)
	}
	for name, change := range map[string]func(*whatsapp.Asset){
		"webp":      func(a *whatsapp.Asset) { a.MIME = "image/webp" },
		"gif file":  func(a *whatsapp.Asset) { a.MIME = "image/gif" },
		"bad id":    func(a *whatsapp.Asset) { a.AssetID = "../asset" },
		"empty":     func(a *whatsapp.Asset) { a.Size = 0 },
		"too big":   func(a *whatsapp.Asset) { a.Size = waGIFMaxBytes + 1 },
		"too long":  func(a *whatsapp.Asset) { a.DurationMS = 10_501 },
		"no length": func(a *whatsapp.Asset) { a.DurationMS = 0 },
	} {
		a := ok
		change(&a)
		if validateWAGIF(a) == nil {
			t.Errorf("%s accepted", name)
		}
	}
	edge := ok
	edge.Size, edge.DurationMS = waGIFMaxBytes, 10_500
	if err := validateWAGIF(edge); err != nil {
		t.Errorf("GIF at the limits refused: %v", err)
	}
}

// waMediaBridge stands in for the bridge's conversion endpoints.
type waMediaBridge struct {
	mu     sync.Mutex
	calls  []string // "gif:<messageId>", "sticker:<messageId>"
	status int
	asset  whatsapp.Asset
}

func (b *waMediaBridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body struct{ GroupJID, MessageID string }
	json.NewDecoder(r.Body).Decode(&body)
	op := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	b.mu.Lock()
	b.calls = append(b.calls, op+":"+body.MessageID)
	status, asset := b.status, b.asset
	b.mu.Unlock()
	if status != 0 {
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]any{"error": "media unavailable"})
		return
	}
	json.NewEncoder(w).Encode(asset)
}

func (b *waMediaBridge) set(status int, asset whatsapp.Asset) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.status, b.asset = status, asset
}

func (b *waMediaBridge) called() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.calls...)
}

type waMediaFixture struct {
	*taskOrg
	cid, gid       id.ID
	kadivJID, jid  string
	groupJID       string
	bridge         *waMediaBridge
	kadivID        id.ID
	kadivTok       string
	connectionPath string
}

func newWAMediaFixture(t *testing.T) *waMediaFixture {
	o := newTaskOrg(t)
	ctx := context.Background()
	f := &waMediaFixture{taskOrg: o, cid: id.New(), gid: id.New(), kadivJID: "6281300000001@s.whatsapp.net", jid: "6281300000002@s.whatsapp.net", groupJID: "120363000000000011@g.us", bridge: &waMediaBridge{}, kadivTok: o.kadiv}
	if err := o.st.DB().QueryRow(ctx, `select id from users where lower(email)='task-kadiv@himasif.test'`).Scan(&f.kadivID); err != nil {
		t.Fatal(err)
	}
	mediatek := o.div["mediatek"].ID
	if _, err := o.st.DB().Exec(ctx, `insert into wa_connections(id,period_id,division_id,display_name,enabled,status,account_jid,created_by,updated_at) values($1,$2,$3,'Bot Mediatek',true,'connected','6281111111111@s.whatsapp.net',$4,now())`, f.cid, o.period.ID, mediatek, f.kadivID); err != nil {
		t.Fatal(err)
	}
	if _, err := o.st.DB().Exec(ctx, `insert into wa_groups(id,connection_id,space_id,period_id,jid,name,commands_enabled) values($1,$2,$3,$4,$5,'Grup Mediatek',true)`, f.gid, f.cid, o.spaces["division-mediatek"], o.period.ID, f.groupJID); err != nil {
		t.Fatal(err)
	}
	for user, jid := range map[id.ID]string{f.kadivID: f.kadivJID, o.memberID: f.jid} {
		if _, err := o.st.DB().Exec(ctx, `insert into wa_identities(period_id,division_id,user_id,jid,verified_by,verified_at) values($1,$2,$3,$4,$3,now())`, o.period.ID, mediatek, user, jid); err != nil {
			t.Fatal(err)
		}
	}
	hs := httptest.NewServer(f.bridge)
	t.Cleanup(hs.Close)
	o.srv.wa = whatsapp.New(hs.URL, strings.Repeat("t", 32))
	f.connectionPath = "/api/whatsapp/" + f.cid.String()
	return f
}

// send queues a group message, as the bridge's event would, and runs the
// command worker. It returns the inbound state and the replies queued.
func (f *waMediaFixture) send(sender, text, quoted string) (string, []whatsapp.SendRequest) {
	f.t.Helper()
	ctx := context.Background()
	iid := id.New()
	if _, err := f.st.DB().Exec(ctx, `insert into wa_inbound(id,connection_id,group_id,group_jid,sender_jid,message_id,envelope_id,revision,kind,body,quoted_message_id,received_at) values($1,$2,$3,$4,$5,$6,$6,'r','message',$7,$8,now())`, iid, f.cid, f.gid, f.groupJID, sender, iid.String(), text, quoted); err != nil {
		f.t.Fatal(err)
	}
	for {
		found, err := f.srv.processWACommand(ctx)
		if err != nil {
			f.t.Fatal(err)
		}
		if !found {
			break
		}
	}
	var state string
	f.st.DB().QueryRow(ctx, `select state from wa_inbound where id=$1`, iid).Scan(&state)
	rows, err := f.st.DB().Query(ctx, `select payload from wa_outbox where source_id=$1 order by event_key`, iid)
	if err != nil {
		f.t.Fatal(err)
	}
	defer rows.Close()
	var replies []whatsapp.SendRequest
	for rows.Next() {
		var raw []byte
		var r whatsapp.SendRequest
		if err := rows.Scan(&raw); err != nil {
			f.t.Fatal(err)
		}
		json.Unmarshal(raw, &r)
		replies = append(replies, r)
	}
	return state, replies
}

func (f *waMediaFixture) command(trigger string) (assetID, mediaType string) {
	f.t.Helper()
	f.st.DB().QueryRow(context.Background(), `select asset_id,media_type from wa_custom_commands where connection_id=$1 and trigger=$2`, f.cid, trigger).Scan(&assetID, &mediaType)
	return
}

// !setgif replying to a GIF, video or image saves the bridge's mp4 under the
// trigger; !trigger then replies with it as a GIF, in one message.
func TestWASetGIFSavesAndReplies(t *testing.T) {
	f := newWAMediaFixture(t)
	ctx := context.Background()
	gif := whatsapp.Asset{AssetID: strings.Repeat("a1", 32), MIME: "video/mp4", Size: 180_000, DurationMS: 4_200}
	f.bridge.set(0, gif)
	state, replies := f.send(f.kadivJID, "!setgif damn", "QUOTED-GIF")
	if state != "applied" || len(replies) != 1 || !strings.Contains(replies[0].Text, "GIF tersimpan sebagai !damn") || replies[0].AssetID != "" {
		t.Fatalf("setgif: %s %+v", state, replies)
	}
	if calls := f.bridge.called(); len(calls) != 1 || calls[0] != "gif:QUOTED-GIF" {
		t.Fatalf("bridge calls %v", calls)
	}
	if asset, media := f.command("damn"); asset != gif.AssetID || media != "gif" {
		t.Fatalf("saved command %q %q", asset, media)
	}
	var name, mime string
	var size int64
	var source id.ID
	if err := f.st.DB().QueryRow(ctx, `select name,mime,size,created_by from wa_assets where connection_id=$1 and bridge_asset_id=$2`, f.cid, gif.AssetID).Scan(&name, &mime, &size, &source); err != nil || name != "GIF WhatsApp" || mime != "video/mp4" || size != gif.Size || source != f.kadivID {
		t.Fatalf("asset %q %q %d %v: %v", name, mime, size, source, err)
	}
	var audits int
	f.st.DB().QueryRow(ctx, `select count(*) from wa_events where connection_id=$1 and action='whatsapp.command.gif'`, f.cid).Scan(&audits)
	if audits != 1 {
		t.Fatalf("audit events %d", audits)
	}
	// Any member of the group may use it; a GIF keeps its caption, so one job.
	state, replies = f.send(f.jid, "!damn", "")
	if state != "applied" || len(replies) != 1 || replies[0].AssetID != gif.AssetID || replies[0].MediaType != "gif" || replies[0].GroupJID != f.groupJID {
		t.Fatalf("!damn: %s %+v", state, replies)
	}
	if len(f.bridge.called()) != 1 {
		t.Fatal("replying converted again")
	}
	// The command editor lists it with its media type.
	var list []api.WACustomCommand
	if c := f.call("GET", f.connectionPath+"/commands", f.kadivTok, nil, &list); c != 200 || len(list) != 1 || list[0].MediaType != "gif" {
		t.Fatalf("commands = %d %+v", c, list)
	}
}

// What the bridge returns is checked again: type, size and duration.
func TestWASetGIFRejectsOutOfLimitMedia(t *testing.T) {
	f := newWAMediaFixture(t)
	good := whatsapp.Asset{AssetID: strings.Repeat("b2", 32), MIME: "video/mp4", Size: 100_000, DurationMS: 2_000}
	for _, c := range []struct {
		name  string
		asset whatsapp.Asset
		says  string
	}{
		{"too long", whatsapp.Asset{AssetID: good.AssetID, MIME: good.MIME, Size: good.Size, DurationMS: 12_000}, "10 detik"},
		{"too big", whatsapp.Asset{AssetID: good.AssetID, MIME: good.MIME, Size: waGIFMaxBytes + 1, DurationMS: good.DurationMS}, "4 MiB"},
		{"not mp4", whatsapp.Asset{AssetID: good.AssetID, MIME: "image/webp", Size: good.Size, DurationMS: good.DurationMS}, "mp4"},
	} {
		f.bridge.set(0, c.asset)
		state, replies := f.send(f.kadivJID, "!setgif damn", "Q")
		if state != "failed" || len(replies) != 1 || !strings.Contains(replies[0].Text, c.says) {
			t.Fatalf("%s: %s %+v", c.name, state, replies)
		}
		if asset, _ := f.command("damn"); asset != "" {
			t.Fatalf("%s: command saved", c.name)
		}
	}
	var assets int
	f.st.DB().QueryRow(context.Background(), `select count(*) from wa_assets where connection_id=$1`, f.cid).Scan(&assets)
	if assets != 0 {
		t.Fatalf("%d assets kept from refused GIFs", assets)
	}
	// Media the bridge cannot convert, and a bridge that is down.
	f.bridge.set(400, whatsapp.Asset{})
	if state, replies := f.send(f.kadivJID, "!setgif damn", "TEXT"); state != "failed" || !strings.Contains(replies[0].Text, "tidak dapat dijadikan GIF") {
		t.Fatalf("unconvertible: %s %+v", state, replies)
	}
	f.bridge.set(500, whatsapp.Asset{})
	if state, replies := f.send(f.kadivJID, "!setgif damn", "Q"); state != "failed" || !strings.Contains(replies[0].Text, "Konversi GIF belum tersedia") {
		t.Fatalf("bridge down: %s %+v", state, replies)
	}
	// Usage, and members without WhatsApp management, never reach the bridge.
	before := len(f.bridge.called())
	if state, replies := f.send(f.kadivJID, "!setgif", "Q"); state != "failed" || !strings.Contains(replies[0].Text, "!setgif <pemicu>") {
		t.Fatalf("usage: %s %+v", state, replies)
	}
	if state, _ := f.send(f.jid, "!setgif mine", "Q"); state != "failed" {
		t.Fatalf("member saved a GIF: %s", state)
	}
	if state, _ := f.send(f.kadivJID, "!setgif tugas", "Q"); state != "failed" {
		t.Fatalf("core command replaced: %s", state)
	}
	if after := len(f.bridge.called()); after != before {
		t.Fatalf("bridge called %d times for refused commands", after-before)
	}
}

// The sticker path: !setsticker saves, !trigger replies with a captionless
// sticker, !sticker converts and replies at once.
func TestWAStickerCommands(t *testing.T) {
	f := newWAMediaFixture(t)
	sticker := whatsapp.Asset{AssetID: strings.Repeat("c3", 32), MIME: "image/webp", Size: 40_000}
	f.bridge.set(0, sticker)
	state, replies := f.send(f.kadivJID, "!setsticker wow", "QUOTED-PHOTO")
	if state != "applied" || len(replies) != 1 || !strings.Contains(replies[0].Text, "Stiker tersimpan sebagai !wow") {
		t.Fatalf("setsticker: %s %+v", state, replies)
	}
	if calls := f.bridge.called(); len(calls) != 1 || calls[0] != "sticker:QUOTED-PHOTO" {
		t.Fatalf("bridge calls %v", calls)
	}
	if asset, media := f.command("wow"); asset != sticker.AssetID || media != "sticker" {
		t.Fatalf("saved command %q %q", asset, media)
	}
	state, replies = f.send(f.jid, "!wow", "")
	if state != "applied" || len(replies) != 1 || replies[0].AssetID != sticker.AssetID || replies[0].MediaType != "sticker" || replies[0].Text != "" {
		t.Fatalf("!wow: %s %+v", state, replies)
	}
	// !sticker on an attached image: no command, the sticker straight back.
	state, replies = f.send(f.jid, "!sticker", "")
	if state != "applied" || len(replies) != 1 || replies[0].MediaType != "sticker" || replies[0].AssetID != sticker.AssetID {
		t.Fatalf("!sticker: %s %+v", state, replies)
	}
	if calls := f.bridge.called(); len(calls) != 2 || !strings.HasPrefix(calls[1], "sticker:") {
		t.Fatalf("bridge calls %v", calls)
	}
	// A sticker image cannot be saved as a GIF through the editor.
	req := api.WACustomCommandRequest{Trigger: "wow", AssetID: sticker.AssetID, MediaType: "gif", GroupIDs: []id.ID{f.gid}}
	var cmdID id.ID
	f.st.DB().QueryRow(context.Background(), `select id from wa_custom_commands where connection_id=$1 and trigger='wow'`, f.cid).Scan(&cmdID)
	req.ExpectedVersion = 1
	if c, _ := f.errCode("PUT", f.connectionPath+"/commands/"+cmdID.String(), f.kadivTok, req); c != 400 {
		t.Fatalf("sticker saved as GIF: %d", c)
	}
	// Replacing it with a GIF later keeps the slot and its group restriction.
	gif := whatsapp.Asset{AssetID: strings.Repeat("d4", 32), MIME: "video/mp4", Size: 90_000, DurationMS: 3_000}
	f.bridge.set(0, gif)
	if state, _ := f.send(f.kadivJID, "!setgif wow", "Q2"); state != "applied" {
		t.Fatalf("replace: %s", state)
	}
	if asset, media := f.command("wow"); asset != gif.AssetID || media != "gif" {
		t.Fatalf("replaced command %q %q", asset, media)
	}
	req.AssetID, req.ExpectedVersion = gif.AssetID, 2
	if c := f.call("PUT", f.connectionPath+"/commands/"+cmdID.String(), f.kadivTok, req, nil); c != 200 {
		t.Fatalf("editor save of the GIF = %d", c)
	}
}
