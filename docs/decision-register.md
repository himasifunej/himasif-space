# Register keputusan Space

Tanggal: 27 Oktober 2026, WIB; diperbarui 25 November 2026 (keputusan menuju 1.0.0, lihat bagian terakhir). Dokumen pendamping [PRD](PRD.md). Tujuannya menjaga setiap cabang pembahasan Q1–Q74, terutama keputusan yang berubah selama diskusi.

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
12. ~~**Flutter sudah dipilih.**~~ **Dicabut 25 Nov 2026 (owner, no. 16):** tanpa Flutter. Mobile memakai PWA dari aplikasi web yang sama sambil menunggu MyGo mendukung mobile (ARC-03 direvisi). Lihat K-16.
13. **Windows+web awal.** Web tidak lagi seluruhnya future plan. Tahap mobile berikutnya adalah PWA, bukan Flutter (koreksi 12).
14. **Brand sudah dipilih.** Space/Ace/himasif-space menggantikan semua kandidat nama terdahulu.
15. **H-2/H-1/H-0 pukul 06.00 WIB.** Jangan membawa bawaan Reo H-1/H-0 atau usulan asisten yang belum dikoreksi.
16. **Overdue harian dan rekap divisi.** Window tujuh hari dan DM pemilik pada Reo bukan kebijakan baru Space.
17. **Tahapan tidak menghapus scope.** Rapat, notes dan template tetap ada; AI/CMS/fitur TBC tetap dicatat sebagai lanjutan.
18. **Ace = Dynamic Island (11 Nov 2026, owner).** Maskot bintang dan jendelanya (mengambang/tepi layar, impor PNG/GIF) dihapus; ACE-02–07 di PRD tidak berlaku lagi. Ace hidup sebagai pil bermata di Dynamic Island.
19. **Pemantauan BPH (16 Nov 2026, owner; menutup OPEN-15).** Izin `org.monitor` per akun dari admin untuk BPH (Ketum, Sekretaris, Bendahara; tanpa Waketum), per periode. Rancangan canvas yang menyebut "hanya-baca", "komentar dan lampiran tertutup" dan "isi notulen tidak dibuka" diganti keputusan di bawah. Rinciannya di PRD §4.4 (MON-01–08).

## Keputusan owner 16 November 2026 — OPEN-15 (pemantauan BPH)

| # | Pertanyaan canvas | Keputusan | PRD |
|---|---|---|---|
| 1 | Ketum saja atau seluruh BPH? | Seluruh BPH, diberikan admin per akun; pengganti jabatan tidak mewarisi | MON-01 |
| 2 | Judul saja, atau deskripsi, bukti, komentar? | Lengkap: tugas dengan deskripsi, bukti/lampiran dan komentar. Catatan privat dan reminder pribadi selalu tertutup | MON-02 |
| 3 | Boleh membuka notulensi divisi lain? | Boleh | MON-03 |
| 4 | Divisi diberi tahu saat dibuka? | Tidak (privat); setiap akses tetap masuk log audit yang dilihat admin | MON-04 |
| 5 | Boleh bertindak? | Boleh: ping kadiv dan menyetujui/meninjau di tempat kadiv dapat; tercatat sebagai tindakan pemantau | MON-05 |
| 6 | Berlaku per periode? | Ya, dicabut otomatis saat periode ditutup | MON-06 |

Penerapan teknis: [ADR 0002](adr/0002-pemantauan-bph.md).

Tambahan 25 Nov 2026 (K-09): pemantau juga melihat **NIM** anggota semua divisi yang dipantaunya, dan akun admin organisasi bersama boleh **membaca** isi tugas dan rapat semua divisi. Keduanya sudah perilaku kode; keputusan ini mengesahkannya.

## Status keputusan vs rincian teknis

Seluruh pertanyaan terpetakan; jawaban yang berkembang dibaca bersama koreksi terakhir dan konfirmasi final pengguna. Parameter teknis yang belum dibahas tercatat sebagai `OPEN-01` sampai `OPEN-16` dalam PRD (OPEN-15 ditutup 16 Nov 2026), bukan disisipkan sebagai keputusan terselubung. Paragraf ini ditulis sebelum implementasi (27 Okt 2026); keadaan terbaru ada di bagian di bawah.

