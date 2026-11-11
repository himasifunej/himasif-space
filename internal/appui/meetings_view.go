package appui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/workspaceui"
)

// meetingTabs are the detail's sub-tabs.
var meetingTabs = [...]string{"Ringkasan", "Kehadiran", "Notulen"}

func (a *App) meetingsView(c *ui.Context) {
	a.ensureMeetings()
	if a.meetings.toast != "" {
		c.AddToast(ui.Toast{Title: a.meetings.toast, Type: "success", Timeout: 4 * time.Second})
		a.meetings.toast = ""
	}
	if st := &a.notes.summary; st.toast != "" && !a.notes.summary.open {
		c.AddToast(ui.Toast{Title: st.toast, Type: "success", Timeout: 5 * time.Second})
		st.toast = ""
	}
	ui.Row(c).Fill().MinWidth(0).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Children(func() { a.meetingsPage(c) })
		if a.meetings.draft != nil && !a.chrome.appearance {
			workspaceui.Drawer(c.Key("meeting-drawer"), &a.chrome.drawerWidth, func() {
				ui.Row(c).FillWidth().Padding(18).AlignItems(ui.Center).Children(func() {
					title := "Rapat baru"
					if a.meetings.draft.id != id.Nil {
						title = "Ubah rapat"
					}
					ui.Text(c, title).FontSize(18).Bold().Grow(1)
					if spaceIconButton(c, "Tutup formulir", "close").Disabled(a.meetings.busy).Clicked() {
						a.meetings.draft = nil
					}
				})
				if a.meetings.draft != nil {
					ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() { a.meetingForm(c) })
				}
			})
		}
	})
	if m := a.selectedMeeting(); m != nil {
		if a.meetings.confirmOpen {
			a.meetingCancelDialog(c, *m)
		}
		if st := &a.notes.summary; st.open && m.NoteID != nil && st.note == *m.NoteID {
			a.noteSummaryDialog(c)
		}
	}
}

// visibleMeetings are the meetings the list shows: all, or with "Saya
// diundang" only those the user organizes or is invited to.
func (a *App) visibleMeetings() []api.Meeting {
	if !a.meetings.mine {
		return a.meetings.items
	}
	out := []api.Meeting{}
	for _, m := range a.meetings.items {
		if meetingInvites(m, a.user.ID) {
			out = append(out, m)
		}
	}
	return out
}

// selectedMeeting is the meeting the detail shows; on a wide screen the
// default one when none is chosen.
func (a *App) selectedMeeting() *api.Meeting {
	items := a.visibleMeetings()
	if i := slices.IndexFunc(items, func(m api.Meeting) bool { return m.ID == a.meetings.selected }); i >= 0 {
		return &items[i]
	}
	if def := meetingDefault(items, time.Now()); def != id.Nil {
		i := slices.IndexFunc(items, func(m api.Meeting) bool { return m.ID == def })
		return &items[i]
	}
	return nil
}

func (a *App) canCreateMeeting() bool {
	sp := a.selectedSpace()
	return sp != nil && !sp.ReadOnly && !a.isAdmin() && slices.Contains(sp.Capabilities, api.CapTasksManage) && a.conn == connOnline
}

func (a *App) selectMeeting(mid id.ID) {
	if a.meetings.selected != mid {
		a.meetings.tab = 0
		a.meetings.participant = id.Nil
	}
	a.meetings.selected = mid
	a.meetings.mobileDetail = true
	a.meetings.menuOpen = false
}

func (a *App) meetingsPage(c *ui.Context) {
	t := c.Theme()
	mobile := isMobile(c)
	if mobile && a.meetings.mobileDetail {
		if m := a.selectedMeeting(); m != nil && m.ID == a.meetings.selected {
			a.meetingMobileDetail(c, *m)
			return
		}
		a.meetings.mobileDetail = false
	}
	ui.Column(c).Fill().MinWidth(0).Children(func() {
		pad := float32(32)
		if mobile {
			pad = 16
		}
		ui.Row(c).FillWidth().Padding(20, pad, 16, pad).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, "Rapat").FontSize(22).Bold()
				sub := ""
				if sp := a.selectedSpace(); sp != nil {
					sub = sp.Name + " · "
				}
				if a.meetings.loaded {
					sub += fmt.Sprintf("%d rapat periode ini", len(a.meetings.items))
				} else {
					sub += "Memuat rapat…"
				}
				muted(c, sub).FontSize(13)
			})
			if pick := segmented(c, "Tampilkan rapat", []string{"Semua", "Saya diundang"}, map[bool]int{false: 0, true: 1}[a.meetings.mine]); pick >= 0 {
				a.meetings.mine = pick == 1
			}
			if a.canCreateMeeting() {
				if outlineButton(c.Key("meeting-new"), "Buat rapat", "plus", true).Disabled(a.meetings.busy).Clicked() {
					a.beginMeeting(nil)
				}
			}
		})
		if a.meetings.err != "" {
			ui.Text(c, a.meetings.err).FontSize(13).TextColor(t.Danger).Padding(0, pad, 8, pad)
		}
		ui.Box(c).FillWidth().Height(1).Background(t.Border)
		if mobile {
			ui.Scroll(c.Key("meetings-list")).Grow(1).MinHeight(0).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Padding(8, 8, 24, 8).Children(func() { a.meetingList(c, true) })
			})
			return
		}
		listW := float32(340)
		if viewportMode(c) == layoutTablet {
			listW = 280
		}
		ui.Row(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
			ui.Scroll(c.Key("meetings-list")).Width(listW).Shrink(0).FillHeight().Children(func() {
				ui.Column(c).FillWidth().Padding(12).Children(func() { a.meetingList(c, false) })
			})
			ui.Box(c).Width(1).FillHeight().Background(t.Border)
			ui.Scroll(c.Key("meeting-detail")).Grow(1).MinWidth(0).FillHeight().Children(func() {
				ui.Column(c).FillWidth().MaxWidth(1040).Padding(24, 32, 40, 32).Gap(16).Children(func() {
					if m := a.selectedMeeting(); m != nil {
						a.meetingDetail(c, *m, false)
					} else if a.meetings.loaded {
						a.meetingsEmpty(c)
					}
				})
			})
		})
	})
}

