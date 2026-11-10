# Onboarding, tur, dan Panduan (varian A)

Branch `feat/onboarding-panduan` dari `origin/main` f578874 (0.3.9). Desainnya adalah canvas `a036cb358772` "Space — Onboarding, tur & Panduan: opsi A/B/C"; varian A ditandai winner untuk semua bagian.

Branch ini **belum di-push ke main dan belum di-deploy**. Ada **satu migrasi aditif (0030)** dan empat endpoint baru. Server lama tetap aman: aplikasi baru hanya menyembunyikan onboarding bila endpoint-nya tidak ada.

## 1. Sambutan dan checklist "Langkah pertama"

**Sambutan** muncul satu kali setelah bergabung lewat undangan.
- Isinya: "Kamu sudah bergabung di <divisi>", chip role, periode, dan tiga baris tentang Space.
- Tombolnya "Mulai tur · 1 menit" dan "Nanti, ke Beranda". Menutup dengan Esc atau klik di luar juga dihitung sudah dilihat.

**Checklist di atas Beranda** berisi empat langkah, dan masing-masing tercentang sendiri:

| Langkah | Tercentang ketika |
|---|---|
| Tautkan WhatsApp | Server melihat `wa_identities` milik akun di periode itu. Selama belum tertaut, aplikasi bertanya lagi setiap 30 detik, jadi menautkan dari HP langsung mencentangnya. |
| Lengkapi profil | Nama tampilan disimpan lewat dialog Profil (`PUT /api/me/profile`). |
| Lihat tugasmu | Akun membuka Board tugas atau Jobdesk saya. |
| Kenali tab Space | Tur diselesaikan atau dilewati. |

Perilaku checklist:
- **Sembunyikan** tersedia kapan saja.
- Setelah keempat langkah selesai, kartu berubah menjadi "Langkah pertama selesai" dengan tombol **Ulangi tur** dan **Sembunyikan checklist**.
- **Progres disimpan per pengguna di server** (`user_onboarding.marks`), jadi tetap sama di desktop, web, dan HP.

**Akun lama**: migrasi 0030 menandai sambutan, tur, checklist, dan serah terima sebagai sudah dilihat, supaya pengurus pilot sekarang tidak tiba-tiba mendapat onboarding. Mereka tetap bisa membuka Panduan dan memakai Ulangi tur.

## 2. Tur sorotan per role

Lima langkah. Area yang disorot diberi bingkai aksen, sisa layar diredupkan, dan kartu langkah ditempatkan di sampingnya.

| Role | Langkah |
|---|---|
| Anggota | Sidebar → Board tugas → Catatan → Ping → Tombol ? |
| 45 (dan pendamping) | Sidebar → Board tugas → Catatan → Ping → Tombol ? |
| Kadiv | Sidebar → Board tugas → Catatan → Anggota → Tombol ? |

Anggota dan 45 melewati target yang sama, tetapi isi tiap langkahnya berbeda sesuai kewenangan.

- Kontrol: Lewati, Kembali, Lanjut, Selesai, dan Esc.
- Tur tampil sekali, karena tandanya `tour` disimpan di server. Tur bisa diulang dari **menu profil → Ulangi tur**, dari checklist yang sudah selesai, dan dari halaman Panduan.
- Kalau tab tujuan disembunyikan pengguna, kartu tampil di tengah dengan keterangan "Bagian ini tidak tampil di layarmu sekarang".
- Di HP, kartu tur muncul di bawah layar dan sorotan sidebar diganti navigasi bawah.

## 3. Panduan dan tombol ?

**Halaman Panduan** (`/panduan`, item "Panduan" di sidebar):
- Topik per role (Anggota, 45, Kadiv), defaultnya role akun sendiri.
- Ada pencarian dan kartu per topik.
- Artikel bergambar memakai ilustrasi skematis yang digambar langsung di aplikasi. Setiap artikel punya bagian "Terkait".

**Tombol ?** ada di bilah atas desktop dan HP.
- Membuka artikel untuk halaman yang sedang dibuka: Board atau Jobdesk, Catatan atau Rapat, Ping, Anggota, Bot WhatsApp, atau Beranda.
- Di desktop artikel tampil di **panel samping 400 px** yang meluncur masuk. Di HP tampil sebagai sheet dari bawah.
- Dari panel bisa "Buka di halaman Panduan".

