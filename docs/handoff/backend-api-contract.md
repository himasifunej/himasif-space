# Backend API — Sesi 1B–1C

Status: **diimplementasikan** di branch `feat/accounts-spaces-access` (dari `627298f`) dan diuji terhadap PostgreSQL; lihat [hasil](backend-1b-1c-results.md). DTO di `internal/api/access.go`. UI tetap MyGo native (desktop/WASM).

## Kompatibilitas dan aturan umum

`GET /api/health`, `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/me`, `GET /api/periods`, `GET /api/divisions` mempertahankan bentuk Sesi 1A. Daftar periods/divisions adalah metadata, bukan bukti akses. Bearer session yang sama berlaku. Semua ID adalah UUID; nama/label tidak menjadi identitas atau dasar otorisasi. List berbentuk `{"items":[]}`; waktu RFC3339 UTC, tanggal YYYY-MM-DD. Body JSON ketat, field tak dikenal ditolak.

Setiap request memeriksa akun/sesi dan scope periode/divisi kembali. Perubahan dan audit satu transaksi. Periode closed hanya-baca; `readOnly:true` berarti kemampuan tulis tidak efektif. Admin mempunyai akses administrasi, tidak akses privat otomatis. API ini tidak menyediakan notes/reminder pribadi; izin Space/arsip tidak boleh dipakai untuk membuka objek privat pada sesi berikutnya.

## Endpoint untuk navigasi UI

- `GET /api/periods/{periodId}/spaces`: hanya Space yang benar-benar boleh diakses pemanggil. `GET /api/spaces/{spaceId}` memakai aturan yang sama; Space terlarang menjawab 403.
- `GET /api/periods/{periodId}/divisions/{divisionId}/access`: role, kemampuan efektif, status, read-only untuk scope divisi tersebut.
- `GET /api/periods/{periodId}/divisions/{divisionId}/members`: anggota aktif yang boleh dibaca anggota divisi/admin. Arsip Space saja tidak membuka direktori divisi lain.
- `GET /api/periods/{periodId}/divisions/{divisionId}/invitations`: hanya kadiv asli/admin; tidak untuk pendamping/45.
- `GET /api/periods/{periodId}/divisions/{divisionId}/roles`: role kustom divisi; anggota boleh membaca.

Contoh `GET .../spaces`:

```json
{"items":[{"id":"019a2810-0000-7000-8000-000000000001","periodId":"019a2810-0000-7000-8000-000000000002","code":"medhum","name":"Medhum","kind":"shared","divisions":[{"id":"019a2810-0000-7000-8000-000000000003","slug":"humas","name":"Humas"},{"id":"019a2810-0000-7000-8000-000000000004","slug":"mediatek","name":"Mediatek"}],"periodStatus":"active","readOnly":false,"access":"member","roles":[{"divisionId":"019a2810-0000-7000-8000-000000000004","roles":["member"],"customRoleIds":[],"capabilities":["space.read","members.read","tasks.own","comments.write","help.request","notes.read"]}],"capabilities":["space.read","members.read","tasks.own","comments.write","help.request","notes.read"]}]}
```

`kind`: division/shared. `code` stabil dalam periode (`division-<slug>`, `medhum`). `divisions` berisi identitas divisi terkait, bukan pemberian akses ke seluruh ruang internalnya. `access`: member/archive/admin. `roles` berisi binding hanya divisi yang memang berwenang bagi pengguna. Kemampuan gabungan Space shared hanya operasional; pengelolaan akses tetap memakai endpoint divisi dan binding divisi yang tepat. Backend Sesi 2A harus tetap memeriksa divisi pemilik operasi (misalnya PJ desain oleh Mediatek).

Role bawaan: `member`, `companion`, `kadiv`, `45`, `organization_admin`. `kadiv` berasal dari penetapan admin; `45` berasal dari appointment khusus. Role kustom tidak dapat berisi kemampuan administrasi. Pengelompokan: `tekno`, `desain`, `video`, hanya Mediatek; tidak memberi kemampuan.

Kemampuan operasional: `space.read`, `members.read`, `tasks.own`, `comments.write`, `help.request`, `notes.read`, `tasks.manage`, `results.review`, `ping.send`, `whatsapp.manage`. Kemampuan administrasi: `invitations.manage`, `members.manage`, `roles.manage`, `archives.manage`, `acting45.manage`, `periods.manage`, `kadiv.manage`. Pendamping mendapat tasks.manage/results.review/ping.send, tanpa whatsapp.manage secara default. 45 mendapat operasional termasuk WhatsApp; larangan administrasi diterapkan setelah penggabungan semua role. Admin tidak mendapat acting45.manage.

