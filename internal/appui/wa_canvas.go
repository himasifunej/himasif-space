package appui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

// The Bot WhatsApp tab as canvas "Bot WhatsApp" option A (Tahap 7d): one
// status sentence, four setup steps, what the bot does and who linked their
// number; the details sit under "Pengaturan lanjutan" with Pratinjau on
// every text the bot sends.

// waTokens maps the server's placeholders to the words the page shows.
var waTokens = []struct{ token, word string }{{"{body}", "[Isi pesan]"}, {"{name}", "[Nama bot]"}, {"{greeting}", "[Sapaan]"}, {"{title}", "[Judul]"}}

// waFriendly shows a template with words instead of placeholders.
func waFriendly(s string) string {
	for _, t := range waTokens {
		s = strings.ReplaceAll(s, t.token, t.word)
	}
	return s
}

// waServerTemplate turns the words back into the server's placeholders.
func waServerTemplate(s string) string {
	for _, t := range waTokens {
		s = strings.ReplaceAll(s, t.word, t.token)
	}
	return s
}

// waTemplateLabels name each message as people know it.
var waTemplateLabels = map[string]string{
	"reminder": "Pengingat tenggat",
	"ping":     "Ping",
	"reply":    "Balasan perintah di grup",
	"weekly":   "Ringkasan mingguan SOP",
	"followup": "Koordinasi H-4 (Medhum)",
}

// waStatus is the page's state: off (not connected), partial (connected
// but a group or members are missing) or ok.
type waStatus struct {
	state                  string
	connected, group       bool
	linked, total          int
	unlinked               []string
	membersKnown, canWrite bool
}

func (a *App) waStatus() waStatus {
	st := &a.whatsapp
	out := waStatus{canWrite: a.waCanManage()}
	if st.connection != nil {
		status := st.session.Status
		if status == "" {
			status = st.connection.Status
		}
		out.connected = status == "connected"
		out.group = len(st.connection.Groups) > 0
	}
	m := &a.access.members
	if m.loaded && m.division == st.division {
		for _, member := range m.members {
			if member.WhatsAppLinked == nil {
				continue
			}
			out.membersKnown = true
			out.total++
			if *member.WhatsAppLinked {
				out.linked++
			} else {
				out.unlinked = append(out.unlinked, strings.Fields(strings.TrimPrefix(member.DisplayName, "[DUMMY] ") + " ")[0])
			}
		}
	}
	switch {
	case !out.connected:
		out.state = "off"
	case !out.group || out.linked < out.total:
		out.state = "partial"
	default:
		out.state = "ok"
	}
	return out
}

// waLinkMessage is what "Kirim cara menautkan" copies for the group.
const waLinkMessage = "Halo! Supaya pengingat dari Space sampai ke WhatsApp kamu: buka Space → tab Bot WhatsApp → Tautkan nomormu → Buat kode, lalu kirim kodenya di grup ini. Cukup sekali."

func (a *App) whatsappSettings(c *ui.Context) {
	phone := isMobile(c)
	ui.Scroll(c.Key("wa-page")).Fill().Children(func() {
		body := ui.Column(c).FillWidth().MinWidth(0).MaxWidth(1440).Padding(24).Gap(16)
		if phone {
			body.Padding(12, 16).Gap(12)
		}
		body.Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				title := ui.Text(c, "Bot WhatsApp").FontSize(20).Bold()
				if phone {
					title.FontSize(17)
				}
				muted(c, "Bot mengirim pengingat dan menjawab perintah di grup WhatsApp divisi.").FontSize(13)
			})
			if !a.waHeader(c) {
				return
			}
			st := &a.whatsapp
			if st.division != id.Nil && (!a.access.members.loaded && !a.access.members.loading || a.access.members.division != st.division) && a.spacePermissionsForDivision(st.division, api.CapMembersRead) {
				a.loadMembers(st.division)
			}
			s := a.waStatus()
			a.waHero(c, s, phone)
			cols := ui.Column(c.Key("wa-cols")).FillWidth().MinWidth(0).Gap(16)
			wide := !phone && cols.Bounds().W >= 960
			if wide {
				cols.Row().AlignItems(ui.Start)
			}
			cols.Children(func() {
				left := ui.Column(c).FillWidth().MinWidth(0).Gap(16)
				if wide {
					left.Grow(1).Basis(0)
				}
				left.Children(func() {
					if s.canWrite {
						a.waSteps(c, s, phone)
						a.waAdvanced(c, phone)
					} else {
						a.waOwnLinkPanel(c)
						pageNotice(c, "shield", "Pengaturan bot", "Bot diatur oleh kadiv atau pemegang izin WhatsApp divisi.")
					}
				})
				right := ui.Column(c).FillWidth().MinWidth(0).Gap(16)
				if wide {
					right.Width(380).Shrink(0)
				}
				right.Children(func() {
					a.waWhatPanel(c)
					a.waMembersPanel(c, s)
				})
			})
			pageNotice(c, "link", "Medhum memakai bot Humas", "Grup bersama Medhum diatur dari Space Humas.")
		})
	})
}

