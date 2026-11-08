# Rencana Tahap 7 canvas A: Evaluasi SOP dan drawer Buat tugas (9 November 2026)

Status: **rencana, menunggu persetujuan lewat GOA**. Belum ada kode, migrasi, atau deploy untuk tahap ini. Tahap 6 sudah live (6e18351, migrasi sampai `0027`).

## Ringkasan
| # | Pekerjaan | Backend | Migrasi DB |
|---|---|---|---|
| 7a | Halaman Evaluasi SOP ditata seperti canvas A (segmen, tabel temuan, kartu grafik per aturan, Ekspor CSV) | 1 endpoint baca baru (opsi A) **atau** tidak ada (opsi B) | tidak |
| 7b | Drawer Buat tugas: chip PJ dan contoh isian | tidak ada | tidak |

Tidak ada migrasi di tahap ini. Deploy cukup binary API (bila opsi A) + web.

## 7a. Evaluasi SOP seperti canvas

### Canvas A (frame "A · Hangat gelap", halaman Evaluasi SOP)
- Kepala: judul "Evaluasi SOP Medhum" + keterangan ("Temuan awal dari waktu konsep, follow-up, penyerahan dan tenggat. Semua mulai berstatus perlu tinjauan; tidak diumumkan ke grup Medhum."); segmen kanan **Temuan · Laporan kuartal · Kebijakan**.
- Kiri: **tabel** kolom TEMUAN (judul konten + keterangan bukti kecil) · ATURAN ("Desain H-2", "Konsep H-7") · PIHAK ("Mediatek · Aruna") · STATUS (titik warna + "Perlu tinjauan" / "Bukan pelanggaran" / "Pelanggaran disahkan").
- Kanan: kartu **"Kuartal 4 · per aturan"**: batang bertumpuk per aturan (Konsep, Desain, Follow-up, Posting), legenda Sesuai / Perlu tinjauan / Pelanggaran disahkan, tombol **Ekspor CSV**.

### App sekarang (dibandingkan 9 Nov, catatan audit)
Tumpukan formulir: pilihan divisi, kotak kebijakan, 6 tab (Temuan, Konteks, Aturan, Laporan, Pengiriman, Izin), filter tanggal/poin (placeholder sudah diperbaiki di 0f69a9d), chip status dan PJ, "Terapkan filter", daftar temuan bukan tabel, tanpa grafik. Ekspor CSV ada di tab Laporan (`sop_export.go`).

### Data yang dibutuhkan dan sumbernya
| Bagian canvas | Sumber yang sudah ada | Kurang |
|---|---|---|
| Baris tabel: konten, aturan, tanggal, PJ, status | `GET …/sop/findings` (`api.SOPFinding`: `ContentTitle`, `Rule`, `EventOn`, `PersonName`, `State`, `Latest.Observation.Measurement/Reason`) | nama divisi untuk kolom PIHAK: dari `GET /api/periods/{p}/sop-scopes` / daftar Space yang sudah dimuat |
| Nama aturan ("Konsep H-7") | kode aturan di `internal/sop` (`7.5` Konsep H-7, `7.11` Desain H-2, `7.12` Dadakan 24 jam, `7.14` Follow-up H-4, `7.19` Final H-12, `7.6` Batas dadakan) | peta label di klien (tanpa backend) |
| Status chip | `state`: needs_review → Perlu tinjauan, confirmed → Pelanggaran disahkan, dismissed → Bukan pelanggaran, corrected → Diralat | — |
| Kuartal | `policy.Settings.Quarters` (label + rentang) | — |
| Grafik: Perlu tinjauan / Pelanggaran disahkan / Bukan pelanggaran per aturan | `GET …/sop/report?quarter=` (`Summary` + `Findings`) dihitung di klien | — |
| Grafik: **Sesuai** per aturan | **tidak tersimpan**: worker hanya membuat temuan bila hasil `late`, `exceeds_limit` atau `insufficient` (`internal/server/sop_worker.go:220`); kejadian tepat waktu tidak dicatat | perlu agregasi baru (opsi A) |
| Ekspor CSV | `sop_export.go` + `GET …/sop/report` | dipindah ke kartu grafik |

