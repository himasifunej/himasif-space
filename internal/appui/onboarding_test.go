package appui

import (
	"context"
	"image"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"os"
	"path/filepath"
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
	h.want("Tugas yang diberikan kepadamu ada di sini. Pindahkan kartu ke Dikerjakan saat mulai.")
	h.click("Langkah sebelumnya")
	h.want("1/5")
	for i := 0; i < 4; i++ {
		h.click("Lanjut tur")
	}
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

func TestOnboardingTourForKadiv(t *testing.T) {
	h, _ := onbHarness(t, api.RoleKadiv, api.OnboardWelcome)
	h.app.startTour()
	h.tt.Advance(300 * time.Millisecond)
	h.want("TUR UNTUK KADIV")
	for i := 0; i < 3; i++ {
		h.click("Lanjut tur")
	}
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
