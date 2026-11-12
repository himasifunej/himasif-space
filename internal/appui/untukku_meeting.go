package appui

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// Rapat (canvas 3747dbbac0d0, design 5), opened from a meeting row or
// Berikutnya: when and where, the agenda, the user's own answer
// (Kehadiranmu: Hadir, Datang terlambat, Tidak bisa hadir with a reason
// for the organizer), who comes, and Gabung for one held online.

// rsvpAPI keeps the user's answer to a meeting.
type rsvpAPI interface {
	MeetingRSVP(context.Context, id.ID, api.RSVPRequest) (api.Meeting, error)
}

func (a *App) rsvpService() (rsvpAPI, bool) {
	if c, ok := a.api.(*client.Client); ok {
		return c.AccountSession(), true
	}
	s, ok := a.api.(rsvpAPI)
	return s, ok
}

// rsvpState is the page's: the reason typed, a request on its way, and
// whether the whole list of people is shown.
type rsvpState struct {
	meeting  id.ID
	reason   string
	busy     bool
	err      string
	expanded bool
}

// meetingPeopleShown is how many people the side lists before "+N lainnya".
const meetingPeopleShown = 5

// openUntukMeeting shows a meeting's Rapat page.
func (a *App) openUntukMeeting(mid id.ID) {
	a.untuk.meeting = mid
	a.navigate(PathUntukRapat)
}

func (a *App) untukMeetingByID(mid id.ID) *api.Meeting {
	for i := range a.untuk.meetings {
		if a.untuk.meetings[i].ID == mid {
			return &a.untuk.meetings[i]
		}
	}
	return nil
}

// meetingSpan is the Waktu: "Kam, 12 Nov · 10.00–11.30".
func meetingSpan(m api.Meeting) string {
	loc := meetingLoc(m)
	start := m.StartsAt.In(loc)
	s := fmt.Sprintf("%s, %d %s · %s", shortDays[start.Weekday()], start.Day(), untukMonths[start.Month()-1], meetingClock(start))
	if m.EndsAt != nil {
		end := m.EndsAt.In(loc)
		if timeutil.DateOf(end) != timeutil.DateOf(start) {
			return s + " – " + fmt.Sprintf("%s, %d %s · %s", shortDays[end.Weekday()], end.Day(), untukMonths[end.Month()-1], meetingClock(end))
		}
		s += "–" + meetingClock(end)
	}
	return s
}

// meetingReminderText is the Pengingat: "30 menit dan 5 menit sebelumnya".
func meetingReminderText(minutes []int) string {
	s := meetingReminders(minutes)
	s = strings.Replace(s, ", dan ", " dan ", 1)
	if strings.HasSuffix(s, " sebelum") {
		s += "nya"
	}
	return s
}

