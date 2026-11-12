package appui

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Onboarding (canvas a036cb358772, option A): a welcome after joining, the
// "Langkah pertama" checklist on Beranda, a spotlight tour by role, the
// guide (guide.go) and guided empty states (onboarding_empty.go). The
// progress lives on the server (/api/me/onboarding), so it follows the
// account; without that endpoint none of it shows.

// OnboardingAPI is the server's onboarding progress and the profile.
type OnboardingAPI interface {
	Onboarding(ctx context.Context, periodID id.ID) (api.Onboarding, error)
	MarkOnboarding(ctx context.Context, m api.OnboardingMark) (api.Onboarding, error)
	UpdateProfile(ctx context.Context, p api.ProfileUpdate) (api.User, error)
}

// Motion of onboarding: short, eased, as the canvas asks (150–300 ms).
// MyGo stops what moves, and only fades, while the desktop asks for less
// motion (Preferences.ReduceMotion), so these need no check of their own.
var (
	onbQuick  = ui.Animation{Duration: 180 * time.Millisecond, Ease: ui.EaseOut}
	onbMove   = ui.Animation{Duration: 280 * time.Millisecond, Ease: ui.EaseInOut}
	onbEnter  = ui.ElementTransition{Duration: 240 * time.Millisecond, Enter: &ui.Motion{Opacity: 0, Y: 8}, Exit: &ui.Motion{Opacity: 0, Collapse: true}}
	onbPop    = ui.ElementTransition{Duration: 220 * time.Millisecond, Enter: &ui.Motion{Opacity: 0, Y: 6, Scale: .97}, Exit: &ui.Motion{Opacity: 0}}
	onbSlide  = ui.ElementTransition{Duration: 260 * time.Millisecond, Size: true, Enter: &ui.Motion{Opacity: 0, X: 32}, Exit: &ui.Motion{Opacity: 0, X: 32}}
	onbCheckT = ui.ElementTransition{Duration: 260 * time.Millisecond, Ease: easeOutBack, Enter: &ui.Motion{Opacity: 0, Scale: .3}}
)

// easeOutBack overshoots a little and settles: the check's pop.
func easeOutBack(t float32) float32 {
	const c1, c3 = 1.70158, 2.70158
	u := float64(t) - 1
	return float32(1 + c3*u*u*u + c1*u*u)
}

// onboardingRefresh is how often the checklist asks again while a step it
// derives (WhatsApp) is still open, so linking from the phone ticks it.
const onboardingRefresh = 30 * time.Second

type onboardingState struct {
	user    id.ID
	off     bool
	loading bool
	loaded  bool
	err     string
	data    api.Onboarding
	fetched time.Time
	// welcome is the onboarding guided by Ace (onboarding_ace.go), open
	// until finished or skipped once.
	welcome bool
	ace     aceWizardState
	tour    tourState
	// profile is the "Lengkapi profil" dialog.
	profileOpen bool
	profileName string
	profileBusy bool
	profileErr  string
	// allDone remembers whether every step was done when last drawn, so
	// finishing the last one celebrates (celebrate) and an account that
	// arrives done does not.
	allDone   bool
	drawnOnce bool
	celebrate time.Time
	// targets are this frame's bounds of what the tour points at.
	targets map[string]ui.Rect
	// notifPerm is the device's notification permission last frame, so a
	// change to granted sends the test notice (watchNotificationPermission).
	notifPerm string
}

func (a *App) onboardingService() (OnboardingAPI, bool) {
	if c, ok := a.api.(*client.Client); ok {
		return c.AccountSession(), true
	}
	s, ok := a.api.(OnboardingAPI)
	return s, ok
}

// onboardingActive tells whether onboarding applies: a person with a
// division Space, once the server's progress is in.
func (a *App) onboardingActive() bool {
	if a.isAdmin() || a.onb.off || !a.onb.loaded || a.user.ID != a.onb.user {
		return false
	}
	return a.onboardingSpace() != nil
}

