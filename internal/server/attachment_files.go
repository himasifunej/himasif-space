package server

import (
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/filestore"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/jackc/pgx/v5"
)

func attachmentUploadPath(r *http.Request) bool {
	if r.Method != http.MethodPut {
		return false
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	// Notes' and tasks' uploads (Tahap 6b) take the attachment size limit.
	if len(parts) != 6 || parts[0] != "api" || (parts[1] != "notes" && parts[1] != "tasks") || parts[3] != "attachments" || parts[5] != "file" {
		return false
	}
	if _, err := id.Parse(parts[2]); err != nil {
		return false
	}
	_, err := id.Parse(parts[4])
	return err == nil
}
func (s *Server) attachmentLimit() int64 {
	if s.cfg.AttachmentMaxBytes > 0 {
		return s.cfg.AttachmentMaxBytes
	}
	return 20 << 20
}
func (s *Server) uploadNoteAttachment(w http.ResponseWriter, r *http.Request, p *principal) error {
	aid, err := pathID(r, "attachmentId")
	if err != nil || aid == id.Nil {
		return badRequest("Identitas unggah tidak valid.")
	}
	name, err := text(r.URL.Query().Get("name"), "nama berkas", 1, 300)
	if err != nil {
		return err
	}
	if strings.ContainsAny(name, "/\\\r\n") || name == "." || name == ".." {
		return badRequest("Gunakan nama berkas tanpa direktori.")
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
	if err = s.withNote(r, p, true, func(q store.Querier, period api.Period, n *store.NoteRow) error {
		if !n.Permissions.Manage {
			return errForbidden
		}
		return nil
	}); err != nil {
		return err
	}
	// No period/account transaction is held while receiving a potentially slow
	// upload. Permissions and the session are checked again before publication.
	blob, err := filestore.Put(s.cfg.AttachmentDir, r.Body, s.attachmentLimit())
	if err != nil {
		var oversized *http.MaxBytesError
		if errors.As(err, &oversized) {
			return fail(413, api.CodeBadRequest, "Berkas melampaui batas ukuran unggah.")
		}
		return err
	}
	var out api.NoteAttachment
	duplicate := false
	err = s.withNote(r, p, true, func(q store.Querier, period api.Period, n *store.NoteRow) error {
		if !n.Permissions.Manage {
			return errForbidden
		}
		if _, err = q.Exec(r.Context(), `select pg_advisory_xact_lock(hashtextextended($1,0))`, "attachment-upload:"+aid.String()); err != nil {
			return err
		}
		var noteID, actor id.ID
		var hash string
		var metadata []byte
		var removed *time.Time
		err = q.QueryRow(r.Context(), `select note_id,actor_id,sha256,metadata,removed_at from attachment_upload_receipts where id=$1 for update`, aid).Scan(&noteID, &actor, &hash, &metadata, &removed)
		if err == nil {
			if noteID != n.ID || actor != p.User.ID || hash != blob.SHA256 {
				return conflict("Identitas unggah sudah dipakai untuk berkas lain.")
			}
			if removed != nil {
				return conflict("Lampiran ini sudah dihapus. Pilih unggah baru bila ingin menambahkan kembali.")
			}
			if err = json.Unmarshal(metadata, &out); err != nil {
				return err
			}
			if out.Name != name {
				return conflict("Nama pada percobaan ulang berbeda dari unggahan pertama.")
			}
			duplicate = true
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var taken bool
		if err = q.QueryRow(r.Context(), `select exists(select 1 from note_attachments where id=$1)`, aid).Scan(&taken); err != nil {
			return err
		}
		if taken {
			return conflict("Identitas lampiran sudah dipakai; mulai unggah baru.")
		}
		out = api.NoteAttachment{ID: aid, Name: name, URL: "/api/notes/" + n.ID.String() + "/attachments/" + aid.String() + "/file", Status: api.AttachmentReady, ContentType: blob.ContentType, SizeBytes: blob.Size, AddedBy: taskPerson(p), CreatedAt: s.now().UTC().Truncate(time.Microsecond), Stored: true, SHA256: blob.SHA256}
		if err = s.store.AddNoteAttachment(r.Context(), q, n.ID, out); err != nil {
			return err
		}
		if _, err = q.Exec(r.Context(), `update note_attachments set storage_key=$2,sha256=$3 where id=$1`, aid, blob.Key, blob.SHA256); err != nil {
			return err
		}
		metadata, err = json.Marshal(out)
		if err != nil {
			return err
		}
		if _, err = q.Exec(r.Context(), `insert into attachment_upload_receipts(id,note_id,actor_id,storage_key,sha256,metadata,created_at) values($1,$2,$3,$4,$5,$6,$7)`, aid, n.ID, p.User.ID, blob.Key, blob.SHA256, metadata, out.CreatedAt); err != nil {
			return err
		}
		return s.noteEvent(r, q, p, n, "note.attachment.upload", "", nil, out)
	})
	// A commit error may have happened after PostgreSQL durably committed. Keep
	// the immutable blob on failure; never erase a possibly referenced file.
	// Explicit maintenance may later collect proven unreferenced old blobs.
	if err != nil {
		return err
	}
	if duplicate {
		_ = filestore.Remove(s.cfg.AttachmentDir, blob.Key)
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}
func (s *Server) downloadNoteAttachment(w http.ResponseWriter, r *http.Request, p *principal) error {
	aid, err := pathID(r, "attachmentId")
	if err != nil {
		return err
	}
	var file *os.File
	var name, ct, hash string
	var size int64
	var created time.Time
	err = s.withNote(r, p, false, func(q store.Querier, period api.Period, n *store.NoteRow) error {
		var key id.ID
		err := q.QueryRow(r.Context(), `select storage_key,name,content_type,size_bytes,sha256,created_at from note_attachments where id=$1 and note_id=$2 and storage_key is not null`, aid, n.ID).Scan(&key, &name, &ct, &size, &hash, &created)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotFound
		}
		if err != nil {
			return err
		}
		if s.cfg.AttachmentDir == "" {
			return fail(503, api.CodeUnavailable, "Penyimpanan lampiran belum tersedia.")
		}
		file, err = filestore.Open(s.cfg.AttachmentDir, key)
		if err != nil {
			return fail(503, api.CodeUnavailable, "Berkas lampiran belum tersedia; hubungi pengelola untuk pemulihan.")
		}
		stat, err := file.Stat()
		if err != nil || !stat.Mode().IsRegular() || stat.Size() != size {
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
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("ETag", `"`+hash+`"`)
	w.Header().Set("X-Space-SHA256", hash)
	http.ServeContent(w, r, name, created, file)
	return nil
}
