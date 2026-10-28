# Backend API — Notes dan template (Sesi 2B)

Status: **kontrak lebih dahulu** agar sesi UI dapat mengintegrasikan paralel sebelum backend selesai. Branch `feat/notes-backend` (dari `origin/main`, baseline integrasi `0e21591`). DTO akan ada di `internal/api/notes.go`. Memenuhi PRD §14 (NTE-01–09) dan AC-33–34.

Melengkapi kontrak [akun/Space 1B–1C](backend-api-contract.md) dan [tugas](backend-api-contract.md); aturan umum tetap berlaku (sesi Bearer sama, ID UUID, waktu RFC3339 UTC, tanggal YYYY-MM-DD, body JSON ketat, field tak dikenal ditolak, bentuk error sama, periode closed hanya-baca).

> Dokumen ini adalah kontrak yang disepakati untuk integrasi. Bila implementasi menuntut perubahan, perubahan dicatat di sini dan di handoff hasil; UI mengikuti versi terakhir dokumen.

## 1. Model dan otorisasi

Note adalah objek **ter-scope ke satu Space** (divisi atau Medhum), seperti tugas. Setiap note `spaceId` menentukan ruang dan periode-nya. Otorisasi:

- **Baca** di-gate `notes.read` lewat keanggotaan ruang (union Space seperti tugas). Note internal divisi hanya terlihat di ruang divisinya; note Medhum dibuat langsung di ruang bersama (NTE-07).
- **Tulis/kelola** (buat, ubah isi, kunci editor, hapus, template, komentar-sebagai-penulis) di-gate kapabilitas baru **`notes.manage`**, diperiksa terhadap scope **divisi pemilik** note. Untuk note Medhum, divisi pemilik adalah divisi tempat penulis berwenang; kadiv divisi lain pada ruang bersama tidak bisa mengubah note divisi lain (pola sama dengan tugas).
- **Komentar** di-gate `comments.write` (anggota pembaca boleh berkomentar walau bukan editor — NTE-06).
- **Periode tertutup**: semua tulis ditolak `409 period_closed`; baca tetap jalan; `notes.read` bertahan di periode closed (termasuk lewat grant arsip).
- Akun **admin organisasi** tidak menulis note (tidak punya kapabilitas kerja); boleh membaca untuk administrasi sesuai ruang.

`notes.manage` ditambahkan ke kapabilitas operasional. Role bawaan yang memperolehnya: `member`, `companion`, `kadiv` (anggota mencatat notulensi). Role kustom boleh memuatnya; 45 mewarisinya dari basis member, bukan dari paket Default45. Ia **bukan** kapabilitas baca — tidak bertahan di periode closed.

## 2. Isi note (NTE-01)

Isi note adalah **dokumen blok** terstruktur (`content`), bukan HTML bebas. Setiap blok punya `type` dan field sesuai jenisnya. Jenis blok rilis awal:

| `type` | Field | Keterangan |
|---|---|---|
| `heading` | `level` (1–3), `text` | Judul bagian |
| `paragraph` | `spans` | Teks dengan penekanan |
| `bulleted_list` / `numbered_list` | `items` (array `spans`) | Daftar |
| `checklist` | `items` (`{spans, checked}`) | Checklist (NTE-01) |
| `table` | `columns` (array string), `rows` (array array `spans`) | Tabel; template jadwal memakai ini (NTE-04) |
| `divider` | — | Garis pemisah |

`spans` adalah array `{ "text": "...", "bold": true|false, "link": "https://..."|null }` (teks tebal dan tautan — NTE-01). `title` note adalah field tersendiri (`title`), bukan blok.

Batas: `title` 1–200 karakter; total `content` ≤ 200 KB saat diserialisasi; ≤ 500 blok; `spans` tiap blok ≤ 4000 karakter teks gabungan; `table` ≤ 50 kolom × 500 baris. Blok/field tak dikenal ditolak `400`.

Contoh `content`:

