package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
	"github.com/jackc/pgx/v5"
)

// Stage C7: "Kirim ringkasan ke WhatsApp". An editor of a note sends its
// decisions (Keputusan callouts) and action items (checklist items with a
// PJ, a due date or a task) to the WhatsApp group of the note's owner
// division, through that division's bot, with a link to the note. The
// first call (Send false) is the preview; sending carries the version the
// preview showed, so what goes out is what was seen.

// noteSummaryCooldown keeps a double tap from sending twice.
const noteSummaryCooldown = time.Minute

type noteSummaryAction struct {
	text, pj string
	due      string
	done     bool
}

// noteSummaryParts reads the decisions and action items of a document,
// also inside toggles, in document order.
func noteSummaryParts(blocks []api.NoteBlock, names map[id.ID]string) (decisions []string, actions []noteSummaryAction) {
	var walk func([]api.NoteBlock)
	walk = func(bs []api.NoteBlock) {
		for _, b := range bs {
			switch {
			case b.Type == api.BlockCallout && b.Variant == api.CalloutDecision:
				if t := strings.TrimSpace(spansText(b.Spans)); t != "" {
					decisions = append(decisions, t)
				}
			case b.Type == api.BlockChecklist:
				for _, it := range b.Items {
					if it.AssigneeID == nil && it.DueOn == nil && it.TaskID == nil {
						continue
					}
					a := noteSummaryAction{text: strings.TrimSpace(spansText(it.Spans)), done: it.Checked}
					if a.text == "" {
						continue
					}
					if it.AssigneeID != nil {
						a.pj = names[*it.AssigneeID]
					}
					if it.DueOn != nil {
						a.due = waShortDate(*it.DueOn)
					}
					actions = append(actions, a)
				}
			case b.Type == api.BlockToggle:
				walk(b.Children)
			}
		}
	}
	walk(blocks)
	return decisions, actions
}

func spansText(spans []api.Span) string {
	var b strings.Builder
	for _, s := range spans {
		b.WriteString(s.Text)
	}
	return b.String()
}

var waMonths = [...]string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
var waDays = [...]string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}

func waShortDate(d interface {
	String() string
}) string {
	t, err := time.Parse("2006-01-02", d.String())
	if err != nil {
		return d.String()
	}
	return fmt.Sprintf("%d %s", t.Day(), waMonths[t.Month()-1])
}

// noteSummaryText is the message: plain text, without the bot's name or
// greeting, as the replies of linking WhatsApp.
func noteSummaryText(title string, meeting *api.Meeting, decisions []string, actions []noteSummaryAction, link string) string {
	var b strings.Builder
	b.WriteString("*Ringkasan rapat: " + strings.TrimSpace(title) + "*")
	if meeting != nil {
		at := meeting.StartsAt.In(timeZoneOrUTC(meeting.TimeZone))
		b.WriteString(fmt.Sprintf("\n%s, %d %s %d", waDays[at.Weekday()], at.Day(), waMonths[at.Month()-1], at.Year()))
		if loc := strings.TrimSpace(meeting.Location); loc != "" {
			b.WriteString(" · " + loc)
		}
	}
	if len(decisions) > 0 {
		b.WriteString("\n\n*Keputusan*")
		for i, d := range decisions {
			b.WriteString(fmt.Sprintf("\n%d. %s", i+1, d))
		}
	}
	if len(actions) > 0 {
		b.WriteString("\n\n*Action item*")
		for _, a := range actions {
			mark := "•"
			if a.done {
				mark = "✓"
			}
			line := "\n" + mark + " " + a.text
			var meta []string
			if a.pj != "" {
				meta = append(meta, "PJ: "+a.pj)
			}
			if a.due != "" {
				meta = append(meta, "tenggat "+a.due)
			}
			if len(meta) > 0 {
				line += " — " + strings.Join(meta, " · ")
			}
			b.WriteString(line)
		}
	}
	if link != "" {
		b.WriteString("\n\nCatatan lengkap: " + link)
	}
	return b.String()
}

// noteSummaryLink is where the message points: the note's public page when
// it is public and the caller sent its live link (C6 keeps only the token's
// hash), else Space, where a member opens it after signing in. public says
// whether the note has a live public link at all.
func (s *Server) noteSummaryLink(ctx context.Context, q store.Querier, n *store.NoteRow, sent string) (link string, usedPublic, public bool, err error) {
	live, err := s.store.NotePublicLink(ctx, q, n.ID)
	if err != nil {
		return "", false, false, err
	}
	public = live != nil
	prefix := strings.TrimSuffix(s.cfg.PublicURL, "/") + "/n/"
	if public && sent != "" && strings.HasPrefix(sent, prefix) {
		token := strings.TrimPrefix(sent, prefix)
		hash := sha256.Sum256([]byte(token))
		var ok bool
		if err := q.QueryRow(ctx, `select exists(select 1 from note_public_links where note_id=$1 and token_hash=$2 and revoked_at is null)`, n.ID, hash[:]).Scan(&ok); err != nil {
			return "", false, false, err
		}
		if ok && publicTokenRe.MatchString(token) {
			return s.publicNoteURL(token), true, true, nil
		}
	}
	if s.cfg.PublicURL == "" {
		return "", false, public, nil
	}
	return strings.TrimSuffix(s.cfg.PublicURL, "/") + "/#/spaces", false, public, nil
}

