# UI administrasi periode, kadiv, serah terima dan arsip

Basis `main` 0e21591. Melengkapi form Sesi 1B–1C yang belum tersambung pada PR #4, sambil Kiro mengerjakan backend Notes Sesi 2B secara terpisah. UI tetap MyGo native; tidak ada migrasi baru dan `0005_notes.sql` tetap dicadangkan untuk Kiro.

## Perilaku

- Admin organisasi membuka Pengaturan organisasi dari menu profil. Tab Periode menyediakan pembuatan periode dengan tanggal/zona/divisi eksplisit, aktivasi dan penutupan terminal melalui konfirmasi. Aktivasi tidak menutup periode lain; membuat periode tidak menyalin pengurus, role atau pekerjaan. Periode baru tetap dapat dibuat ketika periode terpilih sudah ditutup.
- Tab Kadiv memilih divisi yang benar-benar ada pada periode itu, membaca kepengurusan dan memilih akun pribadi aktif dengan email terverifikasi. Konfirmasi mengikat periode/divisi/akun yang ditinjau; kadiv lama menjadi anggota dan objek privat tidak diwariskan. Jika 45 masih aktif, penolakan server ditampilkan tanpa mengubah daftar secara optimistis.
- Tab Serah terima menyimpan penerus opsional, pengurus masuk, pengelola admin, pengelolaan bot, rencana arsip dan pekerjaan tersisa/alasan tidak ada. Ini checklist, bukan pemindahan tugas atau penutupan otomatis. Rotasi password/email admin tetap pada alur akun yang sudah ada. Refresh memperbarui isian yang belum diedit, sementara isian lokal yang belum disimpan dipertahankan.
- Refresh administrasi juga memperbarui status periode dan Space. Penutupan dari sesi lain, termasuk penolakan `period_closed`, menjadikan formulir hanya baca, membatalkan konfirmasi lama dan mempertahankan draft yang belum tersimpan.
- Pengelolaan izin arsip dibuka dari Arsip periode atau Anggota Space. Hanya kadiv asli/admin pada divisi/periode pemberi yang masih terbuka. Pemilihan satu anggota dan satu Space harus eksplisit; 45, pendamping dan anggota biasa tidak dapat mengatur izin, termasuk lewat pemanggilan controller langsung.
- Izin yang diberikan dapat dicabut lewat konfirmasi. Pencabutan tidak menghapus hak baca independen dari keanggotaan lama. Jika endpoint pilihan target belum tersedia pada server lama, daftar grant masih dapat dibaca/dicabut; pembuatan grant baru menunggu daftar pilihan yang sah.
- Semua perubahan menunggu konfirmasi API. Kegagalan mempertahankan isian atau pilihan. Operasi administrasi memerlukan koneksi; tidak ada antrean offline yang diklaim. Snapshot bearer diambil sebelum goroutine, dan hasil lama dibuang saat periode/divisi/akun berubah, termasuk berpindah A → B → A.
- Dialog, pemilih dan konfirmasi memakai komponen aplikasi. Konfirmasi yang muncul setelah tombol di bagian bawah halaman otomatis digulir agar terlihat satu kali; sesudahnya pengguna bebas menggulir.

## Tambahan kontrak arsip

Kadiv baru tidak selalu mempunyai keanggotaan atau grant pada periode lama. Daftar Space biasa hanya menampilkan yang sudah dapat dibaca, sehingga tidak dapat dipakai untuk memilih calon grant. `GET /api/periods/{p}/divisions/{d}/archives/targets` mengisi kebutuhan pemilihan ini dengan tepat lima bidang metadata, di bawah kewenangan `archives.manage` yang sama dengan pengelolaan grant. Query hanya mengambil Space periode closed yang memuat divisi pemberi. Penelusuran target tidak memberikan akses ke isi, direktori atau reminder pribadi.

## Review dan verifikasi

Standards: 0 temuan. Spec: satu temuan status periode basi setelah penutupan eksternal; diperbaiki dengan refresh metadata/anggota dan penolakan hanya-baca yang mempertahankan draft. Pemeriksaan ulang keduanya: **0 temuan tersisa**.

Pengujian mencakup kontrak HTTP ketat, otorisasi/isolasi metadata arsip dengan PostgreSQL nyata, waktu konfirmasi server, penolakan konflik 45, privasi direktori akun, pemilihan identitas eksplisit, status hanya-baca, draft, respons lama dan keterjangkauan formulir/konfirmasi pada 880×640.

Hasil akhir 28 Oktober 2026:

- **302 hasil tes/subtes unik lulus**, 0 gagal/skip yang tersisa, dari suite PostgreSQL seluruh repository dan pengujian ulang paket UI/client setelah perubahan terakhir. Kesalahan kompilasi fixture akibat embedding `Periods` pada dua interface sudah diperbaiki sebelum pengujian UI terakhir; tidak ada kegagalan paket yang belum diverifikasi ulang.
- Vet Linux, Windows amd64 dan WASM lulus. Build server, Linux native, web MyGo/WASM serta portable Windows amd64 lulus. Runtime Windows belum diuji; installer NSIS belum dibuat pada host Linux ini.
- Uji Linux native/Xpra dengan API nyata: membuat periode uji dalam persiapan, menetapkan kadiv Mediatek dari akun terverifikasi, menyimpan checklist, mengaktifkan lalu menutup periode uji lewat konfirmasi, memilih anggota/Space dari selector, memberi dan mencabut grant arsip. API mengonfirmasi setiap tahap; periode contoh asli tetap aktif.
- Tugas contoh tetap done/verified versi 4 dengan ID sama pada Mediatek/Medhum, tanggal tetap date-only dan waktu bukti asli tidak berubah. Reminder contoh tetap privat, ID/waktu pembuatan/riwayat tetap utuh setelah seluruh uji administrasi.
- Preview API diperbarui setelah backup database; migrasi yang sudah ada diverifikasi tanpa menambah/mengubah SQL migrasi. Untuk verifikasi tampilan langsung dipakai tangkapan drawable X11 native, karena capture cache Xpra dapat tertinggal setelah klien viewer terputus.

Produksi, SMTP, Reo, spreadsheet, WhatsApp dan backend Notes tidak diubah. Pengujian Xpra memakai database preview terpisah dengan data sintetis; periode demo tambahan tidak dipakai untuk mengganti atau menutup periode contoh yang sudah berisi tugas.
