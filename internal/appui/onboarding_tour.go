package appui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/api"
)

// The tour: 5 steps by role, a spotlight that glides from one target to
// the next, and a card beside it. A step on a tab first asks to press the
// tab, then lights the page it opened, so the page is seen, not only its
// button. It shows once (welcome → Mulai tur, or
// the checklist), can be skipped, and replays from the profile menu, the
// finished checklist and the Panduan.

type tourState struct {
	open bool
	step int
	role string
}

// tourStep points at a target: "sidebar", "help", or "tab:<title>"; a
// step on a tab names it in tab.
type tourStep struct{ target, title, body, tab string }

// tourSteps are role's steps, each pointing at a tab the account sees
// (visibleTab, by title): a step may fall back to another tab, as the
// WhatsApp step to Bot WhatsApp where Ping is not open to the role or is
// hidden, and is left out when none of its tabs shows. The sidebar and the
// "?" are always there.
func tourSteps(role string, visibleTab func(string) bool) []tourStep {
	type choice struct {
		tab, title, body string
	}
	pick := func(out []tourStep, choices ...choice) []tourStep {
		for _, ch := range choices {
			if visibleTab(ch.tab) {
				return append(out, tourStep{"tab:" + ch.tab, ch.title, ch.body, ch.tab})
			}
		}
		return out
	}
	waBot := choice{"Bot WhatsApp", "Bot WhatsApp", "Tautkan nomormu di sini. Ping dari kadiv dan pengingat tenggat lalu masuk ke WhatsApp-mu; di grup, ketik !tugas untuk melihat tugasmu."}
	sidebar := tourStep{"sidebar", "Sidebar", "Untukku berisi tugas, rapat, dan pengingatmu minggu ini; di bawahnya Space divisimu. Logo membuka daftar Space; Panduan dan Pengaturan ada di bawah.", ""}
	help := tourStep{"help", "Tombol ?", "Bingung di halaman mana pun? Tombol ini membuka panduan untuk halaman itu di panel samping.", ""}
	var out []tourStep
	switch role {
	case api.GuideRoleKadiv:
		out = append(out, tourStep{"sidebar", "Sidebar", "Divisi yang kamu pimpin, periode aktif, dan Pengaturan. Panduan kadiv ada di bawah.", ""})
		out = pick(out, choice{"Board tugas", "Board tugas", "Buat tugas, pilih PJ dan tenggat. PJ menerima pengingat otomatis di WhatsApp."})
		out = pick(out, choice{"Catatan", "Catatan rapat", "Pimpin rapat: agenda dengan durasi, timer, dan action item yang langsung jadi tugas."})
		out = pick(out, choice{"Anggota", "Anggota", "Undang lewat NIM, atur role (Anggota atau 45), dan lihat beban tiap orang."},
			choice{"Ping", "Ping & bot WA", "Kirim pengingat ke anggota. Mereka menerimanya di WhatsApp."}, waBot)
		help.body = "Panduan kadiv ada di sini: serah terima, pergantian periode, dan lainnya."
	case api.GuideRole45:
		out = append(out, sidebar)
		out = pick(out, choice{"Board tugas", "Board tugas", "Kamu bisa membuat dan membagi tugas. Mengatur anggota tetap di tangan kadiv."})
		out = pick(out, choice{"Catatan", "Catatan rapat", "Ambil editor, isi agenda, jalankan timer, dan jadikan action item tugas."})
		out = pick(out, choice{"Ping", "Ping & bot WA", "Kirim pengingat ke anggota. Mereka menerimanya di WhatsApp."}, waBot)
	default:
		out = append(out, sidebar)
		out = pick(out, choice{"Board tugas", "Board tugas", "Tugas yang diberikan kepadamu ada di sini. Pindahkan kartu ke Dikerjakan saat mulai."})
		out = pick(out, choice{"Catatan", "Catatan rapat", "Baca notulen rapat. Action item untukmu ditandai dengan PJ dan tenggat."})
		out = pick(out, choice{"Ping", "Ping & bot WA", "Kadiv bisa mengirim ping ke WhatsApp-mu. Di grup, ketik !tugas untuk melihat tugasmu."}, waBot)
	}
	return append(out, help)
}

// tourVisibleTab reports whether the tour's Space shows tab title in its
// tab bar: open to the account and not hidden by it.
func (a *App) tourVisibleTab(title string) bool {
	sp := a.onboardingSpace()
	if sp == nil {
		return false
	}
	for _, tab := range a.spaceTabsVisible(sp) {
		if tab.title == title {
			return true
		}
	}
	return false
}

