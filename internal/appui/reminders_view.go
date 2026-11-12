package appui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

// Reminder pribadi, Form B "Kalimat" of canvas 3747dbbac0d0 (the owner's
// pick, 12 Nov 2026): a list with round circles to pick several and finish
// them at once, the chosen reminder as a card to read with Tandai selesai,
// Ubah and ⋯, and a dialog that reads as a sentence, "Ingatkan aku untuk …
// pada [tanggal] pukul [jam] WIB". Always WIB: there is no zone to choose.
// The date and time open Space's own picker (when_field.go).

// reminderPlaceholders rotate as the title's example, one per new reminder.
var reminderPlaceholders = [...]string{
	"pesan tiket kereta pulang",
	"kumpulkan berkas LPJ ke bendahara",
	"bayar kas bulan ini",
	"cek revisi desain poster",
	"siapkan materi rapat divisi",
	"balas chat panitia Makrab",
	"kembalikan kunci sekre",
	"upload dokumentasi kegiatan",
}

// reminderWhenKey names the reminder dialog's date-and-time picker.
const reminderWhenKey = "reminder-when"

// Tabs of the list.
const (
	reminderTabActive = iota
	reminderTabDone
	reminderTabShared
)

// wib is where every reminder is set and shown.
func wib() *time.Location {
	loc, err := timeutil.LoadZone(timeutil.DefaultZoneName)
	if err != nil {
		return time.FixedZone("WIB", 7*3600)
	}
	return loc
}

// reminderDue is when a reminder fires: its instant, or 06.00 WIB of its
// date; timed tells which.
func reminderDue(r api.Reminder) (at time.Time, timed, ok bool) {
	switch {
	case r.DueAt != nil:
		return *r.DueAt, true, true
	case r.DueOn != nil:
		return r.DueOn.At(6, 0, wib()), false, true
	}
	return time.Time{}, false, false
}

// reminderShortWhen is "Sen, 2 Nov · 23.20", or "Sen, 2 Nov" for a date
// alone.
func reminderShortWhen(r api.Reminder) string {
	at, timed, ok := reminderDue(r)
	if !ok {
		return "Tanpa waktu"
	}
	at = at.In(wib())
	s := fmt.Sprintf("%s, %d %s", shortDays[at.Weekday()], at.Day(), untukMonths[at.Month()-1])
	if timed {
		s += " · " + at.Format("15.04")
	}
	return s
}

// reminderLate: not done and its time has passed.
func reminderLate(r api.Reminder, now time.Time) bool {
	at, _, ok := reminderDue(r)
	return ok && !r.Completed && at.Before(now)
}

// reminderOwned: the user's own, which they may change.
func (a *App) reminderOwned(r api.Reminder) bool {
	return r.Relation == api.ReminderOwner && r.OwnerID == a.user.ID
}

// reminderTabItems are the reminders of a tab that match the search, by
// time.
func (a *App) reminderTabItems(tab int, query string) []api.Reminder {
	q := strings.ToLower(strings.TrimSpace(query))
	var out []api.Reminder
	for _, r := range a.reminders.items {
		owned := a.reminderOwned(r)
		switch tab {
		case reminderTabActive:
			if !owned || r.Completed {
				continue
			}
		case reminderTabDone:
			if !owned || !r.Completed {
				continue
			}
		case reminderTabShared:
			if owned {
				continue
			}
		}
		if q != "" && !strings.Contains(strings.ToLower(r.Title+" "+r.Notes), q) {
			continue
		}
		out = append(out, r)
	}
	sortReminders(out)
	return out
}

func sortReminders(rs []api.Reminder) {
	for i := 1; i < len(rs); i++ {
		for j := i; j > 0; j-- {
			a, _, _ := reminderDue(rs[j-1])
			b, _, _ := reminderDue(rs[j])
			if !b.Before(a) {
				break
			}
			rs[j-1], rs[j] = rs[j], rs[j-1]
		}
	}
}

// reminderBucket is the heading a reminder of the Aktif tab falls under.
func reminderBucket(r api.Reminder, now time.Time) string {
	if reminderLate(r, now) {
		return "Lewat waktu"
	}
	at, _, _ := reminderDue(r)
	today := startOfDay(now, wib())
	switch day := startOfDay(at, wib()); {
	case day.Equal(today):
		return "Hari ini"
	case day.Equal(today.AddDate(0, 0, 1)):
		return "Besok"
	}
	return "Nanti"
}

