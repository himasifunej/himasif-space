# Register keputusan Space

Tanggal: 27 Oktober 2026, WIB. Dokumen pendamping [PRD](PRD.md). Tujuannya menjaga setiap cabang pembahasan Q1–Q74, terutama keputusan yang berubah selama diskusi.

Keputusan terakhir pengguna mengalahkan rekomendasi awal asisten. Baris yang menyebut rekomendasi ditolak/diganti tidak menjadi fitur aktif. Pengguna sudah mengonfirmasi rangkuman akhir dan meminta PRD lengkap sebelum implementasi.

## Penelusuran seluruh pertanyaan

| Pertanyaan | Keputusan terakhir / klarifikasi | Letak dalam PRD |
|---|---|---|
| Q1 | Masalah utama lupa tugas/deadline dan dokumentasi kurang jelas; reminder perangkat/WA dan delegasi; konteks Reo diminta lalu dibaca | §1, §10–12 |
| Q2 | Pilot Mediatek–Humas/Medhum karena pengguna kadiv Mediatek dan memahami alurnya | §2, §24 |
| Q3 | Desktop wajib bagi pengguna sasaran; awal Medhum, seluruh pengurus setelah fitur divisi siap; web akhirnya masuk tahap awal | §2, OPS-01, §24 |
| Q4 | Keberhasilan terutama mengurangi keterlambatan, termasuk rapat; ping membantu saat bekerja di laptop | §2.2, §12, §15 |
| Q5 | Reo AI agent lama; Space cukup bot. Tiap divisi dapat menghubungkan WA sendiri lewat Baileys | §10–11 |
| Q6 | Tugas minimal hasil yang diminta, satu PJ utama dan deadline; notes/notulensi ditambahkan | §7, §14 |
| Q7 | Rapat lengkap tetap scope, dikerjakan bertahap/per sesi; bukan ditunda seluruhnya | §15, §24 |
| Q8 | Integrasi AI ditolak untuk awal; future plan. Tidak membutuhkan AI gratis/berbayar | BOT-01, §25 |
| Q9 | Nomor khusus maupun pilihan divisi; nama dan style bot bisa dikustomisasi | BOT-02, BOT-05 |
| Q10 | Koneksi dan scheduler di server; bot Humas/Mediatek dapat terpisah; kapasitas perlu pengukuran, tanpa angka jaminan | BOT-03, NFR-01–03 |
| Q11 | Notes menjadi tugas lewat aksi dan pemeriksaan manusia, tanpa ekstraksi AI | NTE-03–05 |
| Q12 | Database aplikasi dan spreadsheet sinkron selama transisi; impor link/fetch berkala; tujuan app-only | §11 |
| Q13 | Command operasional serta custom command divisi, termasuk stiker dan `!damn` | §10.4 |
| Q14 | Rekomendasi satu bot bersama tidak dipakai: bot Humas di Humas+Medhum, bot Mediatek di Mediatek; Humas wajib follow-up H-4 | BOT-04, MED-07–08 |
| Q15 | Kustomisasi ringan nama pesan/bahasa/sapaan/template; command tersimpan mengirim stiker yang ditetapkan | BOT-05, CMD-01–02 |
| Q16 | Kepemilikan per bidang selama sinkronisasi; PJ desain wewenang kadiv Mediatek sesuai SOP | §9.1, §11.1 |
| Q17 | Follow-up H-4 otomatis disetujui mengikuti rekomendasi dan kesepakatan Humas/Litbang; catat waktu kirim | MED-07–08 |
| Q18 | Utamakan mengenali template broadcast Humas, menyelesaikan tahap konsep dan memberi notifikasi ke PJ desain; tidak wajib aksi/ID baru | WA-01–08 |
| Q19 | Temuan otomatis berbasis waktu konsep/desain, tetap perlu tinjauan; tidak diumumkan ke Medhum; mingguan per divisi dan quarter Litbang | §17 |
| Q20 | Auto-fetch Trello untuk konsep/caption selama transisi; hubungan ke spreadsheet | SYN-05, SYN-12 |
| Q21 | Arti broadcast: konfirmasi tahap penyerahan konsep, bukan selesai seluruh tanggung jawab Humas; tercakup dalam rangkuman akhir yang dikonfirmasi | §9.1, WA-03 |
| Q22 | Fokus siapa dan konsep yang ditambahkan; broadcast ulang mempertahankan waktu pertama, tidak mengulang semua notifikasi | WA-02–05 |
| Q23 | Klarifikasi terakhir: tanggal Humas selalu posting, deadline desain Mediatek H-2. Contoh pengurangan berantai sebelumnya sudah diganti Q27 | MED-01–05 |
| Q24 | Tidak wajib ID ketika kirim Drive. Boleh penyerahan sementara perlu tinjauan, satu tugas cocok atau antrean ambigu | WA-09–15 |
| Q25 | Bukti/aturan/tenggat/pihak terkait; keputusan dapat disahkan/dibatalkan dengan alasan; laporan pending/final dibedakan | EVA-01–10 |
| Q26 | Spreadsheet untuk kalender, Trello konsep/caption dan mapping kartu; future app-only tanpa Trello | SYN-01–12, §25 |
| Q27 | Tanggal Humas adalah posting; desain hanya bagi Mediatek. Posting 20 → desain 18, dihitung sekali | MED-01–05 |
| Q28 | Drive sementara dengan konfirmasi bot yang menyebut perlu tinjauan; hindari semua tugas selesai jika ambigu | WA-09–15 |
| Q29 | Web sederhana masuk awal; manfaatkan domain himasif.id | OPS-01, §24 |
| Q30 | Backup database terpisah dan diuji; spreadsheet salinan darurat, perubahan direkonsiliasi | OPS-02–05 |
| Q31 | Akses anggota/kadiv, ruang Medhum, Litbang evaluasi, BPH pemantauan; pekerjaan divisi tetap terpisah | §4, §6 |
| Q32 | Tidak perlu workflow revisi desain. Tinjauan berarti memastikan Drive/hasil cocok ke tugas, bukan approval mutu | TSK-06–08, WA-13, MED-09 |
| Q33 | Ping sekarang default/kustom, terkait tugas atau rapat; sudah lihat/snooze terpisah dari progres | REM-03–04 |
| Q34 | Fokus Windows+web; Go/MyGo desktop; Android berikutnya, bukan harus bersamaan | §2, ARC-01–04, §24 |
| Q35 | `app.himasif.id` sementara; nama brand dicari lebih lanjut | OPS-01, §19 |
| Q36 | Go backend, MyGo frontend web, Baileys Node terpisah; mobile akhirnya Flutter | ARC-01–03 |
| Q37 | Nama lintas periode dengan branding/logo tetap dan maskot default. SIFora bukan pilihan final | §19.2 |
| Q38 | Kriteria Android direvisi menjadi ringan/efisien, bukan cepat coding. Capacitor bukan keputusan final | ARC-03, ARC-08, §21 |
| Q39 | Satu konten bersama menghubungkan tugas Humas dan Mediatek, dengan tanggal/delegasi/PJ detail | §6, §9 |
| Q40 | Repository `himasif-space`, brand Space, maskot Ace; ruang per divisi | §1, §19.2 |
| Q41 | Tidak diwajibkan prototype pembanding; pilihan dari riset/dokumentasi/komunitas, uji implementasi tetap perlu | ARC-08, §21 |
| Q42 | Prioritas baterai idle, nyaman pada HP rendah, ukuran instalasi; mempertimbangkan lintas platform | §21, §20.1 |
| Q43 | Ruang bersama hanya pekerjaan terkait, anggota tetap akses seluruh Space divisinya, tugas sama juga tampil di jobdesk divisi dan pribadi | SPC-01–06 |
| Q44 | Flutter mobile disepakati, Android dulu; iOS kelak dengan lingkungan build macOS lokal/remote | ARC-03–04, §24 |
| Q45 | Anggota melihat jobdesk divisi dan memperbarui tugas sendiri; dapat mengajukan bantuan, kadiv meninjau | SPC-04, TSK-03 |
| Q46 | Reminder pribadi terpisah, privasi/berbagi ke kadiv dapat dipilih | §8 |
| Q47 | Akses otomatis Space divisi serta Medhum bagi seluruh Humas–Mediatek; onboarding/periode dikembangkan | ACC-05, §5 |
| Q48 | Kadiv mengundang, anggota mengaktifkan akun; anggota baru membuat akun saat menerima undangan | ACC-02–04 |
| Q49 | Satu akun lintas periode, jabatan/keanggotaan/akses terpisah per periode | ACC-01, §5 |
| Q50 | Satu akun admin organisasi diteruskan lewat kredensial; bukan admin pribadi banyak orang | ACC-12–13, PER-07 |
| Q51 | Reminder privat default; bisa toggle berbagi kapan saja; default baru tidak membuka reminder lama | PRI-01–05 |
| Q52 | Undangan diikat email; data email calon/pengurus tersedia; link boleh dibagikan WA | ACC-02–04 |
| Q53 | Pemilih periode; pengurus lama baca arsip, pengurus baru perlu izin kadiv; bukan akses otomatis semua arsip | PER-01, PER-03–05 |
| Q54 | Serah terima mencakup pengelolaan dan arsip; tugas tersisa ditutup/diteruskan eksplisit walaupun biasanya sudah selesai | PER-06–09 |
| Q55 | Share reminder ke orang yang dipilih, penerus/45 tidak mewarisi otomatis; kebutuhan pengganti sementara muncul | PRI-03–05, ACC-08–09 |
| Q56 | Role kustom dan subdivisi disepakati. Nama “Plt. Kadiv” ditolak; gunakan **45**, setara operasional kadiv tanpa pengaturan role/akses | ACC-06–11 |
| Q57 | Pendamping berhak operasional; 45 juga boleh mengelola/kustomisasi WA. Role/akses tetap kadiv asli | §4.2, BOT-06 |
| Q58 | 45 tanpa rentang waktu, sampai dicopot. Pilihan kewenangan tersedia, larangan role/akses selalu berlaku | ACC-08–10 |
| Q59 | Kadiv aktif memberi akses arsip divisi; Medhum sesuai ruang bersama, privasi tetap | PER-04–05 |
| Q60 | Maksimal satu pemegang 45 aktif per divisi; pendamping dapat lebih dari satu | ACC-08 |
| Q61 | Bawaan **H-2, H-1, H-0**, bukan hanya H-1/H-0; bisa diubah per tugas/divisi | §12.1 |
| Q62 | Tutup jendela ke tray, pilihan autostart, keluar eksplisit; toast kanan bawah dan bel ringan kustom | DSK-01–05, SND-01–04 |
| Q63 | Rapat lengkap: jadwal/peserta/lokasi/reminder/ping/notes/kehadiran, waktu datang dikonfirmasi penyelenggara | §15 |
| Q64 | Suara/volume pilihan penerima, bel bawaan, file sendiri; satu kejadian satu bel | SND-01–04 |
| Q65 | Overdue harian sampai penyerahan/tutup, dapat diatur; tidak otomatis pelanggaran final | §12, §17 |
| Q66 | Cache offline, progres/notes antre sinkronisasi, indikator tidak ada koneksi dan belum tersimpan cloud | OFF-01–10 |
| Q67 | Ace opsional, pindah posisi, mode tersembunyi/tepi layar dengan hover seperti Wispr Flow, reminder tetap lewat Windows | ACE-01–09 |
| Q68 | Konflik bidang berbeda bisa digabung; bidang sama simpan kedua perubahan dan minta penyelesaian | OFF-05–09 |
| Q69 | Rich notes, template rapat/jadwal/lainnya, simpan template; satu editor, pembaca/komentar, jadikan tugas tanpa AI | NTE-01–09 |
| Q70 | Ace default, pilih/impor gambar transparan atau animasi sederhana; editor penuh lanjutan | ACE-01, ACE-07 |
| Q71 | Bawaan pukul 06.00 WIB, dapat diubah per divisi/tugas; jam deadline berbeda | §12.1, REM-10–11 |
| Q72 | Rapat H-1, satu jam dan 15 menit sebelum; hanya slot relevan jika dibuat mendadak | §12.1, REM-06 |
| Q73 | Tugas organisasi ke perangkat PJ+WA ruang; pribadi hanya perangkat pemilik; grouping dan pilihan kanal ping | REM-02–04, PRI-06 |
| Q74 | Fondasi → pekerjaan → desktop → Medhum/bot → operasional → Flutter; web awal, backup dasar sejak data nyata | §24, implementation-plan.md |

