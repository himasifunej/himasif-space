# Skrip uji pengguna BPH (F1-26)

Tanggal: 25 November 2026, WIB. Item rencana: **F1-26** (keputusan owner no. 14: untuk rilis stabil, BPH cukup **kontrol sederhana dan rapat**). Acuan: [PRD](../PRD.md) §2.3 REL-04 dan §4.4 MON-01–09, [ADR 0002](../adr/0002-pemantauan-bph.md).

## Ringkas

| | |
|---|---|
| Tujuan | Membuktikan BPH bisa memantau divisi dan menjalankan rapat tanpa dibimbing: dasbor Pemantauan organisasi, satu divisi dalam mode pemantauan, Rapat pengurus dengan kehadiran, ping kadiv, dan ringkasan mingguan |
| Penguji | Ketua umum (+ owner sebagai admin yang menyiapkan izin dan mengamati) |
| Waktu | ±20 menit |
| Jadwal | Kam 3 – Jum 4 Des 2026 di 1.0.0-rc.1 (instans uji), **setelah** uji Mediatek berjalan |
| Masukan | Note masukan yang sama dengan uji Mediatek; tiap langkah gagal menjadi to-do di note rencana |

Di luar cakupan uji ini (paket BPH lanjutan, bukan 1.0.0): persetujuan proker/LPJ, keuangan Bendahara, surat bernomor dan absensi Sekretaris, eskalasi otomatis.

## Aturan pengamatan

Sama dengan [uji Mediatek](uji-pengguna-mediatek.md#aturan-pengamatan): owner tidak membantu sampai penguji menyerah, penguji membaca langkah dari daftar ini dan berpikir keras, semua data uji berjudul `[UJI]`, dan tidak ada pesan WA ke grup selain grup Test.

Privasi yang perlu diperhatikan penguji (PRD MON-02–04, MON-09): pemantau melihat tugas, bukti, komentar, Notulensi dan NIM anggota divisi yang dipantau; divisi **tidak** diberi tahu, tetapi setiap pembukaan tercatat di log yang dibaca admin. Reminder pribadi dan draf catatan selalu tertutup.

## Persiapan owner

1. Ketua umum punya akun dan keanggotaan aktif di divisi BPH periode yang diuji.
2. Sebagai admin organisasi: Organisasi → Pemantauan → beri izin pemantauan kepada ketua umum untuk periode itu.
3. Pastikan Space Mediatek berisi data uji dari uji Mediatek: minimal satu tugas `[UJI]` lewat Deadline, satu yang menunggu tinjauan, dan satu rapat dengan Notulensi.
4. Pastikan kadiv Mediatek (owner) bisa menerima ping di perangkat.
5. Ketua umum sudah menautkan WhatsApp, atau punya email aktif untuk ringkasan.

## Langkah

| # | Langkah untuk penguji | Batas | Lolos bila |
|---|---|---|---|
| 1 | Buka dasbor Pemantauan organisasi dan sebutkan divisi mana yang paling banyak tugas lewat Deadline | 2 menit | Penguji menemukan dasbor dan menyebut divisi yang benar |
| 2 | Lihat divisi Mediatek: buka satu tugas yang menunggu tinjauan dan baca buktinya | 3 menit | Tugas, bukti dan komentar terbaca; penguji tidak bisa mengubah tugas atau berkomentar |
| 3 | Baca Notulensi rapat terakhir Mediatek | 2 menit | Notulensi terbaca dari mode pemantauan atau tab Rapat lintas divisi |
| 4 | Setujui bukti tugas yang menunggu tinjauan tadi | 2 menit | Tugas menjadi selesai dan riwayatnya mencatat persetujuan oleh pemantau |
| 5 | Kirim ping ke kadiv Mediatek tentang tugas yang lewat Deadline | 2 menit | Kadiv menerima ping di perangkat (bukan di grup WA divisi) |
| 6 | Di Space BPH, buat Rapat pengurus besok dan undang kadiv semua divisi | 4 menit | Rapat tersimpan dengan jenis Rapat pengurus; para kadiv melihatnya di Agenda/Untukku |
| 7 | Setelah rapat (owner memajukan waktunya bila perlu), isi kehadiran peserta | 3 menit | Kehadiran tersimpan dan terlihat di detail rapat |
| 8 | Buka Pengaturan → Ringkasan mingguan, pilih kanal (WA atau email), lalu buka pratinjau minggu lalu | 2 menit | Pengaturan tersimpan; pratinjau berisi hitungan dan judul tanpa isi catatan |

Langkah tambahan tanpa batas waktu: **Senin 7 Des 2026 pukul 07.00 WIB**, ketua umum memastikan ringkasan mingguan benar-benar datang lewat kanal yang dipilih (lolos bila datang sekali, tidak ganda). Karena jatuh setelah rilis 1.0.0, langkah ini tidak menahan rilis; bila gagal, menjadi perbaikan 1.0.1.

## Pertanyaan penutup

1. Apa yang ingin kamu ketahui tentang divisi tetapi tidak terlihat di dasbor?
2. Ada yang terasa terlalu terbuka atau terlalu tertutup (privasi anggota)?
3. Nilai 1–5: "Saya bisa memantau divisi dan menjalankan rapat pengurus dengan Space tanpa diajari."

## Kriteria lolos

- Langkah **1, 2, 4, 6 dan 7 lolos**.
- Mode pemantauan **tidak** membuka reminder pribadi atau draf, dan **tidak** mengizinkan mengubah tugas atau berkomentar (bila bisa: gagal berat, menahan rilis).
- Nilai pertanyaan 3 **≥ 4**.
- Setiap langkah gagal dicatat sebagai to-do di note rencana, dengan perangkat dan versi.

## Lembar hasil

| # | Hasil | Waktu | Catatan |
|---|---|---|---|
| 1 | | | |
| 2 | | | |
| 3 | | | |
| 4 | | | |
| 5 | | | |
| 6 | | | |
| 7 | | | |
| 8 | | | |
| Senin 7 Des | | — | |

Perangkat: … · Versi: … · Tanggal: …