**Isi panduan** berupa 12 artikel Markdown di `internal/guides/content/`, ditulis dalam bahasa sederhana:
- Memulai: mengenal Space, tautkan WhatsApp, apa itu 45, tombol ?.
- Tugas: membaca dan memindah kartu, membuat dan membagi tugas.
- Rapat: membaca catatan, menjadi notulis.
- Ping & bot WA.
- Anggota: undang lewat NIM dan role.
- Serah terima: checklist kadiv baru, pergantian periode.

Isinya mengikuti fitur yang benar-benar ada: perintah bot (`!tugas`, `!cek`, `!hari`, `!jadwal`, `!done`, `!bantu`, `!help`), kolom board, PINDAH, dan urutan hari H dari `period-rollover-20262027.md`.

**Bisa diperbarui tanpa rilis**: isi `SPACE_GUIDES_DIR` dengan folder `.md` di server.
- Server membaca folder itu di setiap permintaan, jadi perubahan langsung tampil.
- Kalau folder itu rusak, server kembali memakai panduan bawaan dan mencatat peringatan di log.
- Aplikasi memakai salinan bawaan kalau server lama atau sedang offline, dan saat itu menampilkan keterangannya.
- Format kepala artikel ada di `internal/guides/guides.go`.

## 4. Empty state yang membimbing dan serah terima kadiv

Setiap empty state berisi ilustrasi, apa yang perlu dilakukan dulu, tiga fakta singkat, dan tautan "Baca: <artikel>" yang membuka panel ?.

- **Board kosong**: kartu panduan muncul di atas kolom board, sementara kolomnya tetap tampil.
  - Untuk kadiv dan 45 ada tombol "Buat tugas pertama".
  - Untuk anggota, kartunya menjelaskan siapa yang membagi tugas.
- **Catatan kosong**: di desktop panduan tampil di panel detail, karena kolom daftar hanya 220 px. Di HP tampil di daftar. Tombolnya "Buat catatan rapat" dan "Cara membaca catatan".
- **Anggota kosong**: "Undang anggota lewat NIM" beserta cara mengundangnya.
- **Serah terima kadiv baru**: kartu di Beranda untuk kadiv dengan 5 langkah, masing-masing dengan tombol ke tab atau artikel terkait:
  1. Cek divisi.
  2. Undang lewat NIM.
  3. Atur role.
  4. Siapkan bot WA.
  5. Pergantian periode.

  Kartu ditutup dengan "Tandai selesai" (`handover`).

## 5. Motion

Kemampuan MyGo v0.7.1 sudah cukup, jadi tidak perlu cara cadangan:
- `Transition` dengan Enter, Exit, Size, dan Colors.
- `AnimateTo` untuk nilai yang beranimasi.
- `AnimatePose` untuk skala dan geser yang hanya menggambar ulang, tanpa membangun view lagi.

Semua durasi 150–300 ms dengan EaseOut atau EaseInOut, kecuali animasi centang yang memakai sedikit overshoot (`easeOutBack`).

| Bagian | Gerak |
|---|---|
| Sorotan tur | Posisi dan ukuran lubang meluncur ke target berikutnya (280 ms, EaseInOut). Kartu langkah masuk dengan fade, naik 6 px, dan skala .97→1. Titik progres melebar dan berubah warna. |
| Checklist | Lingkaran berubah warna menjadi hijau. Centangnya muncul dari skala .3 dengan overshoot (260 ms). Judul dicoret dan warnanya meredup. Progress bar meluncur ke nilai barunya (300 ms). |
| Semua selesai | Delapan titik aksen, hijau, dan kuning menyebar dari centang lalu memudar dalam kurang dari 1 detik, kemudian muncul kartu "selesai". |
| Panel ? | Meluncur dari kanan 32 px sambil fade dan melebar (260 ms). Sheet di HP naik 60 px. |
| Empty state | Kartu muncul pelan (fade, naik 12 px, 300 ms). Tiga faktanya menyusul satu per satu (jeda 60 ms). Kartu topik Panduan juga muncul bergiliran (jeda 45 ms). |
| Tombol | Saat ditekan mengecil menjadi .96 (180 ms). Hover mengubah warna secara halus (150 ms). Efek tekan ini juga dipasang pada semua tombol utama aplikasi (`primarySpaceButton`). |
| Dialog | Sambutan dan profil masuk dengan fade dan skala. Ketiga baris sambutan menyusul dari kiri. |

