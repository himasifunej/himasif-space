# T2 timer bersama, C7 ringkasan ke WhatsApp, A4 panel anggota, info build

Status: **selesai** (10 Nov 2026), rilis 0.4.3. Rencana: `anggota-catatan-rapat-rencana.md` §4 (T2, C7, A4).

## T2 — Timer bersama
- Status timer disimpan di server pada baris `meetings`, bukan di isi catatan. Migrasi `0032_meeting_timer.sql` menambah kolom `timer jsonb` dan `timer_version`.
  - `timer_version` terpisah dari `version` rapat. Jadi menjalankan timer tidak pernah bentrok dengan menyunting rapat.
  - Trigger realtime `meetings` yang sudah ada mengabari semua klien.
- Endpoint `POST /api/meetings/{id}/timer {action, expectedVersion}`, dengan aksi `start`, `pause`, `resume`, `next`, `extend` dan `stop`.
  - Stempel waktu memakai jam server.
  - Versi yang usang ditolak dengan 409 "Timer baru saja diubah oleh X".
  - Aksi yang tidak cocok dengan keadaan timer ditolak dengan 409. Contohnya jeda dua kali, atau mulai saat timer sudah berjalan.
  - Setiap aksi dicatat di audit `meeting.timer.<aksi>`.
- Logika timer ada di satu fungsi murni, `api.ApplyTimer`, yang dipakai server dan fake test.
- **Siapa yang boleh mengatur**: pengelola rapat (`CanManage`), atau orang yang boleh menyunting catatan rapat itu (`notes.manage` di divisi pemilik catatan).
  - Flag `Meeting.CanRunTimer` memberi tahu klien siapa yang boleh mengatur.
  - Pembaca lain hanya melihat timer, dengan keterangan "Diatur oleh X".
- **Klien** (`notes_timer.go`):
  - `noteTimerCtl` memilih timer bersama untuk catatan yang punya rapat. Catatan tanpa rapat tetap memakai timer lokal T1, dengan teks "Timer hanya di perangkat ini".
  - Waktu dihitung dari stempel server. Selisih jam perangkat dikoreksi lewat `MeetingTimer.ServerNow`.
  - Bila aksi ditolak, pesannya tampil dan daftar rapat dimuat ulang.
  - "Sembunyikan" hanya berlaku di perangkat itu.

## C7 — Ringkasan ke WhatsApp
- Bagian **Ringkasan ke WhatsApp** ada di tab Rapat pada panel catatan, untuk penyunting catatan. Tombol "Kirim ringkasan ke WhatsApp" membuka dialog pratinjau, lalu "Kirim ke {grup}" mengirimnya.
- Endpoint `POST /api/notes/{id}/wa-summary {send, expectedVersion}`:
  - `send:false` menghasilkan pratinjau.
  - `send:true` hanya diterima bila versi catatan masih sama dengan pratinjau.
  - Pengiriman ulang dalam 1 menit ditolak.
- **Isi pesan**: judul, hari/tanggal dan tempat rapat, *Keputusan* (callout Keputusan, termasuk di dalam toggle), dan *Action item*.
  - Action item adalah butir checklist yang punya PJ, tenggat atau tugas, dalam format "• teks — PJ: Nama · tenggat 12 Nov". Butir yang sudah selesai diberi ✓.
  - Pesan diakhiri tautan "Catatan lengkap".
  - Teksnya polos, tanpa nama bot atau sapaan.
- **Pengiriman**:
  - Pesan dikirim lewat bot divisi pemilik catatan ke grup divisinya, melalui `wa_outbox` dengan `source_kind='note_summary'`.
  - Worker baru mengirim selama pengirimnya masih bisa membaca Space.
  - Bila divisi belum punya grup bot, dialog menjelaskannya dan tombol Kirim tidak tampil.
  - Bila bot sedang terputus, ada peringatan bahwa pesan terkirim begitu bot tersambung lagi.
- **Tautan**: {{Asumsi}} untuk sementara mengarah ke `SPACE_PUBLIC_URL/#/spaces` (fungsi `noteSummaryLink`), karena Space belum punya deep link per catatan.
  - Setelah C6 (tautan publik) masuk, fungsi ini memakai tautan publik bila diaktifkan.
- Riwayat catatan mencatat "Ringkasan dikirim ke grup WhatsApp".

## A4 — Panel bawah Anggota
- Setiap baris anggota punya menu **⋯**:
  - **Lihat tugas** membuka board yang tersaring ke anggota itu.
  - **Keluarkan…** membuka dialog keluarkan A3, lengkap dengan pemeriksaan tugas aktif. Menu ini hanya tampil untuk yang berhak mengelola anggota.
- **Panel bawah**:
  - Di layar lebar (≥ 760 px), "Undangan tertunda" (kirim ulang, cabut, status bounce SES) tampil berdampingan dengan "Kewenangan role" versi ringkas per role.
  - Di HP keduanya bertumpuk.

## Status layanan: revisi dan hash
- "Revisi sumber server" dan "Hash snapshot server" kini diisi saat build lewat ldflags, bersama `BuiltAt`.
  - **Makefile**: revisi = commit, ditambah `-dirty` bila ada perubahan lokal. Hash = SHA-256 dari daftar SHA-256 per berkas yang dilacak git, diurutkan per path.
  - **Skrip pilot** `E:\himasif-cache\deploy\build-local.sh`: revisi = SHA yang dirilis. Hash dihitung dengan cara yang sama atas sumber hasil `git archive`, dan disimpan juga di `SOURCE_DIGEST`.

## Test
- `api.TestApplyTimer`: urutan keadaan timer, jeda, lewat tengah malam, aksi yang ditolak.
- `server.TestMeetingTimerShared` (DB):
  - anggota menyunting catatan dan kadiv mengelola rapat, keduanya mengatur timer yang sama;
  - versi usang dan aksi yang tidak cocok ditolak 409;
  - daftar rapat ikut membawa timer, dan versi rapat tidak berubah;
  - divisi lain mendapat 404;
  - pada rapat tanpa catatan, hanya pengelola yang boleh mengatur timer.
- `server.TestNoteWASummary` (DB): tanpa grup, isi pesan, peer mendapat 404, versi usang, isi outbox, kirim dobel.
- appui:
  - `TestNoteSharedTimer`: hanya lihat, mengatur, penolakan, sembunyikan;
  - `TestNoteTimerLocalWithoutMeeting`;
  - `TestNoteWASummaryDialog`;
  - `TestMemberRowMenuAndBottomPanel`.
- Test T1 lama (`TestNoteTimerPanelAndPill`) kini memakai catatan tanpa rapat.
