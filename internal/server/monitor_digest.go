package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/mail"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/whatsapp"
)

// The weekly digest of BPH (PRD MON-08, canvas "Ringkasan mingguan"): on
// Monday from 07.00 in the period's zone (WIB), every holder of org.monitor
// who has not turned it off gets last week's numbers by WhatsApp direct
// message, email or both. Counts and division names only, never the
// content of notes. A digest missed on Monday is not sent later in the week.

// digestHour is when the digest goes out on Monday, local time.
const digestHour = 7

// digestDivision is one division's line of a digest.
type digestDivision struct {
	Name          string
	Done, Overdue int
}

// monitorDigest is the content of one week's digest.
type monitorDigest struct {
	From, To        time.Time // the week: Monday 00.00 to the next Monday
	Divisions       []digestDivision
	Done, Overdue   int
	AwaitingReview  int
	Meetings        int
	MeetingsNoNotes int
	Link            string
}

// digestWeek returns the week a digest sent at now summarises (the Monday
// before this week's Monday, and this week's Monday), and when this week's
// digest is due.
func digestWeek(now time.Time, loc *time.Location) (from, to, due time.Time) {
	to = weekStart(now, loc)
	return to.AddDate(0, 0, -7), to, to.Add(digestHour * time.Hour)
}

// buildMonitorDigest counts a period's week: tasks finished in it by owner
// division (a task's last change while done, an approximation, since a
// task keeps no time of completion), tasks overdue and awaiting review now,
// and the week's meetings with and without notes.
func (s *Server) buildMonitorDigest(ctx context.Context, q store.Querier, period api.Period, from, to time.Time) (monitorDigest, error) {
	d := monitorDigest{From: from, To: to, Link: s.cfg.PublicURL + "/#/pemantauan"}
	today := s.now().In(timeZoneOrUTC(period.TimeZone)).Format(time.DateOnly)
	err := each(ctx, q, func(scan func(...any) error) error {
		var row digestDivision
		if err := scan(&row.Name, &row.Done, &row.Overdue); err != nil {
			return err
		}
		if row.Done > 0 || row.Overdue > 0 {
			d.Divisions = append(d.Divisions, row)
		}
		d.Done += row.Done
		d.Overdue += row.Overdue
		return nil
	}, `select dv.name,
			count(t.id) filter (where t.progress = 'done' and t.updated_at >= $2 and t.updated_at < $3),
			count(t.id) filter (where t.progress in ('todo','doing') and (t.deadline_at < $4 or t.deadline_date < $5::date))
		from period_divisions pd join divisions dv on dv.id = pd.division_id
		left join tasks t on t.period_id = pd.period_id and t.owner_division_id = pd.division_id
		where pd.period_id = $1 group by dv.name order by dv.name`, period.ID, from, to, s.now(), today)
	if err != nil {
		return d, err
	}
	if err := q.QueryRow(ctx, `select count(*) from tasks where period_id = $1 and progress = 'submitted' and match_status = 'pending'`, period.ID).Scan(&d.AwaitingReview); err != nil {
		return d, err
	}
	if err := q.QueryRow(ctx, `select count(*), count(*) filter (where note_id is null) from meetings
		where period_id = $1 and not cancelled and starts_at >= $2 and starts_at < $3`, period.ID, from, to).Scan(&d.Meetings, &d.MeetingsNoNotes); err != nil {
		return d, err
	}
	return d, nil
}

func digestRange(d monitorDigest, loc *time.Location) string {
	a, b := d.From.In(loc), d.To.In(loc).AddDate(0, 0, -1)
	if a.Month() == b.Month() {
		return fmt.Sprintf("%d–%d %s", a.Day(), b.Day(), monthShort(b.Month()))
	}
	return fmt.Sprintf("%d %s – %d %s", a.Day(), monthShort(a.Month()), b.Day(), monthShort(b.Month()))
}

