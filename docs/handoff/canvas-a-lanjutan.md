# Lanjutan: menyamakan Space dengan canvas A (8 November 2026)

Handoff dari sesi Claude di aws-vps (3d9d996fdf80) supaya pekerjaan bisa dilanjutkan di host lain (PC rumah) dengan konteks yang sama. Baca file ini dulu, lalu yang dirujuknya.

## Posisi sekarang
- Branch: `feat/feedback-01-pilot5-ui`, commit terakhir tercatat di `git log`. Semua sudah di-push ke GitHub.
- Live: https://space.tiyoouw.app/ (web saja; server tidak berubah sejak rilis `0.3.0-pilot.13-212f6eaa1793`). Login demo: `demo@mediatek` / `mediatek`.
- Selesai dan live (urutan commit): geser jari di web HP (589f2e3) → Tahap 1 kerangka (77b4f49) → Tahap 2 board + detail (976039f) → Tahap 3 jobdesk + beranda (16d52ef) → Tahap 4 agenda, anggota, reminder, arsip (5265649) → baris tab muat 1440 px (8a05738).
- Test: `go test ./internal/appui ./internal/workspaceui` lolos kecuali 3 test NoteDocument yang sudah gagal sejak sebelum pekerjaan ini.

## Referensi
- Canvas desain: enowX canvas `1fd6e97b5ee7` "Space — UI/UX v2 (interaktif)", frame **A · Hangat gelap** (desktop) dan **A · Mobile**. HTML-nya bisa diambil di host mana pun: `"$ENOWX_BIN" canvas show 1fd6e97b5ee7 --frame 1 --html > cvA.html` (frame 6 = A · Mobile).
- Audit lengkap + to-do tahap: catatan enowX `fff1694dd1a6` "Space — Audit UI vs canvas A (8 Nov)" (`"$ENOWX_BIN" notes show fff1694dd1a6`). Tahap 1–4 tercentang.
- Laporan tiap tahap: `canvas-a-tahap2.md`, `canvas-a-tahap3.md`, `canvas-a-tahap4.md` di folder ini. Aturan kerja untuk sesi paralel: `canvas-a-brief.md`.
- Keputusan desain lama: catatan enowX "Pilihan desain 3 November" dan "Perbaikan Dekstop & Web" (f44f7807cd51).

## Keputusan yang sudah diambil
- Tab Space mengikuti canvas: 9 tab (Beranda, Board tugas, Agenda, Catatan, Rapat, Tinjauan, Ping, Anggota, Bot WhatsApp), bisa digeser urutannya, "Atur tab" untuk tampil/sembunyi. Urutan disimpan **per perangkat** (preferences), belum per akun.
- Tampilan (rekomendasi Claude, menunggu persetujuan owner): tombol Tampilan di sidebar membuka popover kecil (Tema, Aksen, Kepadatan) + tautan "Semua pengaturan tampilan…" ke panel lengkap bergaya enowX (palet, aksen kustom, warna board), juga ada di Pengaturan → Perangkat ini → Tampilan dan tema.
- Font IBM Plex Sans/Mono di-embed (desktop TTF, web WOFF2).
- Web HP: loader (`scripts/release/patch_web_index.py`) mengubah geser jari jadi scroll; elemen yang ditarik (jam analog, tepi resize) memanggil `holdTouchPan`.

## Berikutnya
1. Cek visual yang belum: halaman Reminder dan Arsip (desktop), lalu semua halaman baru di HP 390 px (Jobdesk, Beranda, Agenda, Anggota, detail tugas). Juga font Plex di desktop Windows dan menggeser urutan tab dengan mouse.
2. Periksa label Beranda "minggu ke-49 periode": wajar bila periode 2025/2026 mulai sekitar Desember 2025; kalau tidak, hitungannya salah.
3. Tahap 5 (butuh backend + migrasi DB pilot, backup dulu): tombol Cocok/Tidak cocok langsung di tab Tinjauan, riwayat ping (API belum ada), urutan/tab tersembunyi ikut akun, data WhatsApp per anggota di tabel Anggota (kolom masih "—"), nomor tugas urut #T.. (kartu masih memakai potongan ID), unggah lampiran berkas.
4. Owner: putuskan Tampilan (to-do di catatan audit).

## Build dan deploy
Sejak 9 Nov 2026 pukul 14:04 UTC, pilot berjalan di **tc-vps**; aws-vps sudah pensiun. Alur yang berlaku ada di `docs/handoff/deploy-pilot.md`:
- Build dikerjakan di PC (`build-local.sh`), lalu artefak diunggah dan dipasang (`deploy.sh --dry-run | --stage | --go`).
- Jangan build di tc-vps: RAM-nya 1,9 GB dan dipakai bersama aplikasi lain.
- Rilis web tetap di `/home/enowx/.local/opt/himasif-space-pilot/releases/0.3.0-pilot.13-212f6eaa1793/web`, dengan cadangan `web.before-<versi>`.
- Alat cek visual Playwright dulu ada di aws-vps (`/home/enowx/HIMASIF/.shots/...`) dan tidak ikut dipindah. Uji UI sekarang dilakukan owner sendiri di pilot.
