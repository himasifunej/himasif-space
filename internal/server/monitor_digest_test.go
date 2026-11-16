package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func TestDigestWeek(t *testing.T) {
	wib := time.FixedZone("WIB", 7*3600)
	from, to, due := digestWeek(time.Date(2026, 11, 17, 3, 0, 0, 0, time.UTC), wib) // Tue 10.00 WIB
	if !from.Equal(time.Date(2026, 11, 9, 0, 0, 0, 0, wib)) || !to.Equal(time.Date(2026, 11, 16, 0, 0, 0, 0, wib)) || !due.Equal(time.Date(2026, 11, 16, 7, 0, 0, 0, wib)) {
		t.Fatalf("week %v %v %v", from, to, due)
	}
	d := monitorDigest{From: from, To: to, Done: 14, Overdue: 8, AwaitingReview: 5, Meetings: 4, MeetingsNoNotes: 2, Link: "https://space/#/pemantauan",
		Divisions: []digestDivision{{"Mediatek", 6, 3}, {"Humas", 4, 1}, {"PSDM", 2, 2}, {"Kader", 2, 2}}}
	text := digestText(d, wib)
	for _, want := range []string{"*Ringkasan Space · 9–15 Nov*", "Selesai: 14 tugas (Mediatek 6, Humas 4, lainnya 4)", "Terlambat: 8 tugas, terbanyak Mediatek 3, PSDM 2, lainnya 3", "Menunggu tinjauan: 5 bukti", "notulen belum tertaut 2"} {
		if !strings.Contains(text, want) {
			t.Errorf("digest lacks %q:\n%s", want, text)
		}
	}
}

// MON-08: Monday 07.00 WIB, once per week, to monitors who did not turn it
// off, by email and by WhatsApp direct message.
func TestMonitorDigestSending(t *testing.T) {
	o := newMonitorOrg(t)
	ctx := context.Background()
	path := "/api/me/monitor-digest"
	var st api.MonitorDigestSettings
	if c := o.call("GET", path, o.bph, nil, &st); c != 200 || !st.Enabled || st.Channel != api.DigestBoth || st.Monitor || st.Preview != "" {
		t.Fatalf("default = %d %+v", c, st)
	}
	if c := o.call("GET", path, o.admin, nil, nil); c != 403 {
		t.Fatalf("admin = %d", c)
	}
	if c := o.call("PUT", path, o.bph, api.MonitorDigestRequest{Enabled: true, Channel: "sms"}, nil); c != 400 {
		t.Fatalf("bad channel = %d", c)
	}
	o.grant()
	if c := o.call("GET", path, o.bph, nil, &st); c != 200 || !st.Monitor || !strings.Contains(st.Preview, "Ringkasan Space") || st.WhatsAppLinked {
		t.Fatalf("monitor settings = %d %+v", c, st)
	}

	// A linked number and an enabled bot of the period.
	conn := id.New()
	if _, err := o.st.DB().Exec(ctx, `insert into wa_connections(id,period_id,division_id,display_name,enabled,status,account_jid,created_by,updated_at) values($1,$2,$3,'Bot Humas',true,'connected','6281111111111@s.whatsapp.net',$4,now())`,
		conn, o.period.ID, o.div["humas"].ID, o.bphID); err != nil {
		t.Fatal(err)
	}
	if _, err := o.st.DB().Exec(ctx, `insert into wa_identities(period_id,division_id,user_id,jid,verified_by,verified_at) values($1,$2,$3,'6281234567890@s.whatsapp.net',$3,now())`,
		o.period.ID, o.div["bph"].ID, o.bphID); err != nil {
		t.Fatal(err)
	}

	wib := time.FixedZone("WIB", 7*3600)
	sunday := time.Date(2026, 11, 22, 20, 0, 0, 0, wib)
	monday := time.Date(2026, 11, 23, 7, 5, 0, 0, wib)
	base := o.mail.count("ketum@himasif.test") // the invitation's emails
	mails := func() int { return o.mail.count("ketum@himasif.test") - base }
	run := func(at time.Time) {
		t.Helper()
		defer func() { o.srv.now = time.Now }()
		o.srv.now = func() time.Time { return at }
		if err := o.srv.runMonitorDigests(ctx); err != nil {
			t.Fatal(err)
		}
	}
	count := func(sql string) int {
		t.Helper()
		var n int
		if err := o.st.DB().QueryRow(ctx, sql, o.bphID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	outbox := `select count(*) from wa_outbox where actor_id = $1 and source_kind = 'monitor_digest' and group_id is null`
	runs := `select count(*) from monitor_digest_runs where user_id = $1`

	run(sunday)
	if mails() != 0 || count(runs) != 0 {
		t.Fatal("sent before Monday 07.00")
	}
	run(monday)
	run(monday.Add(time.Hour)) // a later tick or another instance
	if n := mails(); n != 1 || count(runs) != 1 || count(outbox) != 1 {
		t.Fatalf("emails %d runs %d outbox %d", n, count(runs), count(outbox))
	}
	m := o.mail.last(t, "ketum@himasif.test")
	if !strings.Contains(m.Subject, "Ringkasan Space · 16–22 Nov 2026") || !strings.Contains(m.HTML, "Mediatek") || strings.Contains(m.Text, "Keputusan rapat") {
		t.Fatalf("email %q\n%s", m.Subject, m.Text)
	}

	// Turned off: the next Monday sends nothing.
	if c := o.call("PUT", path, o.bph, api.MonitorDigestRequest{Enabled: false, Channel: api.DigestEmail}, &st); c != 200 || st.Enabled {
		t.Fatalf("opt out = %d %+v", c, st)
	}
	run(monday.AddDate(0, 0, 7))
	if mails() != 1 || count(runs) != 1 {
		t.Fatal("sent after opting out")
	}
	// On again, by email only; a digest missed on Monday is not sent on Tuesday.
	o.call("PUT", path, o.bph, api.MonitorDigestRequest{Enabled: true, Channel: api.DigestEmail}, nil)
	run(monday.AddDate(0, 0, 15))
	if mails() != 1 {
		t.Fatal("sent a missed digest on Tuesday")
	}
	run(monday.AddDate(0, 0, 21))
	if mails() != 2 || count(outbox) != 1 {
		t.Fatalf("email-only digest: emails %d outbox %d", mails(), count(outbox))
	}
	// Closing the period ends the grant, and the digest with it.
	if c := o.call("POST", "/api/periods/"+o.period.ID.String()+"/close", o.admin, nil, nil); c != 200 {
		t.Fatalf("close = %d", c)
	}
	run(monday.AddDate(0, 0, 28))
	if mails() != 2 {
		t.Fatal("digest after the period closed")
	}
}
