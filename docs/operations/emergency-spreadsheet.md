# Sheet darurat (Google Sheets) dan salinan CSV

## Sheet darurat (v1, branch `feat/darurat-sheets-v1`)

Rencana dan hasil spike ada di note enowX "Space — Darurat via Google Sheets" (`002f576e0dd5`); desain di canvas `dcb4562af51c` (opsi B dan desain 5). Cakupan v1: **tugas saja**. Server tidak berubah.

**Persiapan, selagi server hidup** (Pengaturan › Pusat darurat › Persiapan):

1. **Simpan template (.xlsx)**: tab Meta (`jam_sheet = NOW()`) dan Tugas. Kolom I–K (`task_id`, `task_version`, `base_progress`) disembunyikan dan berisi dropdown Progres.
2. Pemilik sheet mengunggahnya ke Drive dan menyimpannya sebagai Google Spreadsheet, lalu:
   - mengatur zona waktu Jakarta dan penghitungan "setiap menit";
   - membagikan "Siapa saja yang memiliki link: Editor";
   - melindungi kolom I–K (proteksi dari .xlsx hilang saat impor);
   - mempublikasikan seluruh dokumen sebagai .xlsx dengan publikasi ulang otomatis.
3. **Uji dan simpan** link publish dan link edit. Keduanya disimpan per akun di penyimpanan offline perangkat (`rec-darurat-*`), tidak di server.

**Saat gangguan:** server tak terjangkau 2 menit memulai mode darurat.

- Ace Island menawarkan **Buka sheet** dan **Salin link**. Ace tetap muncul dari tray karena ticker 15 detik desktop memanggil `TickEmergencySheet`.
- Satu pengelola memakai **Isi sheet dari cache**: salin baris lalu tempel di A1 tab Tugas, atau simpan .xlsx untuk diimpor.
- Setiap desktop yang menyimpan link membaca `pub?output=xlsx` tiap 3 menit, ditambah jitter sampai 1 menit.
  - Hanya salinan dengan jam Meta lebih baru yang dipakai. Google bisa mengirim salinan lama sesudah yang baru.
  - Respons HTML, 404, 5xx, jaringan putus atau berkas rusak tidak mengubah data perangkat. Jeda dilipatgandakan sampai 15 menit, dan 429 mengikuti Retry-After sampai 30 menit.
  - Lebih dari 30 baris berubah atau hilang sekaligus ditahan sampai pengelola memilih **Terapkan** atau **Abaikan**.
  - Baris tanpa ID dan baris yang tidak terbaca ditampilkan, tidak diterapkan. Baris yang hilang dari sheet tidak menghapus apa pun.
- Progres dari sheet tampil di board ("· dari sheet"). Progres, pembatalan dan pembukaan kembali tugas yang ada di sheet dikunci di Space sampai **Akhiri mode darurat**.

**Setelah pulih:**

1. **Sudah dibekukan** (link edit dijadikan Pelihat). Space menunggu salinan yang jam Meta-nya sesudah jam pembekuan, yaitu versi final.
2. **Siapkan rekonsiliasi**:
   - Space membuat snapshot server baru per Space lewat endpoint yang sudah ada.
   - Baris dengan `task_version` sheet yang masih sama dengan versi di snapshot masuk ke pratinjau, lalu diterapkan atau ditinjau dengan alasan seperti CSV.
   - Baris yang tugasnya berubah di server, tidak ada di snapshot, atau tanpa ID masuk **Ditinjau manual**.
   - Pelapor kosong dicatat "Tidak dicatat di sheet".
3. Centang **Publikasi dihentikan** dan **Akses dibatasi**, lalu pilih **Akhiri mode darurat**.

**Kode:**
- `internal/emergency/sheet`: link, xlsx, parse, follow, fetch.
- `internal/qr`: encoder QR tanpa dependensi, dicocokkan dengan python-qrcode.
- `internal/appui/emergency_sheet*.go`.
- `app/emergency_links_desktop.go`.

