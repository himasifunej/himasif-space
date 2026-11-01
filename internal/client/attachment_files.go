package client

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func (c *Client) UploadNoteAttachment(ctx context.Context, nid, aid id.ID, name string, data []byte) (api.NoteAttachment, error) {
	var out api.NoteAttachment
	pinned := c.AccountSession()
	path := notePath(nid) + "/attachments/" + aid.String() + "/file?" + url.Values{"name": {name}}.Encode()
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
		if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404 {
			pinned.clearOfflineAccess()
		}
		return out, responseError(resp.StatusCode, body)
	}
	if err = json.Unmarshal(body, &out); err != nil {
		return out, err
	}
	sum := sha256.Sum256(data)
	if out.ID != aid || out.Name != name || !out.Stored || out.Status != api.AttachmentReady || out.SizeBytes != int64(len(data)) || out.SHA256 != hex.EncodeToString(sum[:]) {
		return api.NoteAttachment{}, fmt.Errorf("Konfirmasi unggahan tidak cocok; periksa ulang sebelum menganggap berkas tersimpan.")
	}
	return out, nil
}
func (c *Client) DownloadNoteAttachment(ctx context.Context, nid id.ID, attachment api.NoteAttachment) ([]byte, error) {
	if !attachment.Stored || attachment.Status != api.AttachmentReady {
		return nil, fmt.Errorf("Lampiran bukan berkas tersimpan.")
	}
	data, err := c.download(ctx, notePath(nid)+"/attachments/"+attachment.ID.String()+"/file", attachment.ContentType, 100<<20)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	if int64(len(data)) != attachment.SizeBytes || hex.EncodeToString(sum[:]) != attachment.SHA256 {
		return nil, fmt.Errorf("Isi lampiran tidak cocok dengan checksum; berkas tidak ditawarkan untuk disimpan.")
	}
	return data, nil
}
