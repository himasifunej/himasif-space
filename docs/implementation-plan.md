# Rencana implementasi Space per sesi

Tanggal: 27 Oktober 2026, WIB. Acuan: [PRD](PRD.md), [Register keputusan](decision-register.md), [Riset Android](research/android-options.md).

> **Sejak 25 November 2026 yang berlaku adalah bagian [Menuju 1.0.0](#menuju-100) di akhir dokumen.** Tahap 0–6 di bawah adalah roadmap historis; Tahap 6 (Flutter) dicabut oleh keputusan owner K-16.

Dokumen ini menjabarkan urutan yang disepakati. **Sesi 1A merged melalui PR #1** (`627298f`), **backend 1B–1C melalui PR #2** (`d92a372`), **backend reminder melalui PR #3**, dan **UI native beserta inti tugas melalui PR #4** (`8608fc4`). Penyambungan UI reminder pribadi melengkapi hasil gabungan tersebut; lihat [handoff integrasi tugas](handoff/native-integration-tasks.md) dan [handoff UI reminder](handoff/personal-reminders-ui.md). Pengguna menyetujui prototype UI 03.1. Status tiap tahap tetap bergantung pada verifikasi fungsinya, bukan sekadar tersedianya tampilan. Tidak ada tanggal rilis atau estimasi durasi yang disepakati. Satu tahap dapat memakai beberapa sesi; jangan menganggap seluruh aplikasi selesai dalam enam percakapan.

## Prinsip handoff setiap sesi

Sebelum mengubah kode, baca PRD, koreksi keputusan, keadaan proyek dan instruksi lokal yang berlaku. Bawa hanya scope sesi dan dependensinya. Hasil sesi wajib mencatat:

- Apa yang dibuat dan kebutuhan PRD yang dipenuhi.
- Keputusan teknis baru beserta alasan, tanpa mengganti keputusan produk diam-diam.
- Cara menjalankan/mencoba hasil dan verifikasi yang benar-benar dijalankan.
- Batas yang belum selesai, migrasi/perubahan data, dan dependensi sesi berikutnya.
- Skenario penerimaan relevan yang sudah lulus atau belum bisa diuji.

Jangan menyalin secret akun/bot ke dokumen handoff. Jangan mengubah Reo, sheet produksi, DNS atau deployment live sebagai efek samping membuat prototype lokal.

## Status implementasi 28 Oktober 2026

Implementasi lokal Sesi 2C dan seluruh Tahap 3A–3C ditulis sesuai permintaan pengguna, dengan aplikasi versi 0.2.0. Pengguna meminta **tanpa verifikasi/pengujian** dan akan mengujinya sendiri. Tidak menjalankan build, tes, vet, inspeksi runtime/Windows, pengukuran performa atau migrasi database. Perubahan sesi ini berada pada worktree; belum diklaim terbit/merged di GitHub atau siap produksi. Lihat [handoff](handoff/session-2c-stage-3.md) dan [panduan uji mandiri](handoff/session-2c-stage-3-manual.md).

## Tahap 0 — Dokumentasi sebelum implementasi

- [x] Menyusun PRD lengkap.
- [x] Menelusuri keputusan Q1–Q74 dan koreksi terakhir.
- [x] Menyusun roadmap sesi dan penerimaan.
- [x] Menyelesaikan rancangan teknis Sesi 1A; lihat [ADR 0001](adr/0001-fondasi-teknis.md).

Keluaran: dokumen ini dan PRD; note HIMASIF diperbarui dengan PRD lengkap serta ringkasan navigasi. Identitas visual Space dan logo divisi tersedia dari prototype 03.1; Ace masih kebutuhan desain. Aset UI belum menggantikan kebutuhan ikon paket aplikasi dan semua ukuran distribusi.

## Tahap 1 — Fondasi

### Sesi 1A: struktur aplikasi, model identitas dan branding awal

Status: **selesai pada main**, PR #1. Repository `himasifunej/himasif-space` sudah tersedia, dengan kerangka MyGo native, API/auth, migrasi/seed PostgreSQL, email dan pengujian fondasi. Kontrak runtime dan keputusan arsitektur mengikuti ADR 0001.

Pekerjaan:

- Pilih stack frontend web, database, auth dan kontrak antarlayanan sesuai `OPEN-02`.
- Rancang Go API/server terpisah dari klien MyGo, web dan bridge WA.
- Tetapkan model ID stabil, waktu/zona, audit, periode dan batas privasi.
- Susun UX/navigasi awal; siapkan eksplorasi logo Space dan karakter Ace sebelum aset final dipakai.
- Siapkan konfigurasi pengembangan, penyimpanan secrets yang tepat dan backup dasar ketika mulai ada data nyata.

Keluaran: aplikasi kerangka yang bisa dijalankan, keputusan teknis terdokumentasi, bukan seluruh integrasi aktif.

### Implementasi UI native paralel

Status: UI native dan backend sudah digabung ke main melalui PR #3/#4, kemudian dilengkapi UI reminder. [Handoff UI awal](handoff/native-ui.md) mencatat tahap sebelum integrasi; [integrasi tugas](handoff/native-integration-tasks.md) dan [integrasi reminder](handoff/personal-reminders-ui.md) mencatat perilaku berikutnya.

- Shell native, sidebar tetap/collapse, header/search shortcut, dropdown notifikasi/profil, logout, status koneksi.
- Halaman divisi/Medhum, beranda, board, agenda, notes/template dan pengaturan WhatsApp mengikuti prototype yang disetujui.
- Tema dark/light/system, aksen preset/kustom dan density disimpan per perangkat.
- Board drag/drop, reorder dan panel detail kanan sudah terhubung ke layanan tugas dengan identitas tugas yang sama pada proyeksi pribadi/divisi/bersama.
- Login, alur akun, daftar Space berizin, anggota, role dan inti tugas/reminder memakai API. Hak akses tetap diputuskan server. Notes sudah memakai backend Sesi 2B beserta UI editor native; rapat, bot dan pengiriman notifikasi masih mengikuti tahap berikutnya; tampilannya tidak mengklaim penyimpanan atau koneksi yang belum tersedia.

### Sesi 1B: akun, undangan dan periode

Status: backend selesai di main (PR #2); UI undangan, pemulihan, perubahan kredensial dan keanggotaan terhubung melalui PR #4. Form administrasi periode, penetapan kadiv, checklist serah terima dan pemberian/pencabutan izin arsip kini juga terhubung; lihat [handoff administrasi](handoff/admin-ui.md). Pemilih target arsip memakai endpoint metadata khusus tanpa otomatis membuka isi Space lama.

- Akun baru/lama, verifikasi email, aktivasi undangan, pencabutan dan pemulihan akun.
- Satu akun admin organisasi, terpisah dari akun pribadi; pengelolaan periode/kadiv.
- Keanggotaan/role per periode, akses pengurus lama dan izin arsip pengurus baru.
- Checklist serah terima dan penutupan periode yang tidak menghapus data.

Penerimaan: AC-01, AC-02, AC-10, AC-11; batas email/provider/umur undangan ditetapkan eksplisit.

### Sesi 1C: Space dan role

Status: backend selesai di main (PR #2); UI daftar Space berizin, anggota, kelompok, role kustom dan 45 terhubung melalui PR #4.

- Space divisi, Medhum, pengelompokan Tekno/Desain/Video dan role kustom.
- Anggota melihat ruang divisi; pemisahan kerja internal/bersama.
- Pendamping, satu role 45 aktif, larangan role/akses bagi 45 termasuk lewat API.
- Web awal untuk login dan navigasi ruang.

Penerimaan: AC-03, AC-06–08; belum boleh ada privilege yang hanya dibatasi lewat tombol UI.

## Tahap 2 — Pekerjaan, dokumentasi dan rapat

### Sesi 2A: tugas dan jobdesk

Status: inti tugas/jobdesk tersimpan, kanban/reorder, delegasi, bantuan, bukti/tinjauan, komentar, riwayat dan pembaruan in-app sudah merged melalui PR #4. Backend reminder pribadi PR #3 kini tersambung ke UI native: buat/ubah/selesai, preferensi default, berbagi per akun, pencabutan dan riwayat. Lihat [review gabungan](handoff/pr3-pr4-review.md) dan [integrasi UI reminder](handoff/personal-reminders-ui.md). Pengiriman reminder terjadwal/toast OS tetap Sesi 2C/3A, bukan bagian penyimpanan reminder di 2A.

- Tugas dengan hasil, PJ utama, deadline, pelaksana tambahan, progres dan komentar.
- Jobdesk pribadi/divisi/ruang bersama dari satu objek tugas.
- Pengajuan bantuan dan tinjauan; riwayat perubahan PJ/deadline.
- Reminder pribadi dan toggle share individual, bukan pewarisan berdasarkan jabatan.
- Status penyerahan/pencocokan terpisah, tanpa workflow revisi desain wajib.

Penerimaan: AC-03–05, AC-09; server menjadi sumber hak akses di semua tampilan.

### Sesi 2B: notes dan template

Status: backend Notes PR #6 sudah merged; UI native menyediakan dokumen blok, template, komentar, kunci editor per sesi, pemulihan draf dan konversi tugas/reminder. Lampiran masih metadata; unggah berkas biner dan hubungan ke objek rapat mengikuti tahap berikutnya. Lihat [handoff Notes UI](handoff/notes-ui.md). Pengujian tambahan dilewati sesuai permintaan pengguna.

- Rich notes, template rapat/jadwal/catatan bebas, penyimpanan template lain.
- Tabel jadwal, lampiran, komentar dan satu editor aktif.
- Jadikan teks/baris sebagai tugas atau reminder dengan pemeriksaan manusia.
- Simpan scope dan hubungan ke sumber, tanpa membagikan seluruh note privat otomatis.

Penerimaan: AC-33–34; dasar versi/draf disiapkan untuk offline.

### Sesi 2C: rapat dan penjadwal pengingat

Status: **implementasi lokal ditulis, menunggu pengujian pengguna**. Rapat, peserta/kehadiran, notulensi, scheduler/outbox, kebijakan divisi/tugas, ping, inbox, sudah lihat dan snooze tersedia pada sumber. Adapter WA mencatat belum terhubung; tidak mengklaim pengiriman WA nyata. Lihat handoff 2C/3.

- Jadwal, peserta, lokasi/link, note, kehadiran/waktu datang oleh penyelenggara.
- H-2/H-1/H-0 06.00 WIB untuk tugas; rapat H-1/1 jam/15 menit; overdue harian.
- Ping default/kustom, umum/tugas/rapat, pilihan kanal, sudah lihat dan snooze.
- Kebijakan slot terlewat, deduplikasi dan perubahan/cancel; notifikasi in-app/web dasar.

Penerimaan: AC-25–29, AC-35. Kanal desktop/WA bisa memakai adapter uji sampai tahap terkait; tidak mengklaim pesan nyata sudah terkirim.

## Tahap 3 — Desktop dan offline

### Sesi 3A: MyGo, tray dan notifikasi Windows

Status: **implementasi lokal ditulis, belum dibangun/diuji**. Tray/keluar, autostart, helper WinRT toast modern dengan AUMID/protokol, pembukaan objek, bel bawaan/WAV dan volume perangkat. Pengujian Windows nyata, pengaturan sistem/suara dan distribusi tetap menunggu pengguna.

- MyGo memakai UI native Go yang sama pada desktop dan WASM sesuai ADR 0001; lifecycle tutup-ke-tray dan keluar eksplisit.
- Opsi mulai saat login, toast Windows modern, klik membuka tugas/rapat.
- Bel bawaan, suara/volume pengguna, impor audio dan satu bel per kejadian.
- Verifikasi pada Windows nyata; host Linux saja tidak cukup membuktikan toast berfungsi.

Penerimaan: AC-39–40. Periksa kemampuan versi MyGo saat implementasi; kebutuhan toast tidak diganti menjadi balloon hanya karena API bawaan lebih mudah.

### Sesi 3B: Ace dan personalisasi

Status: **implementasi lokal ditulis, belum diuji/diukur**. Ace SVG bawaan, mode floating/edge/hidden, drag/posisi/pemulihan monitor, dialog dan impor PNG/GIF; preferensi perangkat tersimpan. Pengukuran idle/animasi dan perilaku monitor menunggu pengguna.

- Aset Ace default dan identitas logo tetap melalui proses desain.
- Mengambang, tepi/hover seperti screenshot, disembunyikan, pemulihan dari tray.
- Dialog reminder/ping serta aksi buka/sudah lihat/snooze; posisi multimonitor.
- Pilih/impor maskot sederhana; tidak membangun editor animasi penuh.
- Ukur idle/animasi dan pastikan mode tersembunyi hemat kerja render.

Penerimaan: AC-41–42; notifikasi tetap hidup saat Ace disembunyikan.

### Sesi 3C: cache, antrean dan konflik offline

Status: **implementasi lokal ditulis, belum diuji**. Cache akun untuk data yang diunduh, antrean progres/Notes/respons pengingat, simpan atomik, receipt server, penggabungan bidang berbeda, konflik eksplisit, penolakan izin dan status sinkronisasi. Lampiran pending tidak diklaim sebagai file cloud; unggah biner tidak ditambahkan. Skenario restart/retry/pergantian akun menunggu pengguna.

- Cache tugas/jadwal/notes, draf lokal dan antrean perubahan.
- Indikator offline/lokal/belum cloud/menyinkronkan/tersinkron.
- Penggabungan bidang berbeda dan resolusi konflik bidang sama dengan kedua versi terjaga.
- Periksa ulang hak saat sync; operasi delegasi/role/bot tetap online.
- Isolasi cache per akun, pencabutan akses dan penanganan lampiran pending.

Penerimaan: AC-36–38, termasuk restart perangkat sebelum sinkronisasi dan izin yang berubah.

## Tahap 4 — Medhum dan bot

Checkpoint lokal terbaru: 4A dan sebagian besar alur 4B sudah ditulis pada worktree, termasuk worker bridge/inbox/outbox, identitas/command dan UI pengelolaan WhatsApp. Belum build/test/migrasi atau pairing nyata. Inti parser/bukti dan UI tinjauan 4C juga sudah ditulis; belum dicocokkan dengan contoh broadcast asli atau diuji. Transport telah dipisahkan dari transaksi panjang dan fondasi evaluator/API konfigurasi/konteks/izin SOP 4D ditulis. Backend worker temuan, keputusan/riwayat, follow-up H-4 serta rekap/quarter/CSV 4D juga ditulis. UI Evaluasi, ralat lintas divisi/paginasi, bahasa respons inti, acceptance 4B/4C/4D dan tahap 5–6 masih tersisa. Instruksi kelanjutan adalah menyelesaikan semua tahapan lalu build/test di akhir. Lihat [worklog aktif](handoff/remaining-stages-worklog.md).

### Sesi 4A: kalender, konektor dan kepemilikan bidang

- Model konten/tahap Humas/desain, kedua PJ dan deadline desain H-2 sekali.
- Impor link sheet, pemetaan kolom, fetch berkala/manual dan log sumber.
- Fetch konsep/caption Trello, mapping kartu stabil.
- Mirror kolom aplikasi yang disepakati, menjaga formula/status/bukti.
- Tetapkan jam posting/cutoff dan konfigurasi kasus date-only sebelum penilaian berbasis jam.

Penerimaan: AC-12–13, AC-22–24; sheet kosong/rusak tidak menghapus tugas. Jadwal polling/kolom nyata diputuskan sebelum menulis sumber produksi.

### Sesi 4B: sesi WA dan command

- Bridge Node.js/Baileys, QR, sesi akun terpisah, grup Humas/Medhum dan Mediatek.
- Kewenangan kadiv/45, status koneksi, antrean/status kirim, retry terukur.
- Bahasa/nama/template, command tugas/jadwal/done, stiker dan custom `!damn` per divisi.
- Pemetaan identitas dan metadata mention; pengamanan scope grup/pengirim.

Penerimaan: AC-06, AC-29–32. Uji dua sesi tanpa menjanjikan kapasitas dari perkiraan.

### Sesi 4C: broadcast Humas dan bukti Drive

- Parser tanpa AI memakai contoh template nyata dan variasi yang disepakati.
- Idempotensi pesan/daftar ulang, waktu pertama per tahap dan notifikasi item baru.
- Link Drive tanpa wajib ID: satu kandidat → penyerahan sementara; beberapa kandidat → tinjauan.
- Balasan bot menyebut perlu tinjauan; penyerahan tidak berarti publikasi/approval mutu.
- Riwayat pesan/bukti dan perbaikan salah pasangan tanpa kehilangan waktu kejadian asli.

Penerimaan: AC-14–18, AC-29. Jangan menggeneralisasi satu link menjadi semua tugas selesai.

### Sesi 4D: SOP, rekap dan evaluasi

- Follow-up H-4, aturan/pengecualian SOP, bukti cukup/tidak cukup dan temuan awal.
- Tinjauan kadiv/Litbang, pengesahan/pembatalan/ralat beralasan dan audit.
- Rekap mingguan ke grup divisi masing-masing; quarter Litbang.
- Tetapkan slot rekap/batas quarter dan kesepakatan pemenuhan SOP melalui bot.
- Tidak mengumumkan keputusan ke Medhum atau menulis dugaan sebagai pelanggaran final legacy.

Penerimaan: AC-19–21, serta bukti tepat waktu tetapi klik selesai terlambat.

## Tahap 5 — Operasional dan pilot

### Sesi 5A: backup, restore dan runbook

- Lengkapi backup yang telah dimulai sejak data nyata, lokasi terpisah, lampiran/konfigurasi/audit.
- Tentukan retensi, target kehilangan data/waktu pemulihan dan pengelola.
- Spreadsheet darurat dengan waktu sync dan proses rekonsiliasi perubahan.
- Uji restore pada lingkungan terpisah; cegah pengiriman reminder historis massal.
- Runbook putus WA, kegagalan sumber, serah terima admin/bot dan pemulihan versi desktop.

Penerimaan: AC-43–44; bukan hanya bukti bahwa file backup berhasil dibuat.

### Sesi 5B: migrasi, distribusi dan uji pengguna

- Inventaris sumber Reo dan mapping status/approval/bukti lama sebelum cutover.
- Mode pembandingan tanpa mengirim duplikat, validasi data dan rollback.
- Alihkan pengirim/scheduler produksi satu kali, lalu pantau hasil nyata.
- Distribusi Windows, web pada domain yang tersedia dan rencana update/rollback.
- Uji Mediatek–Humas, peran Litbang, privasi, performa dan ukuran keberhasilan.

Penerimaan: seluruh kriteria pilot pada PRD §24, AC-45 dan hasil pengukuran yang dicatat. Jangan menyatakan siap hanya karena seluruh halaman dapat dibuka.

## Tahap 6 — Mobile

> **Dicabut 25 Nov 2026 (K-16):** tanpa Flutter. Mobile memakai PWA dari build web yang sama (F1-20, 1.0.x) sambil menunggu MyGo mendukung mobile. Isi tahap ini disimpan sebagai riwayat.

### Sesi 6A: Flutter Android

- Gunakan API/akun/periode/otorisasi yang sama, bukan database tugas kedua.
- UI mobile jobdesk, ruang, reminder pribadi, notes dan rapat sesuai prioritas rilis mobile.
- Push, deep link ke tugas/rapat, izin notifikasi dan status sinkronisasi.
- Uji di HP sasaran: idle, layar terkunci, background, restart, daftar panjang dan koneksi tidak stabil.
- Ukur release untuk baterai/RAM/ukuran; tidak mengulang kompetisi framework sebagai syarat mulai.

Penerimaan: AC-46 dan uji lintas klien terhadap status tugas yang sama. Cakupan rinci mobile disusun saat tahap ini karena tidak harus membawa seluruh panel administrasi desktop sekaligus.

### Sesi 6B: penilaian iOS

- Periksa kebutuhan pengguna iPhone dan akses pengujian perangkat.
- Siapkan macOS/Xcode lokal/remote/CI, signing dan distribusi sesuai kebutuhan saat itu.
- Verifikasi plugin/push dan perilaku UI pada iOS; tidak mengklaim APK dapat dipakai sebagai aplikasi iOS.

Status: kemungkinan pengembangan berikutnya, belum jadwal rilis.

## Backlog lanjutan

- App-only untuk konsep/caption/kalender dan board pengganti Trello.
- CMS himasif.id untuk Tekno; petakan akses/API sebelum menulis database web.
- Ulang tahun, Love Letter Kader, fitur PSDM/BPH/Litbang lain yang masih TBC.
- Perluasan pengguna wajib ke semua pengurus setelah kebutuhan divisinya tersedia.
- AI/agent, editor animasi penuh dan kolaborasi notes simultan bila benar-benar diperlukan.

Setiap backlog lanjutan memerlukan rincian kebutuhan tersendiri. Roadmap ini mempertahankan tujuan awal tanpa menganggap semua TBC telah selesai hanya karena PRD pilot lengkap.

## Koreksi scope dan checkpoint 29 Oktober

Pengguna meminta kelanjutan **tanpa mobile**, hasil akhir bisa diunduh, build/test tetap di akhir. Tahap 6A/6B pada roadmap historis di atas ditunda dan tidak menjadi pekerjaan aktif. Tahap 2C–5B beserta sisa integrasi, operasional/pilot dan verifikasi lengkap tetap dilanjutkan. UI Evaluasi, ralat lintas divisi, riwayat/prasyarat pengiriman dan ekspor CSV sudah ditulis; lampiran biner Notes mulai dilengkapi. Semua perubahan terbaru belum dibuild/diuji/dimigrasikan. Lihat [checkpoint](handoff/sop-ui-files.md) dan [worklog](handoff/remaining-stages-worklog.md). Paket rilis unduhan belum tersedia.

## Menuju 1.0.0

Tanggal: 25 November 2026, WIB. Sumber: keputusan owner 25 Nov 2026 ([Register](decision-register.md), bagian terakhir; [PRD](PRD.md) §2.3 dan §24.1) dan note rencana owner "Audit Space — Rencana" (`8d4b128e8a55`), yang memuat rincian tiap item (berkas, uji, ketergantungan) dan status centangnya. Bagian ini ringkasan untuk orang yang hanya membaca repo; bila berbeda, note rencana dan register yang berlaku.

**Sasaran:** 1.0.0 stabil di `space.himasif.id` sebelum pengurus baru mulai (**paling lambat Senin 7 Des 2026**): Mediatek, Humas dan BPH berjalan penuh dan ramah pengguna awal, dan alur Medhum (broadcast → desain → ACC bersama → posting) jalan lintas divisi.

### Jadwal

| Tanggal | Kegiatan |
|---|---|
| Rab 25 – Jum 27 Nov | Gelombang 1 (dokumen, `make check` di Windows, menu SOP tersembunyi, parser broadcast Humas, pengaman server). Branch `release/1.0.0` menjadi tujuan PR |
| Jum 27 – Sab 28 Nov | Uji Mediatek putaran 1 di pilot ([skrip](handoff/uji-pengguna-mediatek.md)) |
| Sab 28 Nov – Sel 1 Des | Gelombang 2 (ACC bersama Medhum, "Tetap masuk", keluar akun membersihkan perangkat, deploy ke repo) |
| Sen 30 Nov | Instans stabil disiapkan kosong; alarm; record DNS `space.himasif.id` aktif |
| Sel 1 Des | Gladi pulih; gelombang 3 (alur anggota dari WA, kontras, `!done T12`, panduan awal) |
| Rab 2 Des | Kunci fitur; 1.0.0-rc.1 ke instans uji lewat kanal rc |
| Kam 3 – Jum 4 Des | Uji Humas + alur Medhum dan BPH ([skrip BPH](handoff/uji-pengguna-bph.md)), ulang singkat Mediatek; perbaikan menjadi rc.2 |
| Sab 5 Des | Terbitkan 1.0.0; instans stabil dibuka |
| Min 6 – Sen 7 Des | Undang kadiv dan BPH baru, lalu anggota Mediatek, Humas dan BPH |

### Wajib 1.0.0

| Kelompok | Item |
|---|---|
| A. Fondasi dan pengaman | F1-01 `make check` hijau di Windows · F1-10 keputusan 25 Nov ke repo · F0-08 keputusan privasi (NIM pemantau, bacaan admin) · F0-04 `recover` worker latar · F0-03 pembatas laju dan login · F0-10 periksa server · F0-02 pengaman repo · F2-12 form grill-me |
| B. Mediatek dan pengalaman awal | F1-24 uji Mediatek · F0-06 keluar akun membersihkan perangkat · F2-01 "Tetap masuk" ([ADR 0003](adr/0003-instans-dan-sesi-1-0.md)) · F1-04 alur anggota dari WA · F1-05 kontras dan warna status · F2-02 `!done T12` · F1-23 onboarding dan panduan pasang |
| C. Humas dan alur Medhum | F1-06 parser broadcast membaca template nyata · F1-28 ACC bersama, broadcast → konten, kabar dua sisi · F1-25 menu SOP disembunyikan tetapi disiapkan · F1-19 uji Humas dan alur Medhum (Reo tetap jalan) |
| D. BPH | F1-26 uji BPH: kontrol sederhana dan rapat |
| E. Gerbang rilis | F1-21 domain `space.himasif.id` · F1-14 deploy ke repo · F0-14 jebakan rollback · F0-16 DB stabil dengan periode 2026/2027 · F0-12 rahasia pemulihan · F0-13 alarm email · F0-11 gladi pulih · F1-22 instans dan DB stabil · F1-18 pengaman rilis desktop · F1-13 terbitkan 1.0.0 |

Sesudah 1.0.0 (1.0.x, 8–20 Des): istilah baku di seluruh aplikasi (F1-08), PWA (F1-20), retensi WA 1 hari (F1-11), bridge yang terlihat (F1-17), akun demo (F0-01), dan sisa daftar di note rencana.

### Aturan kerja menuju 1.0.0

- Branch dari `main`, PR **draft** ke `release/1.0.0`; owner yang menggabungkan. Urutan gabung gelombang 1: dokumen (F1-10) → `.gitattributes` (F1-01) → yang lain, lalu branch lain di-rebase.
- Satu branch tidak menyentuh berkas branch lain yang sedang jalan. Nomor migrasi dipesan: `0039` untuk F1-28, `0040` untuk F1-11.
- Setiap PR: `go vet` dan `go test -p=1` untuk paket yang disentuh, build linux, windows dan wasm bergiliran (Celeron dan PC owner berbagi memori), dan satu baris di "Belum dirilis" `CHANGELOG.md`.
- Jangan deploy, menyentuh server pilot atau DB, mengirim WA atau email, atau menerbitkan rilis tanpa permintaan owner.

### Urutan pemangkasan bila meleset

Dipangkas berurutan, mulai dari yang paling atas:

1. **F1-23** panduan: cukup satu halaman pasang dan masuk.
2. **F2-02** nomor tugas: pindah ke 1.0.1 seluruhnya.
3. **F1-05** kontras: hanya lencana status di tema gelap.
4. **F1-28** alur Medhum: ACC bersama dan kabar dua sisi tetap; broadcast yang membuat konten pindah ke 1.0.1 (broadcast tetap mencocokkan seperti sekarang).
5. **F1-04** alur anggota: hanya RSVP dan "Coba lagi".

Yang **tidak** dipangkas: domain, DB stabil bersih, gladi pulih, alarm, pengaman login, "Tetap masuk", dan ACC bersama.
