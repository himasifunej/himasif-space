# Catatan rilis Space

Versi mengikuti aturan sebelum rilis publik:
- **Patch** naik di setiap pembaruan.
- **Minor** naik untuk pembaruan besar.
- **1.0.0** untuk rilis publik.

## Belum dirilis · 0.4.6: tautan kartu di WhatsApp, buka di aplikasi, bar unduh di web (feat/046-wa-links-web-banner)

- **Tautan kartu di WhatsApp**: pesan bot tentang tugas atau rapat (tugas baru, hasil diserahkan, perubahan PJ/tenggat, tugas Medhum, rapat baru/berubah/batal, pengingat terjadwal, ping tentang tugas/rapat, dan balasan `!cek`) kini diakhiri "Buka di Space: <SPACE_PUBLIC_URL>/t/<id tugas>" atau "/r/<id rapat>".
  - Tautannya tanpa token login, karena pesan diteruskan di grup.
  - Server meneruskan `/t/…` dan `/r/…` ke halaman kartu di web (`#/tugas/<id>`, `#/rapat/<id>`). Yang belum masuk login dulu, lalu kembali ke kartunya.
  - Yang tidak berhak melihat mendapat "Kamu tidak punya akses ke tugas ini".
- **Dari web ke aplikasi**: aplikasi Windows menangani `space://tugas/<id>` dan `space://rapat/<id>`, membuka kartunya dan memfokuskan jendelanya.
  - Skema `space` didaftarkan oleh installer (urlSchemes di `app/mygo.json`; wrapper NSIS menolak template tanpa skema itu) dan oleh aplikasi setiap kali mulai, jadi instalasi yang diperbarui lewat updater juga mendapatkannya.
  - Halaman kartu di web punya "Buka di aplikasi Space". Setelah sekali berhasil, browser itu mengingatnya (localStorage) dan berikutnya mencoba sendiri; bila tidak ada jawaban dalam 2,5 detik, tetap di web. HP tidak pernah mencoba.
- **Bar unduh di web** (hanya browser Windows desktop): "Pakai Space untuk Windows — notifikasi, Ace Island, lebih cepat" dengan Unduh (`/unduh/windows`, installer stabil terbaru dari feed pembaruan yang ditandatangani, atau rc bila belum ada yang stabil), Nanti (14 hari), dan Jangan ingatkan lagi (tanda akun `webAppBannerOff`, berlaku di semua browser). Bar hilang setelah aplikasi pernah terbuka dari web.
- **Tanpa migrasi**: `related_kind`/`related_id` di `delivery_events` sudah ada dan kini diisi saat event dibuat. Tanda bar unduh memakai kunci baru di `user_onboarding.marks` (JSON).
## Belum dirilis · Pembaruan rc dan gerak Ace (fix/updater-ace-motion)

- **Pembaruan**:
  - Build rc (versi berisi `-rc`) yang kanalnya belum pernah dipilih memakai kanal Uji coba (rc). Pilihan pengguna tetap menang.
  - Feed yang menjawab 404 berarti belum ada yang terbit (`update.ErrNoFeed`), bukan gagal. Di Stabil statusnya "Belum ada versi stabil baru"; klien rc tetap membaca `rc.json`.
  - Status tidak pernah "terbaru" bila pemeriksaan terakhir gagal ("pemeriksaan terakhir gagal, HH.MM"). Sebelumnya rc2 di Stabil menampilkan "terbaru, diperiksa 18.50" bersama galat `stable.json: HTTP 404`.
- **Gerak Ace**:
  - Memilih Kiri/Tengah/Kanan (dialog "Yang baru" + Ace dan Pengaturan) menggeser Ace dengan pegas ±400 ms dan sedikit melewati tujuan, tidak lagi melompat.
  - Saat bergeser atau ditarik, Ace bergoyang seperti jeli: memanjang searah gerak sesuai kecepatannya (paling banyak 16%, luasnya tetap), mata sedikit tertinggal, lalu memantul kecil saat berhenti.
  - Kurangi gerak: tanpa goyangan, geser singkat 160 ms.
  - Jendela island tetap berukuran sama; hanya bentuk dan region-nya yang mengikuti goyangan.
  - Setelah berhenti, frame kembali ke irama diam. Mode `--island-spike=ace-slide`/`ace-slide-still` mengukurnya.

