//go:build !js

package island

import (
	"testing"
	"time"
)

// The canvas's stages: 2,5× counted back from 15 is 1, 2–5, 6–14, 15+;
// the other ratios and Atur sendiri keep 1 < heavy < sleepy < top ≤ 15.
func TestTiredThresholds(t *testing.T) {
	for _, tc := range []struct {
		ratio              string
		top, heavy, sleepy int
		want               [4]int
	}{
		{"2.5", 15, 0, 0, [4]int{1, 2, 6, 15}},
		{"2", 15, 0, 0, [4]int{1, 4, 8, 15}},
		{"1.5", 15, 0, 0, [4]int{1, 7, 10, 15}},
		{"2.5", 10, 0, 0, [4]int{1, 2, 4, 10}},
		{"2.5", 5, 0, 0, [4]int{1, 2, 3, 5}},
		{"1.5", 5, 0, 0, [4]int{1, 2, 3, 5}},
		{"", 40, 0, 0, [4]int{1, 2, 6, 15}}, // owner: "maksimal 15"
		{"2", 1, 0, 0, [4]int{1, 2, 3, 5}},
		{RatioCustom, 15, 3, 9, [4]int{1, 3, 9, 15}},
		{RatioCustom, 15, 1, 30, [4]int{1, 2, 14, 15}},
		{RatioCustom, 8, 7, 7, [4]int{1, 6, 7, 8}},
	} {
		if got := Thresholds(tc.ratio, tc.top, tc.heavy, tc.sleepy); got != tc.want {
			t.Errorf("%q top %d (%d, %d): %v, want %v", tc.ratio, tc.top, tc.heavy, tc.sleepy, got, tc.want)
		}
	}
	th := Thresholds("2.5", 15, 0, 0)
	for n, want := range map[int]int{0: StageFresh, 1: StageNews, 2: StageHeavy, 5: StageHeavy, 6: StageSleepy, 14: StageSleepy, 15: StageGrumpy, 99: StageGrumpy} {
		if got := StageOf(n, th); got != want {
			t.Errorf("%d unread: stage %d, want %d", n, got, want)
		}
	}
}

// Each stage is more tired than the one before: lids lower, the body
// lower and flatter, the ink and the "+N" warmer, all inside the pill
// (TiredPose's motions included).
func TestTiredPosesGrowTired(t *testing.T) {
	prev := TiredPose(StageFresh, 0, true)
	if prev != RestPose() {
		t.Fatalf("Segar is not Ace at rest: %+v", prev)
	}
	for s := StageNews; s <= StageGrumpy; s++ {
		p := TiredPose(s, 0, true)
		if p.Warm < prev.Warm || p.Heavy < prev.Heavy {
			t.Errorf("stage %d is less warm than %d: %+v", s, s-1, p)
		}
		if s == StageSleepy && (p.Lid <= prev.Lid || p.Y <= prev.Y || p.SY >= prev.SY) {
			t.Errorf("Ngantuk does not sag below Mulai berat: %+v", p)
		}
		prev = p
	}
	if g := TiredPose(StageGrumpy, 0, true); g.Angle == 0 || g.Warm != 1 {
		t.Errorf("Cemberut has no slant or full warmth: %+v", g)
	}
	for s := StageFresh; s <= StageGrumpy; s++ {
		for ms := 0; ms < 6000; ms += 10 {
			p := TiredPose(s, time.Duration(ms)*time.Millisecond, false)
			if p.Lid < 0 || p.Lid > .9 || p.Close < 0 || p.Close > 1 {
				t.Fatalf("stage %d at %d ms: eyes %+v", s, ms, p)
			}
			// Sagging stays a few DIP: the window has room under the pill.
			if top := 32*(1-p.SY) + p.Y; top < -.01 || top > 8 {
				t.Fatalf("stage %d at %d ms: pill's top at %v", s, ms, top)
			}
		}
	}
}