// waHero is the status sentence with the one next step.
func (a *App) waHero(c *ui.Context, s waStatus, phone bool) {
	t := c.Theme()
	title, text, tone := "", "", t.Success
	switch s.state {
	case "off":
		title, text, tone = "Bot belum terhubung", "Bot belum bisa mengirim apa pun. Mulai dari langkah 1; butuh sekitar 2 menit.", t.Danger
	case "partial":
		tone = t.Warning
		if !s.group {
			title, text = "Bot terhubung, grup belum dipilih", "Pilih grup divisi agar pengingat terkirim ke grup."
		} else {
			title = fmt.Sprintf("Bot aktif, %d anggota belum menautkan", s.total-s.linked)
			text = strings.Join(s.unlinked, " dan ") + " belum menerima pengingat pribadi."
		}
	default:
		title, text = "Bot berjalan normal", fmt.Sprintf("Terhubung ke grup. Semua %d anggota tertaut.", s.total)
		if !s.membersKnown {
			text = "Terhubung ke grup divisi."
		}
	}
	hero := ui.Row(c.Key("wa-hero")).FillWidth().MinWidth(0).Padding(16, 20).Gap(16).AlignItems(ui.Center).Radius(12).Background(t.Surface).Border(1, t.Border)
	if phone {
		hero.Column().AlignItems(ui.Stretch).Padding(16)
	}
	hero.Children(func() {
		ui.Box(c).Size(44, 44).Radius(12).Background(spaceMix(t.Surface, tone, .16)).Center().Shrink(0).Children(func() {
			icon := "whatsapp"
			if s.state == "ok" {
				icon = "check"
			}
			ui.Icon(c, spaceIcon(icon)).Size(22, 22).TextColor(tone)
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, title).FontSize(16).FontWeight(600)
			muted(c, text).FontSize(13)
		})
		if !s.canWrite {
			return
		}
		switch s.state {
		case "off":
			if primarySpaceButton(c.Key("hero-cta"), "Hubungkan WhatsApp", "whatsapp").Shrink(0).Clicked() {
				a.whatsapp.step = 1
			}
		case "partial":
			if !s.group {
				if primarySpaceButton(c.Key("hero-cta"), "Pilih grup", "users").Shrink(0).Clicked() {
					a.whatsapp.step = 2
				}
			} else if primarySpaceButton(c.Key("hero-cta"), "Kirim cara menautkan", "link").Shrink(0).Clicked() {
				c.WriteClipboard(waLinkMessage)
				c.AddToast(ui.Toast{Title: "Pesan disalin. Tempel di grup WhatsApp divisi.", Type: "success", Timeout: 4 * time.Second})
			}
		default:
			ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(4, 10).Radius(999).Background(t.SurfaceHover).Shrink(0).Children(func() {
				ui.Box(c).Size(8, 8).Radius(4).Background(t.Success)
				ui.Text(c, "Terhubung").FontSize(12).TextColor(t.Success)
			})
		}
	})
}

// waSteps is "Langkah penyiapan": four numbered steps, each done, next or
// waiting; the opened step shows its controls below it.
func (a *App) waSteps(c *ui.Context, s waStatus, phone bool) {
	t := c.Theme()
	st := &a.whatsapp
	steps := []struct {
		title, desc, cta string
		done             bool
	}{
		{"Hubungkan nomor WhatsApp divisi", "Pindai kode QR dari ponsel yang memegang nomor divisi.", "Hubungkan", s.connected},
		{"Pilih grup divisi", "Bot hanya mengirim ke grup yang kamu pilih.", "Pilih grup", s.group},
		{"Anggota menautkan nomornya", "Agar pengingat pribadi sampai ke WhatsApp masing-masing.", "Lihat anggota", s.membersKnown && s.linked == s.total},
		{"Atur jadwal pengingat", "Bawaan: H-2, H-1, hari H pukul 06.00.", "Ubah", st.connection != nil},
	}
	doneCount := 0
	for _, step := range steps {
		if step.done {
			doneCount++
		}
	}
	ui.Column(c.Key("wa-steps")).FillWidth().MinWidth(0).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Row(c).FillWidth().Padding(14, 20).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, "Langkah penyiapan").FontSize(15).Bold().Grow(1)
			muted(c, fmt.Sprintf("%d dari 4 selesai", doneCount)).FontSize(12)
		})
		ui.Column(c).FillWidth().Padding(4, 20).Children(func() {
			next := 0
			for i, step := range steps {
				if !step.done {
					next = i + 1
					break
				}
			}
			for i, step := range steps {
				n := i + 1
				locked := n > 1 && !s.connected
				row := ui.Row(c.Key(n)).FillWidth().MinWidth(0).Gap(16).Padding(14, 0).AlignItems(ui.Center)
				if i > 0 {
					row.BorderWidth(1, 0, 0, 0).BorderColor(t.Border)
				}
				row.Children(func() {
					mark := ui.Box(c).Size(28, 28).Radius(14).Center().Shrink(0)
					switch {
					case step.done:
						mark.Background(t.Success).Children(func() { ui.Icon(c, spaceIcon("check")).Size(14, 14).TextColor(t.Background) })
					case n == next:
						mark.Border(2, t.Accent).Children(func() { ui.Text(c, fmt.Sprint(n)).FontSize(12).FontWeight(600) })
					default:
						mark.Border(1, t.Border).Children(func() { ui.Text(c, fmt.Sprint(n)).FontSize(12).TextColor(faint(t)) })
					}
					ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
						ui.Text(c, step.title).FontSize(14).FontWeight(500)
						desc := step.desc
						if n == 3 && s.membersKnown && s.linked < s.total {
							desc = fmt.Sprintf("%d dari %d anggota sudah menautkan.", s.linked, s.total)
						}
						muted(c, desc).FontSize(12)
					})
					switch {
					case locked:
						muted(c, "Setelah langkah 1").FontSize(12).Shrink(0)
					case step.done && st.step != n:
						if spaceButton(c.Key("open"), "Selesai", "check", false).Label("Buka langkah "+fmt.Sprint(n)).Padding(4, 10).MinHeight(30).TextColor(t.Success).Shrink(0).Clicked() {
							st.step = n
						}
					case st.step == n:
						if spaceButton(c.Key("close"), "Tutup", "", false).Padding(4, 10).MinHeight(30).Shrink(0).Clicked() {
							st.step = 0
						}
					default:
						var cta ui.Element
						if n == next {
							cta = primarySpaceButton(c.Key("cta"), step.cta, "").Padding(4, 12).MinHeight(32).Shrink(0)
						} else {
							cta = spaceButton(c.Key("cta"), step.cta, "", false).Padding(4, 12).MinHeight(32).Shrink(0)
						}
						if cta.Clicked() {
							st.step = n
						}
					}
				})
				if st.step == n && !locked {
					ui.Column(c.Key(fmt.Sprint("detail", n))).FillWidth().MinWidth(0).Gap(12).Padding(0, 0, 16, 44).Children(func() {
						switch n {
						case 1:
							a.waSessionStep(c)
						case 2:
							a.waGroupStep(c)
						case 3:
							a.waMembersStep(c, s)
						case 4:
							a.waReminderStep(c)
						}
					})
				}
			}
		})
	})
}

