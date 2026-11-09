# Rencana Tahap 7 canvas A: Evaluasi SOP, drawer Buat tugas, Ping, Bot WhatsApp (9 November 2026)

> **9 Nov 2026:** langkah deploy di bawah ditulis untuk aws-vps, yang sudah pensiun. Pilot sekarang ada di tc-vps; pakai `docs/handoff/deploy-pilot.md` (build di PC, jangan di server).

Status: **disetujui owner 9 Nov (5 butir); dikerjakan berurutan 7b, 7a, 7c, 7d**. Tanpa migrasi; deploy menunggu izin terpisah. Tahap 6 sudah live (6e18351, migrasi sampai `0027`).

Pemenang yang ditandai owner di canvas: **Ping → C · Desktop** (canvas `826e909e5957`, desain 5) dan **Bot WhatsApp → A · Desktop** (canvas `121115cdae78`, desain 1). Versi HP tidak ditandai (canvas hanya menyimpan satu pemenang); {{Assumption}} versi HP mengikuti varian yang sama: Ping C · HP (desain 6), Bot WhatsApp A · HP (desain 2).

## Ringkasan
| # | Pekerjaan | Backend | Migrasi DB |
|---|---|---|---|
| 7a | Evaluasi SOP seperti canvas A (segmen, tabel, grafik per aturan, Ekspor CSV) | 1 endpoint baca `sop/summary` (opsi A, dipilih) | tidak |
| 7b | Drawer Buat tugas: chip PJ dan contoh isian | tidak ada | tidak |
| 7c | Tab Ping = canvas Ping **C** (bilah kirim, Belum melihat, linimasa per hari, halaman per 50) | parameter `filter=` di `GET …/pings` | tidak |
| 7d | Tab Bot WhatsApp = canvas Bot **A** (status + langkah + nomor anggota) + **Pratinjau** pesan | 1 endpoint pratinjau (hanya membaca) | tidak |

Tidak ada migrasi di seluruh Tahap 7. Deploy = binary API + web.

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

### Segmen "Sesuai": opsi A dipilih
- **Opsi A (dipilih; tidak ada alasan kuat untuk opsi B): endpoint baca `GET /api/periods/{p}/divisions/{d}/sop/summary?quarter=<label>` atau `?from=&to=`.**
  Untuk tiap konten Medhum dalam rentang, server menyusun masukan dengan fungsi yang sama dengan worker (`sopInput`, `internal/server/sop_worker.go:298`) lalu menjalankan `sop.Evaluate` per aturan kebijakan, dan menggabungkannya dengan state temuan yang ada. Hasil per aturan: `evaluated`, `compliant` (on_time / within_limit), `needsReview`, `confirmed`, `dismissed`, `corrected`, `insufficient`, `notApplicable` (exempt / not_applicable). Izin sama dengan `report` (baca SOP). Dihitung saat dibaca, **tanpa tabel atau migrasi**, tidak menulis apa pun, tidak membuat temuan.
  Batas: satu kuartal / maks. 120 hari per permintaan dan maks. 2000 konten (400 bila lebih), supaya tidak berat.
- Opsi B (tidak dipakai, dicatat untuk riwayat): grafik hanya dari temuan (`report`): Perlu tinjauan / Pelanggaran disahkan / Bukan pelanggaran. Segmen "Sesuai" dihilangkan dari legenda; ada keterangan "Kejadian tepat waktu belum dihitung". Lebih cepat, tetapi tidak sama dengan canvas.

### Perubahan UI
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

## 7c. Ping = canvas C

### Yang dibuat (UI)
- **Bilah kirim satu baris** di atas: pemilih penerima ("2 penerima"), kolom pesan, pemilih sumber (Umum / tugas / rapat), kanal Perangkat · WhatsApp, tombol Kirim ping. Isinya sama dengan komposer sekarang (`pingComposer`), hanya ditata ringkas.
- **Kolom kiri "Belum melihat"**: anggota yang punya ping belum dilihat (dari riwayat yang dimuat), alasan singkat (mis. "WA gagal · nomor belum tertaut"), tombol **Ping** per orang.
- **Kolom kanan "Riwayat ping"**: linimasa per hari (Hari ini, Kemarin, tanggal), filter **Semua / Belum dilihat / Gagal**; tiap ping menampilkan pengirim, pesan, sumber, "x/y dilihat", avatar penerima dengan titik status, tombol **Ping ulang n** (kirim ulang ke yang belum melihat lewat `POST …/pings` yang ada).
- **Halaman per 50**: "Lebih lama" memanggil `GET /api/spaces/{id}/pings?before=<waktu ping tertua di halaman>&limit=50`; "Lebih baru" kembali ke kursor sebelumnya (tumpukan kursor di klien). Tanpa nomor halaman total (server tidak menghitung total).
- **HP**: bilah kirim di atas, riwayat sebagai lembar yang bisa ditarik, filter chip, tombol "Lebih lama".

