package appui

import (
	"bytes"
	"context"
	"encoding/base64"
	"image/png"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

var waTemplateKeys = [...]string{"reminder", "ping", "reply", "weekly", "followup"}

type whatsappUIState struct {
	seq                                  uint64
	user, space, period, division        id.ID
	loaded, loading, busy, confirmLogout bool
	err, notice                          string
	connection                           *api.WAConnection
	session                              api.WASessionState
	qr                                   *ui.Bitmap
	groups                               []api.WAAvailableGroup
	commands                             []api.WACustomCommand
	outbox                               []api.WAOutboxItem
	name, language, greeting             string
	templates                            [5]string
	link                                 *api.WALinkChallenge
	mapSpace                             id.ID
	mapJID                               string
	commandsEnabled                      bool
	command                              *api.WACustomCommand
}

func (a *App) ensureWhatsApp() {
	sp := a.selectedSpace()
	if sp == nil {
		return
	}
	st := &a.whatsapp
	if st.user != a.user.ID || st.space != sp.ID {
		*st = whatsappUIState{seq: st.seq + 1, user: a.user.ID, space: sp.ID, period: sp.PeriodID, name: "Space", language: "id", commandsEnabled: true, mapSpace: sp.ID}
		for _, role := range sp.Roles {
			if slices.Contains(role.Capabilities, api.CapTasksOwn) {
				st.division = role.DivisionID
				break
			}
		}
	}
	if st.division != id.Nil && !st.loaded && !st.loading && st.err == "" {
		a.loadWhatsApp()
	}
}

func (a *App) waCanManage() bool {
	sp := a.selectedSpace()
	if sp == nil || sp.ReadOnly {
		return false
	}
	return slices.ContainsFunc(sp.Roles, func(role api.Access) bool {
		return role.DivisionID == a.whatsapp.division && slices.Contains(role.Capabilities, api.CapWhatsAppManage)
	})
}
func (a *App) waCurrent(gen int, seq uint64) bool {
	return a.gen == gen && a.whatsapp.seq == seq && a.whatsapp.user == a.user.ID && a.whatsapp.space == a.selectedSpaceID()
}

func (a *App) loadWhatsApp() {
	cl := a.notificationClient()
	st := &a.whatsapp
	if cl == nil || st.loading || st.busy || st.division == id.Nil {
		return
	}
	if !a.waCanManage() {
		st.loaded = true
		return
	}
	gen, seq, pid, did := a.gen, st.seq, st.period, st.division
	st.loading = true
	st.err = ""
	st.qr = nil
	a.async(func() func() {
		connections, err := cl.WAConnections(context.Background(), pid, did)
		var connection *api.WAConnection
		var session api.WASessionState
		var qr *ui.Bitmap
		var groups []api.WAAvailableGroup
		var commands []api.WACustomCommand
		var outbox []api.WAOutboxItem
		notice := ""
		if err == nil && len(connections) > 0 {
			connection = &connections[0]
			var stateErr error
			session, stateErr = cl.WAState(context.Background(), connection.ID)
			// Keep saved configuration usable when the separate bridge is offline.
			if stateErr != nil {
				notice = "Konfigurasi tersimpan. Status bridge belum dapat diperiksa."
			} else {
				if strings.HasPrefix(session.QR, "data:image/png;base64,") && len(session.QR) < 2<<20 {
					data, e := base64.StdEncoding.DecodeString(strings.TrimPrefix(session.QR, "data:image/png;base64,"))
					if e == nil {
						decoded, decodeErr := png.Decode(bytes.NewReader(data))
						if decodeErr == nil {
							qr = ui.NewBitmap(decoded)
						}
					}
				}
				if session.Status == "connected" {
					groups, _ = cl.WAAvailableGroups(context.Background(), connection.ID)
				}
			}
			commands, err = cl.WACommands(context.Background(), connection.ID)
			if err == nil {
				outbox, err = cl.WAOutbox(context.Background(), connection.ID)
			}
		}
		return func() { a.finishWALoad(gen, seq, connection, session, qr, groups, commands, outbox, notice, err) }
	})
}

func (a *App) finishWALoad(gen int, seq uint64, connection *api.WAConnection, session api.WASessionState, qr *ui.Bitmap, groups []api.WAAvailableGroup, commands []api.WACustomCommand, outbox []api.WAOutboxItem, notice string, err error) {
	if !a.waCurrent(gen, seq) {
		return
	}
	st := &a.whatsapp
	st.loading = false
	if err != nil {
		st.err = a.failed(err)
		return
	}
	st.connection = connection
	st.session = session
	st.qr = qr
	st.groups = groups
	st.commands = commands
	st.outbox = outbox
	st.notice = notice
	st.loaded = true
	if connection != nil {
		st.name = connection.DisplayName
		st.language = connection.Language
		st.greeting = connection.Greeting
		for i, key := range waTemplateKeys {
			st.templates[i] = connection.Templates[key]
		}
	}
}

func (a *App) waRun(work func() error) {
	if a.whatsapp.busy || a.whatsapp.loading || a.offlineMode() {
		return
	}
	gen, seq := a.gen, a.whatsapp.seq
	a.whatsapp.busy = true
	a.whatsapp.err = ""
	a.async(func() func() {
		err := work()
		return func() {
			if !a.waCurrent(gen, seq) {
				return
			}
			a.whatsapp.busy = false
			a.whatsapp.qr = nil
			if err != nil {
				a.whatsapp.err = a.failed(err)
				return
			}
			a.whatsapp.command = nil
			a.whatsapp.confirmLogout = false
			a.loadWhatsApp()
		}
	})
}

func (a *App) waHeader(c *ui.Context) bool {
	a.ensureWhatsApp()
	sp := a.selectedSpace()
	st := &a.whatsapp
	if sp == nil {
		pageNotice(c, "info", "Pilih Space", "Pilih ruang divisi untuk mengatur atau menautkan WhatsApp.")
		return false
	}
	ui.Row(c).FillWidth().Gap(8).Children(func() {
		for _, role := range sp.Roles {
			if !slices.Contains(role.Capabilities, api.CapTasksOwn) {
				continue
			}
			name := "Divisi"
			for _, d := range sp.Divisions {
				if d.ID == role.DivisionID {
					name = d.Name
				}
			}
			if spaceButton(c.Key(role.DivisionID.String()), name, "users", st.division == role.DivisionID).Disabled(st.loading || st.busy).Clicked() {
				*st = whatsappUIState{seq: st.seq + 1, user: a.user.ID, space: sp.ID, period: sp.PeriodID, division: role.DivisionID, name: "Space", language: "id", commandsEnabled: true, mapSpace: sp.ID}
				a.loadWhatsApp()
			}
		}
		if spaceButton(c, "Muat ulang", "refresh", false).Disabled(st.loading || st.busy || a.offlineMode()).Clicked() {
			st.err = ""
			a.loadWhatsApp()
		}
	})
	if st.loading {
		muted(c, "Memuat konfigurasi dan status…")
	}
	if st.err != "" {
		pageNotice(c, "alert", "Permintaan belum selesai", st.err)
	}
	if st.notice != "" {
		pageNotice(c, "info", "Status koneksi", st.notice)
	}
	if a.offlineMode() {
		pageNotice(c, "info", "Sedang offline", "Pengaturan bot dan tautan identitas memerlukan koneksi.")
	}
	return st.division != id.Nil
}

func (a *App) whatsappSettings(c *ui.Context) {
	pageBody(c, "WhatsApp divisi", func() {
		if !a.waHeader(c) {
			return
		}
		st := &a.whatsapp
		cl := a.notificationClient()
		disabled := st.busy || st.loading || a.offlineMode() || cl == nil
		pageSection(c, "Identitas akunmu", "Tautkan akun Space ke pengirim WhatsApp dengan kode sekali pakai.", func() {
			if spaceButton(c, "Buat kode tautan", "link", false).Disabled(disabled || a.selectedSpace().ReadOnly).Clicked() {
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
							st.err = a.failed(err)
							return
						}
						st.link = &link
					}
				})
			}
			if st.link != nil && time.Now().Before(st.link.ExpiresAt) {
				ui.Text(c, st.link.Instructions)
				muted(c, "Berlaku sampai "+st.link.ExpiresAt.Local().Format("15:04"))
				c.After(time.Second)
			}
		})
		if !a.waCanManage() {
			pageNotice(c, "shield", "Pengaturan koneksi", "Koneksi dan command dikelola kadiv atau pemegang izin WhatsApp divisi.")
			return
		}
		pageSection(c, "Identitas dalam pesan", "Nama, bahasa dan template tersimpan untuk koneksi divisi ini. Template memakai {body}; tersedia juga {title}, {name}, dan {greeting}.", func() {
			ui.TextInput(c, &st.name).Label("Nama bot").FillWidth()
			ui.Row(c).Wrap().Gap(5).Children(func() {
				for _, language := range []struct{ code, label string }{{"id", "Indonesia"}, {"en", "English"}} {
					if spaceButton(c.Key(language.code), language.label, "", st.language == language.code).Disabled(disabled).Clicked() {
						st.language = language.code
					}
				}
			})
			muted(c, "Bahasa berlaku pada respons inti dan pesan baru. Judul, catatan pengguna, sapaan, template dan isi custom command tetap sesuai isinya.")
			ui.TextInput(c, &st.greeting).Label("Sapaan").FillWidth()
			for i, key := range waTemplateKeys {
				ui.TextInput(c.Key(key), &st.templates[i]).Label("Template " + key).FillWidth()
			}
			if spaceButton(c, "Simpan konfigurasi", "check", false).Disabled(disabled).Clicked() {
				pid, did := st.period, st.division
				conn := st.connection
				req := api.WAConnectionRequest{DisplayName: st.name, Language: st.language, Greeting: st.greeting, Templates: map[string]string{}}
				for i, v := range st.templates {
					if v != "" {
						req.Templates[waTemplateKeys[i]] = v
					}
				}
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
		})
		if st.connection == nil {
			return
		}
		conn := *st.connection
		pageSection(c, "Sesi WhatsApp", "Gunakan nomor divisi; QR hanya ditampilkan kepada pengelola berwenang.", func() {
			status := st.session.Status
			if status == "" {
				status = conn.Status
			}
			ui.Text(c, "Status: "+status).Bold()
			if st.session.LastError != "" {
				muted(c, st.session.LastError)
			}
			ui.Row(c).Gap(8).Children(func() {
				for _, action := range []struct{ key, label string }{{"connect", "Hubungkan / pasangkan"}, {"disconnect", "Putuskan"}} {
					if spaceButton(c.Key(action.key), action.label, "whatsapp", false).Disabled(disabled).Clicked() {
						key := action.key
						a.waRun(func() error {
							return cl.WAAction(context.Background(), conn.ID, api.WAActionRequest{Action: key, ExpectedVersion: conn.Version})
						})
					}
				}
				if spaceButton(c, "Keluarkan akun bot", "logout", false).Disabled(disabled).Clicked() {
					st.confirmLogout = true
				}
			})
			if st.confirmLogout {
				pageNotice(c, "alert", "Keluarkan sesi?", "Tindakan ini menghapus sesi koneksi ini dan memerlukan pairing ulang.")
				if spaceButton(c, "Ya, keluarkan sesi ini", "logout", false).Disabled(disabled).Clicked() {
					a.waRun(func() error {
						return cl.WAAction(context.Background(), conn.ID, api.WAActionRequest{Action: "logout", ExpectedVersion: conn.Version, Confirmed: true})
					})
				}
				if spaceButton(c, "Batal", "close", false).Clicked() {
					st.confirmLogout = false
				}
			}
			if st.qr != nil && st.session.QRExpiresAt != nil && time.Now().Before(*st.session.QRExpiresAt) {
				ui.Image(c, st.qr).Size(240, 240).Fit(ui.Contain)
				muted(c, "Pindai lewat Perangkat tertaut WhatsApp, lalu muat ulang status.")
				c.After(time.Second)
			}
		})
		pageSection(c, "Grup tujuan", "Pilih Space lalu grup yang benar-benar tersedia pada akun bot.", func() {
			for _, g := range conn.Groups {
				muted(c, g.Name+" → "+a.waSpaceName(g.SpaceID))
			}
			ui.Row(c).Gap(8).Children(func() {
				for _, sp := range a.access.spaces {
					if sp.PeriodID != st.period || !slices.ContainsFunc(sp.Divisions, func(d api.Division) bool { return d.ID == st.division }) {
						continue
					}
					if spaceButton(c.Key(sp.ID.String()), sp.Name, "users", st.mapSpace == sp.ID).Disabled(disabled).Clicked() {
						st.mapSpace = sp.ID
					}
				}
			})
			for _, g := range st.groups {
				if spaceButton(c.Key(g.JID), g.Name, "whatsapp", st.mapJID == g.JID).Disabled(disabled).Clicked() {
					st.mapJID = g.JID
				}
			}
			if len(st.groups) == 0 {
				muted(c, "Hubungkan sesi dan muat ulang untuk melihat grup yang tersedia.")
			}
			if spaceButton(c, "Command aktif di grup: "+waYesNo(st.commandsEnabled), "terminal", st.commandsEnabled).Disabled(disabled).Clicked() {
				st.commandsEnabled = !st.commandsEnabled
			}
			if spaceButton(c, "Simpan grup tujuan", "check", false).Disabled(disabled || st.mapJID == "" || st.mapSpace == id.Nil).Clicked() {
				req := api.WAGroupRequest{SpaceID: st.mapSpace, JID: st.mapJID, CommandsEnabled: st.commandsEnabled, ExpectedVersion: conn.Version}
				a.waRun(func() error { return cl.SaveWAGroup(context.Background(), conn.ID, req) })
			}
		})
		pageNotice(c, "link", "Medhum memakai bot Humas", "Bot Mediatek melayani grup divisinya. Pengaturan grup bersama Medhum dilakukan melalui koneksi Humas.")
	})
}