**prefers-reduced-motion** dihormati oleh MyGo sendiri lewat `Preferences.ReduceMotion`:
- Saat aktif, tidak ada yang bergeser atau berubah skala.
- Warna tetap berubah halus, dan elemen yang masuk atau keluar hanya memudar di tempat.
- Karena itu tidak ada pemeriksaan tambahan di kode.

**Batasan**: MyGo belum punya hit-test "lubang". Redupan tur terdiri dari empat kotak yang tidak menahan klik, jadi klik di luar kartu masih diteruskan ke halaman. Karena tur bisa dilewati kapan saja, ini saya biarkan.

## API dan data

- **Migrasi `0030_user_onboarding.sql`**: tabel `user_onboarding(user_id, marks jsonb, updated_at)`, beserta isian untuk akun lama.
- **`GET /api/me/onboarding?periodId=`**: mengembalikan `{marks, whatsappLinked}`.
- **`POST /api/me/onboarding`** dengan `{key, clear}`:
  - Kunci yang dikenali: welcome, profile, tasks, tour, checklistHidden, handover.
  - Waktu tanda pertama dipertahankan.
- **`PUT /api/me/profile`** dengan `{displayName}`:
  - Nama 2–80 huruf, spasi dirapikan.
  - Perubahan diaudit sebagai `account.profile.update` dan menandai `profile`.
  - Admin organisasi ditolak.
- **`GET /api/guides`**: mengembalikan `{guides, source: bawaan|berkas}`.

## Berkas

- **Server**:
  - `internal/api/onboarding.go`, `internal/guides/` (pemuat dan konten), `internal/store/onboarding.go`, `internal/server/onboarding.go`, `internal/client/onboarding.go`.
  - Diubah: `config` (`SPACE_GUIDES_DIR`) dan `.env.example`.
- **Aplikasi**:
  - `internal/appui/onboarding.go`: state, sambutan, checklist, perayaan, profil, tombol.
  - `onboarding_tour.go`: tur.
  - `guide.go`: halaman Panduan, panel, ilustrasi.
  - `onboarding_empty.go`: empty state.
  - Disambung di `chrome.go`, `topbar.go`, `responsive.go`, `sidebar.go`, `space_tabs.go`, `beranda.go`, `pages.go`, `tasks_view.go`, `notes_view.go`, `notes_gaya_c.go`, `members_table.go`, `appearance.go`, `icons.go`.
- **Test**:
  - `internal/guides/guides_test.go`: semua artikel valid; setiap role dan halaman punya artikel; ilustrasi dikenal; folder override dan slug ganda.
  - `internal/appui/onboarding_test.go`:
    - Sambutan lalu checklist, WhatsApp tercentang lewat refresh, board mencentang "Lihat tugasmu".
    - Tur anggota: maju, mundur, selesai, ulangi dari menu, Esc.
    - Tur kadiv dan kartu serah terima.
    - Profil dan perayaan selesai, lalu sembunyikan.
    - Akun lama tidak melihat onboarding.
    - Panel ? dan halaman Panduan: role, pencarian, artikel.
    - Empty state.
    - Capture.
  - `internal/server/onboarding_test.go`: tanda, hapus tanda, akun lain, validasi, profil dan audit, panduan.
    - Test ini **ter-skip di mesin ini** karena tidak ada `SPACE_TEST_DATABASE_URL`, dan kredensial Postgres lokal tidak tersedia. Jalankan dengan `make test-db` di mesin yang punya DB uji.