// onboardingSpace is the division Space onboarding points at: the
// selected one, else the first.
func (a *App) onboardingSpace() *api.Space {
	if s := a.selectedSpace(); s != nil && s.Kind == api.SpaceDivision {
		return s
	}
	for i := range a.access.spaces {
		if a.access.spaces[i].Kind == api.SpaceDivision {
			return &a.access.spaces[i]
		}
	}
	return nil
}

// onboardRole is the guide role of the account in the onboarding Space:
// kadiv, 45 (also for a pendamping), or anggota.
func (a *App) onboardRole() string {
	s := a.onboardingSpace()
	if s == nil {
		return api.GuideRoleMember
	}
	role := api.GuideRoleMember
	for _, b := range s.Roles {
		for _, r := range b.Roles {
			switch r {
			case api.RoleKadiv:
				return api.GuideRoleKadiv
			case api.Role45, api.RoleCompanion:
				role = api.GuideRole45
			}
		}
	}
	return role
}

func guideRoleName(role string) string {
	switch role {
	case api.GuideRoleKadiv:
		return "Kadiv"
	case api.GuideRole45:
		return "45"
	}
	return "Anggota"
}

// ensureOnboarding fetches the progress once per account, and again now
// and then while the checklist waits on WhatsApp.
func (a *App) ensureOnboarding(now time.Time) {
	st := &a.onb
	if a.user.ID == id.Nil || a.isAdmin() {
		return
	}
	if st.user != a.user.ID {
		*st = onboardingState{user: a.user.ID}
	}
	if st.loading || st.off {
		return
	}
	stale := st.loaded && !st.data.WhatsAppLinked && !st.data.Done(api.OnboardChecklistHidden) && now.Sub(st.fetched) >= onboardingRefresh
	if (st.loaded || st.err != "") && !stale {
		return
	}
	svc, ok := a.onboardingService()
	if !ok {
		st.off = true
		return
	}
	gen, uid, pid := a.gen, a.user.ID, a.periodID
	st.loading, st.fetched = true, now
	a.async(func() func() {
		data, err := svc.Onboarding(context.Background(), pid)
		return func() {
			if gen != a.gen || a.onb.user != uid {
				return
			}
			st := &a.onb
			st.loading = false
			if uiPreferencesMissing(err) {
				st.off = true
				return
			}
			if err != nil {
				if !st.loaded {
					st.err = a.failed(err)
				}
				return
			}
			first := !st.loaded
			st.loaded, st.err = true, ""
			st.data = mergeOnboarding(st.data, data)
			if first && !st.data.Done(api.OnboardWelcome) && a.onboardingSpace() != nil {
				st.welcome = true
			}
		}
	})
}

// mergeOnboarding keeps marks set on this device that the answer does not
// have yet: a mark sent and one fetched may cross.
func mergeOnboarding(local, server api.Onboarding) api.Onboarding {
	if server.Marks == nil {
		server.Marks = map[string]time.Time{}
	}
	for k, v := range local.Marks {
		if _, ok := server.Marks[k]; !ok {
			server.Marks[k] = v
		}
	}
	return server
}

// markOnboarding records a step at once on screen and sends it.
func (a *App) markOnboarding(key string) {
	st := &a.onb
	if !st.loaded || st.data.Done(key) {
		return
	}
	if st.data.Marks == nil {
		st.data.Marks = map[string]time.Time{}
	}
	st.data.Marks[key] = time.Now()
	svc, ok := a.onboardingService()
	if !ok {
		return
	}
	gen, uid := a.gen, a.user.ID
	a.async(func() func() {
		data, err := svc.MarkOnboarding(context.Background(), api.OnboardingMark{Key: key})
		return func() {
			if gen != a.gen || a.onb.user != uid {
				return
			}
			if err != nil {
				a.addNotice("Langkah belum tersimpan di server: " + a.failed(err))
				return
			}
			linked := a.onb.data.WhatsAppLinked
			a.onb.data = mergeOnboarding(a.onb.data, data)
			a.onb.data.WhatsAppLinked = linked || data.WhatsAppLinked
		}
	})
}