func (a *App) meetingsEmpty(c *ui.Context) {
	ui.Column(c).FillWidth().Gap(8).Padding(40, 0).Children(func() {
		ui.Text(c, "Belum ada rapat").FontSize(16).Bold()
		if a.meetings.mine {
			muted(c, "Tidak ada rapat yang mengundang kamu. Pilih Semua untuk melihat rapat lain di Space ini.").FontSize(13)
		} else if a.canCreateMeeting() {
			muted(c, "Buat rapat pertama: tanggal, peserta, dan pengingat. Notulennya bisa ditautkan dari Catatan.").FontSize(13)
		} else {
			muted(c, "Rapat yang dibuat pengurus Space ini akan muncul di sini.").FontSize(13)
		}
	})
}

// meetingList draws the grouped list.
func (a *App) meetingList(c *ui.Context, mobile bool) {
	t := c.Theme()
	now := c.Now()
	items := a.visibleMeetings()
	if a.meetings.loading && !a.meetings.loaded {
		ui.Spinner(c).Label("Memuat rapat")
		return
	}
	if a.meetings.loaded && len(items) == 0 {
		if mobile {
			ui.Column(c).Padding(0, 8).Children(func() { a.meetingsEmpty(c) })
		} else {
			muted(c, "Belum ada rapat.").Padding(8)
		}
		return
	}
	sel := id.Nil
	if !mobile {
		if m := a.selectedMeeting(); m != nil {
			sel = m.ID
		}
	}
	for _, s := range meetingSections(items, now) {
		ui.Row(c.Key(fmt.Sprint("group-", s.group))).FillWidth().Padding(14, 10, 6, 10).Children(func() {
			ui.Text(c, strings.ToUpper(meetingGroupNames[s.group])).FontSize(11).FontWeight(600).TextColor(faint(t)).Grow(1)
			ui.Text(c, fmt.Sprint(len(s.items))).Font(FontMono).FontSize(11).TextColor(faint(t))
		})
		for _, m := range s.items {
			a.meetingRow(c, m, m.ID == sel, now)
		}
	}
}

func (a *App) meetingRow(c *ui.Context, m api.Meeting, selected bool, now time.Time) {
	t := c.Theme()
	loc := meetingLoc(m)
	start := m.StartsAt.In(loc)
	b := ui.ButtonBase(c.Key(m.ID)).Label(m.Title).FillWidth().Row().AlignItems(ui.Start).Gap(14).Padding(10, 12).MinHeight(56).Radius(10).Border(1, ui.RGBA(0, 0, 0, 0))
	switch {
	case selected:
		b.Background(accentSoft(t)).BorderColor(ui.RGBA(t.Accent.R, t.Accent.G, t.Accent.B, .45))
	case b.Hovered():
		b.Background(t.SurfaceHover)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		ui.Column(c).Width(32).Shrink(0).AlignItems(ui.Center).Gap(0).Children(func() {
			ui.Text(c, strings.ToUpper(shortDays[start.Weekday()])).FontSize(10).TextColor(faint(t))
			ui.Text(c, fmt.Sprintf("%02d", start.Day())).Font(FontMono).FontSize(18).FontWeight(500).TextColor(t.Text)
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			title := ui.Text(c, m.Title).FontSize(14).FontWeight(500).MaxLines(2)
			if m.Cancelled {
				title.TextColor(t.TextMuted)
			}
			meta, warn := meetingRowMeta(m, now)
			ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, meta).FontSize(12).TextColor(t.TextMuted).MaxLines(1).Shrink(1).MinWidth(0)
				if m.Cancelled {
					ui.Text(c, "Dibatalkan").FontSize(12).TextColor(t.Danger)
				}
				if warn != "" {
					ui.Text(c, warn).FontSize(12).TextColor(t.Warning)
				}
			})
		})
	})
	if b.Clicked() {
		a.selectMeeting(m.ID)
	}
}