### Backend
- Tambah parameter `filter=unseen|failed` pada `GET /api/spaces/{id}/pings` (ada sejak Tahap 5) agar filter berlaku untuk semua halaman, bukan hanya 50 yang dimuat. Tanpa migrasi; indeks `0024` sudah ada.
- Kolom "Belum melihat" dihitung di klien dari halaman yang dimuat ({{Assumption}} cukup untuk pilot).

### Risiko
- "Dilihat" hanya tercatat untuk kanal Perangkat (`seen_at`); WhatsApp tidak memberi tanda baca, jadi kiriman WA tampil "terkirim", bukan "dilihat". Ditulis jelas di UI.
- Ping ulang bisa mengganggu bila diklik berkali-kali: tombol dikunci selama mengirim dan ada konfirmasi singkat "Ping ulang 2 orang?".

### Test
- Server (Postgres/CI): `filter=unseen` / `filter=failed` hanya mengembalikan ping yang cocok; izin tetap (anggota biasa hanya ping untuknya); `before` + `limit` bersama filter.
- UI: kursor Lebih lama/Lebih baru benar; kolom Belum melihat; Ping ulang hanya ke yang belum melihat; versi HP.

## 7d. Bot WhatsApp = canvas A + Pratinjau

### Yang dibuat (UI)
- **Kalimat status** di atas: "Bot belum terhubung" / "Bot aktif, 2 anggota belum menautkan" / "Bot berjalan normal", dengan satu tombol langkah berikutnya. Data: status koneksi dan sesi yang sudah ada.
- **Langkah penyiapan (4)**: 1 Hubungkan nomor WhatsApp divisi (QR, alur yang ada), 2 Pilih grup divisi, 3 Anggota menautkan nomornya (x/y dari `whatsappLinked` Tahap 5d), 4 Atur jadwal pengingat (aturan pengingat yang ada). Tiap langkah: Selesai / tombol / menunggu.
- **Kanan**: "Yang dilakukan bot" (pengingat tenggat, ping, perintah grup) dan daftar **nomor anggota** Tertaut / Belum, dengan **Kirim cara menautkan** (menyalin pesan berisi langkah: buka Space → Bot WhatsApp → Buat kode tautan → kirim kodenya di grup).
- **Pengaturan lanjutan** (lipat): nama bot, bahasa, sapaan, teks pesan (pengingat, ping, balasan, ringkasan mingguan, follow-up), perintah khusus, catatan pengiriman. Istilah teknis ("JID", "konektor", "command", "{body}") tidak tampil; isian otomatis jadi chip "Nama bot", "Judul", "Isi pesan", "Sapaan".
- **Pratinjau (permintaan owner)**: tombol **Pratinjau** di setiap tempat yang mengubah teks pesan (nama bot, sapaan, tiap teks pesan, jadwal pengingat, ringkasan mingguan). Membuka gelembung chat bergaya WhatsApp yang berubah saat mengetik, dengan isian otomatis diisi contoh data (nama anggota, "#T12 Poster open recruitment", tenggat "Sel 10 Nov"), berlabel "Contoh".
- **HP**: kartu status + daftar langkah yang bisa diketuk; Pratinjau muncul sebagai lembar.

### Backend
- Endpoint baca **`POST /api/periods/{p}/divisions/{d}/whatsapp/preview`**: menerima draf (nama, bahasa, sapaan, teks pesan) + jenis pesan, mengembalikan teks persis seperti yang akan dikirim, memakai fungsi server yang sama (`renderWAText` dan teks bawaan per bahasa). Contoh data di server. Tidak menyimpan, tidak mengirim WhatsApp, tanpa migrasi. Izin sama dengan mengubah pengaturan WhatsApp divisi.
- Mengapa lewat server: aturan tambahan nama/sapaan, pemotongan teks panjang dan bahasa ada di server; pratinjau harus sama persis dengan pesan asli.

### Risiko
- Pratinjau memanggil server saat mengetik: diberi jeda 300 ms dan hanya ketika Pratinjau terbuka.
- Pengaturan pindah ke "Pengaturan lanjutan": letaknya berubah bagi kadiv, isinya tetap.

