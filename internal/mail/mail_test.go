package mail

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/config"
)

func TestLetterRendersTextAndEscapedHTML(t *testing.T) {
	m := Letter{To: "a@b.test", Subject: "Undangan", Heading: "Kamu <diundang>", Paragraphs: []string{"Divisi Mediatek & Humas"},
		Button: &Button{Label: "Terima undangan", URL: "https://space.test/#/undangan?token=abc&x=1"}, Notes: []string{"Abaikan bila tidak merasa diundang."}}.Message()
	for _, want := range []string{"Divisi Mediatek & Humas", "Terima undangan:\nhttps://space.test/#/undangan?token=abc&x=1", "— Space HIMASIF"} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("text lacks %q:\n%s", want, m.Text)
		}
	}
	for _, want := range []string{"Kamu &lt;diundang&gt;", "Mediatek &amp; Humas", `href="https://space.test/#/undangan?token=abc&amp;x=1"`, "#fffdf8"} {
		if !strings.Contains(m.HTML, want) {
			t.Errorf("html lacks %q", want)
		}
	}
}

func TestComposeMultipart(t *testing.T) {
	from, _ := mail.ParseAddress("Space HIMASIF <noreply@himasif.id>")
	s := &smtpMailer{cfg: config.Mail{}, from: from}
	to, _ := mail.ParseAddress("a@b.test")
	body, err := s.compose(to, Message{To: "a@b.test", Subject: "Uji", Text: "teks\nbaris", HTML: "<p>html</p>"})
	if err != nil {
		t.Fatal(err)
	}
	raw := string(body)
	if !strings.Contains(raw, "multipart/alternative; boundary=\"space-") || !strings.Contains(raw, "Content-Type: text/plain") || !strings.Contains(raw, "Content-Type: text/html") || !strings.Contains(raw, "teks\r\nbaris") {
		t.Fatalf("multipart:\n%s", raw)
	}
	plain, _ := s.compose(to, Message{To: "a@b.test", Subject: "Uji", Text: "teks"})
	if strings.Contains(string(plain), "multipart") || !strings.Contains(string(plain), `text/plain; charset="utf-8"`) {
		t.Fatalf("plain:\n%s", plain)
	}
}

func TestRejectedOnlyForPermanentReplies(t *testing.T) {
	var re *RejectedError
	if err := rejected(fmt.Errorf("mail: RCPT TO: %w", &textproto.Error{Code: 554, Msg: "Message rejected"})); !errors.As(err, &re) || re.Code != 554 {
		t.Fatalf("554 = %v", err)
	}
	if err := rejected(fmt.Errorf("mail: RCPT TO: %w", &textproto.Error{Code: 451, Msg: "try later"})); errors.As(err, &re) {
		t.Fatalf("451 counted as rejected: %v", err)
	}
}

type countMailer struct{ n int }

func (c *countMailer) Send(context.Context, Message) error { c.n++; return nil }

func TestLimitedSlidesOverAMinute(t *testing.T) {
	next := &countMailer{}
	now := time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)
	l := Limited(next, 2).(*limited)
	l.now = func() time.Time { return now }
	msg := Message{To: "a@b.test", Subject: "s", Text: "t"}
	for i := 0; i < 2; i++ {
		if err := l.Send(context.Background(), msg); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Send(context.Background(), msg); !errors.Is(err, ErrThrottled) {
		t.Fatalf("third in a minute = %v", err)
	}
	now = now.Add(61 * time.Second)
	if err := l.Send(context.Background(), msg); err != nil || next.n != 3 {
		t.Fatalf("after a minute = %v, sent %d", err, next.n)
	}
	if Limited(next, 0) != Mailer(next) {
		t.Fatal("0 should not limit")
	}
}
