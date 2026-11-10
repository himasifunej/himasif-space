# NIM, Tautkan WhatsApp sekali ketuk, dan email lewat SES (10 November 2026)

Branch `feat/nim-wa-ses` dari `origin/main` b38bb29 (0.3.6). Isinya implementasi usulan §8 di `docs/handoff/anggota-catatan-rapat-rencana.md`. **Belum di-push dan belum di-deploy.**

Keputusan owner yang dipakai:
- NIM boleh dilihat semua anggota divisi, tetapi tidak tampil di halaman publik.
- Tautan WhatsApp memakai opsi (a), dengan `!link` di grup tetap ada sebagai cadangan.
- Email dikirim lewat Amazon SES (identitas `himasif.id`, region ap-southeast-2, kredensial vault `ses-smtp`).
- Domain email mahasiswa `@mail.unej.ac.id`.
- Prinsipnya: alur sesimpel mungkin, karena owner tidak lagi menjadi pengurus mulai Desember.

## 1. Undang dengan NIM
- Field form Undang kini **"NIM atau email"**:
  - Bila diisi angka, form menampilkan **"Dikirim ke 232410101131@mail.unej.ac.id"** sebelum dikirim.
  - Bila diisi email lain, undangan tetap bisa dikirim dengan peringatan kuning "Bukan email mahasiswa UNEJ… misalnya untuk dosen atau alumni".
  - Bila NIM salah format, muncul pesan merah dan tombol kirim mati.
- **Server** (`InviteRequest.NIM`) membentuk alamatnya sendiri. NIM berisi 8–20 digit; NIM dan email sekaligus ditolak 400.
- **Email**:
  - Dikirim sebagai teks dan HTML sederhana bergaya Space (krem, kartu, tombol gelap) lewat `mail.Letter` di `internal/mail/template.go`. Desain final dari canvas email cukup mengganti berkas itu.
  - Undangan, verifikasi, pemulihan dan serah terima admin semuanya memakai format ini.
- **Batas kirim**: `SPACE_MAIL_PER_MINUTE` (bawaan 20) berlaku untuk semua alamat sekaligus dan menjawab 429 bila terlewati. Batas 5 email per alamat per 15 menit tetap berlaku.
- **Penolakan dan bounce**:
  - Balasan SMTP 5xx (alamat ditolak, atau di sandbox SES alamat yang belum terverifikasi) menjadi 422 "Server email menolak alamat …; periksa NIM-nya", dan undangan tidak dibuat.
  - Bounce permanen dan komplain dari SES (lewat SNS ke `POST /api/mail/ses-events`, hanya pesan bertanda tangan SNS untuk topik yang dikonfigurasi) menandai undangan **"Tidak terkirim · alasan"** dengan warna merah. Kirim ulang menghapus tanda itu.
- **Mematikan email**: cukup set `SPACE_MAIL_DRIVER=disabled` (atau `log`). Langkah SES ada di `docs/operations/ses-email.md`.

## 2. NIM di tabel Anggota
- `users.nim` adalah **kolom generated** (migrasi 0028), diturunkan dari email terverifikasi `<NIM>@mail.unej.ac.id`. Akun lama otomatis terisi, dan NIM tidak bisa diketik atau diubah selain lewat emailnya. Ada indeks unik.
- `api.Member.NIM` dan `api.User.NIM`:
  - terlihat oleh semua anggota divisi dan admin (`members.read`);
  - divisi lain mendapat 403;
  - tidak ada di halaman publik, pesan grup, atau log.
- **Tabel**: di desktop NIM tampil sebagai baris kecil di bawah nama, di HP di bawah nama pada kartu. Pencarian juga menerima NIM.

## 3. Tautkan WhatsApp sekali ketuk
- **Alur**: Space → Bot WhatsApp → "WhatsApp-mu" → **Tautkan WhatsApp** → **Buka WhatsApp dan kirim**. Chat dengan bot terbuka lewat `wa.me/<bot>?text=SPACE-…`, lalu anggota mengetuk Kirim.
  - Ada dua ketukan karena browser memblokir pembukaan tautan yang tidak langsung dari ketukan.
  - Status di Space berubah sendiri tanpa reload: dicek ulang tiap 4 detik selama kode berlaku, dan juga lewat realtime (trigger `wa_identities`). Muncul toast "WhatsApp tertaut."
  - Setelah tertaut, panel menampilkan **"Tertaut · +62 812-••••-4471 · sejak …"** dan tombol **"Bukan nomormu? Lepas tautan"** dengan konfirmasi.
