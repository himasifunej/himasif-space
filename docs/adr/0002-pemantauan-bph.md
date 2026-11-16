# ADR 0002 — Pemantauan organisasi oleh BPH (`org.monitor`)

Tanggal: 16 November 2026, WIB. Status: **berlaku**. Menerapkan keputusan owner yang menutup `OPEN-15` (PRD §4.4 MON-01–08, [Register keputusan](../decision-register.md)).

## Keputusan

| Area | Keputusan |
|---|---|
| Penyimpanan | Tabel baru `monitor_grants` (migrasi `0036`, aditif): satu izin aktif per akun per periode, `revoked_reason` `admin` atau `period_closed`. Server lama mengabaikannya |
| Siapa | Hanya admin organisasi yang memberi dan mencabut; penerima harus anggota aktif divisi BPH pada periode itu. Izin melekat ke akun, bukan ke role, jadi pengganti tidak mewarisinya |
| Di mana dihitung | Di dua tempat semua akses lain dihitung: `spaceViews` (akses per Space, `access = "monitor"`) dan `loadScope` (akses per divisi). Akibatnya setiap endpoint yang sudah memeriksa kemampuan ikut menegakkan izin tanpa salinan aturan |
| Kemampuan | `authz.MonitorCaps`: `space.read`, `members.read`, `notes.read`, `sop.read`. Tidak ada `tasks.manage`, `comments.write`, `notes.manage` atau kemampuan administrasi |
| Data pribadi | Reminder pribadi dan draf catatan tidak pernah menjadi data divisi: endpoint-nya memeriksa pemilik, bukan kemampuan Space, sehingga izin pemantauan tidak mencapainya. Diuji di `monitor_test.go` |
| Kanal | Izin hanya berlaku untuk permintaan API ber-sesi (`principal.monitor` diisi di `auth.go`). Command WA, worker konektor dan penjadwal membuat principal tanpa izin ini |
| Sidebar | `GET /api/periods/{id}/spaces` tidak menyertakan Space pantauan kecuali `?monitor=1`, agar sidebar pemantau tetap berisi Space miliknya sendiri |
| Audit | Setiap permintaan yang berhasil dan membuka divisi lewat izin ini menulis satu baris `monitor.read` per divisi (setelah respons, di luar transaksi baca). Tindakan menulis `monitor.act` di transaksi tindakan itu sendiri (dari `record`). `monitor.grant` dan `monitor.revoke` dicatat saat diberi/dicabut. Admin membacanya di `GET /api/periods/{id}/monitors/audit` |
| Periode | `MonitorActive` mengabaikan periode tertutup, dan `closePeriod` mencabut semua izin periode itu dalam transaksi yang sama |
| Jenis rapat | Migrasi `0037`: `meetings.kind` (`division`, `medhum`, `bph`, `pengurus`). Tanpa `kind`, perubahan rapat mempertahankan jenisnya, jadi klien lama aman. Timeline lintas divisi: `GET /api/periods/{id}/monitor/meetings` |
| Ringkasan mingguan | Migrasi `0038`: `monitor_digest_preferences` (bawaan: aktif, WA dan email), `monitor_digest_runs` (satu per orang per minggu, kunci terhadap pengiriman ganda), dan `wa_outbox_group_or_dm` kini menerima `monitor_digest`. Berjalan pada tick penjadwal; dikirim Senin mulai 07.00 zona periode, tidak menyusul bila Senin terlewat. WA = pesan pribadi lewat bot aktif periode itu ke nomor yang tertaut (bukan LID), worker WA memeriksa ulang izin sebelum mengirim. "Selesai pekan lalu" memakai `updated_at` tugas yang `done` (tugas tidak menyimpan waktu selesai) |

## Alasan

Menambah cabang di `spaceViews`/`loadScope` membuat izin pemantauan mengikuti aturan yang sama dengan anggota, arsip dan admin, termasuk periode tertutup dan Space bersama. Menulis audit per permintaan (bukan per objek) membuat daftar tugas tetap satu baris log per divisi, tetapi tetap mencatat setiap akses.

## Konsekuensi

- Pemantau yang memakai aplikasi lama tetap bisa membuka divisi lewat endpoint yang ada; layar pemantauan baru ada di aplikasi versi ini.
- Satu baris audit per divisi per permintaan: halaman yang memuat ulang sering menambah log. Tidak ada penggabungan, karena keputusan owner meminta setiap akses tercatat.
- Migrasi `0036`–`0038` (izin, jenis rapat dan ringkasan mingguan; nomor 0034–0035 sudah dipakai main) bersifat aditif dan aman dijalankan pada pilot tc-vps yang sedang hidup.
