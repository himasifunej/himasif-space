package appui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Notes "gaya C" (owner decision after pilot 0.3.5; canvas 37348413b3ef,
// designs 5 and 6): the Space shell stays, the document is wide and plain,
// and everything that sat under it moves to one panel on the right with the
// tabs Rapat / Komentar / Riwayat (a sheet with the same tabs on a phone).
// A thin bar on top carries the editor lock, the save state, a draft chip
// (only when there is a draft) and a "⋯" menu.

const (
	notePanelRapat    = "rapat"
	notePanelKomentar = "komentar"
	notePanelRiwayat  = "riwayat"
	notePanelWidth    = 320
	noteDocWidth      = 760
)

// notePanelShown: the panel opens by default on a desktop, stays closed on
// a tablet until asked for; the person's choice wins.
func (a *App) notePanelShown(c *ui.Context) bool {
	switch a.notes.panelPref {
	case 1:
		return true
	case 2:
		return false
	}
	return viewportMode(c) == layoutDesktop
}

func (a *App) openNotePanel(c *ui.Context, tab string) {
	a.notes.panelTab = tab
	if isMobile(c) {
		a.notes.sheetOpen = true
		return
	}
	a.notes.panelPref = 1
}

func (a *App) notePanelTab() string {
	if a.notes.panelTab == "" {
		return notePanelRapat
	}
	return a.notes.panelTab
}

// noteHasDraft: a device or server recovery draft exists for this note.
func (a *App) noteHasDraft() bool {
	return a.notes.localRecovery != nil || len(a.notes.recovery) > 0
}

// noteDetail: the thin bar, then the document and (desktop) the panel.
func (a *App) noteDetail(c *ui.Context) {
	if a.notes.selected == id.Nil || a.notes.note == nil {
		ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
			ui.Column(c).FillWidth().MinWidth(0).Padding(22).Gap(14).Children(func() {
				if a.notes.detailLoading {
					pageEmpty(c, "note", "Memuat isi catatan…", "Mengambil isi resmi dan draf pemulihan milikmu.")
				} else if a.notes.detailErr != "" {
					pageNotice(c, "alert", "Isi catatan belum dapat dimuat", a.notes.detailErr)
					if spaceButton(c, "Muat ulang isi catatan", "refresh", false).Disabled(a.notes.busy).Clicked() && a.notes.selected != id.Nil {
						a.openNote(a.notes.selected)
					}
				} else {
					pageEmpty(c, "note", "Pilih catatan", "Baca isi, lihat komentar, atau ambil editor untuk mencatat bersama.")
				}
			})
		})
		return
	}
	phone := isMobile(c)
	c.OnShortcut(ui.Ctrl, ui.KeyS, func() {
		if a.notesCanEdit() && a.notes.dirty {
			a.saveNote()
		}
	})
	a.noteUndoShortcuts(c)
	a.noteDocBar(c, phone)
	ui.Row(c.Key("note-body")).Grow(1).MinHeight(0).FillWidth().MinWidth(0).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).MinHeight(0).Children(func() {
			a.noteTimerPill(c, phone)
			a.noteDocumentScroll(c, phone)
			if a.noteEditing() && phone && a.notes.editor != nil {
				ui.Column(c).FillWidth().BorderWidth(1, 0, 0, 0).BorderColor(c.Theme().Border).Background(c.Theme().Surface).Children(func() { a.notes.editor.MobileBar(c) })
			}
		})
		if !phone && a.notePanelShown(c) {
			ui.Column(c.Key("note-panel")).Width(notePanelWidth).Shrink(0).MinHeight(0).BorderWidth(0, 0, 0, 1).BorderColor(c.Theme().Border).Background(c.Theme().Surface).Children(func() {
				a.notePanel(c, false)
			})
		}
	})
	if phone && a.notes.sheetOpen {
		a.noteSheet(c)
	}
}

