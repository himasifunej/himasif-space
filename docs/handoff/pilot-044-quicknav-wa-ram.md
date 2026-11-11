# Pilot 0.4.4 — navigasi cepat, template WA, RAM

Branch `feat/quicknav-wa-template-b`, dari catatan uji owner "TEST PILOT 0.4.4" (gambar navigasi cepat, perintah grup WA, RAM). Belum di-merge, belum dirilis atau di-deploy.

## 1. Navigasi cepat (Ctrl K)

- Palet mencari **halaman** (`appPages`), **Space dan tab-nya** (`spaceTabsAll`), **tab Organisasi** (admin), **aksi** (buat tugas, kirim ping, tema gelap/terang/sistem, panel tampilan, sidebar, tur, muat ulang, keluar), **catatan** yang sudah dimuat, **tugas** yang sudah dimuat, dan **setiap seksi, field, judul dan tombol pengaturan**.
- Isi pengaturan tidak ditulis tangan. `tools/quicknavgen` membaca source tab pengaturan, termasuk Perangkat desktop dan web di `app/`, lalu menulis `internal/appui/quicknav_settings_gen.go`. Bila halaman pengaturan diubah tanpa `go generate ./internal/appui`, `TestQuicknavSettingsIndexUpToDate` gagal. `TestQuicknavPagesCoverRouter` memeriksa bahwa setiap `r.Match(Path…)` di router ada di `appPages`.
- Pencarian fuzzy: cocok utuh, awal kata, bagian kata, salah ketik satu huruf, dan huruf berurutan. Ada sinonim Indonesia/Inggris, misalnya maskot↔Ace, setting↔pengaturan, tema↔tampilan, logout↔keluar, wa↔whatsapp. Judul berbobot dua kali jejak.
- Setiap hasil punya subjudul jejak, misalnya "Ace dan suara · Pengaturan › Perangkat" atau "Nama maskot · Pengaturan › Perangkat › Ace dan suara".
- Membuka hasil pengaturan akan menggulir ke seksinya dan memberinya garis aksen selama 2 detik (`RevealSection`). Untuk "Pengaturan lanjutan" WhatsApp, panelnya dibuka dulu.
- **Ctrl K membuka palet, dan menutupnya bila sedang terbuka.** ↑/↓ memilih hasil, Enter membukanya.
- Dicek pada aplikasi desktop Linux asli (Xvfb, API palsu lokal): Ctrl K, ketik "Ace", lalu Enter membuka Perangkat dengan "Ace dan suara" tersorot.

## 2. Perintah dan template WhatsApp

- Lima template, nama bot, sapaan, dan balasan perintah khusus kini memakai satu editor. Baris **"Sisipkan:"** berisi chip berlabel (Isi pesan, Nama bot, Sapaan, Judul; untuk perintah khusus: Nama pengirim, Tanggal, Jam, Nama grup, Teks setelah perintah, Nama bot). Chip menaruh `[Label]` **di posisi kursor**, dan kata itu berwarna aksen di dalam field.
- **Pratinjau** muncul sendiri saat field difokus atau diketik. Pratinjau tetap terbuka setelah fokus lepas, berpindah saat field lain difokus, dan tertutup lewat "Tutup pratinjau". Hanya satu pratinjau aktif pada satu waktu.
- Template yang tidak memuat [Isi pesan] mendapat peringatan di field itu dan tombol "Tambahkan [Isi pesan]". Simpan ditolak dengan nama field-nya.
- Penyebab galat di gambar owner: pratinjau satu field mengirim *semua* template, sehingga satu template lain tanpa `{body}` menggagalkan pratinjau. Sekarang hanya template yang lengkap yang dikirim.
- Template lama dulu dimuat mentah dan tampil sebagai `{body}`. Sekarang dimuat sebagai `[Isi pesan]`.
- Kompatibilitas: `{body}` dan placeholder lain yang diketik manual tetap tersimpan apa adanya. `[isi pesan]` tidak peka huruf besar. Tanda kurung lain tidak diubah.
- Server: respons perintah khusus mengisi `{sender}`, `{date}`, `{time}`, `{group}`, `{name}`, dan `{args}` (`renderWACommandText`). Respons tanpa `{` dikirim persis seperti dulu. Tidak ada migrasi.
- Command bot: tombol "Buat perintah baru", field "Nama perintah" (tanda `!` di depan dibuang saat simpan), dan field "Balasan bot".

## 3. RAM

### Dari mana 170 MB itu

Angka Task Manager adalah *private working set*. Hasil ukur yang sudah ada dan yang baru:

| Sumber | Ukuran | Bukti |
|---|---|---|
| Proses MyGo dengan satu jendela native yang sudah tergambar (runtime, renderer, DWM/D3D) | ±107 MiB private bytes, 65–114 MiB working set | `dynamic-island-044.md`, PC Tiyo |
| Jendela Dynamic Island, bertahan **10 menit** setelah notifikasi terakhir | +36 MiB private | sama |
| Jendela utama 1440×900 pada skala 150% (spike memakai 640×480) | beberapa permukaan 2160×1350×4 ≈ 11,7 MB per buffer | perkiraan, {{Unverified}} di Windows |
| Heap Go aplikasi (data, state UI, atlas teks, gambar) | 8–10 MiB setelah membuka 13 halaman | pprof proses desktop Linux |
| Logo divisi (didekode saat start, ukuran penuh) | 1,56 MiB | probe heap |
| Maskot impor (jika ada): setiap frame GIF/PNG ukuran penuh | sampai ±64 MiB (budget 16 juta piksel); GIF 512×512×60 = 45 MiB | probe heap |

Pengukuran Linux: proses desktop asli, software renderer, Xvfb 1920×1080, API palsu dengan 5 Space, 60 tugas, dan 12 anggota per divisi. Dalam PSS ±117 MiB itu: biner dan pustaka ±45 MiB, permukaan jendela X (shm) 24 MiB, Go ±23 MiB tinggal (heap 8–10), heap C GTK/Pango ±20 MiB. Goroutine 16 dan stabil; tidak ada kebocoran yang terlihat dalam 30 detik idle. GOGC=50 tidak mengubah hasil berarti, jadi GC bukan masalahnya.

Kesimpulan: sebagian besar RAM adalah biaya dasar renderer native dan jendela, ditambah island selama 10 menit. Kode aplikasi sendiri kecil. 170 MB wajar untuk aplikasi native dengan jendela besar pada DPI tinggi yang baru saja menampilkan island. Angka itu tidak akan tumbuh terus seiring data: heap Go stabil di 8–10 MiB.

### Yang diubah

- Logo divisi didekode baru saat pertama kali tampil, diperkecil ke 132 px (tanda 44 DIP pada 300%). Kader dan Kaderisasi kini satu bitmap. **1,56 → 0,31 MiB.**
- Maskot impor: setiap frame diperkecil ke 192 px (Ace tampil 88 DIP), dan setiap frame punya buffer sendiri. **GIF 512×512×60: 45,0 → 8,4 MiB.** Dengan budget maksimum, ±64 MiB menjadi ≤17 MiB. Di GPU, tekstur ikut mengecil.
- `app/pprof_desktop.go` (build tag `spacepprof`, tidak masuk build biasa) supaya owner bisa mengukur di Windows:
  `go build -tags spacepprof -ldflags -H=windowsgui -o build/space-pprof.exe ./app`, lalu `go tool pprof -top http://127.0.0.1:6061/debug/pprof/heap`.

### Angka proses penuh (Linux, sebelum → sesudah)

Lihat tabel di bagian akhir. Selisihnya kecil karena maskot bawaan berupa SVG. Penghematan besar hanya terjadi bila maskot impor dipakai.

### Disarankan, tidak diubah

- **Island dilepas setelah 1–2 menit, bukan 10** (`islandIdleDestroy`, `app/island_desktop.go`). Ini menghemat ±36 MiB lebih cepat. File itu dikerjakan sesi `feat/island-v2`, dan 10 menit adalah keputusan rencana, jadi owner yang memutuskan.
- Ukur di Windows dengan build `spacepprof` dan `scripts/measure-island.ps1`: login dan idle, setelah island tampil, dan 10 menit kemudian.
- "Space (2)" di Task Manager: {{Unverified}}. Bisa berarti dua jendela tingkat atas, atau proses `powershell.exe` dari notifikasi perangkat yang sedang berjalan. Klik ▸ di Task Manager untuk melihat isinya.

### Tabel: proses desktop Linux (rata-rata 2 putaran, MiB)

| Titik | RSS sebelum → sesudah | PSS sebelum → sesudah | Heap Go sebelum → sesudah | Go dari OS sebelum → sesudah |
|---|---|---|---|---|
| Layar login | 121,3 → 119,0 | 99,9 → 97,5 | 2,8 → 1,2 | 13,3 → 13,2 |
| Setelah masuk | 127,4 → 125,7 | 105,8 → 104,1 | 4,7 → 3,2 | 17,5 → 17,6 |
| Setelah 13 halaman (Ctrl K) | 140,8 → 137,5 | 119,0 → 115,8 | 9,9 → 8,5 | 34,5 → 30,1 |
| Idle 30 detik | 139,8 → 139,4 | 118,0 → 117,6 | 9,6 → 8,3 | 34,5 → 30,1 |

Ini angka Linux dengan software renderer, bukan Windows/D3D. Selisih RSS di bawah ±3 MiB termasuk noise. Yang konsisten adalah heap Go turun ±1,3–1,6 MiB dan memori Go dari OS turun ±4 MiB. Angka Windows perlu diukur owner (build `spacepprof`).
