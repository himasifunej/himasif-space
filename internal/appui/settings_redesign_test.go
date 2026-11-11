package appui

import (
	"strings"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// redesignHarness is a kadiv of Mediatek with the bot connected, a sticker
// command, a day of deliveries, and the division's reminder schedule.
func redesignHarness(t *testing.T) *harness {
	t.Helper()
	h := waHarness(t, 1440, 900, "connected", true, []bool{true, false})
	st := &h.app.whatsapp
	group := api.WAGroup{ID: id.New(), Name: "Mediatek 2025/2026"}
	st.connection.Groups = []api.WAGroup{group}
	st.commands = []api.WACustomCommand{
		{ID: id.New(), Trigger: "halo", Response: "Halo {sender}, selamat datang di {group}.", GroupIDs: []id.ID{}, Version: 2},
		{ID: id.New(), Trigger: "stiker", AssetID: strings.Repeat("a", 64), MediaType: "sticker", GroupIDs: []id.ID{group.ID}, Version: 1},
	}
	now := time.Now()
	st.outbox = []api.WAOutboxItem{
		{ID: id.New(), GroupName: "Mediatek 2025/2026", Status: "read", Attempts: 1, CreatedAt: now.Add(-time.Hour)},
		{ID: id.New(), GroupName: "Aruna Desain", Status: "delivered", Attempts: 1, CreatedAt: now.Add(-2 * time.Hour)},
		{ID: id.New(), GroupName: "Mediatek 2025/2026", Status: "unknown", Attempts: 2, LastError: "Hasil kirim belum pasti; menunggu pemeriksaan jurnal bridge.", CreatedAt: now.Add(-3 * time.Hour)},
		{ID: id.New(), GroupName: "Mediatek 2025/2026", Status: "failed", Attempts: 3, LastError: "Bridge tidak tersambung.", CreatedAt: now.Add(-10 * time.Hour)},
	}
	return h
}

func (h *harness) openSettings(tab string) {
	h.app.chrome.settingsTab = tab
	h.app.navigate(PathPengaturan)
	h.tt.Advance(400 * time.Millisecond)
	h.tt.Frame()
	if tab == "Pengingat" {
		st := &h.app.notificationSettingsState
		st.loaded, st.busy = true, false
		st.days, st.hour, st.minute, st.overdue = "2,1,0", "6", "0", "1"
		h.tt.Frame()
	}
}

// Every redesigned page shows its rows (canvas c66c26d3f3e1, the owner's
// set); SPACE_UI_CAPTURE_DIR keeps a picture of each at laptop width.
func TestSettingsRedesignPages(t *testing.T) {
	for _, page := range []struct {
		tab  string
		want []string
	}{
		{"Akun", []string{"Akun dan aplikasi", "Akunmu", "Jenis akun", "Keamanan akun", "Kata sandi", "Aplikasi ini", "Keluar dari akun"}},
		{"Tampilan", []string{"Tema", "Mode", "Palet Copper", "Warna aksen", "Board tugas", "Pratinjau tampilan"}},
		{"Sinkronisasi", []string{"Sinkronisasi perangkat"}},
		{"Space", []string{"Identitas space", "Jenis", "Ruang divisi", "Akses", "Diatur di tempat lain", "Kelola anggota dan akses"}},
		{"Pengingat", []string{"Jadwal pengingat tugas", "Hari pengingat", "H-3", "H-2", "Hari H", "Jam kirim", "Lewat tenggat", "Contoh", "Simpan jadwal divisi"}},
		{"Command", []string{"Command bot", "!halo", "!stiker", "Perintah inti, mengikuti izin tugas:", "Buat perintah baru"}},
		{"Aktivitas", []string{"Aktivitas WhatsApp", "Semua 4", "Bermasalah 2", "Dibaca", "Terkirim", "Hasil belum pasti", "Gagal", "Bridge tidak tersambung."}},
		{"Tentang", []string{"Tentang Space", "Versi", "Server", "Riwayat perubahan"}},
	} {
		t.Run(page.tab, func(t *testing.T) {
			h := redesignHarness(t)
			h.openSettings(page.tab)
			if page.tab == "Command" {
				h.click("!halo") // the editor and its preview, for the picture
				h.tt.Frame()
			}
			for _, want := range page.want {
				h.want(want)
			}
			captureNativeUI(t, h, "settings-"+strings.ToLower(page.tab))
		})
	}
}

// Day chips toggle; a custom H-N joins the row, largest first.
func TestReminderDaysCustom(t *testing.T) {
	h := redesignHarness(t)
	h.openSettings("Pengingat")
	st := &h.app.notificationSettingsState
	h.click("Ingatkan H-1")
	h.tt.Frame()
	if st.days != "2,0" {
		t.Fatalf("H-1 off: %q", st.days)
	}
	st.customDay = "7"
	h.tt.Frame()
	h.click("Tambah")
	h.tt.Frame()
	if st.days != "7,2,0" || st.customDay != "" {
		t.Fatalf("custom day: %q, typed %q", st.days, st.customDay)
	}
	h.want("H-7")
	st.customDay = "x"
	h.tt.Frame()
	h.click("Tambah")
	if st.err == "" || st.days != "7,2,0" {
		t.Fatalf("bad custom day accepted: %q %q", st.err, st.days)
	}
	for _, hour := range []int{0, 9, 23} {
		now := time.Date(2026, 11, 11, hour, 59, 0, 0, time.UTC) // a Wednesday
		if got := scheduleExample(now, []int{2, 1, 0}, "6", "0"); got != "Tugas bertenggat Jumat 13 Nov diingatkan Rabu, Kamis dan Jumat pukul 06.00." {
			t.Fatalf("%d.59: example %q", hour, got)
		}
	}
}

// Bermasalah keeps what failed or is uncertain; a sticker command says so.
func TestActivityFilterAndStickerCommand(t *testing.T) {
	h := redesignHarness(t)
	h.openSettings("Aktivitas")
	h.click("Bermasalah 2")
	h.tt.Frame()
	h.dontWant("Dibaca")
	h.want("Gagal")
	h.want("Hasil belum pasti")

	h.openSettings("Command")
	h.click("!stiker")
	h.tt.Frame()
	h.want("Stiker")
	h.want("Lepaskan media dari command")
	h.click("!halo")
	h.tt.Frame()
	h.want("Agar perintah ini membalas dengan stiker: di grup WhatsApp, balas gambar, video pendek, atau stiker dengan !setsticker halo.")
	h.want("Contoh di grup WhatsApp")
}

// Tentang opens the newest version and folds the rest.
func TestAboutFolds(t *testing.T) {
	h := redesignHarness(t)
	h.openSettings("Tentang")
	if len(Changelog) < 2 {
		t.Skip("one version only")
	}
	h.want(Changelog[0].Notes[0])
	h.dontWant(Changelog[1].Notes[0])
	h.click("Buka catatan " + Changelog[1].Version)
	h.tt.Frame()
	h.want(Changelog[1].Notes[0])
}
