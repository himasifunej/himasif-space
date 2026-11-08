package server

import (
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/filestore"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/jackc/pgx/v5"
)

// Limits of task attachments (Tahap 6b, approved 9 Nov 2026). The size of
// one file is SPACE_ATTACHMENT_MAX_BYTES (20 MB by default).
const (
	maxTaskAttachments  = 20
	taskAttachmentQuota = int64(2) << 30 // per period, active files
)

// taskAttachmentTypes are the content types, detected from the file's own
// bytes, a task may carry. Office documents are zip containers.
var taskAttachmentTypes = []string{"image/png", "image/jpeg", "image/gif", "image/webp", "application/pdf", "text/plain", "application/zip"}

// taskAttachmentDenied are names refused whatever their bytes look like:
// pages and scripts a browser could run, and programs.
var taskAttachmentDenied = []string{".html", ".htm", ".xhtml", ".svg", ".svgz", ".xml", ".js", ".mjs", ".exe", ".msi", ".bat", ".cmd", ".com", ".scr", ".ps1", ".sh", ".jar", ".apk", ".dll", ".vbs"}

// officeTypes name the zip containers that are Office documents.
var officeTypes = map[string]string{
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
}

// taskAttachmentType returns the type to store for a detected type and a
// name, or "" when the file is not allowed.
func taskAttachmentType(detected, name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if slices.Contains(taskAttachmentDenied, ext) {
		return ""
	}
	base := strings.TrimSpace(strings.SplitN(detected, ";", 2)[0])
	if !slices.Contains(taskAttachmentTypes, base) {
		return ""
	}
	if base == "application/zip" {
		if office, ok := officeTypes[ext]; ok {
			return office
		}
	}
	if base == "text/plain" {
		return "text/plain; charset=utf-8"
	}
	return base
}

// errAttachmentLimit marks a refusal decided before anything referenced
// the uploaded blob, so the blob can be dropped.
type errAttachmentLimit struct{ err error }

func (e errAttachmentLimit) Error() string { return e.err.Error() }
func (e errAttachmentLimit) Unwrap() error { return e.err }

func taskAttachmentUploadName(r *http.Request) (string, error) {
	name, err := text(r.URL.Query().Get("name"), "nama berkas", 1, 300)
	if err != nil {
		return "", err
	}
	if strings.ContainsAny(name, "/\\\r\n") || name == "." || name == ".." {
		return "", badRequest("Gunakan nama berkas tanpa direktori.")
	}
	if slices.Contains(taskAttachmentDenied, strings.ToLower(filepath.Ext(name))) {
		return "", fail(http.StatusUnsupportedMediaType, api.CodeBadRequest, "Tipe berkas ini tidak dapat dilampirkan ke tugas.")
	}
	return name, nil
}

// taskAttachmentAllowed checks who may add files and the task's limits.
func (s *Server) taskAttachmentAllowed(r *http.Request, q store.Querier, period api.Period, t *store.TaskRow, size int64) error {
	if !t.Permissions.Manage && !t.Permissions.Progress {
		return errForbidden
	}
	if t.Progress == api.TaskCancelled {
		return conflict("Tugas ini sudah dibatalkan.")
	}
	count, used, err := s.store.TaskAttachmentUsage(r.Context(), q, t.ID, period.ID)
	if err != nil {
		return err
	}
	if count >= maxTaskAttachments {
		return errAttachmentLimit{conflict("Tugas ini sudah memiliki 20 lampiran. Hapus salah satu dulu.")}
	}
	if size > 0 && used+size > taskAttachmentQuota {
		return errAttachmentLimit{fail(http.StatusRequestEntityTooLarge, api.CodeBadRequest, "Kuota lampiran periode ini (2 GB) sudah penuh.")}
	}
	return nil
}

