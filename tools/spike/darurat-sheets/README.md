# Spike: darurat via Google Sheets

Bukan kode aplikasi. Isinya menguji perilaku Google Sheet yang dipublikasikan ke web, sesuai note "Space — Darurat via Google Sheets" (`002f576e0dd5`).

- `space-darurat-template.xlsx`: template dengan tab Meta, Tugas, Rapat dan Reminder. Kolom ID abu-abu disembunyikan dan dikunci, `Meta!B2 = NOW()` menjadi jam sheet, dan semua baris berisi data `[DUMMY]`.
- `make_template.py`: membuat ulang template (butuh `openpyxl`).
- `spike.py`: poller, hanya memakai standard library Python 3.9+.

## Langkah untuk owner (akun Google sendiri)

1. Unduh `space-darurat-template.xlsx`.
2. Buka Google Drive → **Baru → Upload file** → pilih berkas itu. Klik kanan berkasnya → **Buka dengan → Google Spreadsheet**, lalu **File → Simpan sebagai Google Spreadsheet**. Kerjakan langkah berikutnya di salinan Google Spreadsheet ini, bukan di berkas .xlsx.
3. **File → Setelan**: atur *Zona waktu* ke **(GMT+07:00) Jakarta**. Di tab *Penghitungan*, pilih **Saat ada perubahan dan setiap menit** → Simpan.
4. **Bagikan** (kanan atas) → *Akses umum*: **Siapa saja yang memiliki link** → peran **Editor** → Salin link → Selesai. Ini **link edit**.
5. **File → Bagikan → Publikasikan ke web** → pilih **Seluruh dokumen** dan format **Microsoft Excel (.xlsx)** → buka *Konten & setelan yang dipublikasikan* dan pastikan **Publikasikan ulang secara otomatis setelah ada perubahan** dicentang → **Publikasikan** → OK → salin link yang muncul. Ini **link publish** (bentuknya `…/d/e/2PACX-…/pub?output=xlsx`).
6. Kirim balik:
   - link publish;
   - link edit;
   - jenis akunnya: **Gmail pribadi** (`@gmail.com`) atau **akun kampus/organisasi**;
   - ada atau tidaknya pesan bahwa publikasi/berbagi dibatasi administrator.
7. Saat diminta (sekitar 30 menit setelah spike berjalan), lakukan **satu edit**: di tab Tugas, ubah Progres baris `[DUMMY] Desain poster acara` menjadi `doing`, lalu ketik di kolom Catatan baris itu `edit JJ:MM:DD` sesuai jam saat itu (contoh `edit 14:05:30`). Setelah itu, beri tahu kami jamnya.

Setelah spike selesai, hentikan publikasi (**File → Bagikan → Publikasikan ke web → Hentikan publikasi**) dan ubah akses umum menjadi **Dibatasi**. Data di dalamnya hanya dummy, tetapi langkah ini sekaligus melatih penutupan sheet.

## Menjalankan

    python3 spike.py --pub 'LINK_PUBLISH' --log run.jsonl --duration 3600
    python3 spike.py --summarize run.jsonl

Setiap 3 menit, ditambah jitter sampai 60 detik, `spike.py` mencatat: status, content type dan header cache; apakah xlsx terbaca beserta tab, kolom tersembunyi, proteksi dan validasinya; jam Meta dibandingkan jam sekarang (jeda nyata); dukungan conditional GET; CSV per tab lewat gid (gid dibaca dari `pubhtml`, atau isi `--gids Meta=0,Tugas=…`); sel yang berubah; dan jeda edit owner dari tanda `edit JJ:MM:DD`. Respons 429, 5xx dan HTML dicatat, lalu interval dilipatgandakan sampai maksimal 15 menit.
