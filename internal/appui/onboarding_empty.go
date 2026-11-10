package appui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/api"
)

// Guided empty states (canvas A, "Halaman kosong"): a picture of what the
// page will hold, what to do first, three short facts, and the article
// that explains it. They come in slowly, so an empty page reads as a start
// rather than an error.

type guidedEmptyAction struct {
	label string
	do    func()
}

type guidedEmpty struct {
	key, picture, title, detail string
	primary, secondary          *guidedEmptyAction
	facts                       [][2]string
	guide                       string
}

func (a *App) guidedEmptyView(c *ui.Context, e guidedEmpty) {
	t := c.Theme()
	phone := isMobile(c)
	a.ensureGuides()
	ui.Column(c.Key("empty-"+e.key)).FillWidth().MaxWidth(600).Gap(10).Padding(phoneSize(phone, 16, 40), 0).
		Transition(ui.ElementTransition{Duration: 300 * time.Millisecond, Ease: ui.EaseOut, Enter: &ui.Motion{Opacity: 0, Y: 12}}).Children(func() {
		ui.Column(c).Width(min(float32(320), phoneSize(phone, 300, 320))).Children(func() { guidePicture(c, e.picture) })
		ui.Text(c, e.title).FontSize(18).FontWeight(600).Padding(6, 0, 0, 0)
		ui.Text(c, e.detail).FontSize(13).TextColor(t.TextMuted).MaxWidth(500)
		buttons := ui.Row(c).Gap(8).Padding(6, 0, 0, 0)
		if phone {
			buttons.Column().FillWidth()
		}
		buttons.Children(func() {
			for i, act := range []*guidedEmptyAction{e.primary, e.secondary} {
				if act == nil {
					continue
				}
				var b ui.Element
				if i == 0 {
					b = onbPrimary(c, act.label, act.label, false)
				} else {
					b = onbSecondary(c, act.label, act.label, false)
				}
				if phone {
					b.FillWidth().MinHeight(44)
				}
				if b.Clicked() {
					act.do()
				}
			}
		})
		if len(e.facts) > 0 {
			facts := ui.Row(c).FillWidth().Gap(12).Padding(16, 0, 0, 0)
			if phone {
				facts.Column()
			}
			facts.Children(func() {
				for i, f := range e.facts {
					ui.Column(c.Key(i)).Grow(1).Width(0).MinWidth(0).Gap(2).Padding(8, 0, 0, 0).BorderWidth(2, 0, 0, 0).BorderColor(t.Border).
						Transition(ui.ElementTransition{Animation: &ui.Animation{Duration: 240 * time.Millisecond, Ease: ui.EaseOut, Delay: time.Duration(i+2) * 60 * time.Millisecond}, Enter: &ui.Motion{Opacity: 0, Y: 6}}).Children(func() {
						ui.Text(c, f[0]).FontSize(13).FontWeight(500)
						ui.Text(c, f[1]).FontSize(12).TextColor(faint(t))
					})
				}
			})
		}
		if g, ok := a.guideBySlug(e.guide); ok {
			if onbLink(c, "Baca panduan "+g.Title, fmt.Sprintf("Baca: %s · %d mnt", g.Title, max(1, g.Minutes))).Clicked() {
				a.openGuidePanel(g.Slug)
			}
		}
	})
}

// boardEmpty tells whether the Space board has no task at all (not only
// none that the filter shows).
func (a *App) boardEmpty() bool {
	return a.tasks.loaded && len(a.tasks.items) == 0 && a.tasks.filter == "" && !a.tasks.mineOnly
}

func (a *App) boardEmptyGuide(c *ui.Context) {
	canManage := len(a.manageableDivisions()) > 0
	e := guidedEmpty{key: "board", picture: "board", guide: "board-tugas",
		facts: [][2]string{{"PJ dan tenggat", "pengingat masuk ke WhatsApp PJ"}, {"Empat kolom", "Belum mulai sampai Selesai"}, {"Nomor #T", "dipakai juga di perintah bot"}}}
	if canManage {
		e.title = "Belum ada tugas"
		e.detail = "Buat tugas, pilih PJ dan tenggat. Tugas pertama biasanya lahir dari rapat: action item di catatan rapat bisa langsung dijadikan tugas."
		e.primary = &guidedEmptyAction{"Buat tugas pertama", func() { a.beginTask() }}
		e.secondary = &guidedEmptyAction{"Buka catatan rapat", func() { a.chrome.tab = "Catatan" }}
		e.guide = "membuat-tugas"
	} else {
		e.title = "Belum ada tugas di divisi ini"
		e.detail = "Kadiv atau 45 yang membagi tugas. Begitu ada tugas untukmu, kartunya muncul di sini dan di Jobdesk saya, dan pengingatnya masuk ke WhatsApp."
		e.primary = &guidedEmptyAction{"Lihat catatan rapat", func() { a.chrome.tab = "Catatan" }}
	}
	a.guidedEmptyStrip(c, e)
}

