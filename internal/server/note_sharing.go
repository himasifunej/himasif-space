package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/jackc/pgx/v5"
)

// Catatan C6: "Hanya divisi ini" (the default) or "Publik lewat tautan".
// A public link is a random 32-byte token (base64url) in /n/<token>; the
// server keeps only its SHA-256. The page and its JSON are read-only,
// never indexed or cached, and show a whitelist: no user ID, email, NIM,
// WhatsApp number, role, comment, attachment or link into Space.

// publicTokenRe is the shape of a token; anything else is not looked up.
var publicTokenRe = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

// canShareNote: the owner division's kadiv, or the note's writer, while
// they may manage notes in an open period.
func canShareNote(manage bool, sc *scope, n *store.NoteRow, uid id.ID) bool {
	if !manage {
		return false
	}
	kadiv := sc.member != nil && sc.member.Role == api.RoleKadiv
	return kadiv || n.CreatedBy.ID == uid
}

// noteSharing fills what C6 adds to a note the caller may read.
func (s *Server) noteSharing(ctx context.Context, q store.Querier, n *store.NoteRow) error {
	link, err := s.store.NotePublicLink(ctx, q, n.ID)
	n.PublicLink = link
	if n.Tags == nil {
		n.Tags = []string{}
	}
	return err
}

func (s *Server) publicNoteURL(token string) string {
	return strings.TrimSuffix(s.cfg.PublicURL, "/") + "/n/" + token
}

// createNotePublicLink makes the note public, or replaces its link: the
// old link stops working at once. The URL is answered only now.
func (s *Server) createNotePublicLink(w http.ResponseWriter, r *http.Request, p *principal) error {
	var out api.NotePublicLinkCreated
	err := s.withNote(r, p, true, func(q store.Querier, period api.Period, n *store.NoteRow) error {
		if !n.Permissions.Share {
			return fail(http.StatusForbidden, api.CodeForbidden, "Hanya kadiv divisi atau penulis catatan yang bisa membagikan catatan ke publik.")
		}
		ctx, now := r.Context(), s.now().UTC()
		replaced, err := s.store.RevokeNotePublicLink(ctx, q, n.ID, p.User.ID, now)
		if err != nil {
			return err
		}
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			return err
		}
		token := base64.RawURLEncoding.EncodeToString(raw)
		hash := sha256.Sum256([]byte(token))
		if err := s.store.InsertNotePublicLink(ctx, q, n.ID, p.User.ID, hash[:], now); err != nil {
			return err
		}
		link, err := s.store.NotePublicLink(ctx, q, n.ID)
		if err != nil {
			return err
		}
		out = api.NotePublicLinkCreated{URL: s.publicNoteURL(token), Link: *link}
		action := "note.public_link.create"
		if replaced {
			action = "note.public_link.renew"
		}
		return s.record(r, q, p, store.AuditEvent{Action: action, TargetType: "note", TargetID: &n.ID, PeriodID: &period.ID})
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, out)
	return nil
}

