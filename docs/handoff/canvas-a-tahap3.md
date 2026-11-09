# Canvas A — Tahap 3: Jobdesk saya dan Beranda Space

Branch `canvas-a/tahap3-jobdesk-beranda`, di-rebase ke `976039f` (Tahap 2 sudah masuk).

## Jobdesk saya (`internal/appui/jobdesk.go`)
Sesuai canvas (cv/03-page-jobdesk.png):
- Board kanban di halaman Jobdesk diganti daftar berkelompok: **Lewat tenggat** (judul merah), **Minggu ini** (tenggat ≤ 7 hari), **Nanti** (lebih jauh atau tanpa tenggat), **Diserahkan, menunggu tinjauan**. Tugas selesai/dibatalkan tidak tampil. Kelompok kosong disembunyikan; semua kosong → pesan "Tidak ada tugas terbuka".
- Baris desktop: titik warna label (tagColor), judul + baris pertama deskripsi (atau "Hasil: …"), petak Space (M/H/MH, warna petak sidebar; tugas di ruang bersama tampil di ruang bersama), tanggal Plex Mono ("5 Nov", merah bila lewat), segmen **Belum / Kerjakan / Serahkan**, tombol **Buka**.
- Segmen memanggil `a.moveTask` — aksi yang sama dengan seret di board, dengan izin (`Permissions.Progress/Manage`), antrean offline, dan konflik versi yang sama. **Serahkan** tidak memindah tugas: `moveTask` membuka detail tugas (modal Tahap 2) dengan pesan untuk melampirkan bukti. Segmen nonaktif tanpa izin atau saat menyimpan/offline tanpa antrean.
- **Buka** membuka modal detail tugas yang sudah ada (`workspace.Open`, `taskBoardVisible` sudah mencakup PathJobdesk).
- Filter kanan atas **Semua ruang / per Space** (segmented control canvas), hanya muncul bila tugas tersebar di >1 Space.
- HP (<720): kartu bertumpuk per kelompok (judul, Space + tanggal, segmen penuh lebar + Buka 36 px).
- Board tetap di tab **Board tugas** Space; `personalTasksView` di tasks_view.go tidak dipanggil lagi (dibiarkan agar tidak konflik dengan Tahap 2; boleh dihapus kemudian).

## Beranda Space (`internal/appui/beranda.go`, `overviewPage` dipindah dari pages.go)
Sesuai canvas (cv/01-mediatek.png, cvAm.html):
- Judul **Beranda <Space>** + "Jumat, 6 November 2026 · minggu ke-N periode" (zona waktu periode; minggu dihitung dari `StartsOn`). Hero "Ruang … kita." dan tombol Buka board/Lihat agenda dihapus.
- Pilihan **Minggu ini / Bulan ini / Periode** (7 hari / 30 hari / sejak awal periode) mengatur detail KPI, sparkline, dan hitungan "Tepat waktu".
- 4 KPI dengan sparkline (8 titik): Tugas aktif ("N baru minggu ini"), Lewat tenggat ("Sejak 5 Nov · pengingat harian aktif"), Perlu tinjauan, **Tepat waktu N%** ("x dari y penyerahan · minggu ini"). HP: 2×2 tanpa sparkline, seperti cvAm.
- **Tepat waktu dapat dihitung** dari data yang sudah dimuat: tiap tugas di daftar Space sudah membawa `Evidence` (SubmittedAt, MatchStatus). Aturan: waktu bukti pertama ≤ tenggat → tepat waktu; bukti terakhir ditolak → "bukti belum cukup"; selain itu terlambat. Dihitung dari waktu kirim bukti, jadi lupa klik selesai tidak terhitung terlambat. Tanpa penyerahan di rentang → "—".
- **Ping cepat** (bila `canPing`): chip penerima (Yang lewat tenggat / Semua / per anggota dengan avatar), 3 pesan siap pakai, **Kirim ping** memakai state composer yang sudah ada (`notificationSettingsState`) dan `sendManualPing`; **Opsi lain** membuka dialog ping lengkap. Subjudul menyebut kanal yang aktif (bawaan "Perangkat").
- **Beban kerja anggota**: avatar + nama + bar bertumpuk (Belum mulai / Dikerjakan / Diserahkan / Lewat tenggat), "N tugas" mono, legend. Anggota dari daftar anggota yang sudah dimuat tab rapat (bila boleh membaca anggota) ditambah PJ tugas; anggota tanpa tugas tampil 0.
- **Tenggat 14 hari ke depan**: kolom per hari, blok per tugas berwarna label (tooltip judul, klik → Board tugas + detail), >5 → "+n", hari ini ditandai aksen.
- **Penyerahan 4 minggu** (donat tepat waktu / bukti belum cukup / terlambat) hanya bila ada penyerahan 28 hari terakhir.
- **Agenda** (rapat terdekat, tombol "Lihat kalender" → tab Agenda) dan **Aktivitas terbaru** (bila ada): diturunkan dari daftar tugas — tugas dibuat, hasil dikirim, bukti dicocokkan/ditolak, komentar — 5 terbaru dalam 30 hari.
- Desktop ≥900 px: baris 5/7 kolom seperti grid canvas; HP berhenti setelah Beban kerja, seperti cvAm.

