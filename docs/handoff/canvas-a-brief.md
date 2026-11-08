# Brief bersama — menyamakan Space dengan canvas A

Kamu salah satu dari beberapa sesi agen yang bekerja paralel pada app "Space" (Go + library UI MyGo v0.7.1, sumber di /home/enowx/go/pkg/mod/github.com/egoist/mygo@v0.7.1; kode UI di internal/appui dan internal/workspaceui). Tujuan: tampilan app sama dengan desain terpilih "canvas A". Sesi utama (Claude, sesi 3d9d996fdf80) yang menggabungkan semua branch, build web, cek visual dan deploy — kamu TIDAK deploy.

## Referensi (folder ini: /home/enowx/HIMASIF/.shots/canvasA)
- `cvA.html` — HTML statis canvas A desktop 1440×900 (CSS + JS inline; data contoh di JS: `tasks`, `people`, `ICON`, fungsi render*). `cvAm.html` — frame HP 390×844.
- `cv/*.png` — screenshot canvas per halaman/tab. `pair/*.png` — kiri canvas, kanan app LAMA (sebelum Tahap 1). `new/*.png` — app setelah Tahap 1 (kerangka sudah sesuai).
- Audit lengkap: catatan enowX `fff1694dd1a6` → `"$ENOWX_BIN" notes show fff1694dd1a6`. Tahap 1 (font Plex, sidebar, topbar, baris tab 9 tab) SUDAH selesai; jangan ubah kerangka itu.
- Untuk merender canvas sendiri: `node canvas.cjs <file.html> <outdir> 1440 900 '[["nama","<js untuk klik>"]]'` (butuh `export LD_LIBRARY_PATH=$(find /home/enowx/HIMASIF/space-feedback-qa-aws -name "*.so*" -printf '%h\n' | sort -u | tr '\n' ':')`). Membuka browser memakan RAM: maksimal sekali-dua kali, jangan berulang.

## Aturan wajib
- Branch kerjamu harus berbasis `feat/feedback-01-pilot5-ui` commit 77b4f49 atau sesudahnya. Cek `git log --oneline -1` di awal; kalau worktree-mu dibuat dari `main`, `git reset --hard 77b4f49` sebelum mulai (worktree-mu sendiri, aman).
- RAM VPS 1,9 GB dipakai bersama. SEMUA go build/test/vet harus lewat lock dan hemat memori:
  `flock /tmp/space-build.lock env GOMAXPROCS=1 GOFLAGS=-p=1 GOMEMLIMIT=256MiB GOGC=50 go test ./internal/appui ./internal/workspaceui`
  Jangan `make web`, jangan headless browser untuk app (sesi utama yang cek visual). gofmt sebelum commit.
- Test harus lolos kecuali 3 test NoteDocument yang sudah gagal sejak dulu. Tambah test untuk perilaku baru; perbarui test lama hanya bila perubahannya memang disengaja.
- Teks UI bahasa Indonesia, ikuti kata-kata canvas. Ikuti izin/role yang sudah ada; jangan membuat endpoint/migrasi backend baru kecuali brief tahapmu mengizinkan — kalau data yang dibutuhkan canvas tidak ada di API, tampilkan yang ada dan catat di laporan.
- Pakai data yang sudah dimuat app; jangan menambah request hanya untuk hiasan.
- Jangan sentuh file di luar lingkup tahapmu kecuali terpaksa (sesi lain mengubahnya); sebutkan di laporan kalau terpaksa.
- Commit di branch-mu dengan pesan jelas, akhiri tiap pesan commit dengan baris `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Jangan push, jangan deploy, jangan sentuh /home/enowx/.local/opt.
- Selesai: tulis laporan singkat di `docs/handoff/canvas-a-<tahap>.md` (apa yang sudah sesuai canvas, yang dilewati dan alasannya, file yang disentuh, hasil test verbatim), commit, lalu kirim ringkasan ke sesi utama: `"$ENOWX_BIN" send 3d9d996fdf80 "<tahap> selesai di branch <nama>: <ringkasan 3 baris>"`. Setelah itu berhenti.
