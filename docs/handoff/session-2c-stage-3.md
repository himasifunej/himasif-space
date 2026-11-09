# Sesi 2C dan Tahap 3 — implementasi untuk pengujian pengguna

Tanggal penyerahan: 28 Oktober 2026. Versi aplikasi: 0.2.0.

Permintaan pengguna: kerjakan Sesi 2C serta seluruh Tahap 3 tanpa verifikasi/pengujian; pengguna menguji sendiri. Implementasi ditulis pada worktree ini. **Build, vet, tes, inspeksi visual/runtime, migrasi database dan pengujian Windows/performa tidak dijalankan.** Pemformatan sumber memakai gofmt sebagai bagian penulisan. Tidak ada klaim kriteria penerimaan sudah lulus, binary sudah dibangun atau deployment sudah dilakukan.

## Ruang lingkup

| Bagian | Implementasi |
|---|---|
| 2C — rapat | Jadwal/zona, tujuan, lokasi/link, peserta ruang bersama, penyelenggara, notulensi, pembatalan, versi, hadir/izin/tidak hadir dan waktu datang aktual |
| 2C — scheduler | Pengingat tugas H-2/H-1/H-0 06.00 zona periode, overdue, slot rapat, reminder pribadi, konfigurasi divisi/tugas, generasi jadwal dan outbox PostgreSQL |
| 2C — interaksi | Ping umum/tugas/rapat, pesan bawaan/kustom, pilihan kanal, inbox, sudah lihat dan snooze; respons tidak mengubah tugas/kehadiran |
| 3A — desktop | Tray, keluar eksplisit, autostart Windows, WinRT ToastGeneric, protokol pembukaan objek/aksi, bel WAV bawaan/kustom dan volume |
| 3B — Ace | Aset SVG bawaan, nama, floating/drag, tepi/hover, hidden/pemulihan dari tray, posisi layar/monitor, dialog notifikasi, PNG/GIF |
| 3C — offline | Cache akun, pembukaan ulang dengan sesi tersimpan, antrean progres/Notes/respons notifikasi, simpan atomik, idempotensi, penggabungan konservatif, konflik, penolakan izin dan status sinkronisasi |

## Backend dan migrasi

Migrasi tambahan `internal/store/migrations/0007_meetings_delivery_offline.sql` mempertahankan migrasi lama. Menambahkan rapat/peserta, event/destinasi pengiriman, konfigurasi jadwal, receipt mutasi offline dan revisi penjadwalan tugas/reminder. Migrasi **belum diterapkan** oleh sesi ini.

Server menjalankan scheduler selama `serve`. Advisory lock transaksi PostgreSQL membatasi satu perencana aktif; event key unik dan receipt mutasi disimpan secara persisten. Perubahan tenggat/PJ, konfigurasi jadwal, penyelesaian atau pembatalan membuat slot lama tidak berlaku. Penyerahan menghentikan pengingat tugas sampai dibuka kembali. Slot rapat sebelum waktu pembuatan/perubahan tidak dikejar. Tugas hanya mengejar slot relevan hari ini; inbox mengelompokkan kejadian yang tertinggal menjadi satu pemberitahuan perangkat. Deadline lebih awal dari jam pengingat tidak mendapatkan label pengingat sebelum tenggat sesudah waktunya.

Reminder tanggal-saja tetap tanggal-saja pada objek; scheduler memakai pukul 06.00 zona reminder sebagai kebijakan pengiriman yang ditampilkan pada UI. Slot yang sudah lewat sebelum reminder dibuat tidak dijadwalkan ulang. Ini tidak mengubah `dueOn` menjadi instan.

Rapat memakai `tasks.manage` untuk pembuatan/operasional oleh pengelola; penyelenggara yang masih menjadi anggota divisi pemilik dapat mengelola rapatnya. Hanya penyelenggara mencatat kehadiran/waktu aktual. Pemilih peserta memakai endpoint metadata minimal dalam Space, sehingga Medhum tidak memerlukan akses direktori privat divisi lain. Notulensi harus di Space yang sama dan tidak ditautkan ke dua rapat. Periode tertutup hanya baca; otorisasi diperiksa ulang di server.

Endpoint baru:

- `GET/POST /api/spaces/{spaceId}/meetings`
- `GET /api/spaces/{spaceId}/meeting-candidates`
- `GET/PUT /api/meetings/{meetingId}`
- `PUT /api/meetings/{meetingId}/attendance/{userId}`
- `GET /api/deliveries`
- `POST /api/deliveries/{deliveryId}/response`
- `POST /api/spaces/{spaceId}/pings`
- `GET/PUT /api/periods/{periodId}/divisions/{divisionId}/notification-policy`
- `GET/PUT/DELETE /api/tasks/{taskId}/notification-policy`
- `POST /api/sync`

Kanal `device` dan `whatsapp` memiliki status terpisah. Adapter WhatsApp tetap Tahap 4; pilihan kanal ini mencatat kegagalan eksplisit bahwa adapter belum terhubung. Tidak mengklaim pesan WA dikirim. `accepted` berarti API kanal menerima, bukan bukti manusia membaca.

## Desktop, toast dan Ace

