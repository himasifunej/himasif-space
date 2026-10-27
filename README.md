# himasif-space

**Space — Ruang Kerja HIMASIF.** Aplikasi kerja pengurus HIMASIF: Space per divisi, jobdesk, reminder, rapat, Medhum dan bot WhatsApp. Kebutuhan produk ada di [docs/PRD.md](docs/PRD.md); keputusan teknis di [docs/adr](docs/adr).

Status: Tahap 1 (fondasi), Sesi 1A — kerangka server, login, dan shell navigasi. Lihat [rencana implementasi](docs/implementation-plan.md).

## Struktur

| Path | Isi |
|---|---|
| `cmd/space-server` | Server API (Go): `serve`, `migrate`, `seed`, `mail-test` |
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
