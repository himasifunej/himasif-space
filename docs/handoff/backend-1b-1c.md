# Handoff Codex kedua — backend Sesi 1B–1C

## Basis dan pembagian

Repository: `https://github.com/himasifunej/himasif-space`. Mulai dari `origin/main`, minimal commit `627298f334624cd5edf1d47bc13e4113815d3cf2` (PR #1 sudah merged, termasuk perbaikan login limiter dan contoh env). Buat branch/worktree sendiri `feat/accounts-spaces-access`; jangan meneruskan branch AWS `feat/1a-skeleton` yang masih lama atau mereset pekerjaan sesi lain.

Sesi UI berjalan pada branch `feat/native-ui`, worktree `/mnt/stb/projects/himasif-space-native-ui` di STB. UI dipindahkan ke komponen Go native MyGo, bukan memasang HTML prototype sebagai frontend produksi. UI memakai kontrak auth/periode/divisi Sesi 1A sambil menunggu backend baru. Preview visual yang disetujui: https://himasif-space-preview.lemonyhorse1024.chatgpt.site .

## Scope backend

Baca `docs/PRD.md`, `docs/decision-register.md`, `docs/implementation-plan.md` dan ADR 0001 terlebih dahulu. Kerjakan Sesi 1B lalu 1C secara berurutan pada backend:

- Akun baru/lama melalui undangan email, verifikasi, aktivasi, pengiriman ulang/pencabutan, pemulihan akun; umur undangan 7 hari. Pertahankan satu akun admin organisasi terpisah dari akun pribadi dan mekanisme sesi yang ada.
- Periode, penetapan kadiv asli, membership per periode, penutupan/read-only, izin arsip lintas periode dan handover sesuai PRD.
- Space divisi dan ruang bersama Medhum, membership, subdivisi Tekno/Desain/Video dan role/kemampuan sesuai scope.
- Anggota boleh melihat jobdesk divisinya. Ruang internal Mediatek tidak terbuka otomatis ke Humas hanya karena ada Medhum.
- Pendamping punya kemampuan operasional sesuai matriks, tanpa pengelolaan role/akses/WA secara default.
- Role **45**: satu aktif per divisi/periode, sampai dicopot kadiv asli tanpa expiry wajib; boleh operasional dan mengelola WA, tetapi tidak boleh mengubah role, membership/access, undangan, arsip atau menunjuk/mencopot 45. Larangan tidak dapat dilewati via endpoint/role kustom. Penunjukan/pencopotan 45 khusus kadiv asli, bukan admin organisasi.
- Hak akses server sebagai penentu, audit perubahan, dan perlindungan lintas divisi/periode/arsip. Jabatan/admin tidak otomatis memberi akses reminder pribadi.

## Kepemilikan file dan kontrak

Backend memiliki `internal/server/**`, `internal/store/**`, migrasi SQL baru, DTO baru pada `internal/api/**`, dan penyesuaian mail/config/seed yang memang diperlukan. Jangan mengedit `app/**`, `internal/appui/**`, `internal/workspaceui/**`, `internal/client/**`, prototype HTML, atau `docs/implementation-plan.md` selama sesi UI berjalan. Tambah handoff sendiri di `docs/handoff/backend-1b-1c-results.md`.

Tetapkan kontrak endpoint/DTO lebih dahulu dalam dokumen terpisah, lalu lanjut implementasi tanpa menunggu konfirmasi. Pertahankan auth/Health/Me/Periods/Divisions Sesi 1A yang sudah dipakai klien. Frontend membutuhkan: daftar Space yang benar-benar dapat diakses pengguna per periode; jenis divisi/shared dan identitas divisi terkait; role/kemampuan efektif untuk mengatur tampilan; periode/read-only dan anggota/undangan yang boleh dikelola. Jangan mengandalkan nama tampilan untuk otorisasi. DTO baru sebaiknya ditambah di file khusus agar tidak mengubah kontrak lama tanpa alasan. Kirim ringkasan kontrak awal ke pengguna atau note HIMASIF agar sesi UI dapat menyambungkannya.

## Verifikasi dan handoff

Jalankan unit dan integrasi PostgreSQL terisolasi, bukan sekadar tes yang skip karena env DB kosong. Uji izin positif/negatif untuk anggota, pendamping, kadiv asli, 45 dan admin; grant/revoke/expiry undangan; closed period/arsip; eskalasi role sendiri; serta dua permintaan 45 yang bersamaan. Tambahkan migrasi baru; jangan mengubah migrasi yang sudah diterapkan.

Gunakan development/test environment AWS yang sudah disiapkan. Jangan mengirim undangan sungguhan ke pengurus, menyentuh Reo, sheet produksi, DNS, atau deploy/cutover live. SMTP/Mailpit/test Mailer cukup untuk pengujian. Jangan menulis secret ke repo/log/notes.

Buat PR dengan kontrak, migrasi, hasil tes yang benar-benar berjalan dan batas tersisa. Jangan merge sendiri; koordinasikan review/integrasi dengan sesi UI. Bot Baileys, parser/SOP produksi, tugas Sesi 2A dan Flutter bukan scope PR ini.