func (s *Server) noteWASummary(w http.ResponseWriter, r *http.Request, p *principal) error {
	var req api.NoteWASummaryRequest
	if err := readJSON(r, &req); err != nil {
		return err
	}
	var out api.NoteWASummary
	err := s.withNote(r, p, req.Send, func(q store.Querier, period api.Period, n *store.NoteRow) error {
		ctx := r.Context()
		if !n.Permissions.Manage {
			return errForbidden
		}
		if req.Send && n.Version != req.ExpectedVersion {
			return conflict("Catatan berubah sejak pratinjau. Lihat pratinjau terbaru sebelum mengirim.")
		}
		out.Version = n.Version
		var meeting *api.Meeting
		// A meeting points at its note (as getNote reads it).
		var mid id.ID
		if err := q.QueryRow(ctx, `select id from meetings where note_id=$1 or id=$2 order by (note_id=$1) desc limit 1`, n.ID, n.MeetingID).Scan(&mid); err == nil {
			n.MeetingID = &mid
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if n.MeetingID != nil {
			m, err := s.store.Meeting(ctx, q, *n.MeetingID, store.LockNone)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if err == nil {
				meeting = &m
			}
		}
		var ids []id.ID
		var collect func([]api.NoteBlock)
		collect = func(bs []api.NoteBlock) {
			for _, b := range bs {
				for _, it := range b.Items {
					if it.AssigneeID != nil {
						ids = append(ids, *it.AssigneeID)
					}
				}
				collect(b.Children)
			}
		}
		collect(n.Content)
		names := map[id.ID]string{}
		if len(ids) > 0 {
			rows, err := q.Query(ctx, `select id,display_name from users where id=any($1)`, ids)
			if err != nil {
				return err
			}
			for rows.Next() {
				var uid id.ID
				var name string
				if err := rows.Scan(&uid, &name); err != nil {
					rows.Close()
					return err
				}
				names[uid] = name
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
		}
		decisions, actions := noteSummaryParts(n.Content, names)
		out.Decisions, out.Actions = len(decisions), len(actions)
		link, usedPublic, public, err := s.noteSummaryLink(ctx, q, n, req.PublicURL)
		if err != nil {
			return err
		}
		out.Public = usedPublic
		out.Text = noteSummaryText(n.Title, meeting, decisions, actions, link)

		var connID, gid id.ID
		var jid, status string
		err = q.QueryRow(ctx, `select w.id,g.id,g.jid,g.name,w.status from wa_connections w join wa_groups g on g.connection_id=w.id join spaces sp on sp.id=g.space_id join space_divisions sd on sd.space_id=sp.id and sd.division_id=w.division_id where w.period_id=$1 and w.division_id=$2 and w.enabled and sp.kind='division' order by g.id limit 1`, n.PeriodID, n.OwnerDivisionID).Scan(&connID, &gid, &jid, &out.GroupName, &status)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			out.Problem = "Divisi ini belum punya grup WhatsApp yang tersambung ke bot. Kadiv bisa menyambungkannya di Bot WhatsApp."
		case err != nil:
			return err
		case decisions == nil && actions == nil:
			out.Problem = "Belum ada keputusan atau action item (dengan PJ atau tenggat) di catatan ini."
		case status != "connected":
			out.Warning = "Bot WhatsApp sedang tidak tersambung; ringkasan terkirim begitu tersambung lagi."
		case public && !usedPublic:
			out.Warning = "Catatan ini publik, tetapi tautannya hanya tampil saat dibuat; ringkasan memakai tautan Space. Buat tautan baru di Bagikan bila ingin menyertakan tautan publik."
		}
		if !req.Send {
			return nil
		}
		if out.Problem != "" {
			return conflict(out.Problem)
		}
		var recent bool
		if err := q.QueryRow(ctx, `select exists(select 1 from wa_outbox where source_kind='note_summary' and source_id=$1 and created_at>$2 and status<>'cancelled')`, n.ID, s.now().Add(-noteSummaryCooldown)).Scan(&recent); err != nil {
			return err
		}
		if recent {
			return conflict("Ringkasan catatan ini baru saja dikirim. Tunggu sebentar sebelum mengirim lagi.")
		}
		oid := id.New()
		body, err := json.Marshal(whatsapp.SendRequest{ID: oid, GroupJID: jid, Text: out.Text, Mentions: []string{}})
		if err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `insert into wa_outbox(id,event_key,connection_id,group_id,source_kind,source_id,actor_id,payload,available_at,created_at,updated_at) values($1,$2,$3,$4,'note_summary',$5,$6,$7,$8,$8,$8)`, oid, "note-summary:"+oid.String(), connID, gid, n.ID, p.User.ID, body, s.now()); err != nil {
			return err
		}
		out.Queued, out.OutboxID = true, &oid
		return s.noteEvent(r, q, p, n, "note.wa_summary", "", nil, map[string]any{"outboxId": oid, "group": out.GroupName, "decisions": out.Decisions, "actions": out.Actions, "version": n.Version})
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}