// revokeNotePublicLink makes the note "Hanya divisi ini" again.
func (s *Server) revokeNotePublicLink(w http.ResponseWriter, r *http.Request, p *principal) error {
	err := s.withNote(r, p, true, func(q store.Querier, period api.Period, n *store.NoteRow) error {
		if !n.Permissions.Share {
			return fail(http.StatusForbidden, api.CodeForbidden, "Hanya kadiv divisi atau penulis catatan yang bisa mengubah akses catatan.")
		}
		revoked, err := s.store.RevokeNotePublicLink(r.Context(), q, n.ID, p.User.ID, s.now().UTC())
		if err != nil || !revoked {
			return err
		}
		return s.record(r, q, p, store.AuditEvent{Action: "note.public_link.revoke", TargetType: "note", TargetID: &n.ID, PeriodID: &period.ID})
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// setNoteTags replaces a note's tags. It takes no editor lock: tags are
// not the document.
func (s *Server) setNoteTags(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.NoteTagsRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	tags, err := api.NormalizeNoteTags(req.Tags)
	if err != nil {
		return badRequest(err.Error())
	}
	var out []string
	err = s.withNote(r, p, true, func(q store.Querier, period api.Period, n *store.NoteRow) error {
		if !n.Permissions.Manage {
			return errForbidden
		}
		if err := s.store.SetNoteTags(r.Context(), q, n.ID, tags); err != nil {
			return err
		}
		out = tags
		return s.record(r, q, p, store.AuditEvent{Action: "note.tags.set", TargetType: "note", TargetID: &n.ID, PeriodID: &period.ID,
			Before: map[string]any{"tags": n.Tags}, After: map[string]any{"tags": tags}})
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, api.NoteTagsRequest{Tags: out})
	return nil
}

// publicHeaders keep a public page out of search engines, caches and
// referrers.
func publicHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cache-Control", "private, no-store")
	h.Set("X-Content-Type-Options", "nosniff")
}

// publicNote loads what token opens. Every miss (bad shape, unknown,
// revoked, deleted, closed period, too many requests) is the same 404.
func (s *Server) publicNote(r *http.Request, token string) (api.PublicNote, bool) {
	if !publicTokenRe.MatchString(token) || !s.noteViewLimit.allow("ip:"+clientIP(r), s.now()) {
		return api.PublicNote{}, false
	}
	hash := sha256.Sum256([]byte(token))
	var out api.PublicNote
	err := s.store.Tx(r.Context(), func(q store.Querier) error {
		ctx := r.Context()
		n, err := s.store.PublicNoteByToken(ctx, q, hash[:], s.now().UTC())
		if err != nil {
			return err
		}
		out = api.PublicNote{Title: n.Title, Tags: n.Tags, Content: publicBlocks(n.Content, s.cfg.PublicURL), Version: n.Version, UpdatedAt: n.UpdatedAt}
		if out.Tags == nil {
			out.Tags = []string{}
		}
		if err := q.QueryRow(ctx, `select name from divisions where id=$1`, n.OwnerDivisionID).Scan(&out.Division); err != nil {
			return err
		}
		var m api.PublicMeeting
		err = q.QueryRow(ctx, `select title, starts_at, ends_at, time_zone, location from meetings where note_id=$1 and not cancelled limit 1`, n.ID).
			Scan(&m.Title, &m.StartsAt, &m.EndsAt, &m.TimeZone, &m.Location)
		if err == nil {
			out.Meeting = &m
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return nil
	})
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.log.Warn("public note", "err", err)
		}
		return api.PublicNote{}, false
	}
	return out, true
}

// publicBlocks keeps of a document what a stranger may read: text and
// structure. Mentions become their text, owners and task IDs of list items
// go, links stay only when they lead outside Space, and blocks this build
// does not know are left out.
func publicBlocks(blocks []api.NoteBlock, spaceURL string) []api.NoteBlock {
	out := []api.NoteBlock{}
	for _, b := range blocks {
		if b.Unsupported() {
			continue
		}
		c := api.NoteBlock{Type: b.Type, Level: b.Level, Text: b.Text, Columns: b.Columns, Variant: b.Variant, Minutes: b.Minutes}
		c.Spans = publicSpans(b.Spans, spaceURL)
		for _, it := range b.Items {
			c.Items = append(c.Items, api.NoteListItem{Spans: publicSpans(it.Spans, spaceURL), Checked: it.Checked, DueOn: it.DueOn, Minutes: it.Minutes})
		}
		for _, row := range b.Rows {
			var cells [][]api.Span
			for _, cell := range row {
				cells = append(cells, publicSpans(cell, spaceURL))
			}
			c.Rows = append(c.Rows, cells)
		}
		if len(b.Children) > 0 {
			c.Children = publicBlocks(b.Children, spaceURL)
		}
		out = append(out, c)
	}
	return out
}

func publicSpans(spans []api.Span, spaceURL string) []api.Span {
	if spans == nil {
		return nil
	}
	out := make([]api.Span, 0, len(spans))
	for _, sp := range spans {
		c := api.Span{Text: sp.Text, Bold: sp.Bold}
		if publicLinkAllowed(sp.Link, spaceURL) {
			c.Link = sp.Link
		}
		out = append(out, c)
	}
	return out
}

// publicLinkAllowed: an absolute http(s) link to another site.
func publicLinkAllowed(link, spaceURL string) bool {
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return false
	}
	if su, err := url.Parse(spaceURL); err == nil && su.Host != "" && strings.EqualFold(su.Host, u.Host) {
		return false
	}
	return true
}

// getPublicNoteJSON answers /api/public/notes/{token}, which the public
// page polls so readers see autosaves.
func (s *Server) getPublicNoteJSON(w http.ResponseWriter, r *http.Request) {
	publicHeaders(w)
	n, ok := s.publicNote(r, r.PathValue("token"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "not_found", "message": "Catatan tidak ditemukan."}})
		return
	}
	writeJSON(w, http.StatusOK, n)
}

