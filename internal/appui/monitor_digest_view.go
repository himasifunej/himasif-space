package appui

import (
	"context"

	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
)

// MonitorDigestAPI is a monitor's weekly digest (MON-08).
type MonitorDigestAPI interface {
	MonitorDigest(context.Context) (api.MonitorDigestSettings, error)
	SaveMonitorDigest(context.Context, api.MonitorDigestRequest) (api.MonitorDigestSettings, error)
}

var _ MonitorDigestAPI = (*client.Client)(nil)

type monitorDigestUIState struct {
	loaded, loading, busy bool
	err, notice           string
	settings              api.MonitorDigestSettings
}

func (a *App) monitorDigestService() (MonitorDigestAPI, bool) {
	if c, ok := a.api.(*client.Client); ok {
		return c.AccountSession(), true
	}
	s, ok := a.api.(MonitorDigestAPI)
	return s, ok
}

// runMonitorDigest loads (save nil) or saves the digest settings.
func (a *App) runMonitorDigest(save *api.MonitorDigestRequest) {
	s, ok := a.monitorDigestService()
	if !ok {
		a.digest.err = "Layanan ringkasan mingguan belum tersedia."
		return
	}
	if a.digest.loading || a.digest.busy {
		return
	}
	gen := a.gen
	if save != nil {
		a.digest.busy = true
	} else {
		a.digest.loading = true
	}
	a.digest.err, a.digest.notice = "", ""
	a.async(func() func() {
		var out api.MonitorDigestSettings
		var err error
		if save != nil {
			out, err = s.SaveMonitorDigest(context.Background(), *save)
		} else {
			out, err = s.MonitorDigest(context.Background())
		}
		return func() {
			if gen != a.gen {
				return
			}
			a.digest.loading, a.digest.busy = false, false
			if err != nil {
				a.digest.err = a.failed(err)
				return
			}
			a.conn = connOnline
			a.digest.settings, a.digest.loaded = out, true
			if save != nil {
				a.digest.notice = "Pengaturan ringkasan tersimpan."
			}
		}
	})
}

// monitorDigestSettings is Pengaturan → Ringkasan mingguan (canvas
// "Ringkasan mingguan").
func (a *App) monitorDigestSettings(c *ui.Context) {
	if !a.digest.loaded && !a.digest.loading && a.digest.err == "" {
		a.runMonitorDigest(nil)
	}
	settingsBody(c, "Ringkasan mingguan BPH", "Dikirim Senin 07.00 WIB ke pemegang izin pemantauan, lewat WhatsApp atau email. Isinya angka dan judul, tanpa isi catatan.", func() {
		if a.digest.err != "" {
			pageNotice(c, "alert", "Ringkasan belum dapat dimuat", a.digest.err)
		}
		if a.digest.notice != "" {
			pageNotice(c, "check", "Tersimpan", a.digest.notice)
		}
		if !a.digest.loaded {
			muted(c, "Memuat pengaturan ringkasan…")
			return
		}
		st := a.digest.settings
		if !st.Monitor {
			pageNotice(c, "lock", "Belum memegang izin pemantauan", "Ringkasan hanya dikirim kepada anggota BPH yang diberi izin pemantauan oleh admin pada periode berjalan.")
		}
		blocked := a.digest.busy || a.conn != connOnline
		pageSection(c, "Pengaturan", "Bisa dimatikan kapan saja, untuk akunmu sendiri.", func() {
			label, icon := "Matikan ringkasan mingguan", "close"
			if !st.Enabled {
				label, icon = "Nyalakan ringkasan mingguan", "check"
			}
			if spaceButton(c, label, icon, false).Disabled(blocked).Clicked() {
				a.runMonitorDigest(&api.MonitorDigestRequest{Enabled: !st.Enabled, Channel: st.Channel})
			}
			ui.Text(c, "Kirim lewat").Bold()
			ui.Row(c).Wrap().Gap(6).Children(func() {
				for _, ch := range []struct{ key, label, icon string }{{api.DigestWhatsApp, "WhatsApp", "whatsapp"}, {api.DigestEmail, "Email", "message"}, {api.DigestBoth, "Keduanya", "send"}} {
					if spaceButton(c.Key(ch.key), ch.label, ch.icon, st.Channel == ch.key).Disabled(blocked || !st.Enabled).Clicked() {
						a.runMonitorDigest(&api.MonitorDigestRequest{Enabled: st.Enabled, Channel: ch.key})
					}
				}
			})
			if st.Channel != api.DigestEmail && !st.WhatsAppLinked {
				muted(c, "WhatsApp-mu belum tertaut di periode ini, jadi ringkasan WhatsApp belum bisa terkirim. Tautkan lewat Pengaturan → WhatsApp.")
			}
		})
		if st.Preview != "" {
			pageSection(c, "Pratinjau minggu lalu", "Pesan pribadi dari bot Space. Email berisi angka yang sama dengan tabel per divisi.", func() {
				ui.Text(c, st.Preview).FontSize(13).Selectable()
			})
		}
	})
}
