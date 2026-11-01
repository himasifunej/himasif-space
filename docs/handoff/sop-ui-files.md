# Evaluasi SOP, ralat lintas divisi dan lampiran biner

Checkpoint 29 Oktober 2026. Instruksi pengguna terbaru: lanjutkan desktop/web serta operasional/pilot, **tanpa mobile**, lalu build/test di akhir dan sediakan paket unduhan. Semua uraian berikut adalah sumber lokal, **belum build/test/migrasi/runtime**. Belum ada binary/paket rilis, pairing, kiriman WA, fetch sumber nyata atau perubahan produksi.

## Evaluasi

UI Evaluasi (`internal/appui/sop_{view,forms}.go`) kini mencakup scope, daftar/detail/riwayat temuan, filter/cursor, tinjauan/keputusan, konteks/bukti manual, kategori pengecualian mendesak, konfigurasi kebijakan, persetujuan bot, grant evaluator serta laporan quarter/rentang. Respons daftar berupa ringkasan; bukti lengkap diambil pada detail. Pengesahan memeriksa fingerprint sumber terkini, bukan hanya tick worker terakhir. Bukti manual harus dikonfirmasi eksplisit.

`GET .../sop/deliveries[/{runId}]` menyediakan riwayat paginated follow-up/rekap dan snapshot saat antrean dibuat. Hanya memakai izin SOP divisi; tidak membuka kontrol sesi, QR, kredensial, payload pesan/nomor WA atau antrean command umum. Status accepted/delivered/read dibedakan. `GET .../sop/schedule-readiness[?contentId=...]` menjelaskan konfigurasi/prasyarat yang kurang; bukan bukti pesan telah dikirim. UI Pengiriman menghubungkan keduanya.

Ralat lintas divisi memakai `GET .../findings/{findingId}/transfer-preview?divisionId=...&rule=...` dan `POST .../transfer`. Pengguna harus memiliki izin review pada kedua divisi di periode yang terbuka. Preview menampilkan kebijakan, orang dan temuan tujuan bila sudah ada. UI meminta konfirmasi pembatalan sumber dan pembukaan tinjauan tujuan. Versi temuan sumber/tujuan, observasi sumber dan kebijakan tujuan diperiksa ulang dalam transaksi. Bukti asli dipertahankan, atribusi sumber dibatalkan dan target menjadi needs_review dengan measurement insufficient; tidak otomatis disahkan. Riwayat keputusan sebelumnya tetap tersimpan dan tautan ralat append-only pada migrasi `0013_sop_transfers.sql`. Kasus tujuan memerlukan konteks/bukti yang sesuai sebelum pengesahan. Semua ralat diaudit pada kedua scope.

CSV diambil dengan autentikasi pada rentang laporan, tanpa fallback cache. Pengambilan dan penyimpanan adalah dua tindakan eksplisit. Native memakai dialog simpan, web Blob/anchor unduhan. Ukuran respons dibatasi dan respons parsial tidak disimpan sebagai ekspor sukses. CSV membaca keadaan terbaru, sehingga UI menjelaskan kemungkinan perbedaan dari ringkasan lama. Tautan bukti dapat dibuka melalui URL service MyGo dengan pemeriksaan skema aman.

## Lampiran biner

Migrasi `0014_attachment_files.sql` menambah storage key/checksum dan receipt unggah yang bertahan setelah penghapusan metadata. `PUT /api/notes/{noteId}/attachments/{attachmentId}/file?name=...` menerima berkas mentah, dengan UUID unggah stabil untuk retry. Izin manage/session/periode diperiksa sebelum menerima data dan sesudah file disimpan, tanpa memegang transaksi periode selama streaming. Upload tidak memerlukan lease editor isi, sesuai pengelolaan lampiran yang sudah ada. Receipt yang sama hanya diterima untuk akun, note, isi dan nama yang sama. Receipt yang sudah dihapus tidak menghidupkan lampiran kembali.

Berkas immutable berada pada `SPACE_ATTACHMENT_DIR` (default `space-data/attachments`, mode 0700, di luar `SPACE_WEB_DIR`); file 0600 memakai key acak, bukan nama pengguna. Batas `SPACE_ATTACHMENT_MAX_BYTES` default 20 MiB, dapat diatur 1 byte sampai 100 MiB. Ukuran sebenarnya dihitung sambil streaming, MIME dideteksi server, SHA-256 disimpan. Temp file disinkronkan dan rename dipersistenkan sebelum metadata DB ditulis. Error commit ambigu tidak menghapus blob yang mungkin sudah direferensikan. Orphan setelah crash/penolakan harus dikumpulkan lewat prosedur retensi eksplisit; tidak ada GC otomatis yang berisiko pada backup.