- **Server**:
  - Kode sekali pakai berlaku **15 menit** dan disimpan sebagai hash. `WALinkChallenge.DeepLink` hanya ada bila bot divisi aktif dan nomornya diketahui.
  - Endpoint baru `GET` dan `DELETE …/whatsapp-link` untuk status (nomor tersamar) dan lepas tautan, tercatat di audit `whatsapp.identity.unlink`.
  - Pesan pribadi ke bot **hanya dibaca bila berisi kode** (`SPACE-…` atau `!link SPACE-…`), dibatasi 5 per chat per 15 menit. Pesan pribadi lain tidak disimpan isinya.
  - Bukti nomor adalah pesan yang datang dari akun WhatsApp itu, dan kode mengikat akun Space-nya.
  - Bot membalas **hanya ke pengirim**: "Tertaut. WhatsApp ini sekarang terhubung ke akun Space Aruna Desain (NIM 2324•••••090). Bukan kamu? …". Balasan lewat outbox dengan `group_id` null dan `source_kind='link_dm'`, dan tampil di riwayat kirim sebagai "Pesan pribadi".
  - **Satu nomor satu akun**: nomor yang sudah tertaut ke akun lain ditolak. Kode yang sudah dipakai juga tidak berlaku lagi.
  - Audit `whatsapp.identity.link` mencatat `source: dm|group`.
- **Bridge** (`bridge/src/connections.mjs`):
  - Chat pribadi diteruskan hanya bila berisi kode.
  - Pengirim boleh dibalas selama 1 jam, berupa teks saja: tanpa mention, tanpa media, maksimal 2.000 karakter.
  - Tujuan lain tetap ditolak 403. Sidik jari job grup tidak berubah, jadi job lama aman.
- `!link` di grup tetap bekerja seperti sebelumnya.
- **Tidak dibuat**: QR di desktop (`wa.me` membuka WhatsApp Desktop/Web milik anggota sendiri) dan pengingat bergabung di grup (W2, opsional).

## 4. Migrasi `0028_nim_wa_dm_mail.sql`
Aman untuk data yang ada; hanya menambah atau melonggarkan:
- `users.nim` (generated, unique);
- `invitations.bounced_at` dan `bounce_reason`;
- `wa_outbox.group_id` boleh null, dengan check `group_id is not null or source_kind='link_dm'` (semua baris lama lolos);
- trigger `wa_outbox_target_snapshot` mengisi "Pesan pribadi";
- trigger realtime untuk `wa_identities` dan `invitations`.

Migrasi berjalan otomatis saat API start.

## Test
- **Server (DB)**, `internal/server/nim_wa_ses_test.go`:
  - `TestInviteByNIMAndMemberNIM`: NIM menjadi email dan isi HTML, input salah 400, NIM tampil di `/me` dan daftar anggota, divisi lain 403, penolakan SES 422 tanpa undangan, throttle 429.
  - `TestSESBounceMarksInvitation`: endpoint mati tanpa ARN, konfirmasi langganan, tanda tangan palsu atau topik lain 403, bounce menandai undangan, kirim ulang menghapusnya.
  - `TestWALinkByDirectMessage`: deep link, pesan pribadi biasa dibuang, kode menautkan dan membalas hanya ke pengirim ("Pesan pribadi"), status nomor tersamar, satu nomor tidak bisa ke akun kedua, kode tidak bisa dipakai ulang, lepas tautan.
- **Unit**:
  - `internal/api/nim_test.go`;
  - `internal/mail/mail_test.go` (template, multipart, 5xx versus 4xx, batas per menit);
  - `internal/config/mail_config_test.go`;
  - `internal/appui/invite_nim_test.go` (`inviteTarget`).
- **Bridge**: `bridge/test/persistence.test.mjs` menambah test DM (hanya kode diteruskan; balasan hanya ke pengirim dan dalam jendela waktu; mention ditolak). Di Windows test bridge butuh shim fsync khusus test, karena direktori tidak bisa di-fsync di sana; ini berlaku juga untuk test lama.
- **Screenshot** (native, data dummy): `docs/screenshots/anggota-nim-undang.png` dan `wa-tautkan-menunggu.png`.

## Deploy (ke tc-vps, setelah izin)
1. Merge ke main, set versi (0.3.7), jalankan gofmt/vet/test, push, dan pastikan CI hijau.
2. `E:\himasif-cache\deploy\build-local.sh 0.3.7 <SHA>`. Build kini juga menyertakan sumber bridge.
3. `deploy.sh 0.3.7 --dry-run`, lalu `--stage`, lalu `TOKFILE=… deploy.sh 0.3.7 --go`. Urutannya: backup, API (migrasi 0028 otomatis), web, lalu **bridge** (`r-bridge.sh`). Bridge hanya diganti bila sumbernya berubah dan `package.json` sama; ada cadangan `src.before-0.3.7` dan rollback otomatis bila `/health` gagal.
4. **Email tetap mati** (`SPACE_MAIL_DRIVER=disabled`) sampai owner mengizinkan; lihat `docs/operations/ses-email.md` dan `ses-env.sh`.
