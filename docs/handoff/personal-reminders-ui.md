# Integrasi reminder pribadi ke UI native

UI ini melengkapi backend PR #3 dan aplikasi/tugas PR #4 yang sudah bersatu di `main` (`8608fc4`). Tidak menambah migrasi: `0003_tasks.sql` dan `0004_personal_reminders.sql` tetap utuh.

## Perilaku

- Halaman **Reminder pribadi** mengambil daftar milik sendiri dan yang dibagikan. Filter judul/catatan, pengelompokan belum/selesai, serta panel kanan memakai MyGo native yang sama di desktop dan WASM.
- Pemilik dapat membuat, mengubah, menyelesaikan, membuka kembali dan menghapus reminder. Simpan menunggu konfirmasi API; kegagalan mempertahankan isian. Tombol Simpan tetap terlihat, sedangkan isi panel dapat digulir pada ukuran minimum 880×640.
- Tanggal saja tetap `dueOn`, tanpa jam buatan. Jam eksplisit dikonversi memakai zona IANA reminder, bukan zona periode. Mengubah judul/catatan tidak membulatkan detik atau presisi instan lama.
- Saat dibuka, editor memakai detail server terbaru. Penyegaran berikutnya memperbarui editor yang belum diubah, tetapi tidak menimpa isian lokal yang belum disimpan. Backend belum menyediakan versi/conditional update reminder; perubahan bersamaan di beberapa perangkat masih mengikuti penulisan terakhir.
- Default privat/preferensi default berbagi tersimpan per akun dan hanya memengaruhi reminder baru. Toggle berbagi per reminder tidak memilih penerima secara otomatis. Daftar penerima kosong berarti belum ada akun yang diberi akses.
- Pemilih penerima hanya membaca direktori divisi yang memiliki kemampuan `members.read` dari server pada periode terpilih. Pilihan dideduplikasi berdasarkan akun dan mengecualikan diri sendiri. Pergantian periode/akses menghapus pilihan direktori lama. Tidak ada penambahan akses otomatis untuk kadiv, 45, admin, BPH atau penerus.
- Endpoint penerima khusus tetap dibaca ketika toggle berbagi mati. Pemilik dapat melihat dan mencabut izin yang sedang nonaktif. Penerima hanya mendapat detail baca, tanpa editor, riwayat pemilik atau daftar penerima lain.
- Data reminder tetap melekat pada akun ketika periode/Space berubah. Logout menghapus snapshot, draft dan direktori; respons lama diabaikan. Bearer dibekukan sebelum operasi masuk goroutine, sehingga intent lama tidak memakai akun berikutnya.
- Riwayat menampilkan jenis perubahan dan waktu lokal reminder, hanya untuk pemilik. Hapus memakai konfirmasi dalam panel, bukan dialog bawaan sistem.

## Batas fase ini

Halaman ini menyimpan dan mengelola reminder. Penjadwal pengiriman, toast OS, suara, snooze dan WhatsApp tetap fase berikutnya. UI menyatakan pengiriman terjadwal belum tersedia; tidak mengklaim pengingat telah dijadwalkan. Reminder pribadi tidak dikirim ke grup WhatsApp. Tidak ada perubahan produksi, DNS, SMTP, Reo atau spreadsheet.

## Verifikasi

- Kontrak HTTP diuji untuk method/path/body ketat, date/instant, boolean `false`, grant nonaktif, preferensi dan error 404.
- Controller diuji untuk konfirmasi simpan, kegagalan/draft, default hanya reminder baru, larangan tulis penerima, pencabutan saat berbagi mati, pergantian periode/akun, respons lama, direktori yang diizinkan, presisi instan dan detail terbaru tanpa menimpa edit lokal.
- Pengujian renderer native menjalankan form buat reminder dan memastikan tombol Simpan, tutup, footer serta aksi paling bawah tetap terjangkau di 1440×900 dan 880×640; kontrol tulis tidak muncul untuk penerima.
- Review Standards menemukan satu kesalahan draft dari daftar lama; sudah diperbaiki beserta regresinya. Review Spec tidak menemukan kekurangan terhadap cakupan integrasi PRI-01–08.

Pemeriksaan akhir 28 Oktober 2026:

- Suite penuh `go test -count=1 -json ./...` dengan PostgreSQL nyata dan capture UI menghasilkan **240 tes/subtes lulus, 0 gagal**. Satu capture board opsional awalnya dilewati karena variabel khususnya belum diisi; dijalankan tersendiri dengan `SPACE_UI_SCREENSHOT_DIR` dan lulus. Total hasil unik yang terverifikasi **241 lulus**, tidak ada tes yang masih belum dijalankan.
- `go vet` Linux, Windows amd64 dan js/wasm lulus. Build Linux native, paket web MyGo/WASM dan portable Windows amd64 lulus. Runtime Windows dan installer NSIS belum diuji pada host Linux ini.
- Uji aplikasi native melalui Xpra: buat reminder tanggal-saja, ubah judul/catatan, tandai selesai, buka kembali, lalu restart aplikasi dan masuk ulang. Pembacaan API setelah setiap aksi mengonfirmasi ID dan waktu pembuatan tetap sama, tanggal tidak berubah menjadi instan, berbagi tetap nonaktif dan riwayat create/update/complete/reopen tetap tersimpan. Reminder contoh memakai data sintetis pada database preview lokal.
- Pembacaan ulang tugas contoh setelah restart tetap menunjukkan satu ID di Jobdesk saya, Mediatek dan Medhum, status done/verified versi 4, deadline/posting date-only, serta waktu penyerahan bukti asli yang sama. Tidak ada migrasi baru atau pengubahan migrasi lama.

## Standards

Satu temuan awal: editor memakai draft dari daftar lama walaupun detail server sudah lebih baru. Diperbaiki dengan baseline detail terbaru dan pemeriksaan isian yang belum diubah, disertai regresi. Pemeriksaan ulang: **0 temuan tersisa**.

## Spec

**0 temuan** terhadap cakupan integrasi PRI-01–08 dan kontrak reminder. Pengiriman tetap Sesi 2C/3A; antarmuka tidak mengklaim pengiriman sudah aktif. Batas last-write-wins lintas perangkat dicatat di atas.