// noteDocumentScroll is the plain document: title, chips, notices that
// need attention, the draft review when one is open, and the blocks.
func (a *App) noteDocumentScroll(c *ui.Context, phone bool) {
	n := a.notes.note
	ui.Scroll(c.Key(n.ID)).Grow(1).MinHeight(0).FillWidth().Label("Area isi catatan").Children(func() {
		pad := [2]float32{32, 40}
		if phone {
			pad = [2]float32{16, 16}
		}
		ui.Column(c).FillWidth().MinWidth(0).AlignItems(ui.Center).Padding(pad[0], pad[1]).Children(func() {
			ui.Column(c.Key("note-document")).FillWidth().MaxWidth(noteDocWidth).MinWidth(0).Gap(14).Children(func() {
				ui.Column(c).FillWidth().MinWidth(0).Gap(10).Children(func() {
					a.noteTitleView(c, phone)
					a.noteChips(c)
				})
				if a.notes.note == nil { // closed earlier in this frame
					return
				}
				if a.notes.detailErr != "" {
					pageNotice(c, "alert", "Detail belum diperbarui", a.notes.detailErr)
				}
				if a.notes.saveErr != "" {
					pageNotice(c, "alert", "Catatan belum tersimpan", a.notes.saveErr+" Ketikanmu aman di perangkat ini; simpan otomatis mencoba lagi.")
				}
				if a.notes.localErr != "" {
					pageNotice(c, "alert", "Draf perangkat belum tersimpan", a.notes.localErr)
				}
				if a.notes.draftErr != "" {
					pageNotice(c, "alert", "Draf pemulihan belum dapat diperbarui", a.notes.draftErr)
				}
				if a.notes.compare != nil {
					a.noteComparisonView(c)
				}
				if e := a.notes.editor; e != nil {
					e.SetEditable(a.noteEditing())
					e.OnExtract = nil
					e.ExtractTask = a.noteCanExtractTask()
					e.Compact = phone
					e.Attendance = a.noteAttendanceLines
					if !a.notesWriteBlocked() && a.notes.compare == nil && !a.isAdmin() {
						e.OnExtract = func(text, target string) {
							a.beginNoteConversion(target, text)
							// The form opens in the panel, next to the text.
							a.openNotePanel(c, a.notePanelTab())
						}
					}
					a.wireNoteEditor(c, e)
					e.View(c)
					if e.TakeChangedAt(c.Now()) {
						a.markNoteChanged()
					}
				}
			})
		})
	})
}

// noteChips: the meeting at a glance under the title; each opens Rapat.
func (a *App) noteChips(c *ui.Context) {
	n := a.notes.note
	if n == nil {
		return
	}
	t := c.Theme()
	chip := func(key, icon, label string, accent bool) ui.Element {
		b := ui.ButtonBase(c.Key(key)).Label(label).Row().Height(26).Padding(0, 10).Gap(6).AlignItems(ui.Center).Radius(13).Border(1, t.Border).TextColor(t.TextMuted)
		if accent {
			b.Background(accentSoft(t)).BorderColor(spaceMix(t.Accent, t.Border, .5)).TextColor(t.Accent)
		}
		if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
		b.FocusRing(b.FocusVisible()).Children(func() {
			ui.Icon(c, spaceIcon(icon)).Size(13, 13)
			ui.Text(c, label).FontSize(12).MaxLines(1)
		})
		return b
	}
	ui.Row(c.Key("note-chips")).FillWidth().MinWidth(0).Wrap().Gap(6).Children(func() {
		m, loading := a.noteMeeting()
		switch {
		case m != nil:
			if chip("chip-when", "calendar", noteMeetingWhen(m, true), true).Clicked() {
				a.openNotePanel(c, notePanelRapat)
			}
			if parts := meetingAttendance(m).parts(); len(parts) > 0 {
				if chip("chip-att", "users", strings.Join(parts, " · "), false).Clicked() {
					a.openNotePanel(c, notePanelRapat)
				}
			}
		case loading:
			muted(c, "Memuat data rapat…").FontSize(12)
		default:
			ct := n.CreatedAt.In(noteZone(a.noteViewZone()))
			if chip("chip-created", "calendar", fmt.Sprintf("Dibuat %d %s", ct.Day(), noteMonths[ct.Month()-1]), false).Clicked() {
				a.openNotePanel(c, notePanelRapat)
			}
		}
		if k := noteKindTag(n.TemplateKind); k != "" {
			chip("chip-kind", "note", k, false)
		}
		if len(n.Comments) > 0 && chip("chip-comments", "chat", fmt.Sprintf("%d komentar", len(n.Comments)), false).Clicked() {
			a.openNotePanel(c, notePanelKomentar)
		}
	})
}