## Belum dirilis · Onboarding dipandu Ace (feat/onboarding-ace-a)

- **Onboarding dipandu Ace** (canvas db456de51efb, opsi A): kartu langkah menggantikan dialog "Undangan diterima". Langkahnya Profil, Kenalan dengan Ace (posisi dan gaya), Kapan Ace muncul (selalu atau saat ada kabar, jam tenang, kabar percobaan Buka/Tunda/Tutup), Supaya kabar sampai (izin notifikasi, WhatsApp opsional, Mulai bersama Windows, pembaruan otomatis), lalu Selesai dengan tur opsional. Setiap langkah bisa dilewati dan punya bawaan; "Lewati semua" memakai semuanya.
  - Ace bicara dari island yang asli di setiap langkah, dengan ekspresi mata sendiri (senang, melihat ke kartu, mata lebar, berkedip). Bila island mati, kalimatnya tampil di kartu.
  - Kadiv (yang boleh mengundang) mendapat langkah tambahan: tempel NIM, undangan dikirim ke email kampus.
  - Di web tidak ada langkah Ace dan Windows.
  - Foto profil ditunda.
- **Pengguna lama**: sekali per perangkat, satu dialog berisi "Yang baru" versi itu dan kartu "Ace sekarang bisa diatur" (posisi, gaya, jam tenang). Dialog "Yang baru" biasa tidak ikut muncul. Akun baru tidak melihatnya, karena onboarding sudah mencakupnya.
- **Jam tenang** (baru, Pengaturan › Perangkat ini): 21–05, 22–06 (bawaan) atau 23–07. Selama jam tenang, notifikasi perangkat ditahan tanpa bel, lalu datang sebagai satu kabar setelahnya.
- **Pembaruan otomatis** (baru, bawaan menyala): versi baru diunduh di latar dan dipasang saat jendela Space tertutup ke tray; Space terbuka lagi di tray. Mati: versi baru hanya diumumkan seperti sebelumnya. Pilihan kanal Stabil/Uji coba hanya tampil di build rc atau bila kanal rc sudah dipilih.

## 0.4.5-rc4 — 2026-11-11 · Onboarding dipandu Ace (pre-release)

- **Onboarding dipandu Ace (opsi A)**:
  - Lima kartu langkah menggantikan dialog "Undangan diterima": Profil, Kenalan dengan Ace (posisi dan gaya), Kapan Ace muncul (selalu atau saat ada kabar, jam tenang, kabar percobaan), Supaya kabar sampai (izin notifikasi, WhatsApp opsional, Mulai bersama Windows, pembaruan otomatis), lalu Selesai dengan tur opsional.
  - Setiap langkah bisa dilewati, dan "Lewati semua" memakai pengaturan bawaan.
  - Ace bicara dari island yang asli, dengan ekspresi mata per langkah.
  - Di web tidak ada langkah Ace dan Windows.
- **Pengguna lama** melihat sekali per perangkat dialog "Yang baru" ditambah "Ace sekarang bisa diatur".
- **Jam tenang** (21–05, 22–06, atau 23–07) menahan notifikasi perangkat, tanpa bel, lalu mengirimnya sebagai satu kabar.
- **Pembaruan otomatis** (bawaan menyala) memasang pembaruan saat jendela tertutup ke tray. Kanal rc hanya tampil di build rc.
- Hanya klien yang berubah; server tetap rc3.

## 0.4.5-rc3 — 2026-11-11 · Masukan uji rc2, Kirim ping A + C, riwayat perubahan (pre-release)

- **Kirim ping (A + C)**:
  - Dialog satu layar, tanpa tab: cari nama (Enter mencentang), grup cepat (Semua, PJ tugas aktif, Belum baca ping terakhir), sakelar "Juga ke WhatsApp" yang diingat, dan tombol "Kirim ke N orang". Setelah terkirim, dialog berganti ringkasan; bila gagal, pilihan tetap tersimpan.
  - "Ingatkan <PJ>" di detail dan menu kartu tugas, serta "Ingatkan peserta" di detail rapat, sekali klik. Tombolnya menjadi "Diingatkan HH.MM" selama satu menit.
