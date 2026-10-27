# Review PR #2 — backend Sesi 1B–1C

Acuan: base `627298f334624cd5edf1d47bc13e4113815d3cf2`, head Kiro `f96a0eb6d5ed58fef27f78dbd5cc7534d0abb398`. Diff: `git diff 627298f...f96a0eb`. Review memisahkan standar dari spesifikasi. Perbaikan ditambahkan pada branch PR sebelum merge.

## Standards

- **P1, kebijakan admin berubah saat dibaca.** `scope.go` memfilter capabilities shared Space dengan `slices.DeleteFunc` pada slice yang dapat menunjuk ke `authz.AdminCaps`. GET Space/Medhum oleh admin menghapus kemampuan administrasi dari kebijakan global sampai proses restart dan menimbulkan shared mutation antarreques. Ini melanggar sifat kebijakan murni `internal/authz` serta evaluasi kewenangan per request di `internal/server` (PRD ACC-10). **Diperbaiki:** salin slice sebelum pemfilteran; tes membandingkan kebijakan sebelum/sesudah beberapa lookup active/closed/shared/division.
- **P2, mutasi izin tidak diserialkan dengan pemeriksaannya.** Endpoint member/role/45 membaca scope dalam transaksi tetapi hanya memegang period `FOR SHARE`; dua penulisan dapat sama-sama lolos berdasarkan state sebelum salah satunya berubah. Dokumentasi `write`/`loadScope` menjanjikan wewenang yang dimuat pada transaksi penulisan. **Diperbaiki:** perubahan membership, custom role dan 45 memakai period `FOR UPDATE`, konsisten dengan penggantian kadiv. Tes memakai advisory gate dan observasi lock PostgreSQL untuk mengatur interleaving secara deterministik.

Tidak ada temuan smell heuristik tersisa yang perlu menghalangi merge.

## Spec

- **P2, anggota yang sudah dikeluarkan masih dapat memperoleh 45.** Kontrak API mensyaratkan target 45 sebagai anggota aktif dan melarang penghapusan anggota yang masih 45. PRD ACC-08 mempertahankan 45 sampai kadiv asli mencopotnya. `removeMember` dan `appoint45` semula dapat membaca kondisi lama secara bersamaan, lalu keduanya berhasil. Appointment menjadi yatim dan dapat melekat lagi ketika user yang sama diundang ulang. **Diperbaiki:** serialisasi period di atas. Regresi membuktikan DELETE selesai 204 dan PUT 45 berikutnya ditolak 400, tanpa appointment aktif tersisa.

Tidak ditemukan pelanggaran independen pada alur sekuensial undangan, pembatasan lintas divisi, arsip, periode closed, atau larangan administrasi untuk 45. Penerusan tugas, reminder privat, WA dan halaman klien baru tetap dependensi sesi berikutnya yang sudah dinyatakan.

## Pemeriksaan keamanan lanjutan

Pengujian tambahan menemukan dan memperbaiki empat masalah lain:

1. Login yang telah memverifikasi hash lama dapat menciptakan sesi sesudah pemulihan/email handover/disable. Pembuatan sesi kini mengunci dan memeriksa ulang akun dalam transaksi, sehingga berurutan dengan rotasi/revokasi.
2. Request yang membawa principal sebelum handover dapat menimpa password penerus atau mengirim bukti email setelah sesinya dicabut. Aksi kredensial serta penerimaan undangan akun lama kini memeriksa ulang sesi dan akun terkunci; bukti kata sandi diperiksa ulang jika hash berubah.
3. Dua reset memakai proof berbeda dapat saling deadlock (satu 204, satu 500) karena mengambil lock token sebelum akun. Semua penulis recovery kini mengambil akun sebelum token; dua reset menghasilkan satu 204 dan satu 410.
4. Decoder menerima delimiter JSON penutup tambahan. Pemeriksaan EOF menggantikan `Decoder.More`; login dan endpoint baru memakai decoder ketat yang sama.

Regresi dijalankan terhadap source asli melalui Go overlay dan gagal dengan perilaku di atas, lalu lulus pada patch. Tidak ada perubahan migrasi untuk perbaikan review ini.

## Verifikasi

- PostgreSQL 17.11 terisolasi pada STB, dijalankan sebagai enowx lewat Unix socket privat pada HDD. Tidak menyentuh database Coolify/dev/produksi atau mengirim email sungguhan.
- `go test -json -count=1 -timeout=180s ./...` dengan `SPACE_TEST_DATABASE_URL` aktif: **79 hasil test pass, 0 fail, 0 skip; 28 test server tingkat atas pass**. Log lokal: `/mnt/stb/cache/space-pr2-test-results.jsonl`.
- Format, vet Linux/Windows/WASM dan build server diperiksa sebagai perintah Go langsung (setara target Makefile; `make` tidak terpasang pada STB).

Ringkasan: Standards 2 temuan diperbaiki (terberat P1 mutasi kebijakan admin); Spec 1 temuan diperbaiki (P2 lifecycle 45); empat temuan lanjutan diperbaiki; tidak ada blocker tersisa setelah verifikasi.