// noteDocBar is the thin bar: where the note is, the draft chip, the lock,
// the save state, the panel toggle and "⋯".
func (a *App) noteDocBar(c *ui.Context, phone bool) {
	t := c.Theme()
	n := a.notes.note
	now := c.Now()
	bar := ui.Row(c.Key("note-docbar")).FillWidth().MinWidth(0).Height(44).Padding(0, 10, 0, 14).Gap(8).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border)
	bar.Children(func() {
		if phone {
			if spaceIconButton(c, "Kembali ke daftar catatan", "left").Disabled(a.notes.busy).Clicked() {
				a.closeNote()
			}
		} else {
			ui.Row(c.Key("note-crumbs")).Grow(1).Shrink(1).MinWidth(0).Gap(6).AlignItems(ui.Center).Children(func() {
				if n.MeetingID != nil {
					b := ui.ButtonBase(c.Key("crumb-meeting")).Label("Buka rapat terkait").Tooltip("Buka rapat di Agenda").Row().Gap(6).AlignItems(ui.Center).Padding(3, 6).Radius(6).TextColor(t.TextMuted).Shrink(0)
					if b.Hovered() {
						b.Background(t.SurfaceHover).TextColor(t.Text)
					}
					b.FocusRing(b.FocusVisible()).Children(func() {
						ui.Icon(c, spaceIcon("calendar")).Size(14, 14)
						ui.Text(c, "Rapat").FontSize(13)
					})
					if b.Clicked() {
						a.openNoteMeeting()
					}
				} else {
					ui.Text(c, "Catatan").FontSize(13).TextColor(t.TextMuted).Shrink(0)
				}
				ui.Text(c, "/").FontSize(13).TextColor(faint(t)).Shrink(0)
				ui.Text(c, n.Title).FontSize(13).FontWeight(500).MaxLines(1).MinWidth(0).Shrink(1)
			})
		}
		if phone {
			ui.Box(c).Grow(1)
		}
		if a.noteHasDraft() {
			d := ui.ButtonBase(c.Key("note-draft-chip")).Label("Ada draf pemulihan").Tooltip("Ada draf yang belum menjadi isi resmi. Buka Riwayat untuk meninjau atau membuangnya.").Row().Height(24).Padding(0, 8).Gap(5).AlignItems(ui.Center).Radius(12).Background(spaceMix(t.Warning, t.Surface, .85)).TextColor(t.Warning).Shrink(0)
			d.FocusRing(d.FocusVisible()).Children(func() {
				ui.Icon(c, spaceIcon("alert")).Size(12, 12)
				if !phone {
					ui.Text(c, "Draf").FontSize(12).FontWeight(500)
				}
			})
			if d.Clicked() {
				a.openNotePanel(c, notePanelRiwayat)
			}
		}
		if a.notesCanEdit() && a.notes.compare == nil {
			a.noteModeSwitch(c, phone)
		}
		if !a.notesCanEdit() {
			a.notes.reading = false // the next time the editor is taken, it opens in Edit
		}
		a.noteLockChip(c, phone)
		label, icon, warn := a.noteSaveState(now)
		color := faint(t)
		if warn {
			color = t.Warning
		}
		ui.Row(c.Key("note-save-state")).Gap(5).AlignItems(ui.Center).MinWidth(0).Shrink(1).Tooltip(fmt.Sprintf("Isi resmi versi %d · Ctrl+S menyimpan seketika", n.Version)).Children(func() {
			ui.Icon(c, spaceIcon(icon)).Size(13, 13).Shrink(0).TextColor(color)
			if !phone || warn {
				ui.Text(c, label).FontSize(12).TextColor(color).MaxLines(1).MinWidth(0)
			}
		})
		if !phone && a.noteEditing() {
			a.noteUndoButtons(c)
		}
		if a.notes.saveErr != "" && a.notesCanEdit() && !a.notes.saving {
			if outlineButton(c.Key("note-retry"), "Coba simpan lagi", "refresh", false).Height(28).Clicked() {
				a.commitNote(false)
			}
		}
		if phone {
			b := spaceIconButton(c.Key("note-sheet"), "Rapat, komentar, riwayat", "panel")
			if b.Clicked() {
				a.openNotePanel(c, a.notePanelTab())
			}
		} else {
			shown := a.notePanelShown(c)
			label := "Tutup panel catatan"
			if !shown {
				label = "Buka panel catatan"
			}
			if spaceIconButton(c.Key("note-panel-toggle"), label, "panel").Clicked() {
				if shown {
					a.notes.panelPref = 2
				} else {
					a.notes.panelPref = 1
				}
			}
		}
		a.noteMoreMenu(c)
		if !phone && spaceIconButton(c.Key("note-close"), "Tutup catatan", "close").Disabled(a.notes.busy).Clicked() {
			a.closeNote()
		}
	})
	if a.notes.note != nil && a.notes.note.Permissions.Manage && a.notes.takeoverConfirm && !a.notesCanEdit() {
		ui.Column(c.Key("note-takeover")).FillWidth().Padding(10, 14).Gap(8).Background(spaceMix(t.Warning, t.Surface, .9)).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, "Ambil alih pencatatan?").FontSize(13).Bold()
			muted(c, "Draf pencatat sebelumnya tetap disimpan. Perangkatnya kehilangan hak memperbarui isi resmi sampai mengambil editor kembali.").FontSize(12)
			ui.Row(c).Gap(8).Wrap().Children(func() {
				if outlineButton(c.Key("lease-confirm"), "Konfirmasi ambil alih", "check", true).Disabled(a.notesWriteBlocked()).Clicked() {
					a.takeoverNoteEditing()
				}
				if outlineButton(c.Key("lease-cancel"), "Batal ambil alih", "close", false).Disabled(a.notes.busy).Clicked() {
					a.notes.takeoverConfirm = false
				}
			})
		})
	}
}

