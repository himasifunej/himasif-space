# Rencana implementasi Space per sesi

Tanggal: 27 Oktober 2026, WIB. Acuan: [PRD](PRD.md), [Register keputusan](decision-register.md), [Riset Android](research/android-options.md).

Dokumen ini menjabarkan urutan yang disepakati. **Sesi 1A merged melalui PR #1** (`627298f`), **backend 1B–1C merged melalui PR #2** (`d92a372`). Integrasi UI native dan inti tugas 2A ada pada branch `feat/native-ui`; lihat [handoff integrasi](handoff/native-integration-tasks.md). Pengguna menyetujui prototype UI 03.1 dan memulai implementasi native pada branch `feat/native-ui`; backend 1B–1C dapat dikerjakan paralel dalam worktree terpisah. Status tiap tahap berikut tetap bergantung pada verifikasi fungsinya, bukan sekadar tersedianya tampilan. Tidak ada tanggal rilis atau estimasi durasi yang disepakati. Satu tahap dapat memakai beberapa sesi; jangan menganggap seluruh aplikasi selesai dalam enam percakapan.

## Prinsip handoff setiap sesi

Sebelum mengubah kode, baca PRD, koreksi keputusan, keadaan proyek dan instruksi lokal yang berlaku. Bawa hanya scope sesi dan dependensinya. Hasil sesi wajib mencatat:

- Apa yang dibuat dan kebutuhan PRD yang dipenuhi.
- Keputusan teknis baru beserta alasan, tanpa mengganti keputusan produk diam-diam.
- Cara menjalankan/mencoba hasil dan verifikasi yang benar-benar dijalankan.
- Batas yang belum selesai, migrasi/perubahan data, dan dependensi sesi berikutnya.
- Skenario penerimaan relevan yang sudah lulus atau belum bisa diuji.

Jangan menyalin secret akun/bot ke dokumen handoff. Jangan mengubah Reo, sheet produksi, DNS atau deployment live sebagai efek samping membuat prototype lokal.

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

Status: irisan UI awal pada branch `feat/native-ui`, terpisah dari backend 1B–1C. [Handoff UI](handoff/native-ui.md) menjelaskan kemampuan dan batasnya; [prompt sesi backend](handoff/backend-1b-1c.md) menetapkan kepemilikan berkas.

- Shell native, sidebar tetap/collapse, header/search shortcut, dropdown notifikasi/profil, logout, status koneksi.
- Halaman divisi/Medhum, beranda, board, agenda, notes/template dan pengaturan WhatsApp mengikuti prototype yang disetujui.
- Tema dark/light/system, aksen preset/kustom dan density disimpan per perangkat.
- Komponen board snapshot dengan drag/drop, reorder dan panel detail kanan diuji tersendiri; belum disambungkan ke layanan tugas.
- Login, akun, periode dan direktori divisi memakai API 1A. Direktori divisi bukan bukti membership. Data tugas/notes/bot dan wewenang Space tetap menunggu backend; tampilan tidak mengesahkan akses atau mengklaim penyimpanan/koneksi berhasil.

### Sesi 1B: akun, undangan dan periode

