# himasif-space

**Space — Ruang Kerja HIMASIF.** Aplikasi kerja pengurus HIMASIF: Space per divisi, jobdesk, reminder, rapat, Medhum dan bot WhatsApp. Kebutuhan produk ada di [docs/PRD.md](docs/PRD.md); keputusan teknis di [docs/adr](docs/adr).

Status worktree 29 Oktober 2026: fondasi/akun/akses/tugas/Notes telah digabung sampai PR #8. Perubahan lokal berikutnya memuat rapat/scheduler, desktop/Ace/offline, kalender/konektor Medhum, bridge/command/parser WA, UI dan alur SOP, lampiran biner/pending, backup/restore/karantina, CSV darurat/rekonsiliasi, bahasa bot dan pemantauan operasional. Kandidat aplikasi sekarang `0.3.0-pilot.1`. **Verifikasi akhir sedang berlangsung pada database terisolasi; belum diaktifkan pada produksi.**

Instruksi aktif: selesaikan tahap nonmobile, build dan test di akhir, lalu sediakan paket unduhan. Android/iOS ditunda. Builder distribusi/checksum dan pembanding legacy read-only tersedia pada source. Bukti pemeriksaan dan artefak setiap kandidat dicatat dalam `verification.json`; kode/prosedur bukan bukti cutover produksi selesai. Rincian kebutuhan, pekerjaan tersisa dan batas pembuktian ada di [worklog](docs/handoff/remaining-stages-worklog.md).

Lihat [rencana implementasi](docs/implementation-plan.md), [backup/restore](docs/operations/recovery.md), [CSV darurat](docs/operations/emergency-spreadsheet.md), [distribusi](docs/operations/distribution.md) dan [persiapan pilot/cutover](docs/operations/pilot-cutover.md). Aktivasi restore/cutoff dijelaskan di [panduan aktivasi](docs/operations/restore-activation.md).

## Struktur

| Path | Isi |
|---|---|
| `cmd/space-server` | Server API (Go): `version`, `serve`, `migrate`, `seed`, `quarantine-restore`, `activate-restore`, `mail-test` |
| `app/` | Aplikasi MyGo native UI: desktop Windows, dan web (WebAssembly) |
| `internal/api` | Pesan JSON antara server dan klien |
| `internal/server` | Handler HTTP, autentikasi, middleware, penyajian web |
| `internal/store` | PostgreSQL: migrasi (`migrations/*.sql`) dan query |
| `internal/appui` | Tampilan dan state aplikasi (bisa dites tanpa jendela) |
| `internal/client` | Klien HTTP API untuk aplikasi |
| `internal/auth`, `mail`, `config`, `seed`, `id`, `timeutil` | Kata sandi/token, email, konfigurasi, data awal, ID, aturan waktu |

## Menjalankan untuk pengembangan

Prasyarat: Go 1.27+, PostgreSQL, dan (opsional) [Mailpit](https://mailpit.axllent.org) untuk menangkap email.

```bash
cp .env.example ~/.config/himasif-space/dev.env   # isi nilainya, chmod 600
make seed        # migrasi + periode 2025/2026, divisi, akun admin organisasi
make web         # build aplikasi web ke app/build/web
make serve       # API + web di http://localhost:8787
```

Aplikasi desktop: `go tool mygo dev ./app` (butuh layar), atau build Windows dengan `make desktop-windows`. Desktop memakai `SPACE_API_URL` (default `https://app.himasif.id`).

## Pengujian

```bash
make check       # gofmt, vet (linux/windows/wasm), unit test
make test-db     # termasuk test server terhadap PostgreSQL (SPACE_TEST_DATABASE_URL)
```

Secret (database, SMTP, bot) tidak boleh masuk repository; simpan di file env di luar repo.