// addNotice shows a short message as a toast on the next frame.
func (a *App) addNotice(msg string) {
	a.chrome.notice = msg
}

// onbStep is one item of "Langkah pertama".
type onbStep struct {
	key, title, detail, action, icon string
	done                             bool
}

func (a *App) onboardingSteps() []onbStep {
	d := a.onb.data
	steps := []onbStep{
		{"wa", "Tautkan WhatsApp", "Pengingat tugas dan ping masuk ke WhatsApp-mu lewat bot divisi.", "Tautkan", "whatsapp", d.WhatsAppLinked},
	}
	// Notifications are per device: the step follows this device's
	// permission and shows only where the device has notifications.
	switch perm := a.noticePermission(); perm {
	case BrowserPermissionGranted:
		steps = append(steps, onbStep{onbNotif, "Izinkan notifikasi", "Notifikasi di perangkat ini sudah diizinkan.", "", "bell", true})
	case BrowserPermissionDefault:
		steps = append(steps, onbStep{onbNotif, "Izinkan notifikasi", "Supaya pengingat dan ping muncul di perangkat ini selama Space terbuka.", "Izinkan", "bell", false})
	case BrowserPermissionDenied:
		steps = append(steps, onbStep{onbNotif, "Izinkan notifikasi", a.noticeDeniedHint(), "Lihat caranya", "bell", false})
	}
	return append(steps, []onbStep{
		{api.OnboardProfile, "Lengkapi profil", "Periksa nama yang dilihat rekan di board, catatan, dan ping.", "Buka profil", "user", d.Done(api.OnboardProfile)},
		{api.OnboardTasks, "Lihat tugasmu", "Buka Board tugas divisimu. Belum ada tugas? Wajar di minggu pertama.", "Buka board", "board", d.Done(api.OnboardTasks)},
		{api.OnboardTour, "Kenali tab Space", "Tur singkat, sekitar satu menit.", "Mulai tur", "compass", d.Done(api.OnboardTour)},
	}...)
}

// onbNotif is the checklist's notification step; it has no server mark.
const onbNotif = "notif"

// doOnboardingStep runs a step's action.
func (a *App) doOnboardingStep(key string) {
	sp := a.onboardingSpace()
	switch key {
	case "wa":
		if sp != nil {
			a.selectSpace(sp.ID)
			a.chrome.tab = "Bot WhatsApp"
			a.navigate(PathSpaces)
		}
	case onbNotif:
		if a.noticePermission() == BrowserPermissionDefault && a.Notifier != nil {
			a.Notifier.RequestPermission()
		} else {
			a.OpenDeviceSettings()
		}
	case api.OnboardProfile:
		a.onb.profileOpen, a.onb.profileName, a.onb.profileErr = true, a.user.DisplayName, ""
	case api.OnboardTasks:
		if sp != nil {
			a.selectSpace(sp.ID)
			a.chrome.tab = "Board tugas"
			a.navigate(PathSpaces)
		}
	case api.OnboardTour:
		a.startTour()
	}
}

// onboardingBeranda is what onboarding adds to the top of Beranda: the
// checklist, then the kadiv's handover card.
func (a *App) onboardingBeranda(c *ui.Context, phone bool) {
	if !a.onboardingActive() {
		return
	}
	ui.Column(c.Key("onb-beranda")).FillWidth().MinWidth(0).Gap(12).Children(func() {
		if !a.onb.data.Done(api.OnboardChecklistHidden) {
			a.onboardingChecklist(c, phone)
		}
		if a.onboardRole() == api.GuideRoleKadiv && !a.onb.data.Done(api.OnboardHandover) {
			a.handoverCard(c, phone)
		}
	})
}