// noteLockChip: who holds the editor (a lock past its expiry holds nobody)
// and the one action that fits; the rules are those of the editor lock.
func (a *App) noteLockChip(c *ui.Context, phone bool) {
	t := c.Theme()
	n := a.notes.note
	if n == nil {
		return
	}
	live := n.Lock != nil && a.noteNow().Before(n.Lock.ExpiresAt)
	label, detail, who := "Belum ada pencatat aktif", "", ""
	switch {
	case live && n.Lock.HeldByCaller:
		label, who = "Kamu memegang editor", n.Lock.Editor.DisplayName
		detail = "sampai " + noteZoneClock(n.Lock.ExpiresAt, a.noteViewZone())
	case live:
		label, who = n.Lock.Editor.DisplayName+" sedang mencatat", n.Lock.Editor.DisplayName
		detail = "sampai " + noteZoneClock(n.Lock.ExpiresAt, a.noteViewZone())
	case n.Lock != nil && n.Lock.HeldByCaller:
		detail = "kuncimu berakhir " + noteZoneClock(n.Lock.ExpiresAt, a.noteViewZone())
	case n.Lock != nil:
		detail = "kunci " + n.Lock.Editor.DisplayName + " berakhir " + noteZoneClock(n.Lock.ExpiresAt, a.noteViewZone())
	}
	if !n.Permissions.Manage {
		detail = strings.TrimPrefix(detail+" · hanya baca", " · ")
	}
	chip := ui.Row(c.Key("note-lock")).Height(26).Padding(0, 8, 0, 3).Gap(6).AlignItems(ui.Center).Radius(13).Background(spaceMix(t.Surface, t.Background, .4)).Shrink(1).MinWidth(0).Tooltip(strings.TrimSpace(label + " " + detail))
	chip.Children(func() {
		if who != "" {
			avatar(c, who, 20)
		} else {
			ui.Box(c).Size(20, 20).Center().Children(func() { ui.Icon(c, spaceIcon("lock")).Size(12, 12).TextColor(faint(t)) })
		}
		if !phone {
			ui.Text(c, label).FontSize(12).TextColor(t.TextMuted).MaxLines(1).MinWidth(0)
			if detail != "" {
				ui.Text(c, detail).FontSize(11).TextColor(faint(t)).MaxLines(1)
			}
		}
	})
	if !n.Permissions.Manage {
		return
	}
	small := func(key, label, icon string, primary bool) ui.Element {
		return outlineButton(c.Key(key), label, icon, primary).Height(28)
	}
	switch {
	case a.notesCanEdit():
		if small("lease-finish", "Selesai mencatat", "check", false).Tooltip("Perubahan terakhir disimpan dulu sebelum editor dilepas.").Disabled(a.notes.busy).Clicked() {
			a.finishNoteEditing()
		}
	case a.canEditOfflineNote():
		if small("lease-offline", "Edit draf offline", "edit", true).Tooltip("Editor server diperiksa ulang saat sinkronisasi; draf offline tidak mengambil alih editor orang lain.").Disabled(a.notes.busy).Clicked() {
			a.beginNoteEditing()
		}
	case !live || n.Lock.HeldByCaller:
		if small("lease-take", "Ambil editor", "edit", true).Disabled(a.notesWriteBlocked()).Clicked() {
			a.beginNoteEditing()
		}
	case !a.notes.takeoverConfirm:
		if small("lease-takeover", "Ambil alih editor", "edit", false).Disabled(a.notesWriteBlocked()).Clicked() {
			a.notes.takeoverConfirm = true
		}
	}
}

// noteMoreMenu: what is rarely needed while writing.
func (a *App) noteMoreMenu(c *ui.Context) {
	n := a.notes.note
	if n == nil || (!a.notesCanCreate() && !n.Permissions.Manage) {
		return
	}
	b := spaceIconButton(c.Key("note-more"), "Lainnya untuk catatan ini", "more")
	if b.Clicked() {
		a.notes.moreOpen = !a.notes.moreOpen
	}
	popover(c, b, &a.notes.moreOpen, 320, func() {
		ui.Scroll(c).FillWidth().MaxHeight(380).Children(func() {
			ui.Column(c).FillWidth().MinWidth(0).Gap(10).Children(func() {
				a.noteTemplatesView(c)
				if a.notes.note != nil && a.notes.note.Permissions.Manage {
					a.noteDeleteView(c)
				}
			})
		})
	})
}

