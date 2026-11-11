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
	// background: the install under way is automatic (autoInstall).
	background bool
	// onClose: Nanti on a downloaded version installs it once the window
	// closes, even with automatic updates off.
	onClose bool
	// failed: the last check could not read a feed; noFeed: it found
	// nothing published (idleStatus).
	failed, noFeed bool
}

func newDesktopUpdater(d *desktopDevice) *desktopUpdater {
	return &desktopUpdater{d: d, client: &http.Client{Timeout: 10 * time.Minute, Jar: nil}}
}

func (u *desktopUpdater) dir() string { return filepath.Join(u.d.root, "updates") }

// channel is the device's choice, else rc on a release candidate, else
// Stabil (updateChannel).
func (u *desktopUpdater) channel() string {
	return updateChannel(u.d.prefs.UpdateChannel, appVersion())
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
		var results []feedResult
		for _, ch := range updateFeeds(channel) {
			var r feedResult
			var data []byte
			if data, r.err = update.Fetch(ctx, u.client, update.FeedURL(ch), current); r.err == nil {
				r.manifest, r.err = update.Open(data, update.PublicKeys(), ch)
			}
			results = append(results, r)
		}
		out := summarizeCheck(current, results)
		u.d.win.Update(func() {
			if u.state == updChecking {
				u.state = updIdle
			}
			u.checked = time.Now()
			u.failed, u.noFeed = out.err != nil, out.noFeed
			if out.offer == nil {
				if out.err != nil && manual {
					u.err = "Pembaruan belum dapat diperiksa: " + friendlyUpdateError(out.err)
				}
				return
			}
			u.err = ""
			m := *out.offer
			if u.offer != nil && u.offer.Version == m.Version && u.state != updIdle {
				return // already being handled
			}
			u.offer, u.state, u.setup, u.onClose = &m, updAvailable, "", false
			// Every new version is told (the window's card, Ace); automatic
			// updates also fetch it and install it once Space's window is
			// closed (autoInstall).
			u.announce(manual)
			if !manual && !u.d.prefs.UpdateManual && installerUpdatesSupported() {
				u.download(false)
			}
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

// announce tells of the offer once per version, unless asked or put off
// with Nanti: with the window open its card tells, and the notice goes
// quietly into Ace's inbox; with the window in the tray Ace Island shows
// it (or a Windows toast when the island cannot), and keeps it after.
func (u *desktopUpdater) announce(manual bool) {
	m := u.offer
	p := &u.d.prefs
	if !manual && (p.UpdateAnnounced == m.Version || time.Now().Before(p.UpdateLaterUntil)) {
		return
	}
	p.UpdateAnnounced = m.Version
	u.d.save()
	account := ""
	if u.d.island != nil {
		account = u.d.island.account()
	}
	x := updateNotice(*m, !p.UpdateManual && installerUpdatesSupported(), account)
	if u.d.win.IsVisible() && u.d.island != nil && u.d.island.keep(x) {
		return
	}
	// The app's notifier is the island, with Windows' toast when the island
	// cannot show it or drops it (islandNotifier).
	_ = u.d.app.Notifier.Present(x)
}

// updateNotice is the notice of a version on offer, for account.
func updateNotice(m update.Manifest, auto bool, account string) appui.Notice {
	body := m.Notes
	if body == "" {
		body = "Versi baru Space."
	}
	if auto {
		body = "Diunduh di latar, dipasang saat Space ditutup.\n" + body
	}
	return appui.Notice{Kind: "update", AccountID: account, Title: "Space " + m.Version + " tersedia", Body: body, Tag: "update:" + m.Version,
		Link: updateLink + "?action=install", OpenLabel: "Perbarui",
		Actions: []appui.NoticeAction{{Label: "Lihat perubahan", Link: updateLink + "?action=notes"}, {Label: "Nanti", Link: updateLink + "?action=later"}}}
}

// handleLink answers the actions of the notice.
func (u *desktopUpdater) handleLink(link *url.URL) {
	switch link.Query().Get("action") {
	case "install":
		// The window's card carries on: the download, then "Siap
		// dipasang".
		u.d.win.Show()
		u.d.win.Focus()
		u.InstallUpdate()
	case "notes":
		u.d.win.Show()
		u.d.win.Focus()
		u.UpdateNotes()
	case "later":
		if u.offer != nil {
			u.d.app.UpdateLater(u.offer.Version)
		}
	}
}

// UpdateStatus is the offer for the window's card (appui.Updater).
func (u *desktopUpdater) UpdateStatus() (appui.UpdateStatus, bool) {
	if u.offer == nil {
		return appui.UpdateStatus{}, false
	}
	stage := map[string]string{updAvailable: appui.UpdateAvailable, updDownloading: appui.UpdateDownloading, updReady: appui.UpdateReady}[u.state]
	if stage == "" {
		return appui.UpdateStatus{}, false
	}
	return appui.UpdateStatus{Stage: stage, Version: u.offer.Version, Note: appui.FirstNoteLine(u.offer.Notes), Done: u.done, Total: u.total,
		Auto: !u.d.prefs.UpdateManual && installerUpdatesSupported(), Err: u.err}, true
}

// InstallUpdate is the card's Perbarui, which downloads (the card then
// says "Siap dipasang"), and Mulai ulang sekarang.
func (u *desktopUpdater) InstallUpdate() {
	if u.offer == nil {
		return
	}
	if !installerUpdatesSupported() {
		u.err = "Pembaruan otomatis hanya untuk Space yang dipasang dengan installer Windows. Unduh installer dari halaman rilis."
		return
	}
	switch u.state {
	case updAvailable:
		u.download(false)
	case updReady:
		u.runInstaller()
	}
}

// UpdateNotes is Lihat perubahan: the release's page, else the settings.
func (u *desktopUpdater) UpdateNotes() {
	u.notes = true
	if u.offer != nil && u.offer.NotesURL != "" {
		_ = mygo.Shell.OpenExternal(u.offer.NotesURL)
		return
	}
	u.d.app.OpenDeviceSettings()
}

// UpdateLater is the card's Nanti: a downloaded version then installs
// when the window closes; Ace's inbox lets the notice go.
func (u *desktopUpdater) UpdateLater() {
	if u.state == updReady {
		u.onClose = true
	}
	if u.offer != nil && u.d.island != nil {
		u.d.island.unkeep("update:" + u.offer.Version)
	}
}

// later puts the offer off for updateLater, from the island or the
// settings alike: the offer comes back at the next check after that.
func (u *desktopUpdater) later() {
	u.d.prefs.UpdateLaterUntil = time.Now().Add(updateLater)
	u.d.save()
	if u.offer != nil && u.d.island != nil {
		u.d.island.unkeep("update:" + u.offer.Version)
	}
	if u.state == updAvailable || u.state == updReady {
		u.offer, u.state, u.setup = nil, updIdle, ""
	}
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
			} else {
				u.autoInstall()
			}
		})
	}()
}

