# Rencana — Anggota A dan Catatan rapat A (+ timer)

Status: **rencana** (9 Nov 2026; keputusan owner 9 Nov: Keluarkan wajib alihkan tugas, timer lokal lalu bersama, tanpa edit live tetapi autosave, privasi divisi atau publik lewat tautan, email lewat SES ke @mail.unej.ac.id; usulan NIM/WA di §8). A1 dikerjakan di branch `feat/anggota-a1`. Desain terpilih owner:

- Anggota → canvas `0b2425c385f0`, desain 1 (A · Desktop, winner) dan 2 (A · HP).
- Catatan rapat → canvas `8eef2f092aab`, desain 1 (A · Desktop, winner) dan 2 (A · HP), dengan **timer rapat opsional** (gaya timer agenda varian B).

Semua tanda {{Asumsi}} perlu dikonfirmasi owner sebelum tahap yang menyentuhnya. Data di canvas adalah dummy.

## 1. Yang dibangun

### Anggota A
- Tabel tetap (Nama, Role, Kelompok, Beban aktif, WhatsApp, aksi), dengan isi yang lebih mudah dibaca: avatar, badge role (Kadiv gelap, 45 aksen, Pendamping biru, Anggota netral, role kustom netral), beban sebagai bar 8 segmen (aktif abu, lewat tenggat merah) + teks "5 aktif · 1 lewat", WhatsApp sebagai status berwarna.
- Kepala halaman: kalimat kewenangan, **Izin arsip** (outline), **Undang anggota** (utama).
- Bilah alat: cari nama, chip filter Semua / Pengurus / Anggota / Belum tertaut / Ada yang lewat, urutkan (Beban, Nama).
- Aksi baris: **Ping**, **Ubah akses**, menu ⋯ (Lihat tugas, Keluarkan…). Baris kadiv sendiri: "Pemilik divisi", tanpa aksi.
- **Drawer Ubah akses** (desktop menimpa dari kanan, HP bottom sheet): Role dasar (kartu radio + penjelasan) → Wakil operasional (45) sakelar + penjelasan → Kelompok (chip, "tidak menambah kewenangan") → Role kustom (checkbox + kewenangannya, tautan Kelola role kustom) → **Ringkasan "Yang bisa dilakukan …"** (dihitung di klien) → kotak merah **Keluarkan dari divisi** → kaki "N perubahan belum disimpan · Batal · Simpan akses".
- **Dialog Keluarkan**: akibatnya, pilihan "Alihkan N tugas aktif ke …", Batal / Keluarkan anggota (merah).
- Panel bawah: Undangan tertunda (Kirim ulang / Cabut), Kewenangan role (ringkas).
- Keadaan kosong: hasil cari kosong (Hapus pencarian / Undang lewat email) dan divisi tanpa anggota.