`GET .../attachments/{attachmentId}/file` memeriksa akses note terkini dan hanya membaca blob yang masih memiliki metadata lampiran pada note tersebut. Unduhan berupa attachment dengan nosniff/no-store; tidak ada URL publik. Klien memakai bearer dan memeriksa ukuran serta checksum sebelum menawarkan Simpan. Penghapusan lampiran menghapus akses metadata dan menandai receipt removed; blob sengaja ditahan untuk backup/retensi. Penghapusan note soft-delete juga menutup akses file.

Klien desktop/browser memiliki pemilih berkas, tombol unggah/retry stabil dan unduh/simpan. Lampiran eksternal/metadata pending tetap didukung dan dibedakan dari berkas milik Space (`stored`). Checkpoint berikutnya sudah menambahkan penyimpanan pending privat lintas restart/halaman: native file atomik dan browser IndexedDB, namespace server/akun/note, checksum dan ID unggah stabil, batas 32 file/256 MiB per akun/server. UI memulihkan setelah note dapat dibaca, mengharuskan simpan perangkat sebelum upload dan tidak menghapus salinan sebelum receipt cloud cocok. Kegagalan/kuota ditampilkan. Implementasi pemulihan ini **belum diuji runtime/restart**, sehingga belum ada klaim acceptance offline lulus.

## Pembagian kerja AWS

Snapshot source tanpa .env/secrets dibuat di `/home/enowx/HIMASIF/space-ops-delegation` pada aws-vps melalui Tailscale. Sesi OpenCode2 `b07045e32d60` ditugasi file baru `scripts/ops/`, `docs/operations/`, `docs/handoff/ops-delegated.md`, opsional `cmd/space-ops/`; tanpa build/test/live changes. Agent berhenti sesudah membaca sumber: kuota model gratis habis dan alternatif Copilot menolak PAT (HTTP 400). Pengguna kemudian mengonfirmasi OpenCode2 berhenti dan meminta agen utama melanjutkan sendiri. Tidak ada hasil implementasi delegasi yang diasumsikan selesai; agent AWS tidak menjadi dependensi. Repo utama lama AWS tidak disentuh.

Kontrak untuk backup: direktori lampiran private immutable harus masuk bundle, key/state WA dipisahkan dari secrets yang membukanya, metadata DB dan blob harus konsisten (write freeze atau strategi immutable snapshot yang jelas). Receipt unggah dan audit termasuk dump. Saat restore isolated, outbound scheduler/WA/source/mail perlu dinonaktifkan dengan guard runtime yang benar-benar diimplementasikan; flag baru tidak boleh dianggap bekerja hanya karena ditulis pada env contoh.

## Pekerjaan berikutnya

- Selesaikan operasional 5A/5B dan integrasikan hasil agent jika tersedia, termasuk guard restore, spreadsheet darurat/reconcile dan rencana cutover/rollback.
- Lengkapi bahasa respons inti WA dan parser contoh nyata/acceptance 4B–4C.
- Tinjau kebijakan pending berkas dan integrasikan pemulihan perangkat yang diperlukan.
- Perbarui distribusi desktop/web dan buat paket unduhan beserta panduan/checksum setelah seluruh implementasi nonmobile selesai.
- Jalankan build/test/migrasi terisolasi dan pemeriksaan runtime/akses/integrasi lengkap pada fase akhir, lalu perbaiki seluruh temuan. Tidak ada klaim acceptance lulus di checkpoint ini.

## Kelanjutan operasional

Mode runtime/marker DB/lease (`0015_runtime_control.sql`) dan tooling backup/verify/restore/retensi telah ditulis setelah checkpoint UI. Lihat [prosedur pemulihan](../operations/recovery.md), policy dan contoh service/timer pada direktori yang sama. Semuanya belum dipasang/dijalankan/diuji. Spreadsheet darurat/rekonsiliasi, bahasa respons inti bot, sisa distribusi/pilot, paket unduhan dan verifikasi akhir masih harus dikerjakan.