// meetingDetail is the detail: status, title, the facts, the actions and
// the sub-tabs. On a phone the actions sit in the bottom bar instead.
func (a *App) meetingDetail(c *ui.Context, m api.Meeting, mobile bool) {
	t := c.Theme()
	now := c.Now()
	status, kind := meetingStatus(m, now)
	color := map[meetingStatusKind]ui.Color{statusSoon: t.Accent, statusLive: t.Success, statusDone: t.TextMuted, statusCancelled: t.Danger}[kind]
	if kind == statusSoon && m.StartsAt.Sub(now) > time.Hour {
		c.After(time.Minute)
	} else if kind == statusSoon {
		c.After(15 * time.Second)
	}
	ui.Column(c.Key("meeting-head")).FillWidth().Gap(6).Children(func() {
		ui.Text(c, status).FontSize(13).FontWeight(500).TextColor(color)
		ui.Text(c, m.Title).FontSize(22).Bold().FillWidth()
	})
	a.meetingFacts(c, m, mobile)
	if !mobile {
		ui.Row(c.Key("meeting-actions")).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			a.meetingPrimaryActions(c, m)
			a.meetingMoreMenu(c, m)
		})
	}
	if m.Timer != nil && !m.Cancelled {
		a.meetingTimerBar(c, m)
	}
	a.meetingTabBar(c, m)
	switch a.meetings.tab {
	case 1:
		a.meetingAttendance(c, m, mobile)
	case 2:
		a.meetingNotes(c, m)
	default:
		a.meetingOverview(c, m, mobile)
	}
}