func (a *App) remindersView(c *ui.Context) {
	if a.isAdmin() {
		pageBody(c, "Reminder pribadi", func() {
			pageNotice(c, "lock", "Gunakan akun pribadi", "Akun organisasi tidak dapat mengakses reminder pribadi pengurus.")
		})
		return
	}
	a.ensureReminders()
	st := &a.reminders
	if st.pendingNew && st.loaded && st.prefsLoaded {
		st.pendingNew = false
		a.beginReminder()
	}
	phone := isMobile(c)
	// The card shows the first reminder once the list is in, so the page
	// is never an empty half.
	if !phone && st.loaded && !st.autoPicked {
		st.autoPicked = true
		if st.selected == id.Nil && st.draft == nil {
			if first := a.firstReminder(); first != id.Nil {
				a.openReminder(first)
			}
		}
	}
	t := c.Theme()
	ui.Column(c).Grow(1).MinHeight(0).FillWidth().MinWidth(0).Children(func() {
		ui.Row(c.Key("rm-head")).FillWidth().MinWidth(0).Padding(phoneSize(phone, 16, 20), phoneSize(phone, 16, 32), 16, phoneSize(phone, 16, 32)).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				back := "Untukku"
				if st.from == PathJobdesk {
					back = "Jobdesk saya"
				}
				if backLink(c, back).Clicked() {
					a.navigate(firstNonEmpty(st.from, PathUntukku))
				}
				ui.Text(c, "Reminder pribadi").FontSize(20).FontWeight(600).Padding(6, 0, 0, 0)
				ui.Row(c).Gap(6).AlignItems(ui.Center).MinWidth(0).Children(func() {
					ui.Icon(c, spaceIcon("lock")).Size(14, 14).TextColor(t.TextMuted).Shrink(0)
					muted(c, "Hanya untukmu. Dikirim ke perangkatmu, tidak pernah ke grup WhatsApp.").FontSize(13).MinWidth(0)
				})
			})
			if primarySpaceButton(c, "Reminder baru", "plus").Shrink(0).Disabled(a.reminderCreateBlocked() || !st.loaded || !st.prefsLoaded).Clicked() {
				a.beginReminder()
			}
		})
		a.reminderFeedback(c)
		panelOpen := st.selected != id.Nil
		ui.Row(c.Key("rm-body")).Grow(1).MinHeight(0).FillWidth().MinWidth(0).AlignItems(ui.Stretch).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
			list := ui.Column(c.Key("rm-list")).MinWidth(0).MinHeight(0).FillHeight()
			switch {
			case phone && panelOpen:
				list.Width(0).Hidden(true)
			case phone:
				list.Grow(1)
			default:
				list.Width(380).Shrink(0).BorderWidth(0, 1, 0, 0).BorderColor(t.Border)
			}
			list.Children(func() { a.reminderListColumn(c) })
			if !phone || panelOpen {
				ui.Column(c.Key("rm-panel")).Grow(1).MinWidth(0).MinHeight(0).FillHeight().Label("Panel reminder").Children(func() { a.reminderPanel(c, phone) })
			}
		})
	})
	a.reminderDialog(c)
}

func (a *App) reminderFeedback(c *ui.Context) {
	st := &a.reminders
	if st.err != "" {
		ui.Row(c.Key("rm-err")).FillWidth().Gap(8).AlignItems(ui.Center).Padding(0, 32, 10, 32).Children(func() {
			ui.Text(c, st.err).FontSize(12).TextColor(c.Theme().Danger).Grow(1).MinWidth(0)
			if spaceButton(c, "Muat ulang reminder", "refresh", false).Disabled(st.busy || st.loading).Clicked() {
				a.loadReminders()
			}
		})
	}
	if st.notice != "" {
		a.toast(st.notice, nil)
		st.notice = ""
	}
	if a.conn == connOffline {
		muted(c, "Tidak terhubung. Data terakhir ditampilkan; perubahan belum dikirim ke cloud.").Padding(0, 32, 10, 32)
	}
}

// reminderListColumn is the left side: search, the tabs, the reminders by
// when, and the bar for the ones picked.
func (a *App) reminderListColumn(c *ui.Context) {
	t := c.Theme()
	st := &a.reminders
	now := c.Now()
	ui.Column(c.Key("rm-tools")).FillWidth().Padding(14, 16, 10, 16).Gap(10).Children(func() {
		ui.Row(c).FillWidth().Height(34).Gap(8).AlignItems(ui.Center).Padding(0, 10).Radius(8).Border(1, t.Border).Children(func() {
			ui.Icon(c, spaceIcon("search")).Size(16, 16).TextColor(faint(t)).Shrink(0)
			ui.TextInputBase(c.Key("reminder-filter"), &st.filter).Grow(1).MinWidth(0).FontSize(13).Label("Cari reminder").Placeholder("Cari judul atau catatan…")
		})
		if i := quietSeg(c, "Daftar reminder", []string{"Aktif", "Selesai", "Dibagikan kepadaku"}, nil, st.tab); i >= 0 && i != st.tab {
			st.tab, st.picked = i, nil
		}
	})
	items := a.reminderTabItems(st.tab, st.filter)
	ui.Scroll(c.Key("rm-items")).Grow(1).MinHeight(0).FillWidth().Label("Daftar reminder pribadi").Children(func() {
		ui.Column(c).FillWidth().MinWidth(0).Padding(0, 10, 16, 10).Children(func() {
			if !st.loaded {
				title, detail := "Memuat reminder…", "Mengambil reminder milikmu dan yang dibagikan kepadamu."
				if st.err != "" {
					title, detail = "Reminder belum dapat dimuat", "Gunakan Muat ulang untuk mencoba kembali."
				}
				pageEmpty(c, "bell", title, detail)
				return
			}
			if len(items) == 0 {
				title, detail := "Belum ada reminder", "Buat pengingat pribadi pertamamu."
				switch {
				case strings.TrimSpace(st.filter) != "":
					title, detail = "Reminder tidak ditemukan", "Coba judul atau catatan lain."
				case st.tab == reminderTabDone:
					title = "Belum ada yang selesai"
				case st.tab == reminderTabShared:
					title, detail = "Belum ada yang dibagikan kepadamu", "Reminder yang dibagikan pengurus lain muncul di sini, hanya untuk dibaca."
				}
				ui.Column(c.Key("rm-empty")).FillWidth().Padding(40, 8).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, title).FontSize(15).FontWeight(600)
					muted(c, detail).FontSize(13)
				})
				return
			}
			last := ""
			for _, r := range items {
				if st.tab == reminderTabActive {
					if g := reminderBucket(r, now); g != last {
						head := ui.Text(c.Key("g-"+g), g).FontSize(12).FontWeight(600).TextColor(t.TextMuted).Padding(12, 8, 4, 8)
						if g == "Lewat waktu" {
							head.TextColor(t.Danger)
						}
						last = g
					}
				}
				a.reminderRow(c.Key(r.ID), r, now)
			}
		})
	})
	a.reminderBulkBar(c, items)
}