**Diuji:**
- Unit test: salinan lama sesudah baru, HTML, 429, 5xx, 404, berkas rusak, ID ganda atau rusak, baris tanpa ID, rem 30 baris, kunci progres, pemasangan rekonsiliasi.
- Pembacaan read-only sheet uji owner lewat `SPACE_SHEET_E2E`: 6 baris [DUMMY] terbaca dari kolom ID tersembunyi.

**Belum diuji:**
- jeda dari edit sel sampai salinan publish;
- edit anonim lewat link edit;
- menempel ke kolom tersembunyi Google Sheets;
- "Ganti spreadsheet" dengan tetap mempertahankan link publish;
- banyak desktop di satu NAT;
- rekonsiliasi terhadap server sungguhan.

## Salinan spreadsheet darurat (CSV, cara lama)

Tetap tersedia di Pusat darurat › **Cara lama: salinan CSV**.

Checkpoint 29 Oktober 2026: implementasi API/klien/UI/parser ditulis, **belum build/test/migrasi/runtime**. Tidak membuat spreadsheet publik, mengunggah ke Google atau mengubah sumber Reo. Prosedur dan skenario di bawah masih harus dibuktikan pada fase verifikasi akhir.

## Membuat salinan

Pada **Pengaturan → Darurat**, gunakan akun pribadi, pilih Space yang memang berizin, pilih semua tugas aktif atau tugas dengan diri sendiri sebagai PJ utama, lalu buat snapshot. Tugas selesai/dibatalkan hanya ikut bila dipilih. Simpan CSV melalui dialog desktop atau unduhan browser.

CSV mencakup ID snapshot/baris/tugas, waktu snapshot UTC, versi, judul, hasil yang diminta, PJ, deadline beserta jenis/zona, progres serta kecocokan hasil terakhir. Tanggal tanpa jam tetap `date`; waktu persis memakai RFC3339 dan tidak dikurangi H-2 lagi. Snapshot berisi baseline server yang tidak dapat diedit lewat CSV. Formula berbahaya pada teks diekspor sebagai teks aman spreadsheet.

Snapshot adalah salinan saat dibuat, bukan sinkronisasi otomatis. Tetapkan penanggung jawab dan frekuensi pembaruan salinan sebelum pilot sesuai pola kerja/RPO. Simpan CSV pada lokasi darurat yang dapat dijangkau ketika server mati, dengan akses sesuai isi Space. Reminder pribadi/Notes privat/bearer/kredensial bot tidak diekspor ke sheet divisi ini.

Batas ekspor: 5000 tugas dan 32 MiB per snapshot. Jika melebihi, perkecil cakupan PJ/status; tidak ada pemotongan baris diam-diam. API menyimpan snapshot dan audit sebelum menawarkan file. Jika jaringan pengambilan file terputus, gunakan **Ambil CSV snapshot ini**, bukan menganggap berkas parsial sebagai ekspor sukses.

## Mencatat saat gangguan

Pertahankan header dan seluruh identitas asli. Edit hanya:

| Kolom | Isian |
|---|---|
| `new_progress` | `todo` atau `doing` untuk perubahan progres yang hendak direkonsiliasi |
| `reported_at` | Waktu kejadian/laporan yang dicatat manusia, beserta zona bila diketahui; tidak diperlakukan sebagai waktu bukti terverifikasi |
| `reported_by` | Nama pelapor yang dapat ditindaklanjuti, bukan pengganti identitas akun pelaksana rekonsiliasi |
| `change_note` | Alasan/konteks atau permintaan perubahan lain yang harus ditinjau manual |

Biarkan empat kolom kosong untuk baris yang tidak berubah. Catat penyerahan hasil, perubahan PJ/deadline, publikasi, approval atau keputusan SOP sebagai konteks manual dan simpan bukti asli. Mengubah `new_progress` menjadi `done` tidak menutup tugas atau mengesahkan apa pun. Jangan menyalin satu bukti ke semua tugas yang memiliki judul mirip.

Ekspor kembali sebagai CSV UTF-8, dengan pemisah koma dan struktur kolom asli. Maksimal 100 baris **perubahan** per impor; file boleh tetap memuat sampai 5000 baris tugas yang tidak berubah. Jika perlu pecah perubahan menjadi beberapa file sambil mempertahankan ID aslinya. File Excel biner tidak dibaca sebagai CSV. Isi title/base_progress/versi pada CSV bukan sumber baseline tepercaya; server memakai snapshot yang disimpan saat ekspor.