## Koreksi yang tidak boleh hilang saat implementasi

1. **Tanggal Humas = posting.** Penjelasan awal yang terkesan mengurangi 20→18→16 bukan aturan final.
2. **Kadiv Mediatek menetapkan PJ desain.** Ide awal Humas langsung menetapkan bukan kebijakan wewenang final.
3. **Perlu tinjauan hasil = pencocokan bukti.** Jangan membangun workflow revisi wajib dari usulan awal.
4. **Temuan SOP otomatis belum final.** Handoff Reo melarang auto-flag final karena false positive; keputusan baru membolehkan temuan awal dengan tinjauan dan audit.
5. **Bot awal tanpa AI.** Handoff agent bukan alasan menambahkan LLM/API AI.
6. **Bot Humas di Medhum.** Jangan menambahkan bot Mediatek di grup sama dan menggandakan reminder.
7. **45, bukan Plt.** Tanpa kedaluwarsa otomatis; boleh kelola WA, tidak boleh kelola role/akses.
8. **Satu admin organisasi.** Tidak mengganti pilihan pengguna menjadi akun admin per pengurus tanpa keputusan baru.
9. **Semua anggota melihat Space divisi.** Jobdesk saya bukan satu-satunya tempat kerja pengguna.
10. **Arsip perlu izin untuk pengurus baru.** Tidak membuka otomatis seluruh data periode sebelumnya.
11. **Berbagi reminder ke individu.** Jabatan baru tidak mewarisi akses pribadi secara otomatis.
12. **Flutter sudah dipilih.** Riset pembanding tidak berarti masih perlu persetujuan Capacitor/Compose.
13. **Windows+web awal.** Web tidak lagi seluruhnya future plan; Flutter tahap berikutnya.
14. **Brand sudah dipilih.** Space/Ace/himasif-space menggantikan semua kandidat nama terdahulu.
15. **H-2/H-1/H-0 pukul 06.00 WIB.** Jangan membawa bawaan Reo H-1/H-0 atau usulan asisten yang belum dikoreksi.
16. **Overdue harian dan rekap divisi.** Window tujuh hari dan DM pemilik pada Reo bukan kebijakan baru Space.
17. **Tahapan tidak menghapus scope.** Rapat, notes dan template tetap ada; AI/CMS/fitur TBC tetap dicatat sebagai lanjutan.

## Status keputusan vs rincian teknis

Seluruh pertanyaan terpetakan; jawaban yang berkembang dibaca bersama koreksi terakhir dan konfirmasi final pengguna. Parameter teknis yang belum dibahas tercatat sebagai `OPEN-01` sampai `OPEN-15` dalam PRD, bukan disisipkan sebagai keputusan terselubung. Tugas saat ini adalah dokumentasi PRD; implementasi aplikasi belum dimulai.