## Keputusan owner 25 November 2026 — menuju 1.0.0

Sumber: note enowX "Audit Space" (`5bda2328ea62`) bagian "Keputusan owner (25 Nov)", "putaran kedua" dan "nomor bot", serta rencana kerja (`8d4b128e8a55`). Nomor K-xx mengikuti nomor pertanyaan owner; huruf R menandai putaran kedua dan lanjutan. Baris berlabel **ikut rekomendasi** diputuskan owner dengan menerima rekomendasi yang ditulis di note. Kode yang belum mengikuti keputusan disebut di kolom "Item"; keputusan tidak berarti fiturnya sudah ada.

### Arah rilis

| # | Keputusan | Item | Acuan |
|---|---|---|---|
| K-01 | **Stabil** = rilis yang siap dipakai Mediatek, Humas dan BPH, berjalan penuh dan ramah pengguna awal, dengan alur Medhum jalan lintas divisi, sehingga anggotanya bisa diundang. Urutan uji: Mediatek dulu, baru Humas, alur Medhum, dan BPH | F1-24, F1-19, F1-26 | PRD §2.3 |
| R-01 | **Stabil = 1.0.0**, terbit sebelum pengurus baru mulai, **paling lambat Senin 7 Des 2026**. Nomor versi selama pengembangan bebas (mis. 0.4.6-rc.10, 1.0.0-rc.1). Jadwal: 1.0.0-rc.1 Rab 2 Des, 1.0.0 Sab 5 Des | F1-13 | PRD §2.3, §24.1 |
| K-01a | **Medhum bukan divisi** (koreksi owner). Medhum adalah alur bersama Mediatek + Humas: Humas merencanakan konten (broadcast) → tugas desain ke PIC Mediatek dengan Deadline → ACC bersama → posting. Space bersama `medhum` tetap model datanya | F1-28 | PRD §3, §9 |
| K-02 | **Pergantian periode tidak dijalankan di Space.** Periode 2025/2026 dipakai untuk uji pengguna; pergantian pengurus paling lambat 7 Des 2026 | F0-16 | PRD §5 |
| R-02 | **DB stabil mulai kosong** dengan periode 2026/2027; penguji diundang ulang. Data DB uji tidak dipindah | F1-22, F0-16 | PRD §18 OPS-11 |
| R-03 | **`tiyoouw.app` dipertahankan sebagai instans uji** (`space.tiyoouw.app`). Akun demo tetap di sana dan **tidak pernah** ada di instans stabil | F1-22, F0-01 | PRD §18 OPS-11, [ADR 0003](adr/0003-instans-dan-sesi-1-0.md) |
| K-03 | Akun demo pilot belum dirotasi. Risiko dicatat; penutupannya syarat sebelum DB stabil dibuka (tidak ada akun demo, data `[DUMMY]` atau kredensial yang pernah ter-commit di instans stabil) | F0-01, F1-22 | — |
| K-18 | **Domain stabil `space.himasif.id`**, sementara tetap di tc-vps; migrasi server tahun depan. Menggantikan `app.himasif.id` (Q35) | F1-21 | PRD OPS-01 |
| R-04 | **DNS himasif.id tetap di IDCloudHost** (akun owner), tidak dipindah ke Cloudflare. Owner menambah record manual lewat cPanel: `CNAME space`, `TXT _cf-custom-hostname.space`, `TXT _acme-challenge.space` (jalur Cloudflare for SaaS) | F1-21 | ADR 0003 |

### Medhum (putaran kedua)

