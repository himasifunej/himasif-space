# Handoff sementara backend Sesi 1B–1C — belum diimplementasikan

Pekerjaan dihentikan atas permintaan pengguna untuk dialihkan ke Kiro. Jangan menganggap dokumen kontrak atau DTO sebagai endpoint yang sudah tersedia.

## Yang sudah tersedia

- Branch `feat/accounts-spaces-access` dibuat dari `origin/main`, baseline `627298f334624cd5edf1d47bc13e4113815d3cf2`.
- `docs/handoff/backend-api-contract.md`: draft awal endpoint, bentuk JSON dan aturan otorisasi untuk ditinjau/dilanjutkan Kiro.
- `internal/api/access.go`: draft DTO tambahan, belum digunakan server. Kontrak admin handover masih perlu difinalisasi.
- Dokumen PRD, register keputusan, rencana implementasi dan ADR 0001 sudah dibaca.

## Belum dikerjakan

Seluruh implementasi akun/undangan, periode/membership, Space, role/45, arsip, handover, otorisasi, audit tambahan dan migrasi baru. Belum ada perubahan mail/config/seed. Belum ada unit/integration test baru atau hasil integrasi PostgreSQL. Belum ada PR. Tidak ada migrasi dijalankan, database dev direset, email dikirim, atau deployment dilakukan.

## Lingkungan dan batas

- Panduan SES tersedia di `/home/enowx/HIMASIF/himasif-space/SES-SENDER.md`; kredensial tetap di `/home/enowx/.config/himasif-space/secrets.env` (mode 600). Jangan menampilkan atau menyalinnya. From wajib `@himasif.id`; gunakan Mailpit/test Mailer untuk pengujian. Variabel SES dalam panduan belum diadaptasi ke konfigurasi SPACE_* oleh pekerjaan ini.
- Docker tidak dapat diakses oleh user sesi ini; sudo noninteraktif juga tidak tersedia. Percobaan menyiapkan source PostgreSQL 17.6 dilakukan di `/tmp/space-pg-build`; belum ada database test yang dibuat/dijalankan. Periksa sendiri lingkungan test yang tersedia; tes skipped bukan bukti integrasi lulus.
- UI dikerjakan sesi lain di STB, branch `feat/native-ui`, `/mnt/stb/projects/himasif-space-native-ui`. Handoff aslinya ada di `docs/handoff/backend-1b-1c.md` pada worktree STB tersebut. Kontrak awal telah disampaikan kepada pengguna, belum dikirim langsung ke sesi UI.
- Pertahankan batas kepemilikan file dari prompt asli. Jangan mengubah file UI, migrasi lama atau `docs/implementation-plan.md`. Jangan menyentuh Reo, sheet produksi, DNS atau deploy live. Buat PR untuk review; jangan merge sendiri.