// notePanel is the right panel (or the phone's sheet): the tabs, then the
// conversion form when one is open, then the tab's content.
func (a *App) notePanel(c *ui.Context, sheet bool) {
	t := c.Theme()
	n := a.notes.note
	if n == nil {
		return
	}
	tab := a.notePanelTab()
	ui.Row(c.Key("note-panel-tabs")).FillWidth().MinWidth(0).Height(44).Padding(0, 8).Gap(2).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		tabBtn := func(key, label, icon string, count int, warn bool) {
			on := tab == key
			b := ui.ButtonBase(c.Key("tab-"+key)).Label(label).Row().Height(30).Padding(0, 8).Gap(5).AlignItems(ui.Center).Radius(7).TextColor(t.TextMuted)
			if on {
				b.Background(spaceMix(t.Surface, t.Background, .3)).TextColor(t.Text).Border(1, t.Border)
			} else if b.Hovered() {
				b.Background(t.SurfaceHover)
			}
			b.FocusRing(b.FocusVisible()).Children(func() {
				ui.Icon(c, spaceIcon(icon)).Size(14, 14)
				ui.Text(c, label).FontSize(12).MaxLines(1)
				if count > 0 {
					bg := t.Accent
					if warn {
						bg = t.Warning
					}
					ui.Box(c).Height(16).Padding(0, 5).Radius(8).Center().Background(bg).Children(func() {
						ui.Text(c, fmt.Sprint(count)).FontSize(10).Bold().TextColor(t.AccentText)
					})
				}
			})
			if b.Clicked() {
				a.notes.panelTab = key
			}
		}
		drafts := len(a.notes.recovery)
		if a.notes.localRecovery != nil {
			drafts++
		}
		tabBtn(notePanelRapat, "Rapat", "calendar", 0, false)
		tabBtn(notePanelKomentar, "Komentar", "chat", len(n.Comments), false)
		tabBtn(notePanelRiwayat, "Riwayat", "clock", drafts, true)
		ui.Box(c).Grow(1)
		label := "Tutup panel catatan"
		if sheet {
			label = "Tutup lembar catatan"
		}
		if spaceIconButton(c.Key("panel-close"), label, "close").Clicked() {
			if sheet {
				a.notes.sheetOpen = false
			} else {
				a.notes.panelPref = 2
			}
		}
	})
	ui.Scroll(c.Key("panel-" + tab)).Grow(1).MinHeight(0).FillWidth().Label("Isi panel catatan").Children(func() {
		ui.Column(c).FillWidth().MinWidth(0).Padding(4, 0, 16, 0).Children(func() {
			if a.notes.conversion != nil {
				ui.Column(c.Key("panel-conversion")).FillWidth().MinWidth(0).Padding(10, 12).Children(func() { a.noteConversionView(c) })
			}
			switch tab {
			case notePanelKomentar:
				a.notePanelComments(c)
			case notePanelRiwayat:
				a.notePanelHistory(c)
			default:
				a.notePanelMeeting(c)
			}
		})
	})
}

// noteSheet is the panel as a bottom sheet over the phone's document.
func (a *App) noteSheet(c *ui.Context) {
	t := c.Theme()
	w, h := c.Size()
	ui.Overlay(c, func() {
		scrim := ui.ButtonBase(c.Key("note-sheet-scrim")).Label("Latar lembar catatan").Absolute().Left(0).Top(0).Width(w).Height(h).Background(ui.RGBA(0, 0, 0, .28))
		if scrim.Clicked() {
			a.notes.sheetOpen = false
		}
		sh := h * .72
		ui.Column(c.Key("note-sheet")).Absolute().Left(0).Top(h-sh).Width(w).Height(sh).Background(t.Surface).Radius(16).Shadow(0, -8, 24, 0, ui.RGBA(0, 0, 0, .2)).Children(func() {
			ui.Box(c).Width(36).Height(4).Radius(2).Background(t.Border).Margin(8, 0, 2, 0).AlignSelf(ui.Center)
			a.notePanel(c, true)
		})
	})
}

func notePanelSection(c *ui.Context, key, title string, fn func()) {
	t := c.Theme()
	ui.Column(c.Key(key)).FillWidth().MinWidth(0).Gap(6).Padding(12, 14).BorderWidth(0, 0, 1, 0).BorderColor(spaceMix(t.Border, t.Surface, .4)).Children(func() {
		if title != "" {
			ui.Text(c, strings.ToUpper(title)).FontSize(11).TextColor(faint(t))
		}
		fn()
	})
}

