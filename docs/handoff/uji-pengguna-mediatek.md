# Skrip uji pengguna Mediatek (F1-24)

Tanggal: 25 November 2026, WIB. Item rencana: **F1-24** (keputusan owner no. 1 dan 2: Mediatek diuji dulu, baru Humas, alur Medhum dan BPH). Acuan: [PRD](../PRD.md) §2.3 REL-01–06, [Register](../decision-register.md) bagian 25 Nov 2026.

## Ringkas

| | |
|---|---|
| Tujuan | Membuktikan anggota Mediatek bisa memakai Space **tanpa dibimbing**: dari undangan sampai menyerahkan tugas, ikut rapat, menerima Reminder dan memakai Command WA |
| Penguji | 2 anggota Mediatek (+ owner sebagai kadiv yang menyiapkan data dan mengamati) |
| Waktu | ±30 menit per penguji |
| Putaran 1 | Jum 27 – Sab 28 Nov 2026 di pilot sekarang (0.4.6-rc.9, `space.tiyoouw.app`) |
| Putaran 2 | Kam 3 Des 2026, singkat, di 1.0.0-rc.1 (instans uji) |
| Masukan | Satu note enowX untuk masukan penguji (dibuat owner); tiap langkah gagal menjadi to-do di note rencana |

Istilah dalam skrip ini mengikuti istilah baku (K-13): **Hasil yang diharapkan**, **Reminder**, **Deadline**, **Command**, **Ace (Dynamic Island)**, **Notulensi**, **notulen**. Aplikasi rc.9 masih memakai sebagian istilah lama (mis. "Hasil yang diminta", "tenggat", "pengingat"); itu bukan kegagalan uji, tetapi catat bila penguji bingung karenanya.

## Aturan pengamatan

- **Owner tidak membantu** selama penguji mengerjakan langkah. Boleh menjawab hanya setelah penguji menyerah atau melewati batas waktu langkah; langkah itu lalu ditandai **gagal**.
- Penguji membaca langkah dari daftar ini, bukan dari penjelasan lisan.
- Penguji berpikir keras ("saya cari tombol…") dan owner mencatat kalimat yang menunjukkan bingung.
- Catat perangkat: Windows desktop, web di PC, atau web di HP (browser apa).
- Jangan memakai data asli orang lain. Tugas dan rapat dibuat khusus untuk uji, judul diawali `[UJI]`.
- Tidak ada WA ke grup selain grup uji. Command dicoba di **grup Test** (Reo tidak merespons di sana) atau chat pribadi dengan bot.

## Persiapan owner (sebelum penguji datang)

1. Pastikan penguji punya undangan Mediatek yang belum diterima (putaran 2: undangan baru di instans uji, karena DB stabil kosong dan penguji diundang ulang).
2. Siapkan untuk tiap penguji: satu tugas `[UJI] Desain poster contoh` dengan penguji sebagai PJ utama, Hasil yang diharapkan jelas, Deadline besok, dan Reminder bawaan.
3. Siapkan satu rapat `[UJI] Rapat koordinasi` besok dengan kedua penguji sebagai peserta; tunjuk satu penguji sebagai notulen.
4. Pastikan bot WA aktif dan grup Test terpetakan ke Space Mediatek.
5. Siapkan note masukan dan stopwatch.

## Langkah

Tandai tiap langkah **Lolos / Gagal / Lolos dengan catatan**, dan tulis waktu yang dibutuhkan.

| # | Langkah untuk penguji | Batas | Lolos bila |
|---|---|---|---|
| 1 | Buka tautan undangan dari WA/email, buat akun (atau masuk bila sudah punya), dan terima undangan Mediatek | 4 menit | Penguji masuk dan melihat Space Mediatek di sidebar |
| 2 | Pasang aplikasi Windows dari `/unduh/windows` *(hanya bila penguji memakai PC Windows)* | 5 menit | Aplikasi terbuka dan sudah masuk; peringatan SmartScreen bisa dilewati dari panduan (K-19) |
| 3 | Tautkan nomor WhatsApp-mu ke Space | 3 menit | Status *Tertaut*; pesan uji dari bot sampai di chat pribadi |
| 4 | Temukan tugas yang ditujukan kepadamu dan sebutkan Hasil yang diharapkan serta Deadline-nya | 2 menit | Penguji menemukan tugas `[UJI]` lewat Untukku atau Board tugas dan menyebut keduanya dengan benar |
| 5 | Mulai kerjakan tugas itu (ubah statusnya), lalu tulis satu komentar yang menandai rekanmu dengan @ | 3 menit | Status berubah; rekan menerima kabar tandai |
| 6 | Serahkan hasil tugas itu dengan satu tautan Drive (boleh tautan folder kosong) | 3 menit | Status "menunggu tinjauan"; penguji paham bahwa tugas selesai setelah kadiv mencocokkan |
| 7 | Buka rapat besok dan jawab apakah kamu hadir | 2 menit | Jawaban kehadiran tersimpan dan terlihat oleh penyelenggara |
| 8 | *(Penguji yang ditunjuk notulen)* Tulis Notulensi singkat rapat itu dan simpan | 4 menit | Notulensi tersimpan dan bisa dibaca penguji lain |
| 9 | Buat satu Reminder pribadi untuk dirimu sendiri 5 menit lagi | 2 menit | Reminder tersimpan; notifikasi muncul di perangkat (dan/atau WA) saat waktunya |
| 10 | Di grup Test, kirim Command untuk melihat tugasmu, lalu Command untuk melihat detail satu tugas | 3 menit | Bot membalas daftar tugas (`!tugas`) dan detail (`!cek …`) dengan data yang sama dengan aplikasi |
| 11 | Buka tautan tugas yang dikirim bot dari HP | 2 menit | Tautan membuka tugas yang benar (catat bila berakhir di layar masuk: itu yang diperbaiki "Tetap masuk", F2-01) |

Setelah langkah 6, owner sebagai kadiv meninjau dan menyetujui penyerahan; penguji memastikan statusnya menjadi selesai dan kabarnya sampai.

## Pertanyaan penutup (2 menit)

1. Bagian mana yang paling membingungkan?
2. Apa yang kamu cari tapi tidak ketemu?
3. Kalau besok tugas sungguhan masuk ke Space, apa yang membuatmu tetap membuka spreadsheet atau grup WA?
4. Nilai 1–5: "Saya bisa memakai Space tanpa diajari."

## Kriteria lolos

- Langkah **1, 3, 4, 6, 7 dan 10 lolos** untuk kedua penguji (inti alur anggota Mediatek).
- Tidak ada langkah yang gagal karena **data salah** (tugas, rapat atau status berbeda antara aplikasi, web dan bot).
- Rata-rata nilai pertanyaan 4 **≥ 4**.
- Setiap langkah gagal dan setiap kebingungan berulang dicatat sebagai to-do di note rencana, dengan perangkat dan versi.

Putaran 2 hanya mengulang langkah yang gagal di putaran 1 ditambah langkah 1, 3 dan 11 (instans dan akun baru).

## Lembar hasil

| # | Penguji A | Waktu | Penguji B | Waktu | Catatan |
|---|---|---|---|---|---|
| 1 | | | | | |
| 2 | | | | | |
| 3 | | | | | |
| 4 | | | | | |
| 5 | | | | | |
| 6 | | | | | |
| 7 | | | | | |
| 8 | | | | | |
| 9 | | | | | |
| 10 | | | | | |
| 11 | | | | | |

Perangkat A: … · Perangkat B: … · Versi: … · Tanggal: …
