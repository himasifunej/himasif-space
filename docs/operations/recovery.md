# Backup, restore dan pemulihan Space

Status implementasi 29 Oktober 2026: alat dan guard runtime telah ditulis, **belum dijalankan atau diuji**. Tidak ada jadwal/service aktif yang dibuat pada host mana pun. Panduan ini merupakan prosedur yang harus diuji pada fase verifikasi akhir, bukan bukti pemulihan berhasil. Mobile dikeluarkan dari scope aktif; distribusi desktop/web dan paket unduhan masih menunggu fase akhir.

## Penanggung jawab dan target

Isi kebijakan dari `backup-policy.example.json` dengan lokasi nyata, unit API/bridge, release yang terpasang, pengelola, RPO, RTO dan retensi sebelum pilot. `operator`, `rpo_minutes`, `rto_minutes` dan `retention_days` sengaja belum ditetapkan. Satu salinan minimum merupakan batas pengaman alat, bukan kebijakan retensi yang disepakati. Interval timer harus diturunkan dari RPO dan durasi penghentian layanan yang diukur. Contoh timer pukul 02.00 bukan jadwal organisasi yang sudah disetujui.

`backup_dir` harus menuju lokasi backup terpisah yang dikelola operator (misalnya mount off-host), bukan hanya direktori lain pada disk server yang sama. Alat menolak direktori sumber/staging/tujuan yang saling bertumpuk, tetapi **tidak membuktikan** bahwa mount benar-benar berada pada mesin/lokasi lain. Verifikasi mount, kapasitas, akses dan pemulihan ketika sumber utama tidak tersedia. Buat marker `.space-backup-volume-id` pada tujuan yang benar dan isikan nilainya pada `backup_volume_id` kebijakan. Backup/retensi memerlukan marker yang cocok serta direktori tujuan yang sudah ada; alat tidak membuat fallback tujuan lokal ketika mount hilang. Untuk media lepas atau penyimpanan yang dipasang sesuai kebutuhan, pastikan tujuan terpasang sebelum timer berjalan; jangan membuat direktori lokal pengganti secara otomatis saat mount gagal.

## Cakupan dan secrets

Bundle mencakup dump PostgreSQL custom (termasuk akun, akses, Notes, bukti, temuan/keputusan, receipt dan audit), blob privat lampiran, state WA `.enc` bila dipakai, versi rilis dan konfigurasi nonsecret yang diizinkan. Isi Drive/Trello/Sheets yang hanya direferensikan sebagai tautan **bukan salinan berkas milik Space**. Lampiran pending pada komputer pengguna juga bukan file cloud dan tidak termasuk backup server.