// autoInstall installs a version downloaded by the automatic updates while
// nobody uses Space, its window closed to the tray; Space comes back to the
// tray. Called when the download ends, when the window closes, and with
// the deliveries' poll.
func (u *desktopUpdater) autoInstall() {
	if u.state != updReady || u.d.prefs.UpdateManual && !u.onClose || u.d.win.IsVisible() || u.d.quitting {
		return
	}
	u.background = true
	u.runInstaller()
	u.background = false
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
	// The signed notes go with the version, for its "Yang baru" once it runs.
	u.d.prefs.UpdateNotes, u.d.prefs.UpdateNotesV = m.Notes, m.Version
	u.d.save()
	if err := startUpdateHelper(updateJob{Setup: u.setup, Installer: *m.Windows, Version: m.Version, Dir: u.dir(), Background: u.background}); err != nil {
		u.state = updReady
		u.err = "Pemasang tidak dapat dijalankan: " + err.Error()
		return
	}
	u.d.quitting = true
	mygo.App.Quit()
}

// updateSettings is the "Pembaruan aplikasi" section, in Perangkat ini and
// (through appui.SettingsBlock) in Status layanan.
func (d *desktopDevice) updateSettings(c *ui.Context) {
	u := d.updater
	busy := u.state == updChecking || u.state == updDownloading || u.state == updInstalling
	appui.SettingSection(c, "Pembaruan aplikasi", "Space memeriksa versi baru saat dibuka dan setiap 6 jam, tanpa mengirim data akun.", func() {
		t := c.Theme()
		appui.SettingRow(c, "Versi terpasang", "Space "+appVersion()+" · "+u.statusText(), func() {
			if appui.SettingButton(c, "Periksa pembaruan", "refresh").Disabled(busy).Clicked() {
				u.check(true)
			}
		})
		auto := !d.prefs.UpdateManual
		if appui.SettingSwitch(c, "Pembaruan otomatis", "Versi baru diunduh di latar dan dipasang saat jendela Space tertutup. Mati: versi baru hanya diumumkan.", &auto) {
			d.prefs.UpdateManual = !auto
			d.save()
			if auto {
				u.autoInstall()
			}
		}
		// The channel is for testers: shown on a test build, or once chosen.
		if strings.Contains(appVersion(), "-") || u.channel() == update.ChannelRC {
			u.channelRow(c, busy)
		}
		if u.notice != "" || u.err != "" {
			appui.SettingRow(c, "Kabar pembaruan", "", func() {
				if u.notice != "" {
					ui.Text(c, u.notice).FontSize(13).TextColor(t.Success)
				}
				if u.err != "" {
					ui.Text(c, u.err).FontSize(13).TextColor(t.Danger).MaxWidth(520)
				}
			})
		}
		switch u.state {
		case updAvailable, updReady:
			m := u.offer
			desc := m.Notes
			if u.state == updReady {
				desc = "Sudah diunduh dan diperiksa. Space akan tertutup sebentar dan terbuka lagi."
			}
			appui.SettingRow(c, "Space "+m.Version+" tersedia", desc, func() {
				if appui.SettingButton(c, "Perbarui sekarang", "check").Background(t.Accent).TextColor(t.AccentText).Clicked() {
					u.install()
				}
				if u.state == updAvailable && installerUpdatesSupported() && appui.SettingButton(c, "Unduh di latar", "").Clicked() {
					u.download(false)
				}
				if m.NotesURL != "" && appui.SettingButton(c, "Lihat perubahan", "link").Clicked() {
					_ = mygo.Shell.OpenExternal(m.NotesURL)
				}
				if appui.SettingButton(c, "Nanti", "").Clicked() {
					u.later()
				}
			})
		case updDownloading:
			pct := 0
			if u.total > 0 {
				pct = int(u.done * 100 / u.total)
			}
			appui.SettingRow(c, "Mengunduh Space "+u.offer.Version, fmt.Sprintf("%d%% · %.1f dari %.1f MB", pct, float64(u.done)/1e6, float64(u.total)/1e6), func() {
				ui.Box(c).Width(160).Height(6).Radius(3).Background(t.Border).Children(func() {
					ui.Box(c).Height(6).Radius(3).Background(t.Accent).WidthPercent(float32(pct))
				})
				if appui.SettingButton(c, "Batalkan unduhan", "close").Clicked() && u.cancel != nil {
					u.cancel()
				}
			})
		case updInstalling:
			appui.SettingRow(c, "Memasang pembaruan", "Space akan tertutup sebentar dan terbuka lagi.", func() {})
		}
	})
}