| # | Keputusan | Item | Acuan |
|---|---|---|---|
| R-05 | **Deadline desain = tanggal posting − 2 hari.** Tanggal dalam broadcast Humas adalah tanggal posting. Sama dengan kode sekarang; menegaskan koreksi 1 dan MED-01–02 | — | PRD §9.2 |
| R-06 | **PIC Mediatek ditetapkan kadiv Mediatek di awal bulan**; broadcast mengikuti penetapan itu. `pj med` di broadcast dibaca untuk dicocokkan, bukan untuk menetapkan. Sama dengan kode sekarang; menegaskan koreksi 2 | F1-06, F1-28 | PRD §9.1, WA-16 |
| R-07 | **ACC desain bersama** (ikut rekomendasi): satu ACC cukup dari salah satu: PJ Humas konten itu, pengelola Humas (kadiv/45), atau pengelola Mediatek (kadiv/45, pemilik tugas). PIC desain tidak boleh meng-ACC karyanya sendiri. "Minta perbaikan" boleh dari orang yang sama dan wajib dengan catatan. Jejak: siapa, dari sisi mana, dan kapan; tampil di konten dan tercatat di riwayat tugas dan audit. Konten baru bisa ditandai diposting setelah ACC. **Mengubah koreksi 3** hanya untuk tugas desain Medhum: di sana tinjauan adalah ACC bersama; tugas lain tetap pencocokan bukti oleh pengelola | F1-28 | PRD §9.4 MED-10–14 |
| R-08 | **Reo tetap jalan.** Pindah penuh ke Space tidak mendesak (awalnya Reo adalah Hermes milik owner) | F2-16 | PRD §11.2 |

### Nomor bot (25 Nov, lanjutan; menggantikan rekomendasi sebelumnya untuk F1-19/F1-22)

| # | Keputusan | Item | Acuan |
|---|---|---|---|
| R-09 | Bot Space **memakai nomor Reo** (Reo = Hermes milik owner). Selama Reo jalan, dua bridge (Hermes dan Space) tertaut ke nomor yang sama sebagai perangkat tertaut (multi-device) | F1-19, F1-22 | PRD BOT-12, ADR 0003 |
| R-10 | Untuk uji sudah ada **grup Test**; Reo/Hermes sudah dilarang merespons di grup itu | F1-19 | PRD BOT-12 |
| R-11 | Keduanya wajib memakai **allowlist grup**: Space hanya merespons di grup Test, lalu di grup yang dipindahkan; Reo tidak merespons di grup yang sudah dipindah ke Space. **Cutover per grup**, bukan sekaligus. Dua sistem tidak pernah melayani satu grup bersamaan (OPS-08) | F1-19, F2-16 | PRD BOT-12–13 |

### Operasi dan keamanan

| # | Keputusan | Item | Acuan |
|---|---|---|---|
| K-05 | Pemegang akses server, vault dan rilis sekarang hanya owner; diserahkan ke Rahardan atau kadiv Mediatek baru setelah live | F3-08, F0-12 | — |
| K-06 | **Alarm lewat email** ke owner, lalu Rahardan dan kadiv baru: API mati, backup gagal, disk hampir penuh, bot WA perlu dipasangkan ulang | F0-13 | PRD OPS-13 |
| K-07 | Repo GitHub boleh diberi aturan `main`, Dependabot dan secret scanning | F0-02 | — |
| K-09 | **Privasi:** pemantau BPH boleh melihat NIM semua divisi yang dipantaunya; akun admin organisasi bersama boleh membaca isi tugas dan rapat. Kode tidak berubah. Rekomendasi: baca oleh admin diaudit kelak (F2-09), karena akunnya dipakai bergantian | F0-08 | PRD MON-09, ACC-14; [ADR 0002](adr/0002-pemantauan-bph.md) |
| K-10 | **Retensi pesan WA masuk: 1 hari**, untuk mengejar Command yang terlewat saat server down. Bila tertinggal banyak, bot mengirim satu pesan maaf dan meminta kirim ulang | F1-11 (1.0.x) | PRD OPS-12 |
| R-12 | **RPO 24 jam** (backup harian 02.00 WIB, ditambah backup sebelum tiap deploy dan migrasi); **RTO 4 jam** sejak owner tahu, antara 07.00–22.00 WIB (di luar itu dimulai pagi berikutnya); **retensi 14 harian + 8 mingguan** di luar tc-vps. Sesi WA tidak termasuk RTO (pairing ulang butuh pemegang HP bot). Bila gladi melebihi 4 jam, langkahnya yang diperbaiki, bukan angkanya. Menutup OPEN-11 | F0-11 | PRD OPS-05 |
| K-08 | Izin pemantau saat diganti di tengah periode tetap berlaku sampai admin mencabut; karena BPH berganti bersama periode (izin dicabut otomatis), cukup jaring pengaman berprioritas rendah (ikut rekomendasi) | F0-05 | PRD MON-06 |