func (a *App) waClient() (*client.Client, bool) {
	cl := a.notificationClient()
	st := &a.whatsapp
	return cl, !(st.busy || st.loading || a.offlineMode() || cl == nil)
}

// waSessionStep: create the bot when needed, then connect by QR.
func (a *App) waSessionStep(c *ui.Context) {
	st := &a.whatsapp
	cl, ok := a.waClient()
	if st.connection == nil {
		muted(c, "Bot belum dibuat untuk divisi ini. Buat dulu dengan nama bawaan; namanya bisa diubah di Pengaturan lanjutan.")
		if primarySpaceButton(c.Key("wa-create"), "Buat bot", "plus").Disabled(!ok).Clicked() {
			pid, did := st.period, st.division
			req := api.WAConnectionRequest{DisplayName: st.name, Language: st.language, Greeting: st.greeting, Templates: map[string]string{}}
			a.waRun(func() error {
				_, err := cl.CreateWAConnection(context.Background(), pid, did, req)
				return err
			})
		}
		return
	}
	conn := *st.connection
	a.waPollPairing(c)
	status := st.session.Status
	if status == "" {
		status = conn.Status
	}
	if label := map[string]string{"connected": "Terhubung", "connecting": "Menyambungkan", "pairing": "Menunggu pemindaian QR", "disconnected": "Terputus", "needs_pairing": "Perlu ditautkan ulang"}[status]; label != "" {
		status = label
	}
	ui.Text(c, "Status: "+status).FontSize(13).Bold()
	if st.session.LastError != "" {
		muted(c, st.session.LastError).FontSize(12)
	}
	ui.Column(c).Gap(6).Children(func() {
		for i, line := range []string{"Buka WhatsApp di ponsel divisi.", "Ketuk Perangkat tertaut lalu Tautkan perangkat.", "Arahkan kamera ke kode yang muncul di sini. Halaman berubah sendiri setelah tersambung."} {
			muted(c, fmt.Sprintf("%d. %s", i+1, line)).FontSize(13)
		}
	})
	ui.Row(c).Wrap().Gap(8).Children(func() {
		for _, action := range []struct{ key, label string }{{"connect", "Tampilkan kode QR"}, {"disconnect", "Putuskan"}} {
			action := action
			if spaceButton(c.Key(action.key), action.label, "whatsapp", action.key == "connect").Disabled(!ok).Clicked() {
				if action.key == "connect" {
					st.pollUntil = c.Now().Add(2 * time.Minute)
					st.pollNext = time.Time{}
				} else {
					st.pollUntil = time.Time{}
				}
				a.waRun(func() error {
					return cl.WAAction(context.Background(), conn.ID, api.WAActionRequest{Action: action.key, ExpectedVersion: conn.Version})
				})
			}
		}
		if spaceButton(c, "Keluarkan akun bot", "logout", false).Disabled(!ok).Clicked() {
			st.confirmLogout = true
		}
	})
	if st.confirmLogout {
		pageNotice(c, "alert", "Keluarkan akun bot?", "Sambungan dihapus dan nomor divisi perlu dipindai ulang.")
		ui.Row(c).Gap(8).Children(func() {
			if spaceButton(c, "Ya, keluarkan", "logout", false).Disabled(!ok).Clicked() {
				a.waRun(func() error {
					return cl.WAAction(context.Background(), conn.ID, api.WAActionRequest{Action: "logout", ExpectedVersion: conn.Version, Confirmed: true})
				})
			}
			if spaceButton(c, "Batal", "close", false).Clicked() {
				st.confirmLogout = false
			}
		})
	}
	switch {
	case st.qr != nil && st.session.QRExpiresAt != nil && time.Now().Before(*st.session.QRExpiresAt):
		ui.Image(c, st.qr).Size(220, 220).Fit(ui.Contain)
		muted(c, "Kode berlaku hingga "+st.session.QRExpiresAt.Local().Format("15.04.05")+". Jangan bagikan kode ini.").FontSize(12)
		c.After(time.Second)
	case !st.pollUntil.IsZero() && c.Now().Before(st.pollUntil):
		muted(c, "Menunggu kode dari WhatsApp. Biarkan halaman ini terbuka.").FontSize(12)
	case st.session.Status == "pairing":
		muted(c, "Kode belum tersedia atau sudah kedaluwarsa. Pilih Tampilkan kode QR lagi.").FontSize(12)
	}
}

