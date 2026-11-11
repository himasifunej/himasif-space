package appui

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// fakeAce is a device with an island: what Ace said, and its settings.
type fakeAce struct {
	introduced       bool
	said, faces      []string
	hushed, tries    int
	anchor           float64
	alwaysOn         bool
	quiet            bool
	from, to         int
	autostart, auto  bool
	looks            int
	speaks           bool
	quietNow         bool
	autostartChanged int
}

func newFakeAce() *fakeAce {
	return &fakeAce{anchor: .5, alwaysOn: true, from: 22, to: 6, auto: true, speaks: true}
}

func (f *fakeAce) Introduced() bool { return f.introduced }
func (f *fakeAce) SetIntroduced()   { f.introduced = true }
func (f *fakeAce) Say(line, face string) bool {
	f.said, f.faces = append(f.said, line), append(f.faces, face)
	return f.speaks
}
func (f *fakeAce) Hush()               { f.hushed++ }
func (f *fakeAce) Try()                { f.tries++ }
func (f *fakeAce) Anchor() float64     { return f.anchor }
func (f *fakeAce) SetAnchor(v float64) { f.anchor = v }
func (f *fakeAce) Look(c *ui.Context)  { f.looks++; ui.Text(c, "Gaya island") }
func (f *fakeAce) AlwaysOn() bool      { return f.alwaysOn }
func (f *fakeAce) SetAlwaysOn(on bool) { f.alwaysOn = on }
func (f *fakeAce) QuietHours() (bool, int, int) {
	return f.quiet, f.from, f.to
}
func (f *fakeAce) SetQuietHours(on bool, from, to int) { f.quiet, f.from, f.to = on, from, to }
func (f *fakeAce) Quiet(time.Time) bool                { return f.quietNow }
func (f *fakeAce) Autostart() bool                     { return f.autostart }
func (f *fakeAce) SetAutostart(on bool) error {
	f.autostart = on
	f.autostartChanged++
	return nil
}
func (f *fakeAce) AutoUpdate() bool      { return f.auto }
func (f *fakeAce) SetAutoUpdate(on bool) { f.auto = on }

// inviteFake records the kadiv step's invitations.
type inviteFake struct {
	*onbFake
	WorkspaceAPI
	mu      sync.Mutex
	invites []api.InviteRequest
}

func (f *inviteFake) Invite(_ context.Context, _, _ id.ID, r api.InviteRequest) (api.Invitation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.invites = append(f.invites, r)
	return api.Invitation{ID: id.New()}, nil
}

// aceHarness is a new member on a desktop with an island.
func aceHarness(t *testing.T, role string, marks ...string) (*harness, *onbFake, *fakeAce) {
	t.Helper()
	h, f := onbHarness(t, role, marks...)
	ace := newFakeAce()
	h.app.Ace = ace
	h.app.onb.ace.said = "" // Ace was there from the start
	h.tt.Frame()
	return h, f, ace
}

// The five steps, each with Ace speaking its own line and face from the
// island; every choice goes to the device at once; finishing records the
// welcome, introduces Ace and quiets it.
func TestAceOnboardingFiveSteps(t *testing.T) {
	h, f, ace := aceHarness(t, api.RoleMember)
	h.want("1 DARI 5 · PROFIL")
	// The island speaks, so the card does not repeat the line.
	h.dontWant("Halo, aku Ace! Aku nyolek kamu kalau ada tugas, ping, atau rapat.")
	h.click("Simpan dan lanjut")
	h.tt.Frame()
	h.want("2 DARI 5 · KENALAN DENGAN ACE")
	h.want("Gaya island")
	h.click("Kanan")
	h.tt.Frame()
	if ace.anchor != 1 {
		t.Fatalf("anchor %v", ace.anchor)
	}
	h.click("Lanjut")
	h.tt.Frame()
	h.want("Kapan Ace muncul")
	h.click("Saat ada kabar")
	h.click("Jam tenang")
	h.click("Coba kabar")
	h.tt.Frame()
	if ace.alwaysOn || !ace.quiet || ace.from != 22 || ace.to != 6 || ace.tries != 1 {
		t.Fatalf("ace %+v", ace)
	}
	h.click("Lanjut")
	h.tt.Frame()
	h.want("Supaya kabar sampai")
	h.want("Mulai bersama Windows")
	h.click("Pembaruan otomatis")
	h.tt.Frame()
	if ace.auto {
		t.Fatal("automatic updates still on")
	}
	h.click("Lanjut")
	h.tt.Frame()
	h.want("Ace siap menemani")
	h.want("Ace: kanan · muncul saat ada kabar · jam tenang 22–06")
	h.click("Ke Beranda")
	h.settle()
	if !f.marked(api.OnboardWelcome) || !ace.introduced || ace.hushed == 0 {
		t.Fatalf("welcome %v introduced %v hushed %d", f.marked(api.OnboardWelcome), ace.introduced, ace.hushed)
	}
	wantFaces := []string{AceFaceHappy, AceFaceLook, "", AceFaceWink, AceFaceHappy}
	if !slices.Equal(ace.faces, wantFaces) || len(ace.said) != 5 {
		t.Fatalf("faces %q said %q", ace.faces, ace.said)
	}
	h.dontWant("Ace siap menemani")
}

