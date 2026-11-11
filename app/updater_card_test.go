//go:build !js

package main

import (
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
