# Tahap A2: drawer Ubah akses (desktop) dan sheet (HP)

Branch `feat/anggota-a2` dibuat dari `main` (058fb6c, 0.3.2). Desainnya canvas Anggota `0b2425c385f0`, desain 1 (A · Desktop) dan 2 (A · HP). Rencananya ada di `docs/handoff/anggota-catatan-rapat-rencana.md`, tahap A2. Belum di-push dan belum di-deploy.

## Yang berubah
- **"Ubah role" kini bernama "Ubah akses"** dan membuka **drawer kanan** (desktop, lebar ≤ 480 px, latar diredupkan). Di HP (< 720 px) yang terbuka adalah **bottom sheet** setinggi 92% layar dengan pegangan di atasnya. Panel inline lama di bawah baris sudah dihapus.
- Urutan isi drawer:
  1. **Kepala**: avatar, "Ubah akses · Nama", kelompok · N tugas aktif, dan tombol tutup.
  2. **Role dasar**: kartu radio Anggota / Pendamping kadiv, masing-masing dengan penjelasannya.
  3. **Wakil operasional (45)**: sakelar dengan kalimat yang menjelaskan keadaannya ("45 divisi ini sekarang X", "Menggantikan X saat kamu menyimpan", "dicopot saat kamu menyimpan").
     - Kalau sudah ada 45 lain, menyalakan sakelar membuka **dialog "Ganti 45 dari X ke Y?"** (Batal ganti 45 / Ganti 45).
     - "Sesuaikan kewenangan 45" tersedia untuk penunjukan baru.
     - Hanya kadiv asli yang melihat bagian ini.
  4. **Kelompok**: chip Tekno/Desain/Video, khusus Mediatek seperti sebelumnya.
  5. **Role kustom**: checkbox beserta kewenangannya. Kalau belum ada role kustom, tampil petunjuk tempat membuatnya.
  6. **Ringkasan "Yang bisa dilakukan …"**:
     - Dihitung di klien dengan `authz.RoleCaps`/`authz.Default45`, aturan yang sama dengan server.
     - Asal tiap kewenangan ditulis ("dari 45", "dari Editor web"), lalu "Tidak bisa: …" dan "Atur role, undangan dan 45 · hanya kadiv".
     - Setelah simpan, sumber kebenarannya tetap `Capabilities` dari server.
  7. **Kotak merah "Keluarkan dari …"**:
     - Konfirmasi tampil di dalam drawer: akses dicabut, riwayat tetap, dan N tugas aktif yang perlu dialihkan di board.
     - Anggota yang masih 45 harus dicopot dari 45 dulu.
     - Syarat pengalihan tugas menyusul di A3.
  8. **Kaki**: "N perubahan belum disimpan" (atau "Belum ada perubahan"), **Batal** dan **Simpan akses**. Simpan tidak aktif kalau belum ada perubahan.
- **Simpan berurutan**:
  - `UpdateMember` (role, kelompok, role kustom) dijalankan dulu, lalu `Remove45` bila 45 diganti atau dicopot, lalu `Appoint45`.
  - Kalau langkah kedua atau ketiga gagal, pesannya menyebut yang sudah tersimpan, misalnya "Role, kelompok dan role kustom tersimpan, tetapi menunjuk 45 gagal: …", lalu data dimuat ulang.
- **Tidak ada perubahan yang hilang diam-diam**: menutup lewat X, klik di luar, atau Escape saat ada perubahan menanyakan "Buang N perubahan …?" (Lanjut mengubah / Buang perubahan). "Batal" tetap berarti membuang.
- **Tabel**:
  - Kolom diperkecil (Role 150, Kelompok 80, Beban 116, WhatsApp 112, aksi 136).
  - Kolom **Kelompok disembunyikan kalau tidak ada yang punya kelompok** (seperti pilot sekarang).
  - Hasilnya, nama di lebar 1280 tidak terpotong lagi; di 0.3.2 tertulis "Demo Medi…". Nama juga punya tooltip.
- Konfirmasi tingkat halaman kini hanya untuk cabut undangan dan hapus role kustom.

## Berkas
- `internal/appui/members_access_drawer.go` (baru): drawer/sheet, ringkasan kewenangan, simpan bertahap, dialog ganti 45, keluarkan.
- `internal/appui/members.go`: state drawer, penanganan simpan sebagian di `memberWrite`, `memberEditor` lama dihapus.
- `internal/appui/members_view.go`, `members_table.go`: "Ubah akses", kolom, panel inline dihapus.
- Test: `members_access_drawer_test.go` (baru). `canvas_tahap4_test.go` diperbarui dengan sengaja karena "Ubah role" berganti menjadi drawer.

## Test
`members_access_drawer_test.go` mencakup:
- ringkasan kewenangan per kombinasi role, 45 dan role kustom;
- urutan isi drawer, tanpa panel lama;
- simpan role, kelompok dan ganti 45 dengan urutan `update,remove45,appoint45` serta isi request;
- pesan simpan sebagian;
- keluarkan (45 diblokir, anggota biasa terkirim `RemoveMember`);
- sheet HP;
- "Ubah akses" tersembunyi tanpa hak;
- kolom Kelompok dan ruang untuk nama;
- tanya sebelum membuang perubahan.

## Screenshot
Dibuat dengan renderer native, tema terang, data dummy:
- `docs/screenshots/anggota-a2-1280.png`
- `anggota-a2-drawer.png`
- `anggota-a2-ganti-45.png`
- `anggota-a2-hp-sheet.png`

Dibuat ulang dengan `SPACE_UI_CAPTURE_DIR=<dir> go test ./internal/appui -run TestMembersTableNativeCapture`.

## Hasil cek (Windows)
- gofmt bersih.
- `go vet` linux/windows/wasm ok.
- `go test ./...` dengan Postgres lokal: 12 paket ok. 4 test filestore gagal karena cek izin 0700 di Windows, sama seperti sebelumnya.

## Untuk owner
- Bagian 45 hanya tampil untuk kadiv asli. Akun 45 sendiri tidak bisa membuka "Ubah akses".
- Cek di app nyata: buka "Ubah akses" seorang anggota, ubah role, simpan, lalu lihat badge di tabel. Coba juga menyalakan 45 untuk anggota lain dan pastikan dialog ganti 45 muncul.
