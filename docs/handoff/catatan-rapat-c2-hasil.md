# Tahap C2 — Tata letak dokumen catatan + autosave

Branch `feat/catatan-c2` dari `origin/main` 8c63805 (0.3.3, berisi C1 dan A2). Desain: canvas `8eef2f092aab`, desain 1 (A · Desktop) dan 2 (A · HP). Rencana: `docs/handoff/anggota-catatan-rapat-rencana.md` §4 C2, §7 (branch `design/anggota-dan-catatan-rapat`). Tidak di-deploy, tidak di-push. Area Anggota tidak disentuh (A3 dikerjakan sesi lain).

## Yang dibangun
- **Bilah dokumen** (desktop): remah **Rapat › judul** (Rapat membuka rapat terkait di Agenda; catatan tanpa rapat: **Catatan › judul**), label **Notulen** (template rapat) / Jadwal, status simpan, avatar orang yang sedang mencatat, chip **Hanya divisi ini** (tampilan saja, tooltip "Pengaturan berbagi menyusul"; C6), tutup.
  - Status: pembaca "Diedit baru saja / 12 menit lalu / pukul 19.42 / kemarin 19.42 / 9 Nov 19.42"; 10 detik setelah versi baru tiba "Diperbarui 19.42 oleh Sinta". Notulis "Menyimpan otomatis…" → "Menyimpan…" → "Tersimpan · baru saja"; gagal "Belum tersimpan — coba lagi" + tombol **Coba simpan lagi**.
- **Judul besar** 30 px (HP 24 px), **membungkus ke beberapa baris** di desktop dan HP: untuk notulis text area tanpa bingkai setinggi teksnya (1–6 baris, Enter tidak menambah baris, baris baru hasil tempel disimpan sebagai spasi), untuk pembaca teks biasa. Remah di bilah dokumen sengaja tetap satu baris.
- **Properti rapat** dari `Meeting` (daftar rapat Agenda yang sudah dimuat, tanpa endpoint baru): **Tanggal** "Senin, 9 Nov 2026 · 19.30–20.45" (zona waktu rapat), **Tempat** (lokasi, "+ rapat daring" bila ada tautan), **Kehadiran** dari `Meeting.Participants` (avatar + nama depan; izin/tidak hadir/belum dicatat pudar dengan alasan) + ringkasan "6 hadir · 1 izin". Catatan tanpa rapat: **Dibuat** tanggal + pembuat. Rapat yang tidak bisa diakses: "Data rapat tidak tersedia untuk akunmu."
- **Kolom dokumen 720 px** di tengah; semua bagian lama (blok, draf pemulihan, komentar, lampiran, template, riwayat, hapus) ikut di kolom itu.
- **Kunci editor tetap** (satu penyunting, Ambil alih dengan konfirmasi), kini satu strip ramping di bawah properti: "Kamu memegang editor · sampai 16.22 · Selesai mencatat" / "Sinta Notulis sedang mencatat · sampai … · Ambil alih editor". **Paging blok 8/halaman tetap.**
- **HP**: daftar dan isi bergantian (catatan terbuka memenuhi layar, tombol kembali di bilah), bilah = kembali + status + ikon kunci, properti **dilipat** jadi satu baris "Sen 9 Nov · 19.30 · 6 hadir · 1 izin" yang membuka daftar lengkap.

