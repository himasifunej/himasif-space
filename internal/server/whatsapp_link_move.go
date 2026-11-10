package server

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
	"github.com/jackc/pgx/v5"
)

// waLinkMoveTTL is how long the sender may answer PINDAH after a code met a
// number that belongs to another account.
const waLinkMoveTTL = 15 * time.Minute

// waDMMove is the answer that moves a number linked to another account.
var waDMMove = regexp.MustCompile(`(?i)^\s*!?pindah\s*$`)

// waSenderJIDs are the sender's identities in a message: the chat ID and,
// when WhatsApp gave it, the other form (phone number or LID).
func waSenderJIDs(in waInbound) []string {
	jids := []string{in.SenderJID}
	if in.SenderAltJID != "" && in.SenderAltJID != in.SenderJID && waUserJID.MatchString(in.SenderAltJID) {
		jids = append(jids, in.SenderAltJID)
	}
	return jids
}

// waLinkUser loads the account of a challenge and checks it may link in its
// division (and, in a group, see that group's Space).
func (s *Server) waLinkUser(r *http.Request, q store.Querier, group api.WAGroup, uid, did id.ID) (store.User, *principal, error) {
	u, err := s.store.UserForUpdate(r.Context(), q, uid)
	if err != nil {
		return u, nil, err
	}
	if u.DisabledAt != nil || u.Kind == api.AccountOrganizationAdmin {
		return u, nil, errForbidden
	}
	p := &principal{User: u}
	scRequest := r.Clone(r.Context())
	scRequest.SetPathValue("divisionId", did.String())
	sc, err := s.loadScope(r.Context(), q, scRequest, p, store.LockNone)
	if err != nil {
		return u, nil, err
	}
	if err = sc.require(api.CapTasksOwn); err != nil {
		return u, nil, err
	}
	if group.ID != id.Nil {
		view, err := s.spaceView(r.Context(), q, p, group.SpaceID)
		if err != nil {
			return u, nil, err
		}
		if !slices.Contains(view.Capabilities, api.CapSpaceRead) {
			return u, nil, errForbidden
		}
	}
	return u, p, nil
}

// waWho names an account in a reply; the masked NIM only in a private chat.
func waWho(u store.User, direct bool) string {
	who := "*" + u.DisplayName + "*"
	if direct && u.NIM != "" {
		who += " (NIM " + api.MaskNIM(u.NIM) + ")"
	}
	return who
}

// waLinkIdentities links jids to uid in the period, taking them over from
// any other account (callers decide when that is allowed).
func (s *Server) waLinkIdentities(r *http.Request, q store.Querier, period api.Period, did, uid id.ID, jids []string) error {
	for _, jid := range jids {
		if _, err := q.Exec(r.Context(), `insert into wa_identities(period_id,division_id,user_id,jid,verified_by,verified_at) values($1,$2,$3,$4,$3,$5)
			on conflict(period_id,jid) do update set user_id=excluded.user_id,division_id=excluded.division_id,verified_by=excluded.verified_by,verified_at=excluded.verified_at`,
			period.ID, did, uid, jid, s.now()); err != nil {
			return err
		}
	}
	return nil
}

// consumeWALink links the sender's number to the account of a code. When
// the number already belongs to another account nothing moves: the code
// stays open with the conflict, and the reply tells how to move it (PINDAH)
// or to unlink the other account first.
func (s *Server) consumeWALink(r *http.Request, q store.Querier, period api.Period, group api.WAGroup, in waInbound, fields []string) (string, error) {
	if len(fields) != 2 || !strings.HasPrefix(fields[1], "SPACE-") || !waUserJID.MatchString(in.SenderJID) {
		return "", badRequest("Gunakan !link <kode sekali pakai dari akun Space>.")
	}
	hash := sha256.Sum256([]byte(fields[1]))
	var challenge, uid, did id.ID
	err := q.QueryRow(r.Context(), `select id,user_id,division_id from wa_link_challenges where period_id=$1 and token_hash=$2 and used_at is null and expires_at>$3 for update`, period.ID, hash[:], s.now()).Scan(&challenge, &uid, &did)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", badRequest("Kode tautan tidak berlaku atau sudah kedaluwarsa. Buat kode baru di Space → Bot WhatsApp.")
	}
	if err != nil {
		return "", err
	}
	u, p, err := s.waLinkUser(r, q, group, uid, did)
	if err != nil {
		return "", err
	}
	direct := group.ID == id.Nil
	jids := waSenderJIDs(in)
	var other *store.User
	for _, jid := range jids {
		var owner id.ID
		err = q.QueryRow(r.Context(), `select user_id from wa_identities where period_id=$1 and jid=$2`, period.ID, jid).Scan(&owner)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return "", err
		}
		if err == nil && owner != uid {
			o, err := s.store.UserForUpdate(r.Context(), q, owner)
			if err != nil {
				return "", err
			}
			other = &o
			break
		}
	}
	if other != nil {
		if _, err = q.Exec(r.Context(), `update wa_link_challenges set attempted_at=$2,conflict_user_id=$3,conflict_jids=$4,conflict_until=$5 where id=$1`,
			challenge, s.now(), other.ID, jids, s.now().Add(waLinkMoveTTL)); err != nil {
			return "", err
		}
		answer := "balas *PINDAH*"
		if !direct {
			answer = "kirim *!pindah* di sini"
		}
		return "Nomor WhatsApp ini sudah tertaut ke akun Space " + waWho(*other, false) + ".\n\n" +
			"Untuk memindahkannya ke akun " + waWho(u, direct) + ", " + answer + " dalam 15 menit.\n\n" +
			"Atau lepas tautan dari akun " + other.DisplayName + " di Space (Bot WhatsApp → Lepas tautan), lalu buat kode baru.", nil
	}
	if err = s.waLinkIdentities(r, q, period, did, uid, jids); err != nil {
		return "", err
	}
	if _, err = q.Exec(r.Context(), `update wa_link_challenges set used_at=$2,attempted_at=$2 where id=$1`, challenge, s.now()); err != nil {
		return "", err
	}
	source := "group"
	if direct {
		source = "dm"
	}
	if err = s.record(r, q, p, store.AuditEvent{Action: "whatsapp.identity.link", TargetType: "user", TargetID: &uid, PeriodID: &period.ID,
		After: map[string]any{"inboundId": in.ID, "groupId": group.ID, "jid": in.SenderJID, "source": source}}); err != nil {
		return "", err
	}
	if err = s.waLinkHello(r, q, uid, in); err != nil {
		return "", err
	}
	return "Tertaut ✓\nWhatsApp ini sekarang terhubung ke akun Space " + waWho(u, direct) + ".\nBukan kamu? Di Space buka Bot WhatsApp → Lepas tautan.", nil
}

