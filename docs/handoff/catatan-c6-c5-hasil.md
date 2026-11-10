# Catatan C6 (privasi dan tautan publik, tag) dan C5 (template), 0.4.2

Branch `feat/catatan-c6-c5` dari `origin/main` 28772bb (0.4.1). Rencana yang diikuti: `anggota-catatan-rapat-rencana.md` §5 (privasi) dan §9 (C5, C6).

## C6: Bagikan
- **Pilihan akses**:
  - **Hanya divisi ini** adalah bawaan, sama dengan perilaku sebelumnya.
  - **Publik lewat tautan** dipilih lewat tombol **Bagikan**, atau chip **Publik** di bilah catatan.
- **Siapa yang boleh menyalakan, mencabut, atau membuat ulang**: kadiv divisi pemilik, atau penulis catatan yang masih punya `notes.manage`, dan hanya selama periode masih terbuka.
  - 45 dan anggota lain melihat statusnya tetapi tidak bisa mengubahnya.
  - Server mengirim izin ini sebagai `Note.Permissions.Share`.
- **Token**:
  - 32 byte acak dari `crypto/rand`, ditulis base64url (43 karakter), dengan URL `PUBLIC_URL/n/<token>`.
  - DB hanya menyimpan SHA-256 dari token itu.
  - URL dikirim **sekali**, saat tautan dibuat. Untuk menyalinnya lagi, pengguna membuat tautan baru, dan tautan lama langsung tidak berlaku.
- **Halaman publik** (`GET /n/{token}`):
  - HTML dirender di server, tanpa aplikasi dan tanpa login.
  - Header: `X-Robots-Tag: noindex, nofollow, noarchive`, `<meta name="robots" content="noindex">`, `Referrer-Policy: no-referrer`, `Cache-Control: private, no-store`, dan CSP ketat.
  - Halaman memeriksa `/api/public/notes/{token}` setiap 20 detik dan memuat ulang bila versi berubah, jadi **autosave terlihat** oleh pembaca publik.
  - Batas akses 300 permintaan per 15 menit per IP.
  - Semua kasus gagal memberi 404 yang sama: token salah atau tidak berbentuk, tautan dicabut, catatan dihapus, periode ditutup, atau batas akses terlampaui.
- **Isi yang tampil dibatasi daftar putih** (`api.PublicNote`): judul, nama divisi, tag, waktu dan tempat rapat, isi blok, dan waktu diperbarui.
  - Mention hanya tampil sebagai teksnya ("@Nama").
  - Dari butir checklist, PJ dan ID tugas dibuang.
  - Tautan ke Space dibuang; tautan ke situs lain tetap ada, dengan `nofollow`.
  - Blok yang tidak dikenal tidak ikut.
  - Tidak ada ID pengguna, email, NIM, nomor WA, peran, komentar, lampiran, kunci editor, maupun kehadiran per orang.
- **Tag**:
  - Diatur di panel kanan › Rapat › **Tag** oleh siapa pun yang punya `notes.manage`, tanpa perlu memegang editor.
  - Ditulis huruf kecil; spasi diganti "-"; tanda "#" dan duplikat dibuang. Paling banyak 10 tag, masing-masing paling banyak 32 karakter.
  - Tag tampil di halaman publik.
- **Audit**: `note.public_link.create`, `note.public_link.renew`, `note.public_link.revoke`, `note.tags.set`.
- **Migrasi `0031_note_sharing_templates.sql`** (aditif):
  - Kolom `notes.tags`.
  - Tabel `note_public_links`, dengan indeks unik parsial sehingga hanya ada satu tautan aktif per catatan, dan trigger realtime.
  - Constraint `template_kind` ditambah `evaluation` dan `brief`.
- **Endpoint baru**:
  - `POST /api/notes/{id}/public-link`
  - `DELETE /api/notes/{id}/public-link`
  - `PUT /api/notes/{id}/tags`
  - `GET /api/public/notes/{token}`
  - `GET /n/{token}`

## C5: Template
- **Template bawaan**: Notulen rapat divisi, Evaluasi, Rapat singkat, Kosong, dan Tabel Jadwal.
- **Isi Notulen**: Agenda (butir bernomor, masing-masing 10 menit untuk timer), Pembahasan, Keputusan, dan Action item (checklist dengan PJ, tenggat, dan "Jadikan tugas").
- **Isi otomatis dari rapat terjadwal**:
  - Bila Notulen dipilih dan ruang itu punya rapat terjadwal yang belum punya notulen, popover menampilkan **Isi dari rapat terjadwal**, aktif secara bawaan.
  - Judul boleh dikosongkan; server mengisinya "Notulen <rapat> — 17 Nov 2026".
  - Baris pertama berisi waktu dan tempat rapat. Agenda diambil dari baris-baris deskripsi rapat, dengan penomoran dan tanda "-" dibuang.
  - Rapat langsung tertaut ke catatan lewat `meetings.note_id`.
  - Satu rapat hanya bisa punya satu notulen; percobaan kedua dijawab 409.

## Test
- **Server** (dijalankan pada Postgres 16 sementara di homeserver):
  - `TestNotePublicLinkAccessAndPrivacy` memeriksa:
    - Pembaca dari divisi lain mendapat 404. Anggota yang bukan penulis tidak bisa membagikan.
    - Token tidak terkirim lagi lewat `GET` catatan.
    - JSON dan HTML publik tidak bocor: tidak ada ID anggota atau tugas, `mention`, `assigneeId`, email, atau tautan Space.
    - Header noindex, no-store, dan no-referrer terkirim.
    - Autosave terlihat di halaman publik. Tag dirapikan dan divalidasi.
    - Membuat ulang tautan membuat yang lama 404; mencabut membuat 404; token asal-asalan juga 404.
    - Audit tercatat.
    - Setelah periode ditutup, tautan menjadi 404.
    - Izin arsip untuk Medhum lama tidak membuka catatan divisi Mediatek, dan catatan itu tidak muncul di daftar catatan Medhum.
  - `TestNoteTemplatesAndMeetingFill` memeriksa:
    - Nama-nama template.
    - Pembuatan catatan Evaluasi.
    - Notulen dari rapat: judul, baris waktu dan tempat, agenda dengan menit, dan keempat judul bagian.
    - Rapat tertaut ke catatan. Notulen kedua untuk rapat yang sama mendapat 409. Rapat dari ruang lain mendapat 400.
- **Aplikasi**:
  - `TestNoteShareAndTags`: popover Bagikan, salin tautan, matikan, tombol tidak tampil bagi yang tidak berhak, dan tag.
  - `TestNoteCreateFromScheduledMeeting`.
- **Screenshot** (renderer native, data dummy):
  - `docs/screenshots/catatan-c6-bagikan.png`
  - `docs/screenshots/catatan-c5-template.png`

## {{Asumsi}}
- Lampiran tidak ikut ditampilkan di halaman publik.
- Kehadiran tidak ditampilkan di halaman publik.
- Tautan tidak punya masa kedaluwarsa otomatis.
- Fitur "Kirim ringkasan ke grup WA" (§5) ditunda.
- Penulis yang tidak lagi punya `notes.manage` kehilangan hak berbagi, tetapi tautan yang sudah dibuatnya tetap aktif sampai dicabut oleh kadiv.
