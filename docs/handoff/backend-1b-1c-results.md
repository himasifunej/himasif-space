# Hasil backend Sesi 1B–1C

Branch `feat/accounts-spaces-access`, dari `627298f`. Kontrak final: [backend-api-contract.md](backend-api-contract.md); DTO di `internal/api/access.go`.

## Yang dibuat

- Migrasi `0002_accounts_spaces_access.sql` (baru; `0001` tidak diubah): divisi per periode, Space (divisi + Medhum), membership/role/pengelompokan, role kustom, 45, undangan, token email sekali pakai, grant arsip, catatan serah terima.
- `internal/authz`: aturan kemampuan murni (PRD §4.2). Larangan administrasi 45 diterapkan setelah semua role digabung; role kustom hanya operasional; admin tidak mendapat `acting45.manage` maupun kemampuan kerja.
- `internal/store`: query akses dan akun. `internal/server`: semua endpoint kontrak + `inspect` undangan, ganti kata sandi, `admin-email`, daftar/nonaktifkan akun.
- Setiap perubahan dan audit-nya satu transaksi. Penolakan 403 pada request tulis dicatat `authz.denied` (AC-06).
- Penulisan memuat periode `FOR SHARE`; penutupan periode `FOR UPDATE`, sehingga penulisan bersamaan tidak lolos setelah periode ditutup.
- Seed memberi periode 2025/2026 struktur divisi + Space bila belum ada (database Sesi 1A tetap bisa di-seed ulang).
- Config: produksi dengan SMTP wajib From `@himasif.id`.

## Keputusan teknis baru

- Umur: undangan 7 hari dari setiap pengiriman; bukti email undangan, pemulihan dan email admin 1 jam. Resend merotasi token dan mematikan bukti lama.
- Batas: email ke satu alamat 5/15 menit; endpoint auth publik 30/15 menit per IP; verifikasi kata sandi saat ini memakai limiter login.
- Pemulihan menjawab 202 seketika dan mengirim di background, sehingga waktu respons tidak membocorkan keberadaan akun; `Server.Wait()` dipanggil saat shutdown.
- Tautan email memakai fragment (`/#/undangan?token=…`, `/#/pemulihan?token=…`, `/#/organisasi/serah-terima-admin?verification=…`) agar token tidak masuk log server. **UI perlu menyediakan halaman ini.**
- Pengiriman email adalah langkah terakhir transaksi; kegagalan → 503 dan rollback.
- Penetapan kadiv baru menurunkan kadiv lama menjadi member pada periode itu (tidak menghapus keanggotaannya).
- Grant arsip hanya berlaku selama penerima masih anggota aktif scope pemberi.

## Email (SES)

Panduan `~/HIMASIF/himasif-space/SES-SENDER.md` sudah dibaca. Server membaca `SPACE_*`, bukan `SES_*`; tidak ada kode SES khusus karena SES dipakai lewat SMTP biasa. Pemetaan untuk environment produksi (Coolify), nilainya diambil dari `secrets.env` dan tidak di-commit:

| Space | SES |
|---|---|
| `SPACE_MAIL_DRIVER=smtp` | — |
| `SPACE_SMTP_HOST` / `SPACE_SMTP_PORT` | `SES_SMTP_HOST` / `SES_SMTP_PORT` (587) |
| `SPACE_SMTP_TLS=starttls` | — |
| `SPACE_SMTP_USERNAME` / `SPACE_SMTP_PASSWORD` | `SES_SMTP_USER` / `SES_SMTP_PASS` |
| `SPACE_MAIL_FROM="HIMASIF Space <no-reply@himasif.id>"` | `SES_FROM_NAME` + `SES_FROM_ADDRESS` |

Dev tetap Mailpit (`SPACE_SMTP_TLS=none`, port 1025). Kuota SES dipakai bersama seluruh akun (50.000/hari, ≤14/detik). Email Space hanya dipicu tindakan manual satu per satu dan dibatasi 5 email per alamat per 15 menit, jadi belum ada pacing global; perlu ditambahkan bila nanti ada pengiriman massal. Penanganan bounce belum ada.

## Verifikasi yang dijalankan

`make fmt vet` (linux, windows, js/wasm) dan `make test-db` terhadap PostgreSQL dev Coolify (`SPACE_TEST_DATABASE_URL`, database sementara per test): seluruh 22 test server lulus, tidak ada yang skipped. Unit test `authz` dan `config` lulus.

Skenario penerimaan yang diuji otomatis: AC-01, AC-02, AC-03 (bagian akses/direktori; tugas belum ada), AC-06 (API; WA belum ada), AC-07 (termasuk dua penunjukan 45 bersamaan → 201/409), AC-08, AC-10, AC-11 (checklist serah terima, tanpa auto-copy; penerusan tugas menunggu Sesi 2A). Juga pemulihan, serah terima admin (semua sesi berakhir), nonaktifkan akun.

## Belum / batas

- Tidak ada email nyata yang dikirim melalui SES. Alur undangan/pemulihan diuji dengan Mailer penangkap; driver SMTP diuji ke Mailpit lokal (`mail-test`, From `HIMASIF Space <no-reply@himasif.id>` diterima utuh).
- Belum ada migrasi di database dev/produksi selain database test sementara; tidak ada deploy.
- UI (sesi STB, `feat/native-ui`) belum memakai endpoint ini dan perlu halaman undangan/pemulihan/serah terima admin.
- Penerusan tugas tersisa (PER-06/08), reminder privat (AC-09) dan perintah WA (AC-06 via WA) bergantung sesi berikutnya.
- Semua penulisan pada periode closed, termasuk penetapan kadiv dan undangan, ditolak `409 period_closed`.
- Dokumen ini dan kontrak belum dikirim langsung ke sesi UI.