### Dua opsi untuk segmen "Sesuai"
- **Opsi A (disarankan, sesuai canvas): endpoint baca `GET /api/periods/{p}/divisions/{d}/sop/summary?quarter=<label>` atau `?from=&to=`.**
  Untuk tiap konten Medhum dalam rentang, server menyusun masukan dengan fungsi yang sama dengan worker (`sopInput`, `internal/server/sop_worker.go:298`) lalu menjalankan `sop.Evaluate` per aturan kebijakan, dan menggabungkannya dengan state temuan yang ada. Hasil per aturan: `evaluated`, `compliant` (on_time / within_limit), `needsReview`, `confirmed`, `dismissed`, `corrected`, `insufficient`, `notApplicable` (exempt / not_applicable). Izin sama dengan `report` (baca SOP). Dihitung saat dibaca, **tanpa tabel atau migrasi**, tidak menulis apa pun, tidak membuat temuan.
  Batas: satu kuartal / maks. 120 hari per permintaan dan maks. 2000 konten (400 bila lebih), supaya tidak berat.
- **Opsi B (tanpa backend)**: grafik hanya dari temuan (`report`): Perlu tinjauan / Pelanggaran disahkan / Bukan pelanggaran. Segmen "Sesuai" dihilangkan dari legenda; ada keterangan "Kejadian tepat waktu belum dihitung". Lebih cepat, tetapi tidak sama dengan canvas.

### Perubahan UI (kedua opsi)
- Kepala halaman canvas: judul "Evaluasi SOP <Space/divisi>", keterangan canvas, segmen **Temuan · Laporan kuartal · Kebijakan** di kanan. Pemilih divisi pindah ke kepala (chip kecil) bila akun punya lebih dari satu cakupan.
- **Temuan**: tabel 4 kolom (desktop/tablet, grid kolom tetap seperti tabel Anggota), kartu di HP; klik baris membuka panel detail temuan yang sudah ada (keputusan, riwayat, transfer tetap di sana). Filter (status, PJ, rentang, poin) jadi satu baris ringkas di atas tabel; "Terapkan filter" diganti penerapan langsung saat memilih chip, tanggal tetap lewat tombol.
- **Kartu kanan** (desktop ≥ 1100 px; di bawah tabel bila lebih sempit): judul "<Kuartal> · per aturan", batang bertumpuk per kelompok aturan (Konsep 7.5, Desain 7.11, Follow-up 7.14, Posting 7.19, Dadakan 7.12 + 7.6 bila ada), legenda, tombol **Ekspor CSV** (alur `sop_export.go` yang ada).
- **Laporan kuartal**: isi tab Laporan sekarang (ringkasan + pengiriman laporan), dengan pemilih kuartal dari kebijakan.
- **Kebijakan**: tab Konteks, Aturan, Pengiriman dan Izin sekarang jadi sub-bagian di sini, tetap mengikuti izin yang ada (`api.SOPAccess`).
- Notice "Kebijakan belum ditetapkan" tetap tampil (di atas tabel) bila belum ada kebijakan; grafik kosong dengan keterangan.

### Test
- Server (opsi A, Postgres/CI): kuartal dengan konten tepat waktu, terlambat, bukti kurang dan dikecualikan → angka per aturan; state temuan (disahkan/dibatalkan/diralat) terhitung; izin (bukan evaluator / divisi lain = 403/404); rentang > 120 hari = 400; endpoint tidak menulis (jumlah baris `sop_findings`/`sop_observations` sama sebelum-sesudah).
- UI: tabel dengan label aturan dan status, klik baris membuka detail, segmen berpindah, kartu grafik menghitung dari data contoh, Ekspor CSV memakai rentang kuartal, HP menampilkan kartu.

### Risiko
- Opsi A menjalankan evaluasi di jalur baca: harus memakai fungsi worker yang sama (bukan salinan) agar angka "Sesuai" konsisten dengan temuan; refaktor kecil `sopInput` agar bisa dipanggil tanpa efek samping (sekarang hanya membaca, {{Unverified}} dicek ulang saat implementasi).
- Data pilot: Mediatek belum punya kebijakan SOP ("Kebijakan belum ditetapkan"), jadi grafik kemungkinan kosong sampai kadiv menyimpan kebijakan; ini bukan cacat.
- Memindah tab Konteks/Aturan/Pengiriman/Izin ke "Kebijakan" mengubah letak fitur yang mungkin sudah dipakai; test lama SOP UI perlu disesuaikan.

## 7b. Drawer Buat tugas: chip PJ dan contoh isian