// waGroupStep: the group of the division's Space.
func (a *App) waGroupStep(c *ui.Context) {
	st := &a.whatsapp
	cl, ok := a.waClient()
	if st.connection == nil {
		return
	}
	conn := *st.connection
	for _, g := range conn.Groups {
		ui.Text(c, "Sekarang: "+g.Name+" → "+a.waSpaceName(g.SpaceID)).FontSize(13)
	}
	ui.Row(c).Wrap().Gap(8).Children(func() {
		for _, sp := range a.access.spaces {
			if sp.PeriodID != st.period || !slices.ContainsFunc(sp.Divisions, func(d api.Division) bool { return d.ID == st.division }) {
				continue
			}
			if spaceButton(c.Key(sp.ID.String()), sp.Name, "users", st.mapSpace == sp.ID).Disabled(!ok).Clicked() {
				st.mapSpace = sp.ID
			}
		}
	})
	if len(st.groups) == 0 {
		muted(c, "Grup baru terlihat setelah nomor divisi terhubung. Pastikan nomor divisi sudah masuk grupnya.").FontSize(12)
	}
	for _, g := range st.groups {
		if spaceButton(c.Key(g.JID), g.Name, "whatsapp", st.mapJID == g.JID).Disabled(!ok).Clicked() {
			st.mapJID = g.JID
		}
	}
	if spaceButton(c, "Jawab perintah di grup: "+waYesNo(st.commandsEnabled), "terminal", st.commandsEnabled).Disabled(!ok).Clicked() {
		st.commandsEnabled = !st.commandsEnabled
	}
	if primarySpaceButton(c.Key("save-group"), "Simpan grup", "check").Disabled(!ok || st.mapJID == "" || st.mapSpace == id.Nil).Clicked() {
		req := api.WAGroupRequest{SpaceID: st.mapSpace, JID: st.mapJID, CommandsEnabled: st.commandsEnabled, ExpectedVersion: conn.Version}
		a.waRun(func() error { return cl.SaveWAGroup(context.Background(), conn.ID, req) })
	}
}

// waMembersStep: who is missing and the message to send to the group.
func (a *App) waMembersStep(c *ui.Context, s waStatus) {
	if !s.membersKnown {
		muted(c, "Daftar anggota sedang dimuat.").FontSize(12)
	} else if len(s.unlinked) > 0 {
		muted(c, "Belum menautkan: "+strings.Join(s.unlinked, ", ")+".").FontSize(13)
	}
	ui.Text(c, waLinkMessage).FontSize(13).Selectable().Padding(10, 12).Radius(8).Background(c.Theme().Background).Border(1, c.Theme().Border)
	if primarySpaceButton(c.Key("copy-link"), "Salin pesan untuk grup", "link").Clicked() {
		c.WriteClipboard(waLinkMessage)
		c.AddToast(ui.Toast{Title: "Pesan disalin. Tempel di grup WhatsApp divisi.", Type: "success", Timeout: 4 * time.Second})
	}
	a.waOwnLink(c)
}

