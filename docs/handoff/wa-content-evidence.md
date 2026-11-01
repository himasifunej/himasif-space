# Broadcast konsep dan bukti Drive — implementasi lokal 4C

Status: kode ditulis, **belum build/test/migrasi/runtime**. Parser belum dibandingkan dengan contoh broadcast asli dari pengguna. Pengaturan awal nonaktif; konfirmasi format dan aktivasi dilakukan pengelola melalui aplikasi. Dokumen ini tidak membuktikan acceptance lulus.

## Alur yang ditulis

1. Bridge menyimpan event; backend menyimpan envelope dan mengakui cursor setelah commit.
2. Worker bisnis membaca pesan non-command dari grup Medhum yang dipetakan saat ini. Kebijakan broadcast dan Drive terpisah. Periode tertutup, koneksi nonaktif, dan grup yang berganti tidak menerapkan pesan ke tugas.
3. Daftar bernomor dipisah per item. Judul, PJ Humas, PJ Mediatek/desain, tanggal posting/DL dan penanda DADAKAN diekstrak tanpa AI. Teks blok asli, versi parser, pengirim, waktu kirim dan waktu diterima tetap disimpan.
4. Pencocokan konsep memakai judul dan tanggal kalender yang tepat dalam Space/periode. Tanggal tanpa tahun tetap tanpa tahun; kalender harus menghasilkan satu kandidat. Nama PJ hanya dibandingkan dengan akun kalender yang aktif dan tidak ambigu. Token mention hanya dibandingkan dengan identitas WA terverifikasi.
5. Pengirim harus PJ Humas terkait atau pengelola tugas Humas. Broadcast tidak dapat menetapkan PJ desain atau menggeser kalender. Konsep yang cocok dicatat sebagai fakta tahap konsep; publikasi dan progres tugas Humas tidak ditutup otomatis.
6. Receipt konsep menggunakan kunci konten/PJ/jadwal yang stabil. Repost mempertahankan waktu pertama dan tidak mengulang notifikasi tugas atau balasan item yang sudah tercatat. Item baru/perubahan PJ atau jadwal relevan memiliki receipt sendiri.
7. Tautan Drive hanya dinormalisasi dari host Google yang ditentukan. Probe membaca metadata file/folder melalui API, bukan mengambil URL bebas atau isi berkas. Shortcut, file terhapus, akses yang gagal dan klaim referensi masuk tinjauan. Probe dibatasi waktunya, dilakukan di luar lock database, dan tidak menilai mutu/desain.
8. Satu tugas desain aktif milik PJ/pelaksana pengirim menjadi kandidat sementara. Reply pada pesan terkait dan label `Judul:` mempersempit kandidat. Beberapa kandidat tidak menghentikan reminder semuanya. Repost sumber Drive yang sudah diketahui tidak dipasangkan ke tugas aktif lain hanya karena tugas lama telah selesai.
9. Hasil yang diterapkan menjadi bukti `pending` pada tugas kanonis. Penyerahan menahan reminder tugas itu melalui status `submitted`, kemudian kadiv/pengelola meninjau kecocokan lewat detail tugas. Approval mutu dan publikasi tetap terpisah.
10. Setiap item memiliki antrean, versi dan riwayat append-only. Peninjau berwenang dapat menetapkan, mengabaikan, menarik, memulihkan atau meralat pasangan dengan alasan. Pengirim tetap diambil dari envelope dan identitas yang terverifikasi; kolom teks nama bukan pengganti identitas.
11. Ralat pasangan Drive menolak bukti pada tugas lama melalui aturan tinjauan kanonis dan membuat bukti tujuan dengan waktu sumber asli. Tugas lama kembali ke progres sebelum penyerahan bila bukti tersebut masih yang terbaru. Bukti yang sudah digantikan tidak diubah diam-diam. Seluruhnya satu transaksi.
12. Edit/hapus pesan membuat item perubahan sumber untuk tinjauan. Bukti lama, receipt, waktu pertama dan catatan sumber tidak dihapus otomatis. Peninjau dapat mempertahankan bukti dengan alasan atau meralat item sumber aslinya.

## Format awal yang didukung

Contoh berikut adalah **contoh buatan untuk menjelaskan parser**, bukan contoh pengguna yang telah disetujui:

```text
1. Judul konten
PJ Humas: Nama akun Humas
PJ Mediatek: Nama akun Mediatek
Tanggal: 20 Oktober 2026

DADAKAN
2. Judul lain
PJ Humas: Nama akun Humas
DL: 22/10/2026
```

Nomor dapat memakai titik atau kurung tutup. Tanggal mendukung ISO, format hari/bulan/tahun dengan tahun empat digit, serta nama bulan Indonesia; tahun dua digit dan tanggal/jam bebas tidak ditebak. Variasi template nyata harus diperiksa melalui pratinjau dan diperluas bila perlu. DADAKAN disimpan sebagai informasi sumber, bukan keputusan pelanggaran atau penetapan pengecualian SOP otomatis.

## UI dan konfigurasi

- Tab **Konsep dan bukti** di Medhum berisi inbox, sumber pesan, kandidat kalender, alasan keputusan dan riwayat. Tombol tugas desain membuka alur tinjauan bukti kanonis.
- Pengelola Humas memiliki pratinjau contoh dan aktivasi broadcast. Pengelola Mediatek mengaktifkan pencocokan Drive. Setiap perubahan kebijakan memakai versi.
- Parser tetap nonaktif sampai pengelola mengaktifkannya. Konfirmasi format belum dilakukan agen.
- Identitas pengirim ditautkan melalui **Pengaturan → WhatsApp → Buat kode tautan** lalu `!link <kode>` di grup terhubung.
- Google Drive memakai `SPACE_GOOGLE_DRIVE_ACCESS_TOKEN` atau service account server dengan scope `drive.metadata.readonly`, terpisah dari token/cache Sheets. Akun integrasi harus dapat membuka file/folder terkait. Peninjau yang benar-benar memeriksa tautan dapat mencatat konfirmasi akses manusia; itu tidak menjadi klaim bahwa probe server berhasil.
- Pesan sebelum parser diaktifkan tidak dibackfill otomatis. Tidak ada migrasi atau pembacaan sumber produksi dalam sesi implementasi ini.

## File utama

- `internal/broadcast/parser.go`: parser murni dan normalisasi.
- `internal/integrations/drive.go`: parsing link, metadata dan pembatasan akses.
- `internal/store/migrations/0011_wa_content_evidence.sql`: kebijakan, antrean, receipt, audit dan `task_evidence.recorded_at`.
- `internal/server/wa_content_{worker,apply,api}.go`: pipeline, aturan penerapan dan keputusan.
- `internal/api/wa_content.go`, `internal/store/wa_content.go`, `internal/client/wa_content.go`: kontrak dan penyimpanan.
- `internal/appui/wa_content_view.go`: pratinjau, kebijakan dan tinjauan native.
- Helper penyerahan/tinjauan bukti pada `internal/server/tasks.go` dipakai bersama HTTP dan sumber WA. `submittedAt` menyimpan waktu kejadian; `recordedAt` menentukan urutan pencatatan bukti, sehingga bukti historis yang baru dipasangkan tetap dapat ditinjau.

## Verifikasi yang masih diperlukan pada fase akhir

Parser terhadap contoh nyata dan variasinya; repost/penambahan item tanpa notifikasi duplikat; tanggal/tahun ambigu; nama ganda; clock sumber yang tidak wajar; sender/role/grup berubah; dua kandidat Drive; akses gagal; link referensi; repost file setelah tugas lama selesai; ralat dan pemulihan bukti; bukti lama yang sudah digantikan; edit/revoke; periode tertutup; restart dan dua instance worker; UI pada native/WASM.

Tahap 4B masih membutuhkan bahasa respons inti. Transport sudah direvisi untuk memindahkan network call keluar dari transaksi periode/account; lihat [checkpoint transport dan SOP](sop-foundation.md). Ketahanan dua sesi tetap harus diverifikasi pada fase akhir. Tahap 4D–6 dan verifikasi seluruh perubahan tetap bagian goal aktif.

Referensi resmi: [Drive files.get](https://developers.google.com/workspace/drive/api/reference/rest/v3/files/get), [resource keys](https://developers.google.com/workspace/drive/api/guides/resource-keys). Integrasi memakai metadata saja dan host tetap; belum ada permintaan Drive nyata.