func monthShort(m time.Month) string {
	return [...]string{"", "Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}[m]
}

// top names the divisions with most of a count, as "Mediatek 6, Humas 4".
func digestTop(ds []digestDivision, count func(digestDivision) int, n int) string {
	ds = slices.Clone(ds)
	ds = slices.DeleteFunc(ds, func(d digestDivision) bool { return count(d) == 0 })
	slices.SortStableFunc(ds, func(a, b digestDivision) int { return count(b) - count(a) })
	var parts []string
	rest := 0
	for i, d := range ds {
		if i < n {
			parts = append(parts, fmt.Sprintf("%s %d", d.Name, count(d)))
		} else {
			rest += count(d)
		}
	}
	if rest > 0 {
		parts = append(parts, fmt.Sprintf("lainnya %d", rest))
	}
	return strings.Join(parts, ", ")
}

// digestText is the WhatsApp message (and the email's plain text).
func digestText(d monitorDigest, loc *time.Location) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*Ringkasan Space · %s*\n", digestRange(d, loc))
	done := func(x digestDivision) int { return x.Done }
	late := func(x digestDivision) int { return x.Overdue }
	fmt.Fprintf(&b, "Selesai: %d tugas", d.Done)
	if t := digestTop(d.Divisions, done, 2); t != "" {
		fmt.Fprintf(&b, " (%s)", t)
	}
	fmt.Fprintf(&b, "\nTerlambat: %d tugas", d.Overdue)
	if t := digestTop(d.Divisions, late, 2); t != "" {
		fmt.Fprintf(&b, ", terbanyak %s", t)
	}
	fmt.Fprintf(&b, "\nMenunggu tinjauan: %d bukti", d.AwaitingReview)
	fmt.Fprintf(&b, "\nRapat: %d, notulen belum tertaut %d", d.Meetings, d.MeetingsNoNotes)
	fmt.Fprintf(&b, "\nDetail: %s", d.Link)
	return b.String()
}

// digestMail is the email: the same numbers, with a table per division.
func digestMail(to string, d monitorDigest, loc *time.Location) mail.Message {
	r := digestRange(d, loc) + " " + fmt.Sprint(d.To.In(loc).AddDate(0, 0, -1).Year())
	var rows strings.Builder
	for _, dv := range d.Divisions {
		fmt.Fprintf(&rows, `<tr><td style="padding:4px 12px 4px 0">%s</td><td style="padding:4px 12px">%d</td><td style="padding:4px 12px">%d</td></tr>`, html.EscapeString(dv.Name), dv.Done, dv.Overdue)
	}
	body := fmt.Sprintf(`<p><b>Ringkasan Space · %s</b></p>
<p>Menunggu tinjauan: %d bukti · Rapat: %d, notulen belum tertaut %d</p>
<table style="border-collapse:collapse"><tr><th align="left" style="padding:4px 12px 4px 0">Divisi</th><th align="left" style="padding:4px 12px">Selesai</th><th align="left" style="padding:4px 12px">Terlambat</th></tr>%s</table>
<p><a href="%s">Buka pemantauan</a></p>
<p style="color:#777">Dikirim ke pemegang izin pemantauan organisasi. Matikan di Space: Pengaturan → Ringkasan mingguan.</p>`,
		html.EscapeString(r), d.AwaitingReview, d.Meetings, d.MeetingsNoNotes, rows.String(), html.EscapeString(d.Link))
	text := strings.ReplaceAll(digestText(d, loc), "*", "") + "\n\nMatikan di Space: Pengaturan → Ringkasan mingguan."
	return mail.Message{To: to, Subject: "Ringkasan Space · " + r, Text: text, HTML: body}
}

// digestPreference returns a person's choice, the default when none.
func (s *Server) digestPreference(ctx context.Context, q store.Querier, uid id.ID) (enabled bool, channel string, err error) {
	err = q.QueryRow(ctx, `select enabled, channel from monitor_digest_preferences where user_id = $1`, uid).Scan(&enabled, &channel)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, api.DigestBoth, nil
	}
	return enabled, channel, err
}

