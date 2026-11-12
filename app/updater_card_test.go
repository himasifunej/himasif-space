//go:build !js

package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/update"
)

// The card follows the updater: found, downloading, ready; nothing while
// idle, checking or installing.
func TestUpdaterCardStatus(t *testing.T) {
	d := &desktopDevice{}
	u := newDesktopUpdater(d)
	if _, ok := u.UpdateStatus(); ok {
		t.Fatal("card without an offer")
	}
	u.offer = &update.Manifest{Version: "0.4.6-rc.3", Notes: "- Kartu pembaruan.\n- Lainnya."}
	for state, want := range map[string]string{updAvailable: appui.UpdateAvailable, updDownloading: appui.UpdateDownloading, updReady: appui.UpdateReady, updChecking: "", updInstalling: "", updIdle: ""} {
		u.state = state
		s, ok := u.UpdateStatus()
		if ok != (want != "") || s.Stage != want {
			t.Fatalf("%q: %+v %v", state, s, ok)
		}
		if ok && (s.Version != "0.4.6-rc.3" || s.Note != "Kartu pembaruan.") {
			t.Fatalf("%q: %+v", state, s)
		}
	}
	u.state, u.done, u.total, u.err = updDownloading, 30, 60, "Unduhan gagal: waktu habis."
	if s, _ := u.UpdateStatus(); s.Done != 30 || s.Total != 60 || s.Err == "" {
		t.Fatalf("progress %+v", s)
	}
}

// Nanti on a downloaded version installs it when the window closes, even
// with automatic updates off; before that it only hides the card.
func TestUpdaterLaterInstallsOnClose(t *testing.T) {
	d := &desktopDevice{}
	d.prefs.UpdateManual = true
	u := newDesktopUpdater(d)
	u.offer, u.state = &update.Manifest{Version: "0.4.6-rc.3"}, updAvailable
	u.UpdateLater()
	if u.onClose || u.state != updAvailable || u.offer == nil {
		t.Fatalf("Nanti while available: onClose %v state %q", u.onClose, u.state)
	}
	u.state = updReady
	u.UpdateLater()
	if !u.onClose || u.state != updReady {
		t.Fatalf("Nanti while ready: onClose %v state %q", u.onClose, u.state)
	}
}

// The notice of a version: Ace's inbox keeps it for the account while the
// version is newer than this Space, with the update's icon and actions.
func TestUpdateNoticeInInbox(t *testing.T) {
	account := id.New().String()
	n := updateNotice(update.Manifest{Version: "99.0.0", Notes: "Kartu pembaruan."}, true, account)
	if n.Kind != "update" || n.Title != "Space 99.0.0 tersedia" || n.OpenLabel != "Perbarui" || len(n.Actions) != 2 || !strings.HasPrefix(n.Body, "Diunduh di latar") {
		t.Fatalf("notice %+v", n)
	}
	if manual := updateNotice(update.Manifest{Version: "99.0.0"}, false, account); manual.Body != "Versi baru Space." {
		t.Fatalf("manual body %q", manual.Body)
	}
	if appui.NoticeGlyph("update") == appui.NoticeGlyph("ping") {
		t.Fatal("an update looks like a ping on Ace")
	}
	if !inboxKeeps(n) {
		t.Fatal("newer version not kept")
	}
	for _, v := range []string{appVersion(), "0.0.1"} {
		if inboxKeeps(updateNotice(update.Manifest{Version: v}, false, account)) {
			t.Fatalf("version %s, not newer than %s, kept", v, appVersion())
		}
	}
	if inboxKeeps(updateNotice(update.Manifest{Version: "99.0.0"}, false, "")) {
		t.Fatal("kept without an account")
	}

	// Into the inbox quietly, out with Nanti; an installed version leaves
	// it when the inbox is read again.
	uid := id.New()
	d := &desktopDevice{root: t.TempDir(), activeUser: uid}
	d.notificationPrefs.Store(devicePreferences{IslandEnabled: true})
	p := &islandPresenter{d: d}
	d.island = p
	t.Cleanup(p.inbox.saves.Wait)
	if !p.store([]appui.Notice{updateNotice(update.Manifest{Version: "99.0.0"}, false, p.account())}) || p.inboxCount(uid.String()) != 1 {
		t.Fatal("not kept")
	}
	p.unkeep("update:99.0.0")
	if p.inboxCount(uid.String()) != 0 {
		t.Fatal("Nanti left it")
	}
	p.inbox.saves.Wait() // the presenter's writes first: the file is the test's now
	box := islandInbox{}
	box.add(updateNotice(update.Manifest{Version: "99.0.0"}, false, uid.String()), time.Now())
	box.Entries[0].Notice.Tag = "update:" + appVersion() // as if now installed
	if err := saveIslandInbox(d.root, box); err != nil {
		t.Fatal(err)
	}
	if got := loadIslandInbox(d.root, time.Now()); len(got.Entries) != 0 {
		t.Fatalf("installed version still in the inbox: %+v", got.Entries)
	}
}