## Yang dilewati / catatan
- Sparkline adalah rekonstruksi dari data tugas (CreatedAt, UpdatedAt untuk selesai/batal, waktu bukti), bukan riwayat tersimpan; tidak ada endpoint riwayat KPI. (perkiraan)
- Aktivitas canvas memuat kejadian bot WA dan pengingat terjadwal; data itu tidak ada di daftar tugas, jadi tidak ditampilkan (tanpa request tambahan).
- "Ping terakhir: … · sudah dilihat" tidak ada di API yang dimuat Beranda; diganti status hasil kirim dari composer.
- "Catatan bersama" di Beranda lama dihapus (tidak ada di canvas; tetap di tab Catatan).
- Tidak ada endpoint/migrasi baru.

## File disentuh
- baru: `internal/appui/jobdesk.go`, `internal/appui/beranda.go`, `internal/appui/canvas_tahap3_test.go`
- `internal/appui/pages.go`: Jobdesk memanggil `jobdeskView`; `overviewPage` lama dihapus (pindah ke beranda.go)
- `internal/appui/app.go`: dua field state (`jobdesk`, `home`)
- `internal/appui/access_test.go`: harapan "Ruang Medhum kita." → "Beranda Medhum" (perubahan disengaja)
- `shortMonth` dari Tahap 2 (tasks_controller.go) dipakai ulang, tidak diubah.

## Test
Perintah: `flock /tmp/space-build.lock env GOMAXPROCS=1 GOFLAGS=-p=1 GOMEMLIMIT=256MiB GOGC=50 go test ./internal/appui ./internal/workspaceui` (go vet ./internal/appui bersih). Run penuh pertama:

```
--- FAIL: TestBerandaDateLineAndDeadlineStrip (0.00s)
    canvas_tahap3_test.go:129: date line "Jumat, 6 November 2026 · minggu ke-11 periode"; want "Jumat, 6 November 2026 · minggu ke-10 periode"
--- FAIL: TestNoteDocumentNativeSpanEditingPreservesOtherRichRuns (0.00s)
--- FAIL: TestNoteDocumentExtractionCopiesOnlyChosenTextAndNeverEditsReader (0.00s)
--- FAIL: TestNoteDocumentNativeBlockDeletionHandlesLastVisibleBlock (0.00s)
FAIL	github.com/himasifunej/himasif-space/internal/appui	16.978s
ok  	github.com/himasifunej/himasif-space/internal/workspaceui	3.582s
```

Kegagalan Beranda adalah data test yang salah (28 Agu → 6 Nov adalah hari ke-70 = minggu ke-11); awal periode di test diganti 1 Sep. 3 test NoteDocument sudah gagal sejak dulu (di luar lingkup). Run ulang test terkait:

```
--- PASS: TestLegacyAPIConfirmsNoMembership (0.95s)
--- PASS: TestSpacesUseAuthorizedIDsAndBindings (0.04s)
--- PASS: TestArchiveManagementDoesNotSpreadAcrossMedhumDivisions (0.00s)
--- PASS: TestJobdeskGroupsFollowCanvas (0.00s)
--- PASS: TestJobdeskListStepsUseExistingMoves (0.17s)
--- PASS: TestSubmissionsCountFromEvidenceTime (0.00s)
--- PASS: TestBerandaDateLineAndDeadlineStrip (0.00s)
--- PASS: TestBerandaFollowsCanvas (0.19s)
--- PASS: TestMedhumPageHasCanvasTabsAndStages (0.49s)
ok  	github.com/himasifunej/himasif-space/internal/appui	1.953s
```

Tidak dicek visual (sesuai BRIEF, sesi utama yang mengecek tampilan).
