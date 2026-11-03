package appui

import (
	"context"
	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
	"slices"
	"strconv"
	"strings"
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
}

func (a *App) notificationSettings(c *ui.Context) {
	a.ensureMeetings()
	sp := a.selectedSpace()
	settingsBody(c, "Pengingat dan ping", "Atur jadwal pengingat tugas dan kirim ping sesuai izin di Space.", func() {
		if sp == nil {
			muted(c, "Pilih Space untuk mengatur pengingat divisi atau mengirim ping.")
			for _, s := range a.access.spaces {
				s := s
				if spaceButton(c, s.Name, "grid", false).Clicked() {
					a.selectSpace(s.ID)
				}
			}
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
			*st = notificationSettingsState{period: sp.PeriodID, space: sp.ID, division: did, device: true, pingKind: "general"}
		}
		if st.err != "" {
			ui.Text(c, st.err).TextColor(c.Theme().Danger)
		}
		if did != id.Nil && !sp.ReadOnly {
			if !st.loaded && !st.busy {
				a.loadNotificationPolicy()
			}
			pageSection(c, "Jadwal tugas", "Tentukan hari dan jam pengingat untuk tugas divisi.", func() {
				for _, f := range []struct {
					label string
					value *string
				}{{"Hari sebelum tenggat · pisahkan koma", &st.days}, {"Jam lokal · 0–23", &st.hour}, {"Menit · 0–59", &st.minute}, {"Overdue setiap berapa hari · 1–30", &st.overdue}} {
					ui.Field(c.Key(f.label), f.label, func() { ui.TextInput(c, f.value).Label(f.label).FillWidth() })
				}
				if spaceButton(c, "Simpan jadwal divisi", "check", true).Disabled(st.busy || a.conn != connOnline).Clicked() {
					a.saveNotificationPolicy()
				}
				muted(c, "Bawaan H-2/H-1/H-0 pukul 06.00 pada zona periode. Deadline tetap terpisah dari jam pengiriman. Slot historis tidak dibunyikan seluruhnya saat kembali online.")
			})
		}
		if !slices.Contains(sp.Capabilities, api.CapPingSend) || sp.ReadOnly {
			return
		}
		ui.Text(c, "Kirim ping").Bold()
		ui.Row(c).Wrap().Gap(6).Children(func() {
			for _, k := range []string{"general", "task", "meeting"} {
				k := k
				if spaceButton(c, map[string]string{"general": "Umum", "task": "Tugas", "meeting": "Rapat"}[k], "", st.pingKind == k).Clicked() {
					st.pingKind = k
					st.pingTarget = id.Nil
				}
			}
		})
		if st.pingKind == "task" {
			a.ensureTasks(false)
			for _, t := range a.tasks.items {
				t := t
				if t.Permissions.Manage && spaceButton(c, t.Title, "task", st.pingTarget == t.ID).Clicked() {
					st.pingTarget = t.ID
				}
			}
		}
		if st.pingKind == "meeting" {
			for _, m := range a.meetings.items {
				m := m
				if m.CanManage && spaceButton(c, m.Title, "calendar", st.pingTarget == m.ID).Clicked() {
					st.pingTarget = m.ID
				}
			}
		}
		ui.Field(c, "Pesan · kosong memakai pesan bawaan", func() { ui.TextInput(c, &st.pingMessage).Label("Pesan · kosong memakai pesan bawaan").FillWidth() })
		ui.Row(c).Wrap().Gap(8).Children(func() {
			if spaceButton(c, "Perangkat", "monitor", st.device).Clicked() {
				st.device = !st.device
			}
			if spaceButton(c, "WhatsApp", "whatsapp", st.whatsapp).Clicked() {
				st.whatsapp = !st.whatsapp
			}
		})
		muted(c, "Kanal WhatsApp mencatat kegagalan adapter sampai bridge tahap 4 terhubung; tidak mengklaim pesan terkirim.")
		for _, m := range a.meetings.members {
			m := m
			selected := slices.Contains(st.recipients, m.ID)
			if spaceButton(c, m.DisplayName, "user", selected).Clicked() {
				if selected {
					st.recipients = slices.DeleteFunc(st.recipients, func(uid id.ID) bool { return uid == m.ID })
				} else {
					st.recipients = append(st.recipients, m.ID)
				}
			}
		}
		if spaceButton(c, "Kirim ping sekarang", "bell", true).Disabled(st.busy || a.conn != connOnline).Clicked() {
			a.sendManualPing()
		}
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
				a.notificationSettingsState.err = "Jadwal tersimpan di server."
			}
		}
	})
}
func (a *App) sendManualPing() {
	svc, ok := a.deliveryService()
	sp := a.selectedSpace()
	if !ok || sp == nil {
		return
	}
	st := &a.notificationSettingsState
	req := api.PingRequest{Message: st.pingMessage, Recipients: slices.Clone(st.recipients)}
	if st.pingKind != "general" {
		req.SourceKind = st.pingKind
		target := st.pingTarget
		req.SourceID = &target
	}
	if st.device {
		req.Channels = append(req.Channels, "device")
	}
	if st.whatsapp {
		req.Channels = append(req.Channels, "whatsapp")
	}
	if len(req.Channels) == 0 {
		st.err = "Pilih setidaknya satu kanal."
		return
	}
	gen, sid := a.gen, sp.ID
	st.busy = true
	a.async(func() func() {
		err := svc.SendPing(context.Background(), sid, req)
		return func() {
			if gen != a.gen || sid != a.selectedSpaceID() {
				return
			}
			a.notificationSettingsState.busy = false
			if err != nil {
				a.notificationSettingsState.err = a.failed(err)
			} else {
				a.notificationSettingsState.err = "Ping dibuat. Status perangkat dan WhatsApp dicatat terpisah."
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
			ui.TextInput(c, f.v).Label(f.label).FillWidth().Disabled(st.busy || a.conn != connOnline)
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