// meetingFacts is the grid Waktu · Tempat · Tautan · Pengingat, two by two
// on a phone.
func (a *App) meetingFacts(c *ui.Context, m api.Meeting, mobile bool) {
	t := c.Theme()
	fact := func(key, label string, body func()) {
		grow := float32(1)
		if key == "when" {
			grow = 1.5
		}
		ui.Column(c.Key(key)).Grow(grow).Basis(0).MinWidth(140).Gap(4).Children(func() {
			ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
			body()
		})
	}
	ui.Column(c.Key("meeting-facts")).FillWidth().Gap(0).Children(func() {
		ui.Box(c).FillWidth().Height(1).Background(t.Border)
		ui.Row(c).FillWidth().Wrap().Gap(16).Padding(14, 0).Children(func() {
			fact("when", "Waktu", func() {
				ui.Text(c, meetingWhen(m)).Font(FontMono).FontSize(13)
				if m.EndsAt == nil {
					ui.Text(c, "Jam selesai belum diisi").FontSize(12).TextColor(faint(t))
				}
			})
			fact("where", "Tempat", func() {
				place := meetingPlace(m)
				if place == "" {
					ui.Text(c, "Belum diisi").FontSize(13).TextColor(faint(t))
				} else {
					ui.Text(c, place).FontSize(13)
				}
			})
			fact("link", "Tautan", func() {
				url := strings.TrimSpace(m.JoinURL)
				if url == "" {
					ui.Text(c, "Tidak ada").FontSize(13).TextColor(faint(t))
					return
				}
				b := ui.ButtonBase(c.Key("join")).Label("Buka tautan rapat").Tooltip(url).Row().Justify(ui.Start).AlignItems(ui.Center).FillWidth().MinWidth(0)
				b.FocusRing(b.FocusVisible()).Children(func() {
					txt := ui.Text(c, strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")).FontSize(13).TextColor(t.Accent).MaxLines(1)
					if b.Hovered() {
						txt.Underline()
					}
				})
				if b.Clicked() && ui.SafeToOpen(url) {
					c.OpenURL(url)
				}
			})
			if !mobile {
				fact("remind", "Pengingat", func() { ui.Text(c, meetingReminders(m.ReminderMinutes)).FontSize(13) })
			}
		})
		ui.Box(c).FillWidth().Height(1).Background(t.Border)
	})
}

// meetingPrimaryActions: Mulai rapat (the shared timer), Buka notulen and
// Ubah.
func (a *App) meetingPrimaryActions(c *ui.Context, m api.Meeting) {
	online := a.conn == connOnline
	if !m.Cancelled && m.Timer == nil && m.CanRunTimer {
		if outlineButton(c.Key("meeting-start"), "Mulai rapat", "play", true).Disabled(!online || a.notes.timerBusy).Tooltip("Mulai timer bersama: semua yang membuka rapat ini melihat waktu yang sama").Clicked() {
			a.runMeetingTimer(m.ID, m.TimerVersion, api.TimerStart)
		}
	}
	if m.NoteID != nil {
		if outlineButton(c.Key("meeting-note"), "Buka notulen", "note", false).Clicked() {
			a.openMeetingNote(m)
		}
	}
	if m.CanManage {
		b := ui.ButtonBase(c.Key("meeting-edit")).Label("Ubah").Row().AlignItems(ui.Center).Gap(8).Height(34).Padding(0, 10).Radius(8).TextColor(c.Theme().TextMuted)
		if b.Hovered() {
			b.Background(c.Theme().SurfaceHover).TextColor(c.Theme().Text)
		}
		b.Disabled(!online || a.meetings.busy).FocusRing(b.FocusVisible()).Children(func() {
			ui.Icon(c, spaceIcon("edit")).Size(15, 15)
			ui.Text(c, "Ubah").FontSize(13)
		})
		if b.Clicked() {
			a.beginMeeting(&m)
		}
	}
}

func (a *App) openMeetingNote(m api.Meeting) {
	if m.NoteID == nil {
		return
	}
	a.chrome.tab = "Catatan"
	a.ensureNotes()
	a.openNote(*m.NoteID)
}

// meetingCanSummarize: the meeting has a note the user may steer (the
// server checks the note's permission when it is sent).
func meetingCanSummarize(m api.Meeting) bool {
	return m.NoteID != nil && (m.CanManage || m.CanRunTimer)
}

func (a *App) summarizeMeeting(m api.Meeting) {
	if m.NoteID != nil {
		a.loadNoteSummary(*m.NoteID)
	}
}

// meetingMoreMenu is the … of the detail: copy the link, send the summary,
// cancel or make active again.
func (a *App) meetingMoreMenu(c *ui.Context, m api.Meeting) {
	t := c.Theme()
	_, summaryOK := a.noteSummaryService()
	hasLink := strings.TrimSpace(m.JoinURL) != ""
	canSummary := summaryOK && meetingCanSummarize(m) && !a.offlineMode()
	if !hasLink && !canSummary && !m.CanManage {
		return
	}
	anchor := spaceIconButton(c.Key("meeting-more"), "Lainnya", "more")
	if anchor.Clicked() {
		a.meetings.menuOpen = !a.meetings.menuOpen
	}
	popover(c.Key("meeting-menu"), anchor, &a.meetings.menuOpen, 260, func() {
		item := func(key, label, icon string, danger bool) bool {
			b := ui.ButtonBase(c.Key(key)).Label(label).FillWidth().Row().AlignItems(ui.Center).Gap(10).Height(34).Padding(0, 10).Radius(8).TextColor(t.Text)
			if danger {
				b.TextColor(t.Danger)
			}
			if b.Hovered() {
				b.Background(t.SurfaceHover)
			}
			b.FocusRing(b.FocusVisible()).Children(func() {
				ui.Icon(c, spaceIcon(icon)).Size(16, 16)
				ui.Text(c, label).FontSize(13)
			})
			return b.Clicked()
		}
		if hasLink && item("copy", "Salin tautan rapat", "link", false) {
			c.WriteClipboard(strings.TrimSpace(m.JoinURL))
			a.meetings.menuOpen = false
			a.meetings.toast = "Tautan rapat disalin."
		}
		if canSummary && item("wa", "Kirim ringkasan ke WhatsApp", "whatsapp", false) {
			a.meetings.menuOpen = false
			a.summarizeMeeting(m)
		}
		if m.CanManage && a.conn == connOnline {
			if m.Cancelled {
				if item("restore", "Aktifkan kembali", "undo", false) {
					a.meetings.menuOpen = false
					a.setMeetingCancelled(m, false)
				}
			} else if item("cancel", "Batalkan rapat…", "close", true) {
				a.meetings.menuOpen = false
				a.meetings.confirmOpen = true
			}
		}
	})
}

func (a *App) meetingCancelDialog(c *ui.Context, m api.Meeting) {
	t := c.Theme()
	ui.DialogBase(c.Key("meeting-cancel"), &a.meetings.confirmOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		panel.Width(min(float32(440), mustWidth(c)-32)).Padding(20).Gap(12).Radius(14).Background(t.Surface).Border(1, t.Border).Label("Batalkan rapat").Children(func() {
			ui.Text(c, "Batalkan rapat ini?").FontSize(16).Bold()
			ui.Text(c, m.Title+" · "+meetingWhen(m)).FontSize(13).TextColor(t.TextMuted)
			ui.Text(c, "Rapat tetap ada di daftar dengan tanda Dibatalkan, dan bisa diaktifkan kembali dari menu yang sama.").FontSize(13).LineHeight(1.45)
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				if spaceButton(c.Key("cancel-back"), "Kembali", "", false).Clicked() {
					a.meetings.confirmOpen = false
				}
				b := outlineButton(c.Key("cancel-yes"), "Batalkan rapat", "close", false).TextColor(t.Danger).BorderColor(t.Danger).Disabled(a.meetings.busy)
				if b.Clicked() {
					a.setMeetingCancelled(m, true)
				}
			})
		})
	})
}

