package appui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
)

func (a *App) notesView(c *ui.Context) {
	a.ensureNotes()
	phone := isMobile(c)
	if phone && a.notes.selected != id.Nil {
		// Phone: an open note fills the page; the bar has the way back.
		ui.Column(c.Key(a.gen)).Grow(1).MinHeight(0).FillWidth().MinWidth(0).Gap(0).Children(func() {
			a.notesFeedback(c)
			a.noteDetail(c)
		})
		return
	}
	ui.Column(c.Key(a.gen)).Grow(1).MinHeight(0).FillWidth().MinWidth(0).Gap(0).Children(func() {
		ui.Row(c).FillWidth().MinWidth(0).Wrap().Padding(12, 18).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Catatan bersama").FontSize(15).Bold().Grow(1).MinWidth(0)
			ui.TextInput(c.Key("notes-filter"), &a.notes.filter).Width(180).Label("Cari catatan").Placeholder("Cari judul atau pencatat…")
			if spaceIconButton(c, "Muat ulang catatan", "refresh").Disabled(a.notes.loading || a.notes.busy).Clicked() {
				a.loadNotes()
			}
			b := spaceButton(c, "Buat catatan", "plus", true).Disabled(!a.notesCanCreate() || a.notesWriteBlocked())
			if b.Clicked() {
				if a.notes.createOpen {
					a.notes.createOpen = false
				} else {
					a.beginNote()
				}
			}
			a.noteCreatePopover(c, b)
		})
		a.notesFeedback(c)
		if phone {
			ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Label("Daftar catatan").Children(func() {
				ui.Column(c).FillWidth().MinWidth(0).Padding(12, 16).Gap(9).Children(func() { a.notesList(c) })
			})
			return
		}
		ui.Row(c).Grow(1).MinHeight(0).FillWidth().MinWidth(0).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c).Width(220).Shrink(0).MinHeight(0).BorderWidth(0, 1, 0, 0).BorderColor(c.Theme().Border).Children(func() {
				ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Label("Daftar catatan").Children(func() {
					ui.Column(c).FillWidth().MinWidth(0).Padding(12).Gap(9).Children(func() { a.notesList(c) })
				})
			})
			ui.Column(c).Grow(1).MinWidth(350).MinHeight(0).Children(func() { a.noteDetail(c) })
		})
	})
}

func (a *App) notesFeedback(c *ui.Context) {
	if a.notes.err != "" {
		ui.Text(c, a.notes.err).FontSize(12).TextColor(c.Theme().Danger).FillWidth().MinWidth(0).Padding(8, 18)
	}
	if a.notes.notice != "" {
		ui.Row(c).FillWidth().MinWidth(0).Padding(8, 18).Gap(6).Children(func() {
			ui.Text(c, a.notes.notice).FontSize(12).TextColor(c.Theme().Accent).Grow(1).MinWidth(0)
			if spaceIconButton(c, "Tutup informasi catatan", "close").Clicked() {
				a.notes.notice = ""
			}
		})
	}
	if a.notes.busy {
		muted(c, "Mengirim perubahan ke server…").FontSize(11).Padding(6, 18)
	}
	if a.conn != connOnline {
		muted(c, "Tidak terhubung. Isian lokal dan isi resmi server memiliki status penyimpanan masing-masing.").FontSize(11).Padding(6, 18)
	}
}

func (a *App) notesList(c *ui.Context) {
	if !a.notes.loaded {
		title, detail := "Memuat catatan…", "Mengambil catatan dari ruang yang dapat kamu akses."
		if a.notes.err != "" {
			title, detail = "Catatan belum dapat dimuat", "Gunakan muat ulang untuk mencoba kembali."
		} else if a.selectedSpace() == nil {
			title, detail = "Pilih Space", "Catatan mengikuti ruang dan periode yang dipilih."
		}
		pageEmpty(c, "note", title, detail)
		return
	}
	q := strings.ToLower(strings.TrimSpace(a.notes.filter))
	count := 0
	for _, note := range a.notes.items {
		if q != "" && !strings.Contains(strings.ToLower(note.Title+" "+note.CreatedBy.DisplayName), q) {
			continue
		}
		count++
		b := ui.ButtonBase(c.Key(note.ID)).Label("Catatan "+note.Title).Column().AlignItems(ui.Start).FillWidth().MinWidth(0).Gap(7).Padding(12).Radius(9).Background(c.Theme().Surface).Border(1, c.Theme().Border).Disabled(a.notes.busy)
		if a.notes.selected == note.ID || b.Hovered() {
			b.Background(c.Theme().SurfaceHover)
		}
		b.FocusRing(b.FocusVisible()).Children(func() {
			ui.Text(c, note.Title).Bold().FontSize(13).MaxLines(2).FillWidth().MinWidth(0)
			muted(c, noteTemplateKindLabel(note.TemplateKind)).FontSize(10)
			if note.CreatedBy.DisplayName != "" {
				muted(c, note.CreatedBy.DisplayName).FontSize(10)
			}
			if !note.UpdatedAt.IsZero() {
				muted(c, taskTimestamp(note.UpdatedAt, a.noteViewZone())).FontSize(10)
			}
		})
		if b.Clicked() {
			a.openNote(note.ID)
		}
	}
	if count == 0 {
		title, detail := "Belum ada catatan", "Mulai dari template rapat, jadwal, atau catatan bebas."
		if q != "" {
			title, detail = "Catatan tidak ditemukan", "Coba judul atau nama pencatat lain."
		} else if isMobile(c) {
			// The phone has no detail pane: the guide shows in the list.
			a.notesEmptyGuide(c, a.notesCanCreate())
			return
		}
		pageEmpty(c, "note", title, detail)
	}
}