// getPublicNotePage answers /n/{token}: a small page rendered here, with no
// app, login or script beyond refreshing the text.
func (s *Server) getPublicNotePage(w http.ResponseWriter, r *http.Request) {
	publicHeaders(w)
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; img-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	token := r.PathValue("token")
	n, ok := s.publicNote(r, token)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, publicPage("Catatan tidak ditemukan", `<p class="muted">Tautan ini tidak berlaku. Mungkin sudah dimatikan atau diganti oleh pemilik catatan.</p>`, "", 0))
		return
	}
	fmt.Fprint(w, publicPage(n.Title, renderPublicNote(n), token, n.Version))
}

func publicPage(title, body, token string, version int64) string {
	script := ""
	if token != "" {
		// Re-render when the note changes (autosave), keeping the scroll.
		script = `<script>(function(){var v=` + fmt.Sprint(version) + `;setInterval(function(){fetch("/api/public/notes/` + token + `?fragment=1",{cache:"no-store"}).then(function(r){return r.ok?r.json():null}).then(function(j){if(j&&j.version!==v){location.reload()}}).catch(function(){})},20000)})();</script>`
	}
	return `<!doctype html><html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="robots" content="noindex,nofollow,noarchive"><meta name="referrer" content="no-referrer"><title>` + html.EscapeString(title) + ` · Space HIMASIF</title><style>
:root{--bg:#f7f5ef;--surface:#fff;--text:#2d3029;--muted:#64635a;--border:#d8d3c6;--accent:#9c4f2c;--done:#3d7650}
@media (prefers-color-scheme:dark){:root{--bg:#1c1d19;--surface:#24251f;--text:#e8e4d8;--muted:#aaa699;--border:#3d3e36;--accent:#d27b52;--done:#7fb48f}}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--text);font:16px/1.6 "IBM Plex Sans",system-ui,sans-serif}
main{max-width:760px;margin:0 auto;padding:40px 20px 64px}h1{font-size:28px;line-height:1.25;margin:4px 0 8px}h2{font-size:20px;margin:28px 0 8px}h3{font-size:17px;margin:20px 0 6px}
.muted{color:var(--muted);font-size:14px}.tag{display:inline-block;font-size:12px;padding:1px 8px;border:1px solid var(--border);border-radius:999px;margin:0 4px 4px 0;color:var(--muted)}
.callout{border:1px solid var(--border);border-left:3px solid var(--accent);border-radius:8px;padding:10px 14px;background:var(--surface);margin:12px 0}
ul,ol{padding-left:24px}li{margin:2px 0}.check{list-style:none;padding-left:4px}.check li::before{content:"☐ ";color:var(--muted)}.check li.done::before{content:"☑ ";color:var(--done)}.check li.done span.t{text-decoration:line-through;color:var(--muted)}
table{border-collapse:collapse;width:100%;margin:12px 0;font-size:14px}th,td{border:1px solid var(--border);padding:6px 8px;text-align:left;vertical-align:top}th{background:var(--surface)}
hr{border:0;border-top:1px solid var(--border);margin:24px 0}details{border:1px solid var(--border);border-radius:8px;padding:8px 12px;margin:10px 0;background:var(--surface)}
a{color:var(--accent)}.chip{font-size:12px;color:var(--muted);border:1px solid var(--border);border-radius:6px;padding:0 6px;margin-left:6px}footer{margin-top:48px;border-top:1px solid var(--border);padding-top:12px}
</style></head><body><main>` + body + `<footer class="muted">Dibagikan lewat tautan publik dari Space HIMASIF. Hanya-baca; isinya mengikuti catatan aslinya.</footer></main>` + script + `</body></html>`
}