// meetingTimerBar shows the shared timer while it runs: the running time,
// who steers it and, for them, Jeda / Lanjutkan and Hentikan. The agenda
// steps are in the note's Rapat panel, where the agenda is.
func (a *App) meetingTimerBar(c *ui.Context, m api.Meeting) {
	t := c.Theme()
	now := c.Now()
	tm := sharedNoteTimer(*m.Timer, a.meetings.skew)
	paused := !tm.pausedAt.IsZero()
	if !paused {
		c.After(time.Second)
	}
	ui.Row(c.Key("meeting-timer")).FillWidth().Wrap().Gap(12).Padding(12, 16).Radius(10).Border(1, t.Border).Background(t.Surface).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, spaceIcon("timer")).Size(18, 18)
		ui.Text(c, noteClock(tm.running(now))).Font(FontMono).FontSize(20).FontWeight(600)
		sub := "Timer bersama berjalan"
		if paused {
			sub = "Timer bersama dijeda"
		}
		if m.Timer.Agenda > 0 {
			sub += fmt.Sprintf(" · agenda %d", m.Timer.Agenda+1)
		}
		if by := m.Timer.UpdatedBy.DisplayName; by != "" {
			sub += " · diatur " + by
		}
		muted(c, sub).FontSize(13).Grow(1).MinWidth(0)
		if !m.CanRunTimer {
			return
		}
		busy := a.notes.timerBusy || a.conn != connOnline
		if paused {
			if outlineButton(c.Key("timer-resume"), "Lanjutkan", "play", false).Height(30).Disabled(busy).Clicked() {
				a.runMeetingTimer(m.ID, m.TimerVersion, api.TimerResume)
			}
		} else if outlineButton(c.Key("timer-pause"), "Jeda", "pause", false).Height(30).Disabled(busy).Clicked() {
			a.runMeetingTimer(m.ID, m.TimerVersion, api.TimerPause)
		}
		if outlineButton(c.Key("timer-stop"), "Hentikan", "close", false).Height(30).Disabled(busy).Clicked() {
			a.runMeetingTimer(m.ID, m.TimerVersion, api.TimerStop)
		}
	})
	if a.notes.timerErr != "" {
		ui.Text(c, a.notes.timerErr).FontSize(12).TextColor(t.Danger)
	}
}

func (a *App) meetingTabBar(c *ui.Context, m api.Meeting) {
	t := c.Theme()
	tally := meetingTallyOf(m)
	ui.Column(c.Key("meeting-tabs")).FillWidth().Gap(0).Children(func() {
		ui.Row(c).FillWidth().Gap(4).Children(func() {
			for i, name := range meetingTabs {
				label := name
				if i == 1 && len(m.Participants) > 0 {
					label = fmt.Sprintf("%s · %d/%d", name, tally.present, len(m.Participants))
				}
				b := ui.ButtonBase(c.Key(name)).Label(label).Column().AlignItems(ui.Center).Gap(0).Padding(0, 12).Height(40).Justify(ui.End).TextColor(t.TextMuted)
				on := a.meetings.tab == i
				if on {
					b.TextColor(t.Text)
				} else if b.Hovered() {
					b.TextColor(t.Text)
				}
				b.FocusRing(b.FocusVisible()).Children(func() {
					txt := ui.Text(c, label).FontSize(14).MaxLines(1).Padding(0, 0, 10, 0)
					if on {
						txt.FontWeight(600)
					}
					ink := ui.Box(c).FillWidth().Height(2).Radius(1)
					if on {
						ink.Background(t.Accent)
					}
				})
				if b.Clicked() {
					a.meetings.tab = i
					a.meetings.participant = id.Nil
				}
			}
		})
		ui.Box(c).FillWidth().Height(1).Background(t.Border)
	})
}

