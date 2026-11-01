# Transport WA dan fondasi SOP 4D

Kelanjutan backend worker/keputusan/follow-up/rekap kini ditulis; lihat [checkpoint 29 Oktober](sop-workflows.md). UI dan beberapa alur masih tersisa.

Status: perubahan sumber ditulis, **belum build, test, migrasi atau runtime**. Ini belum merupakan klaim ketahanan dua sesi ataupun implementasi lengkap 4D. Semua tahap tersisa dan verifikasi akhir tetap bagian goal aktif.

## Transport yang direvisi

- `internal/store/locks.go` menyediakan advisory lock sesi melalui koneksi fisik khusus yang selalu ditutup. Koneksi tersebut tidak mengambil slot pool transaksi API sepanjang network call. `internal/server/whatsapp_transport.go` membatasi dua operasi transport per instance dan satu operasi per bot lintas instance.
- Kontrol sesi menyimpan keinginan/versi dan audit dalam transaksi singkat; panggilan bridge dilakukan sesudah commit, dengan periksa versi terbaru sebelum tindakan. Operasi koneksi yang bersamaan memberi respons sibuk, bukan menahan periode lain.
- Polling mengambil snapshot singkat, melakukan Configure/Action/Events di luar transaksi, lalu menyimpan event dengan pemetaan/periode terkini. Cursor tidak boleh berubah selama transport; ack tetap sesudah commit. Status snapshot yang versinya sudah tergantikan tidak menimpa konfigurasi baru.
- Outbox mengklaim job setelah memperoleh lock transport bot, menyimpan pemeriksaan akses dan percobaan dalam transaksi singkat, lalu memanggil Send/Job di luar transaksi. Hasil disimpan dengan pemeriksaan lease yang sama. Ketika percobaan yang sudah diotorisasi masuk `sending`, perubahan izin berikutnya tidak mengklaim dapat menarik kembali pesan yang sedang in flight; pekerjaan yang masih antre diperiksa ulang sebelum percobaan.
- Worker polling, command, parser dan outbox memiliki loop terpisah. Polling/pengiriman dua bot dapat berjalan paralel dengan batas dua operasi. Semua loop ditunggu sebelum store ditutup. Konversi stiker juga dipersiapkan di luar transaksi mutasi; wewenang pengirim/grup/role diperiksa lagi ketika hasil disimpan.
- Bridge mempertahankan `acceptedAt` pertama dan tingkat receipt yang lebih kuat. Penyelesaian `sendMessage` atau receipt yang datang belakangan tidak menurunkan `read` menjadi `accepted`. Waktu penerimaan bridge tersedia pada `whatsapp.Job` dan `wa_outbox.accepted_at` untuk bukti follow-up; waktu polling Go bukan pengganti waktu kirim asli.
- Runtime dua sesi, cancellation, restart, pool kecil, kegagalan bridge, race konfigurasi dan receipt masih harus diuji di fase akhir. Bahasa respons inti 4B juga belum dilengkapi.

## Fondasi SOP yang ditulis

