# Rencana Tahap 6 canvas A: nomor tugas #T dan lampiran berkas tugas (9 November 2026)

Status: **rencana, menunggu persetujuan lewat GOA**. Belum ada kode backend, migrasi, push, atau deploy untuk tahap ini. Tahap 5 sudah live (a74074b, migrasi sampai `0025`).

## Ringkasan
| # | Pekerjaan | Backend | Migrasi DB |
|---|---|---|---|
| 6a | Nomor tugas urut `#T12` per Space (kartu, detail, pencarian) | kolom + penghitung + perubahan cara menyimpan tautan Space | `0026` (kolom, tabel penghitung, backfill) |
| 6b | Lampiran berkas di tugas (unggah, unduh, hapus; hitungan di kartu) | 4 endpoint, memakai filestore catatan (`0014`) | `0027` (tabel lampiran + tanda terima unggah) |

Keduanya bisa dikerjakan dan dideploy terpisah; 6a punya risiko backfill, 6b hanya menambah tabel.

## 6a. Nomor tugas urut per Space

### Fakta yang menentukan desain
- Satu tugas bisa tampil di **beberapa Space** (`task_spaces`, mis. Space divisi + Medhum). Jadi "nomor per Space" berarti satu tugas bisa punya nomor berbeda di tiap Space: `#T12` di Mediatek, `#T3` di Medhum. Kartu dan detail menampilkan nomor **Space yang sedang dibuka**.
- Sekarang kartu/detail memakai `taskRef(id)` = 4 karakter terakhir UUID (`internal/appui/tasks_controller.go:651`).
- **Risiko kode**: `store.TaskSpaces` (`internal/store/tasks.go:180`) menghapus semua baris `task_spaces` lalu menyisipkan ulang di setiap simpan tugas. Kalau nomor disimpan di `task_spaces` tanpa mengubah ini, nomor akan berubah tiap kali tugas disunting. Bagian ini harus diubah menjadi selisih (hapus yang dilepas, tambah yang baru, biarkan yang tetap).

### Desain yang diusulkan
- Kolom `task_spaces.number bigint` + `unique(space_id, number)`.
- Tabel penghitung `space_task_counters(space_id uuid primary key references spaces(id), last_number bigint not null)`. Nomor baru diambil dengan satu pernyataan atomik di transaksi yang sama dengan penyisipan:
  `insert into space_task_counters(space_id,last_number) values($1,1) on conflict(space_id) do update set last_number=space_task_counters.last_number+1 returning last_number`
  Kunci baris per Space; dua tugas baru di Space yang sama berurutan, Space lain tidak saling menunggu. Tidak memakai `sequence` Postgres (satu sequence per Space sulit dikelola dan tidak ikut transaksi saat rollback).
- Aturan nomor: tidak pernah dipakai ulang; tugas yang dilepas dari Space lalu ditautkan lagi mendapat nomor baru; tugas dibatalkan tetap memegang nomornya; Duplikat = tugas baru = nomor baru. Nomor per Space, bukan per periode: Space sudah milik satu periode (`spaces.period_id`), jadi periode baru otomatis mulai dari `#T1`.
- API: `api.Task` mendapat `Numbers map[string]int64` (`spaceId → nomor`, `json:"numbers,omitempty"`). Klien lama mengabaikannya; klien baru jatuh ke `taskRef` bila kosong (server lama).
- Pencarian: kotak cari Space dan Ctrl K mengenali `#T12`, `T12`, atau `12` (hanya angka) sebagai nomor di Space aktif, selain pencocokan judul. Tanpa endpoint baru (data sudah dimuat).
- Tampilan: kartu board (`card.Ref`), kepala detail ("#T12"), baris Jobdesk, tab Tinjauan, dan teks ping/WA yang menyebut tugas (opsional; lihat persetujuan).

### Migrasi `0026_task_numbers.sql`
```sql
alter table task_spaces add column number bigint;
create table space_task_counters (
    space_id    uuid primary key references spaces(id),
    last_number bigint not null check (last_number >= 0)
);
-- Backfill: urut waktu dibuat, lalu id, per Space.
update task_spaces ts set number = n.rn
from (select ts2.task_id, ts2.space_id,
             row_number() over (partition by ts2.space_id order by t.created_at, t.id) rn
      from task_spaces ts2 join tasks t on t.id = ts2.task_id) n
where ts.task_id = n.task_id and ts.space_id = n.space_id;
insert into space_task_counters(space_id, last_number)
select space_id, max(number) from task_spaces group by space_id;
alter table task_spaces alter column number set not null;
alter table task_spaces add constraint task_spaces_number unique (space_id, number);
```
- Backfill memakai urutan waktu dibuat; tugas yang dibuat lebih dulu mendapat nomor kecil. Penautan ke Medhum yang dibuat belakangan tetap diurutkan menurut waktu tugas dibuat. {{Assumption}} urutan ini cukup untuk owner (alternatif: urut waktu ditautkan, tetapi waktu tautan tidak disimpan).
- Trigger realtime `task_spaces_realtime` ikut menyala untuk setiap baris yang di-update. Di DB pilot jumlah tugas puluhan, jadi aman; tetap dijalankan saat sepi.