- **Screenshot** (renderer native, data dummy, tema gelap bawaan tester):
  - `docs/screenshots/onboarding-tur-desktop.png`: tur langkah 2, sorotan pada Board tugas di atas checklist 2/4.
  - `docs/screenshots/onboarding-panduan-desktop.png`: panel ? di Board tugas.
  - `docs/screenshots/onboarding-motion.gif`: sorotan meluncur ke langkah berikutnya, lalu panel ? meluncur masuk. Ukurannya setengah resolusi, sekitar 1,7 MB.

## Hasil cek (Windows)

```
gofmt -l (berkas yang diubah)        → (kosong)
go vet ./...                         → ok
go test ./internal/appui ./internal/guides → ok
go test ./...                        → semua ok kecuali internal/filestore (cek izin 0700, khusus Windows, tidak disentuh)
GOOS=js GOARCH=wasm go build ./internal/appui → ok
```

## {{Asumsi}}

- **Pendamping**: mendapat tur dan panduan role 45.
- **Lengkapi profil**: untuk sekarang hanya nama tampilan, karena Space belum punya foto profil.
- **Serah terima kadiv**: tidak tercentang otomatis, karena langkahnya tidak bisa dideteksi. Kadiv menutupnya dengan "Tandai selesai".

## 0.4.1: izin notifikasi dan tur per role (masukan user)

- **Langkah "Izinkan notifikasi"** di checklist. Langkah ini mengikuti izin di perangkat yang sedang dipakai, bukan disimpan di server:
  - **Web**: membaca `Notification.permission`.
  - **Desktop**: membaca setelan notifikasi Windows (`ToastNotifier.Setting`). Karena butuh PowerShell, nilainya dibaca di latar belakang setiap menit.
  - Bila sudah `granted`, langkah tercentang sendiri.
  - Bila `default`, tombol **Izinkan** memunculkan prompt browser.
  - Bila `denied`, langkah menampilkan cara mengizinkannya lagi (gembok → Setelan situs, atau pengaturan notifikasi Windows), dan tombol **Lihat caranya** membuka Pengaturan › Perangkat.
  - Perangkat tanpa dukungan notifikasi tidak mendapat langkah ini.
- **Notifikasi uji** "Notifikasi aktif ✓ — Kamu akan dapat kabar tugas dan ping di sini."
  - Dikirim begitu izin berubah dari belum diizinkan atau diblokir menjadi diizinkan selama Space terbuka, apa pun yang memintanya: checklist, Pengaturan, atau setelan situs.
  - Izin yang sudah ada saat aplikasi dibuka tidak memicu notifikasi uji.
  - Kodenya ada di `watchNotificationPermission`.
- **Pesan uji WhatsApp** "Tersambung ✓ Kamu akan menerima ping dan pengingat dari Space di sini."
  - Diantrekan sekali per akun setelah tautan pertama berhasil, juga lewat PINDAH. Pesan masuk ke chat pribadi nomor itu, 3 detik setelah balasan "Tertaut ✓", juga bila kodenya dikirim di grup.
  - Pengiriman ditandai di server dengan tanda `waHello`, yang tidak bisa diubah klien.
  - Test: `TestWALinkHelloOnce`.
- **Lapisan notifikasi bersama**, `internal/appui/notifier.go`:
  - `Notifier` terdiri dari `Permission`, `RequestPermission`, dan `Present(Notice)`. `NotifierHint` opsional untuk petunjuk saat izin diblokir.
  - Implementasinya `webNotifications` (popup browser) dan `desktopNotifier` (notifikasi OS).
  - "Dynamic island" desktop nanti cukup berupa `NoticePresenter` yang dipasang lewat `MultiNotifier{Base: osNotifier, Extra: []NoticePresenter{island}}`, tanpa mengubah appui. Island-nya belum dibuat.
- **Tur**:
  - `tourSteps(role, visibleTab)` hanya memilih tab yang memang tampil untuk akun: diizinkan role dan tidak disembunyikan.
  - Langkah WhatsApp memakai Ping bila tab itu ada, lalu Bot WhatsApp, dan dilewati bila keduanya tidak ada. Langkah lain juga dilewati bila tabnya tidak ada.
  - Keterangan "Bagian ini tidak tampil di layarmu sekarang" dihapus.
  - Test: `TestOnboardingTourOnlyVisibleTargets`.