// meetingOverview is Ringkasan: the purpose and agenda beside the people.
func (a *App) meetingOverview(c *ui.Context, m api.Meeting, mobile bool) {
	t := c.Theme()
	purpose := func() {
		col := ui.Column(c.Key("purpose")).MinWidth(0).Gap(8)
		if mobile {
			col.FillWidth()
		} else {
			col.Grow(1).Basis(0)
		}
		col.Children(func() {
			ui.Text(c, "TUJUAN DAN AGENDA").FontSize(11).FontWeight(600).TextColor(faint(t))
			if d := strings.TrimSpace(m.Description); d != "" {
				ui.Text(c, d).FontSize(14).LineHeight(1.55).Selectable().FillWidth()
			} else {
				muted(c, "Belum ada tujuan atau agenda.").FontSize(13)
			}
			if mobile {
				ui.Text(c, "Pengingat: "+meetingReminders(m.ReminderMinutes)).FontSize(12).TextColor(t.TextMuted)
			}
		})
	}
	people := func() {
		w := float32(320)
		col := ui.Column(c.Key("people")).Gap(0).Shrink(0)
		if mobile {
			col.FillWidth()
		} else {
			col.Width(w)
		}
		col.Children(func() {
			ui.Text(c, fmt.Sprintf("PESERTA · %d", len(m.Participants))).FontSize(11).FontWeight(600).TextColor(faint(t)).Padding(0, 0, 8, 0)
			if len(m.Participants) == 0 {
				muted(c, "Belum ada peserta.").FontSize(13)
			}
			for _, p := range m.Participants {
				ui.Row(c.Key(p.UserID)).FillWidth().Gap(10).Padding(8, 0).AlignItems(ui.Center).Children(func() {
					avatar(c, p.DisplayName, 28)
					ui.Text(c, p.DisplayName).FontSize(14).Grow(1).MinWidth(0).MaxLines(1)
					status := ui.Text(c, meetingStatusLabel(p.AttendanceStatus)).FontSize(12).TextColor(t.TextMuted)
					if p.AttendanceStatus == "present" {
						status.TextColor(t.Success)
					}
				})
				ui.Box(c.Key(fmt.Sprint(p.UserID, "-rule"))).FillWidth().Height(1).Background(t.Border)
			}
			if m.OrganizerID == a.user.ID && len(m.Participants) > 0 {
				b := ui.ButtonBase(c.Key("to-attendance")).Label("Catat kehadiran").Row().AlignItems(ui.Center).Gap(4).Padding(10, 0).TextColor(t.Accent)
				b.FocusRing(b.FocusVisible()).Children(func() {
					ui.Text(c, "Catat kehadiran").FontSize(13)
					ui.Icon(c, spaceIcon("right")).Size(14, 14)
				})
				if b.Clicked() {
					a.meetings.tab = 1
				}
			}
		})
	}
	if mobile {
		ui.Column(c.Key("overview")).FillWidth().Gap(20).Children(func() { purpose(); people() })
		return
	}
	ui.Row(c.Key("overview")).FillWidth().Gap(40).AlignItems(ui.Start).Children(func() { purpose(); people() })
}

// meetingAttendance is Kehadiran: one row a person, Hadir / Izin / Tidak
// hadir right in the row for the organizer (the only one the server lets
// record it), the arrival and note under the name.
func (a *App) meetingAttendance(c *ui.Context, m api.Meeting, mobile bool) {
	t := c.Theme()
	now := c.Now()
	tally := meetingTallyOf(m)
	organizer := m.OrganizerID == a.user.ID
	can := organizer && !m.Cancelled && a.conn == connOnline
	ui.Column(c.Key("attendance")).FillWidth().Gap(0).Children(func() {
		ui.Text(c, fmt.Sprintf("%d hadir · %d izin · %d tidak hadir · %d belum dicatat", tally.present, tally.excused, tally.absent, tally.unrecorded)).FontSize(13).TextColor(t.TextMuted).Padding(0, 0, 8, 0)
		switch {
		case !organizer:
			muted(c, "Kehadiran dicatat penyelenggara rapat.").FontSize(12).Padding(0, 0, 8, 0)
		case now.Before(m.StartsAt) && !m.Cancelled:
			muted(c, "Hadir bisa dicatat setelah rapat dimulai; Izin bisa dicatat sekarang.").FontSize(12).Padding(0, 0, 8, 0)
		}
		if len(m.Participants) == 0 {
			muted(c, "Belum ada peserta. Tambahkan lewat Ubah.").FontSize(13)
		}
		for _, p := range m.Participants {
			a.meetingAttendanceRow(c, m, p, can, mobile, now)
		}
	})
}

