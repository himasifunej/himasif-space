# Pembaruan otomatis Space desktop

Space desktop (Windows) memeriksa versi baru sendiri dan memasangnya dengan sekali klik. Web tidak terpengaruh; web selalu memuat versi yang disajikan server.

## Cara kerja

1. **Pemeriksaan.** Berjalan 20 detik setelah Space dibuka, lalu setiap 6 jam, ditambah tombol **Periksa pembaruan**. Permintaannya hanya `GET` ke manifest kanal tanpa cookie, akun, ID perangkat, atau query string. Header `User-Agent: Space-Updater/<versi>` adalah satu-satunya hal yang dikirim.
2. **Kanal.** **Stabil** (bawaan) membaca `stable.json`. **Uji coba (rc)** membaca `stable.json` dan `rc.json` sekaligus, lalu mengambil versi terbaru di antara keduanya. Pengguna rc tetap mendapat rilis stabil yang lebih baru dari rc-nya.
3. **Manifest.** Berisi aplikasi, kanal, versi, waktu rilis, catatan singkat, tautan catatan lengkap, serta URL, SHA-256, dan ukuran installer. Manifest dibungkus amplop:
   ```json
   {"format":1,"keyId":"869a5e520c539842","manifest":"<base64 JSON>","signature":"<base64 ed25519>"}
   ```
   Tanda tangan ed25519 dihitung atas `"space-update-manifest-v1\n" + JSON`. Aplikasi menolak manifest yang:
   - tidak ditandatangani salah satu kunci di `internal/update/feed.go`;
   - berasal dari kanal lain;
   - URL-nya bukan https;
   - hash-nya tidak berupa 64 digit hex;
   - versinya tidak lebih baru dari versi terpasang. Ini sekaligus menolak downgrade dan manifest lama yang diputar ulang.
4. **Pemberitahuan.** "Space X tersedia" muncul di Ace Island, atau sebagai toast Windows bila island mati atau tidak tersedia. Tombolnya: **Perbarui sekarang**, **Lihat perubahan**, dan **Nanti** (menahan pemberitahuan otomatis selama 24 jam). Setiap versi diumumkan sekali. Bagian **Pembaruan aplikasi** ada di Pengaturan › Perangkat dan di Status layanan; Ctrl K juga menemukannya.
5. **Unduh.** Berjalan di latar dengan progres dan bisa dibatalkan. Installer disimpan ke `%APPDATA%\himasif-space\device\updates`. Ukuran dan SHA-256 diperiksa; berkas yang tidak cocok langsung dihapus.
6. **Pasang.** Langkahnya:
   1. SHA-256 diperiksa sekali lagi.
   2. Catatan yang sedang ditulis disimpan.
   3. Space menyalin `Space.exe` menjadi helper di folder updates, menjalankannya, lalu menutup diri.
   4. Helper menunggu Space tertutup, memeriksa SHA-256 lagi, lalu menjalankan `setup.exe /S /D=<folder Space>`. NSIS MyGo menutup Space bila masih jalan, dan memasang per-user tanpa admin.
   5. Helper membuka Space lagi dan menulis `result.json`.
   
   Saat start, Space membaca laporan itu ("Space diperbarui ke X" atau pesan gagal) lalu membersihkan folder updates.
7. **Batasan.** Pemasangan otomatis hanya berlaku untuk Space yang dipasang lewat installer (ada `Uninstall.exe` di sebelah `Space.exe`). Salinan portable dan build pengembangan hanya mendapat pemberitahuan. Tautan `himasif-space://update` dari luar Space diabaikan; aksi pembaruan hanya datang dari island Space sendiri.

## Mengapa GitHub Releases

Manifest ada di aset rilis tetap `update-feed` repo publik `himasifunej/himasif-space`:

- `https://github.com/himasifunej/himasif-space/releases/download/update-feed/stable.json`
- `https://github.com/himasifunej/himasif-space/releases/download/update-feed/rc.json`

Installer ada di rilis `v<versi>`.

Alasannya:
- Tidak perlu mengubah atau men-deploy tc-vps. RAM tc-vps 1,9 GB dan dipakai bersama aplikasi lain.
- Unduhan besar tidak membebani server pilot.
- Pembaruan tetap jalan saat pilot sedang mati.
- `gh` sudah dipakai untuk rilis 0.4.5-rc.

Kepercayaan datang dari tanda tangan, bukan dari host. Bila suatu saat ingin pindah ke `db.tiyoouw.app/downloads`, cukup ganti `FeedBase` dan URL installer; formatnya tetap sama.

## Kunci