// uploadTaskAttachment receives a file for a task: PUT with the raw bytes,
// the attachment ID chosen by the client so a retry is idempotent.
func (s *Server) uploadTaskAttachment(w http.ResponseWriter, r *http.Request, p *principal) error {
	aid, err := pathID(r, "attachmentId")
	if err != nil || aid == id.Nil {
		return badRequest("Identitas unggah tidak valid.")
	}
	name, err := taskAttachmentUploadName(r)
	if err != nil {
		return err
	}
	if s.cfg.AttachmentDir == "" {
		return fail(503, api.CodeUnavailable, "Penyimpanan lampiran belum dikonfigurasi.")
	}
	if r.Body == nil {
		return badRequest("Berkas belum dikirim.")
	}
	if r.ContentLength > s.attachmentLimit() {
		return fail(413, api.CodeBadRequest, "Berkas melampaui batas ukuran unggah.")
	}
	if err = s.withTask(r, p, true, func(q store.Querier, period api.Period, t *store.TaskRow) error {
		return s.taskAttachmentAllowed(r, q, period, t, max(r.ContentLength, 0))
	}); err != nil {
		var limit errAttachmentLimit
		if errors.As(err, &limit) {
			return limit.err
		}
		return err
	}
	// No transaction is held while a slow upload arrives; permissions and
	// limits are checked again before the file is published.
	blob, err := filestore.Put(s.cfg.AttachmentDir, r.Body, s.attachmentLimit())
	if err != nil {
		var oversized *http.MaxBytesError
		if errors.As(err, &oversized) {
			return fail(413, api.CodeBadRequest, "Berkas melampaui batas ukuran unggah.")
		}
		return err
	}
	contentType := taskAttachmentType(blob.ContentType, name)
	if contentType == "" {
		_ = filestore.Remove(s.cfg.AttachmentDir, blob.Key)
		return fail(http.StatusUnsupportedMediaType, api.CodeBadRequest, "Tipe berkas ini tidak dapat dilampirkan ke tugas. Gunakan gambar, PDF, teks, dokumen Office atau zip.")
	}
	var out api.TaskAttachment
	duplicate := false
	err = s.withTask(r, p, true, func(q store.Querier, period api.Period, t *store.TaskRow) error {
		if _, err := q.Exec(r.Context(), `select pg_advisory_xact_lock(hashtextextended($1,0))`, "task-attachment-upload:"+aid.String()); err != nil {
			return err
		}
		var taskID, actor id.ID
		var hash string
		var metadata []byte
		var removed *time.Time
		err := q.QueryRow(r.Context(), `select task_id,actor_id,sha256,metadata,removed_at from task_attachment_receipts where id=$1 for update`, aid).Scan(&taskID, &actor, &hash, &metadata, &removed)
		if err == nil {
			if taskID != t.ID || actor != p.User.ID || hash != blob.SHA256 {
				return errAttachmentLimit{conflict("Identitas unggah sudah dipakai untuk berkas lain.")}
			}
			if removed != nil {
				return errAttachmentLimit{conflict("Lampiran ini sudah dihapus. Pilih unggah baru bila ingin menambahkan kembali.")}
			}
			if err := json.Unmarshal(metadata, &out); err != nil {
				return err
			}
			if out.Name != name {
				return errAttachmentLimit{conflict("Nama pada percobaan ulang berbeda dari unggahan pertama.")}
			}
			duplicate = true
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err := s.taskAttachmentAllowed(r, q, period, t, blob.Size); err != nil {
			return err
		}
		var taken bool
		if err := q.QueryRow(r.Context(), `select exists(select 1 from task_attachments where id=$1)`, aid).Scan(&taken); err != nil {
			return err
		}
		if taken {
			return errAttachmentLimit{conflict("Identitas lampiran sudah dipakai; mulai unggah baru.")}
		}
		out = api.TaskAttachment{ID: aid, Name: name, ContentType: contentType, SizeBytes: blob.Size, URL: store.TaskAttachmentPath(t.ID, aid), AddedBy: taskPerson(p), CreatedAt: s.now().UTC().Truncate(time.Microsecond)}
		if err := s.store.AddTaskAttachment(r.Context(), q, t.ID, out, blob.Key, blob.SHA256); err != nil {
			return err
		}
		metadata, err = json.Marshal(out)
		if err != nil {
			return err
		}
		if _, err := q.Exec(r.Context(), `insert into task_attachment_receipts(id,task_id,actor_id,storage_key,sha256,metadata,created_at) values($1,$2,$3,$4,$5,$6,$7)`, aid, t.ID, p.User.ID, blob.Key, blob.SHA256, metadata, out.CreatedAt); err != nil {
			return err
		}
		return s.taskEvent(r, q, p, t, "task.attachment.upload", "", nil, out)
	})
	if err != nil {
		// A refusal decided inside the transaction referenced nothing. Any
		// other failure may follow a durable commit: keep the blob then.
		var limit errAttachmentLimit
		if errors.As(err, &limit) {
			_ = filestore.Remove(s.cfg.AttachmentDir, blob.Key)
			return limit.err
		}
		if errors.Is(err, errForbidden) {
			_ = filestore.Remove(s.cfg.AttachmentDir, blob.Key)
		}
		return err
	}
	if duplicate {
		_ = filestore.Remove(s.cfg.AttachmentDir, blob.Key)
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}

// downloadTaskAttachment sends a task's file to whoever may read the task,
// always as a download.
func (s *Server) downloadTaskAttachment(w http.ResponseWriter, r *http.Request, p *principal) error {
	aid, err := pathID(r, "attachmentId")
	if err != nil {
		return err
	}
	var file *os.File
	var f store.TaskAttachmentFile
	err = s.withTask(r, p, false, func(q store.Querier, _ api.Period, t *store.TaskRow) error {
		f, err = s.store.TaskAttachment(r.Context(), q, t.ID, aid)
		if err != nil {
			return notFound(err, errNotFound)
		}
		if s.cfg.AttachmentDir == "" {
			return fail(503, api.CodeUnavailable, "Penyimpanan lampiran belum tersedia.")
		}
		file, err = filestore.Open(s.cfg.AttachmentDir, f.Key)
		if err != nil {
			return fail(503, api.CodeUnavailable, "Berkas lampiran belum tersedia; hubungi pengelola untuk pemulihan.")
		}
		stat, err := file.Stat()
		if err != nil || !stat.Mode().IsRegular() || stat.Size() != f.Size {
			_ = file.Close()
			file = nil
			return fail(503, api.CodeUnavailable, "Berkas lampiran tidak utuh; pemulihan diperlukan.")
		}
		return nil
	})
	if err != nil {
		if file != nil {
			_ = file.Close()
		}
		return err
	}
	defer file.Close()
	w.Header().Set("Content-Type", f.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": f.Name}))
	w.Header().Set("ETag", `"`+f.Hash+`"`)
	w.Header().Set("X-Space-SHA256", f.Hash)
	http.ServeContent(w, r, f.Name, f.CreatedAt, file)
	return nil
}

// removeTaskAttachment removes a file from a task: its uploader or the
// task's manager may. The blob stays, as for notes.
func (s *Server) removeTaskAttachment(w http.ResponseWriter, r *http.Request, p *principal) error {
	aid, err := pathID(r, "attachmentId")
	if err != nil {
		return err
	}
	err = s.withTask(r, p, true, func(q store.Querier, _ api.Period, t *store.TaskRow) error {
		f, err := s.store.TaskAttachment(r.Context(), q, t.ID, aid)
		if err != nil {
			return notFound(err, errNotFound)
		}
		if f.AddedBy != p.User.ID && !t.Permissions.Manage {
			return errForbidden
		}
		if t.Progress == api.TaskCancelled {
			return conflict("Tugas ini sudah dibatalkan.")
		}
		if err := s.store.RemoveTaskAttachment(r.Context(), q, t.ID, aid, p.User.ID, s.now().UTC().Truncate(time.Microsecond)); err != nil {
			return notFound(err, errNotFound)
		}
		return s.taskEvent(r, q, p, t, "task.attachment.remove", "", map[string]any{"id": aid, "name": f.Name}, nil)
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