// reminderRow is one reminder: its circle (to pick it), title, when, and
// who else sees it.
func (a *App) reminderRow(c *ui.Context, r api.Reminder, now time.Time) {
	t := c.Theme()
	st := &a.reminders
	owned := a.reminderOwned(r)
	picked := st.picked[r.ID]
	row := ui.ButtonBase(c).Label("Reminder " + r.Title).Row().AlignItems(ui.Start).FillWidth().MinWidth(0).Gap(12).Padding(8).Radius(8)
	switch {
	case picked:
		row.Background(accentSoft(t))
	case st.selected == r.ID || row.Hovered():
		row.Background(t.SurfaceHover)
	}
	if st.selected == r.ID {
		row.Border(1, spaceMix(t.Accent, t.Border, .5))
	}
	row.FocusRing(row.FocusVisible()).Children(func() {
		if owned {
			ck := ui.ButtonBase(c.Key("ck")).Size(18, 18).Radius(9).Shrink(0).Center().Margin(1, 0, 0, 0)
			label := "Pilih"
			switch {
			case picked:
				label = "Batal pilih"
				ck.Background(t.Accent).Border(1.5, t.Accent)
			case r.Completed:
				ck.Background(t.Success).Border(1.5, t.Success)
			default:
				ck.Border(1.5, spaceMix(t.TextMuted, t.Background, .45))
				if ck.Hovered() {
					ck.BorderColor(t.Accent)
				}
			}
			ck.Label(label + " " + r.Title).FocusRing(ck.FocusVisible()).Children(func() {
				if picked || r.Completed {
					ui.Icon(c, spaceIcon("check")).Size(12, 12).TextColor(t.AccentText)
				}
			})
			if ck.Clicked() {
				if st.picked == nil {
					st.picked = map[id.ID]bool{}
				}
				if picked {
					delete(st.picked, r.ID)
				} else {
					st.picked[r.ID] = true
				}
			}
		} else {
			// A shared reminder is read-only: a dashed circle.
			ui.Box(c.Key("ck")).Size(18, 18).Radius(9).Shrink(0).Margin(1, 0, 0, 0).Border(1.5, t.Border).Tooltip("Hanya baca")
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			title := ui.Text(c, firstNonEmpty(r.Title, "Tanpa judul")).FontSize(13).FontWeight(500).MaxLines(1)
			if r.Completed {
				title.TextColor(faint(t)).Strikethrough()
			}
			late := reminderLate(r, now)
			when := reminderShortWhen(r) + " WIB"
			if late {
				when += " · lewat"
			}
			w := ui.Text(c, when).FontSize(12).TextColor(faint(t)).MaxLines(1)
			if late {
				w.TextColor(t.Danger)
			}
		})
		if !owned || r.Shared {
			tip := "Dibagikan"
			if !owned {
				tip = "Dibagikan kepadamu"
			}
			ui.Icon(c, spaceIcon("users")).Size(16, 16).TextColor(faint(t)).Shrink(0).Margin(2, 0, 0, 0).Tooltip(tip)
		}
	})
	if row.Clicked() && !st.busy {
		a.openReminder(r.ID)
	}
}