func notePanelRow(c *ui.Context, key, icon, text string) {
	t := c.Theme()
	ui.Row(c.Key(key)).FillWidth().MinWidth(0).MinHeight(26).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, spaceIcon(icon)).Size(13, 13).TextColor(faint(t)).Shrink(0)
		ui.Text(c, text).FontSize(13).FillWidth().MinWidth(0)
	})
}

// notePanelMeeting is the Rapat tab: the meeting, attendance per person,
// "Di halaman ini" and the attachments.
func (a *App) notePanelMeeting(c *ui.Context) {
	t := c.Theme()
	n := a.notes.note
	m, loading := a.noteMeeting()
	notePanelSection(c, "sec-rapat", "Rapat", func() {
		switch {
		case m != nil:
			notePanelRow(c, "when", "calendar", noteMeetingWhen(m, false))
			if place := noteMeetingPlace(m); place != "" {
				notePanelRow(c, "where", "globe", place)
			}
			if b := ui.ButtonBase(c.Key("open-agenda")).Label("Buka di Agenda").Row().Justify(ui.Start).Gap(6).AlignItems(ui.Center).Padding(4, 0).TextColor(t.Accent); b.Clicked() {
				a.openNoteMeeting()
			} else {
				b.FocusRing(b.FocusVisible()).Children(func() {
					ui.Text(c, "Buka di Agenda").FontSize(12)
					ui.Icon(c, spaceIcon("chevron")).Size(12, 12)
				})
			}
		case loading:
			muted(c, "Memuat data rapat…").FontSize(12)
		case n.MeetingID != nil:
			muted(c, "Data rapat tidak tersedia untuk akunmu.").FontSize(12)
		default:
			ct := n.CreatedAt.In(noteZone(a.noteViewZone()))
			by := ""
			if n.CreatedBy.DisplayName != "" {
				by = " oleh " + n.CreatedBy.DisplayName
			}
			notePanelRow(c, "created", "calendar", fmt.Sprintf("Dibuat %s, %d %s %d%s", longDays[ct.Weekday()], ct.Day(), noteMonths[ct.Month()-1], ct.Year(), by))
			muted(c, "Catatan ini tidak terhubung ke rapat.").FontSize(12)
		}
	})
	a.noteTimerSection(c)
	if m != nil {
		att := meetingAttendance(m)
		notePanelSection(c, "sec-hadir", "Kehadiran · "+strings.Join(att.parts(), " · "), func() {
			if len(m.Participants) == 0 {
				muted(c, "Belum ada peserta.").FontSize(12)
			}
			for _, p := range m.Participants {
				status := map[string]string{"present": "hadir", "excused": "izin", "absent": "tidak hadir"}[p.AttendanceStatus]
				color := t.Success
				switch p.AttendanceStatus {
				case "present":
				case "excused":
					color = t.Warning
				case "absent":
					color = t.Danger
				default:
					status, color = "belum dicatat", faint(t)
				}
				if note := strings.TrimSpace(p.AttendanceNote); note != "" {
					status += " · " + note
				}
				ui.Row(c.Key(p.UserID)).FillWidth().MinWidth(0).Height(28).Gap(8).AlignItems(ui.Center).Children(func() {
					avatar(c, p.DisplayName, 20)
					ui.Text(c, p.DisplayName).FontSize(13).MaxLines(1).Grow(1).MinWidth(0)
					ui.Text(c, status).FontSize(11).TextColor(color).MaxLines(1).Shrink(1)
				})
			}
		})
	}
	if e := a.notes.editor; e != nil {
		heads := e.Headings()
		notePanelSection(c, "sec-outline", "Di halaman ini", func() {
			if len(heads) == 0 {
				muted(c, "Judul bagian (# atau /judul) muncul di sini.").FontSize(12)
			}
			for _, h := range heads {
				b := ui.ButtonBase(c.Key(h.index)).Label("Ke "+h.text).Row().Justify(ui.Start).FillWidth().MinWidth(0).Height(26).Padding(0, 6, 0, float32(6+(h.level-1)*14)).Gap(4).AlignItems(ui.Center).Radius(5).TextColor(t.TextMuted)
				if b.Hovered() {
					b.Background(t.SurfaceHover).TextColor(t.Text)
				}
				b.FocusRing(b.FocusVisible()).Children(func() {
					size := float32(13)
					if h.level >= 3 {
						size = 12
					}
					ui.Text(c, h.text).FontSize(size).MaxLines(1).MinWidth(0)
				})
				if b.Clicked() {
					e.Reveal(h.index)
				}
			}
		})
	}
	notePanelSection(c, "sec-lampiran", fmt.Sprintf("Lampiran · %d", len(n.Attachments)), func() { a.notePanelAttachments(c) })
}

