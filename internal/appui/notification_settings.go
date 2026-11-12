package appui

import (
	"context"
	"fmt"
	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"slices"
	"strconv"
	"strings"
	"time"
)

type notificationSettingsState struct {
	period, space                    id.ID
	division, task                   id.ID
	loaded, busy                     bool
	days, hour, minute, overdue, err string
	pingMessage, pingKind            string
	pingTarget                       id.ID
	recipients                       []id.ID
	device, whatsapp                 bool
	// customDay is the H-N typed beside the day chips.
	customDay string
}

// notificationSettings is Pengingat dan ping, option A: the schedule as
// rows (day chips with a custom H-N, the time, the overdue repeat, an
// example sentence), and ping as a row that opens the ping dialog.
func (a *App) notificationSettings(c *ui.Context) {
	a.ensureMeetings()
	sp := a.selectedSpace()
	desc := "Jadwal pengingat tugas divisi dan ping ke anggota."
	settingsPageView(c, "Pengingat dan ping", desc, false, nil, func() {
		if sp == nil {
			pageNotice(c, "grid", "Pilih Space", "Pengingat dan ping diatur per Space.")
			ui.Row(c.Key("spaces")).FillWidth().Gap(8).Wrap().Children(func() {
				for _, s := range a.access.spaces {
					s := s
					if spaceButton(c.Key(s.ID), s.Name, "grid", false).Clicked() {
						a.selectSpace(s.ID)
					}
				}
			})
			return
		}
		st := &a.notificationSettingsState
		var did id.ID
		for _, role := range sp.Roles {
			if slices.Contains(role.Capabilities, api.CapTasksManage) {
				did = role.DivisionID
				break
			}
		}
		if st.division != did || st.period != sp.PeriodID || st.space != sp.ID {
			a.resetPingState(sp, did)
		}
		if st.err != "" {
			color := c.Theme().Danger
			if st.err == scheduleSaved {
				color = c.Theme().Success
			}
			ui.Text(c, st.err).FontSize(13).TextColor(color)
		}
		if did != id.Nil && !sp.ReadOnly {
			if !st.loaded && !st.busy {
				a.loadNotificationPolicy()
			}
			a.scheduleSection(c)
		}
		if canPing(sp) {
			SettingSection(c, "Ping", "Colek anggota Space ini, di aplikasi dan bila mau juga di WhatsApp.", func() {
				SettingRow(c, "Kirim ping", "Pilih orang atau kelompok: PJ tugas aktif, atau yang belum membaca ping terakhir.", func() {
					if primarySpaceButton(c, "Kirim ping…", "send").Clicked() {
						a.chrome.pingOpen = true
					}
				})
				SettingRow(c, "Riwayat ping", "Siapa yang sudah membaca ada di tab Ping Space.", func() {
					if SettingButton(c, "Buka tab Ping", "").Clicked() {
						a.chrome.tab = "Ping"
						a.navigate(PathSpaces)
					}
				})
			})
		}
		SettingsNote(c, "Slot yang terlewat saat offline tidak dibunyikan seluruhnya saat kembali online.")
	})
}

// scheduleSaved is the line after the schedule is saved.
const scheduleSaved = "Jadwal tersimpan di server."

