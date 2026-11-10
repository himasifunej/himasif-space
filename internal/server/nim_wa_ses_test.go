package server

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/mail"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
)

// Plan §8: the kadiv invites by NIM; the email is the student address and
// the NIM, derived from the verified email, shows to the division.
func TestInviteByNIMAndMemberNIM(t *testing.T) {
	o := newOrg(t)
	k, _ := o.kadiv("mediatek", "nim-kadiv@himasif.test")
	path := o.scope(o.period.ID, "mediatek") + "/invitations"

	var inv api.Invitation
	if c := o.call("POST", path, k, api.InviteRequest{NIM: "232410101090"}, &inv); c != 201 {
		t.Fatalf("invite by NIM = %d", c)
	}
	if inv.Email != "232410101090@mail.unej.ac.id" || inv.NIM != "232410101090" {
		t.Fatalf("invitation %+v", inv)
	}
	sent := o.mail.last(t, inv.Email)
	if !strings.Contains(sent.HTML, "Terima undangan") || !strings.Contains(sent.HTML, "token=") || !strings.Contains(sent.Text, "token=") {
		t.Fatalf("invitation mail text=%q html=%q", sent.Text, sent.HTML)
	}
	for _, bad := range []api.InviteRequest{{NIM: "2324ab"}, {NIM: "1234"}, {NIM: "232410101091", Email: "x@himasif.test"}} {
		if c := o.call("POST", path, k, bad, nil); c != 400 {
			t.Errorf("invite %+v = %d, want 400", bad, c)
		}
	}

	// A student who accepts: the NIM comes from the email.
	aruna, acc := o.join(k, o.period.ID, "mediatek", "232410101092@mail.unej.ac.id", "Aruna Desain")
	if acc.User.NIM != "232410101092" || acc.Membership.NIM != "232410101092" {
		t.Fatalf("accepted user %+v member %+v", acc.User, acc.Membership)
	}
	var me api.User
	o.call("GET", "/api/me", aruna, nil, &me)
	if me.NIM != "232410101092" {
		t.Fatalf("me %+v", me)
	}
	// Every member of the division sees it; other accounts have none.
	var members api.List[api.Member]
	if c := o.call("GET", o.scope(o.period.ID, "mediatek")+"/members", aruna, nil, &members); c != 200 {
		t.Fatal(c)
	}
	nims := map[string]string{}
	for _, m := range members.Items {
		nims[m.DisplayName] = m.NIM
	}
	if nims["Aruna Desain"] != "232410101092" || nims["Kadiv mediatek"] != "" {
		t.Fatalf("member NIMs %v", nims)
	}
	// A member of another division does not read this directory.
	peer, _ := o.join(o.admin, o.period.ID, "humas", "nim-peer@himasif.test", "Anggota Humas")
	if c := o.call("GET", o.scope(o.period.ID, "mediatek")+"/members", peer, nil, nil); c != 403 {
		t.Fatalf("other division reads members = %d", c)
	}

	// The provider refusing the address says so, and nothing is created.
	o.mail.err = &mail.RejectedError{Code: 554, Msg: "Message rejected: Email address is not verified."}
	var er api.ErrorResponse
	if c := o.call("POST", path, k, api.InviteRequest{NIM: "232410101093"}, &er); c != 422 || !strings.Contains(er.Error.Message, "Periksa NIM") {
		t.Fatalf("rejected = %d %+v", c, er)
	}
	o.mail.err = mail.ErrThrottled
	if c := o.call("POST", path, k, api.InviteRequest{NIM: "232410101093"}, nil); c != 429 {
		t.Fatalf("throttled = %d", c)
	}
	o.mail.err = nil
	var list api.List[api.Invitation]
	o.call("GET", path, k, nil, &list)
	for _, i := range list.Items {
		if i.NIM == "232410101093" {
			t.Fatalf("refused invitation was kept: %+v", i)
		}
	}
}

// snsSigner signs SNS messages with a self-signed certificate that the
// server's SNS client is made to fetch.
type snsSigner struct {
	key     *rsa.PrivateKey
	pem     []byte
	certURL string
}

func newSNSSigner(t *testing.T) *snsSigner {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "sns.amazonaws.com"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return &snsSigner{key: key, pem: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		certURL: "https://sns.ap-southeast-2.amazonaws.com/SimpleNotificationService-test.pem"}
}

func (s *snsSigner) sign(t *testing.T, m snsMessage) snsMessage {
	m.SignatureVersion, m.SigningCertURL = "2", s.certURL
	sum := sha256.Sum256([]byte(m.signedText()))
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	m.Signature = base64.StdEncoding.EncodeToString(sig)
	return m
}