func (a *App) untukMeetingView(c *ui.Context) {
	now := c.Now()
	a.ensureMyMeetings(now)
	m := a.untukMeetingByID(a.untuk.meeting)
	phone := isMobile(c)
	th := c.Theme()
	ui.Scroll(c.Key("untuk-meeting")).Fill().Children(func() {
		ui.Column(c).FillWidth().MaxWidth(1000).Padding(20, phoneSize(phone, 16, 32), 40, phoneSize(phone, 16, 32)).Gap(18).Children(func() {
			if backLink(c, "Untukku").Clicked() {
				a.navigate(PathUntukku)
			}
			if m == nil {
				if a.untuk.loading || !a.untuk.loaded {
					muted(c, "Memuat rapat…")
				} else {
					pageNotice(c, "calendar", "Rapat tidak tersedia", "Rapat ini sudah lewat, dibatalkan, atau tidak lagi mengundangmu.")
				}
				return
			}
			meeting := *m
			if a.untuk.rsvp.meeting != meeting.ID {
				a.untuk.rsvp = rsvpState{meeting: meeting.ID}
				if me := a.myParticipant(meeting); me != nil {
					a.untuk.rsvp.reason = me.RSVPNote
				}
			}
			ui.Row(c.Key("mt-h")).FillWidth().Gap(14).AlignItems(ui.Start).Wrap().Children(func() {
				ui.Column(c).Grow(1).MinWidth(240).Gap(4).Children(func() {
					ui.Row(c.Key("crumb")).Gap(8).AlignItems(ui.Center).Children(func() {
						for i, s := range a.access.spaces {
							if s.ID == meeting.SpaceID {
								spaceTile(c, s, i)
								muted(c, s.Name).FontSize(12)
								muted(c, "·").FontSize(12)
							}
						}
						muted(c, "Rapat").FontSize(12)
					})
					title := ui.Text(c, meeting.Title).FontSize(22).FontWeight(600)
					if meeting.Cancelled {
						title.TextColor(faint(th)).Strikethrough()
					}
				})
				ui.Row(c.Key("mt-acts")).Gap(8).Shrink(0).Children(func() {
					online := strings.TrimSpace(meeting.JoinURL) != ""
					if online && primarySpaceButton(c, "Gabung", "link").Clicked() && ui.SafeToOpen(meeting.JoinURL) {
						c.OpenURL(strings.TrimSpace(meeting.JoinURL))
					}
					var notes ui.Element
					if online {
						notes = spaceButton(c, "Catatan rapat", "note", false).Border(1, th.Border).TextColor(th.Text)
					} else {
						notes = primarySpaceButton(c, "Catatan rapat", "")
					}
					if notes.Tooltip("Buka rapat ini di Space-nya, dengan notulennya").Clicked() {
						a.OpenNotification(api.Delivery{SourceKind: "meeting", SourceID: meeting.ID})
					}
				})
			})
			if meeting.Cancelled {
				pageNotice(c, "alert", "Rapat dibatalkan", "Penyelenggara membatalkan rapat ini.")
			}
			grid := ui.Row(c.Key("mt-grid")).FillWidth().Gap(24).AlignItems(ui.Start)
			if phone {
				grid.Column()
			}
			grid.Children(func() {
				ui.Column(c.Key("mt-main")).Grow(1).MinWidth(0).FillWidth().Gap(18).Children(func() {
					detailBox(c, "facts", func() {
						factRow(c, "when", "Waktu", func() { ui.Text(c, meetingSpan(meeting)).Font(FontMono).FontSize(13) })
						factRow(c, "place", "Tempat", func() {
							place := meetingPlace(meeting)
							if place == "" {
								place = "Belum ditentukan"
							}
							ui.Text(c, place).FontSize(13)
						})
						factRow(c, "remind", "Pengingat", func() { ui.Text(c, meetingReminderText(meeting.ReminderMinutes)).FontSize(13) })
					})
					detailBox(c, "agenda", func() {
						boxTitle(c, "Agenda")
						if d := strings.TrimSpace(meeting.Description); d != "" {
							ui.Text(c, d).FontSize(13)
						} else {
							muted(c, "Belum ada agenda.").FontSize(13)
						}
					})
					if me := a.myParticipant(meeting); me != nil && !meeting.Cancelled {
						detailBox(c, "rsvp", func() { a.untukRSVP(c, meeting, *me) })
					}
				})
				aside := detailBox(c, "people", func() { a.untukMeetingPeople(c, meeting) })
				if !phone {
					aside.Width(340).Shrink(0)
				}
			})
		})
	})
}

func (a *App) myParticipant(m api.Meeting) *api.MeetingParticipant {
	for i := range m.Participants {
		if m.Participants[i].UserID == a.user.ID {
			return &m.Participants[i]
		}
	}
	return nil
}

