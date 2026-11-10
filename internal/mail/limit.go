package mail

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrThrottled is returned, without sending, when the server already sent
// its allowance of emails in the last minute.
var ErrThrottled = errors.New("mail: sending limit reached, try again in a minute")

// Limited bounds the emails m sends to perMinute in any sliding minute, all
// recipients together, so a burst of invitations stays within the provider's
// rate (Amazon SES: 1 per second in the sandbox). The per-address limit is
// the server's.
func Limited(m Mailer, perMinute int) Mailer {
	if perMinute <= 0 {
		return m
	}
	return &limited{next: m, max: perMinute, now: time.Now}
}

type limited struct {
	next Mailer
	max  int
	now  func() time.Time
	mu   sync.Mutex
	sent []time.Time
}

func (l *limited) Send(ctx context.Context, m Message) error {
	if !l.take() {
		return ErrThrottled
	}
	return l.next.Send(ctx, m)
}

func (l *limited) take() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	cutoff := now.Add(-time.Minute)
	keep := l.sent[:0]
	for _, t := range l.sent {
		if t.After(cutoff) {
			keep = append(keep, t)
		}
	}
	l.sent = keep
	if len(l.sent) >= l.max {
		return false
	}
	l.sent = append(l.sent, now)
	return true
}