## Akun dan undangan

- `POST /api/periods/{p}/divisions/{d}/invitations` — kadiv asli/admin; body `{"email":"anggota@example.test"}`; 201 Invitation. Selalu anggota biasa; penetapan jabatan terpisah. Email dikirim melalui Mailer; token tidak dikembalikan di API/list/audit/log.
- `POST .../invitations/{invitationId}/resend` — rotasi token dan masa berlaku 7 hari dari pengiriman ulang, membatalkan bukti verifikasi sebelumnya; 200 Invitation.
- `DELETE .../invitations/{invitationId}` — pencabutan; 204.
- `POST /api/auth/invitations/inspect` — publik `{"token":"..."}`; 200 `{"email","periodId","periodLabel","divisionId","divisionName","expiresAt","accountExists"}`. `accountExists` memilih alur: masuk lalu terima, atau verifikasi email lalu buat akun.
- `POST /api/auth/invitations/verify` — publik, `{"token":"<invitation-token>"}`; 202. Mengirim bukti email terpisah (1 jam) ke alamat undangan; tautannya `/#/undangan?token=…&verification=…`. 409 bila email sudah punya akun. Link undangan yang dibagikan WhatsApp saja tidak cukup untuk membuktikan email.
- `POST /api/auth/invitations/accept` — baru: `{"token":"...","verificationToken":"...","displayName":"Nama","password":"minimal-10-karakter"}`; lama: bearer login dari akun personal dengan email undangan yang sudah terverifikasi, body `{"token":"..."}`. 200 `{"user":{...},"membership":{...}}`. Akun baru kemudian login memakai kontrak lama. Satu penerimaan, binding email/divisi/periode diperiksa kembali secara atomik. Akun admin organisasi tidak bisa menerima keanggotaan personal.
- `POST /api/auth/recovery` — publik `{"email":"..."}`; 202 identik untuk alamat ada/tidak ada. Token sekali pakai 1 jam dikirim hanya lewat Mailer.
- `POST /api/auth/recovery/reset` — `{"token":"...","password":"kata-sandi-baru"}`; 204, semua sesi dan token pemulihan sebelumnya dicabut. Tidak mengaktifkan akun disabled.

Invitation:

```json
{"id":"019a2810-0000-7000-8000-000000000005","periodId":"019a2810-0000-7000-8000-000000000002","divisionId":"019a2810-0000-7000-8000-000000000004","email":"anggota@example.test","status":"pending","expiresAt":"2026-11-03T00:00:00Z","createdAt":"2026-10-27T00:00:00Z"}
```

Status: pending/expired/revoked/accepted. Tidak ada raw token. Kegagalan pengiriman mengembalikan 503 dan transaksi dibatalkan; delivery SMTP tidak atomik dengan DB, maka email yang terkirim tepat sebelum rollback mungkin berisi tautan invalid, dapat diatasi dengan resend. Tidak ada pengiriman background atau SES otomatis saat testing.

## Periode, keanggotaan, role dan 45

- `POST /api/periods` — admin; `{"label":"2026/2027","startsOn":"2026-12-01","endsOn":"2027-11-30","timeZone":"Asia/Jakarta","divisionIds":["<uuid>"]}`; 201 Period. Mulai preparing, membuat Space divisi dan Medhum bila Humas+Mediatek ada; tidak menyalin orang/role/tugas. Array divisionIds kosong memakai seluruh struktur divisi global.
- `POST /api/periods/{p}/activate` — admin; 200 Period. Preparing → active; aktivasi tidak menutup periode lain.
- `POST /api/periods/{p}/close` — admin; 200 Period, penutupan terminal. Semua operasi tulis pada scope periode tersebut ditolak kemudian.
- `PUT /api/periods/{p}/divisions/{d}/kadiv` — hanya admin; `{"userId":"<akun-personal-terverifikasi>"}`; 200 Member. Membuat keanggotaan bila belum ada. Penggantian tidak mewariskan objek privat. Tolak jika 45 masih aktif; kadiv asli harus mencopotnya dahulu.
- `PATCH .../members/{userId}` — kadiv asli/admin; `{"role":"member|companion","groups":["desain","video"],"customRoleIds":["<uuid>"]}`; 200 Member. Seluruh field wajib, mengganti konfigurasi operasional anggota; tidak mengangkat kadiv/45.
- `DELETE .../members/{userId}` — kadiv asli/admin; 204. Tidak bisa menghapus kadiv atau 45 aktif; copot jabatan dahulu.
- `POST .../roles` — kadiv asli/admin; `{"name":"Koordinator","capabilities":["tasks.manage","ping.send"]}`; 201 CustomRole.
- `PUT .../roles/{roleId}` — kadiv asli/admin, body sama; 200 CustomRole. `DELETE` endpoint sama → 204, assignment dihapus.
- `PUT .../45` — hanya kadiv asli; `{"userId":"<anggota-aktif>","capabilities":["tasks.manage","results.review","ping.send","whatsapp.manage"]}`. capabilities opsional: default semua kemampuan operasional kadiv. Array eksplisit boleh mempersempit, hanya tasks.manage/results.review/ping.send/whatsapp.manage. 201 appointment. Maksimal satu aktif per divisi/periode; panggilan kedua 409, termasuk bersamaan. Tidak ada expiry.
- `DELETE .../45` — hanya kadiv asli, 204. Keanggotaan dan role lain tetap ada, hak tambahan langsung hilang pada request berikutnya.

