# Kelanjutan seluruh tahapan Space

Instruksi pengguna terbaru: **lanjutkan sampai desktop/web dan operasional/pilot selesai; jangan mengerjakan mobile; build dan test di akhir; hasil akhir harus bisa diunduh**. Tahap 6A/6B dikeluarkan dari scope aktif oleh pengguna pada 29 Oktober. Delegasi OpenCode2 AWS sempat dimulai tetapi terhenti pada kuota/auth; pengguna kemudian meminta agen utama melanjutkan sendiri. Seluruh tahap nonmobile yang tersisa tetap termasuk goal, bukan hanya satu checkpoint.

## Posisi awal

Worktree sudah memuat perubahan Sesi 2C/Tahap 3 dari sesi sebelumnya. Perubahan tersebut belum dibangun/diuji dan tetap harus masuk pemeriksaan akhir. Jangan menganggap handoff implementasi sebagai bukti acceptance/runtime lulus.

## Progres implementasi

- [x] Membaca PRD, roadmap, kondisi worktree dan handoff Reo serta konteks proyek terkait.
- [x] Menambahkan model konten Medhum: kalender posting, deadline desain H-2 sekali, PJ Humas/desain, tautan tugas kanonis yang terpisah.
- [x] Menambahkan API/klien konten dengan pemisahan wewenang Humas dan Mediatek; kalender tugas terhubung tidak dapat diubah melewati model konten.
- [x] Menambahkan adapter Google Sheets/Trello dengan host tetap, pembatasan ukuran/waktu, kredensial server dan error tanpa material secret.
- [x] Menambahkan konfigurasi, riwayat fetch, snapshot item, pratinjau dan keputusan terapkan/tautkan/buat/abaikan lewat API.
- [x] Menambahkan worker polling sumber terpisah dari scheduler reminder, reservasi fetch lintas instance dan pemulihan run yang tertinggal.
- [x] Menambahkan UI native kalender konten dan sumber: pemetaan kolom, konfigurasi read-only, aktif/nonaktif polling, fetch, pratinjau/riwayat, pilihan target/PJ dan konfirmasi keputusan.
- [x] Menambahkan baseline sumber/aplikasi, opt-in mengikuti binding, penggabungan bidang milik sumber dan antrean konflik; pembaruan PJ tetap memakai pemetaan akun yang disetujui.
- [x] Menambahkan mirror selektif dengan header kolom khusus Space, perlindungan formula, metadata baris, ID stabil, no-op dan status gagal/ambigu. Penulisan eksternal tetap default nonaktif.
- [x] Menambahkan kebijakan jam posting/cutoff per wewenang divisi, API/klien/UI dan riwayat perubahan konektor. Semua implementasi 4A ini belum dibangun/diuji.
- [x] Menulis bridge Node/Baileys dengan state terenkripsi, sesi terpisah, QR privat, allowlist grup, event/receipt persisten, jurnal send dan konversi stiker.
- [x] Menulis protokol Go ke bridge, skema WhatsApp, API pengelolaan koneksi/grup, style/template dan tantangan link identitas sekali pakai.
- [x] Menulis worker penerimaan event/ack setelah commit, outbox dengan lease dan rekonsiliasi jurnal, serta satu job grup per event pengingat.
- [x] Menulis konsumsi challenge identitas, pemeriksaan akses saat command, command tugas/jadwal/cek/hari/pending/done/bantu/delegasi, custom command dan konversi/penetapan stiker.
- [x] Menghubungkan UI native pengaturan WhatsApp, QR privat, pemetaan grup, kode identitas, editor command dan aktivitas outbox ke klien/API.
- [ ] Tuntaskan acceptance 4B: bahasa respons inti, peninjauan perilaku retry/receipt dan batas lifecycle dua sesi; verifikasi runtime tetap fase akhir. Command rekap/evaluasi mengikuti 4D.
- [x] Menulis inti 4C: parser broadcast per item, metadata Drive, pencocokan kandidat, receipt/deduplikasi, waktu asli, koreksi pasangan dan UI inbox/tinjauan.
- [ ] Cocokkan parser dengan contoh broadcast nyata dan penuhi acceptance 4C pada fase verifikasi akhir; kode belum dibangun/diuji.
- [ ] Tahap 4D: konfigurasi SOP, temuan awal/tinjauan/ralat, follow-up dan rekap/evaluasi.
- [x] Menulis fondasi 4D: evaluator observasi murni, skema kebijakan/konteks/temuan/keputusan/follow-up/rekap, izin SOP terpisah dan API konfigurasi/izin/konteks/persetujuan bot.
- [ ] Selesaikan seluruh alur dan acceptance 4D; implementasi backend belum merupakan fitur lengkap atau bukti penerimaan lulus.
- [x] Menulis worker temuan/observasi, API keputusan/riwayat/bukti manual, follow-up H-4, rekap mingguan, laporan rentang/quarter/CSV dan binding klien SOP.
- [x] Menulis UI 4D, ralat lintas divisi berizin ganda, paginasi, status/riwayat/prasyarat pengiriman untuk evaluator serta ekspor CSV desktop/web. Verifikasi seluruh alur tetap fase akhir.
- [ ] Tahap 5A: backup/restore, spreadsheet darurat dan runbook.
- [ ] Tahap 5B: migrasi/banding/cutover, distribusi/update/rollback dan kesiapan pilot.
- Ditunda atas instruksi terbaru: Tahap 6A Flutter Android; tidak dikerjakan pada goal aktif.
- Ditunda atas instruksi terbaru: Tahap 6B iOS; tidak dikerjakan pada goal aktif.
- [ ] Siapkan paket unduhan hasil akhir (aplikasi desktop/web, server/bridge yang diperlukan, panduan dan checksum) sesudah build/test akhir.
- [ ] Setelah implementasi lengkap: build, tes, perbaikan dan verifikasi seluruh scope, termasuk perubahan 2C/3 yang belum diuji.

