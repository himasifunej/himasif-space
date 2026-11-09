# Rencana Tahap 5 canvas A (backend), 8 November 2026

> **9 Nov 2026:** langkah deploy di bawah ditulis untuk aws-vps, yang sudah pensiun. Pilot sekarang ada di tc-vps; pakai `docs/handoff/deploy-pilot.md` (build di PC, jangan di server).

Status: **disetujui owner 8 Nov; kode selesai di PC Tiyo** (commit 7b97290, 350e41f, 3f36e61, bf2c085). Deploy dikerjakan sesi aws-vps; lihat "Langkah deploy" di bawah.

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
- Endpoint baru: `GET /api/spaces/{spaceId}/pings?before=<RFC3339>&limit=1..200` (bawaan 50) → daftar {id, pesan, dikirim, pengirim, sumber (tugas/rapat), penerima: [{nama, kanal, status, dilihat}]}. Izin: anggota dengan `ping.send` di Space itu melihat semua ping Space; lainnya hanya ping yang mereka terima. Periode tertutup tetap bisa dibaca (hanya baca).
- Migrasi `0024_ping_history_index.sql` (opsional, aditif): `create index delivery_events_space_ping on delivery_events(space_id, created_at desc) where source_kind='ping';`
- UI: tab Ping menampilkan "Riwayat ping" di bawah komposer (canvas). Test server (Postgres) + test UI.

## 5c. Urutan tab per akun
- Sekarang urutan/tab tersembunyi disimpan per perangkat (`PreferencesStore`).
- Migrasi `0025_user_ui_preferences.sql`: tabel `user_ui_preferences (user_id uuid primary key references users(id), space_tabs jsonb not null default '{}', updated_at timestamptz not null)`. Isi `space_tabs` per Space (tiap Space punya bar sendiri, Medhum punya tab Konten): `{"<spaceId>": {"order":[…], "hidden":[…]}}`. Server memeriksa bentuk saja (ID Space uuid, ≤200 Space, ≤24 kunci, kunci `[a-z][a-z0-9-]*`, tanpa duplikat); kunci tab yang belum dikenal tetap disimpan, sama dengan klien.
- Endpoint: `GET /api/me/ui-preferences`, `PUT /api/me/ui-preferences` (hanya akun sendiri, batas ukuran, audit `ui.preferences.set`). Admin organisasi juga boleh (tidak dibatasi seperti reminder).
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

## Hasil pengerjaan (PC Tiyo, 8 Nov)
| Commit | Isi |
|---|---|
| 7b97290 (+ gofmt 350e41f) | 5a Cocok/Tidak cocok di baris Tinjauan; 5d `whatsappLinked` di daftar anggota |
| 3f36e61 | 5b `GET /api/spaces/{id}/pings`, migrasi `0024`, riwayat di tab Ping |
| bf2c085 | 5c migrasi `0025`, `GET/PUT /api/me/ui-preferences`, sinkron bar tab di klien |

Test lokal (Windows, tanpa Postgres): `go vet` (windows, js/wasm) bersih; `go test ./...` lolos kecuali 3 test NoteDocument lama dan `internal/filestore` `TestImmutableBlobAndLimitCleanup` (cek izin direktori 0700 tidak berlaku di NTFS; paket tidak disentuh, lolos di Linux). Test server baru (`ping_history_test.go`, `ui_preferences_test.go`, tambahan `access_test.go`) dilewati lokal karena tidak ada Postgres; dijalankan CI.

Kompatibilitas: klien baru dengan server lama tetap jalan (riwayat ping "belum tersedia", kolom WhatsApp "—", tab tetap per perangkat). Server baru dengan klien lama: field/endpoint baru diabaikan. Jadi urutan deploy API lalu web aman, dan sebaliknya juga.

## Langkah deploy (sesi aws-vps)
Semua perintah di aws-vps sebagai `enowx`, build lewat lock RAM. Nilai rahasia (URL DB) dibaca dari file env, jangan dicetak. Path yang belum saya lihat langsung ditandai {{Unverified}}: cek dulu dengan `systemctl --user cat himasif-space-pilot-api`.

1. **Ambil kode**
   ```sh
   cd ~/HIMASIF/<checkout branch feat/feedback-01-pilot5-ui>   # {{Unverified}} folder checkout
   git fetch origin && git checkout feat/feedback-01-pilot5-ui && git pull --ff-only
   git log --oneline -1   # harus bf2c085 atau sesudahnya (commit dokumen ini)
   ```