- Kunci publik ditanam di `internal/update/feed.go`, id `869a5e520c539842`.
- Kunci privat berupa seed ed25519 base64. Tempatnya: vault enowX **`space-update-signing`** (platform Env file), yang mengisi `SPACE_UPDATE_SIGNING_KEY`. Tersimpan sejak 11 Nov 2026 dan sudah diperiksa dengan dry-run publish. `tools/updatesign` dan `publish_update.py` membaca kunci dari `SPACE_UPDATE_SIGNING_KEY`, atau dari berkas `--key-file`. Untuk yang kedua, pakai `~/.config/space-release/space-update-signing.key` (dir 0700, berkas 0600) hanya saat vault tidak bisa dipakai, lalu hapus berkas itu dengan `shred -u` setelah selesai. Kunci tidak boleh masuk repo atau handoff, dan tidak boleh dicetak.
- Cadangan terenkripsi age untuk penerima backup Space (`age1xwgxku6n2nre6wu6gfxjjrw50samdmx6qm3l3a9lugkp3w6uzydqppyex8`, identitasnya di vault `space-backup-age-key`): `~/.config/space-release/space-update-signing.key.age` di instabox. Salinan lain ada di repo restic backup Space di R2: `s3:$R2_ENDPOINT/$R2_BUCKET/space` (kredensial vault `r2-tiyoouw` dan `restic-space`), snapshot `57779d3a`, host `instabox-space-release`, tag `space-update-signing`. Kedua salinan sudah dicocokkan lewat hash, dan isinya sama dengan kunci di vault.
- Memulihkan kunci bila vault hilang:
  ```sh
  install -d -m 700 ~/.config/space-release
  # bila berkas .age lokal juga hilang, ambil dari R2:
  enowx creds use r2-tiyoouw restic-space -- sh -c 'restic -r "s3:$R2_ENDPOINT/$R2_BUCKET/space" dump latest --tag space-update-signing /space-update-signing.key.age > ~/.config/space-release/space-update-signing.key.age'
  enowx creds use space-backup-age-key -- sh -c 'age -d -i "$KEY_FILE" -o ~/.config/space-release/space-update-signing.key ~/.config/space-release/space-update-signing.key.age'
  chmod 600 ~/.config/space-release/space-update-signing.key
  go run ./tools/updatesign sign … -key-file ~/.config/space-release/space-update-signing.key   # menolak kunci yang bukan 869a5e520c539842
  ```
  Simpan kembali ke vault (`enowx creds add space-update-signing --platform env-file --value -` dengan isi `SPACE_UPDATE_SIGNING_KEY=<seed>` lewat stdin), lalu `shred -u` berkas polosnya.
- Rotasi:
  1. Buat kunci baru (`go run ./tools/updatesign keygen -out FILE`).
  2. Tambahkan kunci publiknya ke `publicKeys`, lalu rilis versi itu dengan kunci lama.
  3. Setelah pengguna memperbarui, tanda tangani dengan kunci baru dan hapus kunci lama dari daftar.
- Kunci hilang tanpa cadangan berarti klien yang sudah terpasang tidak bisa diperbarui otomatis lagi. Pengguna harus memasang installer baru secara manual.

## Langkah rilis

1. Bangun installer. Gunakan `build_release.py`, atau `mygo build -platform windows/amd64` dengan `MYGO_LDFLAGS="-X main.defaultAPIURL=https://db.tiyoouw.app -X main.version=<versi>"` dan wrapper NSIS `windows_installer.py`. Versi yang ditanam harus sama persis dengan `--version` di bawah.
2. Push commit rilis ke origin.
3. Uji kering, tanpa mengubah GitHub:
   ```sh
   enowx creds use space-update-signing -- python3 scripts/release/publish_update.py \
     --installer dist/space-setup-0.4.6-amd64.exe --version 0.4.6 --channel stable \
     --notes "Ringkasan singkat (≤600 karakter)" --target <commit> --dry-run
   ```
4. Ulangi tanpa `--dry-run`. Skrip akan:
   - menolak bila feed sudah menawarkan versi yang sama atau lebih baru;
   - membuat rilis `v<versi>` bila belum ada (pre-release, bukan latest, untuk rc);
   - mengunggah installer;
   - menandatangani dan memeriksa manifest;
   - mengganti `<kanal>.json` di `update-feed`;
   - membaca ulang feed publik dan installer untuk memastikan semuanya cocok.
5. Rilis rc memakai `--channel rc` dengan versi seperti `0.4.6-rc`. Rilis stabil memakai `--channel stable` dengan versi tanpa akhiran.

Memeriksa manifest yang sedang terbit:

```sh
curl -sL https://github.com/himasifunej/himasif-space/releases/download/update-feed/rc.json -o rc.json
go run ./tools/updatesign verify -channel rc -in rc.json
```

## Catatan

- Installer belum ditandatangani Authenticode. Unduhan lewat updater tidak diberi Mark-of-the-Web, jadi SmartScreen tidak bertanya. Pemasangan manual dari browser tetap memunculkan peringatan.
- Versi pertama yang bisa memperbarui diri adalah versi pertama yang dirilis dari branch ini. Pengguna 0.4.5-rc ke bawah perlu memasangnya secara manual sekali.
