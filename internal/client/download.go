package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
)

// download reads a bounded authenticated file without caching private exports
// or falling back to a stale offline copy. A rejected/partial response is never
// offered as a successful file.
func (c *Client) download(ctx context.Context, path, mediaType string, limit int64) ([]byte, error) {
	pinned := c.AccountSession()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(pinned.BaseURL, "/")+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", mediaType)
	if tok := pinned.token(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	hc := pinned.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: DefaultTimeout}
	}
	resp, err := hc.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, ctx.Err()
		}
		return nil, &UnreachableError{Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404 {
			pinned.clearOfflineAccess()
		}
		return nil, responseError(resp.StatusCode, data)
	}
	expected, _, expectedErr := mime.ParseMediaType(mediaType)
	received, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || expectedErr != nil || received != expected {
		return nil, fmt.Errorf("api: jenis berkas respons tidak sesuai")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("api: berkas terlalu besar; unduhan parsial dibuang")
	}
	return data, nil
}
