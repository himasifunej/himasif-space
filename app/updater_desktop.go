//go:build !js

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/appui"
	"github.com/himasifunej/himasif-space/internal/update"
)

// The desktop app updates itself (docs/operations/auto-update.md): it reads
// the signed manifests of its channel at start and every few hours,
// announces a newer version on Ace Island or a Windows toast, downloads the
// installer in the background, checks its SHA-256 and runs it silently
// (NSIS /S) from a helper once Space has closed, which opens Space again.

const (
	updateFirstCheck = 20 * time.Second
	updateEvery      = 6 * time.Hour
	updateLater      = 24 * time.Hour
	updateLink       = "himasif-space://update"
)

// Update states, as the settings row shows them.
const (
	updIdle        = ""
	updChecking    = "checking"
	updAvailable   = "available"
	updDownloading = "downloading"
	updReady       = "ready"
	updInstalling  = "installing"
)

// desktopUpdater's fields belong to the main window's thread; network and
// file work runs in goroutines that hand results back with win.Update.
type desktopUpdater struct {
	d       *desktopDevice
	client  *http.Client
	state   string
	offer   *update.Manifest
	setup   string // the downloaded installer, checked
	done    int64
	total   int64
	err     string
	checked time.Time
	notice  string // after an update: "Space diperbarui ke …"
	notes   bool   // the notes of offer are open
	cancel  context.CancelFunc
}

func newDesktopUpdater(d *desktopDevice) *desktopUpdater {
	return &desktopUpdater{d: d, client: &http.Client{Timeout: 10 * time.Minute, Jar: nil}}
}

func (u *desktopUpdater) dir() string { return filepath.Join(u.d.root, "updates") }

// channel is the device's choice: Stabil unless Uji coba was chosen.
func (u *desktopUpdater) channel() string {
	if u.d.prefs.UpdateChannel == update.ChannelRC {
		return update.ChannelRC
	}
	return update.ChannelStable
}

// start reads what a finished update left, cleans up, and checks soon
// and then every few hours.
func (u *desktopUpdater) start() {
	u.readResult()
	go func() {
		time.Sleep(updateFirstCheck)
		for {
			u.d.win.Update(func() { u.check(false) })
			time.Sleep(updateEvery)
		}
	}()
}

// readResult shows what the helper of the last update reported, then
// removes the installer, the helper and the report.
func (u *desktopUpdater) readResult() {
	data, err := os.ReadFile(filepath.Join(u.dir(), updateResultFile))
	if err == nil {
		var r updateResult
		if json.Unmarshal(data, &r) == nil {
			switch {
			case r.OK && update.Compare(appVersion(), r.Version) >= 0:
				u.notice = "Space diperbarui ke " + appVersion() + "."
			case r.OK:
				u.err = "Pemasang selesai, tetapi Space masih versi " + appVersion() + ". Coba perbarui lagi."
			default:
				u.err = "Pembaruan ke " + r.Version + " gagal: " + r.Error
			}
		}
	}
	// The helper may still be finishing: what cannot go now goes next time.
	entries, _ := os.ReadDir(u.dir())
	for _, e := range entries {
		_ = os.RemoveAll(filepath.Join(u.dir(), e.Name()))
	}
}

// check reads the channel's manifests; manual reports "up to date" too,
// an automatic check only announces a version not announced or put off.
func (u *desktopUpdater) check(manual bool) {
	if u.state == updChecking || u.state == updDownloading || u.state == updInstalling {
		return
	}
	if u.state == updIdle {
		u.state = updChecking
	}
	if manual {
		u.err, u.notice = "", ""
	}
	channel, current := u.channel(), appVersion()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		channels := []string{update.ChannelStable}
		if channel == update.ChannelRC {
			channels = append(channels, update.ChannelRC)
		}
		var found []update.Manifest
		var firstErr error
		for _, ch := range channels {
			data, err := update.Fetch(ctx, u.client, update.FeedURL(ch), current)
			if err == nil {
				var m update.Manifest
				if m, err = update.Open(data, update.PublicKeys(), ch); err == nil {
					found = append(found, m)
				}
			}
			if err != nil && firstErr == nil {
				firstErr = err
			}
		}
		m, ok := update.Newest(current, found...)
		u.d.win.Update(func() {
			if u.state == updChecking {
				u.state = updIdle
			}
			u.checked = time.Now()
			if !ok {
				if len(found) == 0 && firstErr != nil && manual {
					u.err = "Pembaruan belum dapat diperiksa: " + friendlyUpdateError(firstErr)
				}
				return
			}
			if u.offer != nil && u.offer.Version == m.Version && u.state != updIdle {
				return // already being handled
			}
			u.offer, u.state, u.setup = &m, updAvailable, ""
			u.announce(manual)
		})
	}()
}

