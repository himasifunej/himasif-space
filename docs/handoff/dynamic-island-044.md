# Dynamic Island desktop — kandidat 0.4.4

Rencana: catatan enowX `cfd2fb666954`. Desain: canvas `dceda2c54836`, ketiga gaya disetujui owner. Basis: `d8d2012` (0.4.3). Implementasi opsi A, jendela MyGo di proses Space. Tidak ada WebView/helper process dalam operasi normal. Tidak ada perubahan 9router, server, migrasi, atau deploy tc-vps.

## Yang dibangun

- **Fase 0**: spike native terisolasi, pengukuran RAM/CPU, Panel/ShowInactive/region Windows, script pengukuran yang dapat diulang. Uji visual/perangkat keras dipisahkan di checklist owner.
- **Fase 1**: Hidden/Banner/Expanded; satu banner 7 detik; hover 300 ms atau klik membuka detail; keluar pointer 1,5 detik menutup detail; keluar konten 120 ms. Buka memakai deep link Space, Tunda 15 menit memakai respons API yang sama tanpa mengambil fokus ke jendela utama.
- Antrean prioritas ping/rapat > tenggat > tugas > info, FIFO dalam prioritas yang sama; tag mengganti antrean/notice saat ini; batas 64 item menunggu, overflow ke Windows. Batch delivery tetap mempertahankan aksi setiap item saat island tersedia. Journal penerimaan delivery yang ada dipertahankan.
- `appui.MultiNotifier.ReplaceBase`: presenter tambahan yang menerima notice menekan toast OS. Gagal/tidak tersedia selalu kembali ke Base. Mode lama (OS + presenter) tetap tersedia. Permission dan hint tetap dari notifier OS.
- QUNS hanya mengizinkan `ACCEPTS_NOTIFICATIONS`; cek tambahan foreground borderless fullscreen berdasarkan monitor Windows. Dicek pada penerimaan dan maksimal setiap 2 detik ketika terlihat. Jika island harus sembunyi, notice yang belum selesai dialihkan ke OS; OS menentukan tampilan sesuai Focus Assist. Tidak ada replay ganda setelah keluar fullscreen.
- Preferensi perangkat: aktif (default), gaya **C** (default), ganti toast Windows (default), kurangi gerak (tambahan pilihan Windows). JSON lama memperoleh default baru tanpa mengubah preferensi Ace.
- A hitam dengan glyph jenis notifikasi; B cokelat hangat dan aksen Space; C permukaan kaca terang/gelap dengan native composition blur bila tersedia. Tema C mengikuti tema Space. Icon memakai aset Space yang sudah ada; tidak mengambil kode proyek referensi.
- Spring analitik mass 1, stiffness 170, damping 19, kecepatan diteruskan saat target berubah. Konten masuk delay 60 ms + 240 ms cubic-bezier(.2,0,0,1), transform/blur; konten keluar 120 ms cubic-bezier(.4,0,1,1). ReduceMotion memakai bentuk 180 ms tanpa overshoot dan isi opacity saja 150 ms.
- Geometri DIP: 200×36 r18 (asal transisi/spike), banner 380×64 r32, expanded 440×184 r30. WorkArea Y+8, tengah monitor Space; bila Space tersembunyi/minimized memakai monitor kursor. Koordinat fisik Win32 dipakai pada batas native untuk DPI campuran. Perubahan posisi Space/display mereposisi island.
- Window dibuat malas, dibuang setelah 10 menit idle, memakai Panel, ShowInactive, WS_EX_NOACTIVATE. Region rounded Windows mengeluarkan sudut transparan dari hit testing. Tidak memasang hook keyboard global. Konten akun dibersihkan saat sesi berubah; fallback memeriksa akun lagi sebelum mengirim toast.

**Fase 2 belum dibangun**: Mini live timer rapat 0.4.3, status sinkron/unggah dan tenggat dekat. Ukuran pil sudah menjadi dasar transisi, tetapi belum ada pil live yang menetap. Aksi lanjutan, pengelompokan burst, pilihan monitor, shortcut global, dan aksesibilitas penuh tetap Fase 3–4. Esc diproses jika jendela mendapat event keyboard; tidak menangkap keyboard aplikasi lain.

## Pengukuran Fase 0