// waReminderStep: the division's reminder schedule, with its preview.
func (a *App) waReminderStep(c *ui.Context) {
	sp := a.selectedSpace()
	if sp == nil {
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
	if did == id.Nil || sp.ReadOnly {
		muted(c, "Jadwal diatur oleh pengelola tugas divisi.").FontSize(12)
		return
	}
	if !st.loaded && !st.busy {
		a.loadNotificationPolicy()
	}
	for _, f := range []struct {
		label string
		value *string
	}{{"Ingatkan berapa hari sebelum tenggat · contoh 2,1,0", &st.days}, {"Jam kirim · 0–23", &st.hour}, {"Menit · 0–59", &st.minute}, {"Ingatkan lagi bila lewat tenggat, tiap berapa hari", &st.overdue}} {
		ui.Field(c.Key(f.label), f.label, func() { ui.TextInput(c, f.value).Label(f.label).Placeholder(f.label).FillWidth() })
	}
	ui.Row(c).Gap(8).Children(func() {
		if primarySpaceButton(c.Key("save-policy"), "Simpan jadwal", "check").Disabled(st.busy || a.conn != connOnline).Clicked() {
			a.saveNotificationPolicy()
		}
		a.waPreviewButton(c, "reminder")
	})
	a.waPreviewBubble(c, "reminder")
	if st.err != "" {
		muted(c, st.err).FontSize(12)
	}
}

// waOwnLink is "WhatsApp-mu": the member's own link. With a connected bot it
// is two taps: make a one-time code, then open WhatsApp with the code typed
// in and send it. While the code is open the panel waits visibly (spinner,
// countdown) and checks the server every few seconds and on realtime; it
// says what the bot made of the code, such as a number that belongs to
// another account (answer PINDAH), and every failure has a working retry.
func (a *App) waOwnLink(c *ui.Context) {
	st := &a.whatsapp
	t := c.Theme()
	cl, ok := a.waClient()
	sp := a.selectedSpace()
	if st.linkToast != "" {
		c.AddToast(ui.Toast{Title: st.linkToast, Type: "success", Timeout: 4 * time.Second})
		st.linkToast = ""
	}
	if ok && st.division != id.Nil && (st.linkStatusFor != st.division || st.linkStatus == nil) && !st.linkStatusLoading && st.linkErr == "" {
		a.loadWALinkStatus()
	}
	ui.Text(c, "WhatsApp-mu").FontSize(13).Bold()
	status := st.linkStatus
	if st.linkErr != "" {
		ui.Text(c, st.linkErr).FontSize(12).TextColor(t.Danger)
		if spaceButton(c.Key("link-retry"), "Coba lagi", "refresh", false).Disabled(st.linkStatusLoading).Clicked() {
			st.linkErr = ""
			a.loadWALinkStatus()
		}
		return
	}
	if status == nil {
		ui.Spinner(c).Label("Memeriksa status WhatsApp…")
		return
	}
	if status.Linked {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
			ui.Icon(c, spaceIcon("check")).Size(16, 16).TextColor(t.Success).Shrink(0)
			number := status.Phone
			if number == "" {
				number = "nomor tersembunyi oleh WhatsApp"
			}
			ui.Text(c, "Tertaut · "+number).FontSize(13).FontWeight(500)
			if status.LinkedAt != nil {
				muted(c, "sejak "+status.LinkedAt.Local().Format("2 Jan 2006")).FontSize(12)
			}
		})
		if st.unlinkAsk {
			muted(c, "Lepas tautan? Pengingat dari Space tidak lagi sampai ke nomor ini sampai kamu menautkan lagi.").FontSize(12)
			ui.Row(c).Gap(8).Children(func() {
				if spaceButton(c.Key("unlink-yes"), "Lepas tautan", "unlink", false).Disabled(!ok).Clicked() {
					a.unlinkOwnWA(cl)
				}
				if spaceButton(c.Key("unlink-no"), "Batal", "close", false).Clicked() {
					st.unlinkAsk = false
				}
			})
		} else if spaceButton(c.Key("unlink"), "Bukan nomormu? Lepas tautan", "unlink", false).Disabled(!ok).Clicked() {
			st.unlinkAsk = true
		}
		return
	}
	now := c.Now()
	pending := status.Pending
	if pending != nil && !now.Before(pending.ExpiresAt) {
		pending = nil
	}
	known := st.link != nil && now.Before(st.link.ExpiresAt)
	muted(c, "Pengingat tugas dan rapat dari Space dikirim ke WhatsApp yang tertaut. Cukup sekali per periode.").FontSize(12)
	if !known && pending == nil {
		if st.link != nil {
			ui.Text(c, "Kode sudah kedaluwarsa. Buat kode baru.").FontSize(12).TextColor(t.Warning)
		}
		label := "Tautkan WhatsApp"
		if !status.BotReady {
			label = "Buat kode"
		}
		if primarySpaceButton(c.Key("make-link"), label, "whatsapp").Disabled(!ok || sp == nil || sp.ReadOnly).Clicked() {
			a.createOwnWALink(cl)
		}
		return
	}
	expires := st.link
	until := time.Time{}
	if known {
		until = expires.ExpiresAt
	} else {
		until = pending.ExpiresAt
	}
	if pending != nil && pending.ConflictAccount != "" {
		ui.Column(c).FillWidth().Gap(4).Padding(10, 12).Radius(8).Background(t.Background).Border(1, t.Warning).Children(func() {
			ui.Text(c, "Nomor itu sudah tertaut ke akun "+pending.ConflictAccount+".").FontSize(13).FontWeight(600)
			move := "sebelum habis waktunya"
			if pending.MoveUntil != nil {
				move = "sebelum " + pending.MoveUntil.Local().Format("15.04")
			}
			muted(c, "Balas PINDAH ke bot di WhatsApp "+move+" untuk memindahkannya ke akunmu, atau minta pemilik akun "+pending.ConflictAccount+" melepas tautan di Space lalu kirim kode baru.").FontSize(12)
		})
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Spinner(c)
		left := until.Sub(now).Round(time.Second)
		muted(c, fmt.Sprintf("Menunggu pesan dari WhatsApp… %d:%02d", int(left.Minutes()), int(left.Seconds())%60)).FontSize(12)
	})
	if known {
		if st.link.DeepLink != "" {
			if primarySpaceButton(c.Key("open-wa"), "Buka WhatsApp dan kirim", "send").Clicked() {
				c.OpenURL(st.link.DeepLink)
			}
			muted(c, "WhatsApp terbuka dengan kode sudah terketik; ketuk Kirim. Status di sini berubah sendiri.").FontSize(12)
		}
		command := "!link " + st.link.Code
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
			ui.Text(c, command).FontSize(12).Selectable().Padding(6, 10).Radius(7).Background(t.Background).Border(1, t.Border)
			if spaceButton(c.Key("copy-code"), "Salin", "link", false).Clicked() {
				c.WriteClipboard(command)
				c.AddToast(ui.Toast{Title: "Kode disalin.", Type: "success", Timeout: 4 * time.Second})
			}
		})
		if st.link.DeepLink != "" {
			muted(c, "Cara lain: kirim kode ini di grup WhatsApp divisi.").FontSize(12)
		} else {
			muted(c, "Kirim kode ini di grup WhatsApp divisi.").FontSize(12)
		}
	} else {
		muted(c, "Kodenya dibuat sebelum halaman dimuat ulang dan tidak bisa ditampilkan lagi. Kirim kode yang sudah kamu salin, atau buat kode baru.").FontSize(12)
		if spaceButton(c.Key("make-link-again"), "Buat kode baru", "refresh", false).Disabled(!ok || sp == nil || sp.ReadOnly).Clicked() {
			a.createOwnWALink(cl)
		}
	}
	// Until the bot got the code: check every few seconds (also on realtime).
	if !st.linkStatusLoading && now.After(st.linkPollNext) {
		st.linkPollNext = now.Add(4 * time.Second)
		a.loadWALinkStatus()
	}
	c.After(time.Second)
}