func (a *App) meetingAttendanceRow(c *ui.Context, m api.Meeting, p api.MeetingParticipant, can, mobile bool, now time.Time) {
	t := c.Theme()
	loc := meetingLoc(m)
	editing := a.meetings.participant == p.UserID
	info := func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, p.DisplayName).FontSize(14).MaxLines(1)
			parts := []string{}
			if p.ArrivedAt != nil {
				parts = append(parts, "datang "+meetingClock(p.ArrivedAt.In(loc)))
			}
			if n := strings.TrimSpace(p.AttendanceNote); n != "" {
				parts = append(parts, n)
			}
			if len(parts) > 0 {
				line := strings.Join(parts, " · ")
				if can {
					b := ui.ButtonBase(c.Key("edit-line")).Label("Ubah waktu datang atau catatan").Tooltip("Ubah waktu datang atau catatan").Row().Justify(ui.Start).MinWidth(0)
					b.FocusRing(b.FocusVisible()).Children(func() {
						txt := ui.Text(c, line).FontSize(12).TextColor(t.TextMuted).MaxLines(1)
						if b.Hovered() {
							txt.Underline()
						}
					})
					if b.Clicked() {
						a.beginAttendanceEdit(m, p, p.AttendanceStatus)
					}
				} else {
					ui.Text(c, line).FontSize(12).TextColor(t.TextMuted).MaxLines(2)
				}
			} else if !can {
				ui.Text(c, meetingStatusLabel(p.AttendanceStatus)).FontSize(12).TextColor(t.TextMuted)
			}
		})
	}
	choice := func() {
		if !can {
			if mobile {
				return
			}
			s := ui.Text(c, meetingStatusLabel(p.AttendanceStatus)).FontSize(13).TextColor(t.TextMuted)
			if p.AttendanceStatus == "present" {
				s.TextColor(t.Success)
			}
			return
		}
		statuses := []string{"present", "excused", "absent"}
		selected := slices.Index(statuses, p.AttendanceStatus)
		pick := segmented(c.Key("status"), "Kehadiran "+p.DisplayName, []string{"Hadir", "Izin", "Tidak hadir"}, selected)
		if pick < 0 || a.meetings.busy {
			return
		}
		status := statuses[pick]
		if status == p.AttendanceStatus {
			a.beginAttendanceEdit(m, p, status)
			return
		}
		if status == "present" {
			at, ok := meetingArrivalDefault(m, now)
			if p.ArrivedAt != nil {
				at, ok = *p.ArrivedAt, true
			}
			if !ok {
				a.meetings.err = "Hadir bisa dicatat setelah rapat dimulai."
				return
			}
			a.attendance(m.ID, p.UserID, m.Version, status, at.In(loc).Format("2006-01-02 15:04"), p.AttendanceNote)
			return
		}
		a.attendance(m.ID, p.UserID, m.Version, status, "", p.AttendanceNote)
	}
	ui.Column(c.Key(p.UserID)).FillWidth().Gap(0).Children(func() {
		if mobile {
			ui.Column(c).FillWidth().Gap(8).Padding(12, 0).Children(func() {
				ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
					avatar(c, p.DisplayName, 32)
					info()
				})
				choice()
			})
		} else {
			ui.Row(c).FillWidth().Gap(12).Padding(10, 0).AlignItems(ui.Center).Children(func() {
				avatar(c, p.DisplayName, 32)
				info()
				choice()
			})
		}
		if editing && can {
			a.meetingAttendanceEditor(c, m, p)
		}
		ui.Box(c).FillWidth().Height(1).Background(t.Border)
	})
}

// beginAttendanceEdit opens the arrival and note of one person under the
// row, for the status chosen.
func (a *App) beginAttendanceEdit(m api.Meeting, p api.MeetingParticipant, status string) {
	if a.meetings.participant == p.UserID {
		a.meetings.participant = id.Nil
		return
	}
	a.meetings.participant = p.UserID
	a.meetings.note = p.AttendanceNote
	a.meetings.arrival = ""
	at := p.ArrivedAt
	if at == nil {
		if def, ok := meetingArrivalDefault(m, time.Now()); ok {
			at = &def
		}
	}
	if at != nil {
		a.meetings.arrival = at.In(meetingLoc(m)).Format("2006-01-02 15:04")
	}
}

func (a *App) meetingAttendanceEditor(c *ui.Context, m api.Meeting, p api.MeetingParticipant) {
	t := c.Theme()
	ui.Column(c.Key("editor")).FillWidth().Gap(10).Padding(12, 14).Margin(0, 0, 12, 0).Radius(10).Background(t.Surface).Border(1, t.Border).Children(func() {
		if p.AttendanceStatus == "present" || p.AttendanceStatus == "" || p.AttendanceStatus == "unrecorded" {
			ui.Field(c, "Waktu datang · zona "+m.TimeZone, func() {
				ui.TextInput(c.Key("arrival"), &a.meetings.arrival).Label("Waktu datang").Placeholder("2026-11-05 19:00").FillWidth()
			})
		}
		ui.Field(c, "Catatan · opsional", func() {
			ui.TextInput(c.Key("attendance-note"), &a.meetings.note).Label("Catatan kehadiran").Placeholder("Contoh: terlambat karena kelas").FillWidth()
		})
		ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			status := p.AttendanceStatus
			if status == "" || status == "unrecorded" {
				status = "present"
			}
			if outlineButton(c.Key("save"), "Simpan", "check", true).Height(30).Disabled(a.meetings.busy).Clicked() {
				arrival := ""
				if status == "present" {
					arrival = a.meetings.arrival
				}
				a.attendance(m.ID, p.UserID, m.Version, status, arrival, a.meetings.note)
			}
			if outlineButton(c.Key("close"), "Batal", "", false).Height(30).Clicked() {
				a.meetings.participant = id.Nil
			}
			ui.Box(c).Grow(1)
			if p.AttendanceStatus != "" && p.AttendanceStatus != "unrecorded" {
				b := ui.ButtonBase(c.Key("clear")).Label("Hapus catatan hadir").Padding(4, 6).Radius(6).TextColor(t.Danger)
				b.FocusRing(b.FocusVisible()).Children(func() { ui.Text(c, "Hapus catatan hadir").FontSize(12) })
				if b.Clicked() && !a.meetings.busy {
					a.attendance(m.ID, p.UserID, m.Version, "unrecorded", "", "")
				}
			}
		})
	})
}

