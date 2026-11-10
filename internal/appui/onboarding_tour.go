package appui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/api"
)

// The tour: 5 steps by role, a spotlight that glides from one target to
// the next, and a card beside it. It shows once (welcome → Mulai tur, or
// the checklist), can be skipped, and replays from the profile menu, the
// finished checklist and the Panduan.

type tourState struct {
	open bool
	step int
	role string
}

// tourStep points at a target: "sidebar", "help", or "tab:<title>".
type tourStep struct{ target, title, body string }

func tourSteps(role string) []tourStep {
	sidebar := tourStep{"sidebar", "Sidebar", "Pindah antara Jobdesk saya, Reminder pribadi, dan Space divisimu. Panduan dan Pengaturan ada di bawah."}
	help := tourStep{"help", "Tombol ?", "Bingung di halaman mana pun? Tombol ini membuka panduan untuk halaman itu di panel samping."}
	switch role {
	case api.GuideRoleKadiv:
		return []tourStep{
			{"sidebar", "Sidebar", "Divisi yang kamu pimpin, periode aktif, dan Pengaturan. Panduan kadiv ada di bawah."},
			{"tab:Board tugas", "Board tugas", "Buat tugas, pilih PJ dan tenggat. PJ menerima pengingat otomatis di WhatsApp."},
			{"tab:Catatan", "Catatan rapat", "Pimpin rapat: agenda dengan durasi, timer, dan action item yang langsung jadi tugas."},
			{"tab:Anggota", "Anggota", "Undang lewat NIM, atur role (Anggota atau 45), dan lihat beban tiap orang."},
			{"help", "Tombol ?", "Panduan kadiv ada di sini: serah terima, pergantian periode, dan lainnya."},
		}
	case api.GuideRole45:
		return []tourStep{
			sidebar,
			{"tab:Board tugas", "Board tugas", "Kamu bisa membuat dan membagi tugas. Mengatur anggota tetap di tangan kadiv."},
			{"tab:Catatan", "Catatan rapat", "Ambil editor, isi agenda, jalankan timer, dan jadikan action item tugas."},
			{"tab:Ping", "Ping & bot WA", "Kirim pengingat ke anggota. Mereka menerimanya di WhatsApp."},
			help,
		}
	}
	return []tourStep{
		sidebar,
		{"tab:Board tugas", "Board tugas", "Tugas yang diberikan kepadamu ada di sini. Pindahkan kartu ke Dikerjakan saat mulai."},
		{"tab:Catatan", "Catatan rapat", "Baca notulen rapat. Action item untukmu ditandai dengan PJ dan tenggat."},
		{"tab:Ping", "Ping & bot WA", "Kadiv bisa mengirim ping ke WhatsApp-mu. Di grup, ketik !tugas untuk melihat tugasmu."},
		help,
	}
}

// tourTarget records where el was in the last frame, for the tour.
func (a *App) tourTarget(name string, el ui.Element) {
	if !a.onb.tour.open {
		return
	}
	if a.onb.targets == nil {
		a.onb.targets = map[string]ui.Rect{}
	}
	if r := el.Bounds(); r.W > 0 && r.H > 0 {
		a.onb.targets[name] = r
	}
}

// tourFrameStart forgets the targets of the frame before, so a target
// that is no longer shown is not pointed at.
func (a *App) tourFrameStart() {
	clear(a.onb.targets)
}

// startTour opens the tour on the onboarding Space's Beranda.
func (a *App) startTour() {
	a.onb.tour = tourState{open: true, role: a.onboardRole()}
	a.onb.welcome = false
	a.guide.panel = false
	if sp := a.onboardingSpace(); sp != nil {
		a.selectSpace(sp.ID)
		a.chrome.tab = defaultSpaceTab(*sp)
		a.navigate(PathSpaces)
	}
}

func (a *App) endTour() {
	a.onb.tour.open = false
	a.markOnboarding(api.OnboardTour)
}