### Canvas
"Tugas baru": Judul tugas (contoh "Contoh: Desain poster open recruitment"), Hasil yang diharapkan (contoh "Contoh: Poster 1080×1350 di folder Drive"), **PJ sebagai baris chip avatar** (Aruna, Nara, Raka, Saya; chip terpilih bergaris aksen), Jadwal; kaki Batal / Simpan tugas.

### App sekarang
Drawer sudah "Tugas baru" dengan Batal / Simpan tugas, tetapi isian kosong tanpa contoh, PJ lewat tombol "Pilih PJ utama *" yang membuka popover daftar, dan ada isian tambahan (Deskripsi, Label, Jadwal posting, Bagikan juga ke).

### Perubahan (UI saja, tanpa backend)
- Placeholder contoh canvas di Judul tugas dan Hasil yang diharapkan (`taskInput`/`taskLongInput` diberi varian dengan placeholder).
- **Chip PJ**: anggota divisi pemilik dari `loadTaskMembers` (sudah dimuat untuk popover), avatar + nama depan, "Saya" bila akun sendiri anggota; chip terpilih bergaris aksen. Maks. 8 chip + chip "Lainnya…" yang membuka popover lama (untuk divisi besar). Label tetap terbaca layar ("PJ: Aruna Desain").
- Urutan isian seperti canvas: Judul, Hasil, PJ, Jadwal; isian tambahan (Deskripsi, Label, Jadwal posting, Bagikan juga ke) tetap ada di bawahnya, dikelompokkan dalam bagian lipat **"Detail lainnya"** yang terbuka otomatis bila salah satunya terisi (perlu persetujuan; alternatif: biarkan terbuka seperti sekarang).

### Test
- UI: placeholder tampil; chip PJ memilih `assignee`; "Saya" muncul untuk anggota; > 8 anggota memunculkan "Lainnya…" yang membuka popover; Simpan tetap terkunci tanpa PJ; "Detail lainnya" terbuka bila ada label; test drawer lama (`tasks_view`) disesuaikan.

## Urutan kerja yang diusulkan
1. 7b (kecil, UI saja): kode + test, commit.
2. 7a tampilan (tabel, segmen, kartu dengan data `report`): kode + test, commit.
3. 7a opsi A (endpoint `summary`) bila disetujui: kode + test server, commit; kartu memakai `summary` dan jatuh ke data `report` bila server lama (404).
4. Push, CI lewat PR draft #10.

## Langkah deploy (sesi aws-vps)
Tanpa migrasi, jadi tanpa backup wajib; tetap ikuti pola Tahap 5:
1. `git pull` branch, `go test ./internal/appui ./internal/workspaceui ./internal/server` (dengan lock RAM).
2. Bila opsi A: build `space-server`, simpan binary lama `<binary>.before-tahap7`, pasang binary baru, `systemctl --user restart himasif-space-pilot-api`, cek `curl -fsS https://db.tiyoouw.app/api/health` dan `GET …/sop/summary?quarter=<label>` dengan akun kadiv (200; tanpa kebijakan: pesan galat "kebijakan belum ditetapkan", kode persisnya {{Unverified}}).
3. Web: `make web`, salin ke `web.new`, cadangkan `web` ke `web.before-tahap7`, tukar.
4. Cek: Evaluasi SOP (segmen, tabel, kartu, Ekspor CSV), drawer Buat tugas (contoh isian, chip PJ).

### Rollback
- Web: kembalikan `web.before-tahap7`.
- API (opsi A): kembalikan `<binary>.before-tahap7` dan restart; endpoint baru hanya membaca, tidak ada data yang perlu dibersihkan. Web baru dengan API lama tetap jalan (kartu jatuh ke data `report`).

## Yang perlu disetujui
- [ ] 7a: grafik "Sesuai" lewat endpoint baca baru `sop/summary` (opsi A, disarankan) **atau** tanpa backend tanpa segmen Sesuai (opsi B).
- [ ] 7a: tab Konteks, Aturan, Pengiriman, Izin dipindah ke segmen "Kebijakan"; segmen atas jadi Temuan · Laporan kuartal · Kebijakan.
- [ ] 7a: kelompok batang Konsep 7.5 · Desain 7.11 · Follow-up 7.14 · Posting 7.19 · Dadakan (7.12 + 7.6).
- [ ] 7b: chip PJ maks. 8 + "Lainnya…", contoh isian canvas.
- [ ] 7b: isian tambahan drawer dikelompokkan dalam "Detail lainnya" (atau tetap terbuka).
- [ ] Kode + test + CI di branch ini; deploy menunggu persetujuan terpisah.
