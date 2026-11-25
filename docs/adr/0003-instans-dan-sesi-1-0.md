# ADR 0003 — Instans uji dan stabil, nomor bot bersama, sesi web "Tetap masuk", dan mobile PWA (menuju 1.0.0)

Tanggal: 25 November 2026, WIB. Status: **diterima**; penerapan bertahap menuju 1.0.0. Menerapkan keputusan owner 25 Nov 2026 di [Register keputusan](../decision-register.md) (K-11, K-16, K-18, R-02–R-04, R-09–R-11) dan [PRD](../PRD.md) (§2.3, OPS-01, OPS-11, BOT-12–13, ARC-03). **Merevisi** satu baris [ADR 0001](0001-fondasi-teknis.md): penyimpanan token klien web.

## Konteks

Space masih pilot (0.4.6-rc.9) di `space.tiyoouw.app` dengan DB yang berisi akun demo dan data uji periode 2025/2026. Owner menetapkan rilis stabil 1.0.0 sebelum pengurus baru mulai (paling lambat 7 Des 2026), dengan domain HIMASIF sendiri dan DB yang bersih. Pada saat yang sama Reo (Hermes milik owner) masih melayani grup WhatsApp, dan anggota yang membuka tautan WA di HP selalu berakhir di layar masuk karena token web hanya hidup di `sessionStorage` tab.

## Keputusan

| Area | Keputusan | Asal |
|---|---|---|
| Instans | **Dua instans** di tc-vps, masing-masing dengan port, env, DB, rahasia dan unit backup sendiri: **uji** = `space.tiyoouw.app` (DB uji 2025/2026, akun demo tetap); **stabil** = `space.himasif.id` (DB baru, periode 2026/2027, tanpa akun demo). Data penguji tidak dipindah; penguji diundang ulang | R-02, R-03 |
| Syarat buka stabil | Tidak ada akun demo, data `[DUMMY]` atau kredensial yang pernah ter-commit; rahasia baru (token bridge, kunci state WA, kredensial DB), tidak disalin dari uji; gladi pulih lolos (RPO 24 jam / RTO 4 jam); alarm email aktif | K-03, R-12, K-06 |
| Domain | `space.himasif.id` lewat **Cloudflare for SaaS** (custom hostname di zona `tiyoouw.app`) di atas tunnel yang ada. DNS himasif.id tetap di IDCloudHost; owner menambah `CNAME space`, `TXT _cf-custom-hostname.space`, `TXT _acme-challenge.space` lewat cPanel. Cadangan bila fitur itu tidak tersedia (`A` ke IP tc-vps dengan TLS di server) belum diputuskan | K-18, R-04 |
| Klien desktop | Build 1.0.0 memakai `https://space.himasif.id` sebagai URL API bawaan. Klien kanal rc juga membaca `stable.json`, jadi penguji yang menerima 1.0.0 ikut pindah ke instans stabil; ini sejalan dengan undangan ulang dan perlu diberitahukan | Turunan (F1-21, F1-22) |
| Nomor bot | Bot Space **memakai nomor Reo**. Selama Reo jalan, bridge Hermes dan bridge Space tertaut ke nomor yang sama sebagai perangkat tertaut (multi-device) | R-09 |
| Allowlist grup | Space hanya memproses grup yang dipetakan di Space (awalnya **grup Test**, lalu grup yang dipindahkan); pemetaan itu sudah diterapkan ke bridge sebagai allowlist (`syncWABridgeGroups`, `internal/server/whatsapp.go`). Hermes memakai allowlist sendiri dan tidak merespons di grup yang sudah dipindah. **Cutover per grup**; satu grup tidak pernah dilayani dua sistem | R-10, R-11, OPS-08 |
| Sesi web | **"Tetap masuk di perangkat ini"**: kotak centang di layar masuk, bawaan **mati**. Dicentang → token sesi disimpan di `localStorage` origin itu, sehingga tautan WA di tab baru langsung masuk. Tidak dicentang → perilaku lama (`sessionStorage` tab). Keluar akun selalu menghapus token dari kedua tempat dan membersihkan data perangkat (F0-06). Desktop tidak berubah (credential store OS) | K-11 |
| Mobile | **Tanpa Flutter.** Mobile memakai **PWA** dari build web yang sama sambil menunggu MyGo mendukung mobile | K-16 |