## Autosave (keputusan owner 9 Nov, rencana §7)
- Simpan resmi otomatis **4 detik setelah berhenti mengetik**, dan **paling lambat tiap 30 detik** saat terus mengetik (`noteAutosaveQuiet/Max`, `tickNotes`).
- Jalur simpan baru `commitNote` **tidak memakai `busy`**, jadi editor tetap bisa diketik selama menyimpan, dan **editor tidak dibuat ulang** setelah simpan (dulu `newNoteDocumentEditor` memindahkan fokus/halaman). Hanya baseline dan versi yang diperbarui.
- Ketikan selama simpan berjalan tetap ada dan tetap "belum tersimpan"; simpan berikutnya memakai versi yang baru dikembalikan (`ExpectedVersion`).
- Konflik versi/kunci (409) membuka tinjauan dua versi seperti sebelumnya; galat jaringan/server dicoba lagi setelah 15 detik, salinan perangkat dan draf pemulihan server tetap jadi jaring pengaman (draf pemulihan 3 detik kini hanya dikirim saat autosave tidak bisa jalan).
- "Selesai mencatat" menyimpan yang belum tersimpan dulu, baru melepas kunci. "Simpan sekarang" tetap ada di bilah bawah. Tombol "Simpan draf pemulihan ke server" dihapus (otomatis).
- **Pembaca**: pembaruan realtime mengganti isi editor baca **di tempat** (`noteDocumentEditor.Replace`): kunci tampilan sama sehingga gulir tidak melompat, halaman blok dipertahankan. Hasil muat yang berangkat sebelum autosave sendiri selesai (versi lebih lama) diabaikan, dan autosave yang sedang berjalan tidak dianggap konflik.
- **Riwayat**: simpanan beruntun oleh orang yang sama dengan jeda ≤ 5 menit tampil sebagai satu baris "Isi resmi diperbarui · 6 kali disimpan · 19.31–19.42". Penggabungan dilakukan di klien: tabel `note_events` server append-only (trigger), jadi tetap satu event per simpan dan tidak ada migrasi. Ini beda dari rencana §7 poin 4 (yang menyebut penggabungan di server).

## Belum (sesuai lingkup)
- Handle +/⠇, menu /, @, #tag (C3), timer (T1), privasi/Bagikan sungguhan (C6), avatar banyak pembaca (butuh presence).

## Berkas
- `internal/appui/notes_layout.go` (baru): bilah dokumen, judul, properti rapat, kehadiran, status simpan, waktu relatif, `groupNoteHistory`.
- `internal/appui/notes_controller.go`: `commitNote`, `noteAutosaveDue/Active`, `noteSaveFailed`, `finishNoteEditing` menunggu simpan, guard versi lama/simpan berjalan di `loadNoteDetail`, state autosave.
- `internal/appui/notes_view.go`: HP daftar/isi, kolom 720, strip kunci, bilah bawah autosave, riwayat tergabung.
- `internal/appui/notes_document.go`: `Replace`.
- `internal/appui/notes_c2_test.go` (baru): autosave setelah jeda dan editor tetap sama, batas 30 detik, ketikan selama simpan, konflik → tinjauan, galat → coba lagi, pembaca tanpa lompat + "Diperbarui … oleh Sinta", penggabungan riwayat, waktu relatif, tata letak desktop/HP (lipat, buka, kembali), judul panjang membungkus (notulis dan pembaca, 390 dan 1440 px, Enter tanpa baris baru), bilah notulis dan catatan tanpa rapat, screenshot.
- Screenshot (renderer native, tema terang, data dummy) di `docs/screenshots/`: `catatan-c2-desktop-notulis.png`, `catatan-c2-desktop-pembaca.png`, `catatan-c2-hp.png`, `catatan-c2-hp-properti.png`, `catatan-c2-hp-judul-panjang.png`, `catatan-c2-desktop-judul-panjang.png`. Dibuat ulang dengan `SPACE_UI_CAPTURE_DIR=<dir> go test ./internal/appui -run TestNotesC2NativeCapture`.

## Hasil cek (Windows, go1.27.3)
```
gofmt -l (berkas yang diubah)          → (kosong)
go vet ./...                           → ok
go test ./internal/appui -count=1      → ok  17.9s
go test ./...                          → 13 paket ok; 1 gagal:
--- FAIL: TestImmutableBlobAndLimitCleanup
    files_test.go:18: attachment storage must be a private directory (0700)
FAIL	github.com/himasifunej/himasif-space/internal/filestore
```
Kegagalan `internal/filestore` adalah cek izin direktori Unix 0700 yang tidak berlaku di Windows (berkas tidak disentuh, sama seperti A1). Jalankan ulang di Linux/VPS sebelum digabung.

## Untuk dicek owner di app nyata
- Dua perangkat: notulis mengetik lalu diam ±4 detik → pembaca melihat isi baru tanpa reload dan tanpa gulir melompat.
- Ketik terus > 30 detik → status "Tersimpan" muncul di tengah mengetik tanpa kehilangan fokus.
- Matikan jaringan saat mengetik → "Belum tersimpan — coba lagi", lalu tersimpan sendiri setelah tersambung.
- Riwayat setelah satu rapat: simpanan otomatis tergabung per ±5 menit.
