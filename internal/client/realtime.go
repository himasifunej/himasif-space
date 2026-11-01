package client

import (
	"bufio"
	"context"
	"fmt"
	"github.com/himasifunej/himasif-space/internal/id"
	"net/http"
	"net/url"
	"strings"
)

// WatchChanges holds an account-pinned SSE stream. A separate HTTP client has
// no whole-response timeout: context cancellation closes the stream on route,
// session or application changes. Normal API requests keep their own timeout.
// Callback events are ready, heartbeat and change, never resource contents.
func (c *Client) WatchChanges(ctx context.Context, period, space id.ID, event func(string)) error {
	query := url.Values{"periodId": {period.String()}}
	if space != id.Nil {
		query.Set("spaceId", space.String())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+"/api/realtime?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token())
	req.Header.Set("Accept", "text/event-stream")
	h := http.Client{}
	if c.HTTP != nil {
		h = *c.HTTP
	}
	h.Timeout = 0
	response, err := h.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return &Error{Status: response.StatusCode, Message: "Pembaruan langsung belum tersedia."}
	}
	if !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		return fmt.Errorf("invalid realtime response")
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 1024), 64<<10)
	kind := ""
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event:") {
			kind = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		}
		if line == "" {
			if kind == "ready" || kind == "change" || kind == "heartbeat" {
				event(kind)
			}
			kind = ""
		}
	}
	if err = scanner.Err(); err != nil {
		return err
	}
	return fmt.Errorf("realtime disconnected")
}
