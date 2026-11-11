package appui

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
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
	// activityProblems: Aktivitas shows only what failed or is uncertain.
	activityProblems bool
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

// waCoreCommands are the bot's own commands, which follow task permissions.
var waCoreCommands = []string{"!tugas", "!hari", "!pending", "!rekap", "!setsticker"}

// waMediaLabel says a command's media in words.
func waMediaLabel(kind string) string {
	switch kind {
	case "sticker":
		return "Stiker"
	case "image":
		return "Gambar"
	case "video":
		return "Video"
	}
	return kind
}

// commandSettings is Command bot, option B: the custom commands in a list,
// the one chosen in an editor, and a WhatsApp preview that follows what is
// typed.
func (a *App) commandSettings(c *ui.Context) {
	settingsPageView(c, "Command bot", "Perintah khusus yang dijawab bot di grup WhatsApp divisi.", true, nil, func() {
		if !a.waHeader(c) {
			return
		}
		st := &a.whatsapp
		cl := a.notificationClient()
		if !a.waCanManage() || st.connection == nil {
			pageNotice(c, "info", "Koneksi divisi", "Buat koneksi melalui pengaturan WhatsApp dengan akun pengelola terlebih dahulu.")
			return
		}
		t := c.Theme()
		disabled := st.busy || st.loading || a.offlineMode() || cl == nil
		conn := *st.connection
		ui.Row(c.Key("core")).FillWidth().Gap(6).Wrap().AlignItems(ui.Center).Children(func() {
			muted(c, "Perintah inti, mengikuti izin tugas:").FontSize(13)
			for _, k := range waCoreCommands {
				ui.Row(c.Key(k)).Height(22).Padding(0, 8).Radius(6).Border(1, t.Border).AlignItems(ui.Center).Children(func() {
					ui.Text(c, k).Font(FontMono).FontSize(12).TextColor(t.TextMuted)
				})
			}
		})
		w, _ := c.Size()
		list := func() {
			ui.Column(c.Key("command-list")).Width(220).Shrink(0).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
				for i, command := range st.commands {
					on := st.command != nil && st.command.ID == command.ID
					// Looking at a command needs no connection; saving does.
					b := ui.ButtonBase(c.Key(command.ID.String())).Label("!"+command.Trigger).FillWidth().Column().AlignItems(ui.Start).Gap(2).Padding(12, 14)
					if i > 0 {
						b.BorderWidth(1, 0, 0, 0).BorderColor(t.Border)
					}
					if on || b.Hovered() {
						b.Background(t.SurfaceHover)
					}
					b.FocusRing(b.FocusVisible()).Children(func() {
						ui.Text(c, "!"+command.Trigger).Font(FontMono).FontSize(14).MaxLines(1)
						scope := "Semua grup"
						if len(command.GroupIDs) > 0 {
							scope = fmt.Sprintf("%d grup", len(command.GroupIDs))
						}
						if command.AssetID != "" {
							scope += " · " + waMediaLabel(command.MediaType)
						}
						muted(c, scope).FontSize(12)
					})
					if b.Clicked() {
						copy := command
						copy.GroupIDs = slices.Clone(command.GroupIDs)
						st.command = &copy
						st.commandText = waFriendlyWith(waCommandTokens, command.Response)
					}
				}
				ui.Row(c).FillWidth().Padding(8, 8).BorderWidth(boolBorder(len(st.commands) > 0), 0, 0, 0).BorderColor(t.Border).Children(func() {
					if spaceButton(c, "Buat perintah baru", "plus", false).Disabled(disabled).FillWidth().Clicked() {
						st.command = &api.WACustomCommand{ID: id.New(), GroupIDs: []id.ID{}}
						st.commandText = ""
					}
				})
			})
		}
		preview := func() {
			ui.Column(c.Key("command-preview")).Gap(10).MinWidth(0).Children(func() {
				if st.command != nil {
					waCommandPreview(c, st.commandText, c.Now())
				}
				muted(c, "Nama pengirim, Tanggal, Jam, Nama grup, dan teks setelah perintah diisi oleh server Space 0.4.5 ke atas.").FontSize(12)
			})
		}
		editor := func() {
			draft := st.command
			box := ui.Column(c.Key("command-editor")).Grow(1).MinWidth(0).Padding(16).Gap(14).Radius(12).Background(t.Surface).Border(1, t.Border)
			box.Children(func() {
				if draft == nil {
					pageEmpty(c, "terminal", "Pilih perintah", "Pilih perintah di kiri untuk mengubahnya, atau buat perintah baru.")
					return
				}
				ui.Field(c, "Nama perintah", func() {
					ui.TextInput(c, &draft.Trigger).Label("Nama perintah").Placeholder("misalnya halo, lalu anggota mengetik !halo").FillWidth()
				})
				// The preview beside or under the editor is always open.
				a.waTemplateEditor(c, "command-response-"+draft.ID.String(), "Balasan bot", &st.commandText, waCommandTokens, true, "", nil)
				ui.Column(c.Key("media")).FillWidth().Gap(6).Children(func() {
					ui.Text(c, "Media balasan").FontSize(13).FontWeight(500)
					if draft.AssetID != "" {
						ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Wrap().Children(func() {
							statusPill(c, waMediaLabel(draft.MediaType), t.Accent)
							muted(c, "Dikirim bersama balasan.").FontSize(13).Grow(1)
							if SettingButton(c, "Lepaskan media dari command", "close").Disabled(disabled).Clicked() {
								draft.AssetID = ""
								draft.MediaType = ""
							}
						})
					} else {
						trigger := strings.TrimPrefix(strings.TrimSpace(draft.Trigger), "!")
						if trigger == "" {
							trigger = "namaperintah"
						}
						muted(c, "Agar perintah ini membalas dengan stiker: di grup WhatsApp, balas gambar, video pendek, atau stiker dengan !setsticker "+trigger+".").FontSize(13)
					}
				})
				ui.Column(c.Key("groups")).FillWidth().Gap(6).Children(func() {
					ui.Text(c, "Berlaku di").FontSize(13).FontWeight(500)
					muted(c, "Tanpa pilihan grup, command berlaku di semua grup koneksi ini.").FontSize(13)
					ui.Row(c).FillWidth().Gap(6).Wrap().Children(func() {
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
					})
				})
				ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
					if draft.Version > 0 && SettingButton(c, "Hapus command", "trash").Disabled(disabled).TextColor(t.Danger).Clicked() {
						commandID, version := draft.ID, draft.Version
						a.waRun(func() error { return cl.DeleteWACommand(context.Background(), conn.ID, commandID, version) })
					}
					ui.Box(c).Grow(1)
					if primarySpaceButton(c, "Simpan command", "check").Disabled(disabled).Clicked() {
						trigger := strings.TrimPrefix(strings.TrimSpace(draft.Trigger), "!")
						req := api.WACustomCommandRequest{Trigger: trigger, Response: waServerWith(waCommandTokens, st.commandText), AssetID: draft.AssetID, MediaType: draft.MediaType, GroupIDs: slices.Clone(draft.GroupIDs), ExpectedVersion: draft.Version}
						commandID := draft.ID
						a.waRun(func() error { _, err := cl.SaveWACommand(context.Background(), conn.ID, commandID, req); return err })
					}
				})
			})
		}
		// Three columns where the window allows; else the preview goes under
		// the editor.
		ui.Row(c.Key("command-panes")).FillWidth().Gap(16).AlignItems(ui.Start).Children(func() {
			list()
			if w >= 1600 {
				editor()
				ui.Column(c).Width(300).Shrink(0).Children(preview)
				return
			}
			ui.Column(c).Grow(1).MinWidth(0).Gap(16).Children(func() {
				editor()
				preview()
			})
		})
	})
}

