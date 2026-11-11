# Catatan rilis Space

Versi mengikuti aturan sebelum rilis publik:
- **Patch** naik di setiap pembaruan.
- **Minor** naik untuk pembaruan besar.
- **1.0.0** untuk rilis publik.

## Belum dirilis · Ace island A (branch feat/ace-island-a)

- **Ace = Dynamic Island** (arah A "Pil Mata", canvas dde22eb04942): pil hitam dengan dua mata kapsul. Saat diam, mata di tengah, berkedip tiap 3–6 detik, bernapas halus, sesekali melirik, dan mengikuti kursor dalam jarak 600 DIP. Ada kabar: mata bergeser ke kiri menjadi ikon aktivitas.
- **Keadaan**: ping/notifikasi (360×56), timer rapat live (240×40, hijau; merah saat lewat waktu), media yang sedang diputar lewat SMTC Windows (300×40, gelombang biru), detail (hover 300 ms atau klik; 440×168), dan dua aktivitas sekaligus (gelembung 36 px di samping pil; klik untuk menukar).
- **Timer rapat**: timer catatan yang terbuka (dengan agenda), atau timer bersama rapat yang sudah dimuat. Jeda/Lanjutkan dari island untuk yang berhak.
- **Media**: judul, artis, aplikasi, progres; tombol sebelumnya, putar/jeda, berikutnya. Dibaca tiap 1,5 detik hanya selama island tampil. Butuh Windows 10 1809 atau lebih baru.
- **Hemat**: tanpa loop 60 Hz saat diam (12 fps untuk napas, 60 fps hanya saat bergerak). Kurangi gerak mematikan napas, lirikan, anggukan, dan gelombang. Island berhenti menggambar dan berhenti membaca media saat tersembunyi, saat layar penuh, atau saat layar terkunci.
- **Koordinasi**: pilihan "Gaya A/B/C" di Pengaturan tidak lagi memengaruhi island; pilnya selalu hitam. Pengaturan (feat/pengaturan-a) bebas menghapusnya. Branch ini tidak mengubah halaman Pengaturan dan jendela maskot bintang.

## 0.4.5 — 2026-11-11 · Navigasi cepat, template WA, island v2, perbaikan pilot 0.4.4 (rc)

- **Navigasi cepat (Ctrl K)**: mencari semua halaman, tab Space, aksi, catatan, tugas, dan setiap seksi atau field pengaturan, dengan jejak seperti "Gaya Dynamic Island · Pengaturan › Perangkat › Dynamic Island". Ctrl K membuka dan menutup palet. Enter menjalankan hasil hanya saat daftar hasil tampil. "Izin arsip" hanya muncul untuk pengelola arsip.
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
- **Rapat (desain A pilihan owner)**:
  - Daftar dikelompokkan Hari ini / Akan datang / Sudah lewat menurut zona rapat, di samping detail rapat terpilih. Rapat lewat yang belum dicatat kehadirannya ditandai.
  - Detail berisi status, Waktu/Tempat/Tautan/Pengingat, serta Mulai rapat (timer bersama), Buka notulen, Ubah, dan menu ⋯ (salin tautan, kirim ringkasan ke WhatsApp, batalkan dengan konfirmasi atau aktifkan kembali).
  - Sub-tab Ringkasan · Kehadiran · Notulen. Penyelenggara mencatat Hadir/Izin/Tidak hadir langsung di baris; Hadir terisi jam mulai dan bisa diubah bersama catatan.
  - Rapat tanpa jam selesai ditulis "Jam selesai belum diisi", bukan "— selesai".
  - Di HP: daftar, lalu halaman detail dengan tombol kembali dan bilah aksi bawah. Notifikasi rapat dan tautan dari catatan membuka tab Rapat.
- **RAM**: logo divisi disimpan seukuran tampilannya.
- **Pengaturan › Perangkat (canvas A)**:
  - Bagian di kolom kiri, masing-masing dengan statusnya (misalnya "Selalu tampil"), dan kotak "Cari di Perangkat ini".
  - Baris label dengan saklar yang langsung tersimpan, menggantikan tombol "…: aktif". Ada pratinjau kecil island.
  - Volume bel memakai slider dengan tombol Putar.
  - Di layar sempit dan HP, pencarian pindah ke atas dan kontrol lebar turun ke bawah labelnya. Browser memakai susunan yang sama.
- **Maskot bintang dihapus**: Ace sekarang hanya Dynamic Island.
  - Jendela Ace (mengambang/tepi layar), menu tray "Tampilkan/Sembunyikan Ace", nama maskot, dan impor PNG/GIF hilang.
  - Saat pertama dibuka, preferensi lama ditulis ulang tanpa posisi, nama, dan maskot, lalu gambar maskot impor dihapus. Pengaturan island dan bel tetap.
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
