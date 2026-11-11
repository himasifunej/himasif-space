package appui

import (
	"context"
	"image"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// onbFake is the fake server with onboarding: marks kept as the server
// keeps them, and the profile.
type onbFake struct {
	*fakeAPI
	mu      sync.Mutex
	data    api.Onboarding
	marks   []string
	profile []string
	gets    int
}

func (f *onbFake) Onboarding(_ context.Context, _ id.ID) (api.Onboarding, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gets++
	return f.copy(), nil
}

func (f *onbFake) MarkOnboarding(_ context.Context, m api.OnboardingMark) (api.Onboarding, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.marks = append(f.marks, m.Key)
	if _, ok := f.data.Marks[m.Key]; !ok {
		f.data.Marks[m.Key] = time.Now()
	}
	return f.copy(), nil
}

func (f *onbFake) UpdateProfile(_ context.Context, p api.ProfileUpdate) (api.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.profile = append(f.profile, p.DisplayName)
	f.data.Marks[api.OnboardProfile] = time.Now()
	u := f.fakeAPI.user
	u.DisplayName = p.DisplayName
	return u, nil
}

func (f *onbFake) copy() api.Onboarding {
	out := api.Onboarding{Marks: map[string]time.Time{}, WhatsAppLinked: f.data.WhatsAppLinked}
	for k, v := range f.data.Marks {
		out.Marks[k] = v
	}
	return out
}

func (f *onbFake) marked(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, k := range f.marks {
		if k == key {
			return true
		}
	}
	return false
}

// onbHarness signs a new member of Media dan Teknologi in, with the role
// given, on its Space's Beranda.
func onbHarness(t *testing.T, role string, marks ...string) (*harness, *onbFake) {
	t.Helper()
	h := newChromeHarness(t, nil)
	f := &onbFake{fakeAPI: h.api, data: api.Onboarding{Marks: map[string]time.Time{}}}
	for _, m := range marks {
		f.data.Marks[m] = time.Now()
	}
	h.app.api = f
	h.app.onb = onboardingState{}
	sp := testAuthorizedSpace(h.app, h.api.divisions[0])
	h.app.access.spaces[0].Roles[0].Roles = []string{role}
	// Members own their tasks, so Bot WhatsApp (Tautkan nomormu) shows.
	h.app.access.spaces[0].Roles[0].Capabilities = append(h.app.access.spaces[0].Roles[0].Capabilities, api.CapTasksOwn)
	h.app.chrome.tab = defaultSpaceTab(sp)
	h.app.navigate(PathSpaces)
	h.settle()
	h.settle()
	return h, f
}

func TestOnboardingWelcomeThenChecklist(t *testing.T) {
	h, f := onbHarness(t, api.RoleMember)
	h.want("UNDANGAN DITERIMA")
	h.want("Kamu sudah bergabung di Media dan Teknologi")
	h.want("Checklist \"Langkah pertama\" menunggu di Beranda.")
	h.click("Nanti, ke Beranda")
	h.settle()
	if !f.marked(api.OnboardWelcome) {
		t.Fatal("welcome not recorded")
	}
	h.dontWant("UNDANGAN DITERIMA")
	h.want("Langkah pertama")
	h.want("0 dari 4")
	for _, s := range []string{"Tautkan WhatsApp", "Lengkapi profil", "Lihat tugasmu", "Kenali tab Space"} {
		h.want(s)
	}
	// Linking WhatsApp elsewhere ticks the step on the next refresh.
	f.mu.Lock()
	f.data.WhatsAppLinked = true
	f.mu.Unlock()
	h.tt.Advance(onboardingRefresh + time.Second)
	h.settle()
	h.settle()
	h.want("1 dari 4")
	// Opening the board ticks "Lihat tugasmu".
	h.click("Buka board: Lihat tugasmu")
	h.settle()
	if !f.marked(api.OnboardTasks) || h.app.chrome.tab != "Board tugas" {
		t.Fatalf("board step: marks %v tab %q", f.marks, h.app.chrome.tab)
	}
	h.click("Beranda")
	h.settle()
	h.want("2 dari 4")
}

func TestOnboardingTourByRoleSkipAndReplay(t *testing.T) {
	h, f := onbHarness(t, api.RoleMember)
	h.click("Mulai tur · 1 menit")
	h.settle()
	h.want("TUR UNTUK ANGGOTA")
	h.want("Sidebar")
	h.want("1/5")
	if _, ok := h.app.onb.targets["sidebar"]; !ok {
		t.Fatal("sidebar target not measured")
	}
	h.click("Lanjut tur")
	h.want("2/5")
	// A step on a tab asks to press it first, and shows the page after.
	h.want("Tekan tab Board tugas yang disorot untuk membukanya.")
	h.dontWant("Lanjut tur")
	// The dim keeps the other tabs from presses.
	if r, ok := h.tt.Find("Agenda"); ok {
		h.tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
		h.tt.Advance(300 * time.Millisecond)
	}
	if h.app.chrome.tab != "Beranda" {
		t.Fatalf("a press on the dim opened %q", h.app.chrome.tab)
	}
	pressTourTab(h, "Board tugas")
	h.want("Tugas yang diberikan kepadamu ada di sini. Pindahkan kartu ke Dikerjakan saat mulai.")
	if _, ok := h.app.tourPage(); !ok {
		t.Fatal("the Board page is not lit")
	}
	h.click("Langkah sebelumnya")
	h.want("1/5")
	h.click("Lanjut tur")
	h.click("Lanjut tur")
	h.want("3/5")
	pressTourTab(h, "Catatan")
	h.click("Lanjut tur")
	h.want("4/5")
	// The tab can also be opened from the card.
	h.click("Bukakan tab Bot WhatsApp")
	if h.app.chrome.tab != "Bot WhatsApp" {
		t.Fatalf("tab %q", h.app.chrome.tab)
	}
	h.click("Lanjut tur")
	h.want("5/5")
	h.want("Tombol ?")
	h.click("Selesai tur")
	h.settle()
	if h.app.onb.tour.open || !f.marked(api.OnboardTour) {
		t.Fatalf("tour not finished: %+v %v", h.app.onb.tour, f.marks)
	}
	h.dontWant("TUR UNTUK ANGGOTA")
	// Replay from the profile menu.
	h.click("Menu profil")
	h.click("Ulangi tur")
	h.want("TUR UNTUK ANGGOTA")
	h.tt.Key(0, ui.KeyEscape)
	h.tt.Advance(300 * time.Millisecond)
	h.dontWant("TUR UNTUK ANGGOTA")
}

// pressTourTab presses tab title where the tour's spotlight shows it,
// through the hole in the dim, as a user would.
func pressTourTab(h *harness, title string) {
	h.t.Helper()
	h.tt.Advance(600 * time.Millisecond)
	r, ok := h.app.onb.targets["tab:"+title]
	if !ok {
		h.t.Fatalf("tab %q not measured", title)
	}
	h.tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	h.tt.Advance(600 * time.Millisecond)
	if h.app.chrome.tab != title {
		h.t.Fatalf("pressing %q opened %q", title, h.app.chrome.tab)
	}
}

func TestOnboardingTourForKadiv(t *testing.T) {
	h, _ := onbHarness(t, api.RoleKadiv, api.OnboardWelcome)
	h.app.startTour()
	h.tt.Advance(300 * time.Millisecond)
	h.want("TUR UNTUK KADIV")
	h.click("Lanjut tur")
	for _, tab := range []string{"Board tugas", "Catatan"} {
		pressTourTab(h, tab)
		h.click("Lanjut tur")
	}
	pressTourTab(h, "Anggota")
	h.want("Undang lewat NIM, atur role (Anggota atau 45), dan lihat beban tiap orang.")
	// The kadiv also sees the handover card on Beranda.
	h.tt.Key(0, ui.KeyEscape)
	h.tt.Advance(300 * time.Millisecond)
	h.settle()
	h.want("Serah terima divisi")
	h.want("Undang anggota lewat NIM")
}

func TestOnboardingProfileAndFinish(t *testing.T) {
	h, f := onbHarness(t, api.RoleMember, api.OnboardWelcome, api.OnboardTasks, api.OnboardTour)
	f.data.WhatsAppLinked = true
	h.app.onb.data.WhatsAppLinked = true
	h.tt.Advance(time.Second)
	h.want("3 dari 4")
	h.click("Buka profil: Lengkapi profil")
	h.want("Nama tampilan")
	h.app.onb.profileName = "  Ani   W. "
	h.click("Simpan profil")
	h.settle()
	h.settle()
	if len(f.profile) != 1 || f.profile[0] != "Ani W." || h.app.user.DisplayName != "Ani W." {
		t.Fatalf("profile %v, user %q", f.profile, h.app.user.DisplayName)
	}
	h.want("Langkah pertama selesai")
	if h.app.onb.celebrate.IsZero() {
		t.Fatal("finishing the last step did not celebrate")
	}
	h.click("Sembunyikan checklist")
	h.settle()
	if !f.marked(api.OnboardChecklistHidden) {
		t.Fatal("hide not recorded")
	}
	h.tt.Advance(time.Second)
	h.dontWant("Langkah pertama selesai")
}

func TestOnboardingExistingAccountSeesNothing(t *testing.T) {
	h, _ := onbHarness(t, api.RoleMember, api.OnboardWelcome, api.OnboardTour, api.OnboardChecklistHidden, api.OnboardHandover)
	h.dontWant("UNDANGAN DITERIMA")
	h.dontWant("Langkah pertama")
	h.want("Panduan untuk halaman ini")
}

func TestGuidePanelAndPage(t *testing.T) {
	h, _ := onbHarness(t, api.RoleMember, api.OnboardWelcome, api.OnboardChecklistHidden)
	h.click("Board tugas")
	h.click("Panduan untuk halaman ini")
	h.settle()
	h.want("Panduan · Board tugas")
	h.want("Membaca dan memindah kartu tugas")
	h.want("Perlu tinjauan: hasil sudah diajukan dan menunggu dicek kadiv atau 45.")
	h.click("Tutup panduan")
	h.tt.Advance(400 * time.Millisecond)
	h.dontWant("Panduan · Board tugas")
	// The page: topics of the role, search, another role.
	h.click("Panduan")
	h.settle()
	h.want("Topik untuk Anggota. Ganti role untuk melihat panduan peran lain.")
	h.want("Menautkan WhatsApp ke Space")
	h.dontWant("Checklist kadiv baru")
	h.click("Kadiv")
	h.want("Checklist kadiv baru")
	h.want("Pergantian periode")
	h.click("Cari panduan")
	h.tt.Type("periode")
	h.tt.Advance(300 * time.Millisecond)
	h.dontWant("Membuat dan membagi tugas")
	h.click("Buka panduan Pergantian periode")
	h.want("Urutan hari H")
	h.want("Penutupan tidak bisa dibatalkan.")
	h.click("Kembali ke semua panduan")
	h.want("Panduan")
}

func TestGuidedEmptyStates(t *testing.T) {
	h, _ := onbHarness(t, api.RoleMember, api.OnboardWelcome, api.OnboardChecklistHidden)
	a := h.app
	view := func(fn func(c *ui.Context)) *ui.Tester {
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Fill().Children(func() { fn(c) })
		}, 1100, 800)
		tt.Advance(time.Second)
		return tt
	}
	tt := view(func(c *ui.Context) { a.notesEmptyGuide(c, true) })
	for _, s := range []string{"Catatan pertama biasanya notulen rapat", "Buat catatan rapat", "Cara membaca catatan", "Agenda + durasi", "Baca: Menjadi notulis rapat · 3 mnt"} {
		if !tt.HasText(s) {
			t.Errorf("notes: no %q in %q", s, tt.Texts())
		}
	}
	tt = view(func(c *ui.Context) { a.membersEmptyGuide(c) })
	for _, s := range []string{"Undang anggota lewat NIM", "NIM@mail.unej.ac.id", "Cara mengundang"} {
		if !tt.HasText(s) {
			t.Errorf("members: no %q in %q", s, tt.Texts())
		}
	}
	// A member's empty board says who shares tasks; a manager's offers
	// the first task.
	tt = view(func(c *ui.Context) { a.boardEmptyGuide(c) })
	if !tt.HasText("Belum ada tugas di divisi ini") || tt.HasText("Buat tugas pertama") {
		t.Errorf("member board: %q", tt.Texts())
	}
}