func (a *App) createOwnWALink(cl *client.Client) {
	st := &a.whatsapp
	gen, seq, pid, did := a.gen, st.seq, st.period, st.division
	st.busy = true
	a.async(func() func() {
		link, err := cl.CreateWALink(context.Background(), pid, did)
		return func() {
			if !a.waCurrent(gen, seq) {
				return
			}
			st := &a.whatsapp
			st.busy = false
			if err != nil {
				st.linkErr = "Kode belum bisa dibuat: " + errorText(err)
				return
			}
			st.link = &link
			if st.linkStatus != nil {
				st.linkStatus.Pending = &api.WALinkPending{ExpiresAt: link.ExpiresAt}
			}
			st.linkPollNext = time.Now().Add(4 * time.Second)
		}
	})
}

// loadWALinkStatus fetches the member's own link. A failure is shown with a
// retry instead of pretending there is no bot.
func (a *App) loadWALinkStatus() {
	st := &a.whatsapp
	cl := a.notificationClient()
	if cl == nil || st.division == id.Nil || a.offlineMode() {
		return
	}
	gen, seq, pid, did := a.gen, st.seq, st.period, st.division
	st.linkStatusLoading = true
	a.async(func() func() {
		status, err := cl.WALinkStatus(context.Background(), pid, did)
		return func() {
			st := &a.whatsapp
			st.linkStatusLoading = false
			if !a.waCurrent(gen, seq) || st.division != did {
				return
			}
			if err != nil {
				st.linkErr = "Status WhatsApp belum bisa dimuat: " + errorText(err)
				return
			}
			st.linkErr = ""
			was := st.linkStatus != nil && st.linkStatus.Linked
			st.linkStatus, st.linkStatusFor = &status, did
			if status.Linked && !was && (st.link != nil || status.Pending == nil) {
				if st.link != nil {
					st.linkToast = "WhatsApp tertaut."
				}
				st.link = nil
				a.access.members.loaded = false
			}
		}
	})
}

func (a *App) unlinkOwnWA(cl *client.Client) {
	st := &a.whatsapp
	gen, seq, pid, did := a.gen, st.seq, st.period, st.division
	st.busy = true
	a.async(func() func() {
		err := cl.UnlinkWA(context.Background(), pid, did)
		return func() {
			if !a.waCurrent(gen, seq) {
				return
			}
			st := &a.whatsapp
			st.busy, st.unlinkAsk = false, false
			if err != nil {
				st.linkErr = "Tautan belum bisa dilepas: " + errorText(err)
				return
			}
			st.linkStatus = &api.WALinkStatus{BotReady: st.linkStatus != nil && st.linkStatus.BotReady}
			st.linkToast = "Tautan WhatsApp dilepas."
			a.access.members.loaded = false
		}
	})
}

func (a *App) waOwnLinkPanel(c *ui.Context) {
	t := c.Theme()
	ui.Column(c.Key("wa-own")).FillWidth().MinWidth(0).Padding(16, 20).Gap(8).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() { a.waOwnLink(c) })
}

// waWhatPanel is "Yang dilakukan bot".
func (a *App) waWhatPanel(c *ui.Context) {
	t := c.Theme()
	ui.Column(c.Key("wa-what")).FillWidth().MinWidth(0).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, "Yang dilakukan bot").FontSize(15).Bold().Padding(14, 20)
		for i, item := range []struct{ icon, title, text string }{
			{"clock", "Mengingatkan tenggat tugas", "H-2, H-1 dan hari H pukul 06.00 ke grup dan ke PJ."},
			{"send", "Meneruskan ping dari Space", "Ping yang kamu kirim dengan kanal WhatsApp."},
			{"terminal", "Menjawab perintah di grup", "!tugas  !hari  !pending  !rekap"},
		} {
			ui.Row(c.Key(i)).FillWidth().MinWidth(0).Gap(12).Padding(12, 20).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
				ui.Box(c).Size(28, 28).Radius(8).Background(t.SurfaceHover).Center().Shrink(0).Children(func() {
					ui.Icon(c, spaceIcon(item.icon)).Size(16, 16).TextColor(t.TextMuted)
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, item.title).FontSize(13).FontWeight(500)
					muted(c, item.text).FontSize(12)
				})
			})
		}
	})
}