- **Ace island**:
  - Jendela island kini berukuran tetap. Bentuknya bergerak di dalam jendela, dan region Windows mengikuti bentuk itu. Sebelumnya jendela diubah ukuran dan dipusatkan ulang di setiap frame pegas, sehingga kadang tampil frame lama yang diregangkan dan kotak yang belum tergambar.
  - Blur konten dihapus, dan tinggi mata berubah halus saat kursor datang.
  - Island bisa ditarik di sepanjang tepi atas, dari pojok kiri sampai kanan. Posisinya disimpan (`islandAnchor`), dan Pengaturan › Perangkat punya tombol "Ke tengah".
  - Timer di island: "Buka rapat" membuka rapatnya di tab Rapat, atau catatannya untuk timer catatan.
- **Gaya island**: Ikut tema (bawaan, juga untuk pengguna lama), Hitam, Kaca (tembus pandang tanpa blur Windows, karena blur Windows mewarnai seluruh persegi jendela), dan Custom (warna dan transparansi, kontras teks otomatis). CPU diam per gaya ±0,1–0,3% satu core.
- **Antrean kabar**: Tutup, Buka dan Tunda hanya berlaku untuk kabar yang tampil, lalu kabar berikutnya muncul dengan detail tetap terbuka. Ada "Tutup semua". Kabar sejenis yang datang dalam 10 detik digabung ("8 rapat baru di Medhum"). Kabar rapat berlabel "Space · Rapat", dengan tanggal "5 Des 2026 · 08.00 WIB".
- **Rapat**: lebar daftar bisa ditarik dari 72 sampai 640 px. Di bawah 180 px, daftar menampilkan tanggal saja. Lebarnya disimpan di perangkat.
- **Tinjauan hasil**:
  - "Bukti sesuai" cukup sekali klik. Alasan hanya diminta untuk "Bukti tidak sesuai".
  - Kadiv dan admin boleh menyetujui hasil yang mereka serahkan sendiri. Persetujuan itu tercatat sebagai `task.evidence.self_approve` dengan alasan "Disetujui sendiri", di riwayat tugas dan audit. Peninjau lain melihat "Menunggu ditinjau oleh …", bukan tombol mati.
- **Serahkan hasil**:
  - Tautan divalidasi `https://` saat diketik.
  - Bila gagal, muncul toast merah dan error di dalam form.
  - Pilihan Tautan | Unggah berkas, boleh salah satu atau keduanya. Berkas bukti milik penyerahannya dan tampil di kartu tinjauan.
- **Migrasi 0033** (aditif): `task_attachments.evidence_id`, dan pemeriksaan "tautan atau catatan" di `task_evidence` dilepas karena server yang kini memeriksanya. Server 0.4.5-rc2 yang live tetap jalan dengan skema ini.
- **Riwayat perubahan**:
  - Catatan setiap versi ada di `internal/appui/changelog.json`. Isinya muncul di dialog "Yang baru di 0.4.x" (sekali setelah pembaruan, tidak pada instalasi baru) dan di Pengaturan › Tentang.
  - Catatan yang sama ditandatangani ke field `notes` manifest pembaruan (`publish_update.py`), tanpa field baru, supaya klien lama tetap bisa membaca feed.

## 0.4.5-rc2 — 2026-11-11 · Ace island A, Pengaturan A, Rapat A, pembaruan otomatis (pre-release)

- **Ace island A**:
  - **Ace = Dynamic Island** (arah A "Pil Mata", canvas dde22eb04942): pil hitam dengan dua mata kapsul. Saat diam, mata di tengah, berkedip tiap 3–6 detik, bernapas halus, sesekali melirik, dan mengikuti kursor dalam jarak 600 DIP. Ada kabar: mata bergeser ke kiri menjadi ikon aktivitas.
  - **Keadaan**: ping/notifikasi (360×56), timer rapat live (240×40, hijau; merah saat lewat waktu), media yang sedang diputar lewat SMTC Windows (300×40, gelombang biru), detail (hover 300 ms atau klik; 440×168), dan dua aktivitas sekaligus (gelembung 36 px di samping pil; klik untuk menukar).
  - **Timer rapat**: timer catatan yang terbuka (dengan agenda), atau timer bersama rapat yang sudah dimuat. Jeda/Lanjutkan dari island untuk yang berhak.
  - **Media**: judul, artis, aplikasi, progres; tombol sebelumnya, putar/jeda, berikutnya. Dibaca tiap 1,5 detik hanya selama island tampil. Butuh Windows 10 1809 atau lebih baru.
  - **Hemat**: tanpa loop 60 Hz saat diam. Napas 12 fps hanya selama 12 detik setelah ada interaksi (kursor dekat, hover, klik, kabar, aktivitas), lalu berhenti di ujung napas; kedip dan lirikan tetap. CPU idle di Linux turun dari ±1,87% menjadi ±0,84% satu core. Kurangi gerak mematikan napas, lirikan, anggukan, dan gelombang. Island berhenti menggambar dan berhenti membaca media saat tersembunyi, saat layar penuh, atau saat layar terkunci.
