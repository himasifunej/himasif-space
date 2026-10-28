# Backend API — Reminder pribadi (Sesi 2A)

Status: **diimplementasikan** di branch `feat/personal-reminders` (dari `origin/main`, `d92a372`) dan diuji terhadap PostgreSQL; lihat [hasil](personal-reminders-results.md). DTO di `internal/api/reminders.go`. UI tetap MyGo native (desktop/WASM) dan belum memakai endpoint ini.

Memenuhi PRD §8 (PRI-01–08) dan AC-09. Melengkapi [kontrak akun/Space 1B–1C](backend-api-contract.md); aturan umum di sana tetap berlaku (sesi Bearer sama, ID UUID, waktu RFC3339 UTC, tanggal YYYY-MM-DD, body JSON ketat, field tak dikenal ditolak, bentuk error sama).

## Model dan aturan

Reminder pribadi **dimiliki satu akun** dan **privat secara bawaan** (PRI-01, PRI-02). Tidak ada scope divisi/Space: otorisasi adalah kepemilikan. Hanya pemilik yang boleh membuat, mengubah, menandai selesai, membagikan, dan menghapus; penerima hanya membaca (PRI-07).

- **Privat & toggle berbagi (PRI-02):** `shared` dapat diubah kapan saja lewat `PUT /sharing`. Mematikan toggle menyembunyikan reminder dari penerima tanpa menghapus daftar penerima.
- **Berbagi melekat pada akun, bukan jabatan (PRI-03, PRI-04):** penerima adalah `recipientId` akun pengurus yang dipilih saat itu. Pergantian pemegang jabatan (kadiv pengganti, 45, admin, BPH, penerus periode) **tidak** mewarisi akses. Tidak ada logika berbasis role di sini.
- **Default tidak membuka reminder lama (PRI-05):** `defaultShare` dibaca hanya saat reminder dibuat. Mengubahnya tidak menyentuh reminder yang sudah ada.
- **Pemilik melihat & mencabut penerima (PRI-05):** `GET`/`POST`/`DELETE .../recipients`.
- **Penerima hanya baca (PRI-07):** semua endpoint tulis pada reminder milik orang lain menjawab 403, termasuk ubah, selesai, pindah berbagi, dan tambah penerima. Reminder tidak pernah menjadi tugas divisi — tidak ada endpoint untuk itu.
- **Tidak bocor lewat Space (PRI-08):** endpoint Space/arsip/direktori tidak membaca tabel reminder. Reminder yang tidak boleh dilihat pemanggil menjawab 404, tidak dapat dibedakan dari yang tidak ada.
- **Waktu (MED-04, MED-05, NFR-10):** `dueKind` membedakan tanggal tanpa jam (`"date"`, memakai `dueOn`) dari instan (`"instant"`, memakai `dueAt` UTC). Tanggal tidak pernah diberi jam karangan. `timeZone` (default `Asia/Jakarta`) adalah zona yang dipilih pemilik, untuk menampilkan.
- **Sesi mutasi:** akun/sesi diperiksa ulang di dalam transaksi setelah akun terkait dikunci dalam urutan UUID, sebelum baris reminder. Request yang sudah dicabut tidak dapat melanjutkan penulisan.
- **Riwayat (AC-09):** setiap perubahan dicatat di `audit_events` dalam transaksi yang sama (`target_type = 'reminder'`), dibaca pemilik lewat `GET .../history`.
- **Akun admin organisasi** tidak memiliki reminder pribadi: semua endpoint menjawab 403 untuknya.

## Reminder

```json
{
  "id": "019a2810-0000-7000-8000-000000000010",
  "ownerId": "019a2810-0000-7000-8000-000000000011",
  "title": "Kumpulkan desain feed",
  "notes": "",
  "dueKind": "date",
  "dueOn": "2026-11-10",
  "timeZone": "Asia/Jakarta",
  "completed": false,
  "shared": false,
  "recipients": [],
  "relation": "owner",
  "createdAt": "2026-10-28T02:00:00Z",
  "updatedAt": "2026-10-28T02:00:00Z"
}
```

