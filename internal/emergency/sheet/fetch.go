package sheet

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Polling: every 3 minutes, plus up to a minute at random so that desktops
// behind one campus address do not ask together. A failure doubles the
// wait up to 15 minutes; a 429 waits at least what Google asks.
const (
	PollEvery  = 3 * time.Minute
	PollJitter = time.Minute
	PollMax    = 15 * time.Minute
)

// FetchError is a fetch that brought no usable copy. Nothing local changes.
type FetchError struct {
	Kind       string // FailNetwork, FailHTML, FailRate, FailServer, FailHTTP, FailFormat
	Status     int
	RetryAfter time.Duration
	Err        error
}

const (
	FailNetwork = "network"
	FailHTML    = "html"
	FailRate    = "rate"
	FailServer  = "server"
	FailHTTP    = "http"
	FailFormat  = "format"
)

func (e *FetchError) Error() string {
	switch e.Kind {
	case FailNetwork:
		return "Google tidak terjangkau dari perangkat ini"
	case FailHTML:
		return "Google mengirim halaman, bukan sheet: publikasi mungkin dihentikan"
	case FailRate:
		return "Google meminta menunggu (429)"
	case FailServer:
		return "Google sedang gangguan (" + strconv.Itoa(e.Status) + ")"
	case FailHTTP:
		if e.Status == http.StatusNotFound {
			return "Link publish tidak ditemukan: publikasi mungkin dihentikan"
		}
		return "Google menjawab " + strconv.Itoa(e.Status)
	}
	if e.Err != nil {
		return "Sheet tidak terbaca: " + e.Err.Error()
	}
	return "Sheet tidak terbaca"
}

func (e *FetchError) Unwrap() error { return e.Err }

// Fetch gets and reads the published workbook at url.
func Fetch(ctx context.Context, hc *http.Client, url string, loc *time.Location) (Snapshot, error) {
	if hc == nil {
		hc = &http.Client{Timeout: 45 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Snapshot{}, &FetchError{Kind: FailNetwork, Err: err}
	}
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := hc.Do(req)
	if err != nil {
		return Snapshot{}, &FetchError{Kind: FailNetwork, Err: err}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxXLSX+1))
	if err != nil {
		return Snapshot{}, &FetchError{Kind: FailNetwork, Err: err}
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return Snapshot{}, &FetchError{Kind: FailRate, Status: resp.StatusCode, RetryAfter: retryAfter(resp.Header.Get("Retry-After"))}
	case resp.StatusCode >= 500:
		return Snapshot{}, &FetchError{Kind: FailServer, Status: resp.StatusCode}
	case isHTML(resp.Header.Get("Content-Type"), body):
		return Snapshot{}, &FetchError{Kind: FailHTML, Status: resp.StatusCode}
	case resp.StatusCode != http.StatusOK:
		return Snapshot{}, &FetchError{Kind: FailHTTP, Status: resp.StatusCode}
	}
	s, err := Parse(body, loc)
	if err != nil {
		return Snapshot{}, &FetchError{Kind: FailFormat, Status: resp.StatusCode, Err: err}
	}
	return s, nil
}

func isHTML(contentType string, body []byte) bool {
	if strings.HasPrefix(strings.ToLower(contentType), "text/html") {
		return true
	}
	head := bytes.ToLower(bytes.TrimSpace(body[:min(len(body), 64)]))
	return bytes.HasPrefix(head, []byte("<!doctype html")) || bytes.HasPrefix(head, []byte("<html"))
}

func retryAfter(v string) time.Duration {
	if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
		return time.Duration(min(n, 3600)) * time.Second
	}
	return 0
}

// NextDelay is the wait before the next fetch, after failures failed ones
// in a row (0 after a good one) and the last error. jitter is in [0, 1).
func NextDelay(failures int, err error, jitter float64) time.Duration {
	wait := PollEvery
	for i := 0; i < failures && wait < PollMax; i++ {
		wait *= 2
	}
	wait = min(wait, PollMax)
	var fe *FetchError
	if errors.As(err, &fe) && fe.Kind == FailRate && fe.RetryAfter > wait {
		wait = min(fe.RetryAfter, 2*PollMax)
	}
	return wait + time.Duration(jitter*float64(PollJitter))
}

// Lag says how old a copy is, as "data sheet per 13.34 (±7 menit)".
func Lag(clock, now time.Time) string {
	if clock.IsZero() {
		return "belum ada data sheet"
	}
	s := "data sheet per " + clock.Format("15.04")
	if d := now.Sub(clock); d >= 2*time.Minute {
		s += fmt.Sprintf(" (±%d menit lalu)", int(d.Round(time.Minute)/time.Minute))
	}
	return s
}
