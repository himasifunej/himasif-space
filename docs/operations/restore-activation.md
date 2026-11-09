# Aktivasi database hasil restore

Perintah ini disiapkan untuk operator, **belum pernah dipakai pada produksi**. Restore drill tetap `isolated_restore`; tidak perlu diaktifkan hanya untuk membuktikan backup. Aktivasi bukan pairing WA, persetujuan SOP, atau izin menulis Sheets.

Setelah review hasil drill, mapping sumber, cutoff, tanggung jawab dan rollback disetujui, hentikan seluruh instance Space pada database target. Pengirim Reo harus sudah dihentikan pada waktu cutover yang disepakati sebelum pengirim Space diaktifkan. Gunakan binary yang cocok dengan skema terbaru. Persiapkan migrasi dalam mode terisolasi sebelum preview; command preview tidak menjalankan migrasi. Simpan backup dan bukti review sebelumnya.

Dengan konfigurasi `SPACE_RUN_MODE=maintenance`, koneksi database target `space_restore_...` / `space_shadow_...`, dan referensi restore ID dari laporan karantina:

```sh
space-server activate-restore \
  -database-name space_restore_REPLACE \
  -restore-id UUID_DARI_KARANTINA \
  -operator NAMA_OPERATOR \
  -reason REFERENSI_REVIEW_CUTOVER \
  -cutoff 2026-10-29T08:00:00Z
```

Tanggal contoh harus diganti waktu nyata **dalam 15 menit terakhir**, tidak sebelum waktu karantina dan tidak di masa depan. Tanpa `-apply-plan`, hasil JSON merupakan preview dengan identitas database/restore, cutoff, jumlah baris terdampak dan SHA256. Review dan simpan hasilnya. Menjalankan ulang argumen identik dengan tambahan `-apply-plan SHA256_DARI_PREVIEW` menerapkan rencana hanya bila isinya masih sama. Perubahan jumlah/scope atau cutoff kedaluwarsa memerlukan preview baru.

Dalam satu transaksi dengan lease eksklusif (menolak proses server yang masih aktif), aktivasi:

- Menahan seluruh event delivery dan outbox hasil restore melalui `suppressed_at`, tanpa mengubah receipt accepted/delivered/read/unknown menjadi keberhasilan atau pembatalan palsu. Riwayat tetap tersedia untuk audit. Job ambigu tidak dikirim ulang.
- Mengabaikan input pending lama, mencabut sesi login dan challenge tautan lama.
- Menonaktifkan konektor, bot dan kebijakan SOP serta menaikkan versi konfigurasi. Temuan ditandai perlu diperiksa ulang; keputusan manusia dan bukti tidak dihapus.
- Mencatat cutoff dan UUID generasi aktivasi. Scheduler hanya membuat slot pada/sesudah cutoff, dengan identitas event generasi baru, sehingga event masa depan yang pernah masuk backup tidak memblokir jadwal baru.
- Menghapus karantina database persisten dan mencatat audit berisi rencana/operator. Jika salah satu langkah gagal, seluruh transaksi batal.

Sesudah berhasil, hentikan CLI dan mulai **proses baru** mode live. Koneksi yang dibuka sebelum reset GUC tidak dipakai ulang. Akun masuk kembali. Device hanya menerima event generasi baru; slot lama tidak dikejar. Periksa konfigurasi/scope grup, izin sumber, token dan persetujuan SOP menggunakan akun berwenang sebelum mengaktifkan setiap integrasi lagi. Pairing/secrets/state bridge tidak dibuat atau dihapus oleh command ini. Polling menyimpan input bridge historis untuk audit tetapi menolak eksekusi bila waktu penerimaan/kejadian sumber sebelum cutoff atau waktu kejadian tidak tersedia. Bukti dengan waktu tidak memadai tetap ditinjau manusia, bukan direkayasa waktunya.

Outbox baru untuk slot sebelum cutoff ditahan oleh trigger database, termasuk follow-up dan rekap. Tidak ada auto-resend atau auto-approval pada proses aktivasi. Lihat Status layanan untuk antrean aktif versus baris yang ditahan. Untuk rollback, hentikan Space sebelum mempertimbangkan pengirim lama; jangan menghapus cutoff/suppression atau memakai ulang backup untuk memaksa pesan historis terkirim.