```json
[
  {"type":"heading","level":1,"text":"Notulensi Rapat Konten"},
  {"type":"paragraph","spans":[{"text":"Hadir: ","bold":true},{"text":"Humas dan Mediatek"}]},
  {"type":"checklist","items":[{"spans":[{"text":"Kirim caption"}],"checked":false}]},
  {"type":"table","columns":["Tanggal","Jam","Kegiatan","PJ","Keterangan"],
   "rows":[[[{"text":"2026-11-20"}],[{"text":"19.00"}],[{"text":"Feed"}],[{"text":"Ahimsa"}],[{"text":"-"}]]]}
]
```

## 3. Note (DTO)

```json
{
  "id": "019a...",
  "spaceId": "019a...",
  "periodId": "019a...",
  "ownerDivisionId": "019a...",
  "title": "Notulensi Rapat Konten",
  "content": [ /* blok seperti §2 */ ],
  "templateKind": "meeting",
  "meetingId": null,
  "version": 3,
  "createdBy": {"id":"019a...","displayName":"Ahimsa"},
  "createdAt": "2026-10-28T02:00:00Z",
  "updatedAt": "2026-10-28T03:00:00Z",
  "lock": {
    "editor": {"id":"019a...","displayName":"Ahimsa"},
    "expiresAt": "2026-10-28T03:05:00Z",
    "heldByCaller": true
  },
  "attachments": [ /* §6 */ ],
  "comments": [ /* §7 */ ],
  "permissions": {"manage": true, "comment": true, "canEdit": true}
}
```

- `templateKind`: `meeting` | `schedule` | `free` | `custom` — jenis template yang dipakai saat membuat. Hanya penanda asal; mengubah template tak mengubah note (NTE-09).
- `meetingId`: dicadangkan untuk keterhubungan ke objek rapat (NTE-03); `null` sampai modul rapat (Sesi 2C) ada. Backend ini tidak membuat rapat.
- `version`: naik tiap perubahan isi/judul; dipakai untuk konflik eksplisit.
- `lock`: editor aktif saat ini, atau `null` bila tak ada. `heldByCaller` memudahkan UI. `canEdit` di `permissions` benar bila pemanggil boleh mengelola **dan** (tak ada kunci, atau kunci miliknya).

## 4. Endpoint note

- `GET /api/spaces/{spaceId}/notes` — daftar note ruang yang boleh dibaca pemanggil; `{"items":[Note]}` (tanpa `content`/`comments`/`attachments` penuh pada ringkasan — lihat catatan). Butuh `notes.read`.
- `POST /api/spaces/{spaceId}/notes` — buat note; body `NoteCreateRequest`; 201 Note. Butuh `notes.manage` pada divisi pemilik.
- `GET /api/notes/{noteId}` — satu note lengkap (isi, lampiran, komentar, kunci). 404 bila tak boleh dibaca.
- `PUT /api/notes/{noteId}` — ganti judul + isi; body `NoteWriteRequest` dengan `expectedVersion`; 200 Note. Butuh kunci editor milik pemanggil (lihat §5) dan `notes.manage`. Konflik versi → `409 conflict`.
- `DELETE /api/notes/{noteId}` — hapus note dan anaknya; 204. Butuh `notes.manage`.
- `GET /api/notes/{noteId}/history` — riwayat perubahan (AC-33/34); `{"items":[NoteEvent]}`. Butuh `notes.manage` atau pembuat.

Catatan daftar: ringkasan note pada list memuat `id, spaceId, title, templateKind, version, createdBy, updatedAt, lock, permissions`, tanpa `content` besar. `GET /api/notes/{noteId}` memuat semuanya.

`NoteCreateRequest`:

```json
{"title":"...", "templateId":"019a...|null", "templateKind":"meeting|schedule|free|null", "content":[ /* opsional; bila templateId ada, diisi dari template */ ]}
```

Satu dari `templateId` atau `content` dipakai: `templateId` menyalin isi template (copy-on-create, NTE-09); bila keduanya kosong, note mulai kosong dengan `templateKind` `free`. `templateKind` di-set dari template bila `templateId` ada.

`NoteWriteRequest`:

```json
{"title":"...", "content":[ ... ], "expectedVersion": 3}
```

## 5. Kunci editor tunggal (NTE-06, NTE-08, AC-34)