// reminderBulkBar is the owner's ask (canvas comment, 12 Nov): circles pick
// several reminders, then one button finishes them all, or opens them
// again on Selesai.
func (a *App) reminderBulkBar(c *ui.Context, items []api.Reminder) {
	t := c.Theme()
	st := &a.reminders
	var ids []id.ID
	pickable := 0
	for _, r := range items {
		if !a.reminderOwned(r) {
			continue
		}
		pickable++
		if st.picked[r.ID] {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 {
		return
	}
	done := st.tab != reminderTabDone
	ui.Row(c.Key("rm-bulk")).FillWidth().Label("Aksi untuk yang dipilih").Gap(8).AlignItems(ui.Center).Margin(0, 10, 10, 10).Padding(8, 8, 8, 12).Radius(10).Background(accentSoft(t)).Border(1, spaceMix(t.Accent, t.Border, .5)).Children(func() {
		ui.Text(c, fmt.Sprint(len(ids))).Font(FontMono).FontSize(12).FontWeight(600)
		muted(c, "dipilih").FontSize(12)
		if len(ids) < pickable && accentLink(c, "+Pilih semua").Label("Pilih semua").Clicked() {
			for _, r := range items {
				if a.reminderOwned(r) {
					st.picked[r.ID] = true
				}
			}
		}
		ui.Box(c).Grow(1)
		if onbLink(c, "Batal pilih semua", "Batal").Clicked() {
			st.picked = nil
		}
		label := "Tandai selesai"
		if !done {
			label = "Buka lagi"
		}
		label = fmt.Sprintf("%s (%d)", label, len(ids))
		if primarySpaceButton(c, label, "check").Height(28).MinHeight(28).Padding(0, 10).Disabled(a.reminderCreateBlocked()).Clicked() {
			a.finishReminders(ids, done)
		}
	})
	if c.Shortcut(0, ui.KeyEscape) {
		st.picked = nil
	}
}

// finishReminders finishes (or reopens) the reminders picked, says so,
// and offers Urungkan.
func (a *App) finishReminders(ids []id.ID, done bool) {
	a.setRemindersDone(ids, done, func() {
		a.reminders.picked = nil
		what := "ditandai selesai"
		if !done {
			what = "dibuka lagi"
		}
		title := fmt.Sprintf("%d reminder %s", len(ids), what)
		if len(ids) == 1 {
			if r := a.reminderByID(ids[0]); r != nil {
				title = "“" + r.Title + "” " + map[bool]string{true: "selesai", false: "dibuka lagi"}[done]
			}
		}
		a.toast(title, func() { a.setRemindersDone(ids, !done, nil) })
	})
}

// reminderPanel is the chosen reminder as a card to read, and how Ace
// will show it.
func (a *App) reminderPanel(c *ui.Context, phone bool) {
	t := c.Theme()
	st := &a.reminders
	r := a.reminderByID(st.selected)
	ui.Scroll(c.Key("rm-panel-scroll")).Grow(1).MinHeight(0).FillWidth().Label("Detail reminder").Children(func() {
		ui.Column(c).FillWidth().MaxWidth(780).MinWidth(0).Padding(phoneSize(phone, 16, 28), phoneSize(phone, 16, 36)).Gap(20).Children(func() {
			if phone && spaceButton(c, "Kembali ke daftar", "left", false).Clicked() {
				a.closeReminder()
			}
			if r == nil {
				ui.Column(c.Key("rm-pick")).FillWidth().Padding(80, 0).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, spaceIcon("bell")).Size(22, 22).TextColor(t.TextMuted)
					ui.Text(c, "Pilih reminder").FontSize(15).FontWeight(600)
					muted(c, "atau buat yang baru untuk mulai.").FontSize(13)
					if primarySpaceButton(c, "Reminder baru", "plus").Disabled(a.reminderCreateBlocked() || !st.prefsLoaded).Clicked() {
						a.beginReminder()
					}
				})
				return
			}
			a.reminderCard(c, *r)
			if st.historyOpen && a.reminderOwned(*r) {
				a.reminderHistory(c, *r)
			}
			if !r.Completed && a.reminderOwned(*r) {
				ui.Column(c.Key("ace-prev")).FillWidth().Gap(10).Children(func() {
					ui.Text(c, "Begini Ace akan mengingatkanmu").FontSize(12).TextColor(faint(t))
					at, timed, _ := reminderDue(*r)
					clock := "06.00"
					if timed {
						clock = at.In(wib()).Format("15.04")
					}
					acePill(c, "ace", r.Title, "Pengingat pribadi · "+clock)
				})
			}
		})
	})
}

