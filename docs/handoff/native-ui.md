# Handoff UI native — irisan awal

> Catatan historis irisan awal. Integrasi backend 1B–1C dan inti tugas berikutnya dijelaskan di [handoff integrasi](native-integration-tasks.md); batas “belum tersedia” di bawah hanya berlaku pada commit UI awal.

Tanggal: 27 Oktober 2026. Branch `feat/native-ui`, basis main `627298f` (PR #1).

## Hasil

Prototype HTML 03.1 yang disetujui pengguna dipindahkan menjadi komponen Go native MyGo v0.7.1. Aplikasi desktop dan WASM menggunakan sumber yang sama; HTML prototype tetap referensi visual terpisah.

- Sidebar tetap tanpa scroll, tombol collapse di bawah Penampilan; header tanpa duplikasi tombol sidebar/aksen. Pada jendela pendek, submenu Space berpindah ke tab konten agar kontrol profil tetap terjangkau. Detail tugas dan Penampilan bergantian memakai panel kanan agar board tetap dapat dipakai pada lebar minimum.
- Header dengan pencarian terlihat dan Ctrl/Cmd+K, dropdown kustom pencarian/notifikasi, menu profil termasuk logout sesi asli. Pencarian saat ini hanya navigasi halaman; layanan pencarian isi belum tersedia.
- Footer koneksi memakai pemeriksaan API/ping yang sudah ada dan aksi coba lagi ketika offline. Label ini menyatakan keterjangkauan server; belum merupakan bukti antrean data tersinkron ke cloud.
- Halaman pribadi, direktori divisi, Space divisi/Medhum, beranda, board, agenda mingguan, notes/template, anggota, arsip, akun dan pengaturan WhatsApp. Akun/periode/divisi berasal dari API 1A; layanan lainnya menampilkan kondisi belum tersedia.
- Logo Space dan logo default Mediatek/Humas disematkan dari prototype; sumber/lisensi dicatat di `internal/appui/assets/SOURCES.md`. Penggantian logo divisi belum memiliki endpoint.
- Tema gelap/terang/sistem, lima aksen dan kode hex kustom, density dan collapse sidebar disimpan lokal. Desktop memakai JSON privat; browser memakai localStorage khusus preferensi. Token tetap di credential store OS atau sessionStorage sebagaimana fondasi.
- `internal/workspaceui` menerima salinan snapshot tugas dengan ID bertipe, memvalidasi identitas/lane, menyediakan native drag/drop lintas kolom, reorder, menu pindah untuk keyboard, horizontal edge scroll dan inspector kanan yang mengambil ruang layout. `OnMove` hanya mengirim intent; callback nil membuatnya read-only. Tidak ada simulasi keberhasilan simpan server.

## Batas dan penyambungan berikut

Backend 1A belum menyediakan daftar Space berizin, role efektif, tugas, notes, notifikasi atau sesi WA. Direktori divisi tidak membuktikan membership. Tidak ada data contoh yang dimasukkan ke aplikasi resmi, QR palsu, simpan bot palsu atau notifikasi tugas yang dijadwalkan. Board aplikasi menampilkan scaffold belum tersedia sampai snapshot tugas dapat dikonfirmasi. Komponen board diuji dengan fixture di tes saja.

Sesi backend 1B–1C memiliki server/store/migrasi/DTO baru; sesi UI memiliki app/appui/workspaceui/client. Jangan menggunakan nama divisi atau tombol UI sebagai otorisasi. Setelah kontrak backend siap, tambahkan adapter klien serta data Space/kapabilitas yang dikonfirmasi server, lalu tugas Sesi 2A. Aturan 45, pemisahan internal divisi dan akses arsip harus ditegakkan server.

Notifikasi native Windows, tray, suara, Ace, offline cache/antrean dan Flutter belum dikerjakan dalam irisan ini. Cross-build Windows tidak menggantikan verifikasi pada Windows nyata. Template notes/agenda ditampilkan sebagai arah UI; editor dan penyimpanan menunggu layanannya.

## Menjalankan

Gunakan Go sesuai go.mod. Pada STB arahkan cache ke HDD (`GOCACHE=/mnt/stb/cache/go-build`, `GOPATH=/mnt/stb/go`) dan bekerja dari worktree HDD. `go tool mygo dev ./app` menjalankan desktop; `go tool mygo dev -web ./app` menjalankan WASM. Konfigurasi API/token tetap mengikuti README dan konfigurasi platform fondasi. Aplikasi memerlukan backend asli untuk login.

## Verifikasi

Semua pemeriksaan berikut lulus pada STB, dengan build serial `GOMAXPROCS=2 GOFLAGS=-p=1` dan temporary directory di HDD:

- `go test -count=1 ./...`, termasuk rendering/interaksi native, drag/drop/reorder/menu keyboard, inspector, route board unloaded/confirmed-empty/populated, auth/token/offline, pembersihan antar-akun/periode, preferensi dan tema sistem. Pengujian server yang memerlukan PostgreSQL **skip** karena `SPACE_TEST_DATABASE_URL` tidak diset; backend/migrasi tidak diubah.
- `go vet ./...`, `GOOS=windows go vet ./app ./internal/...` dan `GOOS=js GOARCH=wasm go vet ./app ./internal/appui ./internal/workspaceui ./internal/client ./internal/api`.
- `go tool mygo build -platform web ./app`: berhasil, WASM sekitar 18,3 MB di `app/build/web/`.
- `go tool mygo build -platform windows/amd64 ./app`: berhasil, executable sekitar 11,6 MB di `app/build/windows-amd64/Space.exe`. Installer tidak dibuat karena NSIS belum tersedia. Menjalankan aplikasi di Windows nyata tetap pemeriksaan terpisah.
- Gofmt dan `git diff --check` bersih.

Tangkapan renderer native ditinjau untuk mode gelap/terang, dropdown, WhatsApp, board/inspector, sidebar collapse, serta 880×640 dengan panel Penampilan. Fixture akun dan tugas hanya hidup di tes. Membuat PNG ulang secara opsional: `SPACE_UI_CAPTURE_DIR=/path/out go test ./internal/appui -run TestNativeCapture -count=1`.
