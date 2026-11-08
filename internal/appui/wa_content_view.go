package appui

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/broadcast"
	"github.com/himasifunej/himasif-space/internal/id"
)

type waContentUIState struct {
	seq, selectionSeq                               uint64
	user, space                                     id.ID
	loaded, loading, busy, showAll, accessConfirmed bool
	err, reason, filter, example                    string
	policy                                          api.WAParserPolicy
	preview                                         []broadcast.Item
	items                                           []api.WAContentItem
	contents                                        []api.MedhumContent
	selected                                        *api.WAContentItem
	target                                          id.ID
	history                                         []api.WAContentReview
}

func (a *App) waContentCurrent(gen int, seq uint64) bool {
	return gen == a.gen && seq == a.waContent.seq && a.waContent.user == a.user.ID && a.waContent.space == a.selectedSpaceID()
}
func (a *App) ensureWAContent() {
	sp := a.selectedSpace()
	if sp == nil || sp.Code != "medhum" {
		return
	}
	st := &a.waContent
	if st.space != sp.ID || st.user != a.user.ID {
		*st = waContentUIState{seq: st.seq + 1, space: sp.ID, user: a.user.ID}
	}
	if !st.loaded && !st.loading && st.err == "" {
		a.loadWAContent()
	}
}
func (a *App) loadWAContent() {
	cl := a.notificationClient()
	st := &a.waContent
	if cl == nil || st.loading || st.busy || st.space == id.Nil {
		return
	}
	gen, seq, sid := a.gen, st.seq, st.space
	st.loading = true
	a.async(func() func() {
		policy, err := cl.WAParserPolicy(context.Background(), sid)
		var items []api.WAContentItem
		var contents []api.MedhumContent
		if err == nil {
			items, err = cl.WAContentInbox(context.Background(), sid)
		}
		if err == nil {
			contents, err = cl.MedhumContents(context.Background(), sid)
		}
		return func() {
			if !a.waContentCurrent(gen, seq) {
				return
			}
			st := &a.waContent
			st.loading = false
			if err != nil {
				st.err = a.failed(err)
				return
			}
			st.policy = policy
			st.items = items
			st.contents = contents
			st.loaded = true
			st.err = ""
			st.selected = nil
			st.history = nil
			st.selectionSeq++
		}
	})
}
func (a *App) selectWAContent(item api.WAContentItem) {
	cl := a.notificationClient()
	if cl == nil || a.waContent.busy {
		return
	}
	st := &a.waContent
	st.selected = &item
	st.reason = ""
	st.accessConfirmed = false
	st.filter = ""
	st.target = id.Nil
	st.history = nil
	st.selectionSeq++
	if item.ContentID != nil {
		st.target = *item.ContentID
	} else if len(item.Candidates) == 1 {
		st.target = item.Candidates[0]
	}
	gen, seq, selection, sid := a.gen, st.seq, st.selectionSeq, st.space
	a.async(func() func() {
		history, err := cl.WAContentHistory(context.Background(), sid, item.ID)
		return func() {
			if !a.waContentCurrent(gen, seq) || a.waContent.selectionSeq != selection {
				return
			}
			if err != nil {
				a.waContent.err = a.failed(err)
				return
			}
			a.waContent.history = history
		}
	})
}
func (a *App) decideWAContent(action string) {
	cl := a.notificationClient()
	st := &a.waContent
	if cl == nil || st.selected == nil || st.busy || a.offlineMode() {
		return
	}
	req := api.WAContentDecision{Action: action, ExpectedVersion: st.selected.Version, Reason: st.reason, ConfirmedAccess: st.accessConfirmed}
	if action == "apply" || action == "reassign" {
		target := st.target
		if target == id.Nil {
			st.err = "Pilih konten tujuan."
			return
		}
		req.ContentID = &target
	}
	gen, seq, sid, iid := a.gen, st.seq, st.space, st.selected.ID
	st.busy = true
	st.err = ""
	a.async(func() func() {
		_, err := cl.DecideWAContent(context.Background(), sid, iid, req)
		return func() {
			if !a.waContentCurrent(gen, seq) {
				return
			}
			a.waContent.busy = false
			if err != nil {
				a.waContent.err = a.failed(err)
				return
			}
			a.loadWAContent()
			a.medhum.loaded = false
		}
	})
}