Baris "Penyimpanan token klien" di ADR 0001 kini dibaca: *Desktop: credential store OS; web: `sessionStorage` tab, atau `localStorage` bila pengguna memilih "Tetap masuk"*. Paragraf ADR 0001 yang menyebut "mobile utama tetap Flutter (ARC-03)" tidak berlaku lagi.

## Alasan

**Dua instans, DB stabil kosong.** Membersihkan DB pilot dari akun demo, data rekaan dan riwayat kredensial yang pernah ter-commit lebih sulit dibuktikan daripada memulai DB baru. Pergantian pengurus tidak dijalankan di Space (K-02), jadi DB stabil bisa langsung mulai di periode 2026/2027. Instans uji tetap berguna untuk rc, akun demo dan percobaan tanpa menyentuh data nyata.

**Cloudflare for SaaS.** Instans pilot di tc-vps sudah dipublikasikan lewat tunnel Cloudflare di zona `tiyoouw.app`; DNS himasif.id tidak boleh dipindah, jadi custom hostname adalah cara memakai tunnel yang ada dengan tiga record yang bisa ditambah owner sendiri.

**Nomor bersama dengan allowlist.** Memakai nomor Reo menghindari nomor baru dan pairing ulang di grup, dan grup bisa dipindah satu per satu dengan jalan kembali. Risikonya dua sistem menjawab di grup yang sama; allowlist di kedua sisi dan aturan "satu grup, satu sistem" menutupnya.

**"Tetap masuk" bawaan mati.** Token di `localStorage` terbaca oleh skrip lain di origin yang sama dan bertahan setelah tab ditutup, jadi hanya disimpan bila pengguna memilihnya (PC bersama tetap aman bawaan). Imbangannya: keluar akun membersihkan perangkat (F0-06) dan daftar sesi aktif menyusul (F2-09).

**PWA.** Flutter berarti basis kode kedua untuk satu pengembang. PWA memakai build wasm yang sudah ada; batasannya (notifikasi push dan tray tidak setara desktop) diterima sampai MyGo mobile tersedia.

## Konsekuensi

- Dua proses server dan satu bridge di tc-vps (RAM 1,9 GB, belum diukur untuk dua instans). Bila tidak cukup, instans uji yang dikorbankan (mis. dimatikan di luar jam uji).
- Skrip deploy dan unit systemd perlu parameter instans (F1-14); backup berjalan per instans.
- Selama Reo jalan, Reminder Medhum dari Space tidak dikirim ke grup yang dilayani Reo (BOT-14).
- Sesi web yang tersimpan bisa dipakai siapa saja yang memegang browser itu; pengguna PC bersama tidak boleh mencentang "Tetap masuk", dan panduan pasang (F1-23) menyebutnya.
- `space.tiyoouw.app` tetap hidup sebagai instans uji; tautan lama di README dan dokumen operasi diganti ke `space.himasif.id` saat 1.0.0 terbit, bukan sebelumnya.

## Belum diputuskan

- Instans mana yang menautkan nomor bot setelah stabil dibuka, dan apakah instans uji lalu berjalan tanpa bridge (rekomendasi: nomor ke stabil, uji tanpa bridge).
- **Jalur custom hostname.** Cloudflare for SaaS sudah aktif di zona `tiyoouw.app` untuk newsletter: fallback origin `origin-news.tiyoouw.app`, custom hostname `news.tiyoouw.com` (catatan homeserver, 25 Nov 2026). Fallback origin hanya satu per zona, jadi usulan F1-21 (`space-origin.tiyoouw.app` sebagai fallback origin) akan memindahkan newsletter. Pilihan yang perlu dicek di F1-21, tanpa menyentuh newsletter: (a) *custom origin server* per custom hostname, bila paket akun mengizinkannya; atau (b) tetap fallback origin yang sama, dan tunnel di tc-vps merutekan menurut nama host `space.himasif.id` (Host asli dipertahankan). Belum diverifikasi: kedua pilihan belum dicoba; tunnel newsletter ada di homeserver, sedangkan Space di tc-vps, sehingga (b) hanya jalan bila fallback origin itu menunjuk tunnel yang bisa menjangkau tc-vps.