Windows 11 build 22631, host Tiyo, 16 logical processors. Tiap proses dipanaskan 10 detik, disampel 20 kali dengan interval 1 detik, lalu berhenti sendiri pada 45 detik. Diukur Process.WorkingSet64, PrivateMemorySize64, dan delta TotalProcessorTime. Tidak menjalankan browser atau otomasi UI.

Baseline yang relevan memakai **jendela native biasa 640×480 yang sudah tergambar**, sama pada semua mode. Proses spike tidak masuk akun, mengakses API, menulis preferensi, mendaftarkan protokol/tray/shortcut, atau mengirim notifikasi.

| Mode | Working set rata-rata (MiB) | Private bytes rata-rata (MiB) | CPU / interval | % satu core | % mesin (16 LP) |
|---|---:|---:|---:|---:|---:|
| Baseline renderer aktif | 73,446 | 107,062 | 0,015625 s / 20,377 s | 0,0767 | 0,0048 |
| Tambah island tersembunyi | 61,781 | 104,582 | 0 s / 20,328 s | 0 | 0 |
| Pil kosong C, 200×36, terlihat | 87,221 | 140,228 | 0 s / 20,407 s | 0 | 0 |
| Presenter banner nyata C, stabil | 90,374 | 142,039 | 0 s / 20,479 s | 0 | 0 |

Pil: **+13,775 MiB working set / +33,166 MiB private bytes** dibanding baseline. Banner: **+16,928 / +34,977 MiB**. Angka private bytes lebih besar dari perkiraan awal catatan (5–15 MB); jangan menganggap perkiraan itu hasil ukur. Working set dapat dipangkas Windows dan host sedang tertekan memorinya, sehingga angka tersembunyi yang lebih kecil bukan bukti penghematan. CPU 0 berarti tidak ada tick CPU yang terukur selama sampel, bukan jaminan selalu nol. Data satu host/sampel singkat, bukan budget lintas mesin.

Pengukuran banner memaksa izin lingkungan **hanya pada objek spike** supaya biaya presenter benar-benar diukur walau Focus Assist/fullscreen sesi pengukuran sedang aktif. Presenter produksi tetap mengikuti QUNS. Pada spike ditemukan jendela Hidden belum menggambar sebelum ShowInactive; penerimaan notice sekarang menampilkan jendela secara inactive sebelum meminta frame.

Data mentah lokal: `build/island-spike-warm-results.json` dan `build/island-presenter-visible-results.json` (tidak di-commit sebagai artefak build). Pengukuran awal dengan baseline seluruhnya tersembunyi tidak dipakai untuk delta karena turut menghitung cold start renderer.

Ulangi dengan:

```powershell
$env:GOMAXPROCS='2'
$env:GOMEMLIMIT='768MiB'
go build -p 1 -ldflags '-H=windowsgui' -o build/space-island-spike.exe ./app
./scripts/measure-island.ps1 -Executable build/space-island-spike.exe
./scripts/measure-island.ps1 -Executable build/space-island-spike.exe -Modes banner -Out build/island-presenter-results.json
```

## Verifikasi

- `go test -p 1 ./internal/island ./app ./internal/appui`: lulus. Meliputi antrean, tag replacement saat ini/menunggu, FIFO/prioritas, overflow, durasi/dwell/grace/expiry/click/Esc, pause hover, spring continuity/frame independence, dan routing suppress/fallback/accompany serta propagasi error OS.
- `go vet -p 1 ./app ./internal/island ./internal/appui`: lulus.
- `go tool mygo build -platform windows/amd64 ./app`: exe dan installer 0.4.4 berhasil; tanpa upload.
- `go test -p 1 ./...`: paket desktop dan UI lulus; suite tidak hijau karena `internal/filestore/TestImmutableBlobAndLimitCleanup` menolak izin Windows yang tidak merepresentasikan Unix `0700`. `internal/filestore` tidak berubah. Test integrasi server yang membutuhkan konfigurasi database mengikuti skip bawaan repo.
- Kompilasi pertama dengan paralelisme default gagal VirtualAlloc errno 1455. Mengurangi GOMAXPROCS/paralelisme mengatasi kegagalan build/test terkait fitur ini; tidak menghentikan proses lain.
- Tidak menjalankan browser/otomasi UI. Appearance, input, fokus, DPI campuran dan fullscreen/game sungguhan menunggu owner.

## Checklist owner Windows