- **Rapat (desain A pilihan owner)**:
  - Daftar dikelompokkan Hari ini / Akan datang / Sudah lewat menurut zona rapat, di samping detail rapat terpilih. Rapat lewat yang belum dicatat kehadirannya ditandai.
  - Detail berisi status, Waktu/Tempat/Tautan/Pengingat, serta Mulai rapat (timer bersama), Buka notulen, Ubah, dan menu ⋯ (salin tautan, kirim ringkasan ke WhatsApp, batalkan dengan konfirmasi atau aktifkan kembali).
  - Sub-tab Ringkasan · Kehadiran · Notulen. Penyelenggara mencatat Hadir/Izin/Tidak hadir langsung di baris; Hadir terisi jam mulai dan bisa diubah bersama catatan.
  - Rapat tanpa jam selesai ditulis "Jam selesai belum diisi", bukan "— selesai".
  - Di HP: daftar, lalu halaman detail dengan tombol kembali dan bilah aksi bawah. Notifikasi rapat dan tautan dari catatan membuka tab Rapat.
- **Pengaturan › Perangkat (canvas A)**:
  - Bagian di kolom kiri, masing-masing dengan statusnya (misalnya "Selalu tampil"), dan kotak "Cari di Perangkat ini".
  - Baris label dengan saklar yang langsung tersimpan, menggantikan tombol "…: aktif". Ada pratinjau kecil island.
  - Volume bel memakai slider dengan tombol Putar.
  - Di layar sempit dan HP, pencarian pindah ke atas dan kontrol lebar turun ke bawah labelnya. Browser memakai susunan yang sama.
- **Maskot bintang dihapus**: Ace sekarang hanya Dynamic Island.
  - Jendela Ace (mengambang/tepi layar), menu tray "Tampilkan/Sembunyikan Ace", nama maskot, dan impor PNG/GIF hilang.
  - Saat pertama dibuka, preferensi lama ditulis ulang tanpa posisi, nama, dan maskot, lalu gambar maskot impor dihapus. Pengaturan island dan bel tetap.
  - Pilihan Gaya A/B/C dihapus: island A selalu pil hitam bermata, dan pratinjau mengikutinya.
- **Pembaruan otomatis desktop**:
  - Space desktop memeriksa versi baru saat dibuka dan setiap 6 jam tanpa data akun, lalu mengumumkan "Space X tersedia" di Ace Island atau toast. Tombolnya: Perbarui sekarang, Lihat perubahan, dan Nanti.
  - Installer diunduh di latar dengan progres dan diperiksa SHA-256-nya. Pemasangan berjalan diam-diam (NSIS /S), lalu Space terbuka lagi. Ada kanal Stabil (bawaan) dan Uji coba (rc).
  - Manifest di GitHub Releases (`update-feed`) ditandatangani ed25519. Aplikasi menolak manifest tanpa tanda tangan yang sah, manifest dari kanal lain, dan downgrade. Langkah rilis ada di `docs/operations/auto-update.md`, dengan `scripts/release/publish_update.py` dan `tools/updatesign`.
  - Server dan web tidak berubah.
  - Bagian **Pembaruan aplikasi** ada di Pengaturan › Perangkat ini (kanal Stabil/Uji coba) dan di Status layanan. Versi ini terbit di kanal **rc**.