2. **Test + build** (lock RAM)
   ```sh
   L="flock /tmp/space-build.lock env GOMAXPROCS=1 GOFLAGS=-p=1 GOMEMLIMIT=256MiB GOGC=50"
   $L go test ./internal/appui ./internal/workspaceui ./internal/server ./internal/client   # appui: hanya 3 NoteDocument boleh gagal
   $L go build -o /tmp/space-server-tahap5 ./cmd/space-server
   ```
   Opsional: naikkan `app/mygo.json` ke `0.3.0-pilot.14` agar footer web berbeda dari pilot.13.
3. **Backup DB sebelum migrasi** (format custom, izin 0600, lalu verifikasi daftar isi)
   ```sh
   B=~/.local/share/himasif-space-pilot/manual-backups
   ENVF=~/.config/himasif-space-pilot/server.env                          # {{Unverified}} nama file env API
   F=$B/before-0024-$(date -u +%Y%m%dT%H%M%SZ).dump
   ( set -a; . "$ENVF"; set +a; umask 077; pg_dump -Fc -d "$SPACE_DATABASE_URL" -f "$F" )
   pg_restore -l "$F" | grep -c 'TABLE DATA'   # harus > 0
   pg_restore -l "$F" | grep -E 'delivery_events|users' | head
   ls -l "$F"                                   # -rw------- 
   ```
4. **Pasang binary + migrasi**
   ```sh
   systemctl --user cat himasif-space-pilot-api | grep -E 'ExecStart|EnvironmentFile'   # catat path binary lama
   cp <binary lama> <binary lama>.before-tahap5
   cp ~/.config/systemd/user/himasif-space-pilot-api.service ~/.config/himasif-space-pilot/api-unit-before-tahap5.service
   install -m 0755 /tmp/space-server-tahap5 <path binary di ExecStart>
   <path binary> -env-file "$ENVF" migrate      # log: migrations applied [0024_ping_history_index 0025_user_ui_preferences]
   systemctl --user restart himasif-space-pilot-api
   ```
   (`serve` juga menjalankan migrasi saat start; `migrate` terpisah supaya hasilnya terlihat sebelum restart.)
5. **Cek sehat API**
   ```sh
   systemctl --user is-active himasif-space-pilot-api
   curl -fsS https://db.tiyoouw.app/api/health
   journalctl --user -u himasif-space-pilot-api -n 30 --no-pager | grep -iE 'error|panic' || true
   ( set -a; . "$ENVF"; set +a; psql "$SPACE_DATABASE_URL" -Atc "select name from schema_migrations where name like '002[45]%'; select to_regclass('user_ui_preferences');" )
   ```
   Dengan akun demo (token dari login): `GET /api/spaces/<id Mediatek>/pings` → 200, `GET /api/me/ui-preferences` → 200 `{"spaceTabs":{}}`, `GET /api/periods/<p>/divisions/<d>/members` → tiap anggota punya `whatsappLinked`.
6. **Web**
   ```sh
   $L make web
   R=/home/enowx/.local/opt/himasif-space-pilot/releases/0.3.0-pilot.13-212f6eaa1793
   rm -rf $R/web.new && cp -a app/build/web $R/web.new
   mv $R/web $R/web.before-tahap5 && mv $R/web.new $R/web
   ```
   Cek: buka https://space.tiyoouw.app/ (demo@mediatek), tab Tinjauan (tombol Cocok/Tidak cocok), tab Ping (Riwayat ping), Anggota (kolom WhatsApp Tertaut/Belum), geser tab lalu login di perangkat/browser lain → urutan sama.
7. **Rollback**
   - Web: `mv $R/web $R/web.tahap5-gagal && mv $R/web.before-tahap5 $R/web`.
   - API: kembalikan `<binary>.before-tahap5` ke path ExecStart (dan unit dari `api-unit-before-tahap5.service` bila diubah), `systemctl --user daemon-reload && systemctl --user restart himasif-space-pilot-api`. Server pilot.13 berjalan di atas skema baru: `0024` hanya indeks, `0025` tabel yang tidak dibacanya; tidak perlu membatalkan migrasi.
   - Hanya bila skema harus bersih (setelah backup di langkah 3 terverifikasi): `drop table user_ui_preferences; drop index delivery_events_space_ping; delete from schema_migrations where name in ('0024_ping_history_index','0025_user_ui_preferences');`
   - Pemulihan DB penuh dari dump: `pg_restore --clean --if-exists -d ... $F` hanya dengan persetujuan owner (menghapus data sesudah backup).