// tourPressing reports whether step waits for its tab to be pressed: a
// step on a tab that is not open yet.
func (a *App) tourPressing(step tourStep) bool {
	return step.tab != "" && a.chrome.tab != step.tab
}

// tourAsksFor reports whether the open tour asks to press tab title.
func (a *App) tourAsksFor(title string) bool {
	st := &a.onb.tour
	if !st.open {
		return false
	}
	steps := tourSteps(st.role, a.tourVisibleTab)
	if st.step < 0 || st.step >= len(steps) {
		return false
	}
	return steps[st.step].tab == title && a.tourPressing(steps[st.step])
}

// tourPage is where the open tab's page shows: the Space's page below its
// tab bar.
func (a *App) tourPage() (ui.Rect, bool) {
	r, ok := a.onb.targets["space-page"]
	if !ok {
		return r, false
	}
	if bar, ok := a.onb.targets["space-tab-bar"]; ok {
		cut := bar.Y + bar.H - r.Y
		r.Y += cut
		r.H -= cut
	}
	return r, r.H > 0
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

// endTour closes the tour back on the Space's first tab, where it began.
func (a *App) endTour() {
	a.onb.tour.open = false
	if sp := a.onboardingSpace(); sp != nil && a.selectedSpace() != nil && a.selectedSpace().ID == sp.ID {
		a.chrome.tab = defaultSpaceTab(*sp)
	}
	a.markOnboarding(api.OnboardTour)
}

func (a *App) tourOverlay(c *ui.Context) {
	st := &a.onb.tour
	steps := tourSteps(st.role, a.tourVisibleTab)
	st.step = max(0, min(st.step, len(steps)-1))
	step := steps[st.step]
	if c.Shortcut(0, ui.KeyEscape) {
		a.endTour()
		return
	}
	t := c.Theme()
	w, h := c.Size()
	phone := w < mobileMaxWidth
	pressing := a.tourPressing(step)
	target, found := a.onb.targets[step.target]
	page := step.tab != "" && !pressing
	if page {
		target, found = a.tourPage()
	}
	if !found && step.target == "sidebar" && phone {
		target, found = a.onb.targets["mobile-nav"]
	}
	if !found {
		// Not measured yet (the first frame of a step, or a page still
		// loading): no hole, the card alone. Steps only name tabs that show.
		target = ui.Rect{X: w / 2, Y: h / 2}
	}
	pad := float32(6)
	if step.target == "sidebar" || page {
		pad = -2
	}
	dim := ui.RGBA(0, 0, 0, .46)
	ui.Overlay(c, func() {
		// The hole lets presses through to what it lights, as the tab
		// a step asks to press: only the dim and the card take them.
		root := ui.Box(c.Key("tour")).Absolute().Left(0).Top(0).Width(w).Height(h).PassThrough()
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
				ui.Box(c.Key("ring")).Absolute().Left(x).Top(y).Size(rw, rh).Radius(10).Border(2, t.Accent).Opacity(ring).PassThrough()
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
	case step.tab != "" && !a.tourPressing(step):
		// Over the page's bottom right, leaving its top to be seen.
		card.Left(max(16, r.X+r.W-cardW-24)).Bottom(max(16, h-r.Y-r.H+24)).Width(cardW)
	default:
		left := max(16, min(r.X+r.W/2-cardW/2, w-cardW-16))
		top := r.Y + r.H + 14
		if top+cardH > h-16 {
			top = max(16, r.Y-cardH-14)
		}
		card.Left(left).Top(top).Width(cardW)
	}
	pressing := a.tourPressing(step)
	card.Children(func() {
		ui.Text(c, strings.ToUpper("Tur untuk "+guideRoleName(st.role))).FontSize(11).TextColor(faint(t))
		ui.Text(c, step.title).FontSize(16).Bold()
		if pressing {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, spaceIcon("arrow")).Size(16, 16).Shrink(0).TextColor(t.Accent)
				ui.Text(c, "Tekan tab "+step.tab+" yang disorot untuk membukanya.").FontSize(13).Grow(1).MinWidth(0)
			})
		} else {
			ui.Text(c, step.body).FontSize(13).TextColor(t.TextMuted)
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
			if pressing {
				// The tab is pressed in the page; this opens it for those
				// who cannot reach it, as on a phone.
				if onbLink(c, "Bukakan tab "+step.tab, "Bukakan").Clicked() {
					a.chrome.tab = step.tab
				}
				return
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