## Rekonsiliasi setelah server pulih

1. Masuk dengan akun pribadi, pilih CSV pada halaman Darurat, lalu **Muat ulang pratinjau server**. Memilih file hanya membacanya pada perangkat; tidak langsung mengubah tugas.
2. Periksa waktu/Space snapshot, baseline, data server terkini, usulan dan catatan per baris. Server memeriksa akses Space/tugas sekarang; salinan lama tidak mempertahankan hak yang sudah dicabut.
3. Pilih baris yang memang akan ditangani dan catat alasan. Hanya progres `todo`/`doing` dengan baseline/versi yang masih sesuai, PJ/bukti yang tidak berubah dan kewenangan aktif yang dapat diterapkan melalui tombol **Terapkan progres yang siap**.
4. Bila tugas berubah atau sudah selesai/dibatalkan/ditinjau, buka tugas untuk pemeriksaan manual. Bukti, deadline, PJ dan status server tidak ditimpa otomatis. Lakukan tindakan melalui alur tugas berwenang yang sesuai.
5. Setelah memeriksa hasil manual, pilih **Catat tinjauan manual**; bila catatan tidak perlu diterapkan, pilih **Abaikan ... dengan alasan**. Kedua tindakan hanya mencatat disposisi rekonsiliasi, tidak mengubah tugas atau mengesahkan hasil/SOP. Versi data saat tinjauan juga diperiksa ulang.
6. Periksa hasil tiap baris. Status `applied`, `manual_reviewed` atau `ignored` tersimpan append-only dengan akun pelaksana dan waktu server. Retry CSV/intent yang sama tidak menerapkan dua kali. Baris yang sudah dipakai untuk isi lain menjadi konflik; jangan mengganti ID untuk menyamarkan perubahan—buat snapshot baru sesudah meninjau tugas.

Pelapor/waktu di sheet dipertahankan sebagai keterangan manusia. Aktor audit adalah akun yang benar-benar menjalankan rekonsiliasi dan waktu mutasinya adalah waktu server. Rekonsiliasi tidak memalsukan waktu penyerahan lama demi penilaian SOP. Data bukti dan riwayat asli harus masuk alur pemeriksaan yang sesuai.

Pemrosesan dilakukan per baris. Bila koneksi terputus di tengah batch, sebagian baris mungkin sudah commit; jalankan pratinjau lagi dengan CSV/ID yang sama untuk melihat receipt yang telah tercatat. Jangan membuat ulang ID hanya untuk mencoba ulang. Konflik tetap ditampilkan; tidak ada dua sumber yang saling menimpa bebas setelah pulih.

## Kontrak dan verifikasi akhir

Migrasi `0016_emergency_snapshots.sql` menyimpan snapshot, baseline tiap baris dan receipt disposisi append-only. Endpoint:

- `POST /api/spaces/{spaceId}/emergency-snapshots`
- `GET /api/emergency-snapshots/{snapshotId}/csv`
- `POST /api/emergency-reconciliation/preview`
- `POST /api/emergency-reconciliation/apply`
- `POST /api/emergency-reconciliation/resolve`

Implementasi: `internal/server/emergency.go`, `internal/api/emergency.go`, `internal/client/emergency.go`, `internal/emergency/csv.go`, dan `internal/appui/emergency.go`. Snapshot/receipt termasuk dump database; verifikasi jumlah record backup kini mencakup seluruh tabel publik, bukan daftar tabel lama yang tertinggal ketika fitur ditambah.

Skenario yang wajib diuji nanti: ekspor per scope/pribadi/arsip, CSV dengan koma/baris baru/formula/BOM, ID palsu/duplikat, role dicabut/periode ditutup, tugas berpindah Space, PJ/deadline/bukti berubah, dua pengelola mengimpor baris sama, koneksi hilang sesudah commit, catatan manual/abaikan, verifikasi aktor/waktu, dan pemulihan snapshot/receipt dari backup. Belum ada skenario tersebut yang diklaim lulus.