- **Perbaikan tinjauan rc2**:
  - "Buat notulensi di Catatan" dari tab Notulen menautkan catatan ke rapat yang sedang dibuka, bukan ke rapat berikutnya.
  - Membatalkan atau mengaktifkan kembali rapat tidak lagi gagal karena ada peserta yang sudah keluar dari Space.
  - Dialog batal tetap pada rapatnya meski daftar berubah.
  - Rapat yang melewati tengah malam tetap di Hari ini sampai selesai.
  - Kontrol kehadiran hanya muncul bila server mengizinkan (`canManage`).
  - Jam datang bawaan memakai jam server.
  - Pemberitahuan pembaruan tidak lagi dikirim dua kali ke island, punya toast cadangan, dan tombolnya muat di island.
- **Server**: tidak ada migrasi baru sejak 0.4.3. Satu-satunya perubahan server adalah isian balasan perintah khusus WA. Rinciannya ada di laporan rilis rc2.
- **Known issues**:
  - Aplikasi layar penuh di monitor kedua juga menyembunyikan pil island.
  - Volume bel yang digeser lalu langsung ditinggalkan (<0,5 detik) bisa belum tersimpan.

## 0.4.5-rc — 2026-11-11 · Navigasi cepat, template WA, island v2, perbaikan pilot 0.4.4 (pre-release)

- **Navigasi cepat (Ctrl K)**: mencari semua halaman, tab Space, aksi, catatan, tugas, dan setiap seksi atau field pengaturan, dengan jejak seperti "Selalu tampil · Pengaturan › Perangkat › Dynamic Island". Ctrl K membuka dan menutup palet. Enter menjalankan hasil hanya saat daftar hasil tampil. "Izin arsip" hanya muncul untuk pengelola arsip.
- **Template dan perintah WhatsApp**:
  - Chip "Sisipkan:" berlabel, misalnya [Isi pesan] dan [Nama pengirim], tidak lagi memakai { }.
  - Pratinjau muncul sendiri dan ditutup manual.
  - Template lama berisi `{body}` dan lainnya dimuat sebagai kata, lalu disimpan kembali tanpa berubah.
  - Ada tombol "Buat perintah baru".
- **Perintah khusus WA (server)**: balasan dapat memuat nama pengirim, tanggal, jam, nama grup, nama bot, dan teks setelah perintah. Balasan tanpa `{` dikirim persis seperti dulu. Hasil isian dipotong pada 4096 karakter. Tidak ada migrasi.
- **Dynamic Island v2**:
  - Sudut bening, tanpa bayangan ungu.
  - Muncul kembali setelah layar penuh atau screenshot.
  - Pilihan **Selalu tampil** aktif bawaan, termasuk untuk pengguna yang memperbarui. Ace beristirahat sebagai pil di atas layar, lalu tampil lagi setelah login.
  - Layar penuh diperiksa setiap 0,5 detik.
- **RAM**: logo divisi disimpan seukuran tampilannya.
- **Perbaikan pilot 0.4.4**:
  - Baris tab tanpa scrollbar yang selalu tampil.
  - Tur meminta pengguna menekan tab, lalu menyorot halamannya.
  - Aksi baris Anggota muat di kolomnya, dan "Ubah role & akses" pindah ke menu ⋯.
  - Role kustom punya dialog sendiri, dengan contoh dan izin berikon.
  - Popover PJ dan tenggat di catatan tidak lagi hilang saat dituju.
- **Server**:
  - Klien 0.4.5 tetap jalan dengan server 0.4.3. Bedanya, balasan perintah khusus yang memakai [Nama pengirim], [Tanggal], dan seterusnya dikirim apa adanya, misalnya `{sender}`, sampai server 0.4.5 dipasang.
  - Deploy server bersama atau sebelum klien.
- **Known issues**:
  - Island: aplikasi layar penuh di monitor kedua juga menyembunyikan pil di monitor utama.
  - Pratinjau perintah khusus memakai contoh nilai dan tidak menyertakan pembungkus balasan bot.

## 0.4.4 — 2026-11-10 · Dynamic Island desktop

- Jendela Dynamic Island native Windows: banner dan detail, aksi Buka/Tunda 15 menit, antrean prioritas, dan penggantian berdasarkan tag.
- Pengaturan › Perangkat: aktif/mati, gaya A (Hitam), B (Aksen Space), C (Kaca; bawaan), penggantian toast Windows, dan pengurangan gerak.
- Toast Windows tetap dipakai saat island mati, antrean penuh, fullscreen, presentasi, Focus Assist, atau presenter tidak tersedia. Pilihan untuk menampilkan keduanya juga tersedia.
- Spring mass 1 / stiffness 170 / damping 19, crossfade konten, WorkArea atas tengah, tanpa mengambil fokus, dan sudut transparan yang meneruskan klik.
- Desktop saja; tidak mengubah server atau basis data. Belum dirilis/deploy. Hasil spike dan daftar uji owner: `docs/handoff/dynamic-island-044.md`.