// reminderCard reads the reminder: title, when (with how soon), its note,
// who sees it, and the owner's Tandai selesai, Ubah and ⋯.
func (a *App) reminderCard(c *ui.Context, r api.Reminder) {
	t := c.Theme()
	st := &a.reminders
	owned := a.reminderOwned(r)
	now := c.Now()
	ui.Column(c.Key("rm-card")).FillWidth().MinWidth(0).Gap(14).Padding(22, 24).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
		title := ui.Text(c, r.Title).FontSize(22).FontWeight(600)
		if r.Completed {
			title.TextColor(faint(t)).Strikethrough()
		}
		ui.Row(c.Key("rm-when")).FillWidth().Wrap().Gap(10).AlignItems(ui.End).Children(func() {
			at, timed, ok := reminderDue(r)
			when := "Tanpa waktu"
			if ok {
				when = reminderShortWhen(r)
				if !timed {
					when += " · 06.00"
				}
				when += " WIB"
			}
			ui.Text(c, when).Font(FontMono).FontSize(15).FontWeight(500)
			soon := ""
			switch {
			case r.Completed:
				soon = "selesai"
			case ok && at.Before(now):
				soon = "sudah lewat"
			case ok:
				soon = untukCountdown(at.Sub(now))
			}
			ui.Text(c, soon).Font(FontMono).FontSize(12).TextColor(t.Accent)
		})
		if r.Notes != "" {
			ui.Text(c, r.Notes).FontSize(13).TextColor(t.TextMuted)
		}
		ui.Row(c.Key("rm-meta")).Gap(6).AlignItems(ui.Center).Children(func() {
			icon, text := "lock", "Hanya kamu"
			switch {
			case !owned:
				icon, text = "users", "Dibagikan kepadamu · hanya baca"
			case r.Shared && st.recipientsLoaded && len(st.recipients) > 0:
				names := make([]string, len(st.recipients))
				for i, s := range st.recipients {
					names[i] = s.DisplayName
				}
				icon, text = "users", "Dibagikan ke "+strings.Join(names, ", ")
			case r.Shared:
				icon, text = "users", "Berbagi aktif, belum ada penerima"
			}
			ui.Icon(c, spaceIcon(icon)).Size(14, 14).TextColor(t.TextMuted)
			muted(c, text).FontSize(12)
		})
		if st.detailsErr != "" {
			ui.Text(c, st.detailsErr).FontSize(12).TextColor(t.Danger)
		}
		if !owned {
			return
		}
		blocked := a.reminderWriteBlocked()
		ui.Row(c.Key("rm-acts")).FillWidth().Gap(8).AlignItems(ui.Center).Padding(14, 0, 0, 0).Margin(4, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
			label := "Tandai selesai"
			if r.Completed {
				label = "Buka lagi"
			}
			if primarySpaceButton(c, label, "check").Disabled(blocked).Clicked() {
				a.finishReminders([]id.ID{r.ID}, !r.Completed)
			}
			if spaceButton(c, "Ubah", "edit", false).Border(1, t.Border).Disabled(blocked).Clicked() {
				a.editReminder()
			}
			ui.Box(c).Grow(1)
			more := spaceIconButton(c.Key("rm-more"), "Lainnya", "more")
			if more.Clicked() {
				st.menuOpen = !st.menuOpen
				st.deleteConfirm = false
			}
			popover(c, more, &st.menuOpen, 240, func() {
				if st.deleteConfirm {
					ui.Text(c, "Hapus reminder ini?").FontSize(13).Bold()
					muted(c, "Penerima juga tidak lagi dapat membacanya.").FontSize(12)
					if spaceButton(c, "Hapus permanen", "close", false).FillWidth().TextColor(t.Danger).Disabled(blocked).Clicked() {
						st.menuOpen = false
						a.deleteReminder()
					}
					if spaceButton(c, "Batal hapus", "left", false).FillWidth().Clicked() {
						st.deleteConfirm = false
					}
					return
				}
				if spaceButton(c, "Riwayat reminder", "clock", false).FillWidth().Clicked() {
					st.menuOpen, st.historyOpen = false, !st.historyOpen
				}
				if spaceButton(c, "Hapus reminder", "close", false).FillWidth().TextColor(t.Danger).Disabled(blocked).Clicked() {
					st.deleteConfirm = true
				}
			})
		})
	})
}

func (a *App) reminderHistory(c *ui.Context, r api.Reminder) {
	st := &a.reminders
	ui.Column(c.Key("rm-hist")).FillWidth().Gap(8).Children(func() {
		ui.Text(c, "Riwayat").FontSize(12).FontWeight(600).TextColor(c.Theme().TextMuted)
		if st.detailsLoading {
			muted(c, "Memuat riwayat…").FontSize(12)
			return
		}
		if len(st.history) == 0 {
			muted(c, "Belum ada riwayat yang tersedia.").FontSize(12)
		}
		for i, h := range st.history {
			ui.Row(c.Key(i)).Gap(6).Children(func() {
				ui.Text(c, h.OccurredAt.In(wib()).Format("2 Jan 15.04")).Font(FontMono).FontSize(12).TextColor(c.Theme().TextMuted)
				muted(c, "· "+reminderAuditLabel(h.Action)).FontSize(12)
			})
		}
	})
}

// reminderQuick are the dialog's quick times from now: Nanti sore, Malam
// ini, Besok pagi, Senin pagi.
func reminderQuick(now time.Time) [][3]string {
	today := now.In(wib())
	day := func(d time.Time) string { return d.Format(time.DateOnly) }
	monday := today.AddDate(0, 0, (8-int(today.Weekday()))%7)
	if monday.YearDay() == today.YearDay() {
		monday = monday.AddDate(0, 0, 7)
	}
	return [][3]string{
		{day(today), "17:00", "Nanti sore"},
		{day(today), "20:00", "Malam ini"},
		{day(today.AddDate(0, 0, 1)), "06:00", "Besok pagi"},
		{day(monday), "08:00", "Senin pagi"},
	}
}

