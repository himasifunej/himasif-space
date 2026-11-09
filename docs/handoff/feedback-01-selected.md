# Perbaikan desktop/web — pilihan 30 Oktober 2026

Sumber: note f44f7807cd51, Canvas 2a8264b0e3b4 dan feedback pengguna setelah review. Pilihan final: **1A, 2A, 3C, 4A, 5B, 6C, 7C, 8B**. Pengguna meminta implementasi sekarang; tab drawer hanya backlog. Mobile di luar scope. Perubahan produksi/deployment tidak termasuk pekerjaan lokal ini.

## Acceptance yang harus dibuktikan

1. Kalender ringkas tugas: kalender serta jam memiliki desain Space. Rentang mulai–tenggat tersimpan terpisah dari posting. Jam opsional bisa dilewati tanpa membuang tanggal.
2. Form rapat mengikuti drawer tugas. Label terlihat, tanda wajib satu baris dengan label. Kalender/jam bergaya Space; selesai opsional, kosong berarti sampai selesai (tanpa jam buatan). Dukungan mencakup API dan DB.
3. Lajur permanen tetap terlihat ketika kosong, drop mengikuti permission/progress canonical. Pengguna bisa mengganti warna tiap lajur; warna kategori dapat menandai card.
4. Identitas ikon Space dipakai app/installer. Tidak ada panduan/pusat installer di board. Jika ada kanal update yang benar, notifikasi ringkas versi tersedia/sekarang, catatan rilis, nanti, perbarui. Jangan menampilkan update palsu atau mengaktifkan unduhan tanpa kanal yang siap.
5. Header identitas/tim 5B, CTA kontras. Daftar anggota dan pencarian berupa dropdown tepat di bawah pemicu.
6. Drawer mendorong board, transisi halus dan Ctrl+B. Resize melalui hover/drag tepi, tanpa slider. Batas ukuran menjaga konten usable; reduced motion dihormati.
7. Realtime perubahan pengguna lain, reconnect dan refresh aman. Pilihan transport didelegasikan kepada agen; ukur dari implementasi server, bukan judul artifacts. Draft tidak ditimpa; hasil session/context lama dibuang; permission tetap diperiksa. Tidak ada status realtime palsu.
8. Penampilan drawer 8B: mode, palet, aksen, warna khusus dan density konsisten di seluruh UI serta tersimpan di perangkat.

## Backlog terpisah

Tab di atas untuk beberapa drawer (misalnya Penampilan dan Buat tugas), referensi tab enowX. Belum dikerjakan dalam update ini.

## Status

Implementasi desktop/web dilanjutkan pada proyek AWS terisolasi. Kandidat baru: `0.3.0-pilot.5`.

Tindak lanjut note pilot.5 disiapkan untuk kandidat `0.3.0-pilot.6`: pemilih jam reminder, tata letak board dan title bar, halaman pengaturan, serta diagnosis notifikasi perangkat. Verifikasi Windows asli untuk notifikasi dan title bar tetap diperlukan; versi server aktif tidak otomatis berubah dengan pemasangan desktop.

Kandidat `0.3.0-pilot.7` melanjutkan perbaikan board agar kolom turut mengecil saat panel terbuka dan menjaga pesan kegagalan notifikasi tetap terlihat setelah sinkronisasi ulang.

Kandidat `0.3.0-pilot.8` menambahkan label yang terlihat pada isian pengaturan pengingat, WhatsApp, perangkat, dan rekonsiliasi. Verifikasi notifikasi dan title bar pada Windows asli masih diperlukan.

Kandidat `0.3.0-pilot.9` mempertahankan status notifikasi yang belum pasti ketika jurnal perangkat dibaca ulang; status itu tidak lagi dianggap berhasil tanpa bukti.

QA native pada AWS telah lulus: form kalender/jam dan skip, rentang mulai–tenggat terpisah dari posting, palet dengan aksen khusus, Ctrl+B, drag tepi drawer, batas lebar dan reduced motion. Capture berasal dari renderer MyGo yang dipakai bersama desktop/web. Uji realtime membuktikan reconnect, penolakan callback konteks lama dan pelestarian draf. Refresh detail note mempertahankan baseline/draf serta memperbarui komentar dan mendeteksi konflik.

Uji PostgreSQL disposable telah membuktikan selesai rapat nullable, round trip tanggal mulai termasuk penghapusan dan zona waktu campuran, SSE committed/rollback, komentar tanpa bump versi induk, serta pencabutan membership dan sesi. Migrasi tambahan 0022 mencakup child task/note/rapat dan perubahan akun/sesi. Notifikasi pengiriman pribadi tetap memiliki polling 15 detik; jangan mengklaim semua modul memakai SSE.

Paket akhir harus merujuk `verification.json`, source manifest dan SHA256SUMS hasil builder. Pemeriksaan penuh/build serta inspeksi resource ikon dicatat dalam laporan rilis AWS pendamping; checkpoint dokumen ini tidak menggantikan laporan itu. Perilaku tray/toast/instalasi pada Windows tetap memerlukan runtime Windows. Pilot.3 memakai loopback; pilot.4 dan selanjutnya memakai `https://db.tiyoouw.app`. Pilot.5 menampilkan semua divisi/ruang yang bisa diakses melalui daftar sidebar yang dapat digulir. Lihat [backend AWS](../operations/aws-pilot.md) untuk status deployment, terpisah dari aktivasi pengirim/legacy.


## Audit lanjutan setelah unduhan pertama

Pilot.3 memperbaiki benturan tag toast UUIDv7, mengurangi tautan protokol menjadi identitas navigasi, menyamakan shortcut toast dengan installer, mendaftarkan skema URL dalam installer, dan memperbarui autostart ke lokasi executable saat ini. Adapter NSIS pada builder membersihkan registrasi yang masih dimiliki instalasi tersebut saat uninstall, tanpa menghapus preferensi/draf atau registrasi instalasi lain. Bukti tes/build baru tersedia bersama paket pilot.3; pilot.2 tetap dipertahankan. Pemeriksaan Wine dicatat sebagai emulasi, bukan acceptance Windows asli.
