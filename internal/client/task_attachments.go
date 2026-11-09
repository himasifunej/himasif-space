package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// UploadTaskAttachment sends a file to a task (Tahap 6b). The attachment ID
// is chosen by the caller: retrying with the same ID and bytes is safe.
func (c *Client) UploadTaskAttachment(ctx context.Context, tid, aid id.ID, name string, data []byte) (api.TaskAttachment, error) {
	var out api.TaskAttachment
	pinned := c.AccountSession()
	path := taskPath(tid) + "/attachments/" + aid.String() + "/file?" + url.Values{"name": {name}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, pinned.BaseURL+path, bytes.NewReader(data))
	if err != nil {
		return out, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Accept", "application/json")
	if tok := pinned.token(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	hc := pinned.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: DefaultTimeout}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return out, &UnreachableError{Err: err}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return out, err
	}
	if len(body) > maxBody {
		return out, fmt.Errorf("Respons unggahan terlalu besar; periksa ulang dengan identitas unggah yang sama.")
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return out, responseError(resp.StatusCode, body)
	}
	if err = json.Unmarshal(body, &out); err != nil {
		return out, err
	}
	if out.ID != aid || out.Name != name || out.SizeBytes != int64(len(data)) {
		return api.TaskAttachment{}, fmt.Errorf("Konfirmasi unggahan tidak cocok; periksa ulang sebelum menganggap berkas tersimpan.")
	}
	return out, nil
}

// DownloadTaskAttachment fetches a task's file, checking its type and size.
func (c *Client) DownloadTaskAttachment(ctx context.Context, tid id.ID, a api.TaskAttachment) ([]byte, error) {
	data, err := c.download(ctx, taskPath(tid)+"/attachments/"+a.ID.String()+"/file", a.ContentType, 100<<20)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != a.SizeBytes {
		return nil, fmt.Errorf("Ukuran lampiran tidak cocok; berkas tidak ditawarkan untuk disimpan.")
	}
	return data, nil
}

// RemoveTaskAttachment removes a file from a task.
func (c *Client) RemoveTaskAttachment(ctx context.Context, tid, aid id.ID) error {
	return c.do(ctx, http.MethodDelete, taskPath(tid)+"/attachments/"+aid.String(), nil, nil)
}
