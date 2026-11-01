# Panduan uji mandiri — Sesi 2C dan Tahap 3

**Belum dijalankan oleh agen.** Semua kotak sengaja kosong. Implementasi versi 0.2.0 diserahkan untuk kamu uji sendiri, sesuai permintaan tanpa build/verifikasi/pengujian oleh agen.

## Menyiapkan lingkungan

Gunakan PostgreSQL dan env pengembangan yang terpisah dari produksi. Simpan salinan database dev sebelum menerapkan migrasi baru. Contoh perintah ini untuk dijalankan pengguna; tidak dijalankan dalam sesi implementasi:

```bash
# Pastikan Go 1.27+ dan PostgreSQL tersedia serta dev.env sudah diisi.
make migrate
make serve

# Pada terminal lain, untuk aplikasi native terhadap API lokal:
SPACE_API_URL=http://localhost:8787 go tool mygo dev ./app

# Web:
make web
# Server menyajikan app/build/web di http://localhost:8787
```

Pada PowerShell Windows:

```powershell
$env:SPACE_API_URL = "http://localhost:8787"
go tool mygo dev ./app
```

API localhost berarti API pada komputer tempat aplikasi berjalan. Jika API berada pada host lain, gunakan URL pengembangan yang dapat dijangkau dari komputer Windows. enowX meneruskan port sesi bila preview dijalankan di dalam sesi. Distribusi/installer/signing tetap tahap operasional; pengujian autostart sebaiknya memakai executable di lokasi tetap.

Siapkan minimal akun kadiv Mediatek, anggota/PJ, kadiv/anggota Humas dan admin organisasi; satu periode aktif dan satu periode arsip. Gunakan dua sesi/perangkat untuk konflik.

## Rapat, agenda dan privasi

- [ ] Kadiv membuat rapat divisi dengan tujuan, lokasi/link, zona, mulai/selesai dan peserta; restart lalu baca kembali.
- [ ] Buat rapat Medhum dan pilih peserta Humas serta Mediatek dari nama yang tersedia. Pengambilan pilihan peserta tidak membuka direktori privat divisi lain.
- [ ] Buat notulensi melalui template rapat, kembali ke agenda, muat ulang, pilih note berdasarkan judul dan simpan tautannya. Buka dari kedua arah.
- [ ] Note dari Space berbeda dan note yang sudah terhubung ke rapat lain ditolak.
- [ ] Ubah waktu/peserta dari sesi kedua. Penyimpanan versi lama ditolak dan isian formulir pertama tetap tersedia.
- [ ] Hanya penyelenggara dapat mencatat hadir/izin/tidak hadir, catatan dan waktu datang aktual. Waktu datang masa depan ditolak.
- [ ] Sudah lihat/ping/open tidak menjadikan peserta hadir atau tugas selesai.
- [ ] Periode ditutup/akses dicabut saat formulir terbuka: server menolak tulis dan data lama tetap tersedia sesuai hak baca.

## Scheduler dan pengiriman

- [ ] Buat tugas dengan tenggat tanggal dan instan; atur slot divisi/tugas dekat waktu saat ini agar bisa diamati tanpa menunggu berhari-hari.
- [ ] Jadwal bawaan H-2/H-1/H-0 06.00 zona periode benar. Deadline tanggal-saja tetap tanggal-saja; deadline desain tidak dikurangi ulang.
- [ ] Deadline instan sebelum slot 06.00 tidak menghasilkan pengingat sebelum-deadline setelah tenggat.
- [ ] Tugas overdue mendapat satu pengingat per hari/default; penyerahan/tutup menahannya, pembukaan kembali menjadwalkan generasi baru.
- [ ] Ganti tenggat/PJ/jadwal khusus: slot lama dibatalkan, penerima lama tidak menerima kejadian baru.
- [ ] Atur rapat H-1/60/15 menit; rapat mendadak hanya menerima slot yang masih relevan. Perubahan/pembatalan tidak menyisakan slot lama.
- [ ] Reminder pribadi hanya menuju perangkat pemilik meskipun dibagikan; tidak ke grup WhatsApp.
- [ ] Ping umum/tugas/rapat mendukung pesan bawaan/kustom dan pilihan kanal. Pengirim tanpa wewenang ditolak.
- [ ] WA menunjukkan adapter belum terhubung, bukan sukses kirim. Status perangkat/WA terpisah.
- [ ] Restart server/perangkat dan buka dua instance server: tidak ada duplikasi kejadian. Catch-up menampilkan satu pemberitahuan ringkas, bukan membunyikan seluruh riwayat.
- [ ] Sudah lihat dan snooze tidak mengubah progres/kehadiran. Snooze kejadian mendatang tidak mempercepat jadwal aslinya.