### Test
- Server (Postgres/CI): hasil pratinjau sama dengan teks yang dikirim worker untuk data yang sama (pengingat, ping, balasan, mingguan); teks tanpa isi pesan tetap menampilkan isi; tanpa izin = 403; antrean pengiriman tidak bertambah.
- UI: tiga keadaan status, langkah berikutnya benar, Kirim cara menautkan menyalin teks, Pratinjau berubah saat mengetik, versi HP.

## Hasil pengerjaan (PC Tiyo, 9 Nov)
| Commit | Isi |
|---|---|
| 73179e5 | 7b drawer Buat tugas: contoh isian, chip PJ (maks. 8 + Lainnya, "Saya"), "Detail lainnya" terlipat |
| 8c807fc | 7a Evaluasi SOP: `GET …/sop/summary`, segmen Temuan · Laporan kuartal · Kebijakan, tabel, grafik per aturan, Ekspor CSV satu langkah, HP kartu |
| a852d1e | 7c Ping C: `filter=unseen\|failed`, bilah kirim, Belum melihat, linimasa per hari, Ping ulang dengan konfirmasi, halaman per 50, HP |
| 5b4fef4 | 7d Bot WhatsApp A: status 3 keadaan, 4 langkah, nomor anggota, Pengaturan lanjutan tanpa istilah teknis, Pratinjau (`POST …/whatsapp/preview`) |

Tangkapan cek visual (harness test, desktop dan HP): `E:\himasif-cache\shots\cap\` (drawer-new-task, sop-desktop, sop-phone, ping-c-desktop, ping-c-phone, wa-a-partial, wa-a-preview, wa-a-phone).

Catatan: "Belum melihat" di Ping dihitung dari halaman yang dimuat; tombol "Perintah khusus" di Pengaturan lanjutan membuka halaman Command di Pengaturan (belum dipindah ke tab). Tidak ada migrasi.

## Urutan kerja
1. 7b (kecil, UI saja).
2. 7c Ping (UI + parameter `filter`).
3. 7d Bot WhatsApp (UI + endpoint pratinjau).
4. 7a Evaluasi SOP (UI + endpoint `sop/summary`).
5. Tiap bagian: kode + test, commit; push; CI lewat PR draft #10.

## Langkah deploy (sesi aws-vps, setelah persetujuan terpisah)
Tanpa migrasi, jadi tanpa backup DB wajib (backup harian tetap jalan).
1. `git pull` branch, test dengan lock RAM: `go test ./internal/appui ./internal/workspaceui ./internal/server`.
2. API: build `space-server`, simpan binary lama sebagai `<binary>.before-tahap7`, pasang yang baru, `systemctl --user restart himasif-space-pilot-api`.
3. Cek sehat: `curl -fsS https://db.tiyoouw.app/api/health`; dengan akun kadiv: `GET …/pings?filter=unseen&limit=5` (200), `POST …/whatsapp/preview` (200, teks pesan), `GET …/sop/summary?quarter=<label>` (200, atau pesan "kebijakan belum ditetapkan").
4. Web: `make web`, cadangkan `web` ke `web.before-tahap7`, tukar.
5. Cek di app: Ping, Bot WhatsApp (status, langkah, Pratinjau), Evaluasi SOP, drawer Buat tugas.

### Rollback
- Web: kembalikan `web.before-tahap7`.
- API: kembalikan `<binary>.before-tahap7`, restart. Semua endpoint baru hanya membaca; tidak ada data yang perlu dibersihkan. Web baru di atas API lama tetap jalan (filter Ping hanya di halaman yang dimuat; Pratinjau dan grafik "Sesuai" tampil "belum tersedia").

## Yang perlu disetujui (singkat)
- [x] 1. Kerjakan 7a–7d seperti di atas: Ping pakai desain C, Bot WhatsApp desain A, versi HP ikut varian yang sama.
- [x] 2. Evaluasi SOP: grafik "Sesuai" lewat endpoint baca baru; tab Konteks/Aturan/Pengiriman/Izin pindah ke "Kebijakan".
- [x] 3. Bot WhatsApp: pengaturan detail pindah ke "Pengaturan lanjutan"; Pratinjau lewat server (hasilnya persis pesan asli).
- [x] 4. Drawer Buat tugas: chip PJ (maks. 8 + Lainnya); isian tambahan dilipat di "Detail lainnya".
- [x] 5. Kode + test + CI dulu; deploy minta izin lagi.