func (a *App) onboardingChecklist(c *ui.Context, phone bool) {
	t := c.Theme()
	steps := a.onboardingSteps()
	n := 0
	for _, s := range steps {
		if s.done {
			n++
		}
	}
	all := n == len(steps)
	now := c.Now()
	if a.onb.drawnOnce && all && !a.onb.allDone {
		a.onb.celebrate = now
	}
	a.onb.allDone, a.onb.drawnOnce = all, true

	card := ui.Column(c.Key("onb-checklist")).Transition(onbEnter).FillWidth().MinWidth(0).Padding(16, 20).Gap(4).Radius(12).Background(t.Surface).Border(1, t.Border)
	if phone {
		card.Padding(14, 16)
	}
	card.Children(func() {
		if all {
			a.checklistDone(c, now)
			return
		}
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Text(c, "Langkah pertama").FontSize(15).Bold()
			ui.Text(c, fmt.Sprintf("%d dari %d", n, len(steps))).Font(FontMono).FontSize(12).TextColor(t.TextMuted)
			ui.Box(c).Grow(1)
			if onbLink(c, "Sembunyikan langkah pertama", "Sembunyikan").Clicked() {
				a.markOnboarding(api.OnboardChecklistHidden)
			}
		})
		onbProgress(c, "onb-progress", float32(n)/float32(len(steps)))
		next := -1
		for i, s := range steps {
			if !s.done && next < 0 {
				next = i
			}
		}
		for i, s := range steps {
			a.checklistRow(c, s, i == next, phone)
		}
	})
}

// onbProgress is a bar whose fill glides to frac.
func onbProgress(c *ui.Context, key string, frac float32) {
	t := c.Theme()
	track := ui.Row(c.Key(key)).FillWidth().Height(6).Radius(3).Background(spaceMix(t.Border, t.Surface, .35)).Margin(8, 0, 6, 0)
	w := track.Bounds().W
	if w <= 0 {
		c.Invalidate()
	}
	fill := track.AnimateTo("fill", frac*w, ui.Animation{Duration: 300 * time.Millisecond, Ease: ui.EaseOut})
	track.Children(func() {
		ui.Box(c.Key("fill")).Width(max(0, fill)).FillHeight().Radius(3).Background(t.Accent)
	})
}

func (a *App) checklistRow(c *ui.Context, s onbStep, next, phone bool) {
	t := c.Theme()
	row := ui.Row(c.Key("step-"+s.key)).FillWidth().MinWidth(0).Gap(12).Padding(10, 0).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(spaceMix(t.Border, t.Surface, .45))
	row.Children(func() {
		onbCheck(c, s.done, 22)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			title := ui.Text(c, s.title).FontWeight(500).Transition(ui.ElementTransition{Duration: 200 * time.Millisecond, Colors: true})
			if s.done {
				title.TextColor(t.TextMuted).Strikethrough()
			}
			if !s.done && (!phone || s.key == onbNotif) {
				muted(c, s.detail).FontSize(12)
			}
		})
		if !s.done {
			var b ui.Element
			if next {
				b = onbPrimary(c, s.action+": "+s.title, s.action, true)
			} else {
				b = onbSecondary(c, s.action+": "+s.title, s.action, true)
			}
			if b.Clicked() {
				a.doOnboardingStep(s.key)
			}
		}
	})
}

// onbCheck is a round check: empty, or filled with a tick that pops in.
func onbCheck(c *ui.Context, done bool, size float32) {
	t := c.Theme()
	box := ui.Box(c.Key("check")).Size(size, size).Shrink(0).Radius(size/2).Center().Border(1.5, t.Border).Transition(ui.ElementTransition{Duration: 220 * time.Millisecond, Colors: true})
	if done {
		box.Background(t.Success).BorderColor(t.Success)
	}
	box.Children(func() {
		if done {
			ui.Icon(c.Key("tick"), spaceIcon("check")).Size(size*.6, size*.6).TextColor(ui.Hex("#ffffff")).Transition(onbCheckT)
		}
	})
}

