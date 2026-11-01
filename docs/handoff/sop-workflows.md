# Worker, keputusan dan laporan SOP — checkpoint 29 Oktober 2026

Status: implementasi sumber ditulis, **belum build/test/migrasi/runtime**. Tidak ada kebijakan, persetujuan, pengiriman WA atau data produksi yang diaktifkan agen. Goal seluruh roadmap masih aktif; dokumen ini tidak menyatakan 4D selesai.

## Worker dan bukti

- `internal/server/sop_worker.go` menambahkan `RunSOP`, ditunggu saat shutdown server. Evaluasi memakai transaksi singkat per konten/scope dan advisory lock lintas instance, tanpa network call di dalam transaksi.
- Input berasal dari kalender kanonis, receipt konsep, bukti desain terakhir beserta status kecocokannya, konteks manusia Humas/Mediatek dan follow-up yang benar-benar diterima bridge. Waktu klik selesai bukan waktu penyerahan.
- `sop.EngineVersion` dicatat bersama versi dokumen/kebijakan. Fingerprint sumber mencakup versi evaluator, pihak terkait, kalender, input bukti dan koreksi pesan yang belum ditinjau. Tick dengan sumber yang sama tidak membuat observasi berulang.
- Temuan baru dibuat setelah tenggat terkait berlalu bila ada keterlambatan, kelebihan batas atau bukti belum cukup. Bukti cukup/tepat waktu tidak dibuat menjadi pelanggaran. Temuan yang sudah ada tetap mendapat observasi jika sumber/kebijakan berubah, termasuk aturan yang dinonaktifkan.
- Keputusan manusia, atribusi dan tanggal kejadian keputusan tidak diganti otomatis oleh observasi baru. `needs_recheck` menandai perubahan; observasi terbaru dan snapshot dasar keputusan terakhir disajikan terpisah.
- Trigger pada kalender, bukti/tugas, konteks dan koreksi pesan langsung menandai keputusan yang sumbernya berubah. `sop_source_changes` menyimpan referensi/jenis/waktu perubahan, sehingga kejadian yang berubah lalu kembali sebelum tick berikutnya tidak hilang dari jejak tinjauan. Update bukti identik karena repost tidak dianggap perubahan baru.
- Hitungan dadakan memakai rentang eksplisit, batas lima sesuai SOP, dan urutan sampai konten terkait. Konten pertama dalam jendela tidak dituduh melampaui batas karena ada konten lain yang direncanakan kemudian. Urutan beberapa konten di tanggal/waktu yang sama yang tidak dapat dibedakan menghasilkan bukti belum cukup.

## API keputusan dan metadata

Prefix: `/api/periods/{periodId}/divisions/{divisionId}/sop`.

- `GET /findings`, `GET /findings/{id}`: filter rentang, orang, aturan dan status. Daftar UI saat ini dibatasi 1000; paginasi masih perlu ditambahkan.
- `POST /findings/{id}/decision`: confirm/dismiss/reopen/correct, dengan versi temuan, ID observasi dan alasan. Ralat menetapkan outcome yang berlaku, tidak sekadar label ambigu. Konfirmasi tidak boleh menggunakan observasi `insufficient`/`on_time`/`exempt` sebagai bukti pelanggaran.
- `GET /findings/{id}/history`: keputusan, observasi dan perubahan sumber. Snapshot dasar keputusan terakhir tetap menunjuk versi bukti/dokumen saat diputuskan.
- `POST /findings`: temuan/bukti manual untuk poin 7.1–7.25, termasuk hal visual/isi yang tidak dinilai evaluator waktu. Wajib tautan, penjelasan dan waktu/tanggal kejadian. Hasilnya proposal perlu tinjauan; bukan auto-confirm. Penambahan bukti pada temuan yang sudah ada memakai expected version.
- Atribusi dapat dikoreksi ke akun dengan riwayat keanggotaan pada divisi/periode itu, termasuk anggota yang sudah tidak aktif. **Pemindahan atribusi lintas divisi belum ditulis**; perlu otorisasi kedua scope dan hubungan ralat yang tidak menghilangkan jejak pada divisi asal.
- `GET /people`, `/contents`, `/evaluator-candidates` menyediakan metadata sempit untuk filter/pemilih. Grant SOP tidak membuka Notes atau seluruh Space divisi lain. Kandidat evaluator hanya untuk kadiv asli yang mengatur izin. `SOPAccess.CanApproveBot` membedakan persetujuan Litbang dari izin review umum.
- `internal/client/sop.go` sudah menyediakan binding API kebijakan, konteks, izin, temuan, riwayat dan ringkasan laporan. **Halaman Evaluasi native masih placeholder**; form dan alur UI belum disambungkan.