func boolBorder(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// waOutboxStatus says an outbox status in words; problem marks what failed
// or is uncertain.
func waOutboxStatus(status string) (text string, problem bool) {
	switch status {
	case "queued":
		return "Antre", false
	case "sending":
		return "Mengirim", false
	case "unknown":
		return "Hasil belum pasti", true
	case "accepted":
		return "Diterima", false
	case "delivered":
		return "Terkirim", false
	case "read":
		return "Dibaca", false
	case "failed":
		return "Gagal", true
	case "cancelled":
		return "Dibatalkan", false
	}
	return status, false
}

// whatsappActivity is Aktivitas WhatsApp, option A: a table of the
// deliveries, statuses in words, with a filter for the problems.
func (a *App) whatsappActivity(c *ui.Context) {
	settingsPageView(c, "Aktivitas WhatsApp", "Antrean dan status pengiriman bot divisi.", true, nil, func() {
		if !a.waHeader(c) {
			return
		}
		st := &a.whatsapp
		if !a.waCanManage() {
			pageNotice(c, "info", "Aktivitas bot", "Aktivitas pengiriman terlihat oleh kadiv atau pemegang izin WhatsApp divisi.")
			return
		}
		if len(st.outbox) == 0 {
			pageEmpty(c, "clock", "Belum ada aktivitas", "Belum ada job pengiriman pada koneksi ini.")
			return
		}
		t := c.Theme()
		problems := 0
		for _, item := range st.outbox {
			if _, bad := waOutboxStatus(item.Status); bad || item.SuppressedAt != nil {
				problems++
			}
		}
		ui.Row(c.Key("filter")).FillWidth().Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
			sel := 0
			if st.activityProblems {
				sel = 1
			}
			if i := segmented(c, "Saring aktivitas", []string{fmt.Sprintf("Semua %d", len(st.outbox)), fmt.Sprintf("Bermasalah %d", problems)}, sel); i >= 0 {
				st.activityProblems = i == 1
			}
			muted(c, "Hasil belum pasti direkonsiliasi dengan jurnal; jangan kirim ulang hanya karena belum ada receipt.").FontSize(12).Grow(1).MinWidth(200)
		})
		cols := []struct {
			title string
			grow  float32
		}{{"WAKTU", .9}, {"TUJUAN", 1.6}, {"STATUS", 1.1}, {"PERCOBAAN", .7}, {"CATATAN", 2.4}}
		ui.Column(c.Key("activity")).FillWidth().Radius(12).Background(t.Surface).Border(1, t.Border).Clip().Children(func() {
			ui.Row(c.Key("head")).FillWidth().Height(36).Padding(0, 16).Gap(12).AlignItems(ui.Center).Background(t.SurfacePressed).Children(func() {
				for _, col := range cols {
					ui.Text(c, col.title).Font(FontMono).FontSize(10).TextColor(faint(t)).Grow(col.grow).Basis(0).MinWidth(0).MaxLines(1)
				}
			})
			shown := 0
			for _, item := range st.outbox {
				status, bad := waOutboxStatus(item.Status)
				if st.activityProblems && !bad && item.SuppressedAt == nil {
					continue
				}
				shown++
				note := item.LastError
				if item.SuppressedAt != nil {
					note = "Ditahan setelah restore: " + item.SuppressionReason
				}
				ui.Row(c.Key(item.ID.String())).FillWidth().Padding(12, 16).Gap(12).AlignItems(ui.Start).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
					ui.Text(c, item.CreatedAt.Local().Format("02 Jan 15.04")).Font(FontMono).FontSize(12).Grow(cols[0].grow).Basis(0).MinWidth(0)
					ui.Text(c, item.GroupName).FontSize(13).Grow(cols[1].grow).Basis(0).MinWidth(0).MaxLines(2)
					ui.Row(c).Grow(cols[2].grow).Basis(0).MinWidth(0).Children(func() {
						color := t.TextMuted
						switch {
						case bad:
							color = t.Danger
						case item.Status == "read" || item.Status == "delivered":
							color = t.Success
						}
						statusPill(c, status, color)
					})
					ui.Text(c, fmt.Sprint(item.Attempts)).Font(FontMono).FontSize(12).TextColor(t.TextMuted).Grow(cols[3].grow).Basis(0).MinWidth(0)
					if note == "" {
						note = "—"
					}
					ui.Text(c, note).FontSize(12).TextColor(t.TextMuted).Grow(cols[4].grow).Basis(0).MinWidth(0).MaxLines(3)
				})
			}
			if shown == 0 {
				ui.Row(c).FillWidth().Padding(16).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
					muted(c, "Tidak ada pengiriman bermasalah.").FontSize(13)
				})
			}
		})
	})
}