// meetingNotes is Notulen: the linked note, opening it and sending its
// summary; without one, how to add it.
func (a *App) meetingNotes(c *ui.Context, m api.Meeting) {
	t := c.Theme()
	ui.Column(c.Key("notes")).FillWidth().Gap(12).Children(func() {
		if m.NoteID == nil {
			muted(c, "Belum ada notulen terkait rapat ini.").FontSize(13)
			if m.CanManage {
				ui.Row(c).Wrap().Gap(8).Children(func() {
					if outlineButton(c.Key("link-note"), "Tautkan notulen", "link", false).Disabled(a.conn != connOnline).Clicked() {
						a.beginMeeting(&m)
					}
					if outlineButton(c.Key("new-note"), "Buat notulensi di Catatan", "plus", false).Clicked() {
						a.chrome.tab = "Catatan"
						a.ensureNotes()
						a.beginNote()
						a.notes.templateKind = api.TemplateMeeting
						a.notes.newTitle = "Notulensi · " + m.Title
					}
				})
				muted(c, "Catatan baru belum tertaut otomatis: setelah dibuat, tautkan lewat Ubah.").FontSize(12)
			}
			return
		}
		title := "Notulen rapat"
		if i := slices.IndexFunc(a.meetings.notes, func(n api.Note) bool { return n.ID == *m.NoteID }); i >= 0 {
			title = a.meetings.notes[i].Title
		}
		ui.Row(c).FillWidth().Gap(12).Padding(14, 16).Radius(10).Border(1, t.Border).Background(t.Surface).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, spaceIcon("note")).Size(18, 18)
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, title).FontSize(14).FontWeight(500).MaxLines(1)
				muted(c, "Agenda dengan durasi, keputusan, dan action item ditulis di sini.").FontSize(12)
			})
			if outlineButton(c.Key("open-note"), "Buka", "note", false).Height(30).Clicked() {
				a.openMeetingNote(m)
			}
		})
		if _, ok := a.noteSummaryService(); ok && meetingCanSummarize(m) && !a.offlineMode() {
			ui.Row(c).FillWidth().Wrap().Gap(12).AlignItems(ui.Center).Children(func() {
				if outlineButton(c.Key("wa-summary"), "Kirim ringkasan ke WhatsApp", "whatsapp", false).Disabled(a.notes.summary.busy).Clicked() {
					a.summarizeMeeting(m)
				}
				muted(c, "Keputusan dan action item (PJ + tenggat) dikirim bot ke grup WhatsApp divisi. Kamu melihat pesannya dulu.").FontSize(12).Grow(1).Basis(0).MinWidth(200)
			})
		}
	})
}

// meetingMobileDetail is the phone's detail page: back, the detail, and
// the actions in a bar at the bottom.
func (a *App) meetingMobileDetail(c *ui.Context, m api.Meeting) {
	t := c.Theme()
	ui.Column(c.Key("meeting-mobile")).Fill().MinWidth(0).Children(func() {
		ui.Row(c).FillWidth().Height(48).Padding(0, 4).Gap(4).AlignItems(ui.Center).Children(func() {
			if spaceIconButton(c.Key("meeting-back"), "Kembali ke daftar rapat", "left").Size(44, 44).Clicked() {
				a.meetings.mobileDetail = false
				a.meetings.menuOpen = false
			}
			ui.Text(c, "Rapat").FontSize(15).FontWeight(600).Grow(1)
			a.meetingMoreMenu(c, m)
		})
		ui.Box(c).FillWidth().Height(1).Background(t.Border)
		if a.meetings.err != "" {
			ui.Text(c, a.meetings.err).FontSize(13).TextColor(t.Danger).Padding(8, 16)
		}
		ui.Scroll(c.Key("meeting-mobile-body")).Grow(1).MinHeight(0).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(16, 16, 24, 16).Gap(16).Children(func() { a.meetingDetail(c, m, true) })
		})
		online := a.conn == connOnline
		start := !m.Cancelled && m.Timer == nil && m.CanRunTimer
		if !start && m.NoteID == nil && !m.CanManage {
			return
		}
		ui.Box(c).FillWidth().Height(1).Background(t.Border)
		ui.Row(c.Key("meeting-bar")).FillWidth().Gap(8).Padding(10, 16).Background(t.Surface).Children(func() {
			if start {
				if outlineButton(c.Key("bar-start"), "Mulai rapat", "play", true).Height(44).Grow(1).Justify(ui.Center).Disabled(!online || a.notes.timerBusy).Clicked() {
					a.runMeetingTimer(m.ID, m.TimerVersion, api.TimerStart)
				}
			}
			if m.NoteID != nil {
				b := outlineButton(c.Key("bar-note"), "Notulen", "note", false).Height(44).Justify(ui.Center)
				if !start {
					b.Grow(1)
				}
				if b.Clicked() {
					a.openMeetingNote(m)
				}
			}
			if m.CanManage {
				b := outlineButton(c.Key("bar-edit"), "Ubah", "edit", false).Height(44).Justify(ui.Center).Disabled(!online || a.meetings.busy)
				if !start && m.NoteID == nil {
					b.Grow(1)
				}
				if b.Clicked() {
					a.beginMeeting(&m)
				}
			}
		})
	})
}
