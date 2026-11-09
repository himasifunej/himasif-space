package server

import (
	"context"
	"strings"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
)

// Tahap 7d: the preview is the sender's own text for a draft: phrase,
// template, name and greeting, in the chosen language; it stores and
// queues nothing; only who manages WhatsApp may ask.
func TestWAPreview(t *testing.T) {
	o := newTaskOrg(t)
	path := o.scope(o.period.ID, "mediatek") + "/whatsapp/preview"
	count := func() int {
		var n int
		if err := o.st.DB().QueryRow(context.Background(), `select (select count(*) from wa_outbox)+(select count(*) from wa_connections)`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := count()

	req := api.WAPreviewRequest{Kind: "reminder", DisplayName: "Bot Mediatek", Greeting: "Halo Mediatek!", Templates: map[string]string{"reminder": "{greeting}\n⏰ {body}\n— {name}"}}
	var got api.WAPreview
	if c := o.call("POST", path, o.kadiv, req, &got); c != 200 || !got.Example {
		t.Fatalf("preview = %d %+v", c, got)
	}
	want := "Halo Mediatek!\n⏰ Pengingat: #T12 Poster open recruitment\n— Bot Mediatek"
	if got.Text != want {
		t.Fatalf("reminder preview\n%q\nwant\n%q", got.Text, want)
	}
	// Without the placeholders, name and greeting still lead the message,
	// as the sender does.
	req.Templates = nil
	got = api.WAPreview{}
	o.call("POST", path, o.kadiv, req, &got)
	if !strings.HasPrefix(got.Text, "Bot Mediatek\nHalo Mediatek!\n") {
		t.Fatalf("plain preview %q", got.Text)
	}
	req.Language = "en"
	got = api.WAPreview{}
	o.call("POST", path, o.kadiv, req, &got)
	if !strings.Contains(got.Text, "Reminder: #T12 Poster open recruitment") {
		t.Fatalf("english preview %q", got.Text)
	}
	for _, kind := range []string{"overdue", "ping", "reply", "weekly", "followup"} {
		got = api.WAPreview{}
		if c := o.call("POST", path, o.kadiv, api.WAPreviewRequest{Kind: kind, DisplayName: "Bot"}, &got); c != 200 || got.Text == "" {
			t.Errorf("%s preview = %d %q", kind, c, got.Text)
		}
	}

	if c := o.call("POST", path, o.kadiv, api.WAPreviewRequest{Kind: "reminder", Templates: map[string]string{"reminder": "tanpa isi"}}, nil); c != 400 {
		t.Fatalf("template without {body} = %d", c)
	}
	if c := o.call("POST", path, o.kadiv, api.WAPreviewRequest{Kind: "spam"}, nil); c != 400 {
		t.Fatalf("unknown kind = %d", c)
	}
	if c, _ := o.errCode("POST", path, o.member, req); c != 403 {
		t.Fatalf("member without WhatsApp rights = %d", c)
	}
	if count() != before {
		t.Fatal("preview stored or queued something")
	}
}