## 0.4.3 — 2026-11-10 · Timer rapat bersama, ringkasan ke WhatsApp, panel Anggota

- **Timer rapat bersama (T2)**:
  - Timer catatan rapat kini disimpan di server pada rapat, jadi notulis di laptop dan kadiv di HP melihat timer yang sama.
  - Yang bisa mengatur (mulai, jeda, lanjut, agenda berikutnya, +5 mnt, hentikan): pengelola rapat dan penyunting catatan rapat itu. Yang lain hanya melihat, dengan keterangan "Diatur oleh …".
  - Bila dua orang menekan bersamaan, yang kedua mendapat pesan "Timer baru saja diubah oleh …" dan tampilannya dimuat ulang.
  - Catatan tanpa rapat tetap memakai timer di perangkat itu.
  - Migrasi 0032 hanya menambah kolom.
- **Kirim ringkasan rapat ke WhatsApp (C7)**:
  - Tab Rapat di catatan punya tombol "Kirim ringkasan ke WhatsApp", dengan pratinjau dan konfirmasi.
  - Isinya keputusan, action item (PJ + tenggat) dan tautan catatan. Tautan publik dipakai bila catatan dibagikan publik dari perangkat yang sama.
  - Pesan dikirim lewat bot divisi ke grup divisi.
- **Anggota (A4)**:
  - Setiap baris anggota punya menu ⋯ berisi Lihat tugas dan Keluarkan….
  - Di layar lebar, "Undangan tertunda" (kirim ulang, cabut, status bounce) tampil berdampingan dengan "Kewenangan role" yang ringkas.
- **Status layanan**: "Revisi sumber server" dan "Hash snapshot server" terisi, tidak lagi "unknown".

## 0.4.2 — 2026-11-10 · Catatan: tautan publik, tag, template

- **Bagikan catatan (C6)**: dua pilihan, **Hanya divisi ini** (bawaan) dan **Publik lewat tautan**.
  - Tautan publik bisa dibuka tanpa login dan hanya-baca.
  - Tokennya acak 32 byte, dan server hanya menyimpan hash-nya.
  - Tautan bisa dimatikan atau dibuat ulang; tautan lama langsung 404.
  - Halamannya tidak diindeks, tidak di-cache, dan tidak mengirim referer.
  - Hanya kadiv divisi atau penulis catatan yang boleh menyalakannya.
  - Isinya disaring di server: mention tampil sebagai nama saja; tidak ada nomor WA, NIM, email, ID, komentar, lampiran, atau tautan ke Space.
  - Autosave langsung terlihat oleh pembaca publik.
  - Menutup periode ikut menutup tautannya.
  - Setiap aksi tercatat di audit.
- **Tag catatan** di panel Rapat. Semua tag ditulis huruf kecil, paling banyak 10.
- **Template (C5)**: Notulen rapat divisi, Evaluasi, Rapat singkat, Kosong, dan Tabel Jadwal.
  - Notulen berisi Agenda (setiap butir 10 mnt untuk timer), Pembahasan, Keputusan, dan Action item.
  - Notulen bisa diisi otomatis dari rapat terjadwal berikutnya: judul, waktu dan tempat, agenda dari deskripsi rapat, dan rapat ikut tertaut ke catatan.
- **Migrasi 0031** (aditif): kolom `notes.tags`, tabel `note_public_links`, dan jenis template `evaluation`/`brief`.

## 0.4.1 — 2026-11-10 · Izin notifikasi dan tur yang pas per role

- **Langkah baru "Izinkan notifikasi"** di checklist "Langkah pertama". Langkah ini mengikuti izin di perangkat yang sedang dipakai:
  - Di web, mengikuti izin browser.
  - Di aplikasi desktop, mengikuti setelan notifikasi Windows.
  - Langkah tercentang sendiri bila notifikasi sudah diizinkan.
  - Bila diblokir, langkah ini menampilkan cara mengizinkannya lagi.
