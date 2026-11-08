# Canvas A — Tahap 4: Agenda, Anggota, Reminder pribadi, Arsip

Branch `canvas-a/tahap4-agenda-anggota`, berbasis 77b4f49, sudah digabung dengan Tahap 2+3 (16d52ef) tanpa konflik.

## Yang sudah sesuai canvas
- **Agenda** (`agenda_calendar.go`, baru; `pages.go` case "Agenda" sekarang memanggil `a.agendaCalendar`): judul "Agenda <Bulan> <Tahun>" dan deskripsi canvas, segmented Bulan/Minggu/Daftar, kalender bulan bergrid (Sen–Min, hari bulan lain redup, angka hari ini di atas aksen), satu kalender untuk tenggat tugas, jadwal posting tugas dan rapat (warna: tenggat = aksen, lewat tenggat = danger, rapat = biru "doing", posting = amber "review"), legenda. Navigasi bulan/minggu sebelumnya/berikutnya + "Hari ini" (tidak ada di canvas, ditambahkan agar bulan lain bisa dilihat). Klik item: tugas → detail tugas di Board (pola yang sudah ada), rapat → tab Rapat dengan rapat itu dipilih. HP: sel bulan memakai strip warna, daftar bulan di bawah grid. Data dari tugas dan rapat yang sudah dimuat Space; zona waktu periode.
- **Anggota** (`members_view.go`, baru; `members.go` dirapikan): "Anggota dan role" + deskripsi canvas, tombol "Undang anggota" (membuka form email; izin undangan), tabel Nama (avatar) / Role (satu badge, Kadiv/45 bergaris aksen) / Subdivisi (kelompok) / Beban aktif (jumlah tugas belum selesai/batal dari tugas Space) / WhatsApp, aksi Ping (membuka komposer ping dengan penerima anggota itu) dan Ubah role per baris. Ubah role membuka panel di bawah baris berisi aksi lama: role dasar, kelompok, role kustom, Simpan akses, Hapus anggota, Tunjuk/Copot 45 — semua tetap mengikuti `memberCan`. "Kelola izin arsip" dan muat ulang di kepala halaman. Kartu "Undangan tertunda" (kirim ulang, cabut), kartu "Role kustom" (bila boleh), kartu "Kewenangan role". HP / lebar < 720: kartu ringkas per anggota.
- **Reminder pribadi** (`reminders_view.go`): kepala halaman canvas (judul, deskripsi, tombol primer "Reminder baru"), split view daftar ringkas kiri (ikon, judul, waktu, label Privat/Dibagikan · n) + panel formulir kanan; lebar daftar awal 360 px dan tetap bisa diubah dari tepi (sudah ada). Di desktop langsung memilih reminder pertama (sekali per muat, jadi menutup panel tetap berfungsi). HP: panel menggantikan daftar. "Reminder baru" tidak lagi tertahan selama detail reminder terpilih dimuat (`reminderCreateBlocked`).
- **Arsip** (`archive_page.go`, baru; fungsi lama dihapus dari `pages.go`): judul + deskripsi canvas, kartu per periode tertutup (label, "Ditutup <tanggal>", status hanya baca, "Buka (hanya baca)"), grid 3/2/1 kolom. Kartu "Serah terima" hanya untuk admin organisasi (fitur checklist serah terima memang ada di Pengaturan organisasi), tombolnya membuka tab Serah terima.

## Dilewati / berbeda dan alasannya
- Kolom **WhatsApp** di tabel Anggota menampilkan "—": API belum punya status tautan WhatsApp per anggota.
- Kartu periode Arsip tanpa jumlah tugas/catatan dan tanpa "Minta akses ke kadiv": data jumlah dan alur permintaan akses tidak ada di API.
- Undangan tertunda tanpa pilihan role (undangan di API hanya email).
- Kontrol Reminder "Ulangi" dan toggle "Bagikan reminder ini" gaya canvas tidak dibuat: reminder di API tidak punya pengulangan; berbagi tetap memakai kontrol yang ada di panel.
- To-do catatan Tahap 4 menyebut "Pengaturan global", tapi brief Tahap 4 sesi ini tidak memasukkannya; tidak dikerjakan.
- Tidak ada cek visual (sesuai brief, sesi utama yang mengecek).

## File yang disentuh
Baru: `internal/appui/agenda_calendar.go`, `members_view.go`, `archive_page.go`, `canvas_tahap4_test.go`.
Diubah: `pages.go` (panggil kalender, `archivePage` pindah), `app.go` (satu field `agenda agendaState`), `members.go`, `reminders_view.go`, `reminders_controller.go`, `reminders_layout_test.go` (klik "Reminder baru"), `app_test.go` (teks halaman Arsip). `tasks_view.go` tidak disentuh (`taskAgenda` lama kini tidak dipanggil; boleh dihapus sesi pemilik file).

## Test
`flock /tmp/space-build.lock env GOMAXPROCS=1 GOFLAGS=-p=1 GOMEMLIMIT=256MiB GOGC=50 go test ./internal/appui ./internal/workspaceui`:

```
--- FAIL: TestNoteDocumentNativeSpanEditingPreservesOtherRichRuns (0.00s)
    notes_document_test.go:88: document control "Teks bagian" missing in ["Tambah blok" "Tambah blok" "Blok 1–1 dari 1" "Konsep referensi" "referensi" "Atur blok 1" "Format dan atur blok"]
--- FAIL: TestNoteDocumentExtractionCopiesOnlyChosenTextAndNeverEditsReader (0.00s)
    notes_document_test.go:199: ui: no element shows "Jadikan tugas dari blok 1"
--- FAIL: TestNoteDocumentNativeBlockDeletionHandlesLastVisibleBlock (0.00s)
    notes_document_test.go:218: ui: no element shows "Hapus blok"
FAIL
FAIL	github.com/himasifunej/himasif-space/internal/appui	14.620s
ok  	github.com/himasifunej/himasif-space/internal/workspaceui	(cached)
FAIL
```

Hanya 3 test NoteDocument yang memang sudah gagal sejak dulu. Test baru: kalender bulan/minggu, penggabungan item agenda, badge role dan beban aktif, tabel Anggota dengan Ubah role, Arsip tanpa serah terima untuk non-admin.
