# Berkontribusi ke Space

Terima kasih sudah mau membantu Space. Panduan ini menjelaskan cara kerja di repo ini: dari menyiapkan mesin sampai PR digabung dan dirilis. Dengan berkontribusi, kamu setuju mengikuti [Kode Etik](CODE_OF_CONDUCT.md).

Membaca dengan agen AI? [AGENTS.md](AGENTS.md) berisi aturan yang sama dalam bahasa Inggris, ditambah peta kode.

## Sebelum mulai

- **Ada pertanyaan?** Buka [issue](https://github.com/himasifunej/himasif-space/issues/new) dengan judul diawali `Tanya:`. Pengurus HIMASIF bisa juga bertanya langsung di grup Mediatek.
- **Menemukan bug?** Buka issue: apa yang kamu lakukan, apa yang terjadi, apa yang kamu harapkan, versi Space (Pengaturan › Tentang), dan Windows/web. Tangkapan layar membantu. **Jangan** menyertakan token, kata sandi, nomor HP, atau data anggota.
- **Celah keamanan?** Jangan buka issue publik. Hubungi pemelihara secara pribadi (lihat [Kontak](#kontak)).
- **Ingin fitur baru?** Buka issue dulu sebelum menulis kode. Kebutuhan produk berasal dari [PRD](docs/PRD.md) dan [register keputusan](docs/decision-register.md); fitur yang belum ada di sana perlu disepakati pemilik produk.

## Menyiapkan mesin

Ikuti [Menjalankan untuk pengembangan](README.md#menjalankan-untuk-pengembangan) di README. Ringkasnya: Go 1.27+, PostgreSQL, Python 3, berkas env di `~/.config/himasif-space/dev.env` (di luar repo), lalu `make seed`, `make web`, `make serve`.

## Branch

`main` selalu bisa dibangun dan lulus CI. Jangan push langsung ke `main`.

| Awalan | Untuk | Contoh |
|---|---|---|
| `feat/` | Fitur baru atau perubahan perilaku | `feat/cancel-reason-templates` |
| `fix/` | Perbaikan bug | `fix/updater-ace-motion` |
| `design/` | Eksplorasi desain/canvas, belum tentu digabung | `design/rc5-canvases` |
| `docs/` | Dokumentasi saja | `docs/contributing` |
| `release/` | Penampung satu rilis | `release/0.4.6-rc3` |

- Buat branch dari `main` (atau dari `release/…` yang sedang dibuka bila pemelihara memintanya).
- Satu branch untuk satu topik. Nama pendek, huruf kecil, dipisah `-`.

## Commit

Pesan commit ditulis dalam **bahasa Indonesia**, dari sudut pandang yang terlihat pengguna atau pengembang, bukan nama berkas.

```
<Area>: <apa yang berubah>

<Opsional: mengapa, batasan, dan cara mengujinya.>
```

Contoh dari riwayat:

- `Batalkan tugas: alasan sekali ketuk dan alasan terakhir diingat`
- `Untukku: tes tidak bergantung pada jam saat dijalankan`
- `publish_update: tunggu CDN GitHub sebelum membaca ulang feed`
- `Versi 0.4.6-rc.3: kartu pembaruan di kiri bawah` (commit rilis)

Aturan:

- Satu commit untuk satu perubahan yang utuh; kode, tes, dan dokumentasinya ikut di commit yang sama.
- Baris pertama paling banyak ±72 karakter, tanpa titik di akhir.
- Commit tidak boleh memuat secret, berkas env, database, build (`app/build`, `bin/`, `dist/`), atau data anggota sungguhan.

## Tes

Jalankan sebelum membuka PR:

```bash
make check                       # gofmt, go vet untuk linux/windows/wasm, unit test
make test-db                     # bila mengubah server/store: test terhadap PostgreSQL
python3 scripts/check_contributors.py   # bila mengubah contributors.json
cd bridge && npm test            # bila mengubah bridge WhatsApp
```

- CI (GitHub Actions) menjalankan gofmt, vet, `contributors.json`, dan `go test ./...` dengan PostgreSQL 16 untuk setiap PR. PR merah tidak digabung.
- Perubahan perilaku disertai tes. Tampilan di `internal/appui` bisa dites tanpa jendela; lihat tes yang sudah ada di folder itu.
- Tes tidak boleh bergantung pada jam atau zona waktu mesin. Pakai waktu tetap dan `Asia/Jakarta`.
- Mengubah halaman Pengaturan? Jalankan `go generate ./internal/appui` agar indeks Ctrl K ikut diperbarui.
- Mengubah tampilan? Sertakan tangkapan layar desktop (dan HP/web bila terdampak), terang dan gelap, di PR. Tangkapan yang perlu disimpan masuk `docs/screenshots/`.

## Pull request

1. Push branch-mu lalu buka PR ke `main` (atau ke `release/…` yang ditunjuk). Buka sebagai **draft** selama belum siap ditinjau.
2. Judul mengikuti gaya commit: `Area: apa yang berubah`.
3. Deskripsi berisi:
   - **Apa dan mengapa**, dengan tautan issue atau ID kebutuhan PRD bila ada.
   - **Cara menguji**, termasuk perintah dan hasilnya.
   - **Tangkapan layar** untuk perubahan tampilan.
   - **Migrasi/server**: apakah ada migrasi database atau perubahan API, dan apakah server perlu di-deploy ulang.
4. Tambahkan butir di bagian **"Belum dirilis"** paling atas [CHANGELOG.md](CHANGELOG.md) untuk perubahan yang terasa pengguna.
5. Minimal satu persetujuan pemelihara. Hanya pemelihara yang menggabungkan PR.
6. Tanggapi tinjauan dengan commit baru; jangan force-push setelah tinjauan dimulai kecuali diminta.

Hal yang membuat PR ditolak atau ditahan:

- Wewenang diperiksa hanya di klien. **Server satu-satunya penentu wewenang**; klien hanya menyembunyikan menu.
- Migrasi yang sudah dirilis diubah. Buat migrasi baru dengan nomor berikutnya di `internal/store/migrations`.
- Teks antarmuka bukan bahasa Indonesia, atau bot WhatsApp diberi AI. Bot Space sengaja tanpa AI.
- Dependensi baru tanpa alasan. Space dijaga ringan (RAM, CPU, ukuran unduhan); sebutkan dampaknya bila menambah dependensi.

## Alur rilis

Rilis hanya dilakukan pemelihara, karena butuh kunci penanda tangan pembaruan. Gambaran besarnya:

1. Fitur digabung ke branch `release/<versi>` (mis. `release/0.4.6-rc3`) dengan commit `Merge feat/x ke release/…`.
2. Commit rilis menaikkan versi di `app/mygo.json` **dan** `app/main.go`, memindahkan "Belum dirilis" di `CHANGELOG.md` menjadi bagian versi, dan menambahkan 3–6 butir untuk pengguna di `internal/appui/changelog.json`.
3. Installer dibangun (`scripts/release/build_release.py`), diuji kering, lalu diterbitkan ke kanal `rc` atau `stable` dengan `scripts/release/publish_update.py`.
4. `release/…` digabung ke `main`. Server di-deploy hanya bila berubah; CHANGELOG mencatatnya.

Aturan versi sebelum 1.0: patch naik di setiap pembaruan, minor untuk pembaruan besar. Pre-release memakai `X.Y.Z-rc.N` (bagian angka dibandingkan sebagai angka). Rincian lengkap: [docs/operations/auto-update.md](docs/operations/auto-update.md#langkah-rilis).

## Menambahkan diri ke kontributor

Sudah berkontribusi (kode, desain, dokumentasi, uji pilot)? Tambahkan dirimu ke [`contributors.json`](contributors.json) lewat PR. Formatnya ada di [docs/contributors.md](docs/contributors.md). Namamu akan tampil di halaman Kredit aplikasi.

## Kontak

- Pemelihara: **Ahimsa Jenar Bramsaifstyo Kusuma (Tiyoouw)**, [@Tiyouw](https://github.com/Tiyouw), [tiyoouw.com](https://tiyoouw.com).
- Pertanyaan umum: [issue](https://github.com/himasifunej/himasif-space/issues) berjudul `Tanya: …`.
- Hal pribadi, keamanan, atau pelanggaran kode etik: email pemelihara di [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md#penegakan).
