# Catatan rilis Space

Versi mengikuti aturan sebelum rilis publik:
- **Patch** naik di setiap pembaruan.
- **Minor** naik untuk pembaruan besar.
- **1.0.0** untuk rilis publik.

## 0.4.1 — 2026-11-10 · Izin notifikasi dan tur yang pas per role

- **Langkah baru "Izinkan notifikasi"** di checklist "Langkah pertama". Langkah ini mengikuti izin di perangkat yang sedang dipakai:
  - Di web, mengikuti izin browser.
  - Di aplikasi desktop, mengikuti setelan notifikasi Windows.
  - Langkah tercentang sendiri bila notifikasi sudah diizinkan.
  - Bila diblokir, langkah ini menampilkan cara mengizinkannya lagi.
- **Notifikasi uji langsung** setiap kali izin baru diberikan: "Notifikasi aktif ✓".
- **Pesan uji di WhatsApp** setelah nomor pertama kali tertaut: "Tersambung ✓". Pesan ini dikirim sekali per akun, lewat chat pribadi.
- **Lapisan notifikasi bersama** (`appui.Notifier`): popup browser di web dan notifikasi Windows di desktop. "Dynamic island" untuk Space desktop bisa ditambahkan nanti sebagai presenter tambahan (`MultiNotifier`).
- **Perbaikan tur**: tur hanya menyorot tab yang terlihat oleh akun itu.
  - Anggota tidak punya tab Ping, jadi langkah WhatsApp menyorot tab Bot WhatsApp.
  - Langkah yang tabnya disembunyikan dilewati.
  - Keterangan "tidak tampil di layarmu" sudah dihapus.


Rangkuman 0.3.1 sampai 0.4.0.

### Onboarding dan Panduan (0.4.0)
- **Sambutan setelah bergabung lewat undangan.** Setelah itu muncul checklist **Langkah pertama** di Beranda dengan empat langkah: Tautkan WhatsApp, Lengkapi profil, Lihat tugasmu, Kenali tab. Setiap langkah tercentang sendiri dan checklist bisa disembunyikan. Progres disimpan per akun di server.
- **Tur sorotan 5 langkah sesuai role** (Anggota, 45, Kadiv). Tur bisa dilewati, tampil sekali, dan bisa diulang dari menu profil.
- **Halaman Panduan dan tombol ?.** Tombol ? membuka artikel untuk halaman yang sedang dibuka di panel samping.
  - Ada 12 artikel bahasa sederhana: board, catatan rapat, ping/bot WA, anggota dan role, 45, serah terima, pergantian periode.
  - Isinya bisa diganti di server tanpa rilis lewat `SPACE_GUIDES_DIR`.
- **Empty state yang membimbing** di board, catatan, dan anggota.
- **Kartu serah terima untuk kadiv baru.**
- **Dialog Profil** untuk mengubah nama tampilan.
- **Gerak halus 150–300 ms** di tur, checklist, panel, dan tombol. Pengaturan kurangi gerak dihormati.
- **Migrasi 0030** (`user_onboarding`). Akun yang sudah ada tidak mendapat onboarding, tetapi tetap bisa membuka Panduan dan mengulang tur.

### Akun, undangan, dan WhatsApp (0.3.7–0.3.9)
- **Undangan lewat NIM**: undangan dikirim ke `NIM@mail.unej.ac.id` lewat Amazon SES.
- **Halaman undangan yang tidak berlaku** sekarang menjelaskan sebabnya.
- **Tautkan WhatsApp** dari Space:
  - Kode dikirim ke bot divisi lewat tautan wa.me.
  - Ada status tertaut.
  - Nomor yang terikat ke akun lain bisa dipindah dengan **PINDAH**.
  - Balasan bot lebih bersih.

### Catatan rapat (0.3.2–0.3.7)
- **C1–C3**: blok terstruktur, autosave sebagai pengganti edit live, kunci editor yang bisa kedaluwarsa, dan interaksi blok.
- **Gaya C**: panel kanan bertab, bilah tipis, dan menu / ringkas.
- **G3**: mode Edit | Baca, serta urungkan/ulangi untuk seluruh dokumen.
- **Timer rapat lokal (T1)**: durasi agenda, pil yang menempel, jeda, +5 mnt, dan agenda berikutnya.
- **Mention (C4)**: `@` untuk anggota dan tugas.
- **Action item (C4)**: PJ, tenggat, dan **Jadikan tugas**, yang mengisi `taskId` dan menampilkan chip #T.

### Anggota (0.3.2–0.3.5)
- **Halaman Anggota baru (A1)**: tabel dengan role, beban kerja, dan status WhatsApp.
- **Drawer Ubah akses (A2)**.
- **Mengeluarkan anggota dengan aman (A3)**.

### Server dan operasi
- **Pilot pindah dari aws-vps ke tc-vps** (2026-11-09).
- **Build dilakukan di PC.** Deploy memakai `deploy.sh` dengan urutan `--dry-run`, `--stage`, lalu `--go`.
- **`make server` mencap versi server (0.3.1).**