- `dueKind`: `date` (maka `dueOn` ada, `dueAt` tidak) atau `instant` (maka `dueAt` ada, `dueOn` tidak).
- `relation`: `owner` atau `recipient`. `shared` dan `recipients` hanya terisi untuk pemilik; penerima tidak melihat siapa lagi yang menerima.
- `completedAt` (RFC3339 UTC) ada hanya saat `completed` true.

## Endpoint

Semua membutuhkan sesi Bearer akun pengurus (bukan admin organisasi).

- `GET /api/reminders` — reminder milik sendiri **dan** yang dibagikan kepada pemanggil, masing-masing dengan `relation`. Urutan: jatuh tempo terdekat dahulu, selesai di belakang. `{"items":[Reminder]}`.
- `POST /api/reminders` — buat; body `ReminderRequest`; 201 Reminder. Mulai privat kecuali `defaultShare` pemilik menyala; `shared` diabaikan di body create.
- `GET /api/reminders/{reminderId}` — satu reminder yang boleh dilihat pemanggil (miliknya atau yang sedang dibagikan kepadanya). Selain itu 404 (PRI-08).
- `PUT /api/reminders/{reminderId}` — ganti isi (judul, catatan, waktu); body `ReminderRequest`; 200 Reminder. Hanya pemilik.
- `DELETE /api/reminders/{reminderId}` — hapus reminder dan semua share-nya; 204. Hanya pemilik.
- `POST /api/reminders/{reminderId}/complete` — body `{"completed":true|false}`; 200 Reminder. Hanya pemilik (PRI-07).
- `PUT /api/reminders/{reminderId}/sharing` — body `{"shared":true|false}`; 200 Reminder. Toggle berbagi; daftar penerima tetap.
- `GET /api/reminders/{reminderId}/recipients` — daftar penerima; 200 `{"items":[ReminderShare]}`. Hanya pemilik.
- `POST /api/reminders/{reminderId}/recipients` — body `{"recipientId":"<akun-pengurus-aktif>"}`; 201 Reminder (dengan daftar penerima terbaru). Tidak boleh ke diri sendiri atau ke akun yang bukan pengurus aktif. 409 bila sudah dibagikan ke akun itu.
- `DELETE /api/reminders/{reminderId}/recipients/{recipientId}` — cabut; 204. 404 bila bukan penerima.
- `GET /api/reminders/{reminderId}/history` — riwayat perubahan, terbaru dahulu; 200 `{"items":[ReminderAuditEntry]}`. Hanya pemilik (AC-09).
- `GET /api/reminders/preferences` — `{"defaultShare":bool}`.
- `PUT /api/reminders/preferences` — body `{"defaultShare":bool}`; 200 `{"defaultShare":bool}`. Tidak mengubah reminder yang sudah ada (PRI-05).

`ReminderShare`:

```json
{"recipientId":"019a...","displayName":"Ahimsa","createdAt":"2026-10-28T02:05:00Z"}
```

`ReminderAuditEntry` (nilai `before`/`after` adalah JSON audit mentah):

```json
{"action":"reminder.update","actorUserId":"019a...","occurredAt":"2026-10-28T02:10:00Z","before":{...},"after":{...}}
```

Aksi audit: `reminder.create`, `reminder.update`, `reminder.complete`, `reminder.reopen`, `reminder.sharing`, `reminder.share.add`, `reminder.share.remove`, `reminder.delete`. Preferensi dicatat sebagai `reminder.preferences.set` dengan `target_type = 'user'`.

## Error

Bentuk sama dengan kontrak 1B–1C. 401 tanpa sesi. 403: akun admin organisasi, atau penerima mencoba menulis/menghapus/membaca daftar atau riwayat reminder milik orang lain. 404: reminder tidak ada atau tidak boleh dilihat pemanggil (PRI-08), atau penerima yang dicabut bukan penerima. 400: isi tidak valid (judul kosong, `dueKind` salah, `dueOn`/`dueAt` tidak sesuai `dueKind`, zona tidak dikenal, penerima bukan akun pengurus aktif, berbagi ke diri sendiri). 409: penerima duplikat.

## Di luar cakupan sesi ini

Scheduler/pengiriman reminder ke perangkat, toast Windows dan WhatsApp (PRI-06 hanya menegaskan reminder pribadi tidak dikirim ke grup WA; pengirimannya sendiri sesi lain). Tidak ada perubahan UI, backend tugas (Codex), migrasi DB dev/produksi, atau deploy.