### Risiko backfill di DB pilot
- Volume kecil (data pilot + dummy), jadi `update` + `alter … set not null` + `unique` selesai cepat. Kunci tabel `task_spaces` sesaat menahan simpan tugas, jadi API dihentikan selama migrasi (lihat langkah deploy).
- Nomor dummy `[DUMMY]` ikut terhitung, sehingga tugas nyata pertama bisa mulai dari `#T7` dan seterusnya. Pilihan: biarkan, atau hapus data dummy dulu (keputusan owner, di luar migrasi).
- Server lama + skema baru: `store.TaskSpaces` lama menyisipkan tanpa `number` dan akan **gagal** pada `not null`. Jadi untuk 6a rollback API ke pilot.13/Tahap 5 **tidak** bisa dilakukan tanpa membalik migrasi (lihat rollback). Ini beda dengan Tahap 5.

### Test
- Store/server (Postgres, CI): nomor berurutan per Space dan terpisah antar Space; 20 tugas dibuat paralel di satu Space → nomor 1..20 tanpa lubang/duplikat; sunting tugas tidak mengubah nomor; lepas lalu tautkan ulang → nomor baru; backfill pada data contoh (migrasi dari skema `0025` dengan tugas yang sudah ada); `migration_order_test` tetap lolos.
- UI: kartu/detail menampilkan `#T12` Space aktif; kosong → potongan ID lama; pencarian `#T12`/`T12`/`12`.

## 6b. Lampiran berkas di tugas

### Yang sudah ada dan dipakai ulang
- `internal/filestore`: simpan blob tak berubah (kunci UUID, SHA-256, deteksi tipe dari 512 byte pertama, batas ukuran), root `SPACE_ATTACHMENT_DIR`, batas `SPACE_ATTACHMENT_MAX_BYTES` (bawaan 20 MB, maks 100 MB). Blob tidak dihapus otomatis; backup/restore pilot sudah mencakup direktori lampiran.
- Alur unggah catatan (`internal/server/attachment_files.go`): `PUT …/attachments/{attachmentId}/file?name=` dengan ID dari klien (idempoten untuk coba ulang), izin diperiksa sebelum dan sesudah menerima berkas, tanda terima di `attachment_upload_receipts`, unduh dengan `Content-Disposition: attachment`.

### Desain yang diusulkan
- Migrasi `0027_task_attachments.sql`:
  ```sql
  create table task_attachments (
      id           uuid primary key,
      task_id      uuid not null references tasks(id),
      name         text not null check (length(name) between 1 and 300),
      content_type text not null,
      size_bytes   bigint not null check (size_bytes > 0),
      storage_key  uuid not null unique,
      sha256       text not null check (length(sha256) = 64),
      added_by     uuid not null references users(id),
      created_at   timestamptz not null,
      removed_at   timestamptz,
      removed_by   uuid references users(id)
  );
  create index task_attachments_task on task_attachments(task_id, created_at, id) where removed_at is null;
  create table task_attachment_receipts (   -- sama dengan attachment_upload_receipts, untuk tugas
      id uuid primary key, task_id uuid not null references tasks(id), actor_id uuid not null references users(id),
      storage_key uuid not null unique, sha256 text not null, metadata jsonb not null,
      created_at timestamptz not null, removed_at timestamptz
  );
  create trigger task_attachments_realtime after insert or update or delete on task_attachments
      for each row execute function space_task_child_changed();
  ```
  Tabel terpisah (bukan menambah `task_id` ke tabel catatan) supaya izin catatan dan tugas tidak tercampur. Hapus = *soft delete* (`removed_at`), blob tetap (aturan yang sama dengan catatan).
- Endpoint:
  - `PUT /api/tasks/{taskId}/attachments/{attachmentId}/file?name=` (unggah, body mentah)
  - `GET /api/tasks/{taskId}/attachments/{attachmentId}/file` (unduh)
  - `DELETE /api/tasks/{taskId}/attachments/{attachmentId}`
  - Daftar lampiran ikut di `GET /api/tasks/{id}` (`api.Task.Attachments`), dan `AttachmentCount` di daftar tugas Space untuk kartu (tanpa request tambahan).