// checklistDone is the finished checklist: a short celebration, then the
// way to hide it.
func (a *App) checklistDone(c *ui.Context, now time.Time) {
	t := c.Theme()
	ui.Row(c.Key("onb-done")).FillWidth().MinWidth(0).Gap(14).AlignItems(ui.Center).Transition(onbPop).Children(func() {
		ui.Box(c.Key("burst")).Size(36, 36).Shrink(0).Children(func() {
			onbCheck(c, true, 36)
			onbBurst(c, a.onb.celebrate, now)
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, "Langkah pertama selesai").FontSize(15).Bold()
			muted(c, "Tur dan Panduan tetap ada: tombol ? di kanan atas, dan Ulangi tur di menu profil.").FontSize(12)
		})
		if onbSecondary(c, "Ulangi tur", "Ulangi tur", false).Clicked() {
			a.startTour()
		}
		if onbPrimary(c, "Sembunyikan checklist", "Sembunyikan checklist", false).Clicked() {
			a.markOnboarding(api.OnboardChecklistHidden)
		}
	})
	_ = t
}

// onbBurst is the celebration: dots fly out of the check and fade, for
// under a second after the last step is done.
func onbBurst(c *ui.Context, at, now time.Time) {
	if at.IsZero() {
		return
	}
	age := now.Sub(at)
	if age > 700*time.Millisecond {
		return
	}
	c.After(700*time.Millisecond - age)
	t := c.Theme()
	colors := []ui.Color{t.Accent, t.Success, t.Warning, t.Accent, t.Success, t.Warning, t.Accent, t.Success}
	for i, col := range colors {
		angle := float64(i) * 2 * math.Pi / float64(len(colors))
		r := float32(26)
		if i%2 == 1 {
			r = 34
		}
		dx, dy := r*float32(math.Cos(angle)), r*float32(math.Sin(angle))
		ui.Box(c.Key(fmt.Sprintf("dot-%d", i))).Absolute().Left(15+dx).Top(15+dy).Size(6, 6).Radius(3).Background(col).
			Transition(ui.ElementTransition{Duration: 300 * time.Millisecond, Ease: ui.EaseOut, Enter: &ui.Motion{X: -dx, Y: -dy, Opacity: 1, Scale: .4}, Exit: &ui.Motion{Opacity: 0, Scale: .5}})
	}
}

// handoverCard is the new kadiv's handover: where to go first.
func (a *App) handoverCard(c *ui.Context, phone bool) {
	t := c.Theme()
	sp := a.onboardingSpace()
	items := []struct{ title, detail, action, tab, guide string }{
		{"Cek divisi", "Periode di sidebar, tab yang dipakai, catatan serah terima dari kadiv lama.", "Baca panduan", "", "serah-terima-kadiv"},
		{"Undang anggota lewat NIM", "Undangan dikirim ke email kampus mereka.", "Buka Anggota", "Anggota", ""},
		{"Atur role", "Pilih siapa 45. 45 tidak bisa mengatur anggota atau role.", "Buka Anggota", "Anggota", ""},
		{"Siapkan bot WhatsApp", "Bot dan tautan nomor berlaku per periode: pasang ulang di periode baru.", "Buka Bot WhatsApp", "Bot WhatsApp", ""},
		{"Pergantian periode", "Apa yang ikut, apa yang perlu disiapkan ulang.", "Baca panduan", "", "pergantian-periode"},
	}
	card := ui.Column(c.Key("onb-handover")).Transition(onbEnter).FillWidth().MinWidth(0).Padding(16, 20).Gap(4).Radius(12).Background(t.Surface).Border(1, t.Border)
	if phone {
		card.Padding(14, 16)
	}
	card.Children(func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, "CHECKLIST KADIV BARU").FontSize(11).TextColor(faint(t))
				ui.Text(c, "Serah terima divisi").FontSize(15).Bold()
			})
			if onbLink(c, "Tandai serah terima selesai", "Tandai selesai").Clicked() {
				a.markOnboarding(api.OnboardHandover)
			}
		})
		for i, it := range items {
			ui.Row(c.Key(fmt.Sprintf("handover-%d", i))).FillWidth().MinWidth(0).Gap(12).Padding(9, 0).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(spaceMix(t.Border, t.Surface, .45)).Children(func() {
				ui.Box(c).Size(22, 22).Shrink(0).Radius(11).Center().Border(1.5, t.Border).Children(func() {
					ui.Text(c, fmt.Sprint(i+1)).FontSize(11).TextColor(t.TextMuted)
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, it.title).FontWeight(500)
					if !phone {
						muted(c, it.detail).FontSize(12)
					}
				})
				if onbSecondary(c, it.action+": "+it.title, it.action, true).Clicked() {
					if it.guide != "" {
						a.openGuidePanel(it.guide)
					} else if sp != nil {
						a.selectSpace(sp.ID)
						a.chrome.tab = it.tab
						a.navigate(PathSpaces)
					}
				}
			})
		}
	})
}

