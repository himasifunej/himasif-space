# Deploy pilot Space ke tc-vps (sejak 9 November 2026)

Pilot pindah dari aws-vps ke **tc-vps** pada 9 Nov 2026 pukul 14:04 UTC, dengan 0.3.6 live. aws-vps sudah pensiun: unit-unitnya dihentikan dan dinonaktifkan, data dibiarkan untuk rollback. **Jangan deploy ke aws-vps lagi.** Rincian pindahan ada di `tc-vps:/root/space-migration/CUTOVER.md`.

Setiap deploy tetap butuh persetujuan owner lewat GOA.

## Host
- **Akses:** SSH lewat Tailscale dengan alias `tc-vps` (user `root`, lihat `~/.ssh/config`). Space berjalan sebagai user `enowx`; perintah dijalankan dengan `sudo -iu enowx`.
- **Layanan:** unit systemd `--user` `himasif-space-pilot-{db,api,downloads,bridge,tunnel}` dan `himasif-space-pilot-backup.timer`. Unit api punya `MemoryMax=256M`, dan nilai ini tidak diubah.
- **Path:** sama dengan di aws-vps, berupa bind mount dari disk `vdb`:
  - rilis: `~/.local/opt/himasif-space-pilot/releases/0.3.0-pilot.13-212f6eaa1793/{space-server,web}`;
  - env: `~/.config/himasif-space-pilot/server.env`;
  - backup: `~/.local/share/himasif-space-pilot/manual-backups/`.
- **Tidak ada di tc-vps:** repo, Go dan alat Playwright (`/home/enowx/HIMASIF` tidak ada).
- **RAM 1,9 GB dipakai bersama aplikasi lain. Jangan build atau `go test` di server.** Build dikerjakan di PC, dan yang diunggah hanya artefak jadi.

## Alur (skrip di PC: `E:\himasif-cache\deploy`)
1. Merge ke main, jalankan gofmt/vet/test, push, dan pastikan CI hijau.
2. `build-local.sh VER SHA`: mengambil commit dari `origin/main` dan memeriksa bahwa `app/mygo.json` = VER. Hasilnya, di `E:\himasif-cache\build\VER`:
   - `space-server`, binary statis linux/amd64 dengan ldflags yang sama dengan `make server`;
   - `notecheck`, pemeriksa catatan yang hanya membaca;
   - `web/`, dibangun dengan langkah yang sama dengan `make web`;
   - `SHA256SUMS`.
3. `deploy.sh VER --dry-run` **hanya membaca**:
   - checksum build lokal;
   - di server: status layanan dan timer, MemoryMax, path unit api dan `SPACE_WEB_DIR`, versi yang sedang berjalan, izin tulis direktori rilis, `pg_dump`, RAM dan disk, serta health.
4. `deploy.sh VER --stage` mengunggah build ke `~/.local/opt/himasif-space-pilot/staging/VER`, memeriksa checksum, lalu menjalankan `notecheck` (hanya membaca) terhadap DB pilot. Rilis yang sedang live tidak disentuh.
5. `TOKFILE=… deploy.sh VER --go` menjalankan urutan berikut:
   - snapshot JSON semua catatan;
   - backup `before-VER-<ts>.dump` dan lampiran;
   - API, dengan rollback otomatis bila health gagal;
   - web, dengan versi lama disimpan sebagai `web.before-VER`;
   - bridge WhatsApp (`r-bridge.sh`), hanya bila sumbernya berubah dan `package.json` sama. Cadangannya `src.before-VER`, dengan rollback otomatis bila `/health` gagal;
   - smoke check: layanan, error/panic di log sejak deploy, health;
   - snapshot catatan kedua, lalu dibandingkan dengan yang pertama.

   Login API cukup sekali (`pw/tok.js`); batasnya 10 kali per 15 menit.
6. Tidak ada uji UI lewat browser. Owner menguji sendiri di pilot.

Pelajaran dari deploy 0.3.7:
- Unggahan staging lewat `/tmp` lalu `r-unpack.sh`, karena quoting lewat `sudo -i` menghapus variabel.
- Folder `manual-backups` dibuat otomatis; folder ini tidak ikut pindah dari aws-vps.
- `/health` bridge butuh token.
- Zona waktu tc-vps UTC+8, jadi `--since` smoke check memakai `UTC`.

Skrip lama untuk aws-vps (yang build di server) diarsipkan di `E:\himasif-cache\deploy\aws-old\` dan tidak dipakai lagi.

## Catatan
- Build lokal menghasilkan nama wasm yang berbeda dari build di server untuk commit yang sama, karena versi Go dan path build berbeda. Untuk 0.3.6, `index.html` lokal sama persis dengan yang live kecuali nama wasm.
- **Rollback:** salin balik `space-server.before-VER` lalu restart api; tukar `web` dengan `web.before-VER`.
- Backup harian (timer) menghentikan api dan bridge beberapa detik, dan homeserver menariknya dengan `SPACE_HOST=tc-vps`.