func waYesNo(value bool) string {
	if value {
		return "ya"
	}
	return "tidak"
}
func (a *App) waSpaceName(sid id.ID) string {
	for _, sp := range a.access.spaces {
		if sp.ID == sid {
			return sp.Name
		}
	}
	return "Space"
}

func (a *App) commandSettings(c *ui.Context) {
	pageBody(c, "Command bot", func() {
		if !a.waHeader(c) {
			return
		}
		st := &a.whatsapp
		cl := a.notificationClient()
		if !a.waCanManage() || st.connection == nil {
			pageNotice(c, "info", "Koneksi divisi", "Buat koneksi melalui pengaturan WhatsApp dengan akun pengelola terlebih dahulu.")
			return
		}
		disabled := st.busy || st.loading || a.offlineMode() || cl == nil
		conn := *st.connection
		muted(c, "Command inti tetap mengikuti izin tugas. Balas media dengan !setsticker damn untuk menyimpan stiker melalui WhatsApp.")
		if spaceButton(c, "Tambah command teks", "plus", false).Disabled(disabled).Clicked() {
			st.command = &api.WACustomCommand{ID: id.New(), GroupIDs: []id.ID{}}
		}
		for _, command := range st.commands {
			if spaceButton(c.Key(command.ID.String()), "!"+command.Trigger, "terminal", st.command != nil && st.command.ID == command.ID).Disabled(disabled).Clicked() {
				copy := command
				copy.GroupIDs = slices.Clone(command.GroupIDs)
				st.command = &copy
			}
		}
		draft := st.command
		if draft == nil {
			return
		}
		ui.TextInput(c, &draft.Trigger).Label("Pemicu, misalnya damn").FillWidth()
		ui.TextInput(c, &draft.Response).Label("Respons teks").FillWidth()
		if draft.AssetID != "" {
			muted(c, "Media tersimpan: "+draft.MediaType)
			if spaceButton(c, "Lepaskan media dari command", "close", false).Disabled(disabled).Clicked() {
				draft.AssetID = ""
				draft.MediaType = ""
			}
		}
		muted(c, "Tanpa pilihan grup, command berlaku di semua grup koneksi ini.")
		for _, g := range conn.Groups {
			selected := slices.Contains(draft.GroupIDs, g.ID)
			if spaceButton(c.Key(g.ID.String()), g.Name, "users", selected).Disabled(disabled).Clicked() {
				if selected {
					draft.GroupIDs = slices.DeleteFunc(draft.GroupIDs, func(gid id.ID) bool { return gid == g.ID })
				} else {
					draft.GroupIDs = append(draft.GroupIDs, g.ID)
				}
			}
		}
		if spaceButton(c, "Simpan command", "check", false).Disabled(disabled).Clicked() {
			req := api.WACustomCommandRequest{Trigger: draft.Trigger, Response: draft.Response, AssetID: draft.AssetID, MediaType: draft.MediaType, GroupIDs: slices.Clone(draft.GroupIDs), ExpectedVersion: draft.Version}
			commandID := draft.ID
			a.waRun(func() error { _, err := cl.SaveWACommand(context.Background(), conn.ID, commandID, req); return err })
		}
		if draft.Version > 0 {
			if spaceButton(c, "Hapus command", "trash", false).Disabled(disabled).Clicked() {
				commandID, version := draft.ID, draft.Version
				a.waRun(func() error { return cl.DeleteWACommand(context.Background(), conn.ID, commandID, version) })
			}
		}
	})
}

func (a *App) whatsappActivity(c *ui.Context) {
	pageBody(c, "Aktivitas WhatsApp", func() {
		if !a.waHeader(c) {
			return
		}
		st := &a.whatsapp
		if !a.waCanManage() {
			return
		}
		if len(st.outbox) == 0 {
			muted(c, "Belum ada job pengiriman pada koneksi ini.")
		}
		for _, item := range st.outbox {
			ui.Column(c.Key(item.ID.String())).FillWidth().Gap(4).Children(func() {
				ui.Text(c, item.GroupName+" · "+item.Status).Bold()
				muted(c, item.CreatedAt.Local().Format("02 Jan 15:04"))
				if item.SuppressedAt != nil {
					muted(c, "Ditahan setelah restore: "+item.SuppressionReason)
				}
				if item.LastError != "" {
					muted(c, item.LastError)
				}
			})
		}
	})
}
