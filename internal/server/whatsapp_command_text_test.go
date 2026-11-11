package server

import (
	"testing"
	"time"
)

func TestRenderWACommandText(t *testing.T) {
	v := waCommandValues{Sender: "Aruna Desain", Group: "Mediatek 2026", Bot: "Bot Mediatek", Args: "besok pagi",
		At: time.Date(2026, 11, 10, 23, 30, 0, 0, time.UTC), Zone: "Asia/Jakarta"}
	got := renderWACommandText("Siap {sender}! {date} {time} · {group} · {name}: {args} {unknown}", v)
	if want := "Siap Aruna Desain! 11 Nov 2026 06.30 · Mediatek 2026 · Bot Mediatek: besok pagi {unknown}"; got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
	if got := renderWACommandText("SIAP!", v); got != "SIAP!" {
		t.Fatalf("plain response changed: %q", got)
	}
}