// waContentBody is the Medhum "Inbox broadcast" tab: concepts and Drive
// evidence the bot read from the Medhum group, to pair with content.
func (a *App) waContentBody(c *ui.Context) {
	a.ensureWAContent()
	ui.Column(c.Key("wa-content")).FillWidth().MinWidth(0).Gap(16).Children(func() {
		sp := a.selectedSpace()
		if sp == nil || sp.Code != "medhum" {
			return
		}
		st := &a.waContent
		disabled := st.loading || st.busy || a.offlineMode()
		cl := a.notificationClient()
		ui.Row(c).Wrap().Gap(8).Children(func() {
			if spaceButton(c, "Muat ulang inbox", "refresh", false).Disabled(disabled).Clicked() {
				st.err = ""
				a.loadWAContent()
			}
			if spaceButton(c, "Tampilkan semua status", "archive", st.showAll).Clicked() {
				st.showAll = !st.showAll
			}
		})
		muted(c, "Penyerahan konsep, hasil desain, dan publikasi dicatat terpisah. Pasangan bukti tidak menyatakan persetujuan mutu desain.")
		if st.err != "" {
			pageNotice(c, "alert", "Permintaan belum selesai", st.err)
		}
		if st.loading {
			muted(c, "Memuat inbox dan kalender…")
		}
		humas := contentDivisionCapUI(*sp, "humas", api.CapTasksManage)
		design := contentDivisionCapUI(*sp, "mediatek", api.CapTasksManage)
		if humas || design {
			pageSection(c, "Pemrosesan pesan", "Aktifkan hanya format dan sumber yang sudah diperiksa. Pesan lama tidak dikirim ulang otomatis.", func() {
				if humas {
					ui.TextArea(c, &st.example).Label("Contoh broadcast untuk pratinjau").FillWidth().Height(140)
					if spaceButton(c, "Lihat hasil parser", "search", false).Disabled(disabled || cl == nil || strings.TrimSpace(st.example) == "").Clicked() {
						gen, seq, sid, example := a.gen, st.seq, st.space, st.example
						st.busy = true
						a.async(func() func() {
							items, err := cl.PreviewWABroadcast(context.Background(), sid, example)
							return func() {
								if !a.waContentCurrent(gen, seq) {
									return
								}
								st := &a.waContent
								st.busy = false
								if err != nil {
									st.err = a.failed(err)
									return
								}
								st.preview = items
								if len(items) == 0 {
									st.err = "Format belum dikenali. Periksa daftar bernomor serta label PJ Humas dan tanggal/DL."
								}
							}
						})
					}
					for index, item := range st.preview {
						ui.Column(c.Key(index)).FillWidth().Gap(3).Children(func() {
							ui.Text(c, item.Title+" · "+item.DateText).Bold()
							muted(c, "PJ Humas: "+item.Humas+" · PJ desain: "+item.Design)
							if item.Issue != "" {
								muted(c, item.Issue)
							}
						})
					}
					if spaceButton(c, "Saya sudah memeriksa format contoh", "check", st.policy.TemplateConfirmed).Disabled(disabled).Clicked() {
						st.policy.TemplateConfirmed = !st.policy.TemplateConfirmed
						if !st.policy.TemplateConfirmed {
							st.policy.BroadcastEnabled = false
						}
					}
					if spaceButton(c, "Parser broadcast: "+waYesNo(st.policy.BroadcastEnabled), "terminal", st.policy.BroadcastEnabled).Disabled(disabled || !st.policy.TemplateConfirmed).Clicked() {
						st.policy.BroadcastEnabled = !st.policy.BroadcastEnabled
					}
				}
				if design {
					if spaceButton(c, "Pencocokan Drive: "+waYesNo(st.policy.DriveEnabled), "link", st.policy.DriveEnabled).Disabled(disabled).Clicked() {
						st.policy.DriveEnabled = !st.policy.DriveEnabled
					}
				}
				if spaceButton(c, "Simpan kebijakan parser", "check", false).Disabled(disabled || cl == nil).Clicked() {
					req := st.policy
					gen, seq, sid := a.gen, st.seq, st.space
					st.busy = true
					a.async(func() func() {
						_, err := cl.SetWAParserPolicy(context.Background(), sid, req)
						return func() {
							if !a.waContentCurrent(gen, seq) {
								return
							}
							a.waContent.busy = false
							if err != nil {
								a.waContent.err = a.failed(err)
								return
							}
							a.loadWAContent()
						}
					})
				}
			})
		}
		if st.selected != nil {
			a.waContentDetail(c, *sp, disabled)
		}
		visible := 0
		for _, item := range st.items {
			if !st.showAll && item.State != "pending" {
				continue
			}
			visible++
			item := item
			ui.Column(c.Key(item.ID.String())).FillWidth().Padding(14).Gap(6).Border(1, c.Theme().Border).Radius(10).Children(func() {
				ui.Text(c, waContentItemTitle(item)+" · "+item.State).Bold()
				name := item.SenderName
				if name == "" {
					name = "Identitas belum tertaut"
				}
				muted(c, name+" · kejadian "+taskTimestamp(item.SourceAt, a.period().TimeZone))
				if item.Issue != "" {
					muted(c, item.Issue)
				}
				if spaceButton(c, "Periksa sumber dan pasangan", "search", false).Disabled(disabled).Clicked() {
					a.selectWAContent(item)
				}
			})
		}
		if visible == 0 && !st.loading {
			muted(c, "Tidak ada item pada filter ini. Inbox menampilkan sampai 200 item terbaru dengan antrean tinjauan diutamakan.")
		}
	})
}

