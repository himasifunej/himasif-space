# QA 1.0.0-rc.2 di instans uji (27 Nov 2026)

Uji ulang sepuluh temuan QA 1.0.0-rc.1 dan regresi singkat, di `space.tiyoouw.app` setelah rc.2 dipasang (build `84c264c`, migrasi `0039` diterapkan otomatis). Playwright Chromium (bukan msedge), desktop 1440×900 dan HP 390×844. Akun: empat akun `[DUMMY]` showcase dan dua akun uji baru (lihat [skrip uji Mediatek](uji-pengguna-mediatek.md#akun-uji-di-instans-uji-qa-dan-gladi)); kata sandi dimuat ke memori dari berkas operasional, tidak dicetak.

## Hasil per temuan

| # | Temuan rc.1 | Hasil rc.2 |
|---|---|---|
| 1 | ACC bersama belum ada (F1-28) | **Diperbaiki.** PJ Humas, kadiv Humas dan 45 Mediatek bisa ACC/minta perbaikan; PIC dan anggota biasa ditolak (403); minta perbaikan tanpa catatan 400; sisi tercatat dan tampil "ACC oleh … · Humas" di konten dan bukti; ditandai diposting sebelum ACC 409, sesudah 200; PJ Humas dan PIC dikabari; kadiv Humas tetap tidak bisa meninjau tugas Mediatek lain. Broadcast yang membuat konten sendiri: 1.0.1 (urutan pemangkasan no. 4) |
| 2 | Hasil parser baru tidak tampil | **Diperbaiki.** Pratinjau menampilkan tanggal posting, Deadline desain H-2, PJ (dengan nama panggilan), warna dan nomor baris |
| 3 | Tab Anggota Medhum hanya 2 dari 6 | **Diperbaiki.** Tabel menyebut divisinya, tombol kelola bernama divisi, "5 orang lain di Medhum" di bawahnya (2 + 5 = 7, sama dengan header) |
| 4 | "DIVISI MEDHUM" di Pengaturan | **Diperbaiki.** "Space Medhum (bersama)" |
| 5 | Web menulis "Aplikasi Windows" | **Diperbaiki.** "Aplikasi web · Diperbarui bersama halaman web", tanpa tombol pembaruan |
| 6 | HP 390 terpotong | **Diperbaiki.** + Pengingat turun ke baris baru, judul tugas Untukku utuh dua baris, kartu Medhum dua kolom, sub-tab membungkus. Perbaikan pertama untuk judul Untukku masih salah (wadah kosong mengambil separuh lebar); ditemukan saat QA ini, diperbaiki dan dipasang ulang |
| 7 | Tab "Bot WhatsApp" tertutup, placeholder Ctrl K terpotong | **Diperbaiki.** Semua tab Medhum tampil, Atur tab menjadi ikon saat sempit; placeholder "Cari apa saja" |
| 8 | "Offline" sesaat padahal server 200 | **Diperbaiki.** Selama QA, CPU host penuh sampai satu permintaan habis waktu; status tetap "Tersinkron" dan tidak pernah menampilkan Offline |
| 9 | `/api/deliveries` 5–6 detik | **Diperbaiki.** 0,30–0,42 detik untuk enam akun (3 kali tiap akun); tidak ada permintaan > 3 detik di log server |
| 10 | Periode uji berakhir 30 Nov | **Diperiksa, tanpa perubahan kode.** Server tidak memakai tanggal akhir untuk Reminder, ACC atau akses; periode tetap berjalan selama statusnya aktif. Hanya label "minggu ke-N" yang hilang setelah 30 Nov |
| 11 | Tidak ada akun kadiv Mediatek/BPH | **Dibuat.** `[DUMMY] Uji Pengelola Mediatek` (45, karena kadiv Mediatek adalah akun owner) dan `[DUMMY] Uji BPH` (izin pemantauan); BPH membuka Pemantauan organisasi dan rapat pemantauan (200), anggota biasa 403 |

## Angka

- Uji API: **30/30** lulus (`login`, `/api/health` 1.0.0-rc.2, alur ACC bersama dua sisi, tugas non-Medhum, BPH, waktu `/api/deliveries`, pratinjau parser).
- Uji layar: **15/15** setelah perbaikan Untukku HP (10 desktop, 5 HP), satu gagal di putaran pertama.
- Log server selama QA: 884 jawaban 2xx, 9 jawaban 4xx (penolakan yang memang diuji), **0 jawaban 5xx**, 0 permintaan > 3 detik.
- Console: kosong di desktop; satu 502 di HP tepat saat server dimulai ulang untuk pemasangan kedua.
- Regresi: login desktop dan HP, Untukku, Medhum, Board tugas dan detail tugas, Anggota, Pengaturan, Tentang, Ctrl K (menu SOP hanya di Pengaturan), notifikasi, Lainnya di HP (tanpa SOP).

## Data uji yang tertinggal

- Dua konten `[UJI] … (QA rc.2)` di Medhum: satu terposting, satu di-ACC. Server tidak punya fitur hapus konten; tugas Medhum biasa `[UJI]` sudah dibatalkan.
- Tidak ada WA terkirim: Medhum tidak punya grup WA, jadi kabar WA tercatat "belum terkirim".
- Sesi login API dan HP sudah di-logout; satu sesi browser desktop akun `[DUMMY] Tara Humas` tidak bisa di-logout dari skrip dan habis dengan sendirinya.

## Catatan kecil (belum diubah)

- Hasil yang diharapkan tugas desain Medhum masih berbunyi "Serahkan hasil desain untuk dicocokkan kadiv"; sejak ACC bersama, peninjaunya bisa Humas juga. Ikut penggantian istilah F1-08.
- Waktu "Publikasi" diisi pengguna dan boleh lebih awal dari waktu ACC; server hanya memeriksa bahwa ACC sudah ada saat ditandai diposting.
