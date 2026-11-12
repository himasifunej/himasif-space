package appui

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/api"
)

// splashHarness is a launch with the opening animation on.
func splashHarness(t *testing.T, saved string) (*harness, *fakeAPI, *int) {
	t.Helper()
	f := newFake(api.AccountPerson)
	if saved != "" {
		f.valid[saved] = true
	}
	h := newHarness(t, f, saved)
	h.app.Splash = true
	shown := 0
	h.app.FirstScreen = func() { shown++ }
	return h, f, &shown
}

// A quick launch still gets the whole animation, and not a moment more.
func TestSplashRunsItsLengthThenGoes(t *testing.T) {
	h, _, shown := splashHarness(t, "")
	h.app.Start()
	h.settle() // the login is ready at once
	h.want("by HIMASIF ASTRA 25/26")
	h.want("Menyambungkan…")
	h.dontWant("Masuk")
	h.tt.Advance(splashDur - 100*time.Millisecond)
	h.want("by HIMASIF ASTRA 25/26")
	if *shown != 0 {
		t.Error("FirstScreen told while the animation runs")
	}
	h.tt.Advance(100 * time.Millisecond)
	h.tt.Frame()
	h.want("Masuk")
	h.dontWant("by HIMASIF ASTRA 25/26")
	if *shown != 1 {
		t.Errorf("FirstScreen told %d times", *shown)
	}
	// It never comes back in this launch, not even on Coba lagi.
	h.app.Start()
	h.tt.Frame()
	h.dontWant("by HIMASIF ASTRA 25/26")
	h.settle()
}

// A slow launch loops the track past the animation's end, and the app
// shows the moment it is ready.
func TestSplashLoopsUntilReady(t *testing.T) {
	h, _, _ := splashHarness(t, "tok")
	h.app.Start()
	h.tt.Frame()
	// The session check has not come back: no settle.
	h.tt.Advance(splashDur + 3*splashLoop + 700*time.Millisecond)
	h.want("by HIMASIF ASTRA 25/26")
	if h.app.splash.done {
		t.Fatal("splash over before the app was ready")
	}
	h.settle()
	if h.app.screen != screenShell {
		t.Fatalf("screen %d", h.app.screen)
	}
	h.dontWant("by HIMASIF ASTRA 25/26")
	h.want("Untukku")
}

// While the desktop asks for less motion, nothing moves, and a ready app
// shows at once.
func TestSplashReduceMotion(t *testing.T) {
	h, _, _ := splashHarness(t, "tok")
	h.tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	h.app.Start()
	h.tt.Frame()
	h.want("by HIMASIF ASTRA 25/26")
	if h.tt.FramePending() {
		t.Error("the still splash asks for frames")
	}
	h.settle()
	h.want("Untukku")
	h.dontWant("by HIMASIF ASTRA 25/26")
}

// The launch after an update says so.
func TestSplashAfterUpdate(t *testing.T) {
	h, _, _ := splashHarness(t, "")
	h.app.WhatsNew = &WhatsNew{Version: "0.4.6-rc.5"}
	h.app.Start()
	h.tt.Frame()
	h.tt.Advance(time.Second)
	h.want("Memperbarui ke ")
	h.want("0.4.6-rc.5")
	h.want(" · memulai ulang")
	h.dontWant("Menyambungkan…")
	h.settle()
}