// reminderDialog is Form B: one sentence to fill in, quick times, a note,
// who sees it, and Ace's preview of it.
func (a *App) reminderDialog(c *ui.Context) {
	st := &a.reminders
	d := st.draft
	if !st.dialog || d == nil {
		return
	}
	t := c.Theme()
	w, h := c.Size()
	now := c.Now()
	open := true
	creating := d.reminderID == id.Nil
	placeholder := reminderPlaceholders[st.placeholder%len(reminderPlaceholders)]
	ui.DialogBase(c.Key("reminder-dialog"), &open, func(backdrop, panel ui.Element) {
		dialogMotion(backdrop, panel)
		backdrop.Background(ui.RGBA(10, 9, 7, .62)).Justify(ui.Start).Padding(min(110, max(16, h/8)), 0, 16, 0)
		panel.Width(min(float32(620), w-32)).MaxHeight(h-60).Column().Radius(14).Background(t.Surface).Border(1, spaceMix(t.Accent, t.Border, .3)).Children(func() {
			ui.Row(c).FillWidth().Padding(14, 18, 0, 22).AlignItems(ui.Center).Children(func() {
				head := "Ubah reminder"
				if creating {
					head = "Reminder baru"
				}
				ui.Text(c, head).FontSize(12).TextColor(t.TextMuted).Grow(1)
				if spaceIconButton(c, "Tutup reminder", "close").Clicked() {
					open = false
				}
			})
			ui.Scroll(c.Key("rm-dlg-scroll")).Grow(1).Shrink(1).MinHeight(0).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Padding(10, 22, 18, 22).Gap(16).Disabled(a.reminderWriteBlocked() && !creating || a.reminderCreateBlocked()).Children(func() {
					a.reminderSentence(c, d, placeholder)
					a.reminderQuickRow(c, d, now)
					if due, ok := reminderDraftDue(d); ok && due.Before(now) {
						ui.Text(c, "Waktunya sudah lewat, jadi Ace tidak akan mengirimnya. Pilih waktu nanti.").FontSize(12).TextColor(t.Warning)
					}
					if st.noteOpen || d.notes != "" {
						ui.TextArea(c.Key("reminder-notes"), &d.notes).Label("Catatan reminder").Placeholder("Catatan, misalnya nomor kursi atau tautan").FillWidth().Height(72)
					} else if onbLink(c, "Tambah catatan", "+ Tambah catatan").AlignSelf(ui.Start).Clicked() {
						st.noteOpen = true
					}
					a.reminderShareRow(c, d, creating)
					ui.Column(c.Key("dlg-ace")).FillWidth().Gap(8).Children(func() {
						ui.Text(c, "Pratinjau di Ace").FontSize(12).TextColor(faint(t))
						title := strings.TrimSpace(d.title)
						if title == "" {
							title = strings.ToUpper(placeholder[:1]) + placeholder[1:]
						}
						acePill(c, "ace", title, reminderDraftWhen(d)+" WIB")
					})
				})
			})
			ui.Row(c).FillWidth().Padding(14, 18).Gap(8).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
				ui.Text(c, "Esc untuk batal").FontSize(12).TextColor(faint(t)).Grow(1)
				if spaceButton(c, "Batal", "", false).Border(1, t.Border).Clicked() {
					open = false
				}
				label := "Simpan"
				if creating {
					label = "Buat reminder"
				}
				if primarySpaceButton(c, label, "check").Disabled(a.reminderCreateBlocked() || !creating && a.reminderWriteBlocked()).Clicked() {
					if strings.TrimSpace(d.title) == "" {
						st.titleMissing = true
					} else {
						a.saveReminder()
					}
				}
			})
		})
	})
	if !open && !st.busy {
		a.cancelReminderDialog()
	}
	a.reminderWhenDialog(c, d)
}

// reminderSentence is "Ingatkan aku untuk [judul] pada [tanggal] pukul
// [jam] WIB".
func (a *App) reminderSentence(c *ui.Context, d *reminderDraft, placeholder string) {
	t := c.Theme()
	st := &a.reminders
	ui.Column(c.Key("rm-sentence")).FillWidth().Gap(8).Children(func() {
		ui.Text(c, "Ingatkan aku untuk").FontSize(20).TextColor(t.TextMuted)
		in := ui.TextInputBase(c.Key("reminder-title"), &d.title).Label("Judul reminder").Placeholder(placeholder).FillWidth().FontSize(20).FontWeight(500).Padding(2, 0).BorderWidth(0, 0, 1.5, 0).BorderColor(spaceMix(t.TextMuted, t.Background, .45))
		if in.Focused() {
			in.BorderColor(t.Accent)
		}
		if st.focusTitle {
			in.Focus()
			st.focusTitle = false
		}
		if in.Changed() {
			st.titleMissing = false
		}
		if st.titleMissing && strings.TrimSpace(d.title) == "" {
			ui.Text(c, "Isi dulu mau diingatkan soal apa.").FontSize(12).TextColor(t.Danger)
		}
		ui.Row(c.Key("rm-when-row")).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Padding(6, 0, 0, 0).Children(func() {
			ui.Text(c, "pada").FontSize(20).TextColor(t.TextMuted)
			ws := a.when(reminderWhenKey)
			date := "Pilih tanggal"
			if day, err := time.Parse(time.DateOnly, d.date); err == nil {
				date = fmt.Sprintf("%s, %d %s", shortDays[day.Weekday()], day.Day(), untukMonths[day.Month()-1])
			}
			if reminderPill(c, "rm-date", "Tanggal reminder", "calendar", date, "", ws.open && !ws.clockTab).Clicked() {
				a.openReminderWhen(d, false)
			}
			ui.Text(c, "pukul").FontSize(20).TextColor(t.TextMuted)
			clock, note := strings.Replace(d.clock, ":", ".", 1), ""
			if d.clock == "" {
				clock, note = "06.00", "(tanpa jam)"
			}
			if reminderPill(c, "rm-time", "Jam reminder", "clock", clock, note, ws.open && ws.clockTab).Clicked() {
				a.openReminderWhen(d, true)
			}
			ui.Text(c, "WIB").Font(FontMono).FontSize(14).FontWeight(500).TextColor(t.TextMuted)
		})
	})
}