- **Notifikasi uji langsung** setiap kali izin baru diberikan: "Notifikasi aktif ✓".
- **Pesan uji di WhatsApp** setelah nomor pertama kali tertaut: "Tersambung ✓". Pesan ini dikirim sekali per akun, lewat chat pribadi.
- **Lapisan notifikasi bersama** (`appui.Notifier`): popup browser di web dan notifikasi Windows di desktop. "Dynamic island" untuk Space desktop bisa ditambahkan nanti sebagai presenter tambahan (`MultiNotifier`).
- **Perbaikan tur**: tur hanya menyorot tab yang terlihat oleh akun itu.
  - Anggota tidak punya tab Ping, jadi langkah WhatsApp menyorot tab Bot WhatsApp.
  - Langkah yang tabnya disembunyikan dilewati.
  - Keterangan "tidak tampil di layarmu" sudah dihapus.


Rangkuman 0.3.1 sampai 0.4.0.

### Onboarding dan Panduan (0.4.0)
- **Sambutan setelah bergabung lewat undangan.** Setelah itu muncul checklist **Langkah pertama** di Beranda dengan empat langkah: Tautkan WhatsApp, Lengkapi profil, Lihat tugasmu, Kenali tab. Setiap langkah tercentang sendiri dan checklist bisa disembunyikan. Progres disimpan per akun di server.
- **Tur sorotan 5 langkah sesuai role** (Anggota, 45, Kadiv). Tur bisa dilewati, tampil sekali, dan bisa diulang dari menu profil.
- **Halaman Panduan dan tombol ?.** Tombol ? membuka artikel untuk halaman yang sedang dibuka di panel samping.
  - Ada 12 artikel bahasa sederhana: board, catatan rapat, ping/bot WA, anggota dan role, 45, serah terima, pergantian periode.
  - Isinya bisa diganti di server tanpa rilis lewat `SPACE_GUIDES_DIR`.
- **Empty state yang membimbing** di board, catatan, dan anggota.
- **Kartu serah terima untuk kadiv baru.**
- **Dialog Profil** untuk mengubah nama tampilan.
- **Gerak halus 150–300 ms** di tur, checklist, panel, dan tombol. Pengaturan kurangi gerak dihormati.
- **Migrasi 0030** (`user_onboarding`). Akun yang sudah ada tidak mendapat onboarding, tetapi tetap bisa membuka Panduan dan mengulang tur.

### Akun, undangan, dan WhatsApp (0.3.7–0.3.9)
- **Undangan lewat NIM**: undangan dikirim ke `NIM@mail.unej.ac.id` lewat Amazon SES.
- **Halaman undangan yang tidak berlaku** sekarang menjelaskan sebabnya.
- **Tautkan WhatsApp** dari Space:
  - Kode dikirim ke bot divisi lewat tautan wa.me.
  - Ada status tertaut.
  - Nomor yang terikat ke akun lain bisa dipindah dengan **PINDAH**.
  - Balasan bot lebih bersih.

### Catatan rapat (0.3.2–0.3.7)
- **C1–C3**: blok terstruktur, autosave sebagai pengganti edit live, kunci editor yang bisa kedaluwarsa, dan interaksi blok.
- **Gaya C**: panel kanan bertab, bilah tipis, dan menu / ringkas.
- **G3**: mode Edit | Baca, serta urungkan/ulangi untuk seluruh dokumen.
- **Timer rapat lokal (T1)**: durasi agenda, pil yang menempel, jeda, +5 mnt, dan agenda berikutnya.
- **Mention (C4)**: `@` untuk anggota dan tugas.
- **Action item (C4)**: PJ, tenggat, dan **Jadikan tugas**, yang mengisi `taskId` dan menampilkan chip #T.

### Anggota (0.3.2–0.3.5)
- **Halaman Anggota baru (A1)**: tabel dengan role, beban kerja, dan status WhatsApp.
- **Drawer Ubah akses (A2)**.
- **Mengeluarkan anggota dengan aman (A3)**.

### Server dan operasi
- **Pilot pindah dari aws-vps ke tc-vps** (2026-11-09).
- **Build dilakukan di PC.** Deploy memakai `deploy.sh` dengan urutan `--dry-run`, `--stage`, lalu `--go`.
- **`make server` mencap versi server (0.3.1).**