- Izin: unggah = pengelola tugas atau PJ/peserta (yang boleh mengubah progres); hapus = pengunggah atau pengelola; unduh = siapa pun yang boleh membaca tugas. Tugas dibatalkan atau periode tertutup: hanya baca/unduh. Audit `task.attachment.upload|remove`.
- Batas: ukuran per berkas = `SPACE_ATTACHMENT_MAX_BYTES` (20 MB); maks 20 lampiran aktif per tugas; kuota per periode 2 GB dihitung dari `size_bytes` aktif ({{Assumption}} angka, minta owner). Tipe yang diizinkan (dari deteksi isi, bukan nama): gambar png/jpeg/webp/gif, pdf, teks/csv, docx/xlsx/pptx, zip. Ditolak: html, svg, berkas yang dapat dieksekusi. Unduhan selalu `attachment` + `X-Content-Type-Options: nosniff`.
- UI: bagian **Lampiran** di detail tugas (daftar nama, ukuran, pengunggah, Unduh, Hapus; tombol "Lampirkan berkas" memakai pemilih berkas yang sama dengan catatan, `pending_files.go`); hitungan lampiran di kartu board (ikon klip + angka, seperti canvas); web HP: pemilih berkas peramban.

### Test
- Server (Postgres, CI): unggah, coba ulang idempoten, konflik nama, batas ukuran (413), tipe ditolak, batas jumlah/kuota, izin (anggota lain, divisi lain, tugas batal, periode tertutup), hapus oleh pengunggah vs bukan, unduh membawa header aman, audit.
- UI: daftar di detail, hitungan di kartu, tombol tersembunyi tanpa izin, galat unggah tampil.

## Urutan kerja yang diusulkan
1. 6a di PC: kode + `0026` + test, commit; PR draft #10 untuk CI.
2. 6b di PC: kode + `0027` + test, commit; CI.
3. Deploy dari aws-vps (bisa satu kali untuk keduanya, atau 6b dulu karena tanpa risiko).

## Langkah deploy (sesi aws-vps)
Sama dengan Tahap 5 (`canvas-a-tahap5-rencana.md`, bagian "Langkah deploy"), dengan perbedaan berikut.
1. **Backup** wajib dengan nama `before-0026-<waktu>.dump`, cek `pg_restore -l`. Tambahkan juga salinan direktori lampiran (`tar` dari `SPACE_ATTACHMENT_DIR`, izin 0600) sebelum 6b, walaupun blob lama tidak disentuh.
2. **Hentikan API sebelum migrasi** (`systemctl --user stop himasif-space-pilot-api`), jalankan `space-server -env-file … migrate`, lalu start dengan binary baru. Jangan biarkan binary lama jalan di atas skema `0026` (lihat risiko).
3. **Cek sehat**:
   ```sql
   select count(*) filter (where number is null) from task_spaces;            -- 0
   select space_id, count(*), max(number) from task_spaces group by 1;          -- count = max per Space
   select s.space_id from space_task_counters s join (select space_id, max(number) m from task_spaces group by 1) t using(space_id) where s.last_number <> t.m;  -- kosong
   ```
   API: buat satu tugas uji di Space dummy → nomor = `max+1`; sunting → nomor tetap. 6b: unggah PDF kecil ke tugas dummy, unduh, hapus.
4. **Rollback**
   - Web: `web.before-tahap6` seperti biasa.
   - 6b saja: binary lama aman di atas `0027` (tabel tidak dibaca); tidak perlu membalik.
   - 6a: binary lama **tidak** aman di atas `0026`. Pilihan: (a) jalan maju dengan perbaikan (disarankan); (b) balik skema setelah API berhenti:
     ```sql
     alter table task_spaces drop constraint task_spaces_number;
     alter table task_spaces drop column number;
     drop table space_task_counters;
     delete from schema_migrations where name = '0026_task_numbers';
     ```
     lalu start binary lama; (c) restore penuh dari `before-0026` hanya dengan persetujuan owner (menghapus data sesudah backup).

## Yang perlu disetujui
- [ ] 6a: nomor **per Space** (tugas di dua Space punya dua nomor), backfill urut waktu dibuat, nomor tidak dipakai ulang. Alternatif: satu nomor per tugas per divisi pemilik.
- [ ] 6a: data `[DUMMY]` ikut bernomor (atau dibersihkan dulu oleh owner).
- [ ] 6a: nomor juga ditulis di pesan ping/WA tentang tugas (ya/tidak).
- [ ] 6b: batas 20 MB per berkas, 20 lampiran per tugas, kuota 2 GB per periode, daftar tipe di atas.
- [ ] 6b: izin unggah PJ/peserta + pengelola; hapus pengunggah atau pengelola.
- [ ] Migrasi `0026` dan `0027` ke DB pilot dengan backup dulu, API dihentikan saat `0026`.
- [ ] Push branch, CI lewat PR draft #10, deploy dari aws-vps.