// With the island off, Ace's line is in the card instead.
func TestAceOnboardingCardSpeaksWithoutIsland(t *testing.T) {
	h, _, ace := aceHarness(t, api.RoleMember)
	ace.speaks = false
	h.app.onb.ace.said = ""
	h.tt.Frame()
	h.tt.Frame()
	h.want("Halo, aku Ace! Aku nyolek kamu kalau ada tugas, ping, atau rapat.")
}

// A kadiv gets the invitation step: NIMs pasted, invitations sent.
func TestAceOnboardingKadivInvites(t *testing.T) {
	h, f, _ := aceHarness(t, api.RoleKadiv)
	inv := &inviteFake{onbFake: f}
	h.app.api = inv
	sp := &h.app.access.spaces[0]
	sp.Roles[0].Capabilities = append(sp.Roles[0].Capabilities, api.CapInvitationsManage)
	h.tt.Frame()
	h.want("1 DARI 6 · PROFIL")
	h.click("Lewati langkah ini")
	h.tt.Frame()
	h.want("Siapkan Media dan Teknologi")
	h.app.onb.ace.invite = "2210651001\n2210651014, 2210651001\nbukan-nim"
	h.tt.Frame()
	h.want("2 siap diundang · 1 bukan NIM atau email: bukan-nim")
	h.click("Kirim 2 undangan")
	h.settle()
	h.tt.Frame()
	if len(inv.invites) != 2 || inv.invites[0].NIM != "2210651001" || inv.invites[1].NIM != "2210651014" {
		t.Fatalf("invites %+v", inv.invites)
	}
	h.want("3 DARI 6 · KENALAN DENGAN ACE")
}

// An account onboarded before meets Ace once after the update: one dialog
// with "Yang baru" and Ace's settings, never the plain "Yang baru" too.
func TestAceUpdateDialogOnce(t *testing.T) {
	h, _, ace := aceHarness(t, api.RoleMember, api.OnboardWelcome, api.OnboardTour, api.OnboardChecklistHidden)
	var seen []string
	h.app.WhatsNew = &WhatsNew{Version: "0.4.6", Notes: []string{"Ace bisa diatur"}}
	h.app.SeenWhatsNew = func(v string) { seen = append(seen, v) }
	h.tt.Frame()
	h.tt.Frame()
	h.want("Yang baru di 0.4.6")
	h.want("Ace sekarang bisa diatur")
	h.want("Ace bisa diatur")
	h.dontWant("Mengerti") // the plain dialog's button
	if len(ace.said) != 1 || ace.faces[0] != AceFaceWink {
		t.Fatalf("said %q faces %q", ace.said, ace.faces)
	}
	h.click("Kiri")
	h.click("Selesai")
	h.tt.Frame()
	if !ace.introduced || ace.anchor != 0 || !slices.Equal(seen, []string{"0.4.6"}) || h.app.WhatsNew != nil {
		t.Fatalf("introduced %v anchor %v seen %v", ace.introduced, ace.anchor, seen)
	}
	h.tt.Frame()
	h.dontWant("Ace sekarang bisa diatur")
}

// Quiet hours hold device notices back, unoffered, for after.
func TestQuietHoursHoldDeliveries(t *testing.T) {
	if !QuietAt(23, 22, 6) || !QuietAt(5, 22, 6) || QuietAt(6, 22, 6) || QuietAt(12, 22, 6) || !QuietAt(1, 0, 7) || QuietAt(3, 4, 4) {
		t.Fatal("QuietAt")
	}
	a := &App{}
	ace := newFakeAce()
	a.Ace = ace
	ace.quietNow = true
	if !a.Ace.Quiet(time.Now()) {
		t.Fatal("quiet")
	}
}

func TestAceNIMs(t *testing.T) {
	good, bad := aceNIMs(" 2210651001 ,2210651001\nnama@mail.unej.ac.id;12\n")
	if len(good) != 2 || good[0].NIM != "2210651001" || good[1].Email != "nama@mail.unej.ac.id" || !slices.Equal(bad, []string{"12"}) {
		t.Fatalf("good %+v bad %q", good, bad)
	}
}