// guidedEmptyStrip is a guided empty state as a card above a page that
// still shows its frame, as the board's empty columns.
func (a *App) guidedEmptyStrip(c *ui.Context, e guidedEmpty) {
	t := c.Theme()
	phone := isMobile(c)
	a.ensureGuides()
	ui.Column(c.Key("strip-wrap-"+e.key)).FillWidth().Padding(phoneSize(phone, 10, 14), phoneSize(phone, 12, 22), 0, phoneSize(phone, 12, 22)).Children(func() {
		card := ui.Row(c.Key("strip-"+e.key)).FillWidth().MinWidth(0).Gap(16).Padding(14, 18).Radius(12).Background(t.Surface).Border(1, t.Border).AlignItems(ui.Center).
			Transition(ui.ElementTransition{Duration: 300 * time.Millisecond, Ease: ui.EaseOut, Enter: &ui.Motion{Opacity: 0, Y: 10}, Exit: &ui.Motion{Opacity: 0, Collapse: true}})
		if phone {
			card.Column().AlignItems(ui.Stretch).Gap(10)
		}
		card.Children(func() {
			if !phone {
				ui.Column(c).Width(260).Shrink(0).Children(func() { guidePicture(c, e.picture) })
			}
			ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(func() {
				ui.Text(c, e.title).FontSize(16).FontWeight(600)
				ui.Text(c, e.detail).FontSize(13).TextColor(t.TextMuted)
				ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Padding(4, 0, 0, 0).Children(func() {
					for i, act := range []*guidedEmptyAction{e.primary, e.secondary} {
						if act == nil {
							continue
						}
						var b ui.Element
						if i == 0 {
							b = onbPrimary(c, act.label, act.label, true)
						} else {
							b = onbSecondary(c, act.label, act.label, true)
						}
						if b.Clicked() {
							act.do()
						}
					}
					if g, ok := a.guideBySlug(e.guide); ok {
						if onbLink(c, "Baca panduan "+g.Title, fmt.Sprintf("Baca: %s · %d mnt", g.Title, max(1, g.Minutes))).Clicked() {
							a.openGuidePanel(g.Slug)
						}
					}
				})
			})
		})
	})
}

// notesEmptyGuide is the notes list with no note yet.
func (a *App) notesEmptyGuide(c *ui.Context, canCreate bool) {
	e := guidedEmpty{key: "notes", picture: "catatan", guide: "catatan-rapat",
		title:  "Catatan pertama biasanya notulen rapat",
		detail: "Mulai dari template Rapat: agenda dengan durasi, peserta, dan action item yang bisa langsung dijadikan tugas.",
		facts:  [][2]string{{"Agenda + durasi", "dipakai timer rapat"}, {"Action item + PJ", "jadi tugas di board"}, {"Satu penulis", "yang lain membaca langsung"}}}
	if canCreate {
		e.primary = &guidedEmptyAction{"Buat catatan rapat", a.beginNote}
		e.guide = "menjadi-notulis"
	}
	e.secondary = &guidedEmptyAction{"Cara membaca catatan", func() { a.openGuidePanel("catatan-rapat") }}
	a.guidedEmptyView(c, e)
}

// membersEmptyGuide is the members page of a division with nobody yet.
func (a *App) membersEmptyGuide(c *ui.Context) {
	e := guidedEmpty{key: "members", picture: "anggota", guide: "anggota-dan-role",
		title:  "Undang anggota lewat NIM",
		detail: "Ketik NIM di form Undang anggota. Undangan dikirim ke email kampusnya; setelah ia bergabung, atur role-nya di sini.",
		facts:  [][2]string{{"NIM ke email kampus", "NIM@" + api.StudentEmailDomain}, {"Undangan tertunda", "terlihat sampai diterima"}, {"Role awal Anggota", "ubah ke 45 bila perlu"}}}
	e.primary = &guidedEmptyAction{"Cara mengundang", func() { a.openGuidePanel("anggota-dan-role") }}
	a.guidedEmptyView(c, e)
}