### Catatan rapat A
- Bilah dokumen: remah Rapat › judul, label Notulen, "Diedit baru saja", avatar yang sedang membuka, tombol privasi + Bagikan, ⋯.
- Dokumen satu kolom 720 px: judul besar, **properti rapat** (Tanggal, Tempat, Kehadiran hadir/izin, Tag, Timer), blok.
- Blok: handle **+** dan **⠇** di kiri blok (hover), menu **/** dengan grup *Notulen* (Action item, Keputusan, Kehadiran) di atas *Blok dasar* (Heading, Checklist, Toggle, Callout, dst.), **@** untuk anggota dan tugas (#T), **#tag**.
- Keputusan = blok callout hijau. Action item = checklist + @PJ + tenggat + **Jadikan tugas** (setelah dibuat menjadi chip `#T22`).
- Popover Bagikan: **Hanya divisi ini** (bawaan) / **Publik lewat tautan** (Salin tautan, Matikan, Buat tautan baru), sakelar "Kirim ringkasan ke grup WA". (Canvas masih menampilkan opsi lama; ikuti dokumen ini.)
- Catatan baru: pemilih template (Notulen rapat divisi, Evaluasi, Rapat singkat, Kosong) + info rapat terjadwal berikutnya.
- HP: properti dilipat jadi satu baris, bilah blok di atas keyboard (+, H2, checklist, keputusan, @, #, urungkan), grid blok di sheet.

### Timer rapat (opsional)
- Properti **Timer**: tombol "Mulai timer" (+ "opsional · 45 mnt untuk 3 agenda"). Saat jalan: waktu total, "dari 45:00 · Agenda 2", Jeda, Hentikan.
- Durasi per agenda: chip durasi di butir agenda (atau heading agenda). Saat jalan: agenda selesai "✓ 14:20", agenda sekarang "08:12 tersisa" + bar, berikutnya durasi rencana.
- **Pil menempel** di atas dokumen (desktop kanan, HP tengah): agenda sekarang + sisa waktu + total, Jeda, Sembunyikan (×). Tidak ikut fokus ketik, tidak menutupi teks (berada di alur dokumen, sticky).
- **Waktu agenda habis**: pil berubah kuning "Waktu agenda 2 habis +01:30" dengan **+5 mnt** dan **Agenda 3 →**; di HP bilah kuning di bawah properti. Tanpa suara/modal.
- Timer tidak wajib: tanpa durasi agenda, timer hanya menghitung total.

## 2. Kondisi kode sekarang (yang dibaca)

| Area | Berkas | Catatan |
|---|---|---|
| Anggota | `internal/appui/members_view.go` (457 baris), `members.go` (293) | Tabel kolom tetap (`memberColumns`), "Ubah role" membuka `memberRowEditor` inline; `memberEditor` (role dasar, kelompok khusus Mediatek, role kustom, Simpan akses); konfirmasi `remove-member`, `revoke-invite`, `delete-role`, `appoint45` (+ "Sesuaikan kewenangan 45"), `remove45`; `memberLoad` menghitung tugas belum selesai dari tugas Space yang sudah dimuat. |
| API anggota | `internal/api/access.go` | `Member` punya Role, Roles, Groups, CustomRoleIDs, Capabilities, WhatsAppLinked (*bool). **Tidak ada email** dan tidak ada tanggal bergabung. 45 hanya satu per divisi (`Remove45` tanpa user). |
| Catatan | `notes_document.go` (759), `notes_controller.go` (1303), `notes_view.go` (671) | Editor blok berbasis panel (pilih blok → `blockEditor`), paging blok 8/halaman, kunci editor tunggal (Acquire/Takeover/Release), versi + draf pemulihan, "Jadikan tugas/reminder" dari blok/butir (`OnExtract`, `NoteLinkTask`). |
| API catatan | `internal/api/notes.go`, `notes_validate.go` | Blok: heading, paragraph, bulleted/numbered list, checklist, table, divider. Span: text, bold, link. **Tipe tak dikenal ditolak** saat tulis. `Note.MeetingID`, `TemplateKind=meeting`. Tidak ada field privasi. Isi `jsonb` (`0005_notes.sql`). |
| Rapat | `internal/appui/meetings.go`, `internal/api/meetings.go` | `Meeting` punya Participants dengan `AttendanceStatus`/`AttendanceNote`, `RecordAttendance`, `NoteID`. Tidak ada agenda terstruktur. |
| Komponen | `task_detail.go` (modal), `tasks_view.go` (`cancelTaskDialog`), `chrome.go` (`pingDialog`), drawer Buat tugas | Pakai ulang pola drawer/modal/bottom sheet yang sudah ada (menu ⋯ kartu board, d4d9ba1, sudah punya bottom sheet HP). |

## 3. Data, API, migrasi

### Anggota — tanpa migrasi
- Semua aksi sudah punya endpoint: `UpdateMember` (role, groups, customRoleIds), `Appoint45`/`Remove45`, `RemoveMember`, undangan, role kustom.
- Ringkasan kewenangan: dihitung di klien dari role dasar + 45 + role kustom terpilih memakai tabel yang sama dengan `operationalRoleChoices` dan teks "Kewenangan role". {{Asumsi}} cukup untuk pratinjau; sumber kebenaran tetap `Capabilities` dari server setelah simpan.
- 45 di drawer: menyalakan sakelar saat sudah ada 45 lain → dialog "Ganti 45 dari X ke Y?" (Remove45 lalu Appoint45, atau satu endpoint bila server sudah mengizinkan). "Sesuaikan kewenangan 45" tetap ada sebagai tautan di bawah sakelar.
- Simpan akses = satu `UpdateMember` + (bila berubah) panggilan 45 terpisah; urutan dan pesan galat per langkah.
- Email di baris: **tidak** ditampilkan di A1 (API tidak mengirim email anggota). Identitas anggota lewat NIM dibahas di §8 (keputusan owner menyusul).
- "Lewat tenggat" per anggota: dihitung dari tugas Space yang sudah dimuat (perluasan `memberLoad`), tanpa request baru.
- Dialog Keluarkan — **diputuskan owner (9 Nov)**: tugas aktif wajib dialihkan dulu. Dialog menampilkan jumlah tugas aktif + "Lihat tugas" dan tombol Keluarkan nonaktif sampai jumlahnya 0. Server ikut menolak `RemoveMember` bila anggota masih PJ tugas aktif (pesan jelas), supaya klien lama tidak bisa melewatinya {{Asumsi}}: cek dulu apakah server sudah punya aturan ini. Pengalihan massal (endpoint atomik) = tahap terpisah sesudahnya.

### Catatan — blok baru di dokumen (tanpa migrasi tabel)
Isi tetap `jsonb`; yang berubah validasi + tipe:

| Blok/atribut | Bentuk usulan | Catatan |
|---|---|---|
| `callout` | `{type:"callout", variant:"info"\|"decision", spans}` | Keputusan = `variant:"decision"`. |
| `toggle` | `{type:"toggle", text, children:[NoteBlock]}` | Kedalaman maks 1 {{Asumsi}}; masuk hitungan `NoteMaxBlocks`. |
| Action item | butir `checklist` dengan `assigneeId?`, `dueOn?`, `taskId?` | Tidak perlu tipe baru; `taskId` terisi setelah Jadikan tugas (juga tetap buat `NoteLink`). |
| Mention | `Span{mention:{kind:"user"\|"task", id}}` | Teks span tetap berisi nama (fallback untuk klien lama). |
| Tag | `Note.Tags []string` | Disimpan di kolom note, bukan di isi. |
| Durasi agenda | butir `numbered_list` dengan `minutes?` (atau heading dengan `minutes?`) | Dipakai timer. |

- `ValidateNoteContent` (server + klien) menerima tipe/field baru; klien lama yang menerima tipe tak dikenal harus menampilkan blok "tidak didukung" alih-alih membuang isi. **Risiko kompatibilitas**: kirim perubahan server lebih dulu, klien menyusul.
- **Privasi — diputuskan owner (9 Nov, revisi kedua)**: tidak ada "Hanya saya" dan tidak ada "bagikan ke semua kadiv". Hanya dua pilihan:
  1. **Hanya divisi ini** (bawaan, perilaku sekarang).
  2. **Publik lewat tautan**: siapa saja yang memegang tautan bisa membaca **tanpa login**.
- **Rancangan tautan publik yang aman**:
  - Token acak 32 byte dari `crypto/rand` (base64url, ±43 karakter) di URL `/n/<token>`. Server hanya menyimpan **hash SHA-256**, pola yang sama dengan token undangan dan kode `!link`. Token tidak bisa ditebak atau dihitung dari ID catatan.
  - **Hanya-baca**: tanpa komentar, tanpa lampiran tersembunyi, tanpa kunci editor. Lampiran ikut tampil hanya bila owner memutuskan ({{Asumsi}} bawaan: lampiran tidak ikut).
  - **Bisa dicabut dan dibuat ulang**: "Matikan tautan" menandai `revoked_at`; "Buat tautan baru" mencabut yang lama dan membuat token baru (tautan lama langsung 404). Opsional kedaluwarsa (`expires_at`).
  - **Siapa yang boleh menyalakan/mencabut**: kadiv divisi pemilik, atau notulis (pembuat catatan, `Note.CreatedBy`) yang masih punya `notes.manage`. 45 dan anggota lain tidak. Setiap nyala/cabut/buat ulang tercatat di audit.
  - **Tidak diindeks**: halaman publik mengirim `X-Robots-Tag: noindex, nofollow` dan `<meta name="robots" content="noindex">`, `Referrer-Policy: no-referrer`, `Cache-Control: private, no-store`, dan tidak masuk sitemap. Batas laju per IP dan token salah selalu 404 yang sama (tidak membocorkan apakah token pernah ada).
  - **Tidak membocorkan data**: tampilan publik disusun di server dari daftar putih: judul, tanggal/tempat rapat, isi blok, nama divisi. Mention anggota hanya tampil sebagai **nama tampilan**; tidak ada ID pengguna, email, **NIM**, **nomor/JID WhatsApp**, peran, avatar dari server, beban kerja, maupun kehadiran per orang ({{Asumsi}}: cukup "6 hadir, 1 izin"). Mention tugas tampil sebagai teks `#T22 Judul` tanpa tautan ke Space. Tautan di span yang mengarah ke Space dihapus.
  - Ikon dan label jelas di editor: "Publik — siapa pun dengan tautan bisa membaca" + jumlah dibuka ({{Asumsi}} opsional).
- **Migrasi kecil** `0028_note_meta.sql`:
  - `notes.tags text[] not null default '{}'`
  - tabel `note_public_links(id uuid primary key, note_id uuid not null references notes on delete cascade, token_hash bytea not null unique, created_by uuid not null references users, created_at timestamptz not null, expires_at timestamptz null, revoked_at timestamptz null, last_viewed_at timestamptz null, view_count int not null default 0)` + indeks unik parsial `on note_public_links(note_id) where revoked_at is null` (satu tautan aktif per catatan).
  - trigger realtime pada `note_public_links` → `space_note_child_changed` agar status "Publik" langsung terlihat penyunting lain.
  - Menghapus catatan atau menutup periode ({{Asumsi}}) mencabut tautan aktif.
- **API**: `Note.PublicLink *{createdAt, expiresAt, createdBy}` (token tidak pernah dikirim ulang; hanya ditampilkan sekali saat dibuat, lalu bisa disalin dari editor selama sesi itu atau dibuat ulang), `POST /notes/{id}/public-link` (buat/buat ulang, mengembalikan URL sekali), `DELETE /notes/{id}/public-link` (cabut), `GET /public/notes/{token}` (tanpa auth, DTO publik terpisah dari `api.Note`).
  {{Asumsi}} Bila owner ingin tautan bisa disalin kapan saja, token disimpan terenkripsi (bukan hanya hash). Bawaan rencana: hash saja, lebih aman.
- "Kirim ringkasan ke grup WA": memakai layanan bot yang ada (Tahap 7) — pesan teks berisi keputusan + action item + tautan. Endpoint baru `POST /notes/{id}/summary/whatsapp` {{Asumsi}}; boleh ditunda.
- Kehadiran di properti: **bukan** data catatan; baca/ubah `Meeting.Participants` lewat `RecordAttendance` bila `Note.MeetingID` ada. Catatan tanpa rapat tidak menampilkan baris Kehadiran.

### Timer — rekomendasi: **status di server, sederhana**
- Pilihan lokal saja: tiap perangkat punya timer sendiri, hilang saat pindah perangkat, peserta lain tidak melihat. Murah, tapi tidak cocok untuk rapat (notulis di laptop, kadiv di HP).
- Rekomendasi: simpan **status timer di `meetings`** (bukan di isi catatan, supaya tidak menaikkan versi/konflik kunci editor):
  `0029_meeting_timer.sql` — `timer_started_at timestamptz null`, `timer_paused_at timestamptz null`, `timer_paused_seconds int not null default 0`, `timer_agenda_index int not null default 0`, `timer_agenda_started_at timestamptz null`, `timer_extra_seconds int not null default 0`.
  Endpoint `POST /meetings/{id}/timer` dengan aksi `start|pause|resume|next|extend(+300)|stop`, ExpectedVersion seperti `MeetingRequest`. Waktu dihitung di klien dari stempel waktu server (tanpa tick ke server). Durasi agenda dibaca dari blok agenda catatan.
- Siapa yang boleh: yang boleh menyunting catatan atau mengelola rapat (`Meeting.CanManage`). Yang lain hanya melihat pil.
- "Sembunyikan" = preferensi lokal per perangkat (tidak mengubah timer).
- Catatan tanpa rapat: timer lokal saja (tanpa endpoint), dengan teks "Timer hanya di perangkat ini".

## 4. Tahap kecil (dikirim satu per satu)

Setiap tahap: branch sendiri dari tip terbaru, test lolos (kecuali 3 test NoteDocument lama), gofmt, laporan di `docs/handoff/`, tanpa deploy.

**Anggota**
1. **A1 — Tabel baca**: avatar, badge role, bar beban (+ lewat tenggat dari tugas termuat), status WA berwarna, kepala halaman (Izin arsip, Undang), cari + chip filter + urutkan, keadaan kosong. Tanpa perubahan alur edit. Berkas: `members_view.go`.
2. **A2 — Drawer Ubah akses (desktop) + sheet (HP)**: pindahkan `memberEditor` ke drawer; urutan bagian, kartu radio dengan penjelasan, kelompok, role kustom, ringkasan kewenangan, kaki dengan hitungan perubahan; 45 sebagai sakelar (dialog ganti 45). Hapus panel inline. Berkas: `members.go`, `members_view.go`, komponen drawer/sheet bersama.
3. **A3 — Keluarkan yang aman**: kotak merah di drawer + dialog dengan jumlah tugas aktif; blokir bila masih ada tugas (tautan Lihat tugas). Pengalihan massal = tahap terpisah setelah keputusan.
4. **A4 — Panel bawah**: Undangan tertunda + Kewenangan role ringkas; menu ⋯ baris.

**Catatan rapat**
5. **C1 — Server blok baru** (tanpa UI): tipe `callout`/`toggle`, field butir checklist (`assigneeId`, `dueOn`, `taskId`), span `mention`, `minutes` agenda, **`id` stabil per blok** (disiapkan untuk kunci per blok, §7); validasi + test di `internal/api/notes_validate*.go` dan server. Klien lama: render "blok tidak didukung".
6. **C2 — Tata letak dokumen**: bilah dokumen, judul besar, properti (Tanggal/Tempat dari rapat, Kehadiran dari `Meeting.Participants`), kolom 720 px, autosave "Diedit baru saja" dari alur draf/simpan yang ada.
    **C2a — Autosave** (keputusan owner, §7): simpan otomatis setelah ±4 detik diam, editor tidak tereset, riwayat digabung, pembaca diperbarui tanpa lompat.
7. **C3 — Interaksi blok**: handle + / ⠇ (sisip, seret = `moveBlock`), menu `/` (grup Notulen dulu), bilah blok HP. Ini perubahan terbesar pada `notes_document.go`; pertahankan kunci editor dan paging untuk dokumen panjang.
8. **C4 — Mention & action item**: `@` anggota/tugas, butir action item dengan PJ + tenggat, "Jadikan tugas" mengisi `taskId` + chip #T (memakai `OnExtract`/`NoteLinkTask` yang ada).
9. ✅ (0.4.2) **C5 — Template & catatan baru**: pemilih template; template Notulen berisi Agenda (dengan menit), Pembahasan, Keputusan, Action item; isi otomatis dari rapat terjadwal.
10. ✅ (0.4.2, migrasi 0031) **C6 — Tautan publik & tag**: popover Bagikan (Hanya divisi ini / Publik lewat tautan), endpoint buat/cabut/buat ulang, halaman publik hanya-baca tanpa login dengan noindex dan DTO daftar putih, audit, tag.
11. **T1 — Timer lokal** (disetujui owner, dikerjakan sebelum T2): properti Timer, durasi agenda, pil sticky, jeda/sembunyikan, peringatan waktu habis (+5 mnt / agenda berikutnya). Semua di klien.
12. **T2 — Timer bersama** (migrasi 0029 + endpoint): status di server, sinkron lewat pembaruan rapat yang ada.
13. **C7 — Ringkasan ke WhatsApp** (opsional, setelah keputusan).

Urutan yang disarankan: A1 → A2 → A3 → C1 → C2 → C2a → T1 → C3 → C4 → A4 → C5 → C6 → T2 → C7. A1 dan C1 bisa paralel (berkas berbeda).

## 5. Test

- Anggota: test tabel (label role, teks beban + lewat, WA nil/true/false), filter/cari, drawer hanya untuk yang berhak (`memberCanEdit`, 45 tidak bisa atur akses), hitungan perubahan dan isi `UpdateMemberRequest`, ringkasan kewenangan per kombinasi role, dialog ganti 45, Keluarkan diblokir saat ada tugas aktif.
- Catatan: `ValidateNoteContent` untuk tiap tipe/field baru (batas ukuran, kedalaman toggle, mention id kosong), round-trip JSON, klien lama menampilkan blok tak dikenal tanpa kehilangan isi, `moveBlock`/sisip lewat handle, menu `/` memasukkan blok yang benar, Jadikan tugas mengisi `taskId` dan `NoteLink`, pembaca tanpa izin tugas hanya dapat reminder (keputusan 9 Nov tetap).
- Timer: fungsi murni `timerState(now, meetingTimer, agendaMinutes)` → agenda aktif, sisa, lewat (tabel kasus: jeda, lanjut, +5 mnt, agenda tanpa durasi, lewat tengah malam); sembunyikan tidak mengubah status; T2: endpoint aksi + konflik versi.
- Semua build/test lewat lock memori (`flock /tmp/space-build.lock … GOMAXPROCS=1 …`) seperti brief canvas A.

## 6. Risiko

- **Editor blok gaya Notion di MyGo**: handle hover, seret dan menu `/` di posisi kursor bergantung kemampuan MyGo v0.7.1 (fokus teks, posisi caret, drag). Mulai C3 dengan spike kecil; cadangan: + / ⠇ tampil di blok terpilih (bukan hover), seret diganti Naik/Turun.
- **Kompatibilitas blok**: klien lama menolak/membuang tipe baru. Kirim C1 server + fallback render lebih dulu, tunda pemakaian tipe baru sampai web/desktop diperbarui.
- **Kunci editor tunggal**: timer dan kehadiran tidak boleh menulis isi catatan (karena itu disimpan di rapat).
- **Ringkasan kewenangan di klien bisa beda dengan server**: tampilkan ulang dari `Capabilities` setelah simpan; teks penjelasan role satu sumber.
- **45 satu per divisi**: sakelar di drawer harus jelas soal mengganti 45 yang ada.
- **Email anggota tidak ada di API**: desain menampilkan email; diganti info lain kecuali owner minta perubahan API.
- **Tautan publik** (C6) membuka data ke luar login: test bahwa DTO publik tidak memuat ID pengguna, NIM, JID/nomor WA atau tautan internal; token dicabut = 404; noindex + no-store terkirim; hanya kadiv/notulis yang bisa menyalakan.
- RAM VPS 1,9 GB: tahap kecil, satu build sekaligus.

## 7. Bisakah catatan diedit bareng secara live?

**Keputusan owner (9 Nov): edit live bareng tidak diperlukan.** Gantinya **autosave otomatis**: setelah notulis berhenti mengetik beberapa detik, catatan tersimpan sendiri, dan pembaca melihat pembaruan itu tanpa memuat ulang. Hasilnya terasa live secara tidak langsung. Kunci per blok dan CRDT di bawah dicatat sebagai pembanding saja, tidak dikerjakan.

### Apakah alur sekarang sudah mendukung autosave? (dibaca dari kode)
- **Belum sepenuhnya.** `tickNotes` (`internal/appui/notes_controller.go`) sudah punya dua tahap debounce:
  - 750 ms setelah berhenti mengetik → `persistNoteLocal` (salinan lokal di perangkat);
  - 3 detik → `saveNoteRecovery` → `SaveNoteDraft`, yaitu **draf pemulihan pribadi** per akun. Draf ini **tidak** mengubah catatan resmi dan **tidak** terlihat pembaca.
  Simpan resmi (`saveNote` → `UpdateNote`, naik versi) masih lewat tombol Simpan.
- **Sisi pembaca sudah siap.** Setiap tulis ke tabel `notes` memicu `pg_notify('space_changes')`, dan klien yang membuka catatan memuat ulang detailnya (`realtime.go` → `loadNoteDetail`). Begitu autosave memanggil `UpdateNote`, pembaca melihat versi baru dalam ±1 detik.
- **Yang perlu diubah**:
  1. Debounce resmi: bila memegang kunci, `dirty`, judul valid dan `ValidateNoteContent` lolos, panggil `saveNote` setelah **±4 detik diam** (dan paling lambat tiap 30 detik saat terus mengetik). Draf pemulihan 3 detik tetap ada sebagai jaring pengaman saat offline/galat.
  2. **Jangan mereset editor setelah simpan.** Sekarang `saveNote` mengganti `a.notes.editor = newNoteDocumentEditor(n.Content)` bila revisi belum berubah; untuk autosave itu akan memindahkan fokus/blok terpilih di tengah mengetik. Ganti dengan memperbarui baseline + versi saja dan mempertahankan editor.
  3. Ketikan baru selama simpan berjalan: simpan berikutnya memakai versi baru (`ExpectedVersion`) dan tetap `dirty`; tidak ada data hilang.
  4. **Riwayat**: setiap `UpdateNote` menulis `note_events`; autosave beruntun dari penyunting yang sama dalam ±5 menit digabung jadi satu entri riwayat (server) supaya riwayat tidak penuh.
  5. **Pembaca**: muat ulang detail tanpa melompat; pertahankan posisi gulir, jangan menimpa komentar yang sedang diketik, tampilkan "Diperbarui 19.42 oleh Sinta" sebentar.
  6. Status di bilah dokumen: "Menyimpan…" → "Tersimpan · baru saja" → bila gagal "Belum tersimpan — coba lagi" (draf pemulihan tetap aman).
  7. Mode offline: tetap antre seperti `queueNote` sekarang; autosave tidak berjalan saat offline.
- **Tahap baru C2a — Autosave** (setelah C2, sebelum C3): poin 1–3 dan 6 di klien, poin 4 di server, poin 5 di klien pembaca. Test: debounce (tidak menyimpan saat masih mengetik, menyimpan setelah diam, batas 30 detik), editor tidak tereset, konflik versi, pembaca menerima versi baru tanpa kehilangan gulir, penggabungan riwayat.

### Kondisi kunci (tetap)
- **Satu penyunting per catatan.** `AcquireNoteLock` memberi kunci penyunting; kunci berlaku 5 menit (`noteLockTTL`, `internal/server/notes.go:24`) dan diperpanjang selama menyunting. Pengelola bisa **Ambil alih** (`TakeoverNoteLock` dengan `Force`). Dengan autosave, pengambilalihan tidak lagi membuang banyak ketikan karena yang belum tersimpan paling lama beberapa detik.
- Tambahan kecil yang tetap berguna: tampilkan **siapa yang sedang menyunting** di bilah dokumen pembaca.

### Pembanding (tidak dikerjakan): kunci per blok
- Setiap blok punya `id` stabil (disiapkan di C1). Kunci berpindah dari catatan ke blok: `note_block_locks(note_id, block_id, user_id, expires_at)` dengan TTL pendek (sekitar 60 detik, diperpanjang selama fokus).
- Simpan berubah dari "ganti seluruh isi" menjadi operasi: `PATCH /notes/{id}/blocks` dengan `insert|update|move|delete` per `block_id` + versi blok. Konflik hanya terjadi bila dua orang menyentuh blok yang sama; urutan disatukan server.
- Tampilan: blok yang dipegang orang lain diberi garis + avatar ("Sinta sedang menulis") dan tidak bisa difokus; blok lain bebas. Semua orang menerima perubahan per blok lewat aliran realtime yang sama.
- Biaya: sedang sampai besar (migrasi id blok di `jsonb` yang ada, endpoint patch, editor menyimpan per operasi, kehadiran). Perkiraan 2–3 tahap setelah C4. Langkah lebih ringan sebelum itu: tetap satu kunci, tambahkan **kehadiran** ("Sinta sedang menyunting") dan tombol **Minta giliran** yang memberi tahu penyunting.

### Pembanding (tidak dikerjakan): co-editing realtime penuh (CRDT)
- Butuh dokumen CRDT per catatan (Yjs/Automerge), sinkron dua arah terus-menerus (WebSocket), penyimpanan update + snapshot, konversi CRDT ↔ blok JSON untuk validasi, pencarian, ekspor, arsip dan bot, sinkron kursor di input teks MyGo, serta penggabungan offline.
- MyGo (Go ke WASM/desktop) tidak punya binding Yjs siap pakai: harus porting atau jembatan JS, ditambah klien desktop. Koneksi terus-menerus juga membebani VPS 1,9 GB.
- Perkiraan: beberapa minggu, risiko tinggi. **Tidak disarankan sekarang.** Kunci per blok memberi sebagian besar manfaat untuk notulen (beberapa orang menulis di bagian berbeda) dengan biaya jauh lebih kecil.

## 8. Usulan: identitas NIM dan penautan WhatsApp tanpa `!link` di grup

Status: **diputuskan owner 10 Nov 2026 dan diimplementasi di `feat/nim-wa-ses`**; hasilnya ada di `docs/handoff/nim-wa-ses-hasil.md`. Ada beberapa perbedaan dari usulan di bawah:
- NIM terlihat oleh semua anggota divisi, bukan hanya pengurus.
- Domain mahasiswa menjadi konstanta (`api.StudentEmailDomain`) dan kolom generated, bukan konfigurasi.
- Kode tautan berlaku 15 menit.
- Tidak ada QR di desktop.
- Pesan pribadi selain kode diabaikan tanpa balasan.

### Kondisi sekarang
- **Undangan** (`internal/server/invitations.go`): kadiv mengundang email apa saja (`validEmail`) dan server mengirim tautan. Orang tanpa akun meminta **bukti email** (`verifyInvitationEmail`): token sekali pakai dikirim ke email itu, dan akun dibuat saat menerima undangan dengan bukti tersebut. Jadi email akun **sudah terbukti dimiliki**.
- **NIM tidak ada** di model mana pun (`api.User`, `api.Member`). Data anggota yang dikirim ke klien tidak memuat email.
- **Domain email mahasiswa: `@mail.unej.ac.id` (terverifikasi).** Backend website HIMASIF membentuk akun mahasiswa sebagai `$nim.'@mail.unej.ac.id'` (`D:/HIMASIF/himasif-id-v2-be/app/Console/Commands/generateUserAccount.php:54-77`), dan konteks Hermes menyebut filter blast BE `%@mail.unej.ac.id`. Jadi **bukan** `@unej.ac.id` (contoh dari user) dan bukan `@student.unej.ac.id` (hanya data uji di `canvas_tahap4_test.go`). Domain tetap dijadikan **konfigurasi** `SPACE_STUDENT_EMAIL_DOMAINS=mail.unej.ac.id`, bukan konstanta di kode.
- **Penautan WhatsApp** (`createWALink`, `consumeWALink`): Space membuat kode `SPACE-…` (sekali pakai, 10 menit, disimpan sebagai hash), lalu anggota mengetik `!link SPACE-…` **di grup** yang terhubung. Server mengikat JID pengirim (dan alt/LID) ke akun di `wa_identities`. Pesan di luar grup **diabaikan** (`processWACommand` menolak inbound tanpa grup), dan bridge hanya mengizinkan grup yang dipetakan.
- Penting: **email terverifikasi membuktikan NIM/akun, tetapi tidak membuktikan nomor WhatsApp.** Kepemilikan nomor hanya terbukti bila pesan berasal dari akun WhatsApp itu.

### NIM sebagai identitas
- **Undang dengan NIM**: form Undang menerima NIM (angka), lalu email dibentuk `NIM@mail.unej.ac.id` dan ditampilkan sebelum dikirim. Email lain tetap boleh (dosen, alumni) dengan peringatan "bukan email mahasiswa".
- **NIM diturunkan, bukan diketik**: saat akun dibuat atau menerima undangan dengan email yang domainnya terdaftar, bagian lokal email (pola {{Asumsi}} 12 digit, contoh `232410101090`) disimpan sebagai `users.nim`. Nilai ini terverifikasi karena emailnya sudah terbukti. Pengguna tidak bisa mengubahnya; admin organisasi bisa mengoreksi dengan audit.
- **Migrasi** `00xx_user_nim.sql`: `users.nim text null unique`, diisi otomatis dari email yang cocok, dengan indeks untuk pencarian.
- **API**: `Member.NIM *string` (dan `Invitation.NIM` bila email mahasiswa); pencarian anggota menerima NIM.
- **Siapa melihat NIM** (usulan): orangnya sendiri, **pengurus divisi** (kadiv, 45, pendamping), dan admin organisasi. Anggota biasa hanya melihat nama. Server hanya mengisi `Member.NIM` bila pemanggil berhak (kapabilitas baru `members.identity.read` {{Asumsi}}), jadi NIM tidak terkirim ke klien yang tidak boleh melihatnya. NIM tidak muncul di pesan bot grup, ekspor publik, maupun log aplikasi. Bila owner ingin semua anggota divisi melihat NIM, cukup ubah aturan kapabilitas.
- **Tabel Anggota**: baris kedua di bawah nama menampilkan NIM (mono) bagi yang berhak; yang lain melihat kelompok.

### Penautan WhatsApp: perbandingan opsi

| | (a) Tombol "Tautkan WhatsApp" → `wa.me/<bot>?text=<kode>` | (b) Isi nomor di profil → bot kirim konfirmasi | (c) Bot DM otomatis saat bergabung ke grup |
|---|---|---|---|
| Langkah pengguna | Ketuk tombol → WhatsApp terbuka dengan pesan terisi → **Kirim** | Ketik nomor → tunggu pesan bot → balas atau masukkan kode | Bergabung grup → buka DM bot → tetap perlu kode/tautan |
| Membuktikan nomor? | **Ya**: pesan datang dari akun WA itu, dan kode mengikat sesi Space yang membuatnya | Ya, bila pengguna membalas atau memasukkan kode | Bot tahu nomor/LID tetapi **tidak tahu akun Space mana** tanpa kode |
| Risiko diblokir WA (bridge tidak resmi) | Rendah: pengguna yang memulai chat | **Tinggi**: bot memulai DM ke nomor asing | **Tinggi**: DM proaktif massal |
| Salah ketik nomor | Tidak mungkin | Mungkin | Tidak mungkin |
| Perubahan | Bridge menerima DM berisi kode `SPACE-…` saja; `processWACommand` dan `consumeWALink` tidak lagi mewajibkan grup (divisi/periode diambil dari challenge); nomor bot publik di konfigurasi koneksi | Endpoint simpan nomor, pengiriman DM keluar, antrean konfirmasi | Event bergabung dari bridge, DM keluar, ditambah salah satu opsi lain |
| Privasi | Nomor bot terlihat anggota; nomor pengguna hanya disimpan sebagai JID (seperti sekarang) | Nomor diketik ke Space (data baru disimpan) | Bot memproses lebih banyak event grup; dengan LID nomor sering tersembunyi |

**Rekomendasi: (a)**, dengan `!link` di grup tetap ada sebagai cadangan.
- Alur: Space (banner di Beranda/Profil, dan status "Belum tertaut" di tabel Anggota untuk diri sendiri) → **Tautkan WhatsApp** → server membuat challenge seperti sekarang dan mengembalikan `deepLink`. Di HP WhatsApp langsung terbuka; di desktop tampil **QR** dari tautan yang sama untuk dipindai HP. Pengguna menekan Kirim, bot membalas "Tertaut ke akun Aruna Desain (NIM 2324…090)", dan Space memperbarui status lewat realtime.
- API: `WALinkChallenge.DeepLink string`; `WAConnection.BotNumber` (terlihat anggota); inbound DM (`wa_inbound.group_id` null) hanya untuk pola `^(!link\s+)?SPACE-[A-Za-z0-9_-]+$`. Pesan DM lain dibalas sekali "Bot hanya melayani tautan; gunakan grup divisi" lalu diabaikan. Ada batas laju per JID.
- Keamanan: kode sekali pakai 10 menit dan disimpan sebagai hash (sudah ada). Risiko kode diteruskan ke orang lain (orang itu menautkan nomornya ke akunmu) dicegah dengan tiga hal: balasan bot menyebut nama + NIM tersamar; Space menampilkan nomor tersamar "+62 812-••••-4471 · Bukan saya? Lepas tautan"; audit `whatsapp.identity.link` mencatat sumber `dm`.
- (c) cukup dipakai sebagai **pengingat di grup** (bot menyebut anggota baru: "tautkan lewat Space → Tautkan WhatsApp"), bukan sebagai DM.

### Pengiriman email undangan lewat Amazon SES (keputusan owner, 9 Nov)
- **Kode sudah siap, tidak perlu perubahan pengirim.** Server mengirim lewat SMTP biasa (`internal/config/config.go`: `SPACE_MAIL_DRIVER=smtp`, `SPACE_SMTP_HOST/PORT/USERNAME/PASSWORD/TLS`, `SPACE_MAIL_FROM` dengan bawaan `Space HIMASIF <noreply@himasif.id>`). SES menyediakan antarmuka SMTP, jadi cukup konfigurasi.
- **Pemetaan dari vault enowX `ses-smtp`** (nilai tidak pernah dicetak, ditulis ke berkas, atau di-commit): `SES_SMTP_HOST` → `SPACE_SMTP_HOST` (endpoint SMTP SES region **ap-southeast-2**), `SES_SMTP_PORT` → `SPACE_SMTP_PORT` (587 dengan `SPACE_SMTP_TLS=starttls`), `SES_SMTP_USER`/`SES_SMTP_PASS` → `SPACE_SMTP_USERNAME`/`SPACE_SMTP_PASSWORD`, `SES_FROM` → `SPACE_MAIL_FROM`. Di server produksi variabel ini masuk ke unit/env file API dengan izin 0600, dipasang oleh sesi yang men-deploy (`"$ENOWX_BIN" creds use ses-smtp -- …`), bukan disalin manual.
- **Aturan From**: wajib alamat `@himasif.id` (identitas SES `himasif.id` sudah DKIM OK; SPF/DMARC jangan diubah, sesuai konteks Hermes). Tambahkan validasi saat start: bila driver `smtp` dan domain `SPACE_MAIL_FROM` bukan `himasif.id`, server menolak start dengan pesan jelas. Ini satu perubahan kecil di `config.Validate`.
- **Sebelum dipakai**: pastikan akun SES sudah keluar dari *sandbox* di ap-southeast-2 (di sandbox hanya boleh mengirim ke alamat terverifikasi). {{Unverified}}: belum dicek. Kirim satu email uji ke alamat tim dengan `make mail-test` memakai kredensial dari vault.
- **Deliverability ke `@mail.unej.ac.id`**: email kampus bisa memfilter pengirim baru. Uji ke 2–3 NIM tim lebih dulu; subjek dan isi undangan sederhana tanpa banyak tautan; tangani bounce/complaint SES ({{Asumsi}}: cukup pantau di konsol SES dulu, notifikasi SNS menyusul).
- Tidak ada perubahan migrasi untuk SES. Masuk tahap **N2** (Undang dengan NIM) sebagai prasyarat konfigurasi, atau lebih awal bila undangan email biasa ingin langsung memakai SES.

### Tahap (setelah keputusan owner)
1. **N1**: konfigurasi domain mahasiswa + `users.nim` (migrasi, backfill) + `Member.NIM` sesuai hak + tampil di tabel Anggota.
2. **N2**: Undang dengan NIM (form, pratinjau email).
3. **W1**: Tautkan WhatsApp lewat DM (bridge + inbound DM + `DeepLink` + QR desktop + nomor tersamar dan Lepas tautan).
4. **W2**: pengingat bergabung di grup (opsional).

Yang perlu owner putuskan: siapa yang boleh melihat NIM; apakah bot boleh menerima DM (nomor bot terlihat anggota). Domain (mail.unej.ac.id) dan pengirim (SES, From di himasif.id) sudah diputuskan.

## 9. Catatan gaya C (keputusan owner setelah pilot 0.3.5)

Pilot 0.3.5: menulis terasa sempit dan terlalu banyak elemen; yang paling mengganggu adalah kartu besar di bawah catatan ("Draf pemulihan milikmu", "Komentar" dengan kotak Komentar baru). Owner memilih **varian C** di canvas `37348413b3ef` (desain 5 = winner, desain 6 = HP): kerangka Space tetap, ditambah **satu panel kanan bertab Rapat / Komentar / Riwayat**; di HP panel itu menjadi **satu sheet bertab**.

### Yang diganti dari C2/C3 yang sudah live
- **Dihapus dari bawah dokumen**: kartu draf pemulihan, kartu komentar, lampiran, template ruang, rujukan pekerjaan, riwayat, hapus catatan. Dokumen hanya berisi judul, chip, dan blok.
- **Properti rapat** (tanggal, tempat, kehadiran per orang) dan nanti **timer** (T1) pindah ke tab **Rapat**, bersama **Di halaman ini** (outline judul yang bisa diklik) dan lampiran. Di bawah judul tinggal chip ringkas (tanggal, "6 hadir · 1 izin", "Rapat ›") yang membuka tab itu.
- **Komentar** pindah ke tab **Komentar** (daftar ringkas + satu baris input, badge jumlah di tab).
- **Draf pemulihan dan riwayat** pindah ke tab **Riwayat**; di bilah atas hanya muncul chip kuning kecil "Draf" bila memang ada draf.
- **Bilah atas tipis** (± 44 px): remah Rapat › judul, label Notulen, chip draf (bila ada), chip kunci editor ("Kamu memegang editor" / "Sinta sedang mencatat" / "Belum ada pencatat aktif" + satu tombol yang pas: Ambil editor, Ambil alih, Selesai), status tersimpan, buka/tutup panel, menu ⋯ (simpan sebagai template, hapus catatan). Strip kunci di dalam dokumen dan bilah "Simpan sekarang" di bawah hilang (Ctrl+S tetap menyimpan seketika).
- **Dokumen lebih lebar dan polos**: kolom maks. 760 px tanpa kotak; menu blok C3 (+, ⠇, drag, "/") tetap dipakai.
- **Menu "/" ringkas gaya enowX**: satu kolom, ikon kecil, nama, dan **shortcut markdown di kanan** (`#`, `##`, `###`, `[]`, `-`, `1.`, `---`, …); grup **Notulen** (Action item, Keputusan, Kehadiran) tetap di atas.
- **Tidak berubah**: kunci editor tunggal + ambil alih, paging 8 blok, autosave, draf perangkat/server, data catatan dan API (**tanpa migrasi**).

### Tahap kecil (masing-masing bisa dirilis)
- **G1 — Panel bertab + bilah tipis** (rilis ini): panel kanan Rapat / Komentar / Riwayat (desktop terbuka, tablet tertutup, HP sheet bertab), bilah atas tipis dengan kunci + status + chip draf + ⋯, dokumen 760 px tanpa kartu bawah, chip rapat di bawah judul, outline Di halaman ini, form Jadikan tugas/reminder tampil di panel.
- **G2 — Menu "/" ringkas** (rilis ini bila sempat): baris satu kolom dengan shortcut markdown di kanan; grid HP tetap.
- **G3 — Edit | Baca + urungkan/ulangi** (dikerjakan di `feat/catatan-g3-timer-c4`): mode baca tanpa handle, riwayat urungkan di editor.
- **G4 — Pohon catatan dengan saringan** (ditunda ke rilis berikutnya supaya rilis G3/T1/C4 tidak terlalu besar untuk pilot): kelompok per bulan rapat/tag, saring Semua / Notulen / Milikku / Ada draf, daftar bisa diciutkan.
- **G5 — Timer di tab Rapat** (= T1, dikerjakan bersama G3) dan komentar per blok (penanda di margin; belum, menunggu keputusan owner).

### Hasil G3 + T1 + C4 (branch `feat/catatan-g3-timer-c4`)
Rincian dan daftar uji: `docs/handoff/catatan-g3-timer-c4-hasil.md`. Tanpa migrasi atau perubahan API: field C1 (`minutes`, `assigneeId`, `dueOn`, `taskId`, span `mention`) dipakai apa adanya. Timer T1 disimpan di memori aplikasi per catatan (hilang saat aplikasi ditutup; {{Asumsi}} cukup untuk T1, T2 memindahkannya ke server).
