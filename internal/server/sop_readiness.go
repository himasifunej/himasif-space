package server

import (
	"net/http"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/store"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

func (s *Server) sopReadiness(w http.ResponseWriter, r *http.Request, p *principal) error {
	var cid *id.ID
	if raw := r.URL.Query().Get("contentId"); raw != "" {
		v, err := id.Parse(raw)
		if err != nil {
			return badRequest("Konten tidak valid.")
		}
		cid = &v
	}
	out := api.SOPReadiness{GeneratedAt: s.now(), Weekly: api.SOPScheduleReadiness{Kind: "weekly", Blockers: []string{}, Warnings: []string{}}}
	err := s.withSOPScope(r, p, "read", nil, func(q store.Querier, ac *sopAccess) error {
		sc := ac.scope
		policy, err := s.store.SOPPolicy(r.Context(), q, sc.period.ID, sc.division.ID, store.LockNone)
		if err != nil {
			return err
		}
		out.PolicyVersion = policy.Version
		base := []string{}
		if !s.cfg.OutboundEnabled() {
			base = append(base, "Mode runtime menonaktifkan penjadwalan dan pengiriman.")
		}
		if sc.closed() {
			base = append(base, "Periode sudah ditutup.")
		}
		if !policy.Enabled {
			base = append(base, "Kebijakan SOP belum aktif.")
		}
		loc, err := timeutil.LoadZone(sc.period.TimeZone)
		if err != nil {
			return err
		}
		today := timeutil.DateOf(s.now().In(loc))
		out.Weekly.Blockers = append(out.Weekly.Blockers, base...)
		if policy.Settings.WeeklyWeekday == nil || policy.Settings.WeeklyClock == "" {
			out.Weekly.Blockers = append(out.Weekly.Blockers, "Hari dan jam rekap mingguan belum ditetapkan.")
		} else {
			clock, err := time.Parse("15:04", policy.Settings.WeeklyClock)
			if err != nil {
				return err
			}
			day := today
			for int(day.At(12, 0, loc).Weekday()) != *policy.Settings.WeeklyWeekday {
				day = day.AddDays(1)
			}
			slot := day.At(clock.Hour(), clock.Minute(), loc)
			out.Weekly.ScheduledAt = &slot
			// The scheduler can still queue today's slot after its clock, but never
			// replays an earlier week's missed slot.
			if day.AddDays(-1).Before(sc.period.StartsOn) {
				out.Weekly.Blockers = append(out.Weekly.Blockers, "Belum ada rentang laporan di dalam periode sebelum slot ini.")
			}
			if day.Compare(sc.period.EndsOn) > 0 {
				out.Weekly.Blockers = append(out.Weekly.Blockers, "Slot rekap berikutnya berada setelah akhir periode.")
			}
		}
		configured, connected, err := s.sopBotReadiness(r, q, sc.period.ID, sc.division.ID, nil)
		if err != nil {
			return err
		}
		if !configured {
			out.Weekly.Blockers = append(out.Weekly.Blockers, "Bot aktif dan pemetaan grup divisi belum tersedia.")
		} else if !connected {
			out.Weekly.Warnings = append(out.Weekly.Warnings, "Bot belum tersambung; antrean tidak membuktikan pesan sudah terkirim.")
		}
		if cid != nil {
			content, err := s.store.Medhum(r.Context(), q, *cid, store.LockShare)
			if err != nil {
				return err
			}
			if content.PeriodID != sc.period.ID || content.HumasDivisionID != sc.division.ID {
				return errNotFound
			}
			f := &api.SOPScheduleReadiness{Kind: "followup", Blockers: append([]string{}, base...), Warnings: []string{}}
			out.Followup = f
			if content.PublishedAt != nil {
				f.Blockers = append(f.Blockers, "Konten sudah dipublikasikan.")
			}
			if policy.Settings.FollowupClock == "" {
				f.Blockers = append(f.Blockers, "Jam follow-up H-4 belum ditetapkan.")
			} else {
				clock, err := time.Parse("15:04", policy.Settings.FollowupClock)
				if err != nil {
					return err
				}
				post := content.PostingDate
				if post == nil {
					day := timeutil.DateOf(content.PostingAt.In(loc))
					post = &day
				}
				slot := post.AddDays(-4).At(clock.Hour(), clock.Minute(), loc)
				f.ScheduledAt = &slot
				if timeutil.DateOf(slot.In(loc)).Before(today) {
					f.Blockers = append(f.Blockers, "Hari H-4 sudah lewat; pesan historis tidak dikirim ulang otomatis.")
				}
			}
			if content.ConceptURL == "" {
				f.Blockers = append(f.Blockers, "Tautan konsep belum diisi.")
			}
			if content.DesignPJID == nil {
				f.Blockers = append(f.Blockers, "PJ desain belum ditetapkan.")
			}
			var humas, design, mention bool
			if err = q.QueryRow(r.Context(), `select exists(select 1 from memberships m join users u on u.id=m.user_id where m.period_id=$1 and m.division_id=$2 and m.user_id=$3 and m.removed_at is null and u.disabled_at is null),exists(select 1 from memberships m join users u on u.id=m.user_id where m.period_id=$1 and m.division_id=$4 and m.user_id=$5 and m.removed_at is null and u.disabled_at is null),exists(select 1 from wa_identities where period_id=$1 and user_id=$5)`, sc.period.ID, content.HumasDivisionID, content.HumasPJID, content.DesignDivisionID, content.DesignPJID).Scan(&humas, &design, &mention); err != nil {
				return err
			}
			if !humas {
				f.Blockers = append(f.Blockers, "PJ Humas tidak memiliki akun/keanggotaan aktif.")
			}
			if content.DesignPJID != nil && !design {
				f.Blockers = append(f.Blockers, "PJ desain tidak memiliki akun/keanggotaan aktif.")
			}
			if content.DesignPJID != nil && !mention {
				f.Blockers = append(f.Blockers, "Identitas WhatsApp PJ desain belum terverifikasi untuk mention.")
			}
			configured, connected, err = s.sopBotReadiness(r, q, sc.period.ID, sc.division.ID, &content.SpaceID)
			if err != nil {
				return err
			}
			if !configured {
				f.Blockers = append(f.Blockers, "Bot Humas aktif, kadiv aktif dan pemetaan grup Medhum belum tersedia.")
			} else if !connected {
				f.Warnings = append(f.Warnings, "Bot belum tersambung; pengiriman masih dapat tertunda di antrean.")
			}
			if !policy.Settings.BotFollowupCountsRequested || !policy.BotFollowupCountsApproved {
				f.Warnings = append(f.Warnings, "Follow-up bot belum disepakati sebagai pemenuhan SOP; koordinasi manusia tetap diperlukan.")
			}
			var queued bool
			if err = q.QueryRow(r.Context(), `select exists(select 1 from sop_followups f join wa_outbox o on o.id=f.outbox_id where f.content_id=$1 and f.calendar_key=$2 and o.status in ('queued','sending','unknown','accepted','delivered','read'))`, content.ID, sopCalendarKey(content)).Scan(&queued); err != nil {
				return err
			}
			if queued {
				f.Warnings = append(f.Warnings, "Ada antrean atau pengiriman untuk jadwal konten ini. Periksa riwayat sebelum mengambil tindakan lain.")
			}
			f.Ready = len(f.Blockers) == 0
		}
		out.Weekly.Ready = len(out.Weekly.Blockers) == 0
		return nil
	})
	if err != nil {
		return err
	}
	writeJSON(w, 200, out)
	return nil
}
func (s *Server) sopBotReadiness(r *http.Request, q store.Querier, pid, did id.ID, medhum *id.ID) (bool, bool, error) {
	var configured, connected bool
	err := q.QueryRow(r.Context(), `select exists(select 1 from wa_connections w join wa_groups g on g.connection_id=w.id join spaces sp on sp.id=g.space_id where w.period_id=$1 and w.division_id=$2 and w.enabled and exists(select 1 from memberships m join users u on u.id=m.user_id where m.period_id=$1 and m.division_id=$2 and m.role='kadiv' and m.removed_at is null and u.disabled_at is null) and (($3::uuid is not null and sp.id=$3 and sp.code='medhum') or ($3::uuid is null and sp.kind='division' and exists(select 1 from space_divisions sd where sd.space_id=sp.id and sd.division_id=$2)))),exists(select 1 from wa_connections where period_id=$1 and division_id=$2 and enabled and status='connected')`, pid, did, medhum).Scan(&configured, &connected)
	return configured, connected, err
}