func TestTiredStagesBlendAndRecover(t *testing.T) {
	th := Thresholds("2.5", 15, 0, 0)
	var tr Tired
	t0 := time.Unix(1000, 0)
	at := func(ms int) time.Time { return t0.Add(time.Duration(ms) * time.Millisecond) }
	p, n := tr.Frame(at(0), 0, th, true, true, true, false)
	if p != RestPose() || n != 0 || tr.Next(at(0), true, false) != 0 {
		t.Fatalf("fresh Ace is not at rest or asks for frames: %+v", p)
	}
	// 12 unread: Ngantuk, blending in over 450 ms.
	p, _ = tr.Frame(at(16), 12, th, true, true, true, false)
	if tr.Stage() != StageSleepy || p.Lid >= TiredPose(StageSleepy, 0, true).Lid {
		t.Fatalf("Ngantuk jumped in: %+v", p)
	}
	if tr.Next(at(16), true, false) != 16*time.Millisecond {
		t.Fatal("a blend asks for no frames")
	}
	p, n = tr.Frame(at(600), 12, th, true, true, false, false)
	if n != 12 || p.Lid < .5 {
		t.Fatalf("Ngantuk after its blend: %+v, +%d", p, n)
	}
	// Cleared: the badge leaves with the count it had, Ace shakes it
	// off and is fresh after TiredRecovery.
	p, n = tr.Frame(at(700), 0, th, true, true, false, false)
	if n != 12 || p.Out != 0 {
		t.Fatalf("cleared: +%d, out %v", n, p.Out)
	}
	p, n = tr.Frame(at(1000), 0, th, true, true, false, false)
	if p.Out != 1 || p.Close == 0 {
		t.Fatalf("recovering: %+v (+%d)", p, n)
	}
	p, n = tr.Frame(at(700+1400+16), 0, th, true, true, false, false)
	if p != RestPose() || n != 0 || tr.Next(at(2200), false, false) != 0 {
		t.Fatalf("after the recovery: %+v, +%d", p, n)
	}
}

// Reduced motion: each stage at once and still, no recovery; off, or not
// at rest, Ace is as ever.
func TestTiredReducedOffAndAway(t *testing.T) {
	th := Thresholds("2.5", 15, 0, 0)
	var tr Tired
	now := time.Unix(1000, 0)
	p, _ := tr.Frame(now, 20, th, true, true, true, true)
	if p != TiredPose(StageGrumpy, 0, true) || tr.Next(now, true, true) != 0 {
		t.Fatalf("reduced Cemberut: %+v", p)
	}
	p, n := tr.Frame(now.Add(time.Second), 0, th, true, true, true, true)
	if p != RestPose() || n != 0 {
		t.Fatalf("reduced clear: %+v, +%d", p, n)
	}
	if p, n = tr.Frame(now.Add(2*time.Second), 20, th, false, true, true, false); p != RestPose() || n != 20 {
		t.Fatalf("off: %+v, +%d", p, n)
	}
	if p, _ = tr.Frame(now.Add(3*time.Second), 20, th, true, false, true, false); p != RestPose() {
		t.Fatalf("in the details: %+v", p)
	}
}

// The inbox read in the details (or the bell) while they are open: back
// at rest, Ace still shakes it off.
func TestTiredRecoversAfterTheDetails(t *testing.T) {
	th := Thresholds("2.5", 15, 0, 0)
	var tr Tired
	t0 := time.Unix(1000, 0)
	tr.Frame(t0, 9, th, true, true, true, false)
	tr.Frame(t0.Add(500*time.Millisecond), 9, th, true, false, true, false)
	tr.Frame(t0.Add(time.Second), 0, th, true, false, true, false)
	p, n := tr.Frame(t0.Add(1100*time.Millisecond), 0, th, true, true, true, false)
	if n != 0 || tr.Next(t0.Add(1100*time.Millisecond), true, false) != 16*time.Millisecond {
		t.Fatalf("no recovery after the details: %+v", p)
	}
	p, _ = tr.Frame(t0.Add(1100*time.Millisecond+TiredRecovery), 0, th, true, true, true, false)
	if p != RestPose() {
		t.Fatalf("after: %+v", p)
	}
}

// Asleep (nobody about), a tired Ace holds its pose and asks for no
// frames once the motion under way ends: idle CPU stays as it was.
func TestTiredAsleepHoldsStill(t *testing.T) {
	th := Thresholds("2.5", 15, 0, 0)
	var tr Tired
	now := time.Unix(1000, 0)
	tr.Frame(now, 12, th, true, true, false, false)
	now = now.Add(time.Second)
	p, _ := tr.Frame(now, 12, th, true, true, false, false)
	if p != TiredPose(StageSleepy, 0, true) || tr.Next(now, false, false) != 0 {
		t.Fatalf("asleep Ngantuk moves: %+v", p)
	}
	if tr.Next(now, true, false) == 0 {
		t.Fatal("awake, Ngantuk never moves")
	}
}