// reminderPill is the sentence's date or time, a button that opens the
// picker.
func reminderPill(c *ui.Context, key, label, icon, text, note string, on bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(key)).Label(label+": "+text).Row().Gap(8).AlignItems(ui.Center).Height(34).Padding(0, 12, 0, 10).Radius(8).Background(t.Background).Border(1, t.Border)
	if on {
		b.BorderColor(t.Accent)
	} else if b.Hovered() {
		b.BorderColor(spaceMix(t.Accent, t.Border, .35))
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		ui.Icon(c, spaceIcon(icon)).Size(16, 16).TextColor(t.TextMuted)
		ui.Text(c, text).Font(FontMono).FontSize(14).FontWeight(500)
		if note != "" {
			ui.Text(c, note).FontSize(12).TextColor(t.TextMuted)
		}
	})
	return b
}

func (a *App) openReminderWhen(d *reminderDraft, clock bool) {
	ws := a.when(reminderWhenKey)
	ws.open, ws.clockTab, ws.minutes = true, clock, false
	if day, err := time.Parse(time.DateOnly, d.date); err == nil {
		ws.picker.month = day
	}
	if clock && d.clock == "" {
		d.clock = "06:00"
	}
}

// reminderWhenDialog is Space's date-and-time picker for the sentence:
// the calendar and the analog clock, Tanpa jam, Selesai. Always WIB.
func (a *App) reminderWhenDialog(c *ui.Context, d *reminderDraft) {
	ws := a.when(reminderWhenKey)
	a.whenDialog(c, reminderWhenKey, "Waktu reminder", ws, func(wide bool) {
		s := whenText(d.date, d.clock)
		if d.clock == "" {
			s += " · tanpa jam (dikirim 06.00)"
		}
		ui.Text(c, s+" · WIB").FontSize(13).TextColor(c.Theme().TextMuted)
		whenBody(c, ws, wide, func() {
			before := d.date
			calendarGrid(c, &ws.picker, &d.date)
			if d.date != before && !wide {
				ws.clockTab = true
			}
		}, func() { analogClock(c, reminderWhenKey, &d.clock, &ws.minutes) })
	}, func() {
		if spaceButton(c, "Tanpa jam", "close", false).Clicked() {
			d.clock = ""
		}
	})
}

func (a *App) reminderQuickRow(c *ui.Context, d *reminderDraft, now time.Time) {
	t := c.Theme()
	ui.Row(c.Key("rm-quick")).FillWidth().Wrap().Gap(6).Children(func() {
		for _, q := range reminderQuick(now) {
			on := d.date == q[0] && d.clock == q[1]
			b := ui.ButtonBase(c.Key(q[2])).Label(q[2]+" "+strings.Replace(q[1], ":", ".", 1)).Row().Gap(6).AlignItems(ui.Center).Height(28).Padding(0, 10).Radius(7).Border(1, t.Border).TextColor(t.TextMuted)
			switch {
			case on:
				b.Background(accentSoft(t)).BorderColor(spaceMix(t.Accent, t.Border, .5)).TextColor(t.Accent)
			case b.Hovered():
				b.Background(t.SurfaceHover).TextColor(t.Text)
			}
			b.FocusRing(b.FocusVisible()).Children(func() {
				ui.Text(c, q[2]).FontSize(12)
				ui.Text(c, strings.Replace(q[1], ":", ".", 1)).Font(FontMono).FontSize(11)
			})
			if b.Clicked() {
				d.date, d.clock = q[0], q[1]
			}
		}
	})
}

// reminderShareRow is who sees it: only the owner, or the people it is
// shared with, read-only. Recipients of a reminder already made are chosen
// here; a new one gets them once it exists.
func (a *App) reminderShareRow(c *ui.Context, d *reminderDraft, creating bool) {
	t := c.Theme()
	st := &a.reminders
	ui.Column(c.Key("rm-share")).FillWidth().Gap(8).Children(func() {
		b := ui.ButtonBase(c.Key("share-row")).Label("Siapa yang bisa melihat").Row().FillWidth().Gap(12).AlignItems(ui.Center).Padding(12, 14).Radius(10).Border(1, t.Border)
		if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
		b.FocusRing(b.FocusVisible()).Children(func() {
			icon, head, note := "lock", "Hanya kamu yang melihat", "Ketuk untuk membagikan ke pengurus lain."
			if d.share {
				icon, head, note = "users", "Dibagikan", "Mereka hanya bisa membaca. Ketuk untuk kembali privat."
				if !creating && st.recipientsLoaded && len(st.recipients) > 0 {
					names := make([]string, len(st.recipients))
					for i, s := range st.recipients {
						names[i] = s.DisplayName
					}
					head = "Dibagikan ke " + strings.Join(names, ", ")
				}
			}
			ui.Icon(c, spaceIcon(icon)).Size(18, 18).TextColor(t.TextMuted).Shrink(0)
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, head).FontSize(13).FontWeight(500).MaxLines(2)
				muted(c, note).FontSize(12)
			})
			track := ui.Box(c.Key("switch")).Size(34, 20).Radius(10).Shrink(0).Background(spaceMix(t.TextMuted, t.Background, .7))
			if d.share {
				track.Background(t.Accent)
			}
			track.Children(func() {
				knob := ui.Box(c).Absolute().Top(3).Size(14, 14).Radius(7).Background(t.Text)
				if d.share {
					knob.Left(17).Background(t.AccentText)
				} else {
					knob.Left(3)
				}
			})
		})
		if b.Clicked() {
			d.share = !d.share
		}
		if !d.share {
			return
		}
		if creating {
			muted(c, "Pilih penerimanya setelah reminder dibuat, lewat Ubah.").FontSize(12)
			return
		}
		a.reminderRecipientsView(c, a.reminderWriteBlocked())
	})
	if creating && d.share != st.prefs.DefaultShare && st.prefsLoaded {
		label := "Jadikan privat sebagai bawaan reminder baru"
		if d.share {
			label = "Jadikan berbagi sebagai bawaan reminder baru"
		}
		if onbLink(c, label, label).Clicked() {
			a.setReminderDefaultShare(d.share)
		}
	}
}