// digestWhatsApp finds where a direct message to uid goes in a period: a
// linked phone number (not a private LID) and an enabled bot of the period,
// the one of the division the number was linked in first.
func (s *Server) digestWhatsApp(ctx context.Context, q store.Querier, pid, uid id.ID) (jid string, conn id.ID, ok bool, err error) {
	err = q.QueryRow(ctx, `select i.jid, c.id from wa_identities i join wa_connections c on c.period_id = i.period_id and c.enabled
		where i.period_id = $1 and i.user_id = $2 and i.jid like '%@s.whatsapp.net'
		order by (c.division_id = i.division_id) desc, i.verified_at desc, c.id limit 1`, pid, uid).Scan(&jid, &conn)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", id.Nil, false, nil
	}
	return jid, conn, err == nil, err
}

// runMonitorDigests sends the week's digests that are due. The scheduler
// calls it; each digest is recorded once per person and week before it is
// queued or mailed, so another instance or a restart never repeats it.
func (s *Server) runMonitorDigests(ctx context.Context) error {
	type due struct {
		period api.Period
		user   id.ID
		email  string
	}
	var work []due
	now := s.now()
	err := each(ctx, s.store.DB(), func(scan func(...any) error) error {
		var d due
		if err := scan(&d.period.ID, &d.period.TimeZone, &d.period.Status, &d.user, &d.email); err != nil {
			return err
		}
		work = append(work, d)
		return nil
	}, `select p.id, p.time_zone, p.status, g.user_id, u.email from monitor_grants g
		join periods p on p.id = g.period_id join users u on u.id = g.user_id
		where g.revoked_at is null and p.status <> 'closed' and u.disabled_at is null`)
	if err != nil {
		return err
	}
	for _, w := range work {
		loc := timeZoneOrUTC(w.period.TimeZone)
		from, to, at := digestWeek(now, loc)
		if now.Before(at) || !now.Before(at.Add(24*time.Hour)) {
			continue
		}
		if err := s.sendMonitorDigest(ctx, w.period, w.user, w.email, from, to, loc); err != nil {
			s.log.Error("monitor digest", "user", w.user, "err", err)
		}
	}
	return nil
}

func (s *Server) sendMonitorDigest(ctx context.Context, period api.Period, uid id.ID, email string, from, to time.Time, loc *time.Location) error {
	var digest monitorDigest
	var mailTo string
	err := s.store.Tx(ctx, func(q store.Querier) error {
		enabled, channel, err := s.digestPreference(ctx, q, uid)
		if err != nil || !enabled {
			return err
		}
		if ok, err := s.store.MonitorActive(ctx, q, period.ID, uid); err != nil || !ok {
			return err
		}
		channels := []string{}
		if channel != api.DigestEmail {
			channels = append(channels, api.DigestWhatsApp)
		}
		if channel != api.DigestWhatsApp {
			channels = append(channels, api.DigestEmail)
		}
		tag, err := q.Exec(ctx, `insert into monitor_digest_runs (user_id, period_id, week_start, channels, created_at) values ($1, $2, $3, $4, $5)
			on conflict do nothing`, uid, period.ID, from.In(loc).Format(time.DateOnly), channels, s.now())
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		if digest, err = s.buildMonitorDigest(ctx, q, period, from, to); err != nil {
			return err
		}
		if slices.Contains(channels, api.DigestWhatsApp) {
			jid, conn, ok, err := s.digestWhatsApp(ctx, q, period.ID, uid)
			if err != nil {
				return err
			}
			if ok {
				req := whatsapp.SendRequest{ID: id.New(), ToJID: jid, Text: digestText(digest, loc), Mentions: []string{}}
				payload, err := json.Marshal(req)
				if err != nil {
					return err
				}
				now := s.now()
				if _, err := q.Exec(ctx, `insert into wa_outbox(id,event_key,connection_id,group_id,source_kind,actor_id,payload,available_at,created_at,updated_at)
					values($1,$2,$3,null,'monitor_digest',$4,$5,$6,$6,$6) on conflict(event_key) do nothing`,
					req.ID, "monitor-digest:"+uid.String()+":"+from.In(loc).Format(time.DateOnly), conn, uid, payload, now); err != nil {
					return err
				}
			}
		}
		if slices.Contains(channels, api.DigestEmail) && email != "" {
			mailTo = email
		}
		return nil
	})
	if err != nil || mailTo == "" {
		return err
	}
	sendErr := s.send(ctx, digestMail(mailTo, digest, loc))
	msg := ""
	var sent *time.Time
	if sendErr != nil {
		msg = sendErr.Error()
	} else {
		now := s.now()
		sent = &now
	}
	_, err = s.store.DB().Exec(ctx, `update monitor_digest_runs set email_sent_at = $3, email_error = $4 where user_id = $1 and week_start = $2`,
		uid, from.In(loc).Format(time.DateOnly), sent, msg)
	return err
}

