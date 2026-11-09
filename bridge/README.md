# Space WhatsApp bridge

Layanan internal terpisah dari server Go, memakai upstream resmi `@whiskeysockets/baileys` yang dipin ke `7.0.0-rc14`. Versi ini dipilih dari registry/upstream saat implementasi; instalasi dependensi, lockfile, build dan pengujian akan dilakukan pada fase akhir bersama aplikasi.

Node 22+ dan FFmpeg diperlukan. `bridge/.env.example` menjelaskan bearer internal, kunci AES-256 dan direktori state. File env dan state harus berada di luar git. Proses membaca environment yang sudah dimuat supervisor/container; tidak otomatis membuka `.env` dari direktori kerja.

- Satu koneksi menggunakan namespace state sendiri. Auth/signal keys, pesan media, registry dan jurnal terenkripsi AES-GCM dengan file atomik berizin terbatas.
- Token bridge minimal 32 karakter; listener bawaan localhost. QR tidak dicetak ke log atau disimpan ke database aplikasi.
- Grup tujuan memakai allowlist dari Go. DM dan grup yang tidak dikonfigurasi tidak diproses sebagai command.
- Jurnal job ditulis sebelum mengirim; message ID stabil. Hasil yang belum pasti disimpan `unknown` dan diperiksa melalui endpoint job, bukan dikirim ulang dengan identitas baru.
- `accepted` berarti kanal menerima tindakan, bukan konfirmasi dibaca. Receipt memperbarui `delivered`/`read` bila SDK menyediakannya.
- Event inbound memiliki sequence persisten dan harus diakui backend setelah commit. Antrean penuh menghentikan koneksi sementara, bukan mereset pairing.
- Reconnect memakai jeda bertambah; logout/bad session membutuhkan tindakan pengelola. Tidak ada restart beruntun untuk mencoba mengatasi pesan hilang.
- Stiker memakai media grup yang tersimpan/quoted, input maksimal 16 MiB, durasi enam detik, kanvas transparan 512×512 dan keluaran maksimal 1 MiB. Tidak menjalankan shell dari pesan.

Start setelah dependensi disiapkan: `npm start`. Layanan belum dijalankan oleh agen pada tahap implementasi ini.

Rujukan: [upstream releases](https://github.com/WhiskeySockets/Baileys/releases), [upstream source](https://github.com/WhiskeySockets/Baileys). Integrasi Go menyimpan wewenang/periode/identitas dan seluruh keputusan bisnis; bridge menangani transport, sesi, receipt dan konversi media.
