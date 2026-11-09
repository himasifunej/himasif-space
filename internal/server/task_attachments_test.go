package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func taskAttachmentURL(task api.Task, aid id.ID, name string) string {
	return "/api/tasks/" + task.ID.String() + "/attachments/" + aid.String() + "/file?name=" + name
}

func withAttachmentDir(t *testing.T, o *taskOrg) {
	t.Helper()
	o.srv.cfg.AttachmentDir = t.TempDir()
	if err := os.Chmod(o.srv.cfg.AttachmentDir, 0700); err != nil {
		t.Fatal(err)
	}
	o.srv.cfg.AttachmentMaxBytes = 1024
}

// Tahap 6b: the PJ, a participant or the manager attaches files; whoever
// reads the task downloads them, always as a download; the uploader or the
// manager removes them; types, size, count and quota are bounded.
func TestTaskAttachments(t *testing.T) {
	o := newTaskOrg(t)
	withAttachmentDir(t, o)
	other, _ := o.join(o.kadiv, o.period.ID, "mediatek", "lain@himasif.test", "Anggota Lain")
	task := o.create(false)

	aid := id.New()
	path := taskAttachmentURL(task, aid, "catatan.txt")
	var added api.TaskAttachment
	if c := o.call("PUT", path, o.member, "isi lampiran", &added); c != 201 || added.ContentType != "text/plain; charset=utf-8" || added.SizeBytes != 12 || added.AddedBy.ID != o.memberID {
		t.Fatalf("PJ upload = %d %+v", c, added)
	}
	if c := o.call("PUT", path, o.member, "isi lampiran", nil); c != 201 {
		t.Fatalf("retry = %d", c)
	}
	if c := o.call("PUT", path, o.member, "isi berbeda", nil); c != 409 {
		t.Fatalf("id reuse = %d", c)
	}
	got := o.get(task, o.kadiv)
	if len(got.Attachments) != 1 || got.Attachments[0].ID != aid || got.Attachments[0].URL != strings.Split(path, "?")[0] {
		t.Fatalf("task attachments %+v", got.Attachments)
	}

	// Download: readers of the task only, as an attachment.
	req, _ := http.NewRequest("GET", o.url+got.Attachments[0].URL, nil)
	req.Header.Set("Authorization", "Bearer "+o.kadiv)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "isi lampiran" || !strings.HasPrefix(resp.Header.Get("Content-Disposition"), "attachment") || resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("download %d %q %v", resp.StatusCode, body, resp.Header)
	}
	if c := o.call("GET", got.Attachments[0].URL, o.peer, nil, nil); c != 403 && c != 404 {
		t.Fatalf("other division downloaded: %d", c)
	}

	// Who may add and remove.
	if c := o.call("PUT", taskAttachmentURL(task, id.New(), "lain.txt"), other, "bukan PJ", nil); c != 403 {
		t.Fatalf("non-PJ member upload = %d", c)
	}
	if c := o.call("DELETE", "/api/tasks/"+task.ID.String()+"/attachments/"+aid.String(), other, nil, nil); c != 403 {
		t.Fatalf("non-uploader delete = %d", c)
	}

	// Types and size.
	for name, content := range map[string]string{
		"halaman.html": "<html><body>x</body></html>",
		"gambar.svg":   "<svg xmlns='http://www.w3.org/2000/svg'></svg>",
		"program.bin":  "\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00",
	} {
		if c := o.call("PUT", taskAttachmentURL(task, id.New(), name), o.member, content, nil); c != 415 {
			t.Errorf("%s accepted: %d", name, c)
		}
	}
	var png api.TaskAttachment
	if c := o.call("PUT", taskAttachmentURL(task, id.New(), "poster.png"), o.kadiv, "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR", &png); c != 201 || png.ContentType != "image/png" {
		t.Fatalf("png = %d %+v", c, png)
	}
	if c := o.call("PUT", taskAttachmentURL(task, id.New(), "besar.txt"), o.member, strings.Repeat("a", 2000), nil); c != 413 {
		t.Fatalf("oversized = %d", c)
	}

	// Removal: the uploader removes theirs; a removed upload cannot return.
	if c := o.call("DELETE", "/api/tasks/"+task.ID.String()+"/attachments/"+aid.String(), o.member, nil, nil); c != 204 {
		t.Fatalf("uploader delete = %d", c)
	}
	if c := o.call("DELETE", "/api/tasks/"+task.ID.String()+"/attachments/"+aid.String(), o.member, nil, nil); c != 404 {
		t.Fatalf("second delete = %d", c)
	}
	if c := o.call("PUT", path, o.member, "isi lampiran", nil); c != 409 {
		t.Fatalf("removed upload came back: %d", c)
	}
	if c := o.call("GET", strings.Split(path, "?")[0], o.kadiv, nil, nil); c != 404 {
		t.Fatalf("removed file downloadable: %d", c)
	}
	// The manager removes someone else's file.
	if c := o.call("DELETE", "/api/tasks/"+task.ID.String()+"/attachments/"+png.ID.String(), o.kadiv, nil, nil); c != 204 {
		t.Fatalf("manager delete = %d", c)
	}
	if got := o.get(task, o.member); len(got.Attachments) != 0 {
		t.Fatalf("removed files still listed: %+v", got.Attachments)
	}
}

func TestTaskAttachmentLimits(t *testing.T) {
	o := newTaskOrg(t)
	withAttachmentDir(t, o)
	task := o.create(false)
	for i := 0; i < maxTaskAttachments; i++ {
		if c := o.call("PUT", taskAttachmentURL(task, id.New(), fmt.Sprintf("f%d.txt", i)), o.member, fmt.Sprintf("berkas %d", i), nil); c != 201 {
			t.Fatalf("upload %d = %d", i, c)
		}
	}
	if c := o.call("PUT", taskAttachmentURL(task, id.New(), "lebih.txt"), o.member, "satu lagi", nil); c != 409 {
		t.Fatalf("21st attachment = %d", c)
	}

	// The period's quota counts the active files of all its tasks.
	other := o.create(false)
	if _, err := o.st.DB().Exec(context.Background(), `update task_attachments set size_bytes=$2 where id=(select id from task_attachments where task_id=$1 limit 1)`, task.ID, taskAttachmentQuota-5); err != nil {
		t.Fatal(err)
	}
	if c := o.call("PUT", taskAttachmentURL(other, id.New(), "kuota.txt"), o.member, "melebihi kuota", nil); c != 413 {
		t.Fatalf("over quota = %d", c)
	}
}