- `internal/sop/evaluate.go`: evaluator murni untuk 7.5, 7.11, 7.12, 7.14, 7.19 dan 7.6. Hasilnya observasi `late`, `insufficient`, `on_time`, `exempt`, `within_limit`, `exceeds_limit` atau `not_applicable`; tidak mengesahkan pelanggaran.
- Deadline desain memakai nilai kanonis yang sudah dihitung H-2. Tanggal tanpa jam tetap tanggal. Aturan H-12/24 jam memerlukan waktu posting atau jam yang ditetapkan eksplisit. Waktu kejadian, bukan waktu klik selesai, menjadi dasar perbandingan.
- Broadcast konsep bukan bukti kematangan permintaan dengan sendirinya. Dugaan desain terlambat tetap `insufficient` untuk atribusi bila kematangan konsep/follow-up/pengecualian belum cukup. Bukti hilang tidak otomatis dianggap pelanggaran.
- Kesepakatan khusus harus menyebut aturan yang dicakup; tidak mengecualikan seluruh SOP secara diam-diam. Klasifikasi dadakan, pengecualian dan rentang hitung memerlukan fakta/parameter yang dikonfirmasi manusia.
- `0012_sop.sql`: kebijakan per divisi, izin evaluator, konteks manusia, persetujuan bot per versi kebijakan, temuan, observasi, keputusan append-only, follow-up dan snapshot rekap. Observasi yang materi sumbernya kembali ke keadaan lama tetap dapat menjadi kejadian baru; fingerprint bukan identitas permanen observasi.
- `internal/api/sop.go`, `internal/store/sop.go`: model dan pembacaan/penyimpanan kebijakan, konteks serta temuan. Getter temuan memiliki limit 1000; laporan kuartalan lengkap nantinya harus memakai jalur paginasi/streaming atau menyatakan batas, bukan diam-diam menganggap hasil terbatas sebagai total.
- `internal/server/sop_access.go`: `sop.read`/`sop.review` terpisah dari `results.review`. Kadiv asli mengonfigurasi kebijakan dan izin. Role kustom dapat menerima izin SOP eksplisit; 45 tidak otomatis mendapat hak pengesahan hanya karena meninjau tugas.
- Izin evaluator Litbang berlaku hanya pada data evaluasi divisi pemberi izin. BPH dapat diberi izin baca; hak review evaluator hanya untuk Litbang. Keanggotaan evaluator diperiksa lagi pada akses. Grant tersebut tidak membuka Notes atau semua isi Space lain.
- `sop_policy.go`: API GET/PUT kebijakan, daftar/pemberian/pencabutan izin evaluator, dan persetujuan bot oleh evaluator Litbang yang benar-benar berwenang. Usulan Humas dan persetujuan Litbang dicatat terpisah. Pergantian versi kebijakan memerlukan persetujuan baru bila bot hendak dihitung sebagai pemenuhan SOP. Keputusan historis tetap disimpan.
- `sop_context.go`: API konteks permintaan matang, follow-up, final check, dadakan dan kesepakatan. Scope Humas/Mediatek tetap terpisah. Fakta waktu, URL/catatan dan alasan disimpan dengan versi serta aktor; client tidak dapat memalsukan provenance sebagai event WhatsApp.
- Endpoint terdaftar pada `/api/periods/{periodId}/sop-scopes` dan `/api/periods/{periodId}/divisions/{divisionId}/sop/{policy,bot-agreement,evaluators,contexts/...}`. Ini kode endpoint, bukan bukti telah dijalankan.

## Kelanjutan wajib

1. Worker pembentukan/pembaruan observasi dan temuan dari kalender, receipt konsep, bukti desain terverifikasi, konteks manusia serta follow-up yang benar-benar terkirim. Source fingerprint dibandingkan dengan keadaan sumber terakhir; jangan membuat observasi baru tiap tick atau mengganti keputusan manusia otomatis. Sumber berubah sesudah keputusan harus menyalakan `needs_recheck`, sambil mempertahankan keputusan terakhir dalam laporan.
2. API daftar/filter, detail/observasi, keputusan dan ralat temuan. Koreksi atribusi harus terpisah dari status tugas; sumber keputusan dan alasan tidak dihapus. Bukti/temuan manual diperlukan untuk bagian visual/isi yang tidak bisa dinilai evaluator waktu.
3. Follow-up H-4 dengan judul, mention PJ desain dan tautan konsep melalui bot Humas di Medhum, dengan snapshot semantik kalender. Pengiriman diterima bridge belum membuktikan semua SOP dipenuhi; hitung sebagai pemenuhan hanya jika kesepakatan versi itu disetujui. Policy/kalender/grup yang berubah membatalkan job antre yang usang. Jangan menggandakan pesan yang sudah diterima hanya karena policy disimpan lagi tanpa perubahan materi pesan.
4. Rekap mingguan hanya ke grup divisi masing-masing; temuan/keputusan tidak diumumkan ke Medhum. Bedakan pending, confirmed dan perubahan sumber yang perlu diperiksa lagi. Quarter harus memakai rentang eksplisit. Snapshot rekap/pengiriman harus dapat ditelusuri.
5. Klien dan UI konfigurasi/konteks/izin/tinjauan/riwayat/rekap. Pemilih evaluator perlu metadata khusus Litbang/BPH yang tidak membuka Space divisinya. Endpoint konfirmasi bot perlu metadata kemampuan Litbang khusus, bukan sekadar `CanReview` umum.
6. Bahasa respons inti 4B; sisa 4C terhadap contoh nyata; 5A/5B, lampiran biner/backup yang sesuai PRD, mobile 6A/penilaian 6B; kemudian build/test/perbaikan dan audit acceptance menyeluruh.

Tidak ada kebijakan SOP, grant, persetujuan bot, pairing, kiriman WhatsApp atau perubahan produksi yang diterapkan agen. Catatan parameter terbuka PRD tetap memerlukan konfigurasi manusia sebelum operasional nyata.
