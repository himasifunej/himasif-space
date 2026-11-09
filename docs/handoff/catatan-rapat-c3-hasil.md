# Tahap C3 — Interaksi blok ala Notion

Branch `feat/catatan-c3`, di-rebase ke `origin/main` 3e2c2b5 (0.3.4, berisi C2). Desain: canvas `8eef2f092aab`, desain 1 (A · Desktop) dan 2 (A · HP). Tidak di-deploy, tidak di-push.

## Spike MyGo v0.7.1 (dibuktikan dengan test sementara, tidak di-commit)
| Kebutuhan | Hasil | Cara |
|---|---|---|
| Handle + dan ⠇ saat hover | **Bisa** | `Hovered()` pada baris blok tetap benar saat penunjuk di atas input di dalamnya |
| Drag untuk memindah blok | **Bisa** | `Drag(value)` pada ⠇, `Drop[T]`/`DragOver[T]` pada baris tujuan (garis aksen di atas baris saat melayang) |
| Menu "/" di posisi kursor | **Sebagian** | "/" terdeteksi lewat `Changed`, offset caret lewat `TextSelection`; **tidak ada API posisi caret di layar**, jadi menu menempel di bawah blok yang sedang diketik (naik ke atas bila ruang di bawah tidak cukup) |
| Fokus/caret antar blok | **Bisa** | `OnTextKey` menangkap Enter/Backspace/↑/↓/Esc sebelum input, `Focus()` di frame berikutnya |
| Bilah blok di atas keyboard (HP) | **Sebagian** | tidak ada API tinggi keyboard; bilah menempel di bawah area dokumen, jadi berada di atas keyboard bila viewport menyusut (browser) |

Cadangan rencana dipakai hanya di HP (tanpa hover): tanpa gutter, Naik/Turun dan Pilihan blok ada di bilah blok.

## Yang dibangun
- **Gutter + / ⠇** di kiri setiap blok (menggantung di luar kolom 720 px, teks tetap rata dengan judul), tampil saat hover atau saat alat blok terbuka.
  - **+** menyisipkan baris kosong di bawah dan membuka menu blok.
  - **⠇** diseret ke blok lain untuk memindah; diklik membuka alat blok (Naik/Turun, Hapus, Jadikan tugas/reminder, format teks dan tautan, H1–H3, butir/baris/kolom).
- **Menu blok** ("/" di awal baris kosong, atau "+"): grup **Notulen** (Action item, Keputusan, Kehadiran) di atas **Blok dasar** (Teks, Judul bagian, Subjudul, Daftar poin, Daftar bernomor, Checklist, Toggle, Callout, Tabel, Pemisah). Teks setelah "/" menyaring (`/kep` → Keputusan, `/agenda` → Daftar bernomor). ↑/↓ memilih, Enter memakai, Esc menutup dan teks tetap ada.
  - **Kehadiran** mengisi daftar dari peserta rapat ("Laras Foto — izin (kuliah pengganti)"); tanpa rapat, satu butir kosong.
  - **Action item** = checklist (PJ/tenggat/Jadikan tugas sebagai chip menyusul di C4).
