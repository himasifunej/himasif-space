# Broadcast konsep dan bukti Drive — implementasi lokal 4C

Status: kode ditulis, **belum build/test/migrasi/runtime**. Sejak F1-06 (25 Nov 2026) parser diuji dengan template broadcast asli owner (dianonimkan), lihat "Format yang didukung". Pengaturan awal nonaktif; konfirmasi format dan aktivasi dilakukan pengelola melalui aplikasi. Dokumen ini tidak membuktikan acceptance lulus.

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

## Format yang didukung (parser `humas-fields-v2`, F1-06, 25 Nov 2026)

Parser kini membaca **template Humas asli** yang dibagikan owner (jawaban no. 4 di audit 25 Nov), selain format berlabel lama. Contoh di bawah sudah **dianonimkan** (nama orang diganti); fixture yang sama ada di `internal/broadcast/parser_test.go` (`ownerTemplate`):

```text
[HALO, WELCOME TO NOVEMBER MEDIATEK]

Kami dari humas izin menyampaikan list konsep postingan bulan NOVEMBER yaa!

1. open regis caketum, pj humas @dimas, pj med @Sari Putri SI'24 (19 nov) dark blue
2. mingper 15, pj humas @lala, pj med @Bima SI'23 (Bimbim) (23 nov) light b

DADAKAN

note : tanggal = DL
terima kasih, semangattt!
```

Hasil parse:

| # | Baris | Judul | PJ Humas | PJ Mediatek | Tanggal posting | Deadline desain | Warna | Dadakan |
|---|---|---|---|---|---|---|---|---|
| 1 | 5 | open regis caketum | dimas | Sari Putri | 19/11 | 17/11 | dark blue | tidak |
| 2 | 6 | mingper 15 | lala | Bima (alias Bimbim) | 23/11 | 21/11 | light b | tidak |

Aturan yang dipakai:

- **Bulan dari judul.** Baris sebelum item pertama yang menyebut nama bulan ("WELCOME TO NOVEMBER", "postingan bulan NOVEMBER") menetapkan bulan daftar, dan tahunnya bila tertulis ("DESEMBER 2026"). Tanggal tanpa bulan, misalnya `(19)`, mengambil bulan itu. Tanpa bulan di judul, tanggal tanpa bulan menjadi error baris.
- **Satu baris satu konten.** Judul = teks sebelum label PIC pertama. Label: `pj humas`/`pic humas`/`pj hms` dan `pj med`/`pj mediatek`/`pj medtek`/`pj mdtk`/`pj desain`/`pic med`, dengan atau tanpa `:`; pemisah koma, `—`, `|` atau spasi. Nomor boleh `1.`, `1)`, `1️⃣`, atau tidak ada sama sekali (baris tetap dikenali dari label PIC).
- **Nama.** `@` di depan, tag angkatan (`SI'24`) dan alias dalam kurung (`(Bimbim)`) dipisahkan: nama masuk `humas`/`design`, alias ke `humasAlias`/`designAlias`. Token mention `@628…` tetap dicocokkan ke identitas WA terverifikasi seperti sebelumnya.
- **Tanggal.** Urutan pencarian: tanggal dalam kurung (`(19 nov)`, `(DL 3 agt)`), tanggal berlabel (`tgl 19 nov`, `DL: 19/11`), lalu hari + nama bulan setelah judul. Nama bulan Indonesia/Inggris lengkap atau singkat (`nov`, `nop`, `sept`, `agt`, `okt`, `des`, …), huruf besar-kecil bebas. Tahun dua digit tetap tidak ditebak.
- **Tahun.** Tetap kosong bila tidak tertulis di item atau judul (`postingDate` kosong, `Day`/`Month` terisi); pencocokan kalender memakai hari/bulan seperti sebelumnya. Bila judul punya tahun, item Januari pada daftar Desember mendapat tahun berikutnya.
- **Deadline desain = tanggal posting − 2 hari** (keputusan owner 25 Nov, sama dengan `internal/server/medhum.go`). Tanggal di broadcast dibaca sebagai **tanggal posting**; catatan `tanggal = DL` disimpan sebagai `dateIsDL` di tiap item dan tidak mengubah perhitungan. Tanpa tahun, deadline tetap diisi sebagai `deadlineDay`/`deadlineMonth`, kecuali posting 1–2 Maret (bergantung tahun kabisat; `Item.DesignDeadlineIn(year)` menghitungnya setelah tahun diketahui).
- **Warna.** Teks setelah tanggal dalam kurung disimpan apa adanya sebagai `color` ("dark blue", "light b"); tidak ditafsirkan.
- **DADAKAN.** Baris `DADAKAN`/`DADAKAN:` menandai semua konten sesudahnya `urgent`. Di bagian ini baris tanpa nomor ikut dibaca. DADAKAN tetap informasi sumber, bukan keputusan pelanggaran atau pengecualian SOP.
- **Error per baris.** Konten yang terbaca sebagian tetap muncul dengan `issue` yang menyebut nomor baris pesan dan apa yang kurang, misalnya `Baris 3: PJ Humas belum terbaca.` atau `Baris 2: Format tanggal tidak didukung atau tanggal tidak ada di kalender.` Item ber-issue tidak diterapkan otomatis.
- **Pesan biasa tidak diproses.** Sebuah pesan baru dianggap broadcast bila setidaknya satu baris punya label PIC atau tanggal berlabel; daftar obrolan dengan tanggal dalam kurung saja diabaikan.

Format berlabel lama tetap didukung:

```text
1. Judul konten
PJ Humas: Nama akun Humas
PJ Mediatek: Nama akun Mediatek
Tanggal: 20 Oktober 2026
```

Catatan untuk **F1-28 (ACC bersama, gelombang berikutnya):** satu ACC cukup dari PJ Humas konten, pengelola Humas, atau pengelola Mediatek, dan PIC tidak boleh meng-ACC karyanya sendiri. Parser tidak menyimpan ACC; data yang dibutuhkan F1-28 (PJ Humas dan PIC Mediatek per konten) sudah terbaca. PIC Mediatek tetap **ditetapkan kadiv Mediatek di awal bulan**; `pj med` di broadcast hanya dicocokkan dengan penetapan itu, tidak menetapkan atau mengganti PJ desain.

Belum diuji pada lebih banyak contoh asli: owner baru memberi satu template. Contoh tambahan dari Humas (OPEN-08) masuk sebagai fixture dianonimkan saat tiba.

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