func renderPublicNote(n api.PublicNote) string {
	var b strings.Builder
	b.WriteString(`<p class="muted">` + html.EscapeString(n.Division) + `</p><h1>` + html.EscapeString(n.Title) + `</h1>`)
	if m := n.Meeting; m != nil {
		loc, err := time.LoadLocation(m.TimeZone)
		if err != nil {
			loc = time.UTC
		}
		when := publicDate(m.StartsAt.In(loc))
		if m.Location != "" {
			when += " · " + m.Location
		}
		b.WriteString(`<p class="muted">Rapat: ` + html.EscapeString(when) + `</p>`)
	}
	if len(n.Tags) > 0 {
		b.WriteString(`<p>`)
		for _, t := range n.Tags {
			b.WriteString(`<span class="tag">#` + html.EscapeString(t) + `</span>`)
		}
		b.WriteString(`</p>`)
	}
	b.WriteString(`<p class="muted">Diperbarui ` + html.EscapeString(publicDate(n.UpdatedAt.In(publicZone()))) + ` WIB</p>`)
	renderPublicBlocks(&b, n.Content)
	return b.String()
}

func publicZone() *time.Location {
	if loc, err := time.LoadLocation("Asia/Jakarta"); err == nil {
		return loc
	}
	return time.FixedZone("WIB", 7*3600)
}

var publicMonths = []string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}

func publicDate(t time.Time) string {
	return fmt.Sprintf("%d %s %d, %02d.%02d", t.Day(), publicMonths[t.Month()-1], t.Year(), t.Hour(), t.Minute())
}

func renderPublicBlocks(b *strings.Builder, blocks []api.NoteBlock) {
	for _, bl := range blocks {
		switch bl.Type {
		case api.BlockHeading:
			lvl := min(max(bl.Level+1, 2), 3)
			fmt.Fprintf(b, "<h%d>%s%s</h%d>", lvl, html.EscapeString(bl.Text), publicMinutes(bl.Minutes), lvl)
		case api.BlockParagraph:
			b.WriteString("<p>" + publicSpanHTML(bl.Spans) + "</p>")
		case api.BlockCallout:
			b.WriteString(`<div class="callout">` + publicSpanHTML(bl.Spans) + "</div>")
		case api.BlockBulletedList, api.BlockNumberedList, api.BlockChecklist:
			tag, class := "ul", ""
			if bl.Type == api.BlockNumberedList {
				tag = "ol"
			}
			if bl.Type == api.BlockChecklist {
				class = ` class="check"`
			}
			b.WriteString("<" + tag + class + ">")
			for _, it := range bl.Items {
				li := "<li>"
				if bl.Type == api.BlockChecklist && it.Checked {
					li = `<li class="done">`
				}
				extra := publicMinutes(it.Minutes)
				if it.DueOn != nil {
					extra += `<span class="chip">tenggat ` + html.EscapeString(fmt.Sprintf("%d %s", it.DueOn.Day, publicMonths[it.DueOn.Month-1])) + `</span>`
				}
				b.WriteString(li + `<span class="t">` + publicSpanHTML(it.Spans) + `</span>` + extra + "</li>")
			}
			b.WriteString("</" + tag + ">")
		case api.BlockTable:
			b.WriteString("<table><thead><tr>")
			for _, c := range bl.Columns {
				b.WriteString("<th>" + html.EscapeString(c) + "</th>")
			}
			b.WriteString("</tr></thead><tbody>")
			for _, row := range bl.Rows {
				b.WriteString("<tr>")
				for _, cell := range row {
					b.WriteString("<td>" + publicSpanHTML(cell) + "</td>")
				}
				b.WriteString("</tr>")
			}
			b.WriteString("</tbody></table>")
		case api.BlockDivider:
			b.WriteString("<hr>")
		case api.BlockToggle:
			b.WriteString("<details><summary>" + html.EscapeString(bl.Text) + "</summary>")
			renderPublicBlocks(b, bl.Children)
			b.WriteString("</details>")
		}
	}
}

func publicMinutes(m int) string {
	if m <= 0 {
		return ""
	}
	return fmt.Sprintf(`<span class="chip">%d mnt</span>`, m)
}

func publicSpanHTML(spans []api.Span) string {
	var b strings.Builder
	for _, sp := range spans {
		t := html.EscapeString(sp.Text)
		if sp.Bold {
			t = "<strong>" + t + "</strong>"
		}
		if sp.Link != "" {
			t = `<a href="` + html.EscapeString(sp.Link) + `" rel="nofollow noopener noreferrer" target="_blank">` + t + "</a>"
		}
		b.WriteString(t)
	}
	return b.String()
}