// scheduleDays reads the days field ("2,1,0") as numbers, largest first.
func scheduleDays(days string) []int {
	var out []int
	for _, part := range strings.Split(days, ",") {
		if v, err := strconv.Atoi(strings.TrimSpace(part)); err == nil && v >= 0 && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	slices.SortFunc(out, func(a, b int) int { return b - a })
	return out
}

func joinDays(days []int) string {
	parts := make([]string, len(days))
	for i, d := range days {
		parts[i] = strconv.Itoa(d)
	}
	return strings.Join(parts, ",")
}

func dayChipLabel(d int) string {
	if d == 0 {
		return "Hari H"
	}
	return "H-" + strconv.Itoa(d)
}

// scheduleExample is the schedule said once, for a task due next Friday.
func scheduleExample(now time.Time, days []int, hour, minute string) string {
	if len(days) == 0 {
		return "Tanpa hari pengingat, tugas hanya diingatkan setelah lewat tenggat."
	}
	h, errH := strconv.Atoi(hour)
	m, errM := strconv.Atoi(minute)
	if errH != nil || errM != nil {
		return "Isi jam dan menit untuk melihat contohnya."
	}
	// The first Friday whose earliest reminder is today or later, by date.
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	due := today
	for due.Weekday() != time.Friday || due.Before(today.AddDate(0, 0, days[0])) {
		due = due.AddDate(0, 0, 1)
	}
	names := make([]string, len(days))
	for i, d := range days {
		names[i] = untukWeekdays[due.AddDate(0, 0, -d).Weekday()]
	}
	list := names[0]
	if len(names) > 1 {
		list = strings.Join(names[:len(names)-1], ", ") + " dan " + names[len(names)-1]
	}
	return fmt.Sprintf("Tugas bertenggat %s %d %s diingatkan %s pukul %02d.%02d.", untukWeekdays[due.Weekday()], due.Day(), monthName(due.Month())[:3], list, h, m)
}

// scheduleSection is the division's reminder schedule as rows.
func (a *App) scheduleSection(c *ui.Context) {
	st := &a.notificationSettingsState
	t := c.Theme()
	busy := st.busy || a.conn != connOnline
	SettingSection(c, "Jadwal pengingat tugas", "Untuk semua tugas divisi, pada zona periode.", func() {
		days := scheduleDays(st.days)
		SettingRow(c, "Hari pengingat", "Sebelum dan pada hari tenggat. Klik untuk memilih, atau tambahkan H- sendiri.", func() {
			chips := []int{3, 2, 1, 0}
			for _, d := range days {
				if !slices.Contains(chips, d) {
					chips = append(chips, d)
				}
			}
			slices.SortFunc(chips, func(a, b int) int { return b - a })
			for _, d := range chips {
				on := slices.Contains(days, d)
				if spaceButton(c.Key(d), dayChipLabel(d), "", on).Label("Ingatkan "+dayChipLabel(d)).Padding(4, 10).MinHeight(30).Disabled(busy).Clicked() {
					if on {
						days = slices.DeleteFunc(slices.Clone(days), func(x int) bool { return x == d })
					} else {
						days = append(slices.Clone(days), d)
					}
					st.days = joinDays(scheduleDays(joinDays(days)))
				}
			}
			ui.Row(c.Key("custom-day")).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "H-").FontSize(13).TextColor(t.TextMuted)
				ui.TextInput(c.Key("custom"), &st.customDay).Label("Hari pengingat sendiri").Placeholder("7").Width(56).Disabled(busy)
				if SettingButton(c, "Tambah", "plus").Disabled(busy || strings.TrimSpace(st.customDay) == "").Clicked() {
					v, err := strconv.Atoi(strings.TrimSpace(st.customDay))
					if err != nil || v < 0 || v > 60 {
						st.err = "Hari pengingat sendiri: isi angka 0–60."
						return
					}
					st.days, st.customDay, st.err = joinDays(scheduleDays(st.days+","+strconv.Itoa(v))), "", ""
				}
			})
		})
		SettingRow(c, "Jam kirim", "Jam lokal periode. Tenggat tetap terpisah dari jam pengiriman.", func() {
			ui.TextInput(c.Key("hour"), &st.hour).Label("Jam kirim, jam 0–23").Width(56).Disabled(busy)
			ui.Text(c, ".").FontSize(14)
			ui.TextInput(c.Key("minute"), &st.minute).Label("Jam kirim, menit 0–59").Width(56).Disabled(busy)
		})
		SettingRow(c, "Lewat tenggat", "Ingatkan lagi selama tugas belum selesai.", func() {
			presets := []string{"1", "2", "3", "7"}
			sel := slices.Index(presets, strings.TrimSpace(st.overdue))
			if i := segmented(c, "Ulangi lewat tenggat", []string{"Tiap hari", "2 hari", "3 hari", "7 hari"}, sel); i >= 0 && !busy {
				st.overdue = presets[i]
			}
			if sel < 0 && strings.TrimSpace(st.overdue) != "" {
				muted(c, "Sekarang tiap "+strings.TrimSpace(st.overdue)+" hari").FontSize(12)
			}
		})
		SettingRow(c, "Contoh", scheduleExample(c.Now().In(a.homeZone()), days, st.hour, st.minute), func() {
			if primarySpaceButton(c, "Simpan jadwal divisi", "check").Disabled(busy).Clicked() {
				a.saveNotificationPolicy()
			}
		})
	})
}
func (a *App) loadNotificationPolicy() {
	c := a.notificationClient()
	if c == nil {
		return
	}
	st := &a.notificationSettingsState
	sp := a.selectedSpace()
	if sp == nil {
		return
	}
	gen, did, pid := a.gen, st.division, sp.PeriodID
	st.busy = true
	a.async(func() func() {
		p, err := c.DivisionNotificationPolicy(context.Background(), pid, did)
		return func() {
			if gen != a.gen || did != a.notificationSettingsState.division || a.periodID != pid {
				return
			}
			st := &a.notificationSettingsState
			st.busy = false
			st.loaded = true
			if err != nil {
				st.err = a.failed(err)
				return
			}
			parts := []string{}
			for _, v := range p.TaskDays {
				parts = append(parts, strconv.Itoa(v))
			}
			st.days = strings.Join(parts, ",")
			st.hour = strconv.Itoa(p.LocalHour)
			st.minute = strconv.Itoa(p.LocalMinute)
			st.overdue = strconv.Itoa(p.OverdueDays)
		}
	})
}
func (a *App) saveNotificationPolicy() {
	c := a.notificationClient()
	st := &a.notificationSettingsState
	sp := a.selectedSpace()
	if c == nil || sp == nil {
		return
	}
	p := api.NotificationPolicy{TaskDays: []int{}}
	for _, part := range strings.Split(st.days, ",") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		v, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			st.err = "Hari pengingat harus berupa angka."
			return
		}
		p.TaskDays = append(p.TaskDays, v)
	}
	var err error
	p.LocalHour, err = strconv.Atoi(st.hour)
	if err != nil {
		st.err = "Jam tidak valid."
		return
	}
	p.LocalMinute, err = strconv.Atoi(st.minute)
	if err != nil {
		st.err = "Menit tidak valid."
		return
	}
	p.OverdueDays, err = strconv.Atoi(st.overdue)
	if err != nil {
		st.err = "Frekuensi overdue tidak valid."
		return
	}
	gen, did, pid := a.gen, st.division, sp.PeriodID
	st.busy = true
	a.async(func() func() {
		_, err := c.SetDivisionNotificationPolicy(context.Background(), pid, did, p)
		return func() {
			if gen != a.gen || did != a.notificationSettingsState.division || a.periodID != pid {
				return
			}
			a.notificationSettingsState.busy = false
			if err != nil {
				a.notificationSettingsState.err = a.failed(err)
			} else {
				a.notificationSettingsState.err = scheduleSaved
			}
		}
	})
}

