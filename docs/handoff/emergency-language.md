# Spreadsheet darurat dan bahasa respons bot

Checkpoint 29 Oktober 2026. Scope aktif tetap seluruh tahap nonmobile, kemudian build/test di akhir dan paket unduhan. Pengguna meminta agen utama lanjut sendiri setelah OpenCode2 AWS berhenti. Tidak ada agent/deployment produksi yang menjadi dependensi checkpoint ini.

## Spreadsheet darurat

API/klien/parser/UI Darurat kini ditulis. Snapshot menyimpan baseline server, ID asli, waktu/aktor dan data minimal tugas yang berizin. CSV tidak mengekspor Notes/reminder privat ke sheet divisi. Pratinjau membaca state/akses terbaru; progres terbatas todo/doing hanya diterapkan dengan versi yang masih sesuai dan konfirmasi manusia. Konflik, penyerahan, approval, PJ/deadline dan tugas selesai/dibatalkan masuk pemeriksaan manual. Tindakan tinjauan manual/abaikan beralasan tidak mengubah tugas atau mengesahkan SOP. Receipt baris mencegah pengulangan; waktu laporan sheet tidak menjadi waktu bukti kanonis. Detail, kontrak dan batas ada di [prosedur darurat](../operations/emergency-spreadsheet.md). Migrasi `0016_emergency_snapshots.sql` belum diterapkan.

## Bahasa bot

Pilihan bahasa kini dibatasi eksplisit pada Indonesia (`id`, default) dan English (`en`) untuk pesan inti yang benar-benar didukung. UI menyediakan pilihan yang sesuai; tidak lagi menyimpan label bahasa arbitrer sambil mengklaim responsnya telah berubah. Preset Madura tetap opsi yang dapat dikembangkan sesuai contoh bahasa nyata, bukan default wajib organisasi atau terjemahan yang dikarang pada checkpoint ini. Sapaan, nama, template dan custom command tetap dapat diisi sesuai gaya divisi.

Katalog `whatsapp_messages.go` dipakai pada help, link identitas, daftar/detail/status tugas, penyerahan/done, bantuan/delegasi, stiker, ringkasan, rekap/eval, follow-up dan respons konsep/bukti. Error command umum diterjemahkan dari pesan generated yang tepat; error shared yang tidak ada dalam katalog mendapat penjelasan kegagalan/izin/konflik dalam English dengan kode dan arahan memeriksa Space. Aktor/nama/judul/catatan pengguna/isi custom command tidak diterjemahkan dengan pencarian-penggantian pada teks bebas. Validasi/otorisasi inti tidak berubah.

`0017_delivery_language.sql` menambah descriptor message key dan argumen literal untuk notifikasi generated. Scheduler menandai pengingat/overdue, tugas menandai jenis pemberitahuan, rapat menandai create/change/cancel. Proyeksi WA memakai descriptor agar teks pengguna yang kebetulan berawalan kata Indonesia tidak diubah. Ping bebas dan template/custom command dipertahankan. Pesan yang sudah berada pada outbox mempertahankan payload/jurnal aslinya; perubahan bahasa berlaku untuk antrean baru, bukan mengirim ulang pesan lama dengan isi lain.

Bahasa tampilan aplikasi tetap Indonesia. Jam/zona, tanggal tanpa jam, ID, batas izin, pemisahan penyerahan/pencocokan/publikasi/SOP dan receipt transport tetap sama. Template wajib mempertahankan `{body}`. Verifikasi bahasa yang tersimpan dari data legacy non-id/en perlu dimasukkan ke pemeriksaan migrasi/pilot; fallback pembacaan bahasa lama tetap Indonesia sampai pengelola menyimpan pilihan yang didukung.

## Belum dibuktikan

Semua kode pada checkpoint ini belum build/test/migrasi/runtime. Belum mengirim pesan atau memakai sumber WA/Google nyata. Belum mengklaim acceptance 4B/4C/5A lulus. Masih perlu observabilitas operasional/pilot, persiapan distribusi/update/rollback dan paket unduhan, kesesuaian parser dengan contoh broadcast nyata, kemudian verifikasi menyeluruh seluruh perubahan 2C–5B. Tidak ada pekerjaan mobile yang ditambahkan.
