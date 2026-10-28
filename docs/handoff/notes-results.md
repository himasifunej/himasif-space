# Hasil backend Notes dan template (Sesi 2B)

Branch `feat/notes-backend`, dari `origin/main` (baseline integrasi `0e21591`). Kontrak: [notes-api-contract.md](notes-api-contract.md); DTO di `internal/api/notes.go`. Memenuhi PRD §14 (NTE-01–09) dan AC-33–34.

## Yang dibuat

- **Migrasi** `internal/store/migrations/0005_notes.sql` (baru): `notes`, `note_locks`, `note_drafts`, `note_comments`, `note_attachments`, `note_links`, `note_events` (append-only), `note_templates`. Tidak mengubah 0001–0004.
- **Kapabilitas baru** `notes.manage` (`internal/api/access.go`, `internal/authz/authz.go`): kapabilitas operasional (bukan baca), dimiliki member/companion/kadiv, boleh di role kustom, diwarisi 45 dari basis member; tidak bertahan di periode tertutup. `notes.read` tetap kapabilitas baca.
- **DTO** `internal/api/notes.go` + validator terbagi `internal/api/notes_validate.go` (jenis blok, ukuran, tautan).
- **Store** `internal/store/notes.go`: CRUD note, kunci editor, draf, komentar, lampiran, tautan, riwayat, dan template tersimpan.
- **Server** `internal/server/notes.go` + `notes_extra.go` + rute di `server.go` (`/api/spaces/{spaceId}/notes`, `/api/notes/{noteId}/...`, `/api/spaces/{spaceId}/note-templates`, `/api/note-templates/{templateId}`).
- **Kontrak ditulis lebih dahulu** dan di-commit sebelum implementasi, agar sesi UI bisa integrasi paralel.

## Keputusan teknis

- **Isi note = dokumen blok** (`[]NoteBlock`) disimpan `jsonb`, divalidasi server (jenis/ukuran/tautan) dan oleh validator yang sama yang bisa dipakai klien. Mendukung heading, paragraph (bold + link), daftar, checklist, tabel, divider (NTE-01).
- **Scope seperti tugas**: `space_id` + `owner_division_id` (FK komposit ke `period_divisions`). Baca di-gate `notes.read` lewat union Space (`spaceView`). Kelola di-gate `notes.manage` pada **divisi pemilik**, diperiksa ulang lewat `loadScope` — jadi pada Medhum, kadiv Humas tidak bisa mengubah note milik Mediatek (diuji `TestNoteMedhumOwnerDivision`). Divisi pemilik untuk note baru dipilih dari divisi ruang tempat pemanggil punya `notes.manage`.
- **Versi/konflik eksplisit** (AC-34): `version` naik tiap simpan; `PUT` butuh `expectedVersion` cocok di bawah kunci baris `for update`, kalau tidak `409 conflict` — tidak ada timpa diam-diam.
- **Satu editor aktif** (NTE-06): tabel `note_locks` satu baris per note, TTL 5 menit. `POST /lock` ambil/perpanjang; `409` bila dipegang orang lain yang masih hidup (body error menyebut pemegang). Pembaca/komentar tetap jalan. `PUT` menolak bila pemanggil tak memegang kunci berlaku.
- **Pengambilalihan & pemulihan** (NTE-08): `POST /lock/takeover` diizinkan saat kunci kedaluwarsa, atau `force:true` oleh pemegang `notes.manage`. Pengambilalihan **tidak menghapus draf**: `note_drafts` per (note, user) bertahan dan dapat dibaca editor lama saat tersambung lagi (diuji `TestNoteLockExpiryAndTakeover`). Draf menandai `stale` bila `baseVersion` lebih lama dari versi note.
- **Template copy-on-create** (NTE-02/03/04/09): tiga template bawaan (`meeting` dengan agenda/pembahasan/keputusan/tindak-lanjut, `schedule` tabel tanggal/jam/kegiatan/PJ/keterangan, `free`) disuplai server tanpa baris. Template tersimpan milik divisi. Membuat note **menyalin** isi template; mengubah template **tidak** mengubah note lama (diuji `TestNoteTemplatesCopyOnCreate`).
- **Jadikan tugas/reminder** (NTE-05/07): tanpa AI, tanpa menebak. UI mengisi form manusia lalu memanggil endpoint tugas/reminder yang sudah ada (**kontraknya tidak diubah**), kemudian mencatat `POST /api/notes/{noteId}/links`. Tautan hanyalah rujukan — tidak membuka isi note. Tautan reminder wajib milik pemanggil, sehingga note tak membocorkan reminder privat orang lain (PRI-08). `meeting_id` dicadangkan untuk modul rapat (Sesi 2C), disimpan tapi tidak membuat rapat.
- **Riwayat** (AC-33/34): tiap perubahan dicatat di `note_events` (append-only, trigger sama) **dan** `audit_events`, dalam transaksi yang sama. Dibaca pengelola/pembuat lewat `GET /history`.
- **Periode tertutup**: semua tulis `409 period_closed`; baca tetap jalan (termasuk arsip, `notes.read` bertahan). Akun admin organisasi tidak menulis note.