// waLinkHelloText is the test message after a first link.
const waLinkHelloText = "Tersambung ✓\nKamu akan menerima ping dan pengingat dari Space di sini."

// waLinkHello queues, once per account, a short test message to the number
// just linked, in a private chat (also when the code came through a
// group), a few seconds after the reply: the member sees on WhatsApp
// itself that pings and reminders will arrive there.
func (s *Server) waLinkHello(r *http.Request, q store.Querier, uid id.ID, in waInbound) error {
	ctx := r.Context()
	o, err := s.store.Onboarding(ctx, q, uid)
	if err != nil || o.Done(api.OnboardWAHello) || !waUserJID.MatchString(in.SenderJID) {
		return err
	}
	req := whatsapp.SendRequest{ID: id.New(), ToJID: in.SenderJID, Text: waLinkHelloText, Mentions: []string{}}
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	now := s.now()
	if _, err = q.Exec(ctx, `insert into wa_outbox(id,event_key,connection_id,group_id,source_kind,actor_id,payload,available_at,created_at,updated_at) values($1,$2,$3,null,'link_dm',$4,$5,$6,$7,$7) on conflict(event_key) do nothing`,
		req.ID, "link-hello:"+uid.String(), in.ConnectionID, uid, payload, now.Add(3*time.Second), now); err != nil {
		return err
	}
	return s.store.SetOnboardingMark(ctx, q, uid, api.OnboardWAHello, false, now)
}

// moveWALink answers PINDAH: the number's owner (it is their message) moves
// the link from the other account to the account of the open code.
func (s *Server) moveWALink(r *http.Request, q store.Querier, period api.Period, group api.WAGroup, in waInbound) (string, error) {
	if !waUserJID.MatchString(in.SenderJID) {
		return "", errForbidden
	}
	jids := waSenderJIDs(in)
	var challenge, uid, did, otherID id.ID
	var conflictJIDs []string
	err := q.QueryRow(r.Context(), `select id,user_id,division_id,conflict_user_id,conflict_jids from wa_link_challenges
		where period_id=$1 and used_at is null and conflict_until>$2 and conflict_jids && $3::text[] order by attempted_at desc limit 1 for update`,
		period.ID, s.now(), jids).Scan(&challenge, &uid, &did, &otherID, &conflictJIDs)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", badRequest("Tidak ada permintaan pindah untuk nomor ini, atau waktunya sudah habis. Buat kode baru di Space → Bot WhatsApp, lalu kirim kodenya.")
	}
	if err != nil {
		return "", err
	}
	u, p, err := s.waLinkUser(r, q, group, uid, did)
	if err != nil {
		return "", err
	}
	other, err := s.store.UserForUpdate(r.Context(), q, otherID)
	if err != nil {
		return "", err
	}
	all := slices.Clone(conflictJIDs)
	for _, jid := range jids {
		if !slices.Contains(all, jid) {
			all = append(all, jid)
		}
	}
	if err = s.waLinkIdentities(r, q, period, did, uid, all); err != nil {
		return "", err
	}
	if _, err = q.Exec(r.Context(), `update wa_link_challenges set used_at=$2,conflict_until=null where id=$1`, challenge, s.now()); err != nil {
		return "", err
	}
	if err = s.record(r, q, p, store.AuditEvent{Action: "whatsapp.identity.move", TargetType: "user", TargetID: &uid, PeriodID: &period.ID,
		Before: map[string]any{"userId": otherID}, After: map[string]any{"userId": uid, "inboundId": in.ID, "jid": in.SenderJID}}); err != nil {
		return "", err
	}
	if err = s.waLinkHello(r, q, uid, in); err != nil {
		return "", err
	}
	return "Dipindahkan ✓\nWhatsApp ini sekarang terhubung ke akun Space " + waWho(u, group.ID == id.Nil) + ".\nAkun " + other.DisplayName + " tidak lagi menerima pengingat di nomor ini.", nil
}