- **Keyboard**: Enter membuat blok/butir berikutnya dan caret pindah ke sana; Shift+Enter baris baru dalam paragraf; Enter pada butir terakhir yang kosong keluar dari daftar; Backspace pada baris/butir kosong menghapusnya dan caret kembali ke atas. Penanda Markdown ("# ", "- ", "1. ", "[] ") tetap jalan.
- **Blok C1 bisa dibuat dan disunting**: callout (Info/Keputusan, kotak berwarna dengan ikon, isi diketik langsung) dan toggle (judul, baris isi, Tambah baris, lipat/buka per pembaca, satu tingkat sesuai server).
- **Label teknis dihapus**: "Format dan atur blok", "Butir blok 2 1–3 dari 3", "Blok 1–6 dari 6" (pager hanya muncul bila lebih dari satu halaman, teks "Butir 9–16 dari 20"), "Isi catatan · hanya baca", tombol "Tambah poin" (diganti Enter), tombol "Tambah blok" besar di atas (diganti baris "Klik untuk menambah blok, atau ketik / di baris kosong" di akhir).
- **Lonceng/tugas per butir** tidak lagi selalu tampil: muncul saat hover butir/baris/blok (tetap ada untuk keyboard dan pembaca layar). Pembaca di HP membukanya lewat ⋯ per blok; notulis lewat Pilihan blok. Checklist memakai kotak centang, bukan ikon plus.
- **HP**: bilah blok di bawah dokumen (+, H2, Checklist, Keputusan, Naikkan, Turunkan, Pilihan blok); menu blok tampil sebagai grid ubin dengan grup Notulen diberi latar aksen.
- **Tetap**: kunci editor tunggal, paging 8 blok/halaman (dan butir/baris/kolom), autosave C2. Judul bagian yang baru dipilih dan masih kosong disimpan sebagai paragraf kosong supaya autosave tidak ditolak validasi server (judul heading wajib ≥ 1 karakter); editor tetap menampilkan judulnya.

## Belum (sesuai rencana)
- @ untuk anggota/tugas, #tag, chip Jadikan tugas di Action item (C4); urungkan (belum ada riwayat urungkan di editor); handle per butir di dalam daftar.

## Berkas
- `internal/appui/notes_blocks_c3.go` (baru): menu blok, gutter dan drag, fokus antar blok, Enter/Backspace, paragraf/callout/toggle, daftar, pager, ekstraksi saat hover, bilah blok HP, `noteSaveContent`.
- `internal/appui/notes_document.go`: field C3, label dan `addBlock` untuk callout/toggle, ekstraksi baris tabel saat hover; render lama dipindah.
- `internal/appui/notes_view.go`, `notes_controller.go`, `notes_layout.go`: mode HP dan Kehadiran ke editor, bilah blok HP, `noteSaveContent` saat autosave/draf, `noteAttendanceLines`.
- Test: `internal/appui/notes_c3_test.go` (baru: urutan/saring menu, "/" → Keputusan + caret + Esc, "+" → Kehadiran/Action item, drag ⠇ memindah blok + alat blok, Enter/Backspace antar blok, toggle/callout dibuat dan lolos `ValidateNoteContent`, tanpa label teknis, bilah HP, screenshot). `notes_blocks_c1_test.go` diperbarui dengan sengaja: callout dan toggle kini disunting di input, dicari lewat labelnya.
- Screenshot (renderer native, tema terang, data dummy) di `docs/screenshots/`: `catatan-c3-desktop-hover.png` (+ / ⠇ dan lonceng butir saat hover), `catatan-c3-desktop-menu.png` (menu "/"), `catatan-c3-desktop-pembaca-hover.png`, `catatan-c3-hp.png` (bilah blok), `catatan-c3-hp-menu.png` (grid blok); screenshot C2 diambil ulang. Dibuat ulang dengan `SPACE_UI_CAPTURE_DIR=<dir> go test ./internal/appui -run 'TestNotesC[23]NativeCapture'`.

## Hasil cek (Windows, go1.27.3)
```
gofmt -l (berkas yang diubah)   → (kosong)
go vet ./...                    → ok
go test ./internal/appui        → ok
go test ./...                   → 13 paket ok; 1 gagal:
--- FAIL: TestImmutableBlobAndLimitCleanup
    files_test.go:18: attachment storage must be a private directory (0700)
```
`internal/filestore` gagal hanya karena cek izin 0700 yang tidak berlaku di Windows (berkas tidak disentuh). Jalankan ulang di Linux/VPS sebelum digabung.

## Untuk dicek owner di app nyata
- Desktop: arahkan penunjuk ke blok → + dan ⠇ muncul; seret ⠇ ke blok lain; ketik "/" di baris kosong lalu "kep" dan Enter.
- Enter/Backspace di daftar dan paragraf terasa seperti Notion (caret ikut pindah).
- HP (web): bilah blok tetap terlihat saat keyboard terbuka.
