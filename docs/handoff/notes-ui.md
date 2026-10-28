# Notes native — integrasi Sesi 2B

Backend Kiro pada PR #6 sudah digabung ke main `cf8b8d4`, beserta perbaikan privasi target, kunci editor per sesi, draf pemulihan lintas sesi, penghapusan dengan tombstone, validasi ukuran dan pemeriksaan ulang sesi pada perubahan template. UI memakai kontrak di [notes-api-contract.md](notes-api-contract.md).

## Perilaku aplikasi

Notes mengikuti Space dan periode aktif. Daftar catatan di kiri dan isi di kanan memakai MyGo native. Pembuatan catatan menyediakan template rapat, jadwal dan bebas, serta template kustom divisi. Dokumen mendukung judul bagian, paragraf, teks tebal, tautan, daftar, checklist, tabel dan pemisah. Kontrol halaman membatasi elemen yang dipasang tanpa membuang bagian dokumen yang tidak sedang terlihat.

Isi resmi dibaca sampai pengguna mengambil kunci editor. Kunci diperpanjang berkala dan dilepas saat selesai, berpindah ruang, keluar akun atau menutup jendela. Pengambilalihan memerlukan konfirmasi. Simpan resmi memakai versi yang dibaca; konflik mempertahankan draf dan menawarkan perbandingan dengan isi server sebelum pengguna memilih isinya secara eksplisit.

Perubahan disalin ke perangkat setelah jeda 750 ms dan ke draf server setelah 3 detik bila tersedia. Kedua status ini dibedakan dari penyimpanan catatan resmi. Draf lokal dipisahkan berdasarkan akun/note, dibatasi ukurannya, dan ditulis secara atomik pada desktop atau localStorage pada web. Penutupan jendela melakukan flush sinkron agar pengetikan terakhir tidak tertinggal dalam antrean; kegagalan penyimpanan mempertahankan jendela beserta pesan di area Notes. Logout dan respons jaringan lama tidak menerapkan data akun sebelumnya ke akun baru.

Komentar, metadata lampiran, template tersimpan dan riwayat memakai endpoint server. Lampiran tanpa URL diberi status pending; aplikasi belum mengunggah berkas biner. Template disalin saat catatan dibuat, sehingga perubahan template tidak mengubah catatan lama.

Teks/baris yang dipilih dapat mengisi form tugas atau reminder setelah konfirmasi manusia. Tugas tetap membutuhkan PJ, hasil dan tenggat. Pembuatan tujuan dan penautan sumber adalah dua operasi; identitas tujuan yang sudah diakui server disimpan sebagai rujukan tertunda agar percobaan berikutnya hanya menautkan. Jawaban pembuatan yang tidak dapat dipastikan tidak otomatis mengulang pembuatan. Akses note tidak dibagikan otomatis kepada penerima tugas/reminder.

## Penyerahan dan batas

Sesuai permintaan pengguna pada 28 Oktober 2026, pengujian tambahan tidak dijalankan untuk integrasi UI ini. Pengujian backend terarah yang sudah selesai sebelum permintaan tersebut lulus; suite repository yang dimulai sebelumnya tidak menghasilkan laporan penyelesaian lengkap dan tidak diklaim lulus. File pengujian yang sudah ditulis tetap disertakan untuk pemakaian berikutnya.

Build server, Linux desktop, web/WASM dan portable Windows amd64 disiapkan. Runtime Windows dan perilaku UI Notes belum diuji dalam penyerahan ini. Installer Windows memerlukan NSIS yang belum tersedia di host.

Database preview terpisah dicadangkan ke `before-notes.sql` sebelum migrasi 0005/0006 dan pembaruan binary. Tidak ada perubahan produksi, SMTP, Reo, spreadsheet atau WhatsApp. Modul rapat/kehadiran, pengiriman reminder, integrasi WhatsApp, unggah berkas biner dan sinkronisasi offline penuh tetap tahap berikutnya.