func (a *App) noteCreatePopover(c *ui.Context, anchor ui.Element) {
	popover(c, anchor, &a.notes.createOpen, 320, func() {
		ui.Text(c, "Catatan baru").Bold()
		taskInput(c, "new-note-title", "Judul catatan baru", &a.notes.newTitle)
		muted(c, "Pilih template awal").FontSize(12)
		ui.Scroll(c).FillWidth().MaxHeight(160).Children(func() {
			ui.Column(c).FillWidth().MinWidth(0).Gap(5).Children(func() {
				for _, template := range a.notes.templates {
					active := a.notes.templateID == template.ID
					if template.Builtin {
						active = a.notes.templateID == id.Nil && a.notes.templateKind == template.Kind
					}
					key := template.ID.String() + template.Kind
					if spaceButton(c.Key(key), template.Name, "note", active).FillWidth().Disabled(a.notes.busy).Clicked() {
						if template.Builtin {
							a.notes.templateID = id.Nil
						} else {
							a.notes.templateID = template.ID
						}
						a.notes.templateKind = template.Kind
					}
				}
			})
		})
		muted(c, "Isi template disalin saat dibuat. Perubahan template berikutnya tidak mengubah catatan ini.").FontSize(11)
		if spaceButton(c, "Buat catatan sekarang", "plus", true).FillWidth().Disabled(a.notesWriteBlocked() || !a.notesCanCreate() || strings.TrimSpace(a.notes.newTitle) == "").Clicked() {
			a.createNote()
		}
	})
}

type noteComparisonReaders struct{ official, recovered *noteDocumentEditor }

func (a *App) noteComparisonView(c *ui.Context) {
	n, draft := a.notes.note, a.notes.compare
	if n == nil || draft == nil {
		return
	}
	panel := ui.Column(c).FillWidth().MinWidth(0).Gap(12).Padding(14).Radius(9).Border(1, c.Theme().Warning)
	key := struct {
		draft   *LocalNoteDraft
		version int64
	}{draft, n.Version}
	readers := ui.Local(panel, key, func() noteComparisonReaders {
		return noteComparisonReaders{official: newNoteDocumentEditor(n.Content), recovered: newNoteDocumentEditor(draft.Content)}
	})
	panel.Children(func() {
		ui.Text(c, "Tinjau kedua versi").FontSize(15).Bold()
		muted(c, "Draf belum menjadi isi resmi. Periksa isi server dan drafmu sebelum melanjutkan; tidak ada isi yang digabung atau ditimpa otomatis.")
		pageSection(c, fmt.Sprintf("Isi resmi · versi %d", n.Version), n.Title, func() { readers.official.View(c) })
		pageSection(c, fmt.Sprintf("Draf · berawal dari versi %d", draft.BaseVersion), draft.Title, func() { readers.recovered.View(c) })
		if spaceButton(c, "Gunakan draf yang sudah ditinjau", "check", true).FillWidth().Disabled((a.notesWriteBlocked() && !a.canEditOfflineNote()) || !n.Permissions.Manage).Clicked() {
			a.useComparedNoteDraft()
		}
		muted(c, "Tindakan ini melanjutkan draf untuk diedit, belum menyimpannya sebagai isi resmi. Ambil editor terlebih dahulu bila diperlukan.").FontSize(11)
	})
}

