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
4. **Pemberitahuan.** Setiap versi baru diberitahukan, juga saat pembaruan otomatis menyala:
   - **Kartu di jendela Space**, pojok kiri bawah. Tiga tahap:
     1. "Space X tersedia" dengan satu baris catatan dan tombol **Perbarui**, **Lihat perubahan**, **Nanti**. **Perbarui** mulai mengunduh (pembaruan otomatis yang menyala sudah mengunduh sendiri).
     2. "Mengunduh Space X" dengan progres. Kartu bisa dikecilkan menjadi pil persen.
     3. "Siap dipasang" setelah SHA-256 cocok, dengan **Mulai ulang sekarang** dan **Nanti (saat Space ditutup)**. Nanti di tahap ini memasang versi itu begitu jendela Space ditutup, juga saat pembaruan otomatis mati.

     Kartu tidak mengambil fokus. Kartu menunggu selama pengguna menulis (tugas, pengingat, ping, catatan, pencarian), tetapi tetap tampil bila sudah muncul. Kartu tidak muncul selama onboarding, tur, dan dialog "Yang baru". **Nanti** menyembunyikannya sampai versi berikutnya atau Space dibuka lagi.
   - **Kotak masuk Ace**, berikon pembaruan. Saat jendela terbuka, kabar langsung masuk ke kotak masuk tanpa banner; saat jendela tersembunyi di tray, Ace Island (atau toast Windows bila island mati) menampilkannya lalu menyimpannya. Kabar versi yang sudah terpasang hilang sendiri. Tombol Nanti di island sama dengan Nanti di kartu.

   Setiap versi diumumkan sekali. Bagian **Pembaruan aplikasi** ada di Pengaturan › Perangkat dan di Status layanan; Ctrl K juga menemukannya. Nanti di Pengaturan tetap menahan versi itu selama 24 jam.
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

1. Isi `"version"` di `app/mygo.json` (dan `version` di `app/main.go`) dengan versi rilis yang tepat, misalnya `0.4.5-rc3`. MyGo menanam versi ini ke aplikasi (`mygo.App.Version`) dan ke nama installer; `-X main.version` hanya cadangan. `publish_update.py` menolak installer yang namanya tidak memuat `--version`. Lalu bangun installer: gunakan `build_release.py`, atau `mygo build -platform windows/amd64` dengan `MYGO_LDFLAGS="-X main.defaultAPIURL=https://db.tiyoouw.app -X main.version=<versi>"` dan wrapper NSIS `windows_installer.py`.
   - **Urutan versi pre-release** dibandingkan per bagian dan secara huruf: `rc2 < rc3 < … < rc9`, tetapi `rc10 < rc2` dan `rc.3 < rc2`. Setelah `0.4.5-rc2`, lanjutkan dengan `0.4.5-rc3` … `0.4.5-rc9`, lalu `0.4.5`. Untuk versi berikutnya, pakai `0.4.6-rc.1`, `0.4.6-rc.2`, dan seterusnya (bagian angka dibandingkan sebagai angka).
2. Tulis catatan rilis versi itu di `internal/appui/changelog.json`, entri paling atas. Isinya `version` tanpa akhiran rc (`0.4.6`), `date`, `title`, dan 3–6 butir `notes`: kalimat pendek dalam bahasa pengguna, tentang yang mereka lihat, bukan nama commit. Rilis rc memakai catatan versi rilisnya.
   - Catatan yang sama dipakai di tiga tempat: dialog "Yang baru di 0.4.x" (sekali, setelah pembaruan terpasang), Pengaturan › Tentang › Riwayat perubahan (dibundel di aplikasi, jadi tetap terbaca tanpa koneksi), dan manifest pembaruan yang ditandatangani.
   - Di manifest, butir-butir ditulis ke field `notes` yang sudah ada, satu butir per baris (`• …\n• …`), paling banyak **600 karakter** termasuk tanda butir. `publish_update.py` menolak yang lebih panjang. Tidak ada field baru, karena klien 0.4.5-rc/rc2 membaca manifest dengan `DisallowUnknownFields` dan akan menolak field yang tidak dikenalnya. Klien lama menampilkan butir-butir itu sebagai teks biasa.
   - Dialog memakai catatan bertanda tangan yang disimpan saat installer dijalankan. Bila tidak ada, misalnya pada pemasangan manual, dialog memakai catatan yang dibundel. Instalasi baru tidak menampilkan dialog, karena onboarding didahulukan.
3. Push commit rilis ke origin.
4. Uji kering, tanpa mengubah GitHub. Tanpa `--notes`, skrip mengambil butir-butir versi itu dari `changelog.json`; `--notes "…"` hanya untuk keadaan darurat.
   ```sh
   enowx creds use space-update-signing -- python3 scripts/release/publish_update.py \
     --installer dist/space-setup-0.4.6-amd64.exe --version 0.4.6 --channel stable \
     --target <commit> --dry-run
   ```
5. Ulangi tanpa `--dry-run`. Skrip akan:
   - menolak bila feed sudah menawarkan versi yang sama atau lebih baru;
   - membuat rilis `v<versi>` bila belum ada (pre-release, bukan latest, untuk rc);
   - mengunggah installer;
   - menandatangani dan memeriksa manifest;
   - mengganti `<kanal>.json` di `update-feed`;
   - membaca ulang feed publik dan installer untuk memastikan semuanya cocok.
6. Rilis rc memakai `--channel rc` dengan versi seperti `0.4.6-rc`. Rilis stabil memakai `--channel stable` dengan versi tanpa akhiran.

Memeriksa manifest yang sedang terbit:

```sh
curl -sL https://github.com/himasifunej/himasif-space/releases/download/update-feed/rc.json -o rc.json
go run ./tools/updatesign verify -channel rc -in rc.json
```

## Catatan

- Installer belum ditandatangani Authenticode. Unduhan lewat updater tidak diberi Mark-of-the-Web, jadi SmartScreen tidak bertanya. Pemasangan manual dari browser tetap memunculkan peringatan.
- Versi pertama yang bisa memperbarui diri adalah versi pertama yang dirilis dari branch ini. Pengguna 0.4.5-rc ke bawah perlu memasangnya secara manual sekali.
