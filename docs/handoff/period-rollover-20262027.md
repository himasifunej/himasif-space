# Uji pergantian periode 2025/2026 → 2026/2027 (hari H ~6 Des 2026)

Diuji 9 Nov 2026 **hanya di Postgres 16 lokal** (Windows, `E:\himasif-cache\pg`), bukan pilot/production.
Skenario otomatis: `internal/server/period_rollover_test.go` (`TestPeriodRollover20262027`, `TestPeriodsMarkMembership`).

## Yang terjadi (terbukti oleh test)
| Langkah | Hasil |
|---|---|
| Buat periode 2026/2027 (status persiapan) | Struktur divisi + Space (termasuk Medhum) dibuat. Anggota, role dan tugas **tidak** disalin (PER-02). Pengurus lama tidak melihat periode baru. |
| Aktifkan 2026/2027 | **Dua periode aktif sekaligus** sampai yang lama ditutup (PER-09). Periode lama tetap bisa ditulis selama tumpang tindih. `/api/periods` mengurutkan yang baru lebih dulu. |
| Undang pengurus baru | Akun lama diundang ke periode baru tetap memakai akun yang sama (ACC-01) dan bisa diangkat jadi kadiv. Nomor tugas di Space baru mulai dari #1. Jobdesk periode baru tidak memuat tugas periode lama. |
| Serah terima + tutup 2025/2026 | Checklist serah terima tersimpan. Setelah ditutup, 3 reminder tugas lama yang masih antre dibatalkan scheduler (PER-08). Penutupan **final**: aktifkan ulang ditolak 409. |
| Tugas tersisa | Tetap `todo` dengan PJ lama, hanya-baca, tidak muncul di board baru. **Tidak ada fitur meneruskan tugas** (PER-06 belum dibuat): harus dibuat ulang manual di periode baru. |
| Akses lama | Kadiv lama dan anggota yang keluar tetap membaca Space lama (hanya-baca, PER-03) dan tidak melihat periode baru. |
| Akses baru | Anggota baru tidak melihat arsip sampai kadiv baru memberi akses arsip; akses hanya untuk Space yang diberikan (PER-04). |
| Worker | Scheduler, SOP, konektor, antrean WA dan perintah WA masuk semuanya berhenti untuk periode tertutup (cek kode: `p.status<>'closed'`). |

## Bug yang ditemukan dan sudah diperbaiki di branch ini
**Pengurus lama mendarat di periode kosong.** `defaultPeriod` di aplikasi memilih periode aktif pertama untuk semua akun. Begitu admin mengaktifkan 2026/2027, pengurus 2025/2026 yang membuka atau memuat ulang aplikasi masuk ke periode tanpa Space (Beranda dan Jobdesk kosong). Ini terjadi selama tumpang tindih dan juga sesudah penutupan.

Perbaikannya:
- `GET /api/periods` menandai `member: true` pada periode tempat akun punya keanggotaan aktif.
- Aplikasi memilih periode miliknya dengan urutan aktif, lalu persiapan, lalu ditutup; kalau tidak ada, kembali ke perilaku lama.
- Perubahannya aditif: server lama tidak mengirim penanda itu dan aplikasi berperilaku seperti dulu. Tidak ada migrasi.

## Risiko / tidak terbawa ke periode baru (harus disiapkan ulang)
- **Bot WhatsApp per periode**: `wa_connections`, grup, perintah kustom, template dan `wa_identities` terikat ke periode. Di 2026/2027 bot harus dipasangkan ulang lewat QR (namespace sesi baru), grup dipilih lagi, dan **setiap anggota menautkan nomornya lagi** (status WA di halaman Anggota akan kosong). Koneksi lama tetap `enabled` dan sesinya tetap memakai satu slot perangkat tertaut; matikan setelah penutupan.
- Role kustom, kebijakan notifikasi, kebijakan SOP + evaluator, konektor Drive/Medhum, dan template catatan per divisi tidak disalin.
- Urutan tab Space per akun (0025) dikunci per `spaceId`, jadi kembali ke bawaan di Space baru.
- Penutupan tidak memeriksa apakah serah terima sudah diisi, dan tidak bisa dibatalkan. Salah tutup berarti restore DB.
- Tanggal: 2025/2026 berakhir 30 Nov, tetapi pergantian ~6 Des. Tidak ada penutupan otomatis, jadi aman, tetapi tanggal periode baru sebaiknya mulai 1 Des.

## Checklist hari H (admin, berurutan)
1. H-1: backup `pg_dump -Fc` + tar lampiran, cek `pg_restore -l`. Deploy rilis yang memuat perbaikan `member` sebelum hari H.
2. Buat 2026/2027 (1 Des 2026 – 30 Nov 2027, Asia/Jakarta, semua divisi). Jangan aktifkan dulu.
3. Undang kadiv baru per divisi lalu angkat jadi kadiv. Kadiv baru mengundang anggotanya.
4. Kadiv baru: role kustom, kebijakan notifikasi, SOP + evaluator, konektor, pasang bot WA (QR, grup, anggota `!link`).
5. Aktifkan 2026/2027.
6. Inventaris tugas `todo`/`doing` 2025/2026 per divisi. Buat ulang yang diteruskan di periode baru dengan PJ baru, dan sebut asalnya (#T lama) di deskripsi.
7. Isi serah terima 2025/2026: pengurus baru, admin, bot, rencana arsip, pekerjaan tersisa.
8. Kadiv baru memberi akses arsip yang diperlukan.
9. Matikan koneksi bot WA 2025/2026, lalu **tutup 2025/2026** (final).
10. Serah terima akun admin organisasi (ganti kredensial/pemulihan, akhiri sesi lama).
11. Cek: login sebagai satu pengurus lama, satu pengurus lanjut dan satu anggota baru, lalu pastikan periode yang terbuka dan Space yang terlihat sudah benar.

## Usulan lanjutan (belum dibuat)
- Fitur "Teruskan tugas ke periode baru" (PER-06): salin dengan PJ baru dan tautan ke tugas asal.
- Penutupan periode menolak bila serah terima kosong, dan menampilkan ringkasan tugas tersisa di dialog konfirmasi.
- Menutup periode ikut mematikan koneksi WA-nya.
