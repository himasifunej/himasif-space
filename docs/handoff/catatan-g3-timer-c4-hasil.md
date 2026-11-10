# Catatan — G3 (Edit | Baca, urungkan), T1 (timer lokal), C4 (mention, action item)

Branch `feat/catatan-g3-timer-c4` dari `origin/main` b38bb29 (0.3.6, gaya C G1+G2). Rencana: `docs/handoff/anggota-catatan-rapat-rencana.md` §1 (timer), §3 (blok), §6 (C4, T1), §9 (G3–G5). Tidak di-push ke main, tidak di-deploy. **Tanpa migrasi dan tanpa perubahan API**: semua memakai field yang sudah divalidasi server sejak C1 (`minutes`, `assigneeId`, `dueOn`, `taskId`, span `mention`).

## G3 — Edit | Baca + urungkan/ulangi
- **Edit | Baca** di bilah atas (hanya untuk pemegang editor). Baca menampilkan dokumen tanpa handle, menu, input, dan bilah blok HP. Kunci editor, autosave, dan Ctrl+S tetap berjalan. Saat editor dilepas, mode kembali ke Edit.
- **Urungkan / Ulangi** untuk seluruh dokumen:
  - Tersedia lewat tombol di bilah atas (desktop), di bilah blok HP, dan dengan Ctrl+Z, Ctrl+Shift+Z, atau Ctrl+Y.
  - Ketikan beruntun di satu blok (jeda < 1,2 dtk) dihitung satu langkah. Blok baru, pindah, hapus, centang, PJ, tenggat, dan durasi masing-masing satu langkah.
  - Maksimal 100 langkah.
  - Ctrl+Z diambil sebelum kotak teks memprosesnya, jadi hanya ada satu riwayat.
  - Hasil urungkan adalah perubahan biasa, jadi ikut tersimpan otomatis.
  - Riwayat dikosongkan saat isi resmi baru masuk atau draf dipulihkan.

## T1 — Timer rapat lokal
- **Tab Rapat → TIMER**:
  - Sebelum mulai: "Mulai timer · opsional · 45 mnt untuk 3 agenda". Tanpa durasi agenda, timer hanya menghitung total.
  - Saat berjalan:
    - Total waktu dan "dari 45:00 · Agenda 2".
    - Daftar agenda: yang selesai ditandai "✓ 16:00", yang berjalan "08:12 tersisa" dengan bar, yang berikutnya menampilkan durasi rencana.
    - Tombol: Jeda/Lanjutkan, +5 mnt, Agenda berikutnya (atau Selesaikan agenda), Hentikan timer.
  - Ada keterangan "Timer hanya di perangkat ini".
- **Pil sticky** di atas dokumen, di luar area gulir (kanan di desktop, tengah di HP):
  - Isinya agenda sekarang, sisa waktu, total, Jeda, dan Sembunyikan (×).
  - Sembunyikan hanya menyembunyikan pil; timer tetap berjalan dan bisa dimunculkan lagi dengan "Tampilkan pil timer" di tab Rapat.
- **Waktu agenda habis**: pil menjadi kuning, "Waktu agenda 1 habis +02:00" (di HP cukup "Habis"), dengan tombol **+5 mnt** dan **Agenda 2 →**. Tanpa suara dan tanpa dialog.
- **Durasi agenda**:
  - Chip "⏱ 15 mnt" di butir daftar bernomor.
  - Notulis memilih 5/10/15/20/30/45/60 mnt atau menghapusnya.
  - Pembaca melihat chip itu tetapi tidak bisa mengubahnya.
  - Judul dengan `minutes` juga dihitung sebagai agenda.
- **Penyimpanan**:
  - Timer tidak menulis isi catatan dan tidak menyentuh kunci editor.
  - Timer disimpan di memori aplikasi per catatan, jadi hilang saat aplikasi ditutup ({{Asumsi}} cukup untuk T1; T2 akan memindahkannya ke server).
  - Waktu jeda tidak dihitung, dan rapat yang melewati tengah malam tetap dihitung benar.