Status: backend selesai di main (PR #2); UI undangan, pemulihan, perubahan kredensial dan keanggotaan terhubung di branch UI. Form administrasi periode/kadiv/checklist/izin arsip lengkap masih perlu disambungkan.

- Akun baru/lama, verifikasi email, aktivasi undangan, pencabutan dan pemulihan akun.
- Satu akun admin organisasi, terpisah dari akun pribadi; pengelolaan periode/kadiv.
- Keanggotaan/role per periode, akses pengurus lama dan izin arsip pengurus baru.
- Checklist serah terima dan penutupan periode yang tidak menghapus data.

Penerimaan: AC-01, AC-02, AC-10, AC-11; batas email/provider/umur undangan ditetapkan eksplisit.

### Sesi 1C: Space dan role

Status: backend selesai di main (PR #2); UI daftar Space berizin, anggota, kelompok, role kustom dan 45 terhubung di branch UI.

- Space divisi, Medhum, pengelompokan Tekno/Desain/Video dan role kustom.
- Anggota melihat ruang divisi; pemisahan kerja internal/bersama.
- Pendamping, satu role 45 aktif, larangan role/akses bagi 45 termasuk lewat API.
- Web awal untuk login dan navigasi ruang.

Penerimaan: AC-03, AC-06–08; belum boleh ada privilege yang hanya dibatasi lewat tombol UI.

## Tahap 2 — Pekerjaan, dokumentasi dan rapat

### Sesi 2A: tugas dan jobdesk

Status: inti tugas/jobdesk tersimpan, kanban/reorder, delegasi, bantuan, bukti/tinjauan, komentar, riwayat dan pembaruan in-app dibuat di branch UI. **Reminder pribadi/toggle share belum selesai**, sehingga seluruh Sesi 2A belum dinyatakan selesai.

- Tugas dengan hasil, PJ utama, deadline, pelaksana tambahan, progres dan komentar.
- Jobdesk pribadi/divisi/ruang bersama dari satu objek tugas.
- Pengajuan bantuan dan tinjauan; riwayat perubahan PJ/deadline.
- Reminder pribadi dan toggle share individual, bukan pewarisan berdasarkan jabatan.
- Status penyerahan/pencocokan terpisah, tanpa workflow revisi desain wajib.

Penerimaan: AC-03–05, AC-09; server menjadi sumber hak akses di semua tampilan.

### Sesi 2B: notes dan template

- Rich notes, template rapat/jadwal/catatan bebas, penyimpanan template lain.
- Tabel jadwal, lampiran, komentar dan satu editor aktif.
- Jadikan teks/baris sebagai tugas atau reminder dengan pemeriksaan manusia.
- Simpan scope dan hubungan ke sumber, tanpa membagikan seluruh note privat otomatis.

Penerimaan: AC-33–34; dasar versi/draf disiapkan untuk offline.

### Sesi 2C: rapat dan penjadwal pengingat

- Jadwal, peserta, lokasi/link, note, kehadiran/waktu datang oleh penyelenggara.
- H-2/H-1/H-0 06.00 WIB untuk tugas; rapat H-1/1 jam/15 menit; overdue harian.
- Ping default/kustom, umum/tugas/rapat, pilihan kanal, sudah lihat dan snooze.
- Kebijakan slot terlewat, deduplikasi dan perubahan/cancel; notifikasi in-app/web dasar.

Penerimaan: AC-25–29, AC-35. Kanal desktop/WA bisa memakai adapter uji sampai tahap terkait; tidak mengklaim pesan nyata sudah terkirim.

## Tahap 3 — Desktop dan offline

### Sesi 3A: MyGo, tray dan notifikasi Windows

- MyGo memakai UI native Go yang sama pada desktop dan WASM sesuai ADR 0001; lifecycle tutup-ke-tray dan keluar eksplisit.
- Opsi mulai saat login, toast Windows modern, klik membuka tugas/rapat.
- Bel bawaan, suara/volume pengguna, impor audio dan satu bel per kejadian.
- Verifikasi pada Windows nyata; host Linux saja tidak cukup membuktikan toast berfungsi.

Penerimaan: AC-39–40. Periksa kemampuan versi MyGo saat implementasi; kebutuhan toast tidak diganti menjadi balloon hanya karena API bawaan lebih mudah.

### Sesi 3B: Ace dan personalisasi

- Aset Ace default dan identitas logo tetap melalui proses desain.
- Mengambang, tepi/hover seperti screenshot, disembunyikan, pemulihan dari tray.
- Dialog reminder/ping serta aksi buka/sudah lihat/snooze; posisi multimonitor.
- Pilih/impor maskot sederhana; tidak membangun editor animasi penuh.
- Ukur idle/animasi dan pastikan mode tersembunyi hemat kerja render.

Penerimaan: AC-41–42; notifikasi tetap hidup saat Ace disembunyikan.

### Sesi 3C: cache, antrean dan konflik offline

- Cache tugas/jadwal/notes, draf lokal dan antrean perubahan.
- Indikator offline/lokal/belum cloud/menyinkronkan/tersinkron.
- Penggabungan bidang berbeda dan resolusi konflik bidang sama dengan kedua versi terjaga.
- Periksa ulang hak saat sync; operasi delegasi/role/bot tetap online.
- Isolasi cache per akun, pencabutan akses dan penanganan lampiran pending.

Penerimaan: AC-36–38, termasuk restart perangkat sebelum sinkronisasi dan izin yang berubah.

## Tahap 4 — Medhum dan bot

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