func friendlyUpdateError(err error) string {
	switch {
	case errors.Is(err, update.ErrSignature):
		return "tanda tangan manifest tidak sah, pembaruan diabaikan."
	case errors.Is(err, update.ErrChecksum):
		return "berkas unduhan tidak cocok dengan manifest dan sudah dihapus."
	case errors.Is(err, context.DeadlineExceeded):
		return "waktu habis."
	}
	return err.Error()
}

// announce shows the offer on Ace Island, or as a Windows toast when the
// island cannot, once per version unless asked or put off with Nanti.
func (u *desktopUpdater) announce(manual bool) {
	m := u.offer
	p := &u.d.prefs
	if !manual && (p.UpdateAnnounced == m.Version || time.Now().Before(p.UpdateLaterUntil)) {
		return
	}
	p.UpdateAnnounced = m.Version
	u.d.save()
	body := m.Notes
	if body == "" {
		body = "Versi baru Space siap dipasang."
	}
	x := appui.Notice{Kind: "update", Title: "Space " + m.Version + " tersedia", Body: body, Tag: "update:" + m.Version,
		Link: updateLink + "?action=install", OpenLabel: "Perbarui sekarang",
		Actions: []appui.NoticeAction{{Label: "Lihat perubahan", Link: updateLink + "?action=notes"}, {Label: "Nanti", Link: updateLink + "?action=later"}}}
	prefs := u.d.notificationPrefs.Load().(devicePreferences)
	_ = (appui.MultiNotifier{Base: u.d.app.Notifier, Extra: []appui.NoticePresenter{updateIsland{u.d.island}}, ReplaceBase: prefs.IslandReplaceToast}).Present(x)
}

// updateIsland offers an update notice to Ace Island; Windows' toast is
// MultiNotifier's fallback.
type updateIsland struct{ p *islandPresenter }

func (n updateIsland) Present(x appui.Notice) error { return n.p.offer(x, nil) }

// handleLink answers the actions of the notice.
func (u *desktopUpdater) handleLink(link *url.URL) {
	switch link.Query().Get("action") {
	case "install":
		u.d.win.Show()
		u.d.win.Focus()
		u.d.app.OpenDeviceSettings()
		u.install()
	case "notes":
		u.d.win.Show()
		u.d.win.Focus()
		u.d.app.OpenDeviceSettings()
		u.notes = true
		if u.offer != nil && u.offer.NotesURL != "" {
			_ = mygo.Shell.OpenExternal(u.offer.NotesURL)
		}
	case "later":
		u.later()
	}
}

func (u *desktopUpdater) later() {
	u.d.prefs.UpdateLaterUntil = time.Now().Add(updateLater)
	u.d.save()
}

// install downloads the offer when needed, then hands it to the helper.
func (u *desktopUpdater) install() {
	if u.offer == nil {
		return
	}
	if !installerUpdatesSupported() {
		u.err = "Pembaruan otomatis hanya untuk Space yang dipasang dengan installer Windows. Unduh installer dari halaman rilis."
		return
	}
	switch u.state {
	case updAvailable:
		u.download(true)
	case updReady:
		u.runInstaller()
	}
}

// download fetches the installer in the background with its progress;
// then installs it at once when then is set.
func (u *desktopUpdater) download(then bool) {
	m := *u.offer
	u.state, u.done, u.total, u.err = updDownloading, 0, m.Windows.Size, ""
	ctx, cancel := context.WithCancel(context.Background())
	u.cancel = cancel
	path := filepath.Join(u.dir(), "space-setup-"+safeVersion(m.Version)+".exe")
	current := appVersion()
	go func() {
		defer cancel()
		err := os.MkdirAll(u.dir(), 0o700)
		if err == nil {
			err = update.Download(ctx, u.client, *m.Windows, path, current, func(done, total int64) {
				u.d.win.Update(func() { u.done, u.total = done, total })
			})
		}
		u.d.win.Update(func() {
			u.cancel = nil
			if u.offer == nil || u.offer.Version != m.Version {
				return
			}
			if err != nil {
				u.state = updAvailable
				if !errors.Is(err, context.Canceled) {
					u.err = "Unduhan gagal: " + friendlyUpdateError(err)
				}
				return
			}
			u.state, u.setup = updReady, path
			if then {
				u.runInstaller()
			}
		})
	}()
}

// safeVersion keeps a version to the characters of a file name.
func safeVersion(v string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '.' || r == '-' {
			return r
		}
		return '_'
	}, v)
}