Satu editor per note pada satu waktu; pembaca/komentar tetap jalan. Kunci punya masa berlaku (TTL, bawaan 5 menit) sehingga editor terputus tidak mengunci selamanya.

- `POST /api/notes/{noteId}/lock` — ambil atau perpanjang kunci; 200 `NoteLock`. Bila kunci kosong, kedaluwarsa, atau sudah milik pemanggil → diberikan/diperpanjang. Bila dipegang orang lain yang masih berlaku → `409 conflict` dengan `NoteLock` pemegang saat ini di body error data.
- `POST /api/notes/{noteId}/lock/takeover` — ambil alih kunci yang dipegang orang lain; 200 `NoteLock`. Diizinkan hanya bila kunci **kedaluwarsa** (editor terputus), atau pemanggil punya `notes.manage` dan menegaskan pengambilalihan (`{"force": true}`). Pengambilalihan **tidak membuang draf** pemegang lama: draf server tetap tersimpan dan dapat dipulihkan (§8).
- `DELETE /api/notes/{noteId}/lock` — lepas kunci milik pemanggil; 204.

`NoteLock`:

```json
{"editor":{"id":"019a...","displayName":"Ahimsa"},"acquiredAt":"...","expiresAt":"...","heldByCaller":true}
```

`PUT /api/notes/{noteId}` menolak `409 conflict` dengan pesan jelas bila pemanggil tidak memegang kunci yang berlaku. UI: ambil kunci sebelum mengedit, perpanjang berkala (mis. tiap 60–120 dtk), lepas saat selesai.

## 6. Lampiran (NTE-01, NTE-09)

Lampiran dicatat sebagai metadata; unggah biner berkas sendiri di luar cakupan sesi ini (tanpa penyimpanan objek pada backend 2B). Lampiran yang belum terunggah bertanda `pending` (NTE-09).

- `POST /api/notes/{noteId}/attachments` — daftarkan lampiran; body `{"name":"...", "url":"https://...|", "sizeBytes": 0, "contentType":"..."}`; 201 `NoteAttachment`. Tanpa `url` → status `pending`.
- `DELETE /api/notes/{noteId}/attachments/{attachmentId}` — 204.

`NoteAttachment`:

```json
{"id":"019a...","name":"brief.pdf","url":"","status":"pending|ready","sizeBytes":0,"contentType":"application/pdf","addedBy":{...},"createdAt":"..."}
```

## 7. Komentar (NTE-06)

- `GET` termuat inline pada `GET /api/notes/{noteId}`.
- `POST /api/notes/{noteId}/comments` — body `{"body":"..."}`; 201 `NoteComment`. Butuh `comments.write`; tidak perlu kunci editor.

`NoteComment`: `{"id","author":{...},"body","createdAt"}`.

## 8. Draf dan pemulihan (NTE-08, AC-34)

Draf server adalah salinan kerja per `(note, user)` untuk pemulihan setelah terputus. Ia tidak menimpa isi resmi sampai `PUT` yang sah.

- `PUT /api/notes/{noteId}/draft` — simpan draf pemanggil; body `{"title":"...","content":[...],"baseVersion":3}`; 200 `NoteDraft`. Idempoten; menimpa draf pemanggil sebelumnya.
- `GET /api/notes/{noteId}/draft` — draf pemanggil bila ada; 200 `NoteDraft` atau 404.
- `DELETE /api/notes/{noteId}/draft` — buang draf pemanggil; 204.

`NoteDraft`: `{"title","content":[...],"baseVersion":3,"savedAt":"...","stale": true|false}`. `stale` benar bila `baseVersion` draf lebih lama dari `version` note saat ini (isi resmi sudah maju; UI tawarkan penggabungan/konflik, bukan timpa diam-diam — OFF-05 semangatnya). Pengambilalihan kunci (§5) tidak menghapus draf; editor lama dapat `GET` drafnya saat tersambung lagi.

## 9. Template (NTE-02, NTE-03, NTE-04, NTE-09)

Template membuat note baru; mengubah template tidak menyentuh note lama (copy-on-create).