Windows memakai WinRT melalui helper PowerShell bawaan Windows. Identitas toast `id.himasif.space` didaftarkan pada shortcut Start Menu melalui properti AUMID; `himasif-space://` diteruskan ke satu instance aplikasi. Payload tautan hanya petunjuk navigasi, bukan kredensial atau pemberian akses. Pembukaan tugas/rapat membaca objek, memilih periode/Space yang berizin, lalu membuka tampilannya. Akun penerima harus sesuai; tautan yang diterima sebelum login disimpan sementara sampai akun tersebut masuk.

Menutup jendela menyembunyikannya ke tray bila tray berhasil dibuat. Menu Keluar melakukan flush draf dan menghentikan proses perangkat. Scheduler server tetap terpisah. Autostart memakai HKCU Run dan argumen `--background`. Aktifkan autostart dari executable yang disimpan di lokasi tetap.

Pengaturan perangkat ada di tab **Perangkat**, termasuk nama/mode maskot, pemulihan posisi, impor aset, bel dan volume. Ace bawaan adalah SVG yang digambar aplikasi; logo Space tetap. Maskot impor PNG/GIF dibatasi 8 MiB, 2048×2048, 120 frame dan 16 juta piksel gabungan. Audio impor WAV dibatasi 8 MiB dan lima detik. Aset disalin ke penyimpanan perangkat, sehingga tidak bergantung pada lokasi file asal. Animasi berhenti ketika tersembunyi/idle; mode tepi menyisakan tab dan muncul saat hover/kejadian.

Toast disetel senyap agar bel aplikasi tidak menggandakan bel OS. Bel memperhatikan status penerimaan notifikasi Windows, pembatasan suara dan mixer sistem. Posisi/pop-up/penundaan tetap keputusan Windows. Perilaku Focus Assist/DND, layar terkunci, multi-monitor dan volume **perlu diuji pengguna**; tidak ada klaim pengujian perangkat nyata dalam penyerahan ini.

Jurnal perangkat mencatat identitas notifikasi sebelum pengiriman. Penerimaan ambigu tidak dibunyikan ulang otomatis. Peralihan akun membersihkan dialog/toast akun sebelumnya; pekerjaan lama membawa snapshot akun sendiri.

Rujukan implementasi Windows: [AUMID](https://learn.microsoft.com/en-us/windows/win32/shell/enable-desktop-toast-with-appusermodelid), [desktop toast](https://learn.microsoft.com/en-us/windows/win32/shell/quickstart-sending-desktop-toast).

## Offline dan konflik

Cache dipisahkan menurut server/akun; alias bearer memakai hash, tanpa menyimpan bearer pada cache. Desktop memakai direktori terlindungi dan file atomik; web memakai localStorage. Token tetap pada penyimpanan sesi yang sudah ada. Respons jaringan dipasangkan dengan akun sebelum I/O, sehingga pergantian akun tidak menulis respons akun lama ke cache akun baru.

Batas awal: 4 MiB per respons, 32 MiB cache per akun/server, 200 mutasi/16 MiB antrean. Kegagalan penyimpanan ditampilkan, bukan diperlakukan sebagai simpan cloud. Data yang belum pernah diunduh tidak diklaim ada offline. Penolakan otorisasi membuang cache resmi secara konservatif; draf/outbox pemilik disimpan terpisah. Metadata akses baru menyegarkan ruang yang tersedia.

- Progres offline: belum mulai ↔ sedang dikerjakan. Deadline, PJ, bukti dan waktu penyerahan tidak ditulis ulang.
- Notes: draf perangkat mempertahankan dasar versi. Judul dan isi dibandingkan dengan dasar tersebut. Perubahan pada bidang berbeda dapat digabung; konflik pada bidang sama menyimpan kedua versi untuk keputusan eksplisit. Pengambilan editor server diperiksa ulang saat sinkronisasi; tidak mengambil alih editor lain diam-diam.
- Sudah lihat/snooze: respons pengguna bisa diantrekan, termasuk waktu snooze lokal. Pengingat server yang sudah diterima tidak dijadwalkan ulang oleh penjadwal organisasi kedua di perangkat. Cache outbox mendatang memungkinkan pengingat yang sudah diunduh tampil ketika koneksi putus, memakai identitas kejadian server yang sama.
- Mutasi memiliki UUID stabil; receipt server dan perubahan objek dilakukan dalam transaksi yang sama. Retry sesudah respons hilang tidak menggandakan mutasi. Intent yang sudah dikirim tidak diubah; resolusi konflik membuat identitas baru.
- Penolakan akses/periode mempertahankan isi lokal dan menjelaskan kegagalan. Pengajuan ulang tetap diperiksa server. Delegasi, role/akses, bot, penyerahan/tinjauan hasil dan administrasi tetap online.
- Lampiran yang belum terunggah tetap pending. Unggah/download berkas biner bukan ditambahkan dalam perubahan ini; metadata/link lampiran mengikuti Notes yang sudah ada dan tidak diklaim sebagai file cloud.

Tab **Sinkronisasi** menampilkan antrean, status, error dan kedua versi konflik, serta pilihan mengajukan versi perangkat atau memakai server melalui konfirmasi. Tanggal sinkronisasi terakhir disimpan per akun.

## Penyerahan pengujian

Gunakan [panduan uji mandiri](session-2c-stage-3-manual.md). Seluruh skenario di sana belum dijalankan oleh agen. Tidak ada perubahan pada produksi, Reo, spreadsheet, SMTP, DNS atau sesi WhatsApp.