## Windows, suara dan Ace

- [ ] Tutup jendela → tray; pengingat tetap diproses. Buka kembali dari tray.
- [ ] Keluar aplikasi → proses perangkat berhenti. Server tetap berjalan terpisah.
- [ ] Aktifkan/nonaktifkan autostart dari executable tetap, lalu login ulang Windows; mode background tidak membuka jendela utama.
- [ ] Toast tampil sebagai notifikasi modern Windows; klik membuka tugas/rapat yang benar, termasuk periode berbeda.
- [ ] Aksi toast Sudah lihat dan 15 menit lagi mengubah respons yang benar.
- [ ] Klik toast ketika aplikasi belum berjalan/belum login; setelah masuk sebagai akun penerima, objek yang benar terbuka. Akun lain tidak menerima isinya.
- [ ] Uji Windows notification permissions, Sound off, volume mixer, Focus Assist/DND, fullscreen/presentation dan layar terkunci. Catat perilaku pop-up dan suara aktual.
- [ ] Bel bawaan satu kali per kejadian/batch; volume 0–100 dan WAV sendiri bertahan setelah restart. File terlalu besar/panjang/tidak valid ditolak.
- [ ] Ace floating dapat digeser, posisi disimpan, mode tepi muncul saat hover lalu kembali, hidden dapat dipulihkan lewat tray/pengaturan.
- [ ] Ace hidden tidak mematikan toast. Dialog membuka objek/sudah lihat/snooze tanpa menyelesaikan tugas.
- [ ] Impor PNG transparan/GIF, ganti nama, kembali ke Ace bawaan; pindahkan/hapus file asal dan restart.
- [ ] Sambung/lepas monitor, ubah skala/resolusi, pulihkan posisi: Ace tetap dapat dijangkau.
- [ ] Ukur CPU/RAM idle, GIF aktif dan Ace hidden pada Windows sasaran. Hasil pengukuran dicatat oleh pengguna.

## Offline, restart dan konflik

- [ ] Saat online, buka tugas/agenda/isi note yang akan dipakai offline. Putus koneksi dan buka kembali; data yang belum diunduh menunjukkan belum tersedia.
- [ ] Ubah progres todo/doing dan edit Notes offline; status jelas tersimpan di perangkat/belum cloud. Delegasi, role, bot dan penyerahan tetap memerlukan koneksi.
- [ ] Tutup proses/restart perangkat sebelum sinkronisasi. Masuk dengan sesi tersimpan, buka cache, pulihkan draf melalui perbandingan dan baca antrean yang sama.
- [ ] Sambung kembali: perubahan tersinkron dengan identitas objek yang sama. Putus koneksi setelah server menerima tetapi sebelum respons sampai, lalu coba lagi; mutasi tidak berganda.
- [ ] Perangkat A mengubah progres offline, B mengubah deadline: kedua perubahan bertahan dan waktu bukti tidak berubah.
- [ ] Perangkat A mengubah judul note, B mengubah isi: penggabungan mempertahankan keduanya. Perubahan isi yang sama menghasilkan konflik dengan dua versi.
- [ ] Editor lain memegang lease aktif ketika antrean Notes disinkronkan: tidak ada pengambilalihan diam-diam; draf pemilik tetap ada.
- [ ] Tinjau konflik di Sinkronisasi; gunakan versi perangkat atau buang antrean setelah konfirmasi. Perubahan server berikutnya tetap dapat menyebabkan konflik baru.
- [ ] Cabut akses/tutup periode sebelum reconnect: perubahan ditolak, cache resmi disegarkan/dihapus, draf pribadi dan alasan penolakan tetap tersedia.
- [ ] Keluar akun A lalu masuk B; B tidak melihat cache, draf, antrean, dialog Ace atau toast A. Respons jaringan A yang terlambat tidak masuk ke cache B.
- [ ] Dengan outbox mendatang sudah diunduh, putus koneksi: kejadian tetap muncul sekali di perangkat. Reconnect tidak menggandakan bel kejadian yang sama.
- [ ] Sudah lihat/snooze offline bertahan setelah restart dan disinkronkan saat koneksi kembali.
- [ ] Metadata lampiran pending tidak ditampilkan sebagai berkas yang sudah tersedia di cloud.
- [ ] Simulasikan penyimpanan penuh/browser storage ditolak: UI menampilkan kegagalan penyimpanan tanpa mengklaim perubahan sudah aman di cloud.

Catat hasil, perangkat/versi Windows, zona waktu dan langkah reproduksi masalah. Acuan acceptance: AC-25–29, AC-35–42 dalam PRD. Checklist ini adalah panduan, bukan laporan hasil lulus.
