# Backend AWS untuk pilot desktop/web

Pada 30 Oktober 2026 pengguna meminta backend/database AWS persisten dan memilih `db.tiyoouw.app` melalui Cloudflare. Permintaan ini mengizinkan deployment backend baru, terpisah dari cutover Reo/WA atau pengirim pihak ketiga. Paket desktop pilot.4 ditujukan ke `https://db.tiyoouw.app`; web memakai origin yang sama.

Backend memakai database baru, bukan database restore QA atau salinan STB. PostgreSQL hanya mendengarkan loopback AWS; komputer pengguna mengakses API HTTPS. Database, lampiran dan konfigurasi privat berada di luar repo, web root dan paket unduhan. Akun admin organisasi memakai alamat yang diberikan pengguna; kata sandi awal dibuat baru dan disimpan privat, tidak berada di source atau paket.

Runtime dikelola user systemd dengan lingering: unit `himasif-space-pilot-db.service` dan `himasif-space-pilot-api.service`. Port privat database 55480 dan API 8788 dipilih terpisah dari preview QA. Cloudflare Tunnel baru hanya akan diarahkan ke hostname yang disetujui; proxy, container dan tunnel proyek lain tidak diubah.

Checkpoint awal: database/API privat sudah aktif, migrasi 0001–0022 dan seed admin berhasil. Otorisasi akun Cloudflare, DNS/tunnel dan verifikasi HTTPS publik masih menunggu penyelesaian login Cloudflare pengguna. Laporan operasional AWS terpisah mencatat hasil aktual setelahnya. Jangan menganggap status publik sudah hijau berdasarkan dokumen ini.

Login admin dan keberlanjutan sesi setelah restart API/database telah dibuktikan. Snapshot awal terenkripsi berhasil dipulihkan ke database baru yang dikarantina; health dan login pada hasil restore juga berhasil. Uji ini menemukan dan memperbaiki restore storage kosong, dilengkapi regresi arsip terenkripsi tanpa lampiran maupun state WA. Backup harian lokal terjadwal pukul 20.00 UTC, dengan penghentian singkat API selama capture dan pemulihan service sesudahnya. Salinan off-host dan akses kunci oleh pengelola pengganti belum disiapkan; backup pada disk VPS yang sama belum menjadi jaminan pemulihan bila host/disk hilang.

Email, WhatsApp dan konektor eksternal belum diaktifkan. Undangan/reset lewat email belum tersedia sampai SMTP disiapkan. Tidak ada pengiriman kepada pihak ketiga atau perubahan pengirim legacy. Pengujian Windows asli dan acceptance integrasi pada laporan rilis tetap berlaku; deployment ini tidak mengubah kandidat menjadi rilis stabil 0.10 secara otomatis.