## Koordinasi dengan backend tugas/reminder

Tidak ada perubahan pada kontrak/skema tugas maupun reminder. Tautan note memakai pembacaan non-invasif: `TaskExists` (tugas ada) dan `ReminderOwnedBy` (reminder milik pemanggil) di `internal/store/notes.go`. Tabel `note_links.target_id` sengaja bukan foreign key karena menunjuk salah satu dari dua tabel; keberadaan/izin diperiksa di server.

## Verifikasi yang dijalankan

`make fmt vet` (linux, windows, js/wasm) dan `make test-db` terhadap PostgreSQL dev (`SPACE_TEST_DATABASE_URL`, database sementara per test): **seluruh test lulus, tidak ada yang skipped**.

Test baru:
- `internal/server/notes_test.go` (7 test, PostgreSQL nyata):
  - `TestNoteCreateAndRead` — NTE-01/06/07: blok kaya, scope ruang, divisi lain 403, blok tak dikenal 400, admin 403.
  - `TestNoteSingleEditorAndVersionConflict` — AC-34: satu editor, pembaca/komentar jalan, konflik versi eksplisit.
  - `TestNoteLockExpiryAndTakeover` — NTE-08: kunci kedaluwarsa → takeover; live → butuh force; draf selamat.
  - `TestNoteTemplatesCopyOnCreate` — AC-33/NTE-09: bawaan tersedia, note dari template tidak berubah saat template diubah.
  - `TestNoteMedhumOwnerDivision` — NTE-07: note Medhum hanya dikelola divisi pemilik; divisi lain baca saja.
  - `TestNoteLinksToTaskAndReminder` — NTE-05/07: tautan tugas/reminder, reminder wajib milik sendiri.
  - `TestNoteHistoryAndDraftStale` — AC-33/34, NTE-08.
- `internal/api/notes_validate_test.go` — validator dokumen blok (unit, tanpa DB).

## Belum / batas

- **Penyimpanan biner lampiran** di luar cakupan: lampiran hanya metadata, tanpa `url` → `pending` (NTE-09).
- **Editor kolaboratif simultan** bukan syarat awal (NTE-06); modul ini satu editor + kunci.
- **Modul rapat/kehadiran** (Sesi 2C): `meeting_id` disiapkan, belum ada pembuatan rapat.
- **UI** (`app/`, `internal/appui`, `internal/client`) belum memakai endpoint ini; perlu editor Notes native + pemilih template + indikator kunci/konflik + alur Jadikan tugas/reminder (sesi UI).
- **Tidak ada** migrasi DB dev/produksi, email, deploy, WhatsApp, Reo, atau spreadsheet.
- Offline sync perangkat (OFF-xx) adalah pekerjaan klien; server menyediakan versi + draf + konflik eksplisit sebagai fondasinya.

## Catatan push

Branch `feat/notes-backend` di-commit lokal; push ke origin sempat gagal saat vault enowX terkunci (git butuh kredensial GitHub). Setelah vault terbuka, branch dan PR dapat didorong.
