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
- **Pengaman di server**, ditambahkan setelah review; celah yang sebelumnya dicatat di sini sudah ditutup:
  - `DELETE …/members/{userId}` menolak dengan **409 `member_has_tasks`** bila anggota masih PJ tugas aktif. Tugas aktif berarti todo, doing atau diserahkan, dalam periode itu, yang dimiliki divisi atau ada di Space mana pun milik divisi, termasuk Space bersama seperti Medhum.
  - Pesan penolakannya jelas, misalnya "Nara Tekno masih PJ 2 tugas aktif (Mediatek 1, Medhum 1). Alihkan PJ tugas itu ke anggota lain dulu, lalu keluarkan." Kalau satu tugas ada di dua Space, pesannya menjadi "… 1 tugas aktif di Medhum dan Mediatek".
  - `data` berisi `MemberActiveTasks {count, spaces:[{spaceId, name, count}]}`.
  - Endpoint baru **`GET …/members/{userId}/active-tasks`** (hak `members.manage`) mengembalikan bentuk yang sama, supaya dialog bisa menampilkan angka server sebelum mencoba mengeluarkan.
- **Klien memakai jawaban server**:
  - Dialog meminta `active-tasks` saat dibuka dan menampilkan jumlah serta Space-nya.
  - Ada tombol "Lihat tugas di <Space>" untuk tiap Space yang boleh dibuka. Tombol ini membuka board Space itu yang sudah difilter ke anggota.
  - Kalau server menolak saat mengeluarkan (misalnya ada tugas baru sejak dialog dibuka), dialog berganti ke angka dari penolakan itu.
  - Hanya kalau server tidak bisa menjawab (server lama atau offline), dialog menghitung tugas Space yang terbuka dan menulis "Dihitung dari Space ini saja; server memeriksa semua Space divisi saat mengeluarkan."
- Sekalian diperbaiki: membuka board Space lain dari dialog dulu mengosongkan filter, karena daftar tugas diikat ulang setelah filter diisi. Sekarang filter diisi setelah daftar diikat.
- Pengalihan massal tetap ditunda sesuai keputusan owner.

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
`internal/server/member_remove_tasks_test.go` (DB):
- `TestRemoveMemberBlockedByActiveTasks` mencakup:
  - tanpa tugas: `active-tasks` kosong dan keluarkan 204;
  - tugas aktif di Space divisi: 409 dengan data 1 tugas di Mediatek;
  - selesai dan batal tidak dihitung;
  - tugas di Medhum (Space lain milik divisi yang sama) menolak, termasuk tugas yang **hanya** ada di Medhum dan tidak terlihat di board Mediatek;
  - tugas diserahkan tetap menolak;
  - setelah semuanya selesai, keluarkan 204;
  - anggota divisi lain tidak boleh bertanya (403).
- `TestMemberTaskPlaces`: kalimat tempat tugas.

`members_access_drawer_test.go`:
- `TestMemberRemovalUsesServerCount`: board terbuka tidak punya tugas, tetapi server menyebut 1 tugas di Medhum, jadi keluarkan diblokir; "Lihat tugas di Medhum" membuka Medhum dengan filter nama.
- `TestMemberRemovalRefusedByServer`: dialog sempat mengizinkan, server menjawab 409, lalu dialog menampilkan angka server dan tidak menutup.
- `TestMemberTasksFromError`.

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