// The timeline is canvas design 3's: Ace in at 0.4 s, the track full at
// 3.6 s, everything in place at 4.8 s; the loop starts where it ends.
func TestSplashTimeline(t *testing.T) {
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	start := splashAt(0)
	if start.ace != 0 || start.logo != 0 || start.credit != 0 || start.width != 120 || start.groupY != -56 {
		t.Errorf("start %+v", start)
	}
	full := splashAt(ms(3600))
	if full.width != 320 || full.fill != 320 {
		t.Errorf("at 3.6 s %+v", full)
	}
	if mid := splashAt(ms(2600)); mid.fill <= 0 || mid.fill >= 320 || mid.eyesX < 14 {
		t.Errorf("at 2.6 s %+v", mid)
	}
	if b := splashAt(ms(980)); b.eyesY > .2 {
		t.Errorf("no blink at 0.98 s: %v", b.eyesY)
	}
	end := splashAt(splashDur - time.Millisecond)
	loop := splashAt(splashDur)
	for _, f := range []splashFrame{end, loop} {
		if math.Abs(float64(f.width-120)) > .5 || f.logo < .99 || f.credit < .99 || math.Abs(float64(f.groupY)) > .01 {
			t.Errorf("settled frame %+v", f)
		}
	}
	if math.Abs(float64(end.eyesX-loop.eyesX)) > .5 {
		t.Errorf("the loop jumps: %v to %v", end.eyesX, loop.eyesX)
	}
	if run := splashAt(splashDur + ms(1600)); run.width != 320 || run.fill <= 0 {
		t.Errorf("loop run %+v", run)
	}
}

// The mark is the kit's: the settled outline is tiyoouw-symbol-*.svg's
// path, point for point.
func TestMarkGeometryIsTheKits(t *testing.T) {
	const kit = "M40 54L120 32.56L120 54L200 75.44L200 207.44L138 190.82L138 128.82L168 136.86L168 106.86L72 81.14L72 111.14L102 119.18L102 181.18L40 197.79Z"
	n := func(x float64) string {
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", math.Round(x*100)/100), "0"), ".")
	}
	var b strings.Builder
	for i, p := range markPath {
		if i == 0 {
			b.WriteString("M")
		} else {
			b.WriteString("L")
		}
		b.WriteString(n(p[0]) + " " + n(p[1]))
	}
	b.WriteString("Z")
	if b.String() != kit {
		t.Errorf("path\n%s\nwant\n%s", b.String(), kit)
	}
	// The intro ends on the settled pieces.
	f, alpha := markIntro(markIntroDur)
	if alpha != 1 {
		t.Errorf("alpha %v", alpha)
	}
	for i, p := range f.R.o {
		if math.Abs(p[0]-markFinal.R.o[i][0]) > 1e-6 || math.Abs(p[1]-markFinal.R.o[i][1]) > 1e-6 {
			t.Errorf("intro ends off the mark at %d: %v", i, p)
		}
	}
}

// Hovered, the mark parts, turns into eyes and follows the pointer; left,
// it springs back.
func TestMarkHoverEyes(t *testing.T) {
	m := &tiyoouwMark{}
	at := time.Unix(1000, 0)
	m.rest()
	m.wake(at)
	m.tx = 1
	var f markFrame
	for i := 0; i <= 80; i++ { // 1.33 s at 60 a second
		f, _, _, _ = m.step(at.Add(time.Duration(i) * time.Second / 60))
	}
	if m.mode != markEyesOn {
		t.Fatalf("mode %d after a hover", m.mode)
	}
	if m.look < .5 {
		t.Errorf("eyes look %v, want toward the pointer at the right", m.look)
	}
	if f.L.o[0][0] >= markFinal.L.o[0][0] {
		t.Errorf("left eye did not part from the fold: %v", f.L.o[0])
	}
	leave := at.Add(80 * time.Second / 60)
	m.sleep(leave)
	moving := true
	for i := 1; moving && i < 120; i++ {
		_, _, _, moving = m.step(leave.Add(time.Duration(i) * time.Second / 60))
	}
	if moving || m.mode != markIdle {
		t.Errorf("still moving after leave, mode %d", m.mode)
	}
}

