# Catatan gaya C — G1 (panel bertab + bilah tipis) dan G2 (menu "/" ringkas)

Branch `feat/catatan-gaya-c` dari `origin/main` 79253d3 (0.3.5). Desain: canvas `37348413b3ef`, desain 5 (C · Desktop, winner) dan 6 (C · HP, bintang). Rencana: `docs/handoff/anggota-catatan-rapat-rencana.md` §9 (tahap G1–G5). Tidak di-deploy, tidak di-push. Tanpa migrasi atau perubahan API.

## G1 — Panel bertab + bilah tipis
- **Bilah atas 44 px**: remah Rapat / judul (Rapat membuka Agenda), chip kuning **Draf** hanya bila ada draf (membuka tab Riwayat), chip kunci editor ("Kamu memegang editor" / "Sinta Notulis sedang mencatat" / "Belum ada pencatat aktif", kunci kedaluwarsa tetap terbaca tidak terkunci) dengan satu tombol yang pas (Ambil editor, Ambil alih editor + konfirmasi di bawah bilah, Selesai mencatat, Edit draf offline), status simpan, buka/tutup panel, ⋯ (simpan sebagai template, hapus catatan), tutup. **Ctrl+S** menyimpan seketika.
- **Dokumen polos**: maks. 760 px; hanya judul, chip (tanggal, "6 hadir · 1 izin", Notulen, "1 komentar" — masing-masing membuka tab terkait), pemberitahuan galat bila ada, tinjauan draf bila dibuka, lalu blok. **Dihapus dari bawah dokumen**: kartu draf pemulihan, komentar, lampiran, template, rujukan, riwayat, hapus; juga strip kunci dan bilah "Simpan sekarang".
- **Panel kanan 320 px** (terbuka di desktop, tertutup di tablet sampai dibuka; pilihan pengguna diingat selama sesi), tab:
  - **Rapat**: tanggal, tempat, Buka di Agenda; **kehadiran per orang** dengan status berwarna dan alasan; **Di halaman ini** (judul bagian, klik → halaman blok itu dan dipilih); **lampiran** (daftar, unggah berkas, "Tambah lampiran tautan" membuka form). Catatan tanpa rapat: tanggal dibuat dan pembuat.
  - **Komentar** (badge jumlah): utas ringkas dan satu baris input (Enter atau tombol kirim).
  - **Riwayat** (badge jumlah draf): draf perangkat/server sebagai baris kecil (Tinjau, Buang), riwayat tergabung, pekerjaan dari catatan ini.
  - Form **Jadikan tugas/reminder** dari blok tampil di atas tab, di samping teksnya.
- **HP**: bilah = kembali, chip draf, kunci (avatar), status, ikon panel, ⋯; chip di bawah judul; panel menjadi **satu sheet bertab** (72 % tinggi, latar menutup); bilah blok C3 tetap.
- Timer belum ada (tahap T1/G5).

## G2 — Menu "/" ringkas gaya enowX
- Satu kolom, 260 px, baris 30 px: ikon/glyph, nama, **shortcut di kanan** (mono): Judul 1 `#`, Judul 2 `##`, Judul 3 `###`, Daftar poin `-`, Daftar bernomor `1.`, Checklist `[]`, Pemisah `---`; Notulen: Action item `/tugas`, Keputusan `/kep`, Kehadiran `/hadir`. Keterangan pindah ke tooltip. Grup Notulen tetap di atas.
- Shortcut yang tampil memang bekerja: `---` di baris kosong kini menjadi pemisah + baris baru (baru); "Judul 1" ditambahkan.
- Grid HP, handle +/⠇, drag, Enter/Backspace dari C3 tidak berubah.

## Tetap
Kunci editor tunggal + ambil alih, paging 8 blok, autosave + draf perangkat/server, realtime pembaca, data catatan.

## Berkas
- `internal/appui/notes_gaya_c.go` (baru): bilah, chip kunci, chip judul, panel/sheet bertab, tab Rapat/Komentar/Riwayat.
- `internal/appui/notes_view.go`, `notes_layout.go`: tampilan lama (strip kunci, properti, kartu bawah) dihapus.
- `internal/appui/notes_blocks_c3.go`: shortcut menu, Judul 1, `---`, `Headings`/`Reveal`, baris tambah blok rata kiri.
- `internal/appui/notes_controller.go`: state panel/sheet/⋯/lampiran.
- Test: `notes_gaya_c_test.go` (baru: panel bertab + tanpa kartu bawah + chip draf → Riwayat + tutup/buka panel; bilah notulis + form konversi di panel; outline pindah halaman; sheet HP; shortcut menu dan `---`). `notes_c2_test.go`: dua test tata letak C2 lama dihapus (digantikan); `notes_c3_test.go`: label "Judul 2" dan shortcut `/kep`.
- Screenshot (2, renderer native, tema terang, data dummy): `docs/screenshots/catatan-gaya-c-desktop.png`, `catatan-gaya-c-hp.png`.
- Rencana §9 ditambahkan ke `docs/handoff/anggota-catatan-rapat-rencana.md` (berkas ini sebelumnya hanya ada di branch desain).

## Hasil cek (Windows)
```
gofmt -l (berkas yang diubah)  → (kosong)
go vet ./...                   → ok
go test ./internal/appui       → ok
go test ./...                  → 13 paket ok; internal/filestore gagal (cek izin 0700 tidak berlaku di Windows, tidak disentuh)
```
