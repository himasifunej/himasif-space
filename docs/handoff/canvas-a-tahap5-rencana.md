# Rencana Tahap 5 canvas A (backend), 8 November 2026

Status: **rencana, menunggu persetujuan lewat GOA**. Belum ada perubahan backend, migrasi, push, atau deploy.

## Ringkasan
Dari empat pekerjaan Tahap 5, hanya **satu yang butuh tabel baru** (urutan tab per akun). Satu lagi butuh indeks tambahan (riwayat ping, opsional). Cocok/Tidak cocok dan WhatsApp per anggota cukup memakai data yang sudah ada.

| # | Pekerjaan | Backend | Migrasi DB |
|---|---|---|---|
| 5a | Cocok / Tidak cocok langsung di tab Tinjauan | tidak ada (endpoint sudah ada) | tidak |
| 5b | Riwayat ping | 1 endpoint GET baru | opsional: 1 indeks (`0024`) |
| 5c | Urutan dan tab tersembunyi per akun | 2 endpoint (GET/PUT) | ya: tabel baru (`0025`) |
| 5d | WhatsApp per anggota di tabel Anggota | tambah field di respons anggota | tidak |

## 5a. Tinjauan: Cocok / Tidak cocok di baris
- Sudah ada: `POST /api/tasks/{taskId}/evidence/{evidenceId}/review` (`client.ReviewTaskEvidence`), dipakai detail tugas ("Bukti sesuai" / "Bukti tidak sesuai", alasan wajib). Tab Tinjauan (`space_tabs.go` `spaceReviewRow`) sekarang hanya "Cocokkan di tugas".
- Rencana: tombol **Cocok** dan **Tidak cocok** di tiap baris "Perlu tinjauan" (hanya bila `task.Permissions.Review`). Tidak cocok membuka isian alasan satu baris di bawah baris itu (wajib); Cocok memakai alasan bawaan "Dicocokkan dari tab Tinjauan". Server menerima alasan 1–2000 karakter (`reviewTaskEvidence`, `internal/server/tasks.go:959`), jadi alasan bawaan sah.
- Setelah berhasil: muat ulang tugas Space (pola `taskFeedback` yang ada). Test UI baru.

## 5b. Riwayat ping
- Sudah tersimpan: tiap ping = `delivery_events` (`source_kind='ping'`, `space_id`, judul, isi, `created_at`) + `delivery_recipients` (penerima, kanal, status, `seen_at`) + audit `ping.created` (pengirim). Yang belum ada hanya **API baca**.
- Endpoint baru: `GET /api/spaces/{spaceId}/pings?before=&limit=50` → daftar {id, pesan, dikirim, pengirim, sumber (tugas/rapat), penerima: [{nama, kanal, status, dilihat}]}. Izin: anggota dengan `ping.send` di Space itu melihat semua ping Space; lainnya hanya ping yang mereka terima. Periode tertutup tetap bisa dibaca (hanya baca).
- Migrasi `0024_ping_history_index.sql` (opsional, aditif): `create index delivery_events_space_ping on delivery_events(space_id, created_at desc) where source_kind='ping';`
- UI: tab Ping menampilkan "Riwayat ping" di bawah komposer (canvas). Test server (Postgres) + test UI.

## 5c. Urutan tab per akun
- Sekarang urutan/tab tersembunyi disimpan per perangkat (`PreferencesStore`).
- Migrasi `0025_user_ui_preferences.sql`: tabel `user_ui_preferences (user_id uuid primary key references users(id), space_tabs jsonb not null default '{}', updated_at timestamptz not null)`. Isi `space_tabs`: `{"order":[…], "hidden":[…]}`, divalidasi terhadap 9 nama tab yang dikenal.
- Endpoint: `GET /api/me/ui-preferences`, `PUT /api/me/ui-preferences` (hanya akun sendiri, batas ukuran, audit `ui.preferences.set`).
- Klien: saat login ambil dari server; kalau server kosong, unggah pilihan perangkat sekali (migrasi lunak); simpanan perangkat tetap jadi cadangan saat offline. Test klien + server.

## 5d. WhatsApp di tabel Anggota
- Sudah tersimpan: `wa_identities (period_id, division_id, user_id, jid, verified_at)` dari alur tautan WhatsApp.
- Rencana: tambah `whatsappLinked bool` (dan `whatsappVerifiedAt` opsional) ke `api.Member` di respons daftar anggota, dihitung dari `wa_identities` periode+divisi itu. **Nomor/JID tidak dikirim** ke klien. Kolom WhatsApp: "Tertaut" / "Belum" (kartu HP: "WA tertaut" / "WA belum", seperti canvas). Tanpa migrasi.

## Di luar Tahap 5 ini (usul ditunda)
- Nomor tugas urut `#T12`: butuh kolom nomor + backfill + sequence per Space; risiko lebih besar, sebaiknya tahap tersendiri.
- Unggah lampiran berkas untuk tugas: penyimpanan berkas sudah ada untuk catatan (`0014`), tapi perlu tabel tautan tugas + kuota; tahap tersendiri.

## Urutan kerja yang diusulkan
1. 5a dan 5d (tanpa migrasi): kode + test di PC ini, commit.
2. 5b dan 5c: kode, migrasi `0024`/`0025`, test server terhadap Postgres (CI `ci: run gofmt, vet and all tests against Postgres`; di PC ini belum ada Postgres, {{Unverified}} apakah perlu dipasang atau cukup CI).
3. Push branch (setelah disetujui), lalu dari aws-vps:
   - Backup sebelum migrasi: `pg_dump -Fc` ke `~/.local/share/himasif-space-pilot/manual-backups/before-0024-<waktu>.dump` (izin 0600), cek dengan `pg_restore -l`; ditambah backup terenkripsi biasa bila prosedur pilot memintanya.
   - Build server, `space-server -env-file … migrate`, restart layanan pilot, cek `/api` sehat.
   - `make web`, salin ke `web.new`, cadangkan `web` ke `web.before-tahap5`, tukar.
4. Rollback: kedua migrasi aditif (indeks + tabel baru), server lama tetap jalan di atasnya; web dikembalikan dari `web.before-tahap5`. Bila perlu: `drop table user_ui_preferences; drop index delivery_events_space_ping;` setelah backup diverifikasi.

## Yang perlu disetujui
- [ ] Lingkup 5a–5d seperti di atas (dan menunda #T serta lampiran).
- [ ] Migrasi `0024` (indeks) dan `0025` (tabel `user_ui_preferences`) ke DB pilot, dengan backup dulu.
- [ ] Push branch dan deploy dari aws-vps setelah test lolos.
