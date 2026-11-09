# Canvas A · Tahap 2 — Board tugas dan detail tugas

Branch `canvas-a/tahap2-board`, berbasis `77b4f49`. Acuan: `cv/02-tab-board.png`, `cv/05-card-detail.png`, `pair/board.png`, `pair/detail.png`, `cvA.html` (`#view-board`, `renderBoard()`, `openDetail()`, drawer "Tugas baru").

## Yang sudah sesuai canvas

**Toolbar board** (hanya board Space; Jobdesk tidak diubah, itu Tahap 3)
- Satu kepala: "Board tugas" + "Geser kartu antar kolom. Klik kartu untuk detail.", di kanan: segmen **Warna: label · Status · PJ · Polos**, tombol **Filter**, tombol **Saya saja**.
- Baris tambahan "Tugas Space" dan judul ganda dihapus. Isi baris lama tidak hilang: kotak filter teks, "Tampilkan yang dibatalkan", "Hapus filter" dan "Muat ulang tugas" ada di popover Filter.
- Mode warna baru "PJ" (warna avatar orang yang sama dengan sidebar/topbar Tahap 1). Segmen yang sama juga tampil di panel Tampilan.
- "Saya saja": tugas yang PJ-nya kamu atau kamu sebagai delegasi tambahan.
- Di layar sempit (HP, atau board yang menyempit karena drawer) toolbar turun ke bawah judul.

**Kartu**
- Beberapa label sebagai chip berwarna; aksen warna hanya di tepi kiri (strip 3 px), border lain tetap warna border biasa.
- Avatar PJ berwarna + nama depan ("[DUMMY] Aruna Desain" → "Aruna").
- Tenggat pendek "5 Nov"; tugas lewat tenggat: "5 Nov · lewat" merah.
- Baris bawah: kode singkat, jumlah bukti/lampiran (ikon klip), checklist "1/3" (hijau saat lengkap).
- Deskripsi dan nama Space tidak lagi tampil di kartu.

**Kolom**
- Header titik warna + nama + hitungan polos; kolom berlatar surface, kartu surface-2.
- Slot putus-putus "Lepas kartu di sini" di bawah kartu; kolom kosong: "Belum ada tugas / Tarik kartu ke kolom ini".
- "+ Tambah tugas" di bawah tiap kolom (hanya bila punya izin kelola tugas dan online) → membuka drawer "Tugas baru".

**Modal detail**
- Kepala: pil status (titik + label + panah; menu "Pindahkan ke" yang lewat `moveTask`, jadi aturan lama tetap: Diserahkan butuh bukti, Selesai lewat pencocokan, buka kembali butuh alasan) + kode + breadcrumb "Space · Board tugas", menu ⋯ (Ubah tugas / delegasi, Duplikat, Batalkan tugas) dan tutup.
- Baris aksi, masing-masing hanya bila izin/API mendukung: **Serahkan hasil** (primer), **Label**, **Checklist** (fokus ke input item baru), **PJ** (pilih anggota + alasan wajib, sesuai validasi server), **Lampiran**, **Ping PJ** (membuka dialog ping yang ada, jenis Tugas, tugas ini terpilih; hanya bila Space punya izin ping), **Ajukan bantuan**.
- Grid PJ (avatar) · Label (chip) · Tenggat + badge **Lewat** / **Selesai** · Hasil yang diminta. Di bawahnya baris kecil Mulai · Posting · status bukti, dan delegasi tambahan bila ada.
- Deskripsi: bisa diedit langsung oleh pengelola (tombol "Simpan deskripsi" muncul saat berubah); lainnya baca saja.
- Checklist dengan bilah progres hijau, item selesai dicoret.
- "Lampiran dan bukti": daftar bukti (tautan/catatan, status cocok, penyerah, alasan tinjauan), tombol cocok/tidak cocok untuk peninjau, form serahkan hasil dibuka lewat "Serahkan hasil"/"Lampiran"/"Tambah". Seret kartu ke Diserahkan langsung membuka form ini.
- Kolom kanan **Komentar dan aktivitas**: input komentar (Enter mengirim) dan satu timeline terbaru-dulu berisi komentar + aktivitas. Aktivitas dibangun dari data tugas yang sudah dimuat (dibuat & PJ, bukti diserahkan, dicocokkan, bantuan); "Muat riwayat lengkap" menggantinya dengan riwayat server (tanpa request tambahan sebelum diminta).
- **Pengaturan pengingat** per tugas (Hari sebelum deadline, Jam lokal, Menit, Overdue setiap N hari, Simpan jadwal khusus tugas, Ikuti jadwal divisi) tetap ada, dipindah ke bagian terlipat "Pengaturan pengingat tugas ini" di bawah kolom kiri, bukan kolom kanan.
- Tampilan HP (inspector layar penuh) memakai isi yang sama, dengan pil status + kode di atas judul.

**Drawer "Tugas baru"**
- Judul "Tugas baru" dengan garis bawah, isi scroll, footer tetap **Batal** + **Simpan tugas** (pesan galat di atas footer). Semua isian lama tetap (divisi pemilik, judul, hasil, deskripsi, label, PJ, jadwal, posting, bagikan ke Space bersama). Field "Kategori" berganti nama "Label · pisahkan dengan koma".

