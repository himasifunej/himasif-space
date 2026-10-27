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

## Verifikasi yang dijalankan

`make fmt vet` (linux, windows, js/wasm) dan `make test-db` terhadap PostgreSQL dev Coolify (`SPACE_TEST_DATABASE_URL`, database sementara per test): seluruh 22 test server lulus, tidak ada yang skipped. Unit test `authz` dan `config` lulus.

Skenario penerimaan yang diuji otomatis: AC-01, AC-02, AC-03 (bagian akses/direktori; tugas belum ada), AC-06 (API; WA belum ada), AC-07 (termasuk dua penunjukan 45 bersamaan → 201/409), AC-08, AC-10, AC-11 (checklist serah terima, tanpa auto-copy; penerusan tugas menunggu Sesi 2A). Juga pemulihan, serah terima admin (semua sesi berakhir), nonaktifkan akun.

## Belum / batas

- Tidak ada email nyata yang dikirim; test memakai Mailer penangkap. Mailpit/SES belum diuji dengan alur ini.
- Belum ada migrasi di database dev/produksi selain database test sementara; tidak ada deploy.
- UI (sesi STB, `feat/native-ui`) belum memakai endpoint ini dan perlu halaman undangan/pemulihan/serah terima admin.
- Penerusan tugas tersisa (PER-06/08), reminder privat (AC-09) dan perintah WA (AC-06 via WA) bergantung sesi berikutnya.
- Semua penulisan pada periode closed, termasuk penetapan kadiv dan undangan, ditolak `409 period_closed`.
- Dokumen ini dan kontrak belum dikirim langsung ke sesi UI.