// notePanelAttachments: the list, and the form behind "Tambah lampiran".
func (a *App) notePanelAttachments(c *ui.Context) {
	t := c.Theme()
	n := a.notes.note
	a.noteFileActions(c)
	if len(n.Attachments) == 0 {
		muted(c, "Belum ada lampiran.").FontSize(12)
	}
	for _, at := range n.Attachments {
		ui.Row(c.Key(at.ID)).FillWidth().MinWidth(0).MinHeight(28).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, spaceIcon("link")).Size(13, 13).TextColor(faint(t)).Shrink(0)
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				switch {
				case at.Stored:
					if b := ui.ButtonBase(c.Key("dl")).Label("Unduh " + at.Name).TextColor(t.Accent).Disabled(a.notes.busy || a.offlineMode()); b.Clicked() {
						a.downloadNoteFile(at)
					} else {
						b.Children(func() { ui.Text(c, at.Name).FontSize(13).MaxLines(1) })
					}
				case at.Status == api.AttachmentReady && at.URL != "":
					noteDocumentRichText(c, []api.Span{{Text: at.Name, Link: at.URL}}, true)
				default:
					ui.Text(c, at.Name+" · pending").FontSize(13).MaxLines(1)
				}
			})
			if n.Permissions.Manage && spaceIconButton(c.Key("rm"), "Hapus lampiran "+at.Name, "close").Disabled(a.notesWriteBlocked()).Clicked() {
				a.removeNoteAttachment(at.ID)
			}
		})
	}
	if !n.Permissions.Manage {
		return
	}
	if !a.notes.attachOpen {
		if b := ui.ButtonBase(c.Key("attach-open")).Label("Tambah lampiran tautan").Row().Justify(ui.Start).Gap(6).AlignItems(ui.Center).Padding(4, 0).TextColor(t.TextMuted); b.Clicked() {
			a.notes.attachOpen = true
		} else {
			b.FocusRing(b.FocusVisible()).Children(func() {
				ui.Icon(c, spaceIcon("plus")).Size(12, 12)
				ui.Text(c, "Tambah lampiran tautan").FontSize(12)
			})
		}
		return
	}
	taskInput(c, "note-attachment-name", "Nama lampiran", &a.notes.attachmentName)
	taskInput(c, "note-attachment-url", "Tautan lampiran (opsional)", &a.notes.attachmentURL)
	ui.Row(c).Gap(6).Wrap().Children(func() {
		if outlineButton(c.Key("attach-add"), "Tambahkan lampiran", "link", true).Disabled(a.notesWriteBlocked() || strings.TrimSpace(a.notes.attachmentName) == "").Clicked() {
			a.addNoteAttachment()
			a.notes.attachOpen = false
		}
		if outlineButton(c.Key("attach-cancel"), "Batal", "close", false).Clicked() {
			a.notes.attachOpen = false
		}
	})
}

// notePanelComments is the Komentar tab: the thread and one input line.
func (a *App) notePanelComments(c *ui.Context) {
	t := c.Theme()
	n := a.notes.note
	if len(n.Comments) == 0 {
		ui.Column(c).FillWidth().Padding(16, 14).Children(func() { muted(c, "Belum ada komentar.").FontSize(12) })
	}
	for _, cm := range n.Comments {
		ui.Column(c.Key(cm.ID)).FillWidth().MinWidth(0).Gap(3).Padding(10, 14).BorderWidth(0, 0, 1, 0).BorderColor(spaceMix(t.Border, t.Surface, .4)).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				avatar(c, cm.Author.DisplayName, 18)
				ui.Text(c, cm.Author.DisplayName).FontSize(12).FontWeight(500).MaxLines(1)
				ui.Text(c, noteRelative(cm.CreatedAt, c.Now(), a.noteViewZone())).FontSize(11).TextColor(faint(t))
			})
			ui.Text(c, cm.Body).FontSize(13).FillWidth().MinWidth(0).Selectable()
		})
	}
	if !n.Permissions.Comment {
		return
	}
	ui.Row(c.Key("comment-box")).FillWidth().MinWidth(0).Margin(10, 12, 0, 12).Height(36).Padding(0, 4, 0, 10).Gap(6).AlignItems(ui.Center).Radius(8).Border(1, t.Border).Background(t.Background).Children(func() {
		in := ui.TextInput(c.Key("note-comment"), &a.notes.comment).Label("Komentar baru").Placeholder("Tulis komentar…").Grow(1).MinWidth(0).FontSize(13).Background(ui.RGBA(0, 0, 0, 0)).BorderWidth(0).Padding(0)
		if in.Submitted() && strings.TrimSpace(a.notes.comment) != "" && !a.notesWriteBlocked() {
			a.addNoteComment()
		}
		if spaceIconButton(c.Key("comment-send"), "Kirim komentar catatan", "send").Disabled(a.notesWriteBlocked() || strings.TrimSpace(a.notes.comment) == "").Clicked() {
			a.addNoteComment()
		}
	})
}