## Yang dilewati / berbeda, dan alasannya
- **Nomor #T12**: API tidak punya nomor tugas. Kartu dan modal menampilkan kode 4 karakter terakhir ID (mis. `#5A65`) dengan tooltip "Kode singkat tugas". Kode ini *bukan* kode `!done #T..` WhatsApp. Perlu kolom nomor di backend bila ingin sama persis.
- **Beberapa label**: API hanya punya satu `category`. Label disimpan sebagai teks dipisah koma ("Desain, Urgent") dan dipecah di UI; tidak ada migrasi. Daftar label tetap per Space (seperti `LBL` canvas) belum ada.
- **Lampiran berkas** (PNG, folder Drive): tidak ada API lampiran tugas; "Lampiran" = bukti hasil (tautan/catatan) yang sudah ada.
- **Pemilih sampul/warna kartu** (ikon di kepala modal canvas): tidak ada di API, dilewati.
- **"Tambah tugas" inline** di canvas mengetik judul langsung di kolom; di sini membuka drawer karena API butuh hasil, PJ dan tenggat. Tugas baru selalu mulai di Belum mulai.
- **Bot WhatsApp di timeline** ("Bot mengirim notifikasi…"): tidak ada di data tugas; muncul hanya bila ada di riwayat server.
- **Mention @** di komentar: belum didukung API.

## File yang disentuh
- `internal/workspaceui/state.go`, `internal/workspaceui/view.go` — data kartu (Labels, Due, Late, Ref, AssigneeColor, hitungan), Toolbar/Subtitle/OnAdd, kolom, kartu.
- `internal/appui/tasks_controller.go` — proyeksi kartu, Saya saja, helper label/kode/tanggal pendek, pesan "detail tugas".
- `internal/appui/task_detail.go` — modal detail, toolbar board, checklist, bukti, bantuan, timeline.
- `internal/appui/tasks_view.go` — board Space tanpa baris tambahan, drawer "Tugas baru"; isi detail lama dipindah ke `task_detail.go`. `taskAgenda` tidak diubah.
- `internal/appui/appearance.go` — `taskCardColor` (mode PJ, label pertama) dan segmen warna. `preferences.go` — komentar mode "pj".
- Tes: `internal/workspaceui/card_test.go`, `internal/appui/task_board_test.go` (baru); `chrome_test.go` dan `layout_test.go` diperbarui karena teks "N tugas" diganti toolbar dan kepala modal kini pil status, bukan "Detail tugas" (perubahan disengaja).

Kartu Jobdesk ikut memakai kartu baru karena komponen board sama; baris atas Jobdesk tidak diubah (wilayah Tahap 3).

## Hasil test
`flock /tmp/space-build.lock env GOMAXPROCS=1 GOFLAGS=-p=1 GOMEMLIMIT=256MiB GOGC=50 go test ./internal/appui ./internal/workspaceui` (vet kedua paket juga bersih):

```
--- FAIL: TestNoteDocumentNativeSpanEditingPreservesOtherRichRuns (0.00s)
    notes_document_test.go:88: document control "Teks bagian" missing in ["Tambah blok" "Tambah blok" "Blok 1–1 dari 1" "Konsep referensi" "referensi" "Atur blok 1" "Format dan atur blok"]
--- FAIL: TestNoteDocumentExtractionCopiesOnlyChosenTextAndNeverEditsReader (0.00s)
    notes_document_test.go:199: ui: no element shows "Jadikan tugas dari blok 1"
--- FAIL: TestNoteDocumentNativeBlockDeletionHandlesLastVisibleBlock (0.00s)
    notes_document_test.go:218: ui: no element shows "Hapus blok"
FAIL
FAIL	github.com/himasifunej/himasif-space/internal/appui	15.247s
ok  	github.com/himasifunej/himasif-space/internal/workspaceui	(cached)
```

Hanya 3 tes NoteDocument yang memang sudah gagal sejak dulu. Tes baru/diubah dijalankan ulang dengan `-count=1 -v`:

```
--- PASS: TestChromeBoardRouteDistinguishesUnloadedAndConfirmedSnapshots (0.28s)
--- PASS: TestTaskModalAndAppearanceAreExclusiveAtMinimumWidth (2.39s)
--- PASS: TestTaskLabelsRefAndShortDate (0.00s)
--- PASS: TestBoardMineOnly (0.02s)
--- PASS: TestTaskDetailModalFollowsCanvas (0.18s)
--- PASS: TestTaskCreateSaveRemainsReachable (0.11s)
ok  	github.com/himasifunej/himasif-space/internal/appui	3.066s
--- PASS: TestCardFollowsCanvas (0.05s)
--- PASS: TestBoardToolbarAndNoAddWithoutHandler (0.01s)
--- PASS: TestFirstNameSkipsTags (0.00s)
ok  	github.com/himasifunej/himasif-space/internal/workspaceui	0.066s
```

Belum dicek visual (tanpa browser/`make web`, sesuai brief); sesi utama yang membandingkan tangkapan layar.
