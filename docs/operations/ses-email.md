# Email undangan lewat Amazon SES

Status 10 Nov 2026: kode siap (branch `feat/nim-wa-ses`). **Email di pilot belum diaktifkan**; pilot berjalan dengan `SPACE_MAIL_DRIVER=disabled`. Mengaktifkannya butuh izin owner lewat GOA.

## Yang dilakukan server
- **Driver** (`SPACE_MAIL_DRIVER`):
  - `disabled` menolak mengirim, sehingga undangan gagal dengan pesan "Email tidak dapat dikirim saat ini";
  - `log` hanya mencatat penerima dan subjek;
  - `smtp` benar-benar mengirim. Mematikan email cukup dengan mengganti nilai ini lalu restart API.
- **Format**: setiap email dikirim sebagai teks dan HTML sederhana bergaya Space (`internal/mail/template.go`). Desain final dari canvas email cukup mengganti berkas itu.
- **Batas kirim**:
  - `SPACE_MAIL_PER_MINUTE` (bawaan 20) membatasi semua alamat sekaligus. Kelebihannya ditolak dengan 429 "Terlalu banyak email…", tanpa antre.
  - Per alamat tetap berlaku 5 email per 15 menit.
- **Penolakan langsung**: balasan SMTP 5xx (misalnya di sandbox SES "Email address is not verified") menjadi 422 "Server email menolak alamat …; periksa NIM-nya". Undangan tidak dibuat.
- **Bounce dan komplain**:
  - SES mengirim laporannya ke topik SNS, lalu SNS mem-POST ke `/api/mail/ses-events`.
  - Server hanya menerima pesan yang ditandatangani SNS untuk topik `SPACE_SES_SNS_TOPIC_ARN`, dan mengonfirmasi langganan sendiri.
  - Undangan yang tertunda untuk alamat itu ditandai **"Tidak terkirim · alasan"** di kartu Undangan tertunda, dan tercatat di audit `invitation.bounce`.
  - Kirim ulang menghapus tanda itu. Bounce sementara (transient) diabaikan.

## Pemetaan kredensial vault `ses-smtp`
Nilainya tidak pernah dicetak, disalin manual, atau di-commit.

| Vault | Env API |
|---|---|
| `SES_SMTP_HOST` | `SPACE_SMTP_HOST` (endpoint SMTP SES ap-southeast-2) |
| `SES_SMTP_PORT` | `SPACE_SMTP_PORT` (587) |
| `SES_SMTP_USER` | `SPACE_SMTP_USERNAME` |
| `SES_SMTP_PASS` | `SPACE_SMTP_PASSWORD` |
| `SES_FROM` | `SPACE_MAIL_FROM` (wajib `@himasif.id`; server menolak start bila bukan) |
| — | `SPACE_SMTP_TLS=starttls`, `SPACE_MAIL_PER_MINUTE=20` |

## Langkah aktivasi di pilot (tc-vps), **setelah izin**
Skripnya ada di `E:\himasif-cache\deploy\ses-env.sh` dan dijalankan dari PC.

1. **Cek akun SES** di konsol AWS ap-southeast-2: sudah keluar dari sandbox? {{Unverified}}: belum dicek. Selama masih di sandbox, hanya alamat terverifikasi yang bisa menerima email, dan alamat lain ditolak dengan 422 di atas.
2. **`ses-env.sh --stage`**: menulis `~/.config/himasif-space-pilot/ses.env.pending` (0600) dari vault, tanpa mencetak nilai dan tanpa menyentuh `server.env` atau layanan.
3. **Bounce (opsional tetapi disarankan)**:
   - Buat topik SNS `space-ses-events` di ap-southeast-2.
   - Tambahkan langganan HTTPS ke `https://db.tiyoouw.app/api/mail/ses-events`.
   - Di SES, identitas `himasif.id` → Notifications, arahkan Bounce dan Complaint ke topik itu.
   - Isi ARN-nya sebagai `SPACE_SES_SNS_TOPIC_ARN` di berkas pending. Langganan terkonfirmasi otomatis setelah API memakai ARN tersebut.
4. **`ses-env.sh --activate`**:
   - membuat cadangan `server.env.before-ses`;
   - mengganti `SPACE_MAIL_DRIVER=disabled` menjadi `smtp`;
   - menambahkan isi berkas pending;
   - restart API dan memeriksa health. Bila health gagal, `server.env` dikembalikan.
5. **Uji kirim**: `space-server -env-file … mail-test -to <alamat tim>` sekali. Lalu undang 2–3 NIM tim lebih dulu, karena email kampus bisa memfilter pengirim baru.
6. **Mematikan lagi**: `ses-env.sh --disable` mengembalikan `SPACE_MAIL_DRIVER=disabled` dan me-restart API. Kredensial tetap ada di `server.env` tetapi tidak dipakai.
