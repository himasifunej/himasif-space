# Pemantauan operasional dan persiapan distribusi

Checkpoint 29 Oktober 2026. Semua uraian adalah sumber lokal; **belum build/test/migrasi/runtime/deployment**. Scope aktif tanpa mobile, agen utama lanjut sendiri, paket unduhan sesudah pemeriksaan akhir. Belum ada installer/ZIP final yang diklaim tersedia.

## Pemantauan

`0018_worker_health.sql` menambah heartbeat per instance/worker/mode dan waktu polling bridge terakhir. Registry worker merekam progres/tick nyata dan kegagalan; monitor hanya mempersist waktu tersebut setiap 15 detik, tidak memajukan waktu worker yang macet. Scheduler, connector, empat loop WA dan SOP telah diinstrumentasi. Shutdown monitor menunggu worker berhenti lalu flush. Batas sunyi disesuaikan dengan batas kerja masing-masing loop, bukan satu angka yang membuat batch parser panjang terlihat mati.

`GET /api/operations` memberi metadata versi/build/mode dan status worker tanpa payload/PII kepada akun yang sudah masuk. `/api/periods/{periodId}/divisions/{divisionId}/operations` membaca scope saat permintaan: kewenangan tasks.manage membuka statistik notifikasi kerja anggota/konektor; whatsapp.manage membuka status bot/antreannya. Akun admin organisasi hanya mendapat infrastruktur, tidak rincian kerja divisi. Reminder pribadi, Notes, nomor/JID, QR, template/bearer dan raw error jaringan tidak masuk DTO. Status cached bridge memiliki `last_polled_at`; poller yang tertinggal tidak ditampilkan seolah kontak bridge baru saja berhasil.

UI **Pengaturan → Status layanan** terhubung pada klien/API, dengan pengaman hasil async akun/periode/divisi. Status bersifat snapshot saat dimuat ulang. Heartbeat/cycle berhasil bukan receipt pengiriman atau pembacaan. Mode restore/maintenance menampilkan worker dan transport nonaktif. Antrean unknown tetap dibedakan dari gagal; tidak ada tombol kirim ulang yang melewati jurnal.

## Versi dan builder

Versi source candidate menjadi `0.3.0-pilot.1` pada `app/mygo.json` dan fallback app. Server memiliki BuildInfo versi/revisi/digest/waktu; `space-server version` tidak memerlukan koneksi DB. Health dan Status layanan membawa identitas build, sehingga commit lama tidak disalahartikan sebagai seluruh isi worktree yang belum di-commit.

`scripts/release/build_release.py` mengambil snapshot source tanpa secret/state/output, menjalankan pemeriksaan Go/DB, vet lintas target, tes Node bridge dan ops (menolak suite nol tes/server skip), membangun server Linux amd64/arm64 serta Windows installer/web, lalu merakit ZIP/tar/source, laporan dan checksum serta ZIP gabungan. Semua tahapan ini **belum dieksekusi**. Lockfile/dependensi bridge dan suite tes final masih harus disiapkan pada fase verifikasi; script tidak membuat laporan lulus hanya karena ada file konfigurasi. Candidate tetap menandai acceptance perangkat/integrasi yang belum dibuktikan. Distribusi pilot memakai installer/paket manual versi tetap; tidak mengaktifkan auto-update/signing tanpa kanal/kunci yang siap.

## Pembandingan legacy dan cutover

Note Reo dibaca ulang sebagai inventaris handoff, bukan status produksi terkini. `scripts/ops/compare_legacy.py` menyiapkan pembandingan ekspor jobdesk vs CSV Space memakai binding eksplisit dan format/status yang ditetapkan. Tidak menghubungi sistem lain, menulis sheet, mengurangi H-2 lagi atau menyamakan done/approval legacy dengan bukti/publikasi/SOP. Hasil review tetap memerlukan operator/aktor berwenang dan sumber aktual. Contoh mapping/binding tidak berisi pasangan akun karangan. Panduan [pilot/cutover](../operations/pilot-cutover.md) memetakan cron/sumber/formula/log lama dan gerbang persetujuan eksekusi konkret.

## Berikutnya

- Lengkapi perintah aktivasi restore/cutoff yang benar-benar menahan antrean historis dan memerlukan peninjauan konfigurasi sebelum pengirim live. Dokumen belum merupakan implementasi promosi.
- Lengkapi fixture/tes akhir serta audit kesesuaian seluruh tahap nonmobile (2C–5B), kemudian jalankan build/test/migrasi terisolasi dan perbaiki seluruh temuan.
- Buktikan parser dengan contoh nyata/dua sesi WA/perangkat sasaran sesuai akses yang tersedia, tanpa mengirim kepada pihak lain tanpa instruksi spesifik.
- Susun paket unduhan dengan bukti pemeriksaan sebenarnya dan catat limit/pending secara jujur. Tindakan produksi hanya setelah hasil konkret siap ditinjau dan pengguna mengizinkannya.