- [ ] Mulai `Space.exe` 0.4.4; Pengaturan › Perangkat memuat Dynamic Island aktif, gaya C dan ganti toast aktif. Ubah A/B/C, tutup/buka aplikasi, pastikan pilihan menetap dan tema terang/gelap mengikuti Space.
- [ ] Gunakan “Coba Dynamic Island”; ketik di aplikasi lain saat notice datang: fokus/ketikan tetap di sana. Klik sudut transparan mencapai aplikasi di bawahnya. Periksa gaya kaca dengan transparency Windows aktif/mati.
- [ ] Hover 300 ms/klik: banner menjadi detail; keluar 1,5 detik menutup; Buka menuju tugas/ping/rapat yang benar; Tunda 15 menit tidak memunculkan jendela utama dan pengingat datang ulang sesuai server.
- [ ] Kirim beberapa ping/tugas: satu banner sekaligus, prioritas/FIFO, jumlah antrean, tag update, pergantian akun/logout tidak membocorkan isi lama. Cek island mati memakai OS dan opsi ganti toast mati menampilkan keduanya.
- [ ] Uji Focus Assist, presentasi, video borderless fullscreen, game exclusive fullscreen: island sembunyi maksimal 2 detik, Windows menerima fallback dan menentukan DND, tidak ada replay ganda.
- [ ] DPI 100/150/200%, dua monitor dengan DPI berbeda/koordinat negatif, Space dipindah monitor, minimize/tray, cabut/pasang layar: pusat WorkArea, Y+8 DIP, bentuk dan click-through tetap benar.
- [ ] ReduceMotion Windows dan toggle perangkat: tanpa spring overshoot/scale/blur. Uji native Panel di virtual desktop dan bersama aplikasi topmost lain.
- [ ] Biarkan island kosong 10 menit: window dilepas; notifikasi berikutnya membuatnya kembali. Ukur ulang RAM/CPU di PC owner yang idle; periksa tidak ada putaran repaint saat tersembunyi.

Artefak lokal: `app/build/windows-amd64/Space.exe`, `app/build/windows-amd64/space-setup-0.4.4-amd64.exe`. Ini kandidat uji; GOA dan owner memutuskan rilis. Tidak deploy ke tc-vps.

## Tinjauan kedua (Claude, 10 Nov 2026)

Kode c44a80e dibaca ulang dan dicocokkan dengan source MyGo v0.7.1 di module cache, canvas `dceda2c54836`, dan rencana `cfd2fb666954`. Perbaikan ada di commit sesudah c44a80e.

**Diperbaiki**
- **Spring melompat (berat).** `p.last` tidak direset saat island sembunyi. Akibatnya notifikasi kedua dan seterusnya langsung tampil dalam ukuran banner tanpa gerak, karena dt-nya puluhan detik sampai menit. Hal yang sama terjadi pada klik untuk membuka detail setelah banner diam 2 detik. Sekarang pegas mulai dari dt 0 pada saat state berubah, dt per frame dibatasi 50 ms, dan `hideEmpty` mereset `last`. Ditambah test `TestIslandFrameStepKeepsTheMotion`.
- **NotifyBatch (berat).**
  - Setiap item membunyikan bel, jadi 10 pengingat menghasilkan 10 bel.
  - Saat "Gantikan toast Windows" mati, setiap item juga memunculkan toast Windows sendiri-sendiri, padahal batch sengaja diringkas menjadi satu toast.
  - Sekarang bel berbunyi sekali, dan jalur per-item hanya dipakai bila island menggantikan toast dan muat di antrean. Selain itu, ringkasan satu toast yang lama tetap dipakai.
- **Teks tombol Buka di gaya C (ringan).** Warnanya diambil dari permukaan yang tembus pandang (alfa 0,42/0,55), jadi labelnya pudar. Sekarang memakai warna opak.
- **Warna glyph per jenis tidak sesuai canvas (ringan).**
  - B memakai satu aksen untuk semua jenis, padahal canvas memberi warna sendiri untuk ping/tugas, tenggat dan rapat.
  - Tenggat di A seharusnya merah.
  - Rapat di C seharusnya `#34c759`.
  - Diperbaiki lewat `islandGlyphColor`, plus test.