// onboardingOverlays are drawn last, above the shell: the welcome, the
// profile dialog, the tour, and on a phone the guide sheet.
func (a *App) onboardingOverlays(c *ui.Context) {
	if a.onboardingActive() {
		if a.onb.welcome && !a.onb.tour.open {
			a.aceWizard(c)
		}
		if a.onb.profileOpen {
			a.profileDialog(c)
		}
	}
	if a.onb.tour.open {
		a.tourOverlay(c)
	}
	a.aceUpdateDialog(c)
	if isMobile(c) && a.guide.panel {
		a.guideSheet(c)
	}
	if a.chrome.notice != "" {
		c.AddToast(ui.Toast{Title: a.chrome.notice, Type: "info", Timeout: 6 * time.Second})
		a.chrome.notice = ""
	}
}

func (a *App) dismissWelcome() {
	a.onb.welcome = false
	a.markOnboarding(api.OnboardWelcome)
	if sp := a.onboardingSpace(); sp != nil && a.router.Path() != PathSpaces {
		a.selectSpace(sp.ID)
		a.chrome.tab = defaultSpaceTab(*sp)
		a.navigate(PathSpaces)
	}
}

func onbRoleChip(c *ui.Context, role string) {
	t := c.Theme()
	chip := ui.Row(c).Height(22).Padding(0, 8).Radius(6).AlignItems(ui.Center).Background(spaceMix(t.Border, t.Surface, .4))
	label := ui.Color(t.TextMuted)
	switch role {
	case api.GuideRoleKadiv:
		chip.Background(t.Text)
		label = t.Surface
	case api.GuideRole45:
		chip.Background(spaceMix(t.Accent, t.Surface, .88)).Border(1, spaceMix(t.Accent, t.Surface, .6))
		label = t.Accent
	}
	chip.Children(func() { ui.Text(c, guideRoleName(role)).FontSize(12).TextColor(label) })
}