func (a *App) tourOverlay(c *ui.Context) {
	st := &a.onb.tour
	steps := tourSteps(st.role)
	st.step = max(0, min(st.step, len(steps)-1))
	step := steps[st.step]
	if c.Shortcut(0, ui.KeyEscape) {
		a.endTour()
		return
	}
	t := c.Theme()
	w, h := c.Size()
	phone := w < mobileMaxWidth
	target, found := a.onb.targets[step.target]
	if !found && step.target == "sidebar" && phone {
		target, found = a.onb.targets["mobile-nav"]
	}
	if !found {
		// Not shown here (a hidden tab, a phone): no hole, the card alone.
		target = ui.Rect{X: w / 2, Y: h / 2}
	}
	pad := float32(6)
	if step.target == "sidebar" {
		pad = -2
	}
	dim := ui.RGBA(0, 0, 0, .46)
	ui.Overlay(c, func() {
		root := ui.Box(c.Key("tour")).Absolute().Left(0).Top(0).Width(w).Height(h)
		x := root.AnimateTo("x", target.X-pad, onbMove)
		y := root.AnimateTo("y", target.Y-pad, onbMove)
		rw := root.AnimateTo("w", max(0, target.W+2*pad), onbMove)
		rh := root.AnimateTo("h", max(0, target.H+2*pad), onbMove)
		ring := root.AnimateTo("ring", b2f(found), onbQuick)
		root.Children(func() {
			ui.Box(c.Key("dim-top")).Absolute().Left(0).Top(0).Width(w).Height(max(0, y)).Background(dim)
			ui.Box(c.Key("dim-bottom")).Absolute().Left(0).Top(y + rh).Width(w).Height(max(0, h-y-rh)).Background(dim)
			ui.Box(c.Key("dim-left")).Absolute().Left(0).Top(y).Width(max(0, x)).Height(rh).Background(dim)
			ui.Box(c.Key("dim-right")).Absolute().Left(x + rw).Top(y).Width(max(0, w-x-rw)).Height(rh).Background(dim)
			if ring > 0.01 {
				ui.Box(c.Key("ring")).Absolute().Left(x).Top(y).Size(rw, rh).Radius(10).Border(2, t.Accent).Opacity(ring)
			}
			a.tourCard(c, steps, ui.Rect{X: x, Y: y, W: rw, H: rh}, found, phone, w, h)
		})
	})
}

func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// tourCard is the step's card, beside the spotlight: right of the
// sidebar, else below the target (above it near the bottom); on a phone
// along the bottom. Each step's card enters on its own.
func (a *App) tourCard(c *ui.Context, steps []tourStep, r ui.Rect, found, phone bool, w, h float32) {
	t := c.Theme()
	st := &a.onb.tour
	step := steps[st.step]
	const cardW, cardH = float32(340), float32(210)
	card := ui.Column(c.Key(fmt.Sprintf("tour-card-%d", st.step))).Absolute().Padding(16, 18).Gap(6).Radius(12).Background(t.Surface).Border(1, t.Border).
		Shadow(0, 16, 40, 0, ui.RGBA(0, 0, 0, .22)).Transition(onbPop)
	switch {
	case phone:
		card.Left(12).Width(w - 24).Bottom(12)
	case !found:
		card.Left((w - cardW) / 2).Top((h - cardH) / 2).Width(cardW)
	case step.target == "sidebar":
		card.Left(r.X + r.W + 16).Top(max(16, min(r.Y+80, h-cardH-16))).Width(cardW)
	default:
		left := max(16, min(r.X+r.W/2-cardW/2, w-cardW-16))
		top := r.Y + r.H + 14
		if top+cardH > h-16 {
			top = max(16, r.Y-cardH-14)
		}
		card.Left(left).Top(top).Width(cardW)
	}
	card.Children(func() {
		ui.Text(c, strings.ToUpper("Tur untuk "+guideRoleName(st.role))).FontSize(11).TextColor(faint(t))
		ui.Text(c, step.title).FontSize(16).Bold()
		ui.Text(c, step.body).FontSize(13).TextColor(t.TextMuted)
		if !found && !phone {
			muted(c, "Bagian ini tidak tampil di layarmu sekarang.").FontSize(11)
		}
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Margin(8, 0, 0, 0).Children(func() {
			ui.Row(c.Key("dots")).Gap(4).AlignItems(ui.Center).Children(func() {
				for i := range steps {
					dot := ui.Box(c.Key(i)).Height(4).Radius(2).Background(t.Border).Transition(ui.ElementTransition{Duration: 220 * time.Millisecond, Size: true, Colors: true})
					if i == st.step {
						dot.Width(18).Background(t.Accent)
					} else if i < st.step {
						dot.Width(8).Background(spaceMix(t.Accent, t.Surface, .45))
					} else {
						dot.Width(8)
					}
				}
			})
			ui.Text(c, fmt.Sprintf("%d/%d", st.step+1, len(steps))).Font(FontMono).FontSize(11).TextColor(faint(t))
			ui.Box(c).Grow(1)
			if st.step == 0 {
				if onbLink(c, "Lewati tur", "Lewati").Clicked() {
					a.endTour()
				}
			} else if onbLink(c, "Langkah sebelumnya", "Kembali").Clicked() {
				st.step--
			}
			last := st.step == len(steps)-1
			label := "Lanjut"
			if last {
				label = "Selesai"
			}
			if onbPrimary(c, label+" tur", label, true).Clicked() {
				if last {
					a.endTour()
				} else {
					st.step++
				}
			}
		})
	})
}