// Tentang's "Dibuat oleh" opens Kredit: the owner, the links, the
// contributors of contributors.json; back goes to Tentang.
func TestKreditFromTentang(t *testing.T) {
	h := redesignHarness(t)
	h.openSettings("Tentang")
	h.want("Dibuat oleh")
	h.want("Tiyoouw dan kontributor")
	h.click("Dibuat oleh: Tiyoouw dan kontributor")
	h.tt.Frame()
	if h.app.chrome.settingsTab != "Kredit" {
		t.Fatalf("tab %q", h.app.chrome.settingsTab)
	}
	for _, s := range []string{kreditOwner, "tiyoouw.com", "github.com/Tiyouw", "Kontributor", "Space by HIMASIF ASTRA 25/26",
		"Ahimsa Jenar Bramsaifstyo Kusuma (Tiyoouw)", "Pengembang utama · Mediatek HIMASIF 25/26", "CONTRIBUTING.md di GitHub"} {
		h.want(s)
	}
	captureNativeUI(t, h, "kredit")
	h.click("Kembali ke Tentang")
	h.tt.Frame()
	h.want("Riwayat perubahan")
	h.dontWant(kreditOwner)
}

// The copy the app embeds is the repository's contributors.json.
func TestContributorsEmbedded(t *testing.T) {
	root, err := os.ReadFile("../../contributors.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(root, contributorsJSON) {
		t.Fatal("internal/appui/contributors.json differs from the root contributors.json: copy it over")
	}
	if len(Contributors.Contributors) == 0 || Contributors.Contributors[0].ID != "tiyouw" {
		t.Errorf("contributors %+v", Contributors)
	}
	if got := parseContributors([]byte(`{"format":2,"contributors":[{"id":"a","name":"A","role":"r","kind":["code"]}]}`)); len(got.Contributors) != 0 {
		t.Error("format 2 read as format 1")
	}
	if got := parseContributors([]byte(`{"format":1,"contributors":[{"id":"a","name":"A","role":"r","kind":["code","new-kind"],"extra":true}]}`)); len(got.Contributors) != 1 {
		t.Error("an unknown field or kind dropped the entry")
	}
}

// SPACE_UI_CAPTURE_DIR keeps frames of the opening animation and of
// Kredit's mark: in the intro, settled, and awake under the pointer.
func TestSplashKreditFrames(t *testing.T) {
	if os.Getenv("SPACE_UI_CAPTURE_DIR") == "" {
		t.Skip("set SPACE_UI_CAPTURE_DIR for frames")
	}
	_ = RegisterFonts()
	for _, dark := range []bool{true, false} {
		theme := "light"
		if dark {
			theme = "dark"
		}
		h, _, _ := splashHarness(t, "tok")
		h.tt.SetSize(1280, 800)
		h.tt.SetDark(dark)
		h.app.chrome.preferences.Theme = theme
		h.app.Start()
		h.tt.Frame()
		last := time.Duration(0)
		for _, at := range []int{300, 900, 1400, 2400, 3400, 4000, 4800, 6800} {
			h.tt.Advance(time.Duration(at)*time.Millisecond - last)
			last = time.Duration(at) * time.Millisecond
			captureNativeUI(t, h, fmt.Sprintf("splash-%s-%04dms", theme, at))
		}
		h.settle()

		k := redesignHarness(t)
		k.tt.SetDark(dark)
		k.app.chrome.preferences.Theme = theme
		k.openSettings("Tentang")
		captureNativeUI(t, k, "tentang-"+theme)
		k.click("Dibuat oleh: Tiyoouw dan kontributor")
		k.tt.Advance(900 * time.Millisecond)
		captureNativeUI(t, k, "kredit-"+theme+"-intro")
		k.tt.Advance(3 * time.Second)
		captureNativeUI(t, k, "kredit-"+theme+"-settled")
		r, ok := k.tt.Find("Logo Tiyoouw, klik untuk memutar ulang")
		if !ok {
			t.Fatal("no mark")
		}
		k.tt.Move(r.X+r.W*.85, r.Y+r.H*.4)
		k.tt.Advance(1200 * time.Millisecond)
		captureNativeUI(t, k, "kredit-"+theme+"-hover-eyes")
		k.tt.Move(r.X-200, r.Y)
		k.tt.Advance(time.Second)
		captureNativeUI(t, k, "kredit-"+theme+"-after-leave")
	}
}
