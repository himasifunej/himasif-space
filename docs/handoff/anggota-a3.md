# Tahap A3: keluarkan anggota dengan aman, dan baris tetap di tempat setelah simpan

Branch `feat/anggota-a3` dibuat dari `main` (8c63805, 0.3.3). Rencananya `docs/handoff/anggota-catatan-rapat-rencana.md`, tahap A3, ditambah perbaikan UX dari uji 0.3.3. Belum di-push dan belum di-deploy.

## 1. Keluarkan anggota (A3)
- Kotak merah **"Keluarkan dari <divisi>"** ada di bagian akhir drawer Ubah akses: "Akses divisi dicabut. Tugas aktifnya perlu dialihkan dulu." dan tombol **"Keluarkan <nama>…"**.
- Tombol itu membuka **dialog** "Keluarkan X dari <divisi>?" yang menyebut akibatnya: akses ke board, catatan dan Bot WhatsApp dicabut segera, sedangkan riwayat komentar dan hasil kerja tetap ada.
- **Keputusan owner**: kalau anggota itu masih PJ tugas aktif (belum selesai atau batal, termasuk yang sudah diserahkan), tombol **Keluarkan anggota diblokir**.
  - Dialog menampilkan "X masih PJ N tugas aktif. Alihkan PJ tugas-tugas itu ke anggota lain dulu…".
  - Tombol **"Lihat tugas X"** menutup drawer dan membuka *Board tugas* yang difilter dengan nama anggota itu.
  - Selama tugas Space belum termuat, dialog menulis "Menghitung tugas aktif…" dan tombolnya juga diblokir.
- Anggota yang masih 45 harus dicopot dari 45 dulu (sudah sejak A2).
- **Batasan**:
  - Hitungan tugas berasal dari tugas Space yang sedang dibuka. Tugas yang hanya ada di Space bersama (Medhum) belum ikut dihitung.
  - Pemblokiran berlaku di klien; server belum menolak `RemoveMember` untuk anggota yang masih punya tugas.
  - Pengalihan massal ditunda sesuai keputusan owner.

## 2. Perbaikan UX setelah "Simpan akses"
- **Baris tetap di tempatnya**:
  - Setelah simpan (atau keluarkan), urutan baris saat itu dikunci (`pinned`), jadi anggota yang role-nya berubah tidak berpindah posisi.
  - Urutan dilepas saat tombol **Muat ulang anggota** ditekan, urutan diganti (**Urutkan**), atau Space diganti/halaman dimuat ulang.
  - Anggota baru muncul di bawah, terurut.
- **Tabel tidak berkedip atau hilang**: memuat ulang divisi yang sama tetap menampilkan data lama beserta pencarian, filter dan urutannya sampai data baru datang. Sebelumnya tabel diganti "Memuat anggota…" dan filter ikut hilang.
- **Pesan tersimpan berupa toast** ("Perubahan tersimpan · Akses X disimpan.", 5 detik) di atas jendela, tidak lagi kotak di atas tabel yang menggeser baris.
- Toast sukses kini memakai ikon centang hijau, dan toast galat memakai ikon peringatan merah. Ini berlaku untuk semua toast Space.

## Berkas
- `internal/appui/members_access_drawer.go`: kotak merah, `memberRemoveDialog`, `memberRemoveState`, `showMemberTasks`.
- `internal/appui/members.go`: `pinned`, `loadMembers` mempertahankan tabel dan toolbar saat memuat ulang divisi yang sama, `memberWrite` mengunci urutan.
- `internal/appui/members_table.go`: `visibleMembers` menghormati `pinned`, `pinMemberOrder`, dan Urutkan melepas kunci.
- `internal/appui/members_view.go`: toast menggantikan kotak notice; Muat ulang melepas kunci.
- `internal/appui/chrome.go`: ikon toast per jenis.

## Test
`members_access_drawer_test.go`:
- `TestMemberRemovalBlockedByActiveTasks`: 2 tugas aktif (todo + diserahkan, yang selesai tidak dihitung) membuat tombol diblokir dan tidak ada panggilan; "Lihat tugas" membuka board terfilter; saat tugas belum termuat, keluarkan juga diblokir.
- `TestMemberRowsStayAfterSave`: setelah Nara jadi Pendamping, urutan dan posisi baris tidak berubah, dan tabel tidak kosong saat memuat ulang. "Muat ulang anggota" mengurutkan lagi. Test ini terbukti gagal kalau kunci urutan dimatikan.
- `TestMemberSortReleasesPinnedOrder`.
- Test A2 disesuaikan: pesan kini berupa toast, dan keluarkan menampilkan "tidak punya tugas aktif".

## Screenshot
Dibuat dengan renderer native, tema terang, data dummy:
- `docs/screenshots/anggota-a3-keluarkan-diblokir.png`
- `anggota-a3-keluarkan.png`
- `anggota-a3-toast.png`
- `anggota-a3-hp-keluarkan-diblokir.png`

## Hasil cek (Windows)
- gofmt bersih.
- vet linux/windows/wasm ok.
- `go test ./...` dengan Postgres lokal: 12 paket ok; 4 test filestore gagal karena cek 0700 di Windows, seperti biasa.