type taskNotificationState struct {
	task                             id.ID
	busy, loaded                     bool
	days, hour, minute, overdue, err string
}

func (a *App) taskNotificationControls(c *ui.Context, t api.Task) {
	if !t.Permissions.Manage {
		return
	}
	st := &a.taskNotification
	if st.task != t.ID {
		*st = taskNotificationState{task: t.ID}
	}
	cl := a.notificationClient()
	if cl == nil {
		return
	}
	ui.Text(c, "Pengingat tugas ini").Bold()
	if !st.loaded && !st.busy {
		st.busy = true
		gen, tid := a.gen, t.ID
		a.async(func() func() {
			p, err := cl.TaskNotificationPolicy(context.Background(), tid)
			return func() {
				if gen != a.gen || a.taskNotification.task != tid {
					return
				}
				st := &a.taskNotification
				st.loaded = true
				st.busy = false
				if err != nil {
					st.err = a.failed(err)
					return
				}
				parts := []string{}
				for _, d := range p.TaskDays {
					parts = append(parts, strconv.Itoa(d))
				}
				st.days = strings.Join(parts, ",")
				st.hour = strconv.Itoa(p.LocalHour)
				st.minute = strconv.Itoa(p.LocalMinute)
				st.overdue = strconv.Itoa(p.OverdueDays)
			}
		})
	}
	for _, f := range []struct {
		label string
		v     *string
	}{{"Hari sebelum deadline", &st.days}, {"Jam lokal", &st.hour}, {"Menit", &st.minute}, {"Overdue setiap N hari", &st.overdue}} {
		ui.Field(c.Key(f.label), f.label, func() {
			ui.TextInput(c, f.v).Label(f.label).Placeholder(f.label).FillWidth().Disabled(st.busy || a.conn != connOnline)
		})
	}
	if st.err != "" {
		muted(c, st.err)
	}
	if spaceButton(c, "Simpan jadwal khusus tugas", "bell", false).Disabled(st.busy || a.conn != connOnline).Clicked() {
		a.saveTaskNotification(t.ID, false)
	}
	if spaceButton(c, "Ikuti jadwal divisi", "refresh", false).Disabled(st.busy || a.conn != connOnline).Clicked() {
		a.saveTaskNotification(t.ID, true)
	}
}
func (a *App) saveTaskNotification(tid id.ID, reset bool) {
	cl := a.notificationClient()
	if cl == nil {
		return
	}
	st := &a.taskNotification
	p := api.NotificationPolicy{TaskDays: []int{}}
	if !reset {
		for _, part := range strings.Split(st.days, ",") {
			if strings.TrimSpace(part) == "" {
				continue
			}
			v, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil {
				st.err = "Isi hari dengan angka, dipisahkan koma."
				return
			}
			p.TaskDays = append(p.TaskDays, v)
		}
		var err error
		p.LocalHour, err = strconv.Atoi(st.hour)
		if err != nil {
			st.err = "Jam tidak valid."
			return
		}
		p.LocalMinute, err = strconv.Atoi(st.minute)
		if err != nil {
			st.err = "Menit tidak valid."
			return
		}
		p.OverdueDays, err = strconv.Atoi(st.overdue)
		if err != nil {
			st.err = "Frekuensi overdue tidak valid."
			return
		}
	}
	gen := a.gen
	st.busy = true
	a.async(func() func() {
		var err error
		if reset {
			err = cl.ResetTaskNotificationPolicy(context.Background(), tid)
		} else {
			_, err = cl.SetTaskNotificationPolicy(context.Background(), tid, p)
		}
		return func() {
			if gen != a.gen || a.taskNotification.task != tid {
				return
			}
			a.taskNotification.busy = false
			if err != nil {
				a.taskNotification.err = a.failed(err)
			} else {
				a.taskNotification.loaded = false
				a.taskNotification.err = "Konfigurasi tersimpan."
			}
		}
	})
}

func (a *App) notificationClient() *client.Client {
	c, ok := a.api.(*client.Client)
	if !ok {
		return nil
	}
	return c.AccountSession()
}

func canPing(sp *api.Space) bool {
	return sp != nil && !sp.ReadOnly && slices.Contains(sp.Capabilities, api.CapPingSend)
}