// waMembersPanel is "Nomor anggota": Tertaut / Belum per member.
func (a *App) waMembersPanel(c *ui.Context, s waStatus) {
	t := c.Theme()
	m := &a.access.members
	ui.Column(c.Key("wa-members")).FillWidth().MinWidth(0).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Row(c).FillWidth().Padding(14, 20).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, "Nomor anggota").FontSize(15).Bold().Grow(1)
			if s.membersKnown {
				muted(c, fmt.Sprintf("%d/%d tertaut", s.linked, s.total)).FontSize(12)
			}
		})
		ui.Column(c).FillWidth().Padding(4, 20).Children(func() {
			if !s.membersKnown {
				ui.Column(c).Padding(12, 0).Children(func() { muted(c, "Status nomor anggota belum tersedia.").FontSize(12) })
				return
			}
			for i, member := range m.members {
				if member.WhatsAppLinked == nil {
					continue
				}
				row := ui.Row(c.Key(member.UserID)).FillWidth().MinWidth(0).Gap(12).Padding(8, 0).AlignItems(ui.Center)
				if i > 0 {
					row.BorderWidth(1, 0, 0, 0).BorderColor(t.Border)
				}
				row.Children(func() {
					avatar(c, member.DisplayName, 28)
					ui.Text(c, member.DisplayName).FontSize(13).Grow(1).MinWidth(0).MaxLines(1)
					if *member.WhatsAppLinked {
						ui.Text(c, "Tertaut").FontSize(12).TextColor(t.Success)
					} else {
						ui.Text(c, "Belum").FontSize(12).TextColor(t.Warning)
					}
				})
			}
		})
	})
}

// waAdvanced is "Pengaturan lanjutan": name, language, greeting, the
// messages (each with Pratinjau), custom commands and the send log.
func (a *App) waAdvanced(c *ui.Context, phone bool) {
	t := c.Theme()
	st := &a.whatsapp
	ui.Column(c.Key("wa-advanced")).FillWidth().MinWidth(0).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
		head := ui.ButtonBase(c.Key("adv-toggle")).Label("Pengaturan lanjutan").FillWidth().Row().AlignItems(ui.Center).Gap(12).Padding(14, 20)
		if head.Hovered() {
			head.Background(t.SurfaceHover)
		}
		head.FocusRing(head.FocusVisible()).Children(func() {
			ui.Icon(c, spaceIcon("settings")).Size(16, 16).TextColor(t.TextMuted)
			ui.Text(c, "Pengaturan lanjutan").FontSize(14).FontWeight(500)
			if !phone {
				muted(c, "nama bot, sapaan, teks pesan, perintah khusus, catatan pengiriman").FontSize(12).Grow(1).MinWidth(0).MaxLines(1)
			} else {
				ui.Box(c).Grow(1)
			}
			icon := "right"
			if st.advancedOpen {
				icon = "down"
			}
			ui.Icon(c, spaceIcon(icon)).Size(16, 16).TextColor(t.TextMuted)
		})
		if head.Clicked() {
			st.advancedOpen = !st.advancedOpen
		}
		if !st.advancedOpen {
			return
		}
		_, ok := a.waClient()
		ui.Column(c).FillWidth().MinWidth(0).Padding(4, 20, 20, 20).Gap(14).Children(func() {
			a.waField(c, "wa-name", "Nama bot", &st.name, "reminder")
			ui.Row(c).Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
				muted(c, "Bahasa pesan bot").FontSize(12)
				for _, language := range []struct{ code, label string }{{"id", "Indonesia"}, {"en", "English"}} {
					if spaceButton(c.Key(language.code), language.label, "", st.language == language.code).Padding(4, 10).MinHeight(30).Disabled(!ok).Clicked() {
						st.language = language.code
					}
				}
			})
			a.waField(c, "wa-greeting", "Sapaan", &st.greeting, "reminder")
			ui.Text(c, "Teks pesan").FontSize(13).Bold()
			muted(c, "Kosongkan untuk teks bawaan. Bagian dalam kurung diisi otomatis: [Isi pesan] wajib ada, [Nama bot], [Sapaan] dan [Judul] boleh dipakai.").FontSize(12)
			for i, key := range waTemplateKeys {
				a.waField(c, "tpl-"+key, waTemplateLabels[key], &st.templates[i], key)
			}
			if primarySpaceButton(c.Key("save-config"), "Simpan pengaturan", "check").Disabled(!ok).Clicked() {
				a.saveWAConfig()
			}
			ui.Row(c).Wrap().Gap(8).Children(func() {
				if spaceButton(c.Key("commands"), "Perintah khusus", "terminal", false).Clicked() {
					a.chrome.settingsTab = "Command"
					a.navigate(PathPengaturan)
				}
				muted(c, fmt.Sprintf("Catatan pengiriman: %d", len(st.outbox))).FontSize(12)
			})
			for _, item := range st.outbox[:min(3, len(st.outbox))] {
				muted(c, item.GroupName+" · "+item.Status+" · "+item.CreatedAt.Local().Format("02 Jan 15.04")).FontSize(12)
			}
		})
	})
}

// waField is a text the bot sends, with its token chips and Pratinjau.
func (a *App) waField(c *ui.Context, key, label string, value *string, preview string) {
	ui.Column(c.Key(key)).FillWidth().MinWidth(0).Gap(6).Children(func() {
		if strings.HasPrefix(key, "tpl-") {
			ui.Field(c, label, func() { ui.TextArea(c, value).Label(label).Placeholder("Bawaan").FillWidth().Height(72) })
		} else {
			ui.Field(c, label, func() { ui.TextInput(c, value).Label(label).Placeholder("Bawaan").FillWidth() })
		}
		ui.Row(c).Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
			if strings.HasPrefix(key, "tpl-") {
				for _, tk := range waTokens {
					tk := tk
					if spaceButton(c.Key("ins"+tk.word), "+ "+strings.Trim(tk.word, "[]"), "", false).Label("Sisipkan "+tk.word+" ke "+label).Padding(2, 8).MinHeight(26).Clicked() {
						*value += tk.word
					}
				}
			}
			a.waPreviewButton(c, preview+"|"+key)
		})
		a.waPreviewBubble(c, preview+"|"+key)
	})
}