- **Jarak dari atas (ringan).** Kode memakai 8 DIP, sedangkan canvas yang disetujui menulis 6 px. Sekarang `islandTopGap = 6`.
- **DPI campuran.** Saat island dipindah ke monitor dengan DPI lain, Windows mengirim WM_DPICHANGED, dan MyGo (`internal/windows/window.go`, case wmDpiChanged) menerapkan rect saran. Hasilnya ukuran ikut terskala sekali lagi. `islandPosition` sekarang memeriksa rect hasil, dan memindahkan sekali lagi bila belum sesuai. {{Unverified}}: belum diuji di dua monitor.
- **Deteksi fullscreen.** Jendela yang punya title bar (WS_CAPTION) tidak lagi dianggap fullscreen. Sebelumnya, pengguna taskbar auto-hide yang membuka jendela maksimal akan selalu kehilangan island.
- **Prosedur Win32 dibuat sekali di level paket.** Sebelumnya `NewProc` dibuat ulang di setiap panggilan, termasuk pada setiap frame animasi.
- **Penghancuran saat idle.**
  - Setelah `Destroy` kini memanggil `debug.FreeOSMemory()`.
  - `idleSince` kini selalu direset saat timer berbunyi, termasuk saat jendela sudah nil.
  - Spike mendapat mode `idle`: banner menutup sendiri setelah 2 detik, jendela dilepas 3 detik kemudian, lalu spike menulis `island-idle-state.txt`.

**RAM (pertanyaan 90/142 MiB).** Angka itu adalah total proses yang sedang menampilkan banner, bukan biaya island saja.

| Mode (PC Tiyo, 10 s warmup + 20 sampel) | Working set (MiB) | Private (MiB) |
|---|---:|---:|
| baseline (satu jendela native) | 114,4 | 107,3 |
| banner C tampil | 90,6 | 143,7 |
| idle, setelah jendela island dilepas (`destroyed=true`) | 65,0 | 107,2 |

- Biaya island adalah sekitar **+36 MiB private bytes selama jendelanya ada**, lalu **kembali ke baseline** setelah jendela dilepas.
- Working set naik-turun karena Windows memangkasnya, jadi angka itu bukan ukuran yang bisa diandalkan.
- Dengan aturan 10 menit, biaya itu tertahan sampai 10 menit setelah notifikasi terakhir.
- Saran untuk owner: turunkan ke 1–2 menit. Membuat jendela lagi itu murah. Ini belum diubah karena 10 menit adalah keputusan rencana.

**Dicatat, belum diubah**
- Tata letak tidak persis sama dengan canvas. Lingkaran ikon 34 px, padahal canvas memakai 38 px untuk banner dan 42 px untuk expanded. Padding banner 14 px di semua sisi, padahal canvas memakai 0/18/0/14. Ukuran judul 14/13 px, padahal canvas memakai 13,5/12,5 dan 15 px di expanded. Tombol memakai bentuk bawaan Space, tidak berbentuk pil. Gaya A diberi border putih, padahal canvas tidak memakainya. Di pengaturan, tombol teks dipakai, padahal canvas memakai switch. Semua ini perlu dicek di layar dulu sebelum diubah.
- Esc tidak akan pernah sampai ke island, karena WS_EX_NOACTIVATE membuat jendela itu tidak pernah memegang keyboard. Ini sudah tercatat sebagai keterbatasan Fase 4.
- Pemeriksaan fullscreen melihat monitor jendela depan, bukan monitor island. Akibatnya, game fullscreen di monitor 2 juga menyembunyikan island di monitor 1. Perilaku ini konservatif.
- QUNS diperiksa di setiap frame selama animasi. Komentar yang bilang "maksimal setiap 2 detik" hanya benar saat island diam. Biayanya kecil sekarang, karena prosedurnya sudah di-cache.
- `go test -race` (golang:1.27.3 di homeserver): lihat laporan GOA. Race di `internal/appui` `TestReviewRowRejectNeedsReason` (canvas tahap 5) sudah ada sebelum island dan bukan bagian dari island.
- Belum ada test untuk `islandPresenter.offer`, `reset` dan fallback. Test perutean yang ada hanya menguji `MultiNotifier`.

Ulangi pengukuran idle:

```powershell
./scripts/measure-island.ps1 -Executable build/space-island-spike.exe -Modes baseline,banner,idle -Out build/island-review-results.json
Get-Content build/island-idle-state.txt   # destroyed=true
```
