# Handoff pilot.10 — 4 November 2026

Lanjutkan di AWS pada branch yang memuat file ini. User meminta menghentikan sesi ini dan menyimpan pekerjaan dalam commit; commit bukan tanda release siap. Jaga perubahan lain dan jangan reset/clean proyek atau mengganggu proyek AWS lain. Mobile dan tab drawer berada di luar lingkup saat ini. Jangan deploy/aktifkan produksi, mengaktifkan bridge WhatsApp, pairing, atau mengirim pesan ke pihak ketiga tanpa instruksi baru.

## Pilihan dan implementasi

- Catatan divisi B: daftar dan halaman dokumen. Pintasan awal di paragraf polos mendukung `# `, `## `, `### `, `- `, `* `, `1. `, `[] ` dan `[ ] ` untuk blok baru; ini belum editor Markdown lengkap atau parser tempelan Markdown. Isi rich text yang sudah ada dipertahankan dalam editor lanjutan. Periksa fokus, penulisan berkelanjutan, blok daftar, format kaya, penyimpanan, lock, dan pemulihan sebelum dianggap selesai.
- Kehadiran rapat B: pilih peserta pada daftar kiri, ubah kehadiran dan keterangannya di kanan. Perlu uji simpan, hak akses, dan ruang desktop sempit.
- Snackbar tugas dan reminder berada di atas. Error dan status offline tetap terlihat. User perlu mencoba penyimpanan dan snackbar di Windows.
- Reminder D: daftar kiri ringkas dan formulir kanan lebih lebar; tepi daftar bisa ditarik dan diatur dengan panah keyboard. Pengaturan privasi dapat dibuka dari daftar. Perlu uji langsung membuka, mengubah lebar, menggulir, menyimpan, dan hak baca.
- Form buat tugas dan detail tugas dipindah ke sisi kanan area utama penuh, mulai dari atas, bukan dari tengah board. Pratinjau web berhasil membuka form buat tugas setelah memilih Space Mediatek; pengujian detail, resize, navigasi, desktop sempit dan penyimpanan masih diperlukan.
- WhatsApp C: tampilan status dan ringkasan aktivitas, panduan QR dan pembaruan status saat pairing disiapkan. Bridge live tetap nonaktif; backend baru tidak dideploy, sehingga QR sungguhan, expiry, penyambungan, grup, dan alur lain belum diverifikasi.
- Penjelasan kegagalan notifikasi Windows diperjelas tanpa menampilkan PowerShell CLIXML; izin OS dan pengiriman reminder asli masih perlu diuji di Windows.

## Build dan batas verifikasi

User meminta **jangan menjalankan tes Go**; user akan menguji fitur satu per satu. Build terakhir Windows amd64 dan web berhasil pada AWS dengan `GOMAXPROCS=1 GOFLAGS=-p=1 GOMEMLIMIT=256MiB GOGC=50`; tes Go tidak dijalankan. Jangan menjalankan build/test di STB. Tidak ada klaim semua perubahan lulus QA atau siap release.

Pratinjau terbaru: [installer Windows](https://db.tiyoouw.app/downloads/pilot10-split/space-setup-pilot10-split-amd64.exe) (SHA-256 `7934da05b386c7f45e43fe1e949276aca7417d5b90b08879aa9937079b078f8b`) dan [web](https://db.tiyoouw.app/downloads/pilot10-split/web/). Tautan diperiksa HTTP 200. Ini pratinjau, bukan release; backend live masih pilot.4, dengan data dan kredensial disimpan terpisah. Jangan menyalin kredensial ke repo atau handoff.

Pilihan visual dan catatan revisi lainnya ada di note enowX **Perbaikan Dekstop & Web**. Masih ada pekerjaan lain pada kalender tugas, parity desain, installer, pengaturan tampilan, notifikasi Windows, dan QA; audit status aktual sebelum mengklaimnya selesai. User tidak perlu mengulang feedback.

`dist` pada workspace AWS adalah symlink lama yang rusak menuju penyimpanan STB, bukan file sumber; dibiarkan tidak ter-track. Auto-sync STB↔AWS tetap mati.