// An installer downloaded and checked at an earlier start is ready at
// once at the next: its card says "Siap dipasang" before any check. It
// is dropped when the file changed, the signature fails, the version is
// no longer newer, or the offer was put off.
func TestUpdaterRestoresReadyDownload(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	old := updateKeys
	updateKeys = func() []ed25519.PublicKey { return []ed25519.PublicKey{pub} }
	defer func() { updateKeys = old }()
	setupData := []byte("installer")
	sum := sha256.Sum256(setupData)
	sign := func(version string) []byte {
		env, err := update.Sign(update.Manifest{App: update.App, Channel: update.ChannelRC, Version: version, Released: "2026-11-12T00:00:00Z", Notes: "Kartu lebih cepat.",
			Windows: &update.Installer{URL: "https://example.com/s.exe", SHA256: hex.EncodeToString(sum[:]), Size: int64(len(setupData))}}, priv)
		if err != nil {
			t.Fatal(err)
		}
		return env
	}
	prepare := func(envelope, setup []byte) *desktopUpdater {
		d := &desktopDevice{root: t.TempDir()}
		d.prefs.UpdateChannel = update.ChannelRC
		u := newDesktopUpdater(d)
		_ = os.MkdirAll(u.dir(), 0o700)
		_ = os.WriteFile(filepath.Join(u.dir(), "space-setup-99.0.0.exe"), setup, 0o600)
		_ = os.WriteFile(filepath.Join(u.dir(), "result.json"), []byte("{}"), 0o600)
		u.saveReady(envelope, update.ChannelRC, filepath.Join(u.dir(), "space-setup-99.0.0.exe"))
		return u
	}
	u := prepare(sign("99.0.0"), setupData)
	u.readResult()
	if s, ok := u.UpdateStatus(); !ok || s.Stage != appui.UpdateReady || s.Version != "99.0.0" {
		t.Fatalf("restored: %+v %v", s, ok)
	}
	if _, err := os.Stat(u.setup); err != nil {
		t.Fatalf("installer removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(u.dir(), "result.json")); err == nil {
		t.Fatal("the helper's report kept")
	}

	cases := map[string]*desktopUpdater{
		"changed file": prepare(sign("99.0.0"), []byte("tampered")),
		"older":        prepare(sign("0.0.1"), setupData),
		"bad envelope": prepare([]byte(`{"format":1}`), setupData),
	}
	later := prepare(sign("99.0.0"), setupData)
	later.d.prefs.UpdateLaterUntil = time.Now().Add(time.Hour)
	cases["put off"] = later
	stable := prepare(sign("99.0.0"), setupData)
	stable.d.prefs.UpdateChannel = update.ChannelStable
	cases["other channel"] = stable
	for name, u := range cases {
		u.readResult()
		if _, ok := u.UpdateStatus(); ok {
			t.Fatalf("%s: restored", name)
		}
		if entries, _ := os.ReadDir(u.dir()); len(entries) != 0 {
			t.Fatalf("%s: %d files left", name, len(entries))
		}
	}
}

// The first check waits for the first screen, not a fixed delay.
func TestUpdaterFirstScreen(t *testing.T) {
	u := newDesktopUpdater(&desktopDevice{})
	select {
	case <-u.firstScreen:
		t.Fatal("open before the first screen")
	default:
	}
	u.screenShown()
	u.screenShown() // told once per frame is fine
	select {
	case <-u.firstScreen:
	case <-time.After(time.Second):
		t.Fatal("not told")
	}
}
