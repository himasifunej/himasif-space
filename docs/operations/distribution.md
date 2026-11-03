# Distribusi dan pemulihan versi Space

Candidate `0.3.0-pilot.10`, tanpa mobile. Pengguna meminta tidak menjalankan tes Go pada tahap ini; jangan menyatakan perubahan sesudah pilot.9 sudah lulus tes. Hasil build/test aktual untuk setiap paket dicatat dalam `verification.json` dan log pendamping. Keberadaan panduan ini sendiri bukan bukti tes lulus. Builder tidak mengaktifkan DNS atau auto-update. Deployment backend AWS yang diminta pengguna dicatat terpisah dalam [backend AWS](aws-pilot.md).

## Isi paket akhir

Builder `scripts/release/build_release.py` menyiapkan:

- Windows amd64: aplikasi, resource dan installer per pengguna dalam ZIP versi tetap.
- Web: static MyGo/WebAssembly beserta gzip, memakai origin backend yang sama.
- Server Linux amd64/arm64: binary API, bridge source + lockfile, `.env.example`, tooling dan panduan operasional. PostgreSQL, Node.js yang sesuai `bridge/package.json`, npm, ffmpeg, age dan alat PostgreSQL merupakan prasyarat host; paket tidak mengklaim membawa runtime/dependensi OS tersebut.
- Source ZIP dari snapshot worktree aktual, manifest SHA-256 per file, laporan pemeriksaan dan checksum artefak.
- ZIP gabungan, checksum ZIP gabungan terpisah dan panduan distribusi/cutover.

Builder tidak memakai commit hash sebagai satu-satunya identitas ketika worktree belum di-commit: `sourceDigest` mengidentifikasi isi snapshot aktual. `space-server version`, health dan Status layanan menyajikan versi/revisi/digest/waktu build. ZIP source tidak membawa `.git`, konfigurasi real, private key, state WA, blob/lampiran privat, dump atau cache/node_modules. Untuk mengembangkan kembali source ZIP, inisialisasi repo Git lokal bila ingin memakai builder yang berbasis inventaris Git; build Go biasa tetap memakai source tersebut.

## Membangun pada fase akhir

Prasyarat builder: Go sesuai `go.mod`, git, Node/npm, Python 3 dan NSIS/makensis untuk installer Windows. Gunakan `SPACE_TEST_DATABASE_URL` dari environment terlindungi dengan nama DB **baru/terisolasi** `space_test_...`; suite database tidak boleh diarahkan pada DB produksi. Builder menolak tidak adanya target uji yang eksplisit. Jangan menaruh URI/kredensial pada command history.

```bash
python3 scripts/release/build_release.py --api-url https://API_PILOT_YANG_DISETUJUI --go /path/ke/go
```

Alamat API desktop harus ditetapkan nyata sebelum build final; HTTP hanya diterima untuk loopback. Web memakai origin tempat ia disajikan. Untuk menguji desktop ke API lain, launcher/environment `SPACE_API_URL` dapat mengarahkan ke backend yang dimaksud tanpa membuat database tugas kedua. Tidak ada bukti domain pilot sudah diterapkan hanya karena alamat disematkan pada binary.

Builder menyalin source ke direktori kerja terpisah, mengunci/memasang dependensi bridge pada snapshot, lalu menjalankan vet/unit/integrasi Go, vet target Windows/web, tes bridge dan tes tooling operasional. Suite bridge/ops nol tes ditolak; server test yang di-skip ditolak. Pemeriksaan ini masih harus dijalankan dan diperluas sesuai hasil audit akhir. Log disimpan terpisah dan disanitasi. Pekerjaan dibuat serial dengan pembatasan resource untuk host kecil.

Sesudah pemeriksaan lulus, builder menyusun binary server untuk kedua arsitektur dan installer/static web. Ini membuktikan hasil **build** dan pemeriksaan yang tercatat, bukan perilaku Windows atau integrasi produksi yang belum dicoba. Laporan awal tetap `productionReady=false` dan mencatat acceptance eksternal yang perlu dibuktikan. Jangan menghapus daftar pending untuk membuat paket terlihat selesai; lengkapi dengan bukti runtime/artifact yang sesuai.

## Memasang dan memperbarui desktop

Pilot memakai **update manual melalui installer/paket versi tetap**, bukan updater otomatis tanpa kunci dan kanal rilis yang disiapkan. Simpan paket versi sebelumnya dan checksum yang dipercaya secara terpisah. Signing Authenticode/sertifikat organisasi belum dikonfigurasi pada checkpoint ini; jangan menyebut kandidat sebagai binary bertanda tangan.

1. Periksa versi/arsitektur serta checksum paket terhadap catatan rilis tepercaya. Simpan paket sebelumnya dan draf/pending perangkat.
2. Keluar dari Space melalui tray, sehingga draf dan jurnal perangkat selesai disimpan.
3. Pasang installer Windows per pengguna, atau ekstrak paket portable pada direktori **versi baru**. Jangan menjalankan dari ZIP atau menimpa executable yang masih berjalan.
4. Jalankan, cek versi/target server, login dan periksa tugas/Notes serta status sinkronisasi. Autostart harus menunjuk executable terpasang yang tetap, bukan folder unduhan temporer. Verifikasi tray, toast, tautan/aksi, Ace dan restart pada perangkat sasaran.
5. Jika gagal, keluar dan jalankan/pasang kembali paket sebelumnya yang telah diverifikasi. Pertahankan penyimpanan akun/draf/pending; jangan menghapusnya untuk memaksa pembaruan terlihat berhasil. Pemulihan client tidak otomatis menurunkan schema server.