### Produk

| # | Keputusan | Item | Acuan |
|---|---|---|---|
| K-13 | **Istilah baku:** "Hasil yang diharapkan", "Reminder", "Deadline", "Command", "Ace (Dynamic Island)", "Notulensi" untuk hasil catatan rapat, "notulen" untuk orang yang mencatat. Penggantian di seluruh aplikasi menyusul (1.0.x); dokumen baru memakai istilah ini | F1-08 | PRD §3 (istilah baku) |
| K-14 | **BPH untuk 1.0.0:** kontrol sederhana dan rapat: Pemantauan organisasi, Rapat BPH dan Rapat pengurus, ringkasan mingguan, ping kadiv. Paket BPH lanjutan menjadi future upgrade | F1-26 | PRD §25 |
| K-15 | **Kebutuhan divisi** digali lewat lima form grill-me (Mediatek, Humas, gabungan Medhum, BPH, Litbang); jawaban tidak menahan 1.0.0. Paling banyak satu modul baru per rilis minor. **Space fokus Reminder dan tidak berperan terhadap CMS** | F2-12 | PRD §25 |
| K-16 | **Mobile tanpa Flutter.** PWA dari aplikasi web yang sama sambil menunggu MyGo mendukung mobile. Mencabut koreksi 12 dan merevisi ARC-03 | F1-20, F2-14 | PRD ARC-03, ADR 0003 |
| K-17 | **Litbang belum ikut pilot.** Arahnya deteksi otomatis tanpa manusia (kelak). SOP follow-up H-4 digantikan Reminder Space | F1-25, F2-15 | PRD §17 |
| R-13 | **Menu SOP disembunyikan sampai Litbang ikut, tetapi disiapkan:** menu Evaluasi SOP, follow-up H-4 dan rekap SOP tidak tampil dan tidak berjalan bila divisi tidak punya kebijakan SOP aktif; admin tetap bisa mengaturnya tanpa rilis baru | F1-25 | PRD §17 EVA-11 |
| K-04 | **Template broadcast Humas** dari jawaban owner menjadi acuan parser: `N. <konsep>, pj humas @nama, pj med @nama (19 nov) <catatan>`, judul bulan dalam kurung siku, bagian `DADAKAN`, catatan `tanggal = DL` | F1-06 | PRD §10.2 WA-16 |
| K-11 | **"Tetap masuk di perangkat ini"** di web (ikut rekomendasi): pilihan di layar masuk, bawaan mati; token disimpan di `localStorage` hanya bila dicentang; keluar akun menghapusnya. Merevisi baris penyimpanan token web di ADR 0001 | F2-01, F0-06 | ADR 0003 |
| K-12 | **Nomor tugas di Command WA** (ikut rekomendasi): `!done T12` dan `#T12` diterima; UUID lama tetap diterima. Untuk 1.0.0 cukup `!done` dan `!cek`. Catatan "tidak dulu" di canvas A tahap 6 dicabut | F2-02 | PRD CMD-07 |
| K-19 | **Authenticode belum** (ikut rekomendasi). Peringatan SmartScreen saat pemasangan pertama diterima, asal panduan pasang jelas; ditinjau lagi sesudah 1.0.0 | F1-23 | PRD OPS-09 |
| K-20 | **Note lama ditandai arsip** dengan tautan ke `docs/`, bukan disinkronkan. Repo adalah sumber kebenaran (ikut rekomendasi) | F1-27 | — |

### Yang masih menunggu konfirmasi owner

Rekomendasinya tertulis di note rencana; sampai dijawab, perlakukan sebagai **belum diputuskan**:

- Cloudflare for SaaS tersedia di akun Cloudflare owner? Bila tidak, jalur cadangan `A` ke IP tc-vps butuh keputusan (F1-21).
- Instans mana yang menautkan nomor bot (R-09) dan apakah instans uji berjalan tanpa bridge setelah stabil dibuka (F1-22).
- Kesediaan meninjau ±3 PR per hari dan menjalankan uji 3–4 Des; bila tidak, pemangkasan sesuai urutan di note rencana.

Rekomendasi ACC bersama (R-07) dan "bot Space tidak masuk grup yang dilayani Reo" sudah tercakup oleh R-07 dan R-11.