// A permanent bounce reported by SES marks the pending invitation, until it
// is sent again; unsigned or foreign-topic posts change nothing.
func TestSESBounceMarksInvitation(t *testing.T) {
	o := newOrg(t)
	k, _ := o.kadiv("mediatek", "ses-kadiv@himasif.test")
	path := o.scope(o.period.ID, "mediatek") + "/invitations"
	topic := "arn:aws:sns:ap-southeast-2:123456789012:space-ses-events"
	signer := newSNSSigner(t)
	o.srv.sns.fetch = func(_ context.Context, u string) ([]byte, error) {
		if u != signer.certURL {
			t.Fatalf("fetched %s", u)
		}
		return signer.pem, nil
	}
	confirmed := ""
	o.srv.sns.confirm = func(_ context.Context, u string) error { confirmed = u; return nil }

	// Off until configured.
	if c := o.call("POST", "/api/mail/ses-events", "", "{}", nil); c != 404 {
		t.Fatalf("unconfigured = %d", c)
	}
	o.srv.cfg.Mail.SNSTopicARN = topic

	var inv api.Invitation
	if c := o.call("POST", path, k, api.InviteRequest{NIM: "232410101099"}, &inv); c != 201 {
		t.Fatal(c)
	}
	sub := signer.sign(t, snsMessage{Type: "SubscriptionConfirmation", MessageID: "m1", Token: "tok", TopicArn: topic, Message: "confirm",
		SubscribeURL: "https://sns.ap-southeast-2.amazonaws.com/?Action=ConfirmSubscription&Token=tok", Timestamp: "2026-11-10T01:00:00.000Z"})
	if c := o.call("POST", "/api/mail/ses-events", "", sub, nil); c != 200 || confirmed != sub.SubscribeURL {
		t.Fatalf("subscription = %d confirmed %q", c, confirmed)
	}

	body, _ := json.Marshal(map[string]any{"notificationType": "Bounce", "bounce": map[string]any{"bounceType": "Permanent", "bounceSubType": "General",
		"bouncedRecipients": []map[string]string{{"emailAddress": "232410101099@mail.unej.ac.id"}}}})
	note := signer.sign(t, snsMessage{Type: "Notification", MessageID: "m2", TopicArn: topic, Message: string(body), Timestamp: "2026-11-10T01:01:00.000Z"})

	tampered := note
	tampered.Message = strings.Replace(note.Message, "Permanent", "Transient", 1)
	if c := o.call("POST", "/api/mail/ses-events", "", tampered, nil); c != 403 {
		t.Fatalf("tampered = %d", c)
	}
	foreign := note
	foreign.TopicArn = "arn:aws:sns:ap-southeast-2:123456789012:other"
	if c := o.call("POST", "/api/mail/ses-events", "", foreign, nil); c != 403 {
		t.Fatalf("foreign topic = %d", c)
	}
	bounced := func() *api.Invitation {
		var list api.List[api.Invitation]
		o.call("GET", path, k, nil, &list)
		for _, i := range list.Items {
			if i.ID == inv.ID {
				return &i
			}
		}
		t.Fatal("invitation missing")
		return nil
	}
	if b := bounced(); b.BouncedAt != nil {
		t.Fatalf("bounced before a valid report: %+v", b)
	}
	if c := o.call("POST", "/api/mail/ses-events", "", note, nil); c != 200 {
		t.Fatalf("notification = %d", c)
	}
	if b := bounced(); b.BouncedAt == nil || !strings.Contains(b.BounceReason, "tidak sampai") {
		t.Fatalf("not marked: %+v", b)
	}
	if c := o.call("POST", path+"/"+inv.ID.String()+"/resend", k, nil, nil); c != 200 {
		t.Fatalf("resend = %d", c)
	}
	if b := bounced(); b.BouncedAt != nil || b.BounceReason != "" {
		t.Fatalf("resend kept the bounce: %+v", b)
	}
}

