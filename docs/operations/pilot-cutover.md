# Persiapan pilot, pembandingan Reo dan cutover

Checkpoint 29 Oktober 2026: prosedur dan pembanding read-only ditulis. **Tidak ada cron Reo, sheet, bot produksi, DNS, service atau pengirim live yang diubah.** Seluruh implementasi nonmobile serta verifikasi akhir masih aktif. Dokumen ini tidak menjadi izin mengeksekusi cutover.

## Inventaris baseline dari handoff

Acuan: note proyek **Reo — handoff** (`bfd4b6123a0c`), dibaca kembali pada checkpoint ini. Fakta berikut merupakan inventaris handoff, bukan pemeriksaan status live terbaru:

| Sumber/komponen | Peran dan pemilik data | Yang harus ditinjau sebelum cutover |
|---|---|---|
| Kalender Humas, sheet `1hzK9oCeOGBXJQAfhvvzaDk7nA3pE0v7t3LnEgm3WspY`, tab `Postingan` | Konsep/jadwal posting/PJ Humas/PJ Mediatek; diisi Humas | Header/tab aktual, format tanggal dan pemetaan akun yang disetujui |
| Jobdesk Mediatek, sheet `1mFreEyn3uW8TD5aRGT9IMgk3IWHfaC1DyaxhLYuOdr4`, tab `Reminders` | B judul; C **deadline desain sudah posting−2**; G status; H PJ desain; I approval; J key | Jangan kurangi H-2 lagi; status/approval bukan otomatis bukti cocok, publikasi atau temuan SOP final |
| `MyReminders` | Layout/manual/tahunan lama; status Q | Pisahkan tugas organisasi, reminder pribadi dan kejadian tahunan. Jangan menutup kejadian tahunan karena satu reminder selesai |
| `VIOL_LOG`, kolom K/L | Histori pelanggaran manual melalui alur legacy | Pertahankan audit; tidak menulis dugaan Space ke legacy atau memindahkan penilaian otomatis sebagai keputusan final |
| `SYNC_LOG`, formula F dan peta nama/nomor | Provenance sinkronisasi dan resolve mention legacy | Pertahankan formula/log. Nama/nomor legacy tidak otomatis menjadi identitas akun Space terverifikasi |
| Reo sync `5ac9ad8a6a92` | Materialisasi kalender 5 kali/hari | Inventaris ulang wrapper/host/jadwal dan pemiliknya; satu pengendali sumber setelah cutoff |
| Reo daily `a0bf4ef450b2` | Reminder grup pukul 06.00 WIB menurut handoff | Nonaktifkan hanya pada cutover yang disetujui, sebelum pengirim Space aktif untuk event yang sama |
| Reo weekly `18cc5ee6defa` | Rekap lama ke DM pemilik | Konfirmasi audiens berbeda: Space mengirim rekap divisi sesuai kebijakan, bukan otomatis DM legacy |
| Bridge/s6/container Reo | Sesi Baileys lama, supervisor dan jurnal | Catat state/versi aktual dan rollback. Jangan re-pair atau restart beruntun hanya untuk menyiapkan Space |
| Broadcast konsep dan Drive | Bukti sumber dan waktu asli | Konfirmasikan contoh template nyata serta kasus ambigu/repost/edit sebelum aktivasi parser |

Snapshot baseline nyata harus mencatat waktu UTC/WIB, hash ekspor, rentang data, pemilik, mapping dan status pengirim yang benar-benar diperiksa. Handoff tidak membuktikan cron/sesi masih hidup atau data sheet sekarang sama. Tidak ada nomor/JID pribadi yang dimasukkan ke contoh paket rilis ini.

## Mode pembandingan

Jalankan Space pada DB/storage salinan yang dikarantina, tanpa pengirim/polling/mirror otomatis. Gunakan ekspor sumber yang diperoleh dengan akses sah dan staging/binding aplikasi. Source kosong/rusak atau baris hilang masuk tinjauan; tidak menghapus tugas. Jangan membuat mapping berdasarkan judul atau nomor baris saja, terutama judul berulang/reschedule.

`scripts/ops/compare_legacy.py` membandingkan **jobdesk desain lama** yang tanggalnya sudah H-2 terhadap CSV snapshot tugas Space. Masukan:

- CSV legacy dengan header yang dipetakan eksplisit pada `reo-jobdesk-mapping.example.json`.
- CSV Darurat Space yang waktunya diketahui, sebaiknya mencakup tugas selesai untuk pembandingan historis.
- Binding `legacy_key,task_id` yang ditinjau manusia; file contoh kosong, tanpa mapping karangan.
- Format tanggal/status yang ditetapkan eksplisit. Contoh memakai date serial Google sebagaimana handoff; header `Task`/`Deadline` dan lainnya harus disesuaikan dengan ekspor aktual.

```bash
python3 scripts/ops/compare_legacy.py \
  --legacy /srv/space-review/reo-reminders.csv \
  --space /srv/space-review/space-emergency.csv \
  --bindings /srv/space-review/bindings.csv \
  --mapping /srv/space-review/jobdesk-mapping.json \
  --output /srv/space-review/comparison-RUN_ID
```

Alat tidak menghubungi Google/WA/DB dan tidak melakukan penulisan sumber, pairing, subtraction H-2, atau pencocokan judul otomatis. Hasil JSON/CSV memuat hash input dan daftar masalah: key/binding hilang/duplikat, beberapa baris ke satu tugas, target hilang, perubahan judul/deadline/status, precision tanggal/jam, status tak dikenal, serta penyelesaian/approval legacy yang perlu diperiksa terpisah. `cutoverAuthorized` selalu false. Isilah reviewer/keputusan/alasan/referensi Space pada hasil review; rekonsiliasi aktual tetap melalui API/UI berizin, bukan mengubah report menjadi perintah SQL.

