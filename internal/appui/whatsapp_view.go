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
	// The member's own link (waOwnLink): status, polled while a code is
	// open, and the unlink confirmation.
	linkStatus          *api.WALinkStatus
	linkStatusFor       id.ID
	linkStatusLoading   bool
	linkPollNext        time.Time
	unlinkAsk           bool
	linkErr             string
	linkToast           string
	mapSpace            id.ID
	mapJID              string
	commandsEnabled     bool
	command             *api.WACustomCommand
	polling             bool
	pollSeq             uint64
	pollNext, pollUntil time.Time
	// Tahap 7d: the opened setup step, "Pengaturan lanjutan", previews.
	step         int
	advancedOpen bool
	previews     map[string]*waPreviewState
	// previewOpen is the one preview shown: the field typed in last, until
	// it is closed or another field opens its own.
	previewOpen string
	editors     map[string]*waEditorState
	// commandText is the custom command's response being edited, with
	// words for its placeholders.
	commandText string
}

func waQRBitmap(value string) *ui.Bitmap {
	if !strings.HasPrefix(value, "data:image/png;base64,") || len(value) >= 2<<20 {
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "data:image/png;base64,"))
	if err != nil {
		return nil
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	return ui.NewBitmap(decoded)
}

// Poll only the session while pairing, without replacing unsaved settings.
func (a *App) waPollPairing(c *ui.Context) {
	st := &a.whatsapp
	if st.connection == nil || st.pollUntil.IsZero() || c.Now().After(st.pollUntil) {
		return
	}
	if st.polling || st.loading || st.busy {
		c.After(time.Second)
		return
	}
	if c.Now().Before(st.pollNext) {
		c.After(st.pollNext.Sub(c.Now()))
		return
	}
	cl := a.notificationClient()
	if cl == nil {
		return
	}
	gen, seq, pollSeq, connectionID := a.gen, st.seq, st.pollSeq, st.connection.ID
	st.polling = true
	st.pollNext = c.Now().Add(3 * time.Second)
	a.async(func() func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		session, err := cl.WAState(ctx, connectionID)
		qr := waQRBitmap(session.QR)
		return func() {
			if !a.waCurrent(gen, seq) || a.whatsapp.pollSeq != pollSeq {
				return
			}
			current := &a.whatsapp
			current.polling = false
			if err != nil {
				current.notice = "Status perangkat belum dapat diperbarui. Coba muat ulang."
				return
			}
			current.session, current.qr = session, qr
			if session.Status == "connected" {
				current.pollUntil = time.Time{}
				current.notice = "WhatsApp berhasil terhubung."
				a.loadWhatsApp()
			}
		}
	})
	c.After(3 * time.Second)
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
				qr = waQRBitmap(session.QR)
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
			st.templates[i] = waFriendly(connection.Templates[key])
		}
	}
}

func (a *App) waRun(work func() error) {
	if a.whatsapp.busy || a.whatsapp.loading || a.offlineMode() {
		return
	}
	gen, seq := a.gen, a.whatsapp.seq
	a.whatsapp.pollSeq++
	a.whatsapp.polling = false
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
				a.whatsapp.pollUntil = time.Time{}
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
			// Also the member's own link: the reload button must reach it.
			st.linkErr = ""
			a.loadWALinkStatus()
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
	settingsBody(c, "Command bot", "Kelola pemicu, respons, dan cakupan grup untuk command khusus.", func() {
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
		if spaceButton(c, "Buat perintah baru", "plus", false).Disabled(disabled).Clicked() {
			st.command = &api.WACustomCommand{ID: id.New(), GroupIDs: []id.ID{}}
			st.commandText = ""
		}
		for _, command := range st.commands {
			if spaceButton(c.Key(command.ID.String()), "!"+command.Trigger, "terminal", st.command != nil && st.command.ID == command.ID).Disabled(disabled).Clicked() {
				copy := command
				copy.GroupIDs = slices.Clone(command.GroupIDs)
				st.command = &copy
				st.commandText = waFriendlyWith(waCommandTokens, command.Response)
			}
		}
		draft := st.command
		if draft == nil {
			return
		}
		ui.Field(c, "Nama perintah", func() {
			ui.TextInput(c, &draft.Trigger).Label("Nama perintah").Placeholder("misalnya halo, lalu anggota mengetik !halo").FillWidth()
		})
		pv := "command|" + draft.ID.String()
		a.waTemplateEditor(c, "command-response-"+draft.ID.String(), "Balasan bot", &st.commandText, waCommandTokens, true, pv, func() { waCommandPreview(c, st.commandText, c.Now()) })
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
			trigger := strings.TrimPrefix(strings.TrimSpace(draft.Trigger), "!")
			req := api.WACustomCommandRequest{Trigger: trigger, Response: waServerWith(waCommandTokens, st.commandText), AssetID: draft.AssetID, MediaType: draft.MediaType, GroupIDs: slices.Clone(draft.GroupIDs), ExpectedVersion: draft.Version}
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
	settingsBody(c, "Aktivitas WhatsApp", "Tinjau antrean dan status pengiriman pada koneksi divisi.", func() {
		if !a.waHeader(c) {
			return
		}
		st := &a.whatsapp
		if !a.waCanManage() {
			return
		}
		if len(st.outbox) == 0 {
			pageEmpty(c, "clock", "Belum ada aktivitas", "Belum ada job pengiriman pada koneksi ini.")
		}
		for _, item := range st.outbox {
			ui.Column(c.Key(item.ID.String())).FillWidth().MinWidth(0).Padding(16).Gap(6).Radius(10).Background(c.Theme().Surface).Border(1, c.Theme().Border).Children(func() {
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
