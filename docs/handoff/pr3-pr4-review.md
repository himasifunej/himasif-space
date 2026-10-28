# Review dan integrasi PR #3 / #4

Tanggal: 28 Oktober 2026. Basis tetap `d92a372e51af5e24084d6973bd7dc4313f83af36`. PR #3 mula-mula `23b8bbc`, PR #4 `96553f6`; review dua sumbu memakai kandidat gabungan `931023d` dengan `git diff d92a372...931023d`. Perbaikan reminder ada di `ffc0195`, lalu dimasukkan ke kandidat integrasi.

## Standards

Laporan awal:

- **P2, pelanggaran kontrak:** `listReminders` menambahkan semua reminder milik sendiri lalu semua yang dibagikan, sehingga tidak memenuhi urutan terdekat dahulu/selesai di belakang pada `personal-reminders-api.md`. Diperbaiki dengan satu query `VisibleReminders` yang mengurutkan seluruh objek yang boleh dilihat.
- **P3, penilaian heuristik Duplicated Code:** `collectReminders` menduplikasi pemetaan row `scanReminder`. Keduanya kini memakai decoder yang sama.

Pemeriksaan ulang: **0 temuan tersisa**. Tidak ditemukan pelanggaran berarti pada MyGo native, UUIDv7, audit transaksi, pemisahan migrasi, atau penyimpanan kredensial.

## Spec

Laporan awal:

- **P1, sesi yang dicabut masih dapat menulis:** kontrak akun meminta pemeriksaan ulang akun/sesi; seluruh mutasi reminder sebelumnya hanya mempercayai principal middleware dan kepemilikan. `reminderWrite` kini mengunci akun dan memanggil `lockedAccount` di dalam transaksi sebelum mutasi. Delapan handler diuji setelah logout maupun disable; data dan audit mutasi tidak berubah.
- **P2, deadlock berbagi silang:** kontrak berbagi menjanjikan 201 untuk permintaan valid. Sebelumnya A→B dan B→A dapat saling menunggu lock akun karena FK `shared_by`. Akun pemilik/penerima kini dikunci menurut UUID sebelum baris reminder. Tes PostgreSQL dengan dua permintaan yang benar-benar menunggu lock berubah dari satu 500/satu grant menjadi dua 201/dua grant.
- **P2, urutan gabungan:** kontrak menyatakan “jatuh tempo terdekat dahulu, selesai di belakang”. Query visibilitas tunggal memperbaiki urutan tanpa mengubah field date-only/instant.

Pemeriksaan ulang: **0 temuan tersisa**. Tidak ditemukan scope tambahan atau kebutuhan current slice yang hilang; UI reminder dan pengiriman terjadwal memang belum termasuk.

Standards: 2 temuan awal (tertinggi P2), kini 0. Spec: 3 temuan awal (tertinggi P1), kini 0. Kedua sumbu dilaporkan terpisah; temuan urutan muncul pada keduanya.

## Verifikasi

- Ketiga regresi reminder gagal pada kode awal dan lulus setelah perbaikan. Seluruh 10 tes utama reminder lulus dengan PostgreSQL nyata.
- Kandidat gabungan: `go test -count=1 -json ./...` memakai PostgreSQL 17.11 dan kedua opsi screenshot native. **214 hasil tes/subtes lulus, 0 gagal, 0 tes dilewati.** Package tanpa tes ditandai skip oleh Go, bukan tes database yang dilewati.
- `TestMigrationsIndependentBranchOrder` menguji 0004-sebelum-0003 dan 0003-sebelum-0004: data cabang yang sudah ada serta timestamp ledger tetap sama, hanya migrasi yang hilang diterapkan, seluruh tabel tersedia, dan pemanggilan ulang tidak menerapkan apa pun.
- Konflik pendaftaran rute di `internal/server/server.go` diselesaikan dengan mempertahankan endpoint tugas dan reminder. Tidak ada perubahan pada isi migrasi 0003/0004.
- Vet lintas platform serta build API/Linux/web/Windows dijalankan kembali sebelum merge. Runtime Windows nyata/installer NSIS tetap pemeriksaan terpisah; runtime Linux/Xpra sudah diuji pada irisan UI/tugas.

## Batas hasil

Backend reminder pribadi tersedia dari PR #3; halaman/klien reminder belum disambungkan. Scheduler, pengiriman perangkat/WA, notes tersimpan, tray/Ace dan form admin organisasi lengkap tetap tahap berikutnya. Produksi, DNS, Reo dan spreadsheet tidak diubah. Runtime preview memakai database terpisah.
