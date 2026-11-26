# Rencana: nomor bot WhatsApp pindah ke instans stabil

F1-22. Ditulis 26 Nov 2026. **Rencana saja, belum dijalankan.** Dikerjakan sesudah [cutover instans stabil](cutover-stabil-30nov.md) dan dengan persetujuan owner lewat GOA.

## Keputusan yang dipakai

- Bot Space memakai **nomor Reo** sebagai perangkat tertaut (multi-device); Reo (Hermes) tetap jalan dengan allowlist sendiri (ADR 0003, R-09).
- **Satu nomor hanya tersambung di satu instans Space.** Rekomendasi F1-22: nomor ke instans stabil, instans uji berjalan **tanpa bridge**.
- **Cutover per grup** (R-10, R-11): satu grup tidak pernah dilayani dua sistem. Grup pertama adalah grup **Test**.
- Bridge dan sesinya per instans; kunci state WA (`SPACE_WA_STATE_KEY`) dan token bridge **baru** untuk instans stabil, tidak disalin dari uji.

## Kenapa tidak cukup memindahkan sesi

Koneksi WA di Space terikat ke **periode** (`wa_connections`, grup, `wa_identities`; lihat [period-rollover-20262027.md](../handoff/period-rollover-20262027.md)). DB stabil mulai kosong dengan periode 2026/2027, jadi koneksi, pemetaan grup dan tautan nomor anggota dibuat baru di instans stabil. Sesi lama di tc-vps tidak dibawa; nomor yang sama dipasangkan ulang lewat QR sebagai perangkat tertaut baru.

## Prasyarat

- Instans stabil hidup di `space.himasif.id`, admin dan periode 2026/2027 ada, email jalan (runbook langkah 4).
- Pemegang HP nomor bot siap memindai QR, di jam yang disepakati.
- Kapasitas perangkat tertaut nomor itu: Reo + bridge uji + bridge stabil tidak melebihi batas WhatsApp (4 perangkat tertaut). Daftar perangkat di HP dicatat dulu.
- Di `tiyoouw-studio`: Node 22 (binary sungguhan, bukan symlink) dan `ffmpeg`/`ffprobe` di `PATH` unit bridge.

## Langkah

1. **Siapkan bridge stabil (tanpa pairing).** Salin `bridge/` dari rilis stabil ke `~/.local/opt/himasif-space-stable/releases/<VER>/bridge`, `npm ci --omit=dev` di host ini. Buat `~/.config/himasif-space-stable/bridge.env` (0600) dengan token bridge baru (≥32 karakter), `SPACE_WA_STATE_KEY` baru, `SPACE_WA_STATE_DIR=~/.local/share/himasif-space-stable/whatsapp`, listen `127.0.0.1` port **8791** (uji memakai port lain di tc-vps). Simpan kedua nilai di vault (`himasif-space-stable-bridge`), bukan di catatan.
2. **Unit** `himasif-space-stable-bridge.service` meniru unit bridge uji (`MemoryMax=256M`, `NODE_OPTIONS=--max-old-space-size=160`, `Restart=on-failure`). Isi `SPACE_WA_BRIDGE_URL=http://127.0.0.1:8791` dan token yang sama di `server.env` stabil, restart api. Tambahkan `himasif-space-stable-bridge.service` ke `bridge_units` di `backup-policy.json` dan direktori state ke `wa_state_dir`, supaya backup menghentikan bridge dan ikut menyimpan state terenkripsi.
3. **Hentikan bridge uji** di tc-vps setelah owner setuju: `systemctl --user disable --now himasif-space-pilot-bridge` (sebagai `enowx`). Kosongkan `SPACE_WA_BRIDGE_URL`/`TOKEN` di `server.env` uji lalu restart api uji, supaya instans uji tidak mencoba mengirim. Sesi dan `bridge.env` uji **disimpan**, tidak dihapus (jalan kembali).
4. Di HP bot: **keluarkan perangkat tertaut milik bridge uji** dari daftar perangkat tertaut. Perangkat Reo tidak disentuh.
5. **Pairing stabil.** Admin/kadiv masuk ke `space.himasif.id` → pengaturan bot WA periode 2026/2027 → tampilkan QR (hanya di UI privat, tidak di log) → pemegang HP memindai. Status koneksi harus tersambung.
6. **Grup Test dulu.** Petakan hanya grup Test. Uji `!tugas`, `!hari`, satu Reminder terjadwal dan satu bukti kiriman. Pastikan Reo tidak menjawab di grup Test (allowlist Hermes).
7. **Grup nyata satu per satu** sesuai jadwal owner: hapus grup dari allowlist Hermes → petakan di Space → uji satu command → catat waktu pindah. Anggota menautkan nomornya lagi (`!link`).
8. Catat hasil di F1-22 (catatan rencana) dan `docs/operations`.

## Verifikasi

- `journalctl --user -u himasif-space-stable-bridge` tanpa loop reconnect; Status layanan di Space menunjukkan koneksi sehat dan antrean outbox kosong/terproses.
- Tidak ada pesan ganda di grup yang dipindah (Space dan Reo tidak sama-sama menjawab).
- Instans uji: tidak ada bridge berjalan; health uji tetap 200.

## Rollback

1. Hentikan dan disable `himasif-space-stable-bridge`; kosongkan URL/token bridge di `server.env` stabil, restart api stabil.
2. Kembalikan grup yang sudah dipindah ke allowlist Hermes, supaya tidak ada grup tanpa layanan.
3. Bila instans uji perlu bot lagi: keluarkan perangkat tertaut stabil di HP, isi kembali URL/token bridge di `server.env` uji, `systemctl --user enable --now himasif-space-pilot-bridge`. Bila sesi uji sudah tidak diterima WhatsApp, pasangkan ulang lewat QR.
4. State WA stabil disimpan (terenkripsi), tidak dihapus, sampai owner memutuskan.

## Risiko

- Pairing ulang butuh pemegang HP; di luar jam itu bot mati (tidak termasuk RTO 4 jam, F0-11).
- Satu sesi Baileys yang dipakai di dua host sekaligus bisa membuat WhatsApp mengeluarkan perangkat itu. Bridge tidak boleh jalan di dua host dengan state yang sama.
- Selama Reo jalan, Reminder Medhum Space tidak dikirim ke grup yang masih dilayani Reo (BOT-14).