## Keputusan dan batas

- Belum menjalankan build, tes, migrasi atau fetch sumber nyata pada kelanjutan ini.
- Konektor belum diaktifkan pada data produksi; sumber Reo/sheet/WA tidak diubah.
- Sumber Reo lama tetap referensi. Pengurangan tanggal jobdesk lama tidak dipakai sebagai input untuk mengurangi H-2 lagi.
- Nama sumber tidak dianggap identitas akun. PJ Humas dipilih eksplisit; PJ desain tetap penetapan Mediatek, bukan pengambilalihan dari kolom lama.
- Source key yang tidak tersedia memakai kandidat turunan untuk pratinjau saja; perubahan ambigu memerlukan binding/tinjauan. Snapshot kosong/rusak tidak menghapus tugas.
- Konfirmasi pilihan sumber sudah diminta secara opsional melalui pertanyaan sesi; pekerjaan kode yang tidak bergantung pada jawabannya tetap berjalan.

## Referensi adapter

[Sheets values.get](https://developers.google.com/workspace/sheets/api/reference/rest/v4/spreadsheets.values/get), [Trello authorization](https://developer.atlassian.com/cloud/trello/guides/rest-api/authorization/), [Trello cards](https://developer.atlassian.com/cloud/trello/rest/api-group-cards/).

## Catatan kelanjutan kode

Migrasi baru: `0008_medhum_connectors.sql`. Model/API/klien/store konten berada pada `internal/*/medhum.go`; UI di `internal/appui/medhum_view.go`. Sumber eksternal di `internal/integrations/sources.go`; konfigurasi/pratinjau API di `internal/server/connectors.go`, worker di `connector_worker.go`, UI di `connectors_view.go`. Semua masih perubahan worktree bersama perubahan 2C/3, belum build/test/migrasi/deploy.

Polling mengisi staging/pratinjau dan dapat mengikuti binding yang disetujui secara eksplisit. Mirror ditulis dengan sakelar penulisan eksternal default mati dan header khusus Space; belum ada penulisan sumber nyata oleh agen. Tombol/konfigurasi bawaan read-only. Tahapan 4B–6 tetap pekerjaan aktif berikutnya; jangan menandai goal lengkap hanya karena 4A sudah memiliki UI/API.

## Checkpoint kelanjutan terbaru

- `0009_source_ownership_policy.sql`: baseline binding/follow-up source, kebijakan jam Medhum dan riwayat mirror.
- `0010_whatsapp.sql`: koneksi/grup, identitas/challenge, inbox/outbox, custom command, aset dan audit WA.
- `internal/server/connector_apply.go`, `connector_mirror.go`, `medhum_policy.go`: auto-apply, mirror dan policy. UI/klien konektor/kalender diperluas.
- `bridge/src/`: store AES-GCM, lifecycle Baileys, pengiriman persisten, media/stiker dan HTTP internal. Runtime/dependensi belum diinstal atau dijalankan.
- `internal/whatsapp/client.go`: protokol bridge; `internal/server/whatsapp.go`: API koneksi/grup/link; `internal/api/whatsapp.go` dan `internal/store/whatsapp.go`: kontrak/model.
- Worker Go/command/UI WhatsApp kini sudah ditulis; lihat checkpoint di bawah. Berikutnya menuntaskan celah 4B lalu parser/bukti 4C dan SOP 4D, operasional/pilot dan Flutter/iOS sesuai roadmap; build/test seluruh perubahan tetap fase akhir.

Dokumentasi mirror memakai [developer metadata](https://developers.google.com/workspace/sheets/api/guides/metadata) dan [row association](https://developers.google.com/workspace/sheets/api/reference/rest/v4/spreadsheets.developerMetadata). Dependensi bridge dipilih dari registry npm upstream; tidak ada fetch Sheets/Trello nyata, pairing, pengiriman WA atau perubahan Reo.

## Checkpoint worker, command dan UI WhatsApp

- `whatsapp_worker.go`: lifetime terpisah yang ditunggu sebelum DB ditutup. Pembaca bridge diserialisasi per koneksi; event dan cursor disimpan satu transaksi, ack hanya setelah commit. Gap sequence tidak dilewati diam-diam. Koneksi/grup/periode diperiksa ulang.
- Outbox memiliki klaim/lease yang di-commit sebelum network call. Job UUID tetap dipakai di bridge. Hasil ambigu atau proses mati diperiksa lewat jurnal; jurnal kosong tidak menjadi alasan mengirim ulang. Retry otomatis hanya untuk penolakan bridge yang menyatakan belum mencoba, dibatasi delapan percobaan. Receipt accepted/delivered/read diproyeksikan ke status delivery tanpa menurunkan receipt yang lebih kuat.
- `whatsapp_delivery.go`: satu job grup per event, mention berasal dari identitas terverifikasi dan keanggotaan aktif. Reminder pribadi tidak masuk WA. Tugas yang terhubung ke Medhum memakai ruang bersama secara deterministik; notifikasi perubahan tugas tidak lagi membuat satu pesan grup untuk setiap penerima. Kegagalan lama tidak dibackfill otomatis saat bot baru aktif.
- `whatsapp_inbound.go`, `whatsapp_task_commands.go`, `whatsapp_stickers.go`: challenge sekali pakai, identitas PN/LID dari metadata bridge, otorisasi berdasarkan akun/ruang/pemilik tugas yang berlaku sekarang. Tidak memalsukan browser session. Mutasi, audit, receipt inbound dan job balasan atomik. `!done` tanpa bukti cocok tidak menutup tugas; pengajuan hasil memakai helper yang sama dengan HTTP dan mempertahankan waktu WA/receipt asli.
- Custom command/stiker memiliki scope koneksi/grup dan tidak mengganti command inti. API CRUD memakai versi dan memeriksa kepemilikan aset/grup. Respons teks bersama stiker dijadikan dua job karena stiker tidak memiliki caption. Nama, sapaan dan template diterapkan ke pesan; template wajib mempertahankan `{body}`. Pilihan `language` masih metadata, respons inti baru bahasa Indonesia: perlu diselesaikan sebelum mengklaim 4B lengkap.
- `internal/client/whatsapp.go`, `internal/appui/whatsapp_view.go`: halaman placeholder diganti kontrol koneksi/QR/grup, kode tautan pribadi, editor command dan riwayat pengiriman. QR/kode tidak disimpan ke preferensi/cache; respons async dilindungi generasi akun/Space/divisi. Runtime/render belum diperiksa.
- `0010_whatsapp.sql` ditambah `wa_outbox.actor_id` untuk pemeriksaan ulang akses peminta sebelum balasan dikirim. Migrasi ini masih belum diterapkan.
- Bridge memakai generation guard tambahan saat menyiapkan socket dan menyimpan update kredensial. API kontrol memegang lock scope selama panggilan bridge untuk mencegah connect terlambat mendahului disconnect/penutupan periode. Sinkronisasi grup membaca mapping terbaru.
- Perbaikan terkait 4A: keputusan membuat konten baru mengisi PJ Humas eksplisit sebelum memanggil validasi izin/nama PJ.
- **Belum menjalankan build, test, migrasi, instalasi dependensi, bridge, pairing atau kiriman WA nyata.** Seluruh klaim checkpoint adalah implementasi sumber, bukan bukti acceptance lulus. Tahap 4C–6 dan verifikasi keseluruhan tetap bagian goal aktif.

## Checkpoint parser, Drive dan tinjauan 4C

- Inti 4C ditulis pada `internal/broadcast`, `internal/integrations/drive.go`, `internal/server/wa_content_{worker,apply,api}.go`, model/store/klien `wa_content.go`, serta UI `internal/appui/wa_content_view.go`.
- `0011_wa_content_evidence.sql` menambah kebijakan parser default nonaktif, antrean item, receipt konsep/Drive, asosiasi repost dan riwayat keputusan append-only. `task_evidence.recorded_at` memisahkan urutan pencatatan dari waktu kejadian asli.
- Worker bisnis memproses non-command dan koreksi pesan; hasil per item memakai savepoint. Kegagalan pencocokan satu item tidak membatalkan item valid lain. Probe metadata Drive dibatasi, berada di luar lock DB, dan memakai scope/token cache terpisah dari Sheets.
- Konsep hanya otomatis bila judul/tanggal/PJ/pengirim cocok; tidak menetapkan PJ desain atau menutup publikasi Humas. Bukti Drive menjadi `pending` hanya untuk kandidat tunggal yang relevan dan akses terkonfirmasi. Ambiguitas, referensi, waktu tidak wajar dan akses gagal tetap ditinjau.
- Repost mempertahankan receipt/waktu pertama dan menekan notifikasi/balasan duplikat atas fakta yang sudah diterapkan. Link yang sudah diketahui tidak dipasangkan ke tugas aktif baru hanya karena tugas lamanya selesai. Edit/revoke tidak menarik fakta otomatis.
- API/UI Medhum menyediakan pratinjau contoh, aktivasi terpisah Humas/Mediatek, inbox, sumber, pilihan konten, pengabaian/penarikan/pemulihan/ralat beralasan, riwayat dan tautan ke detail tugas desain untuk tinjauan kecocokan kanonis. Pengirim selalu berasal dari identitas envelope, bukan pilihan nama bebas.
- Helper penyerahan dan tinjauan bukti dipakai bersama HTTP/WA/koreksi; ralat mempertahankan source timestamp. Repost yang ikut ditarik mendapat riwayat keputusan sendiri.
- `SPACE_GOOGLE_DRIVE_ACCESS_TOKEN` disediakan sebagai konfigurasi opsional metadata; tidak diisi atau dipakai mengakses file nyata.
- **Masih perlu:** contoh broadcast asli (pertanyaan opsional telah dikirim), validasi variasi parser, bahasa respons inti 4B dan pengurangan lock periode selama network call transport yang lambat. Seluruh build/test/runtime tetap belum dijalankan sesuai instruksi fase akhir.
- Berikutnya: selesaikan celah 4B tersebut dan Tahap 4D (SOP/follow-up/rekap), lalu 5A/5B dan mobile 6A/6B. Rincian perilaku, batas dan skenario verifikasi ada di [handoff 4C](wa-content-evidence.md). Goal keseluruhan tetap aktif.

## Checkpoint transport dan fondasi 4D

- Panggilan polling, kontrol sesi dan pengiriman bridge dipisahkan dari transaksi periode/account yang panjang. Advisory lock khusus per bot memakai koneksi fisik di luar pool API, dibatasi dua operasi transport per instance. Worker command/parser/outbox/polling dipisah dan menunggu shutdown.
- Persiapan stiker juga berada di luar transaksi mutasi, dengan revalidasi sebelum commit. Bridge mempertahankan receipt kuat dan `acceptedAt` asli; `0012_sop.sql` menambah `wa_outbox.accepted_at`.
- Evaluator murni 4D, model/store, migrasi serta API kebijakan, konteks dan izin evaluator ditulis. `sop.read`/`sop.review` terpisah dari kecocokan hasil; grant Litbang/BPH tidak membuka Notes/Space lain. Kesepakatan pemenuhan melalui bot memerlukan usulan Humas dan persetujuan Litbang pada versi kebijakan yang sama.
- Rincian kode serta kelanjutan worker/keputusan/follow-up/rekap/UI ada di [handoff fondasi SOP](sop-foundation.md). Bahasa respons inti dan verifikasi lifecycle dua sesi tetap belum selesai.
- **Belum build, test, migrasi, runtime, pairing, pengiriman atau penulisan sumber nyata.** Semua scope 4D–6, sisa 4B/4C dan verifikasi seluruh perubahan tetap goal aktif.

## Checkpoint backend alur 4D — 29 Oktober

- Worker `RunSOP` ditambahkan pada lifetime server. Bukti dibandingkan per konten/scope; perubahan sumber menghasilkan observasi dan tanda tinjau ulang, sambil mempertahankan keputusan/atribusi/tanggal keputusan manusia.
- API temuan, filter, keputusan, histori, bukti manual dan metadata pemilih ditulis; snapshot observasi terbaru dan dasar keputusan terakhir dipisahkan. Trigger/log sumber mencegah perubahan cepat hilang di sela tick.
- Follow-up H-4 memakai bot Humas, calendar key, mention terverifikasi dan outbox. Rekap mingguan hanya ke grup divisi masing-masing. Laporan quarter/rentang dan CSV memakai query lengkap, bukan batas 1000 daftar UI.
- Command summary/rekap/eval ditambahkan sesuai scope. Akses SOP untuk balasan rekap/eval diperiksa ulang saat dispatch. Cache offline mengecualikan data bot/konektor/SOP.
- `internal/client/sop.go` ditulis. UI Evaluasi masih placeholder; ralat lintas divisi, paginasi, ekspor berkas klien dan riwayat pengiriman evaluator masih perlu diselesaikan sebelum mengklaim 4D lengkap.
- Rincian file/alur/batas ada di [handoff backend SOP](sop-workflows.md). Belum build/test/migrasi/runtime atau aktivasi data nyata. Goal tetap mencakup seluruh 4D–6, sisa 4B/4C dan verifikasi keseluruhan.

## Koreksi scope dan checkpoint UI — 29 Oktober

Instruksi terbaru pengguna mengesampingkan referensi scope 6/mobile pada checkpoint historis di bawah/atas: **mobile tidak dikerjakan**. Hasil akhir nonmobile harus disiapkan sebagai paket unduhan, setelah build/test akhir. Belum ada klaim binary tersedia.

UI Evaluasi kini terhubung ke scope, daftar/detail/riwayat temuan, keputusan/ralat dalam divisi, konfigurasi kebijakan, konteks/bukti manual, izin evaluator dan laporan quarter/rentang. Daftar temuan memakai cursor dan ringkasan ringan; detail mengambil bukti lengkap. Pengesahan memeriksa fingerprint sumber terkini agar observasi lama tidak disahkan sebelum worker mencatat perubahan. Bukti manual memerlukan konfirmasi manusia; pengecualian mendesak memakai kategori eksplisit. Riwayat pengiriman SOP khusus evaluator sedang ditambahkan, tanpa membuka kontrol/payload WhatsApp. Ralat lintas divisi dan ekspor berkas klien masih perlu. Semua ini masih sumber lokal belum build/test/migrasi/runtime.

## Checkpoint SOP lengkap di UI dan awal lampiran biner

UI, filter/cursor, ralat lintas divisi berizin ganda dengan preview/konfirmasi, riwayat/snapshot pengiriman SOP, pemeriksaan prasyarat dan ekspor CSV autentik desktop/browser kini ditulis. Migrasi baru `0013_sop_transfers.sql` mempertahankan tautan ralat append-only. Belum diuji.

Awal 5A melengkapi lampiran biner Notes: storage private immutable, batas ukuran/checksum, unggah idempoten dengan receipt/tombstone, download berizin terkini, pemilih berkas desktop/browser, unggah/retry serta unduh/simpan. Migrasi `0014_attachment_files.sql`. Pilihan berkas pending masih memori halaman, belum durable offline. Backup/restore, guard isolated restore dan distribusi belum selesai. Detail: [handoff SOP/UI/files](sop-ui-files.md).

Delegasi AWS: Tailscale SSH berhasil dan OpenCode2 tersedia. Salinan kerja terpisah dibuat tanpa secrets; sesi `b07045e32d60` untuk 5A berhenti karena kuota model/auth provider, tanpa hasil implementasi. Pertanyaan pilihan kelanjutan agent sedang menunggu; pekerjaan lokal tidak terblokir. Mobile tetap dikeluarkan dari scope terbaru. Paket unduhan/build/test masih fase akhir.

## Checkpoint pending berkas dan pengaman restore

Instruksi terbaru: OpenCode2 memang berhenti; pengguna meminta agen utama melanjutkan sendiri. Tidak menunggu agent AWS dan tidak menganggap ada hasil delegasi. Scope tetap tanpa mobile, paket unduhan setelah build/test akhir.

Penyimpanan pending lampiran lintas restart/halaman ditulis: namespace server/akun/note, UUID unggah stabil, SHA-256, batas 32 file/256 MiB per akun/server. Desktop memakai file privat atomik dan browser IndexedDB transaksi. UI memulihkan dari perangkat setelah note yang sama dapat dibaca; unggah tetap online dan diotorisasi ulang. Kuota/kerusakan/gagal simpan ditampilkan; salinan tidak diklaim cloud. Belum diuji restart/browser/runtime.

Mode `SPACE_RUN_MODE` live/isolated_restore/maintenance, marker DB dan runtime lease ditulis (`0015_runtime_control.sql`). Restore/maintenance mematikan worker/transport WA, sumber, email dan notifikasi perangkat; maintenance menolak mutasi HTTP. `quarantine-restore` memerlukan konfirmasi DB baru dan mencabut sesi salinan sambil mempertahankan histori. Mode live ditolak oleh marker/parameter DB karantina. Belum diuji.

`scripts/ops/space_ops.py`, kebijakan/contoh systemd dan `docs/operations/recovery.md` kini ditulis: snapshot dengan penghentian unit + lease eksklusif, dump/blob/state terenkripsi, manifest/checksum, age, verify, restore target baru, jurnal pemulihan layanan dan retensi dry-run default. Belum ada service/timer dipasang, backup/restore dijalankan atau dependensi diinstal. Alur spreadsheet darurat/reconcile, bahasa bot, sisa 5B dan paket rilis masih dilanjutkan sebelum verifikasi akhir.

## Checkpoint spreadsheet darurat dan bahasa bot

Snapshot/ekspor CSV tugas, parser impor, pratinjau, penerapan progres terpilih dan disposisi manual/abaikan kini ditulis. Baseline server tidak dipercayakan pada kolom CSV; versi/PJ/bukti/izin yang berubah memerlukan tinjauan. Receipt append-only menjaga retry, pelapor/waktu sheet bukan aktor/waktu bukti kanonis. UI ada pada Pengaturan → Darurat. Migrasi `0016_emergency_snapshots.sql`.

Bahasa inti WA Indonesia/English ditulis untuk command, error umum, pengingat/overdue, pemberitahuan tugas/rapat, parser receipt, follow-up dan rekap. Message key/argumen literal menjaga teks pengguna/custom command tetap utuh (`0017_delivery_language.sql`). Antrean lama tidak dirender ulang/dikirim ulang hanya karena bahasa diganti. Preset Madura opsional belum dibuat tanpa contoh nyata. Detail: [handoff darurat/bahasa](emergency-language.md).

Belum build/test/migrasi/runtime/WA nyata. Observabilitas, sisa persiapan 5B, paket unduhan dan verifikasi seluruh perubahan masih dikerjakan. Scope tetap tanpa mobile dan agen utama bekerja sendiri.

## Checkpoint observabilitas dan persiapan rilis

Registry/heartbeat progres worker, status polling WA terakhir, API infrastruktur/scoped divisi dan UI Status layanan ditulis (`0018_worker_health.sql`). Tidak membuka isi pesan/nomor/QR/kredensial/Notes/reminder pribadi. Heartbeat tidak disamakan dengan receipt.

Versi candidate `0.3.0-pilot.1`, BuildInfo/hash source dan CLI version ditambahkan. Builder snapshot + pemeriksaan + Windows installer/web/server Linux/source/full ZIP/checksum ditulis; belum dijalankan dan suite final masih perlu dilengkapi. Pembanding legacy CSV read-only dan panduan distribusi/pilot/cutover kini tersedia, memakai inventaris handoff Reo yang dibaca ulang tanpa perubahan sumber live. Detail [handoff observabilitas/rilis](operations-release-prep.md).

Berikutnya perintah aktivasi restore/cutoff yang aman, audit dan fixture/tes akhir seluruh scope nonmobile, build/test/perbaikan, lalu paket unduhan. Tidak ada klaim production-ready/cutover/rilis diuji. Mobile tetap tidak dikerjakan.

## Checkpoint aktivasi restore dan peralihan ke verifikasi akhir

Migrasi 0019, `store/runtime_activation.go` dan CLI `activate-restore` menambah preview ber-hash, konfirmasi database/restore/cutoff/operator, lease eksklusif dan transaksi aktivasi. Antrean hasil restore ditahan dengan marker terpisah tanpa memalsukan receipt; pending inbound diabaikan, konektor/bot/SOP nonaktif, sesi/challenge dicabut, temuan perlu tinjauan ulang. Epoch baru menghindari event masa depan tertahan unique key lama, cutoff menolak backfill; input bridge sebelum cutoff ditahan untuk audit. Panduan konkret di `docs/operations/restore-activation.md`. Belum dieksekusi pada data live.

Implementasi nonmobile masuk **fase verifikasi akhir**: menyiapkan tool/dependensi dan DB uji terisolasi, fixture regresi lintas fitur, build/test, perbaikan temuan dan paket unduhan. Acceptance perangkat Windows, pairing dua WA, contoh broadcast/sumber produksi dan cutover nyata tetap memerlukan lingkungan/data aktual; tidak diklaim lulus oleh tes sintetik. Mobile tidak dikerjakan.

## Checkpoint verifikasi akhir berjalan

Tool Go/Node/PostgreSQL/age/NSIS disiapkan tanpa mengubah layanan produksi. Database uji berada pada cluster localhost terpisah port 55479. Percobaan paralel menekan memori/swap host; proses build/test milik sesi ini dihentikan dan verifikasi selanjutnya dibatasi satu worker, GOMEMLIMIT 256 MiB dan GOGC 50. AWS diperiksa, RAM bebas juga terbatas; tidak dijadikan ketergantungan.

Pemeriksaan awal menemukan benturan nama state offline, tipe bitmap QR MyGo dan tab pengaturan keluar viewport; diperbaiki. Error 500 server kini tetap memiliki tipe/isi yang benar saat cache offline tidak tersedia. Tes migrasi branch diperbaiki agar merekonstruksi skema historis sebelum dependency baru, bukan membuang tabel dari skema terkini. Tes regresi menemukan argumen timestamp berlebih pada statement aktivasi tanpa parameter; diperbaiki.

Regresi terarah telah membuktikan receipt offline idempoten, privasi/retry/download lampiran, snapshot darurat dan scope monitoring, H-2 dan kewenangan kalender, serta preview/hash/lease/cutoff/receipt/generasi aktivasi restore. Satu ekspektasi fixture rapat diperbaiki untuk menerima penolakan 403 selain 404; pemeriksaan penuh berikutnya tetap wajib. Tujuh tes unit ops dan tiga tes bridge sempat lulus; builder akan mengulang pada snapshot akhir dan menjalankan drill PostgreSQL/age/blob/isolated runtime sungguhan. Status seluruh suite/build/paket belum diklaim lulus; laporan `verification.json` hasil builder menjadi bukti aktual.

Audit AC-42 menemukan impor maskot/audio yang sebelumnya diam saat gagal. Pesan kegagalan kini terlihat di pengaturan perangkat. GIF diperiksa record/frame/piksel sebelum `DecodeAll`, termasuk ketika memuat aset tersimpan, supaya batas tidak baru diperiksa setelah seluruh alokasi. Tes fixture PNG/GIF/pending perangkat ditambahkan pada fase verifikasi akhir. Kandidat snapshot pertama dibatalkan sebelum paket dibuat agar binary final menyertakan perbaikan ini; pemeriksaan penuh memakai snapshot baru.
