# Tahap A1 — Tabel Anggota (baca)

Branch `feat/anggota-a1` dari `origin/main` (b17a3c6). Desain: canvas Anggota `0b2425c385f0`, desain 1 (A · Desktop) dan 2 (A · HP). Rencana: `docs/handoff/anggota-catatan-rapat-rencana.md` (branch `design/anggota-dan-catatan-rapat`). Tidak di-deploy, tidak di-push.

## Yang sudah sesuai desain
- **Kepala halaman**: judul "Anggota", "N anggota · Beban kerja terlihat oleh semua anggota. Hanya kadiv yang mengatur role, undangan, dan 45.", tombol **Izin arsip** (outline, hanya bila boleh) dan **Undang anggota** (utama), muat ulang.
- **Bilah alat**: cari (nama, kelompok, label role), chip filter **Semua / Pengurus / Anggota / Belum tertaut / Ada yang lewat** dengan jumlah, tombol **Urutkan: Peran → Beban → Nama**. Chip "Ada yang lewat" disembunyikan selama tugas belum dimuat.
- **Tabel** (lebar ≥ 900 px): Anggota (avatar 32 + nama + "kamu"), Role (badge: Kadiv gelap dengan ikon, "45 · Anggota" aksen, "Pendamping kadiv" bergaris, Anggota netral, role kustom netral), Kelompok (huruf awal kapital), **Beban aktif** (bar 8 segmen: lewat tenggat merah, aktif abu, sisa kosong + "5 aktif · 1 lewat" / "Kosong"), **WhatsApp** (ikon + "Tertaut" hijau, jam + "Belum tertaut" kuning, "—" bila server tidak memberi tahu), aksi (Ping, Ubah role; baris kadiv sendiri "Pemilik divisi").
- **HP / lebar sempit**: kartu dengan avatar 40, nama + ikon WA kuning bila belum tertaut, badge role, kelompok, bar mini + teks beban, tombol Ping 44 px; "Ubah role" di baris bawah bila boleh.
- **Keadaan kosong**: divisi tanpa anggota (pesan + ajakan undang), dan cari/filter tanpa hasil ("Tidak ada anggota bernama “zaki”" + **Hapus pencarian dan filter**).
- "Lewat tenggat" memakai `taskLate` yang sama dengan board, dari tugas Space yang sudah dimuat; tidak ada request baru.

## Dilewati (sesuai rencana)
- Drawer/sheet **Ubah akses** (A2): editor masih terbuka di bawah baris dari "Ubah role", hanya baris yang sedang disunting kini diberi latar aksen.
- Dialog Keluarkan dengan syarat tugas dialihkan (A3), menu ⋯ baris dan tata ulang panel bawah (A4).
- **Email/NIM** di baris: tidak ditampilkan (API tidak mengirimnya; usulan NIM di §8 rencana).

## Berkas
- `internal/appui/members_table.go` (baru): filter/cari/urut (`visibleMembers`, `memberMatches`, `memberWork`), badge role, bar beban, status WA, bilah alat, tabel, baris, kartu.
- `internal/appui/members_view.go`: kepala halaman, tombol Izin arsip, `toggleMemberEditor`; tabel lama dipindah ke berkas baru.
- `internal/appui/members.go`: state `query`, `filter`, `sort`.
- Test: `internal/appui/members_table_test.go` (baru); `canvas_tahap4_test.go` (judul "Anggota"), `canvas_tahap5_test.go` ("Belum tertaut", kartu HP baru) diperbarui dengan sengaja.
- Screenshot (renderer native, tema terang, data dummy): `docs/screenshots/anggota-a1-desktop.png`, `anggota-a1-desktop-filter-lewat.png`, `anggota-a1-desktop-kosong.png`, `anggota-a1-hp.png`. Dibuat ulang dengan `SPACE_UI_CAPTURE_DIR=<dir> go test ./internal/appui -run TestMembersTableNativeCapture`.

## Hasil cek (Windows, go1.27.3)
```
gofmt -l internal/appui/members*.go        → (kosong)
go vet ./...                               → ok
go test ./internal/appui -count=1          → ok  github.com/himasifunej/himasif-space/internal/appui  17.820s
go test ./...                              → 13 paket ok; 1 gagal:
--- FAIL: TestImmutableBlobAndLimitCleanup (0.00s)
    files_test.go:18: attachment storage must be a private directory (0700)
FAIL	github.com/himasifunej/himasif-space/internal/filestore
```
Kegagalan `internal/filestore` adalah cek izin direktori Unix 0700 yang tidak berlaku di Windows; berkas itu tidak disentuh tahap ini. Perlu dijalankan ulang di Linux/VPS (dengan lock memori seperti brief) sebelum digabung.

## Catatan untuk pengecekan owner
- Di screenshot tes, Ping tidak muncul karena fixture tidak memberi izin ping pada Space. Izin arsip kini tampil: fixture kadiv diberi `archives.manage`, sama seperti kadiv di server.

## Perbaikan sebelum rilis 0.3.2
- **Chip anggota di header**: sebelumnya tertulis "0 anggota" padahal tabel berisi 7.
  - Penyebab di screenshot: fake API tes tidak punya daftar kandidat rapat, jadi chip gagal memuat dan menulis 0.
  - Di app nyata ada dua kelemahan yang sama sumbernya. Pertama, chip menghitung kandidat rapat yang tidak memuat akun nonaktif, sedangkan tabel memuat semua keanggotaan. Kedua, selama memuat atau saat gagal, chip menulis "0 anggota".
  - Sekarang (`headerMemberCount`): di Space divisi yang tabelnya sudah dimuat, chip memakai jumlah tabel. Kalau tabel belum dimuat, chip memakai jumlah kandidat. Kalau keduanya belum ada, chip hanya menulis "Anggota" tanpa angka.
  - Test: `TestHeaderMemberChipMatchesTable`.
- **Izin arsip**: tombolnya sudah ada untuk yang berhak (`archives.manage`, bukan 45, periode belum ditutup). Di screenshot tidak tampil hanya karena fixture-nya.
  - Test: `TestMembersHeaderArchiveButton` (kadiv tampil; kadiv tanpa izin, 45 dan anggota tidak).
  - Screenshot dibuat ulang.
- Cek di app nyata: tabel di 1440 px dan di HP, chip "Belum tertaut" untuk anggota tanpa status WA (status `—` tidak ikut dihitung).