Bundle dienkripsi dengan [age](https://github.com/FiloSottile/age#usage), menggunakan daftar penerima publik. Identity dekripsi, `SPACE_WA_STATE_KEY`, bearer bridge, kredensial DB/SMTP/Google/Trello dan file `.env` tidak dimasukkan. Simpan secrets pada penyimpanan terbatas **terpisah** dan uji ketersediaannya bagi pengganti pengelola. State WA terenkripsi tanpa kunci yang sesuai tidak dapat dipulihkan. Jangan memasukkan identity/kunci ke Notes, chat, repository, manifest atau log.

Simpan checksum ciphertext pada catatan backup tepercaya yang terpisah. Restore mewajibkan checksum tersebut; jangan mengambil nilainya secara otomatis dari file sampingan yang mungkin ikut terganti. Enkripsi ke public key sendiri tidak membuktikan siapa pembuat arsip. Kehilangan kunci/rekaman backup atau tidak adanya salinan off-host harus tercatat sebagai kegagalan kesiapan operasional.

## Snapshot konsisten

Prasyarat Linux: Python 3, `psql`, `pg_dump`, `pg_restore`, `createdb`, `age`, systemd, serta binary `space-server` yang sesuai untuk restore. Versi alat PostgreSQL harus kompatibel dengan server/dump; verifikasi saat build/test akhir. Tidak ada kredensial yang diberikan lewat argumen proses; skrip menerjemahkan URI PostgreSQL dari environment ke variabel libpq. URI keyword/opsi yang tidak dikenali ditolak.

Sediakan direktori backup dan staging privat mode 0700, file kebijakan/konfigurasi mode 0600, serta daftar penerima age di luar repo. Semua instance API dan bridge yang dapat menulis sumber snapshot harus dideklarasikan/dihentikan. Alat:

1. Memeriksa unit systemd dan mencatat unit yang awalnya aktif pada jurnal pemulihan privat.
2. Dengan `--stop-services`, menghentikan API lalu bridge. Tanpa flag ini, unit harus sudah berhenti.
3. Mengambil advisory lock runtime eksklusif PostgreSQL; instance Space versi ini yang masih berjalan membuat backup ditolak.
4. Membuat dump dan salinan file saat penulis berhenti. Menolak symlink/file sumber tidak dikenal; temporary file yang belum dipublikasikan tidak dianggap lampiran.
5. Memeriksa seluruh referensi blob DB, ukuran, SHA-256 dan jumlah record tabel utama.
6. Melepas lock dan menghidupkan kembali hanya unit yang awalnya aktif, kemudian mengenkripsi snapshot. Enkripsi tidak memperpanjang downtime pengambilan data.
7. Menulis manifest internal dan indeks checksum ciphertext tanpa mengklaim restore telah diuji.

Raw SQL/operator atau aplikasi versi lama yang tidak memakai runtime lease tetap harus dihentikan. Lock tidak menggantikan inventaris seluruh penulis. Tidak ada proses yang dihentikan hanya berdasarkan tebakan nama PID.

Contoh pemakaian, setelah operator menyediakan konfigurasi nyata dan environment terlindungi:

```bash
python3 /opt/space/current/scripts/ops/space_ops.py --policy /etc/space-backup/policy.json backup --stop-services
```

`SPACE_DATABASE_URL` dibaca dari environment yang terlindungi, bukan ditulis pada command history. Jangan menyisipkan nilai secret pada contoh perintah/log. Alat tidak membaca `.env` repository secara otomatis.

Jurnal `backup-services.json` berada pada staging root. SIGTERM mencoba memulihkan layanan melalui `finally`. Jika proses mati paksa/mesin padam, `resume` atau `ExecStopPost` memulihkan unit yang tercatat setelah memastikan pemilik proses lama tidak lagi hidup. Periksa keadaan layanan dan alasan admin menghentikannya sebelum menjalankan pemulihan manual:

```bash
python3 /opt/space/current/scripts/ops/space_ops.py --policy /etc/space-backup/policy.json resume
```

Contoh `space-backup.service.example` dan `.timer.example` **belum diinstal/diaktifkan**. Sesuaikan path, unit, kapasitas dan pembatasan resource pada host sasaran. Service menjalankan resume sebelum/sesudah backup agar jurnal tertinggal tidak diabaikan. Tidak ada penghapusan backup dalam job harian.

## Verifikasi dan retensi

`verify` memeriksa checksum ciphertext tepercaya, dekripsi lengkap, path/jenis/ukuran anggota arsip, manifest/checksum seluruh file serta keterbacaan daftar dump. Ini belum merupakan restore drill.

```bash
python3 /opt/space/current/scripts/ops/space_ops.py --policy /etc/space-backup/policy.json verify \
  --archive /mnt/space-backup/archives/ARSIP.tar.age \
  --identity /media/backup-key/identity.txt --expected-sha256 CHECKSUM_TEPERCAYA
```

`prune` hanya menampilkan kandidat. `--apply` diperlukan untuk penghapusan, retensi harus telah dikonfigurasi dan setidaknya `minimum_copies` terbaru dipertahankan. Alat memeriksa checksum sebelum menghapus. Berkas `.partial` tidak dihitung sebagai salinan baik. Retensi otomatis/off-host object lifecycle harus mengikuti kebijakan yang sama dan tidak diasumsikan tersedia.

## Restore terisolasi

Gunakan klaster/role target yang dipilih eksplisit melalui `SPACE_RESTORE_DATABASE_URL`, menunjuk database administrasi. Skrip tidak memakai URI produksi sebagai fallback. Nama target baru harus `space_restore_...` atau `space_shadow_...`, penyimpanan harus direktori baru, dan alat **tidak memiliki opsi overwrite/drop**. Akun/role DB target diprovisikan operator; ACL pengguna aplikasi tetap berasal dari isi DB, sedangkan owner/grant PostgreSQL perlu disiapkan kembali secara terbatas.

```bash
python3 /opt/space/current/scripts/ops/space_ops.py --policy /etc/space-backup/policy.json restore \
  --archive /mnt/space-backup/archives/ARSIP.tar.age \
  --identity /media/backup-key/identity.txt --expected-sha256 CHECKSUM_TEPERCAYA \
  --target-db space_restore_DRILL_ID --target-root /srv/space-drills/DRILL_ID \
  --server-bin /opt/space/releases/RELEASE/space-server
```

Ganti `DRILL_ID` dengan huruf kecil/angka/underscore. Tidak ada contoh secret literal. Sesudah arsip lulus verifikasi, alat membuat database kosong dan langsung memasang parameter DB `space.restore_quarantined=true`. Parameter ini tidak hilang hanya karena dump aplikasi dipulihkan. Restore dump satu transaksi, cocokkan jumlah record sebelum perubahan karantina, periksa blob terhadap metadata, lalu jalankan `space-server quarantine-restore` dengan konfirmasi nama DB dan alasan.

`quarantine-restore` memerlukan `SPACE_RUN_MODE=isolated_restore`, menolak target di luar prefix, memerlukan tidak adanya server aktif, menambahkan marker `runtime_control`, mencabut sesi salinan dan mencatat audit. Histori pengiriman/keputusan tidak dihapus. Laporan menandai runtime acceptance masih belum diuji. Jika pemulihan gagal, database/direktori parsial ditinggalkan untuk inspeksi, tidak dijalankan atau dihapus diam-diam.

`restore.env` memuat mode isolated dan alamat loopback, tanpa URI/kredensial DB. Sediakan URI untuk role target terbatas secara terpisah sebelum `serve`. State WA diletakkan pada `wa-state-quarantined`, tidak dipasang sebagai state bridge dan bridge tidak dijalankan. Jangan menyalin secrets produksi ke konfigurasi drill.

Guard runtime:

- `SPACE_RUN_MODE=isolated_restore`: scheduler tugas/SOP, worker WA, polling/mirror sumber, semua HTTP transport sumber, transport bridge, SMTP/email dan penyajian inbox notifikasi perangkat dinonaktifkan. API bisnis tetap dapat diuji pada DB salinan, dengan izin pengguna normal.
- `SPACE_RUN_MODE=maintenance`: transport/worker/nonaktif yang sama, ditambah penolakan mutasi HTTP. Pembacaan masih memakai izin sesi. Untuk snapshot file, gunakan penghentian penulis lengkap sebagaimana prosedur backup, bukan sekadar membuka halaman maintenance.
- `serve` memegang shared runtime lease selama hidup. Perubahan karantina menolak server aktif; kehilangan koneksi lease menghentikan server.
- Mode live ditolak apabila marker DB masih isolated atau parameter DB karantina masih aktif. Mode isolated mewajibkan marker DB yang sesuai serta bind/public URL loopback. Gunakan port forwarding untuk melihat drill.

Tidak ada aktivasi produksi otomatis atau perintah promosi pada alat ini. Sebelum memulihkan produksi, siapkan rencana cutover terpisah yang meninjau antrean historis, cutoff, kredensial, versi dan rollback. Mengubah satu variabel environment tidak boleh menjadi mekanisme mengirim ulang semua pengingat lama.

## Bukti drill yang wajib dicatat

Pada fase verifikasi akhir, lakukan restore dari bundle terenkripsi ke target baru. Catat checksum/versi/baseline, waktu mulai–selesai dan hasil pemeriksaan:

- Akun, periode, keanggotaan/role, akses privat lintas divisi dan akun nonaktif tetap benar; sesi dari sumber tidak dapat dipakai pada salinan.
- Tugas/status/PJ/bukti, Notes/template/komentar, file lampiran dan checksum, temuan/keputusan SOP serta audit sesuai snapshot.
- Lampiran yang dihapus/note yang tak berizin tidak dapat diunduh, meskipun blob fisiknya masih ditahan untuk retensi.
- Tidak ada koneksi WA/SMTP/Google/Trello atau toast catch-up dari histori; salah mode live ditolak.
- Restart proses/kehilangan lease dan kegagalan backup memulihkan/menolak layanan sesuai jurnal.
- Waktu pemulihan dan data terbaru yang tersedia dibandingkan RPO/RTO yang benar-benar disepakati; kegagalan harus menghasilkan tindakan koreksi.

Belum ada skenario di atas yang diklaim lulus.

## Gangguan dan serah terima

**WA terputus:** periksa status layanan, koneksi dan outbox di Space. Bedakan queued, gagal yang belum mencoba, unknown, accepted, delivered dan read. Jangan menghapus jurnal atau mengirim ulang unknown secara manual. Hubungkan ulang sesi bila dibutuhkan melalui pengelola yang berizin; QR/kode hanya pada UI privat. Pergantian nomor/sesi memerlukan peninjauan mapping grup dan mention. Pemulihan state lama tidak membuktikan sesi masih diterima WhatsApp.

**Sheets/Trello/Drive gagal:** matikan polling/mirror bermasalah bila perlu, pertahankan snapshot baik terakhir dan baca staging/log konflik. Sumber kosong/error bukan instruksi menghapus tugas. Periksa kredensial/izin di penyimpanan operator, jangan mengirimkannya ke chat. Ikuti mapping/binding yang disetujui sebelum mengaktifkan lagi. Jangan menyentuh formula, VIOL_LOG atau status manual Reo untuk mencoba perbaikan.

**Server mati:** gunakan salinan darurat terakhir dan catat waktu snapshot. Rekam perubahan darurat beserta ID asli, pelaku dan waktu; jangan menganggap sheet darurat sudah tersinkron otomatis. Restore pada lingkungan terisolasi, verifikasi, lalu gunakan rencana cutover/rekonsiliasi yang ditinjau. Alur ekspor/rekonsiliasi darurat masih pekerjaan berikutnya pada checkpoint ini.

**Desktop bermasalah:** gunakan web pada backend yang sama bila server sehat. Simpan draf/pending perangkat, catat versi/OS, lalu kembali ke paket desktop sebelumnya yang kompatibel. Jangan menghapus cache/draf sebagai langkah pertama. Kembali ke binary lama tidak berarti boleh menurunkan skema DB; kompatibilitas versi diuji sebelum rilis.

**Pergantian admin/pengelola bot:** lakukan alur serah terima akun organisasi melalui UI/API yang sudah ada, bedakan akun pribadi dari admin organisasi, tinjau akun pemulihan, sesi aktif, akses arsip, mapping bot/grup dan penjagaan kunci backup/WA. Operator pengganti harus mampu melakukan drill memakai aksesnya sendiri. Rotasi secret tidak dilakukan dengan menyalin nilainya ke catatan handoff.