func contentDivisionCapUI(view api.Space, slug, cap string) bool {
	if view.ReadOnly {
		return false
	}
	var did id.ID
	for _, d := range view.Divisions {
		if d.Slug == slug {
			did = d.ID
		}
	}
	return slices.ContainsFunc(view.Roles, func(role api.Access) bool {
		return role.DivisionID == did && slices.Contains(role.Capabilities, cap) && !role.ReadOnly
	})
}
func waContentItemTitle(item api.WAContentItem) string {
	if item.Kind == "concept" {
		var parsed broadcast.Item
		if json.Unmarshal(item.Parsed, &parsed) == nil && parsed.Title != "" {
			return "Konsep: " + parsed.Title
		}
	}
	if item.Kind == "drive" {
		return "Bukti Drive"
	}
	if item.Kind == "correction" {
		return "Perubahan pesan sumber"
	}
	return "Item Medhum"
}
func (a *App) waContentDetail(c *ui.Context, sp api.Space, disabled bool) {
	st := &a.waContent
	item := st.selected
	pageSection(c, "Tinjauan: "+waContentItemTitle(*item), "Waktu kejadian asli tetap dipertahankan saat pasangan dikoreksi.", func() {
		ui.Text(c, item.Body)
		muted(c, "Diterima Space: "+taskTimestamp(item.ReceivedAt, a.period().TimeZone))
		if item.MatchedAt != nil {
			muted(c, "Dipasangkan: "+taskTimestamp(*item.MatchedAt, a.period().TimeZone))
		}
		if item.ContentID != nil {
			for _, content := range st.contents {
				if content.ID == *item.ContentID && content.DesignTaskID != nil {
					if spaceButton(c, "Buka tugas desain / tinjau kecocokan bukti", "task", false).Disabled(disabled).Clicked() {
						a.OpenNotification(api.Delivery{SourceKind: "task", SourceID: *content.DesignTaskID, SpaceID: &content.SpaceID})
					}
				}
			}
		}
		if item.ReviewedAt != nil {
			muted(c, "Ditinjau: "+taskTimestamp(*item.ReviewedAt, a.period().TimeZone))
		}
		if item.Issue != "" {
			pageNotice(c, "info", "Perlu diperhatikan", item.Issue)
		}
		authority := contentDivisionCapUI(sp, "humas", api.CapTasksManage)
		if item.Kind == "drive" {
			authority = contentDivisionCapUI(sp, "mediatek", api.CapResultsReview)
		}
		if item.Kind == "correction" {
			authority = authority || contentDivisionCapUI(sp, "mediatek", api.CapResultsReview)
		}
		if authority {
			if item.Kind != "correction" {
				ui.TextInput(c, &st.filter).Label("Cari konten tujuan").Placeholder("Cari konten tujuan").FillWidth()
				for _, content := range st.contents {
					if st.filter != "" && !strings.Contains(strings.ToLower(content.Title), strings.ToLower(st.filter)) {
						continue
					}
					content := content
					label := content.Title + " · " + taskDate(content.PostingDate, content.PostingAt, a.period().TimeZone)
					if slices.Contains(item.Candidates, content.ID) {
						label += " · kandidat"
					}
					if spaceButton(c.Key(content.ID.String()), label, "calendar", st.target == content.ID).Disabled(disabled).Clicked() {
						st.target = content.ID
					}
				}
			}
			if item.Kind == "drive" {
				if spaceButton(c, "Saya sudah membuka dan memeriksa akses tautan", "link", st.accessConfirmed).Disabled(disabled).Clicked() {
					st.accessConfirmed = !st.accessConfirmed
				}
			}
			ui.TextArea(c, &st.reason).Label("Alasan keputusan / ralat").FillWidth().Height(76)
			ui.Row(c).Wrap().Gap(8).Children(func() {
				if item.Kind == "correction" {
					if spaceButton(c, "Catat tinjauan; pertahankan bukti", "check", false).Disabled(disabled || strings.TrimSpace(st.reason) == "").Clicked() {
						a.decideWAContent("acknowledge")
					}
				} else {
					if slices.Contains([]string{"pending", "ignored", "retracted"}, item.State) {
						if spaceButton(c, "Tetapkan pasangan", "check", true).Disabled(disabled || st.target == id.Nil || strings.TrimSpace(st.reason) == "").Clicked() {
							a.decideWAContent("apply")
						}
					}
					if item.ContentID != nil && st.target != id.Nil && *item.ContentID != st.target {
						if spaceButton(c, "Ralat ke konten terpilih", "edit", false).Disabled(disabled || strings.TrimSpace(st.reason) == "").Clicked() {
							a.decideWAContent("reassign")
						}
					}
					if item.ContentID != nil && slices.Contains([]string{"applied", "corrected"}, item.State) {
						if spaceButton(c, "Tarik penerapan bukti ini", "close", false).Disabled(disabled || strings.TrimSpace(st.reason) == "").Clicked() {
							a.decideWAContent("retract")
						}
					}
				}
				if item.State == "pending" {
					if spaceButton(c, "Abaikan dengan alasan", "close", false).Disabled(disabled || strings.TrimSpace(st.reason) == "").Clicked() {
						a.decideWAContent("ignore")
					}
				}
			})
		}
		for _, review := range st.history {
			ui.Text(c, review.ActorName+" · "+review.Action)
			muted(c, taskTimestamp(review.OccurredAt, a.period().TimeZone)+" · "+review.Reason)
		}
		if spaceButton(c, "Tutup tinjauan", "close", false).Disabled(disabled).Clicked() {
			st.selected = nil
			st.history = nil
			st.selectionSeq++
		}
	})
}