## Follow-up H-4

- `sop_followup.go` menjadwalkan hanya slot H-4 hari ini dengan jam yang dikonfigurasi; tidak backfill hari-hari lama.
- Wajib ada PJ Humas/desain aktif, tautan konsep, identitas WA PJ desain terverifikasi dan pemetaan Medhum pada bot Humas yang aktif. Pesan berisi judul, nama PJ, metadata mention, tanggal posting dan tautan konsep.
- Calendar key mencakup materi pesan, sehingga pembaruan yang tidak mengubah materi tidak menggandakan follow-up. Job yang sudah sending/unknown/accepted/delivered/read tidak dibuat ulang sebagai kiriman baru.
- Dispatch memeriksa lagi policy/version, kalender, grup, PJ dan hari pengiriman. Perubahan grup membatalkan antrean lama; tidak berarti pesan lama dibackfill ke grup baru secara otomatis.
- Pemenuhan melalui bot hanya masuk input SOP bila diminta Humas, persetujuan versi terkait berlaku pada saat kirim dan `acceptedAt` bridge tersedia. Tanpa itu, pesan tetap bantuan koordinasi. Receipt bukan bukti bahwa seluruh ketentuan SOP atau mutu desain sudah terpenuhi.
- Prasyarat yang belum lengkap saat ini membuat scheduler tidak mengantrekan job. UI/status follow-up perlu menjelaskan prasyarat ini agar tidak mengklaim job sudah dibuat.

## Rekap dan laporan

- `sop_reports.go` membuat rekap mingguan berdasarkan hari/jam konfigurasi. Rentangnya tujuh hari sebelumnya, dibatasi awal/akhir periode. Tidak ada asumsi slot Senin dari sistem lama.
- Rekap hanya dirutekan ke Space/grup **divisi pemilik**; dispatcher menolak Medhum. Snapshot hitungan, referensi observasi/keputusan, alasan dan outbox disimpan agar kiriman historis dapat ditelusuri.
- `GET /report?from=...&to=...` atau `?quarter=LABEL` memakai rentang eksplisit. Quarter harus terdaftar di kebijakan. Laporan lengkap memakai query tanpa batas 1000 daftar UI.
- `format=csv` mengekspor data serta versi/measurement dasar keputusan dan perubahan sumber; nilai teks yang dapat menjadi formula spreadsheet diperlakukan sebagai teks. `summaryOnly=true` mengembalikan hitungan lengkap tanpa payload seluruh bukti, untuk klien native dengan batas respons.
- `!summary` menampilkan hitungan tugas ruang. `!rekap` dan `!eval <label quarter>` memerlukan akses SOP dan grup divisi; tidak mengumumkan temuan/keputusan ke Medhum. Akses SOP diperiksa lagi sebelum balasan antre dikirim.
- Hitungan `confirmed` adalah keputusan manusia terakhir, disertai jumlah yang perlu diperiksa ulang. Observasi baru tidak diam-diam menarik pengesahan lama atau menyatakan pengesahan baru.
- `wa_outbox.target_name` menyimpan nama grup saat antrean dibuat, sehingga riwayat tidak memakai nama tujuan yang baru setelah pemetaan berubah.
- Riwayat rekap/follow-up terjadwal belum memiliki endpoint/UI khusus untuk evaluator; perlu disediakan tanpa memberi hak pengelolaan WA secara tidak sengaja.

## Pekerjaan berikutnya

1. UI native konfigurasi/konteks/izin, temuan, keputusan/riwayat dan laporan; metadata pemilih sudah tersedia. Ekspor CSV perlu pengunduhan berautentikasi dan penyimpanan berkas native/web yang benar.
2. Ralat atribusi lintas divisi, paginasi daftar temuan, serta status/riwayat follow-up dan rekap untuk scope evaluasi.
3. Bahasa respons inti 4B dan parser 4C terhadap contoh broadcast asli (masih belum diberikan). Read/cache bot, konektor dan SOP kini dikecualikan dari cache offline umum; operasi tersebut memerlukan koneksi.
4. Operasional 5A/5B, lampiran biner/backup sesuai PRD, mobile 6A/penilaian iOS 6B.
5. Build, migrasi pada DB uji terpisah, tes dan audit seluruh acceptance di akhir. Skenario penting: sumber berubah sesudah keputusan, keputusan sebelum/berbeda dari observasi terbaru, urutan dadakan ambigu, repost tanpa perubahan, lebih dari 1000 temuan, pembatasan evaluator, periode tertutup, follow-up/status kirim nyata, dua instance dan restart.

Semua klaim di atas adalah perilaku yang ditulis pada sumber. Kebenaran runtime dan acceptance belum dibuktikan.
