package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
)

// Linking a member's WhatsApp to their Space account (plan §8, option a):
// "Tautkan WhatsApp" makes a one-time code and opens a chat with the
// division's bot with the code typed in; the member taps Send. The message
// comes from their WhatsApp account, which proves the number; the code
// proves the Space account. "!link <code>" in a connected group still works.

// waLinkTTL is how long a link code is valid.
const waLinkTTL = 15 * time.Minute

// waDMCode is the only direct message the bot reads: a link code, with or
// without "!link" in front.
var waDMCode = regexp.MustCompile(`(?i)^\s*(?:!link\s+)?(SPACE-[A-Za-z0-9_-]{16,64})\s*$`)

// waBotNumber is the phone number of a connection's WhatsApp account, from
// its JID (6281234567890:12@s.whatsapp.net), or "" when unknown.
func waBotNumber(accountJID string) string {
	user, server, ok := strings.Cut(accountJID, "@")
	if !ok || server != "s.whatsapp.net" {
		return ""
	}
	user, _, _ = strings.Cut(user, ":")
	if len(user) < 8 || strings.Trim(user, "0123456789") != "" {
		return ""
	}
	return user
}

// divisionBotNumber is the number of the division's enabled bot, if any.
func (s *Server) divisionBotNumber(r *http.Request, q store.Querier, sc *scope) (string, error) {
	conns, err := s.store.WAConnections(r.Context(), q, sc.period.ID, sc.division.ID)
	if err != nil {
		return "", err
	}
	for _, c := range conns {
		if n := waBotNumber(c.AccountJID); c.Enabled && n != "" {
			return n, nil
		}
	}
	return "", nil
}

func (s *Server) createWALink(w http.ResponseWriter, r *http.Request, p *principal) error {
	bytes := make([]byte, 18)
	if _, err := rand.Read(bytes); err != nil {
		return err
	}
	code := "SPACE-" + base64.RawURLEncoding.EncodeToString(bytes)
	sum := sha256.Sum256([]byte(code))
	out := api.WALinkChallenge{Code: code, ExpiresAt: s.now().Add(waLinkTTL),
		Instructions: "Kirim !link " + code + " di grup divisi/Medhum yang terhubung. Kode sekali pakai ini hanya untuk akunmu."}
	err := s.store.Tx(r.Context(), func(q store.Querier) error {
		sc, err := s.loadScope(r.Context(), q, r, p, store.LockUpdate)
		if err != nil {
			return err
		}
		if p.admin() {
			return errForbidden
		}
		if err := sc.require(api.CapTasksOwn); err != nil {
			return err
		}
		if _, err := s.lockedAccount(r, q, p); err != nil {
			return err
		}
		bot, err := s.divisionBotNumber(r, q, sc)
		if err != nil {
			return err
		}
		if bot != "" {
			out.DeepLink = "https://wa.me/" + bot + "?text=" + url.QueryEscape(code)
			out.Instructions = "Ketuk Kirim di WhatsApp yang terbuka. Kode sekali pakai ini hanya untuk akunmu; jangan diteruskan ke orang lain."
		}
		if _, err := q.Exec(r.Context(), `update wa_link_challenges set used_at=$2 where user_id=$1 and used_at is null`, p.User.ID, s.now()); err != nil {
			return err
		}
		_, err = q.Exec(r.Context(), `insert into wa_link_challenges(id,period_id,division_id,user_id,token_hash,expires_at) values($1,$2,$3,$4,$5,$6)`, id.New(), sc.period.ID, sc.division.ID, p.User.ID, sum[:], out.ExpiresAt)
		return err
	})
	if err != nil {
		return err
	}
	writeJSON(w, 201, out)
	return nil
}

// waLinkStatus is the caller's own link: whether, which number (masked),
// since when, and whether the bot can be reached for linking.
func (s *Server) waLinkStatus(w http.ResponseWriter, r *http.Request, p *principal) error {
	var out api.WALinkStatus
	err := s.store.Tx(r.Context(), func(q store.Querier) error {
		sc, err := s.loadScope(r.Context(), q, r, p, store.LockShare)
		if err != nil {
			return err
		}
		if p.admin() {
			return errForbidden
		}
		if err := sc.require(api.CapTasksOwn); err != nil {
			return err
		}
		rows, err := q.Query(r.Context(), `select jid, verified_at from wa_identities where period_id=$1 and user_id=$2 order by verified_at`, sc.period.ID, p.User.ID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var jid string
			var at time.Time
			if err := rows.Scan(&jid, &at); err != nil {
				rows.Close()
				return err
			}
			out.Linked = true
			if out.LinkedAt == nil || at.After(*out.LinkedAt) {
				t := at.UTC()
				out.LinkedAt = &t
			}
			if user, server, _ := strings.Cut(jid, "@"); server == "s.whatsapp.net" && out.Phone == "" {
				out.Phone = api.MaskPhone(user)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		bot, err := s.divisionBotNumber(r, q, sc)
		out.BotReady = bot != ""
		return err
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

// unlinkWA removes the caller's WhatsApp link in the period ("Bukan saya?
// Lepas tautan"). Reminders stop reaching that number until they link again.
func (s *Server) unlinkWA(w http.ResponseWriter, r *http.Request, p *principal) error {
	err := s.store.Tx(r.Context(), func(q store.Querier) error {
		sc, err := s.loadScope(r.Context(), q, r, p, store.LockShare)
		if err != nil {
			return err
		}
		if p.admin() {
			return errForbidden
		}
		if err := sc.require(api.CapTasksOwn); err != nil {
			return err
		}
		if _, err := s.lockedAccount(r, q, p); err != nil {
			return err
		}
		tag, err := q.Exec(r.Context(), `delete from wa_identities where period_id=$1 and user_id=$2`, sc.period.ID, p.User.ID)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		uid := p.User.ID
		return s.record(r, q, p, sc.audit(store.AuditEvent{Action: "whatsapp.identity.unlink", TargetType: "user", TargetID: &uid,
			Before: map[string]any{"identities": tag.RowsAffected()}}))
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// waDirectInbound decides what to keep of a direct message to the bot: the
// canonical "!link <code>" when it is a link code from a person, within
// the per-chat limit; otherwise nothing, ignored.
func waDirectInbound(s *Server, e whatsapp.Event, live bool) (body, state, reason string) {
	m := waDMCode.FindStringSubmatch(e.Text)
	switch {
	case e.Type != "message" || m == nil || !waUserJID.MatchString(e.SenderJID):
		return "", "ignored", "Pesan pribadi selain kode tautan tidak dibaca."
	case !live:
		return "", "ignored", "Bot tidak aktif atau periode ditutup saat penerimaan."
	case !s.waDMLimit.allow("dm:"+e.SenderJID, s.now()):
		return "", "ignored", "Terlalu banyak kode tautan dari chat ini."
	}
	return "!link " + m[1], "queued", ""
}