Pembanding ini tidak menganggap kalender posting sama dengan jobdesk desain. Kalender Humas diperiksa melalui staging konektor 4A dan tugas Humas/desain kanonis. Bila ekspor legacy masih berupa tanggal posting, jangan berikan sebagai deadline desain ke comparator; gunakan pemetaan/tinjauan kalender yang sesuai. Kolom manual/formula/log legacy tidak ditulis oleh alat.

## Gerbang sebelum pilot

- Build dan seluruh pemeriksaan akhir yang relevan benar-benar dijalankan; tidak ada klaim lulus hanya dari file/verifier yang dibuat.
- Aktor Humas/Mediatek, kadiv, 45, Litbang/BPH dan akun admin organisasi diuji: akses/penolakan API, periode/arsip, Notes privat, reminder pribadi dan akun nonaktif.
- Dua sesi WA, pemetaan grup/PN-LID/mention, receipt, antrean unknown dan retry dibuktikan tanpa pengiriman ganda. Pengiriman uji nyata memerlukan instruksi spesifik untuk penerima/grup uji; jangan mengirim pesan kepada orang lain hanya karena sedang menyiapkan pilot.
- Template broadcast nyata dan variasi diuji; data ambigu/koreksi tidak menjadi hasil final otomatis. Bukti tepat waktu dengan klik selesai terlambat tidak menjadi pelanggaran otomatis.
- Slot follow-up, rekap, quarter, jam date-only, kesepakatan bot dan penanggung jawab sumber telah dikonfigurasi oleh pihak yang berwenang.
- Backup pada lokasi terpisah, akses kunci/secrets oleh pengelola pengganti, restore drill, RPO/RTO/retensi dan salinan darurat yang dapat dibaca saat server mati dibuktikan.
- Paket Windows/web dikenali versinya; update/rollback, tray/toast/autostart/deep link/offline/Ace dan ukuran/RAM/idle diukur pada perangkat sasaran. Tidak mengekstrapolasi angka kapasitas dari perkiraan.
- Selisih baseline/mapping selesai ditinjau dengan provenance; data status/approval/VIOL_LOG lama tidak hilang atau ditafsirkan ulang tanpa keputusan manusia.

Semua butir di atas masih merupakan pekerjaan/pembuktian yang harus diselesaikan, bukan checklist yang sudah ditandai lulus.

## Rencana eksekusi yang perlu disetujui terakhir

Siapkan lembar cutover konkret berisi: versi/digest/checksum, DB/storage/host/API URL target, daftar cron/unit lama yang akan dinonaktifkan, mapping akun/grup/sumber, waktu cutoff dan zona, pengelola yang memantau, hasil backup/restore/baseline/acceptance, daftar antrean di sekitar cutoff, serta batas pemicu rollback yang telah disepakati. Jangan meminta izin generik sebelum hasil persiapan ini bisa ditinjau.

Urutan sesudah lembar konkret disetujui:

1. Simpan baseline akhir dan backup konsisten; bekukan perubahan yang memengaruhi mapping/cutoff.
2. Nonaktifkan pengirim legacy yang akan digantikan, verifikasi status aktualnya dan simpan bukti. Matikan mirror Space sampai kepemilikan kolom dipastikan. Jangan menyalakan dua scheduler untuk reminder produksi yang sama.
3. Rekonsiliasi perubahan terakhir dan cek waktu/versi/bukti. Tentukan hanya kejadian setelah cutoff yang boleh dijadwalkan Space; receipt accepted/unknown lama tidak dijadikan antrean kirim ulang.
4. Aktifkan satu pengendali pengiriman sesuai scope yang disetujui, lalu verifikasi event baru dengan jurnal/receipt dan observasi penerima yang memang ditugaskan menguji.
5. Catat hasil nyata, kegagalan, latensi dan duplikasi. Status health/cron sukses tidak cukup membuktikan pesan diterima.

Alat saat ini sengaja tidak menyediakan promosi DB restore otomatis. Parameter/marker karantina hanya dilepas melalui langkah operasional terpisah yang ditinjau setelah antrean/cutoff benar; mengganti `SPACE_RUN_MODE` saja akan ditolak. Implementasi perintah promosi/cutoff yang benar masih perlu dilengkapi sebelum menawarkan cutover sebagai tindakan siap jalan.

## Rollback

Pemicu dan jendela observasi ditetapkan pada lembar cutover, bukan angka yang dikarang di sini. Jika dipicu: hentikan pengirim Space terlebih dahulu, simpan audit/outbox/jurnal/receipt dan perubahan baru, lalu putuskan jalur pemulihan versi atau data. Jangan memulihkan snapshot lama di atas live sambil pengirim tetap berjalan.

Pengirim legacy hanya boleh diaktifkan kembali setelah status Space benar-benar berhenti dan kejadian di sekitar cutoff telah didedup terhadap receipt kedua sistem. Jika data baru sudah tercatat di Space, susun rekonsiliasi sebelum kembali ke sheet lama; jangan menghapusnya karena rollback binary. Setelah pemulihan, verifikasi satu pengendali aktif dan catat alasan/hasil.

Distribusi binary sebelumnya tidak otomatis mengembalikan schema atau status WA. Jika schema baru tidak kompatibel dengan binary lama, gunakan restore ke target baru dan alih layanan setelah verifikasi/rekonsiliasi. Pertahankan kedua versi dan bukti sampai periode retensi yang disetujui selesai.
