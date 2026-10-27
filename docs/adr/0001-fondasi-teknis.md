# ADR 0001 — Fondasi teknis Space (Sesi 1A)

Tanggal: 27 Oktober 2026, WIB. Status: **berlaku**. Menyelesaikan sebagian `OPEN-02` dan `OPEN-10` PRD. Keputusan produk di [PRD](../PRD.md) dan [Register keputusan](../decision-register.md) tidak diubah.

## Keputusan

| Area | Keputusan | Asal |
|---|---|---|
| Backend | Go 1.27, satu proses `cmd/space-server`, router `net/http` standar | ARC-01 (disepakati) |
| Klien desktop + web | **Satu aplikasi MyGo native UI** (`package ui`, digambar GPU) di `app/`. Desktop Windows dikompilasi native; web `app.himasif.id` adalah aplikasi yang sama dikompilasi ke WebAssembly dan disajikan oleh server | Pengguna memilih MyGo native (27 Okt) |
| Database | PostgreSQL 17, dijalankan di Coolify VPS (project `himasif-space`, environment `development`, tidak publik) | Pengguna: pakai Coolify |
| Akses data | `pgx/v5` langsung, SQL tertulis tangan; migrasi SQL ter-embed dengan runner sendiri + advisory lock | Turunan teknis |
| Identitas | UUIDv7 dibuat aplikasi, tidak pernah dari judul/baris/deadline | SYN-06 |
| Waktu | `timestamptz` (UTC) untuk instan; `date` + tipe `timeutil.Date` untuk tanggal tanpa jam; zona ruang default `Asia/Jakarta` | MED-04/05, NFR-10 |
| Auth | Email + kata sandi; Argon2id (19 MiB, t=2, p=1); token sesi bearer 32 byte acak, server hanya menyimpan SHA-256-nya; TTL default 30 hari; batas 10 percobaan login / 15 menit per email dan per IP | Pengguna: email+pw dulu |
| Penyimpanan token klien | Desktop: credential store OS (`mygo.Secrets`, Windows Credential Manager); web: `sessionStorage` tab | NFR-09 |
| Email | Antarmuka `mail.Mailer` dengan driver SMTP generik; dev = Mailpit (SMTP lokal). Produksi: SMTP Amazon SES (`himasif.id` terverifikasi, kredensial di `secrets.env` di luar repo; terbukti mengirim ke simulator SES pada 27 Okt) atau Zoho Mail bila diputuskan — cukup ganti env | Pengguna: Zoho/SES, tunda kredensial |
| Umur undangan | 7 hari, bisa dikirim ulang (diimplementasikan Sesi 1B) | Pengguna (OPEN-10) |
| Audit | Tabel `audit_events` append-only (trigger menolak update/delete), mencatat aktor, sumber kanal, scope, target, nilai sebelum/sesudah | ACC-11, ACC-13, NFR-05 |
| Admin organisasi | Satu akun `kind = organization_admin`, dijaga indeks unik parsial | ACC-12 |

## Alasan dan konsekuensi

**MyGo native UI untuk desktop dan web.** Satu basis kode Go untuk dua klien; desktop tidak butuh WebView2, terbuka cepat dan hemat memori, sejalan dengan prioritas hemat baterai (NFR-01). Konsekuensi yang diterima: unduhan awal web ±18 MB WASM (±5 MB gzip, disajikan dari salinan `.gz`); konten kanvas belum terbaca screen reader di browser (versi desktop terbaca); tray/notifikasi tidak tersedia di browser. Web memang jalur alternatif/darurat (OPS-01), mobile utama tetap Flutter (ARC-03).

**Token bearer, bukan cookie.** Desktop dan web memakai mekanisme yang sama; tidak perlu CSRF karena tidak ada kredensial otomatis yang dikirim browser. Web memakai `sessionStorage` sehingga sesi hilang saat tab ditutup.

**Server satu-satunya penentu wewenang** (ACC-10, ARC-05): klien hanya menyembunyikan menu sebagai kenyamanan. Role, Space dan 45 diimplementasikan di Sesi 1C dengan pemeriksaan di server.

## Belum diputuskan / batas

- Domain/DNS produksi, kredensial Zoho, dan deploy server ke Coolify (`OPEN-03`) — dikerjakan saat dibutuhkan; `app.himasif.id` saat ini masih menunjuk ke IP lain.
- Postgres dev dijangkau lewat IP kontainer di jaringan Docker Coolify (`10.0.1.x`), yang bisa berubah saat kontainer dibuat ulang. Server produksi akan berjalan di jaringan yang sama dan memakai nama host internal.
- Backup basis data (`OPEN-11`) wajib sebelum ada data nyata, bukan di Sesi 1A.
- Logo, Ace, palet dan tipografi (`OPEN-01`) belum ada; ikon aplikasi adalah placeholder.
- Tray, toast Windows, autostart, suara dan Ace: Tahap 3.