Bila gagal memperbarui atau desktop rusak, web pada backend yang sama menjadi jalur kerja sementara. Paket web juga memiliki versi; deploy sebagai direktori immutable dan alihkan pointer/reverse proxy secara terkontrol, bukan menyalin separuh file WASM/JS ke direktori live.

## Memasang dan memperbarui server/web

Ekstrak server yang sesuai arsitektur ke `/opt/space/releases/VERSI`; simpan konfigurasi 0600 dan state di luar direktori rilis. Direktori lampiran terpisah mode 0700; bridge memiliki state/key sendiri. Pasang dependensi dari lockfile bridge pada arsitektur host (`npm ci --omit=dev`), siapkan PostgreSQL dan runtime/ffmpeg dari sumber operator yang disetujui. Jangan menyalin `node_modules` dari arsitektur lain.

Gunakan symlink `current`/pointer service untuk memilih satu versi. Pasang web ZIP ke direktori rilis web, set `SPACE_WEB_DIR` pada konfigurasi server dan cek API/public URL/reverse proxy/TLS untuk domain yang telah disetujui. Contoh path service backup mengasumsikan `current/scripts/ops/space_ops.py`; isi paket server mempertahankan struktur tersebut.

Sebelum upgrade: inventaris versi/migrasi, buat backup lengkap dan lakukan restore drill terisolasi. Uji binary baru terhadap salinan DB yang akan dimigrasikan. Tidak ada perintah undo schema atau `reset --hard` sebagai rollback data. Jika server lama kompatibel dengan schema baru yang telah dibuktikan, rollback pointer binary; jika tidak, gunakan rencana pemulihan backup + rekonsiliasi perubahan setelah cutoff, dengan persetujuan konkret. Jangan memulihkan DB secara buta di atas DB live.

Promosi restore ke produksi dan pengaktifan pengirim mengikuti [cutover pilot](pilot-cutover.md). Marker karantina tidak dilepas oleh pergantian environment biasa. Mengaktifkan ulang semua antrean historis dilarang sebagai efek samping upgrade.

## Bukti yang harus menyertai unduhan

Catat versi, sourceDigest, checksum tiap artefak, platform, hasil tes aktual, skenario yang di-skip/pending, API target, status signing, kompatibilitas schema/client serta langkah rollback yang telah dicoba. Bedakan compiled, tested, installed dan production-approved. Paket candidate boleh disiapkan untuk ditinjau; klaim seluruh scope selesai tetap memerlukan audit kriteria penerimaan dan bukti yang sesuai. Belum ada klaim tersebut pada checkpoint ini.

## Alamat server pada Windows

Alamat bawaan desktop pilot.4 dan seterusnya adalah `https://db.tiyoouw.app`. Pengguna tidak perlu menjalankan database/server di komputer Windows. Alamat setiap build juga dicatat di `verification.json` (`desktopAPIURL`). Paket lama pilot.3 membawa loopback dan perlu diganti. Bila pernah menetapkan environment `SPACE_API_URL`, hapus override lama agar alamat bawaan baru digunakan. Login dan data berasal dari backend AWS, tidak disertakan dalam installer. Web memakai origin tempat server menyajikannya. Jangan menempelkan token/password pada URL. Status HTTPS aktual dicatat dalam laporan deployment terpisah; alamat dalam binary saja bukan bukti deployment selesai.

Pilot.5 menjaga pintasan semua divisi/ruang yang dapat diakses dalam area sidebar yang bisa digulir. Daftar tidak lagi memotong ruang sesudah batas lima item atau tergantung tinggi jendela.

Pilot.5 juga menyematkan emblem divisi resmi dari profil himasif.id (PSDM, Kaderisasi, Litbang, Humas, Mediatek) untuk sidebar dan header Space, memakai slug seed `kader` dengan alias `kaderisasi`. BPH belum memiliki emblem resmi sehingga tetap memakai mark netral; ruang bersama seperti Medhum tidak mewarisi identitas divisi mana pun. Jarak antara merek Space dan pemilih periode kini 8 px (6 px pada layout compact).

## Perbaikan lifecycle Windows pada pilot.3

Tag toast memakai hash seluruh UUID agar notifikasi yang dibuat bersamaan tidak saling menggantikan. Tautan aksi hanya membawa identitas navigasi, bukan isi pesan. Shortcut notifikasi memakai `Space.lnk` yang sama dengan installer; skema `himasif-space` didaftarkan oleh installer. Autostart yang sudah dipilih diperbarui ke executable yang sedang berjalan.

Builder memakai `scripts/release/windows_installer.py` untuk menambahkan pembersihan registrasi perangkat pada template NSIS MyGo tanpa mengubah SDK. Adapter menolak template yang tidak dikenali dan menghasilkan installer native amd64 sesuai aplikasi Windows amd64. Saat uninstall, Run/StartupApproved dan registrasi notifier hanya dihapus bila masih menunjuk instalasi tersebut; preferensi, akun, draf, dan berkas pending tetap disimpan. Build installer langsung lewat SDK tanpa builder tidak menyertakan adapter ini.

Tes registry Windows memakai key privat khusus tes. Bukti yang dijalankan lewat Wine di AWS adalah bukti emulasi API, bukan pengganti pengujian Explorer, toast WinRT, tray, autostart/login dan instalasi di Windows asli.

Pemeriksaan lanjutan juga membedakan path induk yang berupa berkas dari direktori yang benar-benar belum dibuat. Preferensi/draf tidak diam-diam dianggap kosong pada kegagalan penyimpanan Windows tersebut.
