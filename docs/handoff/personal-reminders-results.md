# Hasil backend reminder pribadi (Sesi 2A, bagian PRI)

Branch `feat/personal-reminders`, dari `origin/main` (`d92a372`). Kontrak: [personal-reminders-api.md](personal-reminders-api.md); DTO di `internal/api/reminders.go`. Memenuhi PRD §8 (PRI-01–08) dan AC-09.

## Yang dibuat

- Migrasi `internal/store/migrations/0004_personal_reminders.sql` (baru): tabel `reminders`, `reminder_shares`, `reminder_preferences`. Tidak mengubah `0001`/`0002`; `0003_tasks.sql` (sesi Codex) tidak dibuat/diubah.
- `internal/api/reminders.go`: DTO Reminder, request buat/ubah/selesai/berbagi/toggle/preferensi, `ReminderShare`, `ReminderAuditEntry`.
- `internal/store/reminders.go`: CRUD reminder, share/unshare, daftar milik-sendiri dan yang-dibagikan, visibilitas (`ReminderVisibleTo`), preferensi default, pembaca riwayat audit per reminder.
- `internal/server/reminders.go` + rute di `server.go` (`/api/reminders...`): endpoint kontrak, otorisasi berbasis kepemilikan, penerima hanya-baca.
- Setiap perubahan dan audit-nya satu transaksi (`target_type = 'reminder'`), dibaca pemilik lewat `GET .../history`.

## Keputusan teknis

- **Tanpa scope divisi.** Reminder adalah objek akun, bukan objek Space. Rute di `/api/reminders`, bukan di bawah `/api/periods/{p}/divisions/{d}`. Backend tugas (PJ desain milik divisi, dikerjakan Codex) tetap memeriksa divisi pemiliknya sendiri; reminder tidak menyentuh itu.
- **Privat vs. berbagi.** `shared` adalah toggle pemilik; penerima adalah baris `reminder_shares`. Penerima melihat reminder hanya ketika `shared` menyala **dan** ada baris share untuknya. Mematikan toggle menyembunyikan tanpa menghapus daftar — menghidupkan lagi memakai daftar yang sama (PRI-02, PRI-05).
- **Melekat pada akun (PRI-03/04).** `recipient_id` menunjuk `users.id` langsung. Tidak ada penurunan akses dari role/jabatan, sehingga pergantian kadiv/45/admin/BPH/penerus tidak mewarisi apa pun.
- **Default (PRI-05).** `reminder_preferences.default_share` dibaca hanya pada create. Perubahan preferensi tidak menyentuh baris reminder mana pun (diuji di `TestReminderDefaultShareDoesNotReopenOld`).
- **Waktu (MED-04/05).** `due_kind` + constraint memastikan tepat satu dari `due_on` (tanggal) / `due_at` (instan, UTC). `time_zone` divalidasi lewat `timeutil.LoadZone`. `internal/timeutil` dipakai apa adanya; tidak diubah.
- **PRI-08.** Reminder yang tidak boleh dilihat menjawab 404 (tak dapat dibedakan dari tidak ada). Endpoint Space/arsip/direktori tidak membaca tabel reminder; diuji di `TestReminderDoesNotLeakThroughSpaces`.
- **Admin organisasi** ditolak 403 di semua endpoint reminder (akun bersama, bukan pengurus pribadi).

## Verifikasi yang dijalankan

`make fmt vet` (linux, windows, js/wasm) dan `make test-db` terhadap PostgreSQL dev (`SPACE_TEST_DATABASE_URL`, database sementara per test): seluruh test lulus, **tidak ada yang skipped**. Tujuh test reminder baru di `internal/server/reminders_test.go`:

- `TestReminderCreatePrivateByDefault` — PRI-01/02/08: dibuat privat, pemilik melihat, orang lain tidak (404).
- `TestReminderAdminHasNone` — PRI-04: admin organisasi 403.
- `TestReminderDateVersusInstant` — MED-04/05: tanggal vs instan, tolak campuran, tolak zona tak dikenal.
- `TestReminderSharingIsPerAccountAndReadOnly` — PRI-03/05/07: berbagi ke akun, pemilik lihat/cabut, penerima 403 pada semua tulis + baca daftar/riwayat.
- `TestReminderDefaultShareDoesNotReopenOld` — PRI-02/05.
- `TestReminderHistoryForOwner` — AC-09: create/update/complete tercatat, pemilik membaca; reopen.
- `TestReminderDoesNotLeakThroughSpaces` — PRI-08.

## Belum / batas

- **Pengiriman reminder** ke perangkat, toast Windows dan WhatsApp **tidak termasuk** sesi ini (Sesi 2C/3A). PRI-06 (tidak ke grup WA) ditegakkan nanti di lapisan pengiriman; backend ini tidak mengirim apa pun.
- **UI** (`app/`, `internal/appui`, `internal/client`) belum memakai endpoint ini; perlu halaman daftar reminder, form buat/ubah, toggle berbagi + pemilih penerima (dari daftar akun yang boleh dilihat UI), dan tampilan riwayat. Tidak diubah pada sesi ini.
- **Tidak ada** migrasi DB dev/produksi, email, atau deploy. Migrasi 0004 baru diterapkan pada database test sementara.
- Backend tugas (`0003_tasks.sql`, jobdesk, delegasi) milik sesi Codex; tidak disentuh. `docs/implementation-plan.md` tidak diubah.

## Untuk penyambungan UI

1. Daftar: `GET /api/reminders` sudah menggabungkan milik-sendiri dan yang-dibagikan; pakai `relation` untuk membedakan dan menonaktifkan aksi tulis pada `recipient`.
2. Buat/ubah: kirim `dueKind` + salah satu `dueOn`/`dueAt` + `timeZone`. Untuk input tanggal-saja jangan kirim jam.
3. Berbagi: tambah penerima lewat `recipientId` (gunakan daftar akun yang sudah boleh dilihat pengguna), lalu nyalakan toggle `PUT /sharing`. Tampilkan `recipients` hanya untuk pemilik.
4. Riwayat: `GET /history` untuk menampilkan perubahan; `before`/`after` adalah objek bebas.