// channelRow is the choice of Stabil or Uji coba (rc).
func (u *desktopUpdater) channelRow(c *ui.Context, busy bool) {
	d := u.d
	appui.SettingRow(c, "Kanal pembaruan", "Uji coba menerima versi rc lebih dulu, dan tetap menerima rilis Stabil yang lebih baru.", func() {
		sel := 0
		if u.channel() == update.ChannelRC {
			sel = 1
		}
		if i := appui.Segmented(c, "Kanal pembaruan", []string{"Stabil", "Uji coba (rc)"}, sel); i >= 0 && i != sel && !busy {
			d.prefs.UpdateChannel = []string{update.ChannelStable, update.ChannelRC}[i]
			d.prefs.UpdateAnnounced = ""
			d.save()
			u.offer, u.state, u.setup = nil, updIdle, ""
			u.check(true)
		}
	})
}

// statusText is the updater's state in a few words.
func (u *desktopUpdater) statusText() string {
	switch u.state {
	case updChecking:
		return "memeriksa…"
	case updAvailable, updReady:
		return "versi " + u.offer.Version + " tersedia"
	case updDownloading:
		return "mengunduh…"
	case updInstalling:
		return "memasang…"
	}
	return idleStatus(u.channel(), u.checked, u.failed, u.noFeed)
}

// navStatus is the section's state in the column of sections.
func (u *desktopUpdater) navStatus() string {
	if u.offer != nil {
		return u.offer.Version + " tersedia"
	}
	if u.channel() == update.ChannelRC {
		return "Uji coba"
	}
	return "Stabil"
}