func (a *App) noteTemplatesView(c *ui.Context) {
	if !a.notesCanCreate() {
		return
	}
	pageSection(c, "Template ruang", "Simpan isi catatan ini agar dapat digunakan sebagai awal catatan lain.", func() {
		taskInput(c, "note-template-name", "Nama template", &a.notes.templateName)
		if a.notes.templateEditing == id.Nil {
			if spaceButton(c, "Simpan sebagai template baru", "plus", false).FillWidth().Disabled(a.notesWriteBlocked() || strings.TrimSpace(a.notes.templateName) == "").Clicked() {
				a.saveNoteTemplate()
			}
		} else {
			muted(c, "Isi template terpilih akan diganti dengan isi catatan ini. Catatan yang sudah dibuat tidak berubah.").FontSize(11)
			if spaceButton(c, "Perbarui template terpilih", "check", true).FillWidth().Disabled(a.notesWriteBlocked() || strings.TrimSpace(a.notes.templateName) == "").Clicked() {
				a.updateNoteTemplate(a.notes.templateEditing)
			}
			if spaceButton(c, "Batal memperbarui template", "close", false).FillWidth().Disabled(a.notes.busy).Clicked() {
				a.notes.templateEditing, a.notes.templateName = id.Nil, ""
			}
		}
		for _, template := range a.notes.templates {
			if template.Builtin {
				continue
			}
			ui.Column(c.Key(template.ID)).FillWidth().MinWidth(0).Gap(7).Padding(10).Radius(8).Border(1, c.Theme().Border).Children(func() {
				ui.Text(c, template.Name).FontSize(12).Bold().FillWidth().MinWidth(0)
				if spaceButton(c, "Pilih template "+template.Name+" untuk diperbarui", "edit", a.notes.templateEditing == template.ID).FillWidth().Disabled(a.notesWriteBlocked()).Clicked() {
					a.notes.templateEditing, a.notes.templateName = template.ID, template.Name
				}
				if a.notes.templateDeleteConfirm != template.ID {
					if spaceButton(c, "Hapus template "+template.Name, "close", false).FillWidth().Disabled(a.notesWriteBlocked()).Clicked() {
						a.notes.templateDeleteConfirm = template.ID
					}
				} else {
					muted(c, "Hapus template tersimpan ini? Catatan yang pernah dibuat darinya tetap ada.").FontSize(11)
					if spaceButton(c, "Konfirmasi hapus template "+template.Name, "close", true).FillWidth().Disabled(a.notesWriteBlocked()).Clicked() {
						a.deleteNoteTemplate(template.ID)
					}
					if spaceButton(c, "Batal hapus template "+template.Name, "arrow", false).FillWidth().Disabled(a.notes.busy).Clicked() {
						a.notes.templateDeleteConfirm = id.Nil
					}
				}
			})
		}
	})
}

func (a *App) noteDeleteView(c *ui.Context) {
	if !a.notes.deleteConfirm {
		if spaceButton(c, "Hapus catatan", "close", false).FillWidth().Disabled(a.notesWriteBlocked()).Clicked() {
			a.notes.deleteConfirm = true
		}
		return
	}
	pageNotice(c, "alert", "Hapus catatan ini?", "Isi resmi, komentar, dan rujukan dalam catatan akan dihapus. Tugas atau reminder yang sudah dibuat tetap menjadi objek tersendiri.")
	if spaceButton(c, "Konfirmasi hapus catatan", "close", true).FillWidth().Disabled(a.notesWriteBlocked()).Clicked() {
		a.deleteNote()
	}
	if spaceButton(c, "Batal hapus catatan", "arrow", false).FillWidth().Disabled(a.notes.busy).Clicked() {
		a.notes.deleteConfirm = false
	}
}