// notePanelHistory is the Riwayat tab: drafts first (rows, not cards),
// then the history and where tasks made from the note came from.
func (a *App) notePanelHistory(c *ui.Context) {
	t := c.Theme()
	n := a.notes.note
	if a.noteHasDraft() {
		notePanelSection(c, "sec-draf", "Draf pemulihan", func() {
			muted(c, "Belum menjadi isi resmi. Tinjau untuk membandingkan dengan versi terbaru.").FontSize(12)
			draft := func(key, title, detail string, stale bool, review func(), discard func()) {
				ui.Column(c.Key(key)).FillWidth().MinWidth(0).Gap(6).Padding(8, 10).Radius(8).Background(spaceMix(t.Warning, t.Surface, .9)).Children(func() {
					ui.Text(c, title).FontSize(13).FontWeight(500)
					muted(c, detail).FontSize(11)
					if stale {
						muted(c, "Isi resmi sudah berubah sejak draf ini.").FontSize(11)
					}
					ui.Row(c).Gap(6).Wrap().Children(func() {
						if outlineButton(c.Key("review"), "Tinjau "+strings.ToLower(title), "note", false).Height(28).Disabled(a.notes.busy || a.notes.detailLoading).Clicked() {
							review()
						}
						if discard != nil && outlineButton(c.Key("discard"), "Buang "+strings.ToLower(title), "close", false).Height(28).Disabled(a.notesWriteBlocked()).Clicked() {
							discard()
						}
					})
				})
			}
			if l := a.notes.localRecovery; l != nil {
				draft("local", "Draf perangkat", fmt.Sprintf("%s · dari versi %d", noteRelative(l.SavedAt, c.Now(), a.noteViewZone()), l.BaseVersion), false, func() { a.restoreNoteRecovery(0, true) }, nil)
			}
			for i, d := range a.notes.recovery {
				i, d := i, d
				title := fmt.Sprintf("Draf server %d", i+1)
				if d.CurrentSession {
					title = "Draf sesi ini"
				}
				draft(d.ID.String(), title, fmt.Sprintf("%s · dari versi %d", noteRelative(d.SavedAt, c.Now(), a.noteViewZone()), d.BaseVersion), d.Stale, func() { a.restoreNoteRecovery(i, false) }, func() { a.discardNoteRecovery(d.ID) })
			}
		})
	}
	if n.Permissions.Manage || n.CreatedBy.ID == a.user.ID || len(a.notes.history) > 0 {
		notePanelSection(c, "sec-riwayat", "Riwayat", func() {
			if a.notes.historyErr != "" {
				ui.Text(c, a.notes.historyErr).FontSize(12).TextColor(t.Danger)
				return
			}
			if len(a.notes.history) == 0 {
				muted(c, "Belum ada riwayat yang tersedia.").FontSize(12)
			}
			for _, ev := range groupNoteHistory(a.notes.history) {
				label := noteEventLabel(ev.Action)
				when := noteRelative(ev.CreatedAt, c.Now(), a.noteViewZone())
				if ev.count > 1 {
					label += fmt.Sprintf(" · %d kali disimpan", ev.count)
					when = noteZoneClock(ev.first, a.noteViewZone()) + "–" + noteZoneClock(ev.CreatedAt, a.noteViewZone())
				}
				ui.Row(c.Key(ev.ID)).FillWidth().MinWidth(0).MinHeight(30).Gap(8).AlignItems(ui.Center).Children(func() {
					avatar(c, ev.Actor.DisplayName, 18)
					ui.Column(c).Grow(1).MinWidth(0).Children(func() {
						ui.Text(c, label).FontSize(12).MaxLines(2)
						ui.Text(c, ev.Actor.DisplayName+" · "+when).FontSize(11).TextColor(faint(t)).MaxLines(1)
					})
				})
			}
		})
	}
	if len(a.notes.links) > 0 || a.notes.linksErr != "" {
		notePanelSection(c, "sec-asal", "Pekerjaan dari catatan ini", func() {
			if a.notes.linksErr != "" {
				ui.Text(c, a.notes.linksErr).FontSize(12).TextColor(t.Danger)
			}
			for _, l := range a.notes.links {
				label := "Tugas"
				if l.TargetType == api.NoteLinkReminder {
					label = "Reminder pribadi"
				}
				notePanelRow(c, l.ID.String(), "task", label+" · "+noteRelative(l.CreatedAt, c.Now(), a.noteViewZone()))
			}
		})
	}
}