// runInstaller checks the installer once more, saves what must be saved,
// starts the helper and closes Space; the helper installs and opens it
// again.
func (u *desktopUpdater) runInstaller() {
	m := u.offer
	if m == nil || u.setup == "" {
		return
	}
	if err := update.VerifyFile(u.setup, *m.Windows); err != nil {
		u.state, u.setup = updAvailable, ""
		u.err = "Installer berubah sejak diunduh dan tidak dijalankan. Unduh ulang."
		return
	}
	if err := u.d.app.FlushNoteDraft(); err != nil {
		u.err = "Simpan catatan yang sedang ditulis dulu, lalu perbarui lagi."
		return
	}
	u.state = updInstalling
	if err := startUpdateHelper(updateJob{Setup: u.setup, Installer: *m.Windows, Version: m.Version, Dir: u.dir()}); err != nil {
		u.state = updReady
		u.err = "Pemasang tidak dapat dijalankan: " + err.Error()
		return
	}
	u.d.quitting = true
	mygo.App.Quit()
}

// updateSettings is the "Pembaruan aplikasi" block, in Perangkat and in
// Status layanan.
func (d *desktopDevice) updateSettings(c *ui.Context) {
	u := d.updater
	deviceSection(c, "Pembaruan aplikasi", "Space memeriksa versi baru saat dibuka dan setiap 6 jam, tanpa mengirim data akun.", func() {
		t := c.Theme()
		ui.Text(c, "Versi terpasang: "+appVersion()).FontSize(13)
		ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Kanal").FontSize(13).TextColor(t.TextMuted)
			for _, ch := range []struct{ key, label string }{{update.ChannelStable, "Stabil"}, {update.ChannelRC, "Uji coba (rc)"}} {
				b := ui.Button(c, ch.label)
				if u.channel() == ch.key {
					b.Background(t.Accent).TextColor(t.AccentText)
				}
				if b.Disabled(u.state == updDownloading || u.state == updInstalling).Clicked() && u.channel() != ch.key {
					u.d.prefs.UpdateChannel = ch.key
					u.d.prefs.UpdateAnnounced = ""
					u.d.save()
					u.offer, u.state, u.setup = nil, updIdle, ""
					u.check(true)
				}
			}
		})
		if u.notice != "" {
			ui.Text(c, u.notice).FontSize(13).TextColor(t.Success)
		}
		if u.err != "" {
			ui.Text(c, u.err).FontSize(13).TextColor(t.Danger)
		}
		switch u.state {
		case updChecking:
			ui.Text(c, "Memeriksa pembaruan…").FontSize(13).TextColor(t.TextMuted)
		case updAvailable, updReady:
			m := u.offer
			ui.Text(c, "Space "+m.Version+" tersedia").FontSize(14).Bold()
			if m.Notes != "" && (u.notes || len(m.Notes) <= 140) {
				ui.Text(c, m.Notes).FontSize(13).TextColor(t.TextMuted)
			}
			ui.Row(c).Gap(8).Wrap().Children(func() {
				label := "Perbarui sekarang"
				if ui.Button(c, label).Background(t.Accent).TextColor(t.AccentText).Clicked() {
					u.install()
				}
				if u.state == updAvailable && installerUpdatesSupported() && ui.Button(c, "Unduh di latar").Clicked() {
					u.download(false)
				}
				if ui.Button(c, "Lihat perubahan").Clicked() {
					u.notes = !u.notes
					if u.offer.NotesURL != "" {
						_ = mygo.Shell.OpenExternal(u.offer.NotesURL)
					}
				}
				if ui.Button(c, "Nanti").Clicked() {
					u.later()
					u.offer, u.state = nil, updIdle
				}
			})
			if u.state == updReady {
				ui.Text(c, "Sudah diunduh dan diperiksa. Space akan tertutup sebentar dan terbuka lagi.").FontSize(12).TextColor(t.TextMuted)
			}
		case updDownloading:
			pct := 0
			if u.total > 0 {
				pct = int(u.done * 100 / u.total)
			}
			ui.Text(c, fmt.Sprintf("Mengunduh Space %s… %d%% (%.1f dari %.1f MB)", u.offer.Version, pct, float64(u.done)/1e6, float64(u.total)/1e6)).FontSize(13)
			ui.Box(c).FillWidth().Height(6).Radius(3).Background(t.Border).Children(func() {
				ui.Box(c).Height(6).Radius(3).Background(t.Accent).WidthPercent(float32(pct))
			})
			if ui.Button(c, "Batalkan unduhan").Clicked() && u.cancel != nil {
				u.cancel()
			}
		case updInstalling:
			ui.Text(c, "Memasang… Space akan terbuka lagi.").FontSize(13)
		default:
			last := "belum diperiksa"
			if !u.checked.IsZero() {
				last = "diperiksa " + u.checked.Format("15.04")
			}
			ui.Text(c, "Space sudah versi terbaru ("+last+").").FontSize(13).TextColor(t.TextMuted)
		}
		if ui.Button(c, "Periksa pembaruan").Disabled(u.state == updChecking || u.state == updDownloading || u.state == updInstalling).Clicked() {
			u.check(true)
		}
	})
}