// untukRSVP is Kehadiranmu: the three answers, and for late or absent a
// short reason the organizer reads.
func (a *App) untukRSVP(c *ui.Context, m api.Meeting, me api.MeetingParticipant) {
	th := c.Theme()
	st := &a.untuk.rsvp
	boxTitle(c, "Kehadiranmu")
	answers := []string{api.RSVPYes, api.RSVPLate, api.RSVPNo}
	labels := []string{"Hadir", "Datang terlambat", "Tidak bisa hadir"}
	selected := -1
	for i, v := range answers {
		if me.RSVP == v {
			selected = i
		}
	}
	ui.Row(c.Key("rsvp-seg")).Disabled(st.busy).Children(func() {
		if i := quietSeg(c, "Kehadiranmu", labels, nil, selected); i >= 0 && i != selected {
			note := ""
			if answers[i] != api.RSVPYes {
				note = st.reason
			}
			a.sendRSVP(m.ID, answers[i], note, "Kehadiran dicatat: "+labels[i])
		}
	})
	if me.RSVP == api.RSVPLate || me.RSVP == api.RSVPNo {
		ui.Row(c.Key("rsvp-reason")).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			in := ui.TextInput(c.Key("reason"), &st.reason).Label("Alasan").Placeholder("Alasan singkat untuk notulen (opsional)").Grow(1).MinWidth(0)
			changed := strings.TrimSpace(st.reason) != me.RSVPNote
			send := onbSecondary(c, "Kirim alasan", "Kirim", true).Disabled(st.busy || !changed)
			if (send.Clicked() || in.Submitted()) && changed {
				a.sendRSVP(m.ID, me.RSVP, st.reason, "Alasan dikirim ke penyelenggara")
			}
		})
		if me.RSVPNote != "" {
			muted(c, "Penyelenggara melihat: "+me.RSVPNote).FontSize(12)
		}
	}
	if st.err != "" {
		ui.Text(c, st.err).FontSize(12).TextColor(th.Danger)
	}
}

func (a *App) sendRSVP(mid id.ID, status, note, done string) {
	svc, ok := a.rsvpService()
	st := &a.untuk.rsvp
	if !ok || st.busy {
		return
	}
	gen := a.gen
	st.busy, st.err = true, ""
	a.async(func() func() {
		m, err := svc.MeetingRSVP(context.Background(), mid, api.RSVPRequest{Status: status, Note: strings.TrimSpace(note)})
		return func() {
			if gen != a.gen {
				return
			}
			st := &a.untuk.rsvp
			st.busy = false
			if err != nil {
				st.err = a.failed(err)
				return
			}
			for i := range a.untuk.meetings {
				if a.untuk.meetings[i].ID == m.ID {
					a.untuk.meetings[i] = m
				}
			}
			a.toast(done, nil)
		}
	})
}

// untukMeetingPeople is Peserta: the user first, as "(kamu)", then the
// rest, five at a time.
func (a *App) untukMeetingPeople(c *ui.Context, m api.Meeting) {
	st := &a.untuk.rsvp
	boxTitle(c, fmt.Sprintf("Peserta · %d", len(m.Participants)))
	people := make([]api.MeetingParticipant, 0, len(m.Participants))
	if me := a.myParticipant(m); me != nil {
		people = append(people, *me)
	}
	for _, p := range m.Participants {
		if p.UserID != a.user.ID {
			people = append(people, p)
		}
	}
	shown := people
	if len(shown) > meetingPeopleShown && !st.expanded {
		shown = shown[:meetingPeopleShown]
	}
	ui.Column(c.Key("mt-people")).FillWidth().Gap(8).Children(func() {
		if len(people) == 0 {
			muted(c, "Belum ada peserta.").FontSize(13)
		}
		for _, p := range shown {
			note := ""
			if p.UserID == a.user.ID {
				note = "(kamu)"
			}
			switch p.RSVP {
			case api.RSVPLate:
				note = strings.TrimSpace(note + " terlambat")
			case api.RSVPNo:
				note = strings.TrimSpace(note + " tidak bisa hadir")
			}
			personLine(c, p.UserID, p.DisplayName, note)
		}
	})
	if more := len(people) - len(shown); more > 0 {
		if onbLink(c, fmt.Sprintf("Tampilkan %d peserta lainnya", more), fmt.Sprintf("+%d lainnya", more)).AlignSelf(ui.Start).Clicked() {
			st.expanded = true
		}
	}
}