- `GET /api/spaces/{spaceId}/note-templates` — template yang tersedia untuk ruang: tiga bawaan (`meeting`, `schedule`, `free`) + template tersimpan divisi ruang; `{"items":[NoteTemplate]}`. Butuh `notes.read`.
- `POST /api/spaces/{spaceId}/note-templates` — simpan template divisi; body `NoteTemplateRequest`; 201 NoteTemplate. Butuh `notes.manage`.
- `PUT /api/note-templates/{templateId}` — ubah template tersimpan; 200. Tidak mengubah note yang sudah dibuat dari versi lama (NTE-09).
- `DELETE /api/note-templates/{templateId}` — 204.

`NoteTemplate`:

```json
{"id":"019a...","spaceId":"019a...|null","kind":"meeting|schedule|free|custom","name":"Rapat Konten",
 "builtin": true, "content":[ /* blok §2 */ ], "createdAt":"...","updatedAt":"..."}
```

- Bawaan (`builtin:true`, `spaceId:null`): **meeting** (agenda, pembahasan, keputusan, tindak lanjut — NTE-03); **schedule** (tabel tanggal/jam/kegiatan/PJ/keterangan — NTE-04); **free** (kosong).
- Tersimpan (`builtin:false`): `kind:"custom"`, milik divisi ruang.

`NoteTemplateRequest`: `{"name":"...","content":[ ... ]}`.

## 10. Jadikan tugas / reminder dari note (NTE-04, NTE-05, NTE-07)

Konversi **memerlukan konfirmasi pengguna**, tanpa AI dan tanpa menebak isi. Backend tidak mengubah kontrak tugas/reminder yang ada.

Alur yang disepakati:

1. UI memilih teks/baris/tindak lanjut, lalu membuka form “Jadikan tugas”/“Jadikan reminder” yang **diisi manusia** (hasil diminta, PJ, tenggat untuk tugas; judul/waktu untuk reminder). Backend tidak mengekstrak otomatis.
2. UI memanggil endpoint yang sudah ada: `POST /api/spaces/{spaceId}/tasks` (tugas) atau `POST /api/reminders` (reminder pribadi). Kontrak keduanya tidak berubah.
3. Agar asal terlacak tanpa membagikan seluruh note, UI lalu mencatat tautan melalui:
   `POST /api/notes/{noteId}/links` — body `{"targetType":"task|reminder","targetId":"019a..."}`; 201 `NoteLink`. Butuh `notes.manage` pada note **dan** pemanggil adalah pembuat/berwenang objek tujuan. Tautan hanyalah rujukan; membuat tugas dari note **tidak** membuka atau membagikan isi note (NTE-07).
4. `GET /api/notes/{noteId}/links` — daftar tautan note (butuh `notes.read` pada note). `DELETE /api/notes/{noteId}/links/{linkId}` — 204 (butuh `notes.manage`).

`NoteLink`: `{"id":"019a...","targetType":"task|reminder","targetId":"019a...","createdBy":{...},"createdAt":"..."}`.

> Keterhubungan ke objek rapat (NTE-03, MTG) memakai `meetingId` pada note, diisi saat modul rapat (Sesi 2C) tersedia; backend 2B menyimpan field tetapi tidak membuat rapat.

## 11. Error

Bentuk sama. 401 tanpa sesi. 403: tanpa `notes.read`/`notes.manage`/`comments.write` pada scope, atau admin mencoba menulis. 404: note/template/lampiran/tautan tak ada atau tak boleh dibaca (privasi ruang, NTE-07). 400: isi/blok tidak valid, template dan content sekaligus, dsb. 409 `conflict`: versi usang (`expectedVersion` ≠ `version`), atau kunci dipegang editor lain yang masih berlaku (data error memuat `NoteLock`). 409 `period_closed`: tulis pada periode tertutup. 429/503 seperti kontrak umum.

## 12. Di luar cakupan 2B

Penyimpanan biner lampiran, editor kolaboratif simultan (NTE-06 menyatakan bukan syarat awal), modul rapat/kehadiran (Sesi 2C), scheduler/pengiriman, WhatsApp, offline sync perangkat (OFF-xx di sisi klien), AI. UI (`app/`, `internal/appui`, `internal/client`), backend tugas/reminder yang ada (kontraknya tidak diubah), produksi/Reo/spreadsheet tidak disentuh.