func (a *App) noteConversionView(c *ui.Context) {
	conversion := a.notes.conversion
	if conversion == nil {
		return
	}
	task := conversion.kind == api.NoteLinkTask
	title := "Jadikan reminder pribadi"
	if task {
		title = "Jadikan tugas"
	}
	pageSection(c, title, "Periksa isian sebelum membuat. Hanya cuplikan yang kamu pilih digunakan sebagai bahan, bukan seluruh catatan.", func() {
		ui.Text(c, conversion.text).FontSize(12).FillWidth().MinWidth(0).Selectable()
		if conversion.err != "" {
			ui.Text(c, conversion.err).FontSize(12).TextColor(c.Theme().Danger).FillWidth().MinWidth(0)
		}
		if conversion.target != id.Nil && conversion.reader {
			what := "Reminder pribadi"
			if task {
				what = "Tugas"
			}
			pageNotice(c, "check", what+" sudah dibuat", "Tidak ditautkan ke catatan ini karena kamu pembaca catatan. Cuplikan blok disalin ke catatannya.")
			if spaceButton(c, "Selesai", "check", false).FillWidth().Clicked() {
				a.notes.conversion = nil
			}
			return
		}
		if conversion.target != id.Nil {
			pageNotice(c, "check", "Objek sudah dibuat", "Pembuatan tidak perlu diulang. Pencatatan hubungan ke catatan memiliki status tersendiri.")
			muted(c, "Referensi "+conversion.target.String()).FontSize(10).Selectable()
			if conversion.linkPending {
				if spaceButton(c, "Coba catat hubungan lagi", "link", true).FillWidth().Disabled(a.notesWriteBlocked()).Clicked() {
					a.retryNoteConversionLink()
				}
			}
			return
		}
		if conversion.uncertain {
			pageNotice(c, "alert", "Hasil pembuatan belum dapat dipastikan", "Periksa Board tugas atau Reminder pribadi terlebih dahulu. Jangan membuat ulang sebelum memastikan objek belum ada.")
			return
		}
		ui.Column(c).FillWidth().MinWidth(0).Gap(12).Disabled(a.notesWriteBlocked()).Children(func() {
			taskInput(c, "note-conversion-title", "Judul pekerjaan *", &conversion.title)
			if task {
				taskLongInput(c, "note-conversion-output", "Hasil yang diminta *", &conversion.output)
				taskInput(c, "note-conversion-category", "Kategori tugas", &conversion.category)
				name := "Pilih PJ utama"
				for _, member := range conversion.members {
					if member.UserID == conversion.assignee {
						name = member.DisplayName
					}
				}
				b := spaceButton(c, name, "users", false).FillWidth().Disabled(conversion.membersLoading)
				if b.Clicked() {
					conversion.memberOpen = !conversion.memberOpen
				}
				popover(c, b, &conversion.memberOpen, 280, func() {
					ui.Text(c, "PJ utama tugas").Bold()
					ui.Scroll(c).FillWidth().MaxHeight(220).Children(func() {
						ui.Column(c).FillWidth().MinWidth(0).Gap(5).Children(func() {
							for _, member := range conversion.members {
								if spaceButton(c.Key(member.UserID), member.DisplayName, "user", conversion.assignee == member.UserID).FillWidth().Clicked() {
									conversion.assignee, conversion.memberOpen = member.UserID, false
								}
							}
							if len(conversion.members) == 0 {
								muted(c, "Belum ada anggota yang dapat dipilih.")
							}
						})
					})
				})
				if conversion.membersLoading {
					muted(c, "Memuat anggota divisi pemilik tugas…").FontSize(11)
				}
			} else {
				muted(c, "Reminder milik akunmu; penerima tidak ditambahkan otomatis. Preferensi berbagi reminder tetap berlaku, dan reminder tidak dikirim ke grup WhatsApp.").FontSize(11)
			}
			taskInput(c, "note-conversion-date", "Tanggal tenggat *", &conversion.date)
			taskInput(c, "note-conversion-clock", "Jam tenggat (opsional)", &conversion.clock)
			muted(c, "Tanggal YYYY-MM-DD, jam HH:MM. Jam kosong tetap berarti tanggal tanpa jam.").FontSize(11)
			taskInput(c, "note-conversion-zone", "Zona waktu tenggat", &conversion.zone)
		})
		label := "Buat reminder pribadi"
		if task {
			label = "Buat tugas dan catat asalnya"
		}
		if spaceButton(c, label, "check", true).FillWidth().Disabled(a.notesWriteBlocked() || conversion.membersLoading).Clicked() {
			a.saveNoteConversion()
		}
		if spaceButton(c, "Batal membuat pekerjaan", "close", false).FillWidth().Disabled(a.notes.busy).Clicked() {
			a.notes.conversion = nil
		}
	})
}

func (a *App) noteViewZone() string {
	if period := a.period(); period != nil && period.TimeZone != "" {
		return period.TimeZone
	}
	return timeutil.DefaultZoneName
}

func noteTemplateKindLabel(kind string) string {
	if label, exists := map[string]string{api.TemplateMeeting: "Notulensi rapat", api.TemplateSchedule: "Tabel jadwal", api.TemplateFree: "Catatan bebas", api.TemplateCustom: "Template tersimpan"}[kind]; exists {
		return label
	}
	return "Catatan"
}

func noteEventLabel(action string) string {
	labels := map[string]string{"note.create": "Catatan dibuat", "note.update": "Isi resmi diperbarui", "note.lock.takeover": "Editor diambil alih", "note.comment": "Komentar ditambahkan", "note.comment.add": "Komentar ditambahkan", "note.attachment.add": "Lampiran ditambahkan", "note.attachment.upload": "Berkas lampiran diunggah", "note.attachment.remove": "Lampiran dihapus", "note.link.add": "Hubungan pekerjaan dicatat", "note.link.remove": "Hubungan pekerjaan dihapus"}
	if label, exists := labels[action]; exists {
		return label
	}
	return "Perubahan catatan"
}
