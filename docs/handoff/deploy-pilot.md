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

## Siap migrasi (tc-vps → host berikutnya)
tc-vps direncanakan dipakai sekitar 4–5 bulan sejak Nov 2026, sampai langganan Tencent habis. Pindahan sebelumnya (aws-vps → tc-vps, 9 Nov 2026) hanya **down sekitar 25 detik** (14:03:45–14:04:10 UTC), karena semuanya disalin dan dicoba dulu di host baru. Pola dan perintahnya ada di `tc-vps:/root/space-migration/CUTOVER.md`.

**Yang harus dipindah.** Semuanya milik user `enowx`; ukurannya per 11 Nov 2026.
- **DB:** Postgres 16 dari `postgres-runtime`. Datanya di `~/.local/share/himasif-space-pilot/postgres`, sekitar 70 MB di disk; dump `pg_dump -Fc` di bawah 1 MB.
- **Lampiran:** `share/attachments`, plus unduhan `share/public-downloads` (sekitar 580 MB, boleh disalin lebih dulu).
- **Env dan rahasia:** isi `~/.config/himasif-space-pilot/`, yaitu `server.env`, `bridge.env`, `backup-policy.json`, `backup-recipients.txt` dan kredensial tunnel. Salin apa adanya; jangan dicetak.
- **Bridge WA:** sesi di `share/whatsapp`, terenkripsi dengan `SPACE_WA_STATE_KEY` dari `bridge.env`. Dengan sesi ini bridge tersambung tanpa scan QR. **Bridge tidak boleh jalan di dua host sekaligus.**
- **Rilis:** direktori `~/.local/opt/himasif-space-pilot/` (releases, `postgres-runtime`, `node-runtime`). Pastikan `node` berupa binary sungguhan, bukan symlink.
- **Unit systemd `--user`:** `himasif-space-pilot-{db,api,downloads,bridge,tunnel}` dan `backup.{service,timer}`. Aktifkan `loginctl enable-linger enowx`.
- **Tunnel dan DNS:** tunnel Cloudflare `56423224-…`, kredensial yang sama. CNAME `space.`/`db.tiyoouw.app` sudah menunjuk ke id tunnel, jadi **DNS tidak perlu diubah**. Tunnel tidak boleh jalan di dua host sekaligus.
- **Backup:** timer harian di host. Homeserver menarik backup dengan `SPACE_HOST=…`; nilai ini diganti ke host baru.

**Urutan:**
1. **Persiapan, tanpa downtime:**
   - Host baru: buat user `enowx`, salin rilis, runtime, env dan `public-downloads`.
   - Jalankan db dan api dengan data salinan lewat tunnel uji, lalu cek health.
   - Hapus lagi semua sisa uji: tunnel, DNS dan env uji.
2. **Jendela pindah:** di host lama, hentikan bridge dan api, lalu jeda `backup.timer`.
3. Di host lama: `pg_dump`. Di host baru: `pg_restore`, lalu bandingkan jumlah baris per tabel.
4. rsync `attachments` dan `whatsapp` ke host baru.
5. Di host baru, jalankan db, api, downloads dan bridge. Cek `/api/health` di 127.0.0.1 dan pastikan bridge tersambung.
6. Pindahkan tunnel: **stop dan disable dulu di host lama**, baru `enable --now` di host baru. Dari sini downtime berakhir.
7. Verifikasi: health di kedua domain, login API sekali, buka `/downloads/`, lalu owner mencoba WA.
8. Aktifkan backup timer di host baru, ganti `SPACE_HOST` di homeserver, lalu perbarui alias host di `E:\himasif-cache\deploy\config.sh` dan dokumen ini.
9. Host lama tetap dibiarkan, dalam keadaan unit berhenti dan disable, sekitar 2 minggu sebagai jalan rollback. Setelah itu baru dihapus.

**Rollback:** di host baru, stop tunnel, bridge dan api. Di host lama, `enable --now` tunnel, api, bridge dan timer. Data yang masuk sesudah pindah ikut di-`pg_dump` lebih dulu.

## Catatan
- Build lokal menghasilkan nama wasm yang berbeda dari build di server untuk commit yang sama, karena versi Go dan path build berbeda. Untuk 0.3.6, `index.html` lokal sama persis dengan yang live kecuali nama wasm.
- **Build saat memori PC mepet:** sejak 0.4.5-rc2 (11 Nov 2026), artefak dibangun di homeserver-girli, di dalam Docker `golang:1.27.3` dengan `-p=1`, `-m 3g` dan `--cpus 2`. Langkahnya sama dengan `build-local.sh`, termasuk digest sumber, `patch_web_index.py`, gzip dan `SHA256SUMS`. Hasilnya disalin ke `E:\himasif-cache\build\VER`, lalu `deploy.sh` dijalankan seperti biasa. Alasannya: build di PC pernah menghabiskan pagefile sampai laptop bluescreen.
- **Rollback:** salin balik `space-server.before-VER` lalu restart api; tukar `web` dengan `web.before-VER`.
- Backup harian (timer) menghentikan api dan bridge beberapa detik, dan homeserver menariknya dengan `SPACE_HOST=tc-vps`.