// monitorDigestSettings is GET and PUT /api/me/monitor-digest: a person's
// weekly digest, with last week's as a preview when they hold org.monitor.
func (s *Server) monitorDigestSettings(w http.ResponseWriter, r *http.Request, p *principal) error {
	if p.admin() {
		return errForbidden
	}
	ctx := r.Context()
	if r.Method == http.MethodPut {
		var req api.MonitorDigestRequest
		if err := readJSON(r, &req); err != nil {
			return err
		}
		if !slices.Contains([]string{api.DigestWhatsApp, api.DigestEmail, api.DigestBoth}, req.Channel) {
			return badRequest("Pilih WhatsApp, email atau keduanya.")
		}
		err := s.store.Tx(ctx, func(q store.Querier) error {
			before, beforeChannel, err := s.digestPreference(ctx, q, p.User.ID)
			if err != nil {
				return err
			}
			if _, err := q.Exec(ctx, `insert into monitor_digest_preferences (user_id, enabled, channel, updated_at) values ($1, $2, $3, $4)
				on conflict (user_id) do update set enabled = excluded.enabled, channel = excluded.channel, updated_at = excluded.updated_at`,
				p.User.ID, req.Enabled, req.Channel, s.now()); err != nil {
				return err
			}
			uid := p.User.ID
			return s.record(r, q, p, store.AuditEvent{Action: "monitor.digest.preference", TargetType: "user", TargetID: &uid,
				Before: map[string]any{"enabled": before, "channel": beforeChannel}, After: req})
		})
		if err != nil {
			return err
		}
	}
	q := s.store.DB()
	out := api.MonitorDigestSettings{}
	var err error
	if out.Enabled, out.Channel, err = s.digestPreference(ctx, q, p.User.ID); err != nil {
		return err
	}
	periods, err := s.store.MonitorPeriodIDs(ctx, q, p.User.ID)
	if err != nil {
		return err
	}
	for pid := range periods {
		period, err := s.store.Period(ctx, q, pid, store.LockNone)
		if err != nil {
			return err
		}
		out.Monitor = true
		loc := timeZoneOrUTC(period.TimeZone)
		from, to, _ := digestWeek(s.now(), loc)
		d, err := s.buildMonitorDigest(ctx, q, period, from, to)
		if err != nil {
			return err
		}
		out.Preview = digestText(d, loc)
		uid, kind := p.actor()
		if err := s.store.Audit(ctx, q, store.AuditEvent{ActorUserID: uid, ActorKind: kind, Action: "monitor.read", Source: "api", PeriodID: &pid,
			RequestID: requestID(ctx), OccurredAt: s.now(), After: map[string]any{"detail": "Membaca pratinjau ringkasan mingguan", "route": r.Pattern}}); err != nil {
			return err
		}
		if _, _, ok, err := s.digestWhatsApp(ctx, q, pid, p.User.ID); err != nil {
			return err
		} else if ok {
			out.WhatsAppLinked = true
		}
		break
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}