func (a *App) profileDialog(c *ui.Context) {
	t := c.Theme()
	w, h := c.Size()
	phone := w < mobileMaxWidth
	st := &a.onb
	open := true
	ui.DialogBase(c.Key("profile-dialog"), &open, func(backdrop, panel ui.Element) {
		dialogMotion(backdrop, panel)
		backdrop.Background(ui.RGBA(0, 0, 0, .45))
		if phone {
			backdrop.Justify(ui.End)
			panel.Width(w).MaxHeight(h-40).Radius(18, 18, 0, 0).Padding(20)
		} else {
			panel.Width(min(float32(440), w-48)).Radius(16).Padding(24)
		}
		panel.Column().Gap(12).Background(t.Surface).Border(1, t.Border).Transition(onbPop).Children(func() {
			ui.Text(c, "Profil").FontSize(17).Bold()
			muted(c, "Nama ini yang dilihat rekan divisi di board, catatan rapat, dan ping.").FontSize(13)
			ui.TextInput(c.Key("profile-name"), &st.profileName).Label("Nama tampilan").Placeholder("Nama yang biasa dipanggil").FillWidth().Disabled(st.profileBusy)
			info(c, [][2]string{{"Email", a.user.Email}, {"NIM", firstNonEmpty(a.user.NIM, "—")}})
			if st.profileErr != "" {
				ui.Text(c, st.profileErr).FontSize(12).TextColor(t.Danger)
			}
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				if onbSecondary(c, "Batal", "Batal", false).Clicked() {
					st.profileOpen = false
				}
				if onbPrimary(c, "Simpan profil", "Simpan", false).Disabled(st.profileBusy).Clicked() {
					a.saveProfile()
				}
			})
		})
	})
	if !open {
		st.profileOpen = false
	}
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func (a *App) saveProfile() {
	st := &a.onb
	name := strings.Join(strings.Fields(st.profileName), " ")
	if n := len([]rune(name)); n < 2 || n > 80 {
		st.profileErr = "Nama tampilan 2 sampai 80 huruf."
		return
	}
	svc, ok := a.onboardingService()
	if !ok {
		st.profileOpen = false
		return
	}
	gen, uid := a.gen, a.user.ID
	st.profileBusy, st.profileErr = true, ""
	a.async(func() func() {
		u, err := svc.UpdateProfile(context.Background(), api.ProfileUpdate{DisplayName: name})
		return func() {
			if gen != a.gen || a.onb.user != uid {
				return
			}
			st := &a.onb
			st.profileBusy = false
			if err != nil {
				st.profileErr = a.failed(err)
				if a.onb.welcome {
					a.addNotice("Nama belum tersimpan: " + st.profileErr)
				}
				return
			}
			a.user.DisplayName = u.DisplayName
			st.profileOpen = false
			if st.data.Marks == nil {
				st.data.Marks = map[string]time.Time{}
			}
			st.data.Marks[api.OnboardProfile] = time.Now()
			a.addNotice("Profil tersimpan.")
		}
	})
}

// Buttons of onboarding: the theme's, with a press that gives a little.

func onbPress(b ui.Element) ui.Element {
	s := float32(1)
	if b.Pressed() {
		s = .96
	}
	return b.TransformOrigin(.5, .5).AnimatePose(ui.Pose{Scale: s}, onbQuick)
}

func onbPrimary(c *ui.Context, label, text string, small bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Row().AlignItems(ui.Center).Justify(ui.Center).Gap(6).Height(36).Padding(0, 14).Radius(8).Shrink(0).Background(t.Accent).TextColor(t.AccentText).Transition(ui.ElementTransition{Duration: 150 * time.Millisecond, Colors: true})
	if small {
		b.Height(30).Padding(0, 10)
	}
	if b.Hovered() {
		b.Background(t.AccentHover)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		tx := ui.Text(c, text).FontWeight(600).MaxLines(1)
		if small {
			tx.FontSize(12)
		}
	})
	return onbPress(b)
}

func onbSecondary(c *ui.Context, label, text string, small bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Row().AlignItems(ui.Center).Justify(ui.Center).Gap(6).Height(36).Padding(0, 14).Radius(8).Shrink(0).Border(1, t.Border).Background(t.Surface).TextColor(t.Text).Transition(ui.ElementTransition{Duration: 150 * time.Millisecond, Colors: true})
	if small {
		b.Height(30).Padding(0, 10)
	}
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		tx := ui.Text(c, text).MaxLines(1)
		if small {
			tx.FontSize(12)
		}
	})
	return onbPress(b)
}

func onbLink(c *ui.Context, label, text string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Row().AlignItems(ui.Center).Height(28).Padding(0, 8).Radius(7).Shrink(0).TextColor(t.TextMuted).Transition(ui.ElementTransition{Duration: 150 * time.Millisecond, Colors: true})
	if b.Hovered() {
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.FocusRing(b.FocusVisible()).Children(func() { ui.Text(c, text).FontSize(12) })
	return onbPress(b)
}