Member mencakup `id, userId, periodId, divisionId, displayName, role, roles, groups, customRoleIds, capabilities`. Email anggota tidak diumbar di daftar biasa; email undangan hanya terlihat pengelolanya.

## Arsip dan serah terima

- `GET /api/periods/{p}/divisions/{d}/archives` — kadiv asli/admin periode pemberi; daftar grant yang diberikan dari scope itu.
- `POST .../archives` — kadiv asli periode aktif terkait/admin; `{"userId":"<anggota-scope-aktif>","spaceId":"<space-periode-closed>"}`; 201 ArchiveGrant. Space asal harus memuat divisi yang dipimpin pemberi (Medhum bisa dari Humas atau Mediatek). Grant untuk satu Space saja. Tidak berlaku pada direktori divisi atau objek privat. Pengurus lama tetap read-only pada Space dari keanggotaan lamanya.
- `DELETE .../archives/{grantId}` — pemberi scope sama, 204; pencabutan tidak menghapus hak anggota lama yang memang independen.
- `GET /api/periods/{p}/handover` — admin; 200 Handover (`updatedAt:null` bila belum diisi). `PUT` body `{"successorPeriodId":"<uuid>|null","incomingLeadership":"...","adminCustodian":"...","botManagement":"...","archivePlan":"...","remainingWork":"..."}`; `remainingWork` wajib (pekerjaan tersisa atau alasan tidak ada); 200 Handover. Catatan/checklist, bukan auto-copy tugas. Hanya sebelum closed.
- `POST /api/auth/admin-email` — admin, `{"currentPassword":"...","email":"baru@..."}`; 202, mengirim bukti 1 jam ke email baru (`/#/organisasi/serah-terima-admin?verification=…`).
- `POST /api/auth/admin-handover` — admin, `{"currentPassword":"...","password":"baru","email":"baru@... (opsional)","verificationToken":"... (wajib bila email)","custodian":"nama pengelola baru"}`; 204. Semua sesi akun admin berakhir, termasuk sesi ini; token pemulihan lama dicabut. Audit `admin.handover` mencatat akun admin bersama dan pengelola baru.

## Tambahan akun

- `POST /api/auth/password` — sesi; `{"currentPassword","password"}`; 204, sesi lain berakhir.
- `GET /api/users` — admin; akun pribadi (untuk memilih kadiv).
- `POST /api/users/{userId}/disable` | `/enable` — admin; 200 User. Disable mengakhiri semua sesi; keanggotaan tetap. Pemulihan tidak mengirim email ke akun disabled.

## Batas dan umur

Undangan 7 hari dari tiap pengiriman; bukti email/pemulihan 1 jam; maksimal 5 email per alamat dan 30 request auth publik per IP per 15 menit. Tautan email memakai fragment URL sehingga token tidak masuk log server.

## Error

Bentuk tetap: `{"error":{"code":"forbidden","message":"Anda tidak berwenang pada scope ini."}}`.

401 unauthenticated: sesi tidak ada/expired/revoked. 403 forbidden: lintas divisi, akun berbeda saat accept, atau 45 mencoba akses administrasi. 404 not_found: ID tidak ditemukan pada scope endpoint. 409 conflict: dua 45, email/undangan pending duplikat, atau penetapan kadiv ketika 45 aktif. 409 period_closed: penulisan periode closed. 410 invalid_token: kedaluwarsa/dicabut/digunakan atau token invalid; pesan seragam. 400 bad_request: input/kemampuan tak valid. 429 rate_limited: batas permintaan email/auth publik. 503 unavailable: Mailer gagal.