// TestOnboardingNativeCapture writes the review's screenshots when
// SPACE_UI_CAPTURE_DIR is set: the tour on its second step, the "?" panel,
// and a short GIF of the motion (the spotlight gliding to the next target,
// then the panel sliding in).
func TestOnboardingNativeCapture(t *testing.T) {
	dir := os.Getenv("SPACE_UI_CAPTURE_DIR")
	if dir == "" {
		t.Skip("SPACE_UI_CAPTURE_DIR is not set")
	}
	h, f := onbHarness(t, api.RoleMember, api.OnboardWelcome)
	f.data.WhatsAppLinked = true
	h.app.onb.data.WhatsAppLinked = true
	h.app.markOnboarding(api.OnboardTasks)
	h.settle()
	h.tt.SetSize(1440, 900)
	h.app.startTour()
	h.tt.Advance(time.Second)
	h.tt.Advance(time.Second)

	var frames []image.Image
	shoot := func(n int, step time.Duration) {
		for i := 0; i < n; i++ {
			h.tt.Advance(step)
			frames = append(frames, h.tt.Image())
		}
	}
	shoot(3, 120*time.Millisecond)
	h.click("Lanjut tur")
	frames = frames[:len(frames):len(frames)]
	shoot(9, 35*time.Millisecond)
	shoot(4, 150*time.Millisecond)
	captureNativeUI(t, h, "onboarding-tur-desktop")
	h.app.endTour()
	h.settle()
	h.click("Board tugas")
	shoot(3, 120*time.Millisecond)
	if err := h.tt.Click("Panduan untuk halaman ini"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	shoot(9, 35*time.Millisecond)
	shoot(5, 150*time.Millisecond)
	captureNativeUI(t, h, "onboarding-panduan-desktop")
	writeGIF(t, filepath.Join(dir, "onboarding-motion.gif"), frames, 2)
}

// writeGIF encodes frames, halved in size by every other pixel, at 8 fps
// steps of 50 ms (delay in hundredths: 5).
func writeGIF(t *testing.T, name string, frames []image.Image, scale int) {
	t.Helper()
	out := &gif.GIF{}
	for _, fr := range frames {
		b := fr.Bounds()
		small := image.NewRGBA(image.Rect(0, 0, b.Dx()/scale, b.Dy()/scale))
		for y := 0; y < small.Bounds().Dy(); y++ {
			for x := 0; x < small.Bounds().Dx(); x++ {
				small.Set(x, y, fr.At(b.Min.X+x*scale, b.Min.Y+y*scale))
			}
		}
		p := image.NewPaletted(small.Bounds(), palette.Plan9)
		draw.FloydSteinberg.Draw(p, small.Bounds(), small, image.Point{})
		out.Image = append(out.Image, p)
		out.Delay = append(out.Delay, 5)
	}
	out.Delay[len(out.Delay)-1] = 120
	fh, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	if err := gif.EncodeAll(fh, out); err != nil {
		t.Fatal(err)
	}
}

// fakeNotifier is a device's notification layer whose permission the test
// sets; RequestPermission grants it, as a user clicking Allow.
type fakeNotifier struct {
	perm      string
	hint      string
	requested int
	shown     []Notice
}

func (n *fakeNotifier) Permission() string { return n.perm }
func (n *fakeNotifier) RequestPermission() {
	n.requested++
	n.perm = BrowserPermissionGranted
}
func (n *fakeNotifier) DeniedHint() string { return n.hint }
func (n *fakeNotifier) Present(x Notice) error {
	n.shown = append(n.shown, x)
	return nil
}

func TestOnboardingNotificationStep(t *testing.T) {
	h, _ := onbHarness(t, api.RoleMember, api.OnboardWelcome)
	n := &fakeNotifier{perm: BrowserPermissionDefault}
	h.app.Notifier = n
	h.tt.Advance(300 * time.Millisecond)
	h.want("Izinkan notifikasi")
	h.want("0 dari 5")
	h.click("Izinkan: Izinkan notifikasi")
	h.tt.Advance(300 * time.Millisecond)
	if n.requested != 1 {
		t.Fatalf("requested %d", n.requested)
	}
	// Granted: one test notice at once, and the step ticks itself.
	if len(n.shown) != 1 || n.shown[0].Kind != NoticeTest || n.shown[0].Title != "Notifikasi aktif ✓" {
		t.Fatalf("test notice %+v", n.shown)
	}
	h.want("1 dari 5")
	h.tt.Advance(time.Second)
	if len(n.shown) != 1 {
		t.Fatalf("test notice repeated: %d", len(n.shown))
	}
	// Blocked in the browser later: the hint shows, the button opens the
	// device settings; allowed again in site settings: a new test notice.
	n.perm = BrowserPermissionDenied
	n.hint = "Klik ikon gembok di samping alamat situs"
	h.tt.Frame() // the browser's change redraws the window
	h.want("Klik ikon gembok di samping alamat situs")
	h.click("Lihat caranya: Izinkan notifikasi")
	if h.app.router.Path() != PathPengaturan || h.app.chrome.settingsTab != "Perangkat" {
		t.Fatalf("hint button went to %s %q", h.app.router.Path(), h.app.chrome.settingsTab)
	}
	n.perm = BrowserPermissionGranted
	h.tt.Frame()
	if len(n.shown) != 2 {
		t.Fatalf("no test notice after allowing again: %d", len(n.shown))
	}
}

func TestOnboardingNotificationAlreadyAllowed(t *testing.T) {
	h, _ := onbHarness(t, api.RoleMember, api.OnboardWelcome)
	n := &fakeNotifier{perm: BrowserPermissionGranted}
	h.app.Notifier = n
	h.tt.Advance(time.Second)
	if len(n.shown) != 0 {
		t.Fatalf("a permission given before sent a test: %+v", n.shown)
	}
	h.want("1 dari 5")
	// Without notifications on the device, the step is not offered.
	h.app.Notifier = nil
	h.tt.Frame()
	h.want("0 dari 4")
	h.dontWant("Izinkan notifikasi")
}

// The tour points only at what the account sees: a member has no Ping tab,
// so the WhatsApp step points at Bot WhatsApp; without that tab either, the
// step is left out.
func TestOnboardingTourOnlyVisibleTargets(t *testing.T) {
	h, _ := onbHarness(t, api.RoleMember, api.OnboardWelcome, api.OnboardChecklistHidden)
	steps := tourSteps(api.GuideRoleMember, h.app.tourVisibleTab)
	for _, s := range steps {
		if tab, ok := strings.CutPrefix(s.target, "tab:"); ok && !h.app.tourVisibleTab(tab) {
			t.Fatalf("step %q points at hidden tab %q", s.title, tab)
		}
	}
	if len(steps) != 5 || steps[3].target != "tab:Bot WhatsApp" {
		t.Fatalf("member steps %+v", steps)
	}
	h.app.startTour()
	h.app.onb.tour.step = 3
	h.tt.Advance(time.Second)
	h.want("Bot WhatsApp")
	h.want("4/5")
	if _, ok := h.app.onb.targets["tab:Bot WhatsApp"]; !ok {
		t.Fatal("Bot WhatsApp tab not measured for the spotlight")
	}
	h.dontWant("Bagian ini tidak tampil")
	// A member who hid Bot WhatsApp: four steps, none missing.
	sp := h.app.selectedSpace()
	tabs := SpaceTabLayouts{h.app.spaceTabPreferenceKey(sp): {Hidden: []string{"bot"}}}
	h.app.chrome.preferences.SpaceTabs = &tabs
	steps = tourSteps(api.GuideRoleMember, h.app.tourVisibleTab)
	if len(steps) != 4 || steps[3].target != "help" {
		t.Fatalf("steps without the bot tab %+v", steps)
	}
	// A 45 who may ping gets the Ping tab.
	h.app.access.spaces[0].Capabilities = append(h.app.access.spaces[0].Capabilities, api.CapPingSend)
	h.app.access.spaces[0].Roles[0].Capabilities = append(h.app.access.spaces[0].Roles[0].Capabilities, api.CapPingSend)
	if steps = tourSteps(api.GuideRole45, h.app.tourVisibleTab); steps[3].target != "tab:Ping" {
		t.Fatalf("45 steps %+v", steps)
	}
}
