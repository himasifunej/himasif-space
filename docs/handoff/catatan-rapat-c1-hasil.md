# Catatan rapat A, tahap C1: blok baru di server dan klien (hasil)

Rencana: `docs/handoff/anggota-catatan-rapat-rencana.md` (branch `design/anggota-dan-catatan-rapat`, 5c5dc9f), bagian 3 dan tahap C1. Belum ada UI penyuntingan baru dan belum ada migrasi.

## Yang dibuat
- `internal/api/notes.go`
  - Tipe blok baru `callout` (`variant` `info`/`decision`) dan `toggle` (`text` dan `children`).
  - `Span.Mention` berisi `{kind user|task, id}`.
  - Butir checklist punya `assigneeId`, `dueOn` dan `taskId`.
  - `minutes` ada di `heading` dan di butir `numbered_list`.
  - `NoteBlock.Raw`: blok yang jenisnya tidak dikenal build ini disimpan apa adanya dan ditulis kembali byte demi byte. Ini juga berlaku untuk anak toggle. `NoteBlock.Unsupported()` menandai blok seperti itu.
- `internal/api/notes_validate.go`
  - `ValidateNoteContent` tetap ketat dan dipakai server. Aturan barunya:
    - variant callout wajib;
    - toggle tidak boleh bersarang;
    - anak toggle dihitung ke batas 500 blok;
    - mention wajib punya id, teks nama, dan tanpa tautan;
    - field action item hanya untuk checklist;
    - minutes hanya untuk heading atau numbered list, 0–480;
    - field yang tidak sesuai jenis bloknya ditolak.
  - `ValidateNoteContentPreserving` dipakai klien. Bedanya hanya satu: blok `Raw` dari server diterima, tapi tetap dihitung ke batas jumlah blok dan ukuran.
- Klien, di `internal/appui/notes_controller.go`, `offline.go` dan `app/notes_drafts.go`, sekarang memakai `ValidateNoteContentPreserving`.
- `internal/appui/notes_document.go`
  - Salinan kerja menyalin dalam (deep copy) mention, field action item, anak toggle dan `Raw`.
  - Input inline tidak lagi menimpa span yang berisi mention (`notePlainSpans`).
  - Callout dan toggle tampil sebagai blok baca.
  - Jenis yang tidak dikenal tampil sebagai "Blok tidak didukung", dan isinya tetap tersimpan.
- Kontrak API (`notes-api-contract.md` §2) sudah diperbarui.

## Test
- `internal/api/notes_blocks_test.go`: validasi tiap tipe/field baru, round-trip JSON (dokumen lama tidak berubah bentuk), dan blok tak dikenal (disimpan dan ditulis kembali utuh; diterima klien, ditolak server; tetap dihitung ke batas).
- `internal/appui/notes_blocks_c1_test.go`: editor menampilkan "Blok tidak didukung", Keputusan dan toggle; `Content()` sama byte-demi-byte dengan dokumen dari server; tidak ada aliasing; ketik inline tidak menghapus field action item; mention tidak dianggap teks polos.
- `app/notes_drafts_desktop_test.go`:
  - Fixture "isi tidak valid" kini memakai heading level 9. Draf yang berisi blok dari server lebih baru bukan draf rusak.
  - Test baru memastikan draf berisi blok `Raw` tetap terbaca utuh.
- `internal/server/notes_blocks_test.go` (DB): create, read dan save dengan blok C1 mengembalikan JSON yang sama; jenis tak dikenal, toggle bersarang, mention kosong, PJ di daftar biasa dan variant salah ditolak `400`.

## Kompatibilitas dan urutan deploy
- Klien 0.3.1 yang sekarang live **belum** punya `Raw` dan validasi preserving. Kalau sebuah catatan berisi callout/toggle/mention, klien itu akan:
  - membuang field baru saat membaca;
  - menolak menyimpan bila ada jenis blok baru.
- Karena itu, UI yang membuat blok baru (C3/C4) baru boleh dirilis setelah klien dengan C1 terpasang di semua perangkat (web langsung; desktop lewat pembaruan).
- Server C1 aman dideploy lebih dulu: dokumen lama tidak berubah bentuk, dan tidak ada migrasi.
- Field tak dikenal di dalam blok yang **dikenal** (misalnya field baru di butir checklist pada versi berikutnya) tetap hilang di klien lama. Bila perlu, pola `Raw` bisa diperluas ke field sisa di tahap berikut.
