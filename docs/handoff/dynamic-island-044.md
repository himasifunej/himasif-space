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