// Plan §8 option a: "Tautkan WhatsApp" opens the bot's chat with the code;
// the member sends it; the bot links that number, replies to it only, and
// Space shows the masked number. A number links one account only.
func TestWALinkByDirectMessage(t *testing.T) {
	o := newTaskOrg(t)
	ctx := context.Background()
	scope := o.scope(o.period.ID, "mediatek")
	var status api.WALinkStatus
	if c := o.call("GET", scope+"/whatsapp-link", o.member, nil, &status); c != 200 || status.Linked || status.BotReady {
		t.Fatalf("before a bot = %d %+v", c, status)
	}
	cid := id.New()
	if _, err := o.st.DB().Exec(ctx, `insert into wa_connections(id,period_id,division_id,display_name,enabled,status,account_jid,created_by,updated_at) values($1,$2,$3,'Bot Mediatek',true,'connected','6281111111111@s.whatsapp.net',$4,now())`,
		cid, o.period.ID, o.div["mediatek"].ID, o.memberID); err != nil {
		t.Fatal(err)
	}
	o.call("GET", scope+"/whatsapp-link", o.member, nil, &status)
	if !status.BotReady || status.Linked {
		t.Fatalf("with a bot %+v", status)
	}
	var link api.WALinkChallenge
	if c := o.call("POST", scope+"/whatsapp-link", o.member, nil, &link); c != 201 {
		t.Fatal(c)
	}
	if link.DeepLink != "https://wa.me/6281111111111?text="+link.Code || time.Until(link.ExpiresAt) < 14*time.Minute {
		t.Fatalf("challenge %+v", link)
	}

	sender := "6282222222222@s.whatsapp.net"
	dm := func(text string) id.ID {
		t.Helper()
		e := whatsapp.Event{Type: "message", SenderJID: sender, Text: text}
		body, state, reason := waDirectInbound(o.srv, e, true)
		iid := id.New()
		if _, err := o.st.DB().Exec(ctx, `insert into wa_inbound(id,connection_id,group_id,group_jid,sender_jid,message_id,envelope_id,revision,kind,body,received_at,state,error) values($1,$2,null,'',$3,$4,$4,'r','message',$5,now(),$6,$7)`,
			iid, cid, sender, iid.String(), body, state, reason); err != nil {
			t.Fatal(err)
		}
		for {
			found, err := o.srv.processWACommand(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if !found {
				return iid
			}
		}
	}
	// Anything else sent to the bot privately is dropped, text and all.
	chat := dm("halo bot, tugasku apa?")
	var body, state string
	o.st.DB().QueryRow(ctx, `select body,state from wa_inbound where id=$1`, chat).Scan(&body, &state)
	if body != "" || state != "ignored" {
		t.Fatalf("plain DM kept: %q %s", body, state)
	}

	in := dm(link.Code)
	o.st.DB().QueryRow(ctx, `select state from wa_inbound where id=$1`, in).Scan(&state)
	if state != "applied" {
		t.Fatalf("code DM state %s", state)
	}
	var linked id.ID
	if err := o.st.DB().QueryRow(ctx, `select user_id from wa_identities where period_id=$1 and jid=$2`, o.period.ID, sender).Scan(&linked); err != nil || linked != o.memberID {
		t.Fatalf("identity %v %v", linked, err)
	}
	var group *id.ID
	var source, target string
	var payload []byte
	if err := o.st.DB().QueryRow(ctx, `select group_id,source_kind,target_name,payload from wa_outbox where source_id=$1`, in).Scan(&group, &source, &target, &payload); err != nil {
		t.Fatal(err)
	}
	var reply whatsapp.SendRequest
	json.Unmarshal(payload, &reply)
	if group != nil || source != "link_dm" || target != "Pesan pribadi" || reply.ToJID != sender || reply.GroupJID != "" || !strings.Contains(reply.Text, "PJ Mediatek") {
		t.Fatalf("reply group=%v source=%s target=%s %+v", group, source, target, reply)
	}
	o.call("GET", scope+"/whatsapp-link", o.member, nil, &status)
	if !status.Linked || status.Phone != "+62 822-••••-2222" || status.LinkedAt == nil {
		t.Fatalf("linked status %+v", status)
	}

	// The same number cannot link the kadiv's account as well.
	var other api.WALinkChallenge
	o.call("POST", scope+"/whatsapp-link", o.kadiv, nil, &other)
	refused := dm(other.Code)
	o.st.DB().QueryRow(ctx, `select state from wa_inbound where id=$1`, refused).Scan(&state)
	var n int
	o.st.DB().QueryRow(ctx, `select count(*) from wa_identities where period_id=$1 and jid=$2`, o.period.ID, sender).Scan(&n)
	if state != "failed" || n != 1 {
		t.Fatalf("second account on one number: state=%s identities=%d", state, n)
	}
	// A used code does not work twice.
	again := dm(link.Code)
	o.st.DB().QueryRow(ctx, `select state from wa_inbound where id=$1`, again).Scan(&state)
	if state != "failed" {
		t.Fatalf("reused code state %s", state)
	}

	if c := o.call("DELETE", scope+"/whatsapp-link", o.member, nil, nil); c != 204 {
		t.Fatalf("unlink = %d", c)
	}
	status = api.WALinkStatus{}
	o.call("GET", scope+"/whatsapp-link", o.member, nil, &status)
	if status.Linked {
		t.Fatalf("still linked %+v", status)
	}
}