// saveWAConfig saves name, language, greeting and the messages.
func (a *App) saveWAConfig() {
	st := &a.whatsapp
	cl, ok := a.waClient()
	if !ok {
		return
	}
	pid, did, conn := st.period, st.division, st.connection
	req := api.WAConnectionRequest{DisplayName: st.name, Language: st.language, Greeting: st.greeting, Templates: a.waTemplatesForServer()}
	if conn != nil {
		req.ExpectedVersion = conn.Version
	}
	a.waRun(func() error {
		if conn == nil {
			_, err := cl.CreateWAConnection(context.Background(), pid, did, req)
			return err
		}
		_, err := cl.UpdateWAConnection(context.Background(), conn.ID, req)
		return err
	})
}

func (a *App) waTemplatesForServer() map[string]string {
	out := map[string]string{}
	for i, v := range a.whatsapp.templates {
		if strings.TrimSpace(v) != "" {
			out[waTemplateKeys[i]] = waServerTemplate(v)
		}
	}
	return out
}

// --- Pratinjau ---

// waPreviewState is one preview: open, the draft last asked for, its text.
type waPreviewState struct {
	open           bool
	asked, pending string
	pendingAt      time.Time
	text, err      string
	loading        bool
}

func (a *App) waPreviewButton(c *ui.Context, pv string) {
	st := &a.whatsapp
	if st.previews == nil {
		st.previews = map[string]*waPreviewState{}
	}
	p := st.previews[pv]
	open := p != nil && p.open
	label := "Pratinjau"
	if open {
		label = "Tutup pratinjau"
	}
	if spaceButton(c.Key("pv-"+pv), label, "search", open).Padding(2, 10).MinHeight(26).Clicked() {
		if p == nil {
			p = &waPreviewState{}
			st.previews[pv] = p
		}
		p.open = !p.open
	}
}

// waPreviewDraft is the request a preview of id stands for now.
func (a *App) waPreviewDraft(pv string) api.WAPreviewRequest {
	st := &a.whatsapp
	kind, _, _ := strings.Cut(pv, "|")
	return api.WAPreviewRequest{Kind: kind, DisplayName: st.name, Language: st.language, Greeting: st.greeting, Templates: a.waTemplatesForServer()}
}

// waPreviewBubble shows the preview as a WhatsApp message, asking the
// server again 300 ms after the draft stops changing.
func (a *App) waPreviewBubble(c *ui.Context, pv string) {
	st := &a.whatsapp
	p := st.previews[pv]
	if p == nil || !p.open {
		return
	}
	req := a.waPreviewDraft(pv)
	key := fmt.Sprintf("%v", req)
	now := c.Now()
	if key != p.pending {
		p.pending, p.pendingAt = key, now
	}
	if p.pending != p.asked && !p.loading {
		if wait := 300*time.Millisecond - now.Sub(p.pendingAt); wait > 0 {
			c.After(wait)
		} else {
			a.waAskPreview(pv, req, key)
		}
	}
	waBubble(c, p.text, p.err, p.loading && p.text == "")
}

func (a *App) waAskPreview(pv string, req api.WAPreviewRequest, key string) {
	st := &a.whatsapp
	p := st.previews[pv]
	cl := a.notificationClient()
	if cl == nil || st.division == id.Nil {
		p.asked, p.err = key, "Pratinjau memerlukan koneksi ke server."
		return
	}
	gen, seq, pid, did := a.gen, st.seq, st.period, st.division
	p.loading, p.asked = true, key
	a.async(func() func() {
		out, err := cl.PreviewWA(context.Background(), pid, did, req)
		return func() {
			if !a.waCurrent(gen, seq) {
				return
			}
			p := a.whatsapp.previews[pv]
			if p == nil {
				return
			}
			p.loading = false
			if uiPreferencesMissing(err) {
				p.err = "Pratinjau belum tersedia di server ini."
				return
			}
			if err != nil {
				p.err = a.failed(err)
				return
			}
			p.text, p.err = out.Text, ""
		}
	})
}

// waBubble draws a message as WhatsApp shows it in a group, labelled as an
// example.
func waBubble(c *ui.Context, text, err string, loading bool) {
	t := c.Theme()
	ui.Column(c.Key("bubble")).FillWidth().MinWidth(0).Padding(12).Gap(8).Radius(12).Background(ui.Hex("#0b141a")).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, spaceIcon("users")).Size(14, 14).TextColor(ui.Hex("#8696a0"))
			ui.Text(c, "Contoh di grup WhatsApp").FontSize(12).TextColor(ui.Hex("#8696a0")).Grow(1)
		})
		switch {
		case err != "":
			ui.Text(c, err).FontSize(12).TextColor(t.Danger)
		case loading:
			ui.Text(c, "Menyusun pratinjau…").FontSize(12).TextColor(ui.Hex("#8696a0"))
		default:
			ui.Column(c).MaxWidth(520).MinWidth(0).Padding(8, 12).Gap(4).Radius(8).Background(ui.Hex("#202c33")).Children(func() {
				ui.Text(c, text).FontSize(13).TextColor(ui.Hex("#e9edef")).Selectable()
				ui.Text(c, "Contoh data").FontSize(11).TextColor(ui.Hex("#8696a0"))
			})
		}
	})
}