func (a *App) reminderRecipientsView(c *ui.Context, blocked bool) {
	t := c.Theme()
	st := &a.reminders
	ui.Row(c.Key("rm-recips")).FillWidth().Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
		if !st.recipientsLoaded {
			muted(c, "Daftar penerima belum dimuat.").FontSize(12)
			return
		}
		for _, s := range st.recipients {
			ui.Row(c.Key(s.RecipientID)).Height(28).Gap(6).AlignItems(ui.Center).Padding(0, 4).Radius(14).Border(1, t.Border).Children(func() {
				avatar(c, s.DisplayName, 20)
				ui.Text(c, s.DisplayName).FontSize(12)
				x := ui.ButtonBase(c.Key("x")).Label("Cabut akses "+s.DisplayName).Size(20, 20).Radius(10).Center().TextColor(faint(t)).Disabled(blocked)
				if x.Hovered() {
					x.Background(t.SurfaceHover).TextColor(t.Text)
				}
				x.Children(func() { ui.Icon(c, spaceIcon("close")).Size(12, 12) })
				if x.Clicked() {
					a.removeReminderRecipient(s.RecipientID)
				}
			})
		}
		add := onbSecondary(c, "Tambah penerima", "+ Penerima", true).Disabled(blocked)
		if add.Clicked() {
			st.recipientOpen = !st.recipientOpen
			if st.recipientOpen {
				a.loadReminderCandidates()
			}
		}
		popover(c, add, &st.recipientOpen, 300, func() {
			ui.Text(c, "Pilih akun penerima").FontSize(13).Bold()
			muted(c, "Hanya baca, terikat ke akun ini.").FontSize(12)
			if st.candidatesLoading {
				muted(c, "Memuat anggota…")
				return
			}
			if st.candidatesErr != "" {
				ui.Text(c, st.candidatesErr).FontSize(12).TextColor(t.Danger)
				if spaceButton(c, "Muat ulang calon penerima", "refresh", false).FillWidth().Clicked() {
					a.loadReminderCandidates()
				}
				return
			}
			ui.TextInput(c.Key("reminder-recipient-filter"), &st.recipientFilter).FillWidth().Label("Cari akun penerima").Placeholder("Cari nama…")
			q := strings.ToLower(strings.TrimSpace(st.recipientFilter))
			count := 0
			for _, m := range st.candidates {
				if q != "" && !strings.Contains(strings.ToLower(m.DisplayName), q) {
					continue
				}
				already := false
				for _, existing := range st.recipients {
					already = already || existing.RecipientID == m.UserID
				}
				if already {
					continue
				}
				count++
				choice := ui.ButtonBase(c.Key(m.UserID)).Label("Pilih penerima "+m.DisplayName).Row().Gap(8).AlignItems(ui.Center).FillWidth().Padding(6, 8).Radius(7).Disabled(blocked)
				if choice.Hovered() {
					choice.Background(t.SurfaceHover)
				}
				choice.FocusRing(choice.FocusVisible()).Children(func() {
					avatar(c, m.DisplayName, 22)
					ui.Text(c, m.DisplayName).FontSize(13).MaxLines(1)
				})
				if choice.Clicked() {
					a.addReminderRecipient(m.UserID)
					st.recipientOpen = false
				}
			}
			if count == 0 && st.candidatesLoaded {
				muted(c, "Tidak ada akun lain yang cocok.").FontSize(12)
			}
		})
	})
}

func reminderAuditLabel(action string) string {
	labels := map[string]string{
		"reminder.create":       "Reminder dibuat",
		"reminder.update":       "Reminder diperbarui",
		"reminder.complete":     "Reminder diselesaikan",
		"reminder.reopen":       "Reminder dibuka kembali",
		"reminder.sharing":      "Pengaturan berbagi diubah",
		"reminder.share.add":    "Penerima ditambahkan",
		"reminder.share.remove": "Akses penerima dicabut",
	}
	if label, ok := labels[action]; ok {
		return label
	}
	return "Perubahan: " + action
}

// firstReminder is the first reminder of the tab shown, else of any tab.
func (a *App) firstReminder() id.ID {
	for _, tab := range []int{a.reminders.tab, reminderTabActive, reminderTabDone, reminderTabShared} {
		if items := a.reminderTabItems(tab, ""); len(items) > 0 {
			return items[0].ID
		}
	}
	return id.Nil
}