## C4 — Mention dan action item
- **@ di paragraf, callout, atau butir daftar** membuka pemilih "Sebut anggota atau tugas":
  - Isinya anggota divisi pemilik catatan dan tugas Space (dicari lewat nomor #T atau judul).
  - Navigasi dengan ↑/↓, pilih dengan Enter/Tab, tutup dengan Esc.
  - Pilihan menjadi span mention berteks "@Nama" atau "#T22 Judul", sehingga klien lama dan tampilan publik tetap bisa membaca namanya.
  - Pembaca melihat mention berwarna aksen dan tebal.
- **Teks bermention tetap bisa diketik langsung**:
  - Mention yang teksnya masih utuh tetap dipertahankan.
  - Mention yang teksnya diubah menjadi teks biasa.
  - Teks tebal atau bertautan tetap diubah lewat "Atur blok" seperti sebelumnya.
- **Action item = butir checklist**, dengan chip:
  - **PJ**: pemilih anggota dengan pencarian, bisa dihapus.
  - **Tenggat**: Hari ini, Besok, Lusa, Minggu depan, atau tanggal YYYY-MM-DD. Tenggat lewat yang belum dicentang berwarna merah.
  - Chip kosong hanya muncul saat butir disorot, jadi checklist biasa tetap polos.
- **Jadikan tugas** dari action item:
  - Membuka form tugas yang sudah ada di panel, dengan judul, PJ, dan tenggat terisi.
  - Setelah tugas dibuat, `taskId` disimpan di butir itu (tetap ketemu walau butir sempat dipindah) dan tampil sebagai chip **#T22**. Klik chip membuka tugasnya.
  - Tautan catatan (`NoteLink`) tetap dibuat seperti sebelumnya.
  - Butir tanpa PJ tetap memakai form biasa.
  - Pembaca tetap memakai "Jadikan reminder/tugas" seperti sebelumnya, tanpa mengisi `taskId`.

## Yang belum
- **G4 (pohon catatan dengan saringan)**: ditunda ke rilis berikutnya.
- **Komentar per blok**: menunggu keputusan owner.
- **Mention belum bisa diklik di dalam teks**: chip #T pada action item bisa diklik.
- **Pemilih @ hanya bekerja di akhir teks**: MyGo v0.7.1 belum memberi posisi kursor.

## Berkas
- **Kode baru**:
  - `internal/appui/notes_g3.go`: riwayat urungkan dan tombol Ctrl+Z di kotak teks.
  - `notes_timer.go`: `noteTimerState` murni, tab Timer, pil, chip durasi.
  - `notes_c4.go`: mention, chip action item, `SetItemTask`.
  - `notes_c4_app.go`: anggota divisi, target mention, buka tugas, Edit | Baca, tombol urungkan.
- **Kode yang diubah**:
  - `notes_document.go`, `notes_blocks_c3.go`: kait editor (riwayat, mention, chip).
  - `notes_gaya_c.go`: pil, shortcut, Edit | Baca, Timer di tab Rapat.
  - `notes_controller.go`: state timer, mode baca, anggota, `source` konversi yang mengisi `taskId`.
  - `icons.go`: ikon undo, redo, timer, pause, play, eye, at.
- **Test**:
  - Baru di `notes_g3_c4_test.go`:
    - Tabel kasus `noteTimerState`: jeda, lanjut, lewat, +5, agenda berikutnya, selesai, tanpa agenda, lewat tengah malam.
    - Agenda dari blok.
    - Panel timer dan pil: habis → agenda berikutnya, jeda, sembunyikan, hentikan, dan catatan tidak berubah.
    - Urungkan/ulangi di editor.
    - Ctrl+Z / Ctrl+Shift+Z di halaman dan mode Baca.
    - Respan mention dan deteksi @.
    - Pemilih @ dengan Enter.
    - Chip PJ dan tenggat, Jadikan tugas terisi, `taskId` kembali ke butir yang sudah dipindah.
  - Diubah dengan sengaja: `notes_blocks_c1_test.go`, karena paragraf bermention sekarang diketik langsung sehingga label inputnya yang dicari.
- **Screenshot** (2, renderer native, data dummy):
  - `docs/screenshots/catatan-g3-timer-c4-desktop.png`: notulis, agenda 1 habis, PJ dan tenggat, mention.
  - `catatan-g3-timer-c4-hp.png`: pil kuning di HP.

## Hasil cek (Windows)
```
gofmt -l (berkas yang diubah)  → (kosong)
go vet ./...                   → ok
go test ./internal/appui       → ok
go test ./...                  → semua paket ok kecuali internal/filestore (cek izin 0700 tidak berlaku di Windows, tidak disentuh)
```
