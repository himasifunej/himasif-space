# Runbook cutover instans stabil `space.himasif.id` (Sen 30 Nov 2026)

F1-22 · F1-21. Ditulis 26 Nov 2026. **Belum dijalankan.** Semua langkah butuh persetujuan owner lewat GOA. Rencana nomor bot WA ada di [wa-bot-ke-stabil.md](wa-bot-ke-stabil.md).

## Keadaan per 26 Nov 2026

| Hal | Nilai |
|---|---|
| Host | `tiyoouw-studio` (VPS AdvinServers Johor, Ubuntu 26.04, 4 vCPU, 7,8 GB RAM), hanya lewat Tailscale. Pindah dari tc-vps karena RAM tc-vps habis. |
| Cara jalan | Unit systemd `--user` milik `enowx`, meniru instans uji di tc-vps: `himasif-space-stable-db` (PostgreSQL 16 PGDG, klaster privat `127.0.0.1:55490`), `himasif-space-stable-api` (`127.0.0.1:8790`), `himasif-space-stable-backup.timer` (19:00 UTC = 02:00 WIB). Linger aktif. |
| Path | rilis `~/.local/opt/himasif-space-stable/releases/<VER>-<sha12>/`, symlink `current`; env `~/.config/himasif-space-stable/server.env` (0600); data `~/.local/share/himasif-space-stable/{postgres,attachments,backups}` |
| Versi terpasang | `1.0.0-rc.1` (`663f488`), artefak build yang sama dengan instans uji, `SHA256SUMS` cocok. **Hanya pengisi**: instans stabil menerima rilis kanal **stable** saja, jadi sebelum dibuka (Sab 5 Des) wajib diganti build `1.0.0`. |
| DB | Kosong: skema sudah dimigrasi, 0 akun, 0 periode, 0 divisi. **Belum di-seed.** |
| Env | Rahasia baru (kredensial DB baru, tidak disalin dari uji). `SPACE_ENV=production`, `SPACE_PUBLIC_URL=https://space.himasif.id`, `SPACE_MAIL_DRIVER=disabled`, bridge WA kosong. |
| Tunnel | Tunnel Cloudflare baru `tiyoouw-studio` (`e8c57f56-…`, dikelola lokal), unit sistem `cloudflared.service`, config `/etc/cloudflared/config.yml`. Aturan `space.himasif.id` dan `origin-space.tiyoouw.my.id` **masih `http_status:404`**. |
| DNS | `origin-space.tiyoouw.my.id` (fallback origin Cloudflare for SaaS di zona `tiyoouw.my.id`) → tunnel `tiyoouw-studio`. Sebelumnya → tunnel tc-vps `56423224-…`, yang aturannya juga 404. Custom hostname `space.himasif.id`: status dan SSL **active**. |
| Backup | Harian, terenkripsi age ke penerima backup Space (identitas di vault `space-backup-age-key`), **masih lokal di host yang sama**. Salinan di luar host belum ada (to-do, lihat di bawah). |

**R-03:** `space.himasif.id` tidak pernah diarahkan ke instans uji (`127.0.0.1:8788` di tc-vps, `space.tiyoouw.app`). Rollback juga tidak.

## 0. Gerbang sebelum mulai

- [ ] Owner menyetujui jendela cutover lewat GOA.
- [ ] `systemctl --user status himasif-space-stable-{db,api}` aktif dan `curl -s http://127.0.0.1:8790/api/health` → `"status":"ok","database":"ok"`.
- [ ] Backup terbaru ada di `~/.local/share/himasif-space-stable/backups/` dan `verify` lolos (perintah di bagian 6).
- [ ] Catat baseline, harus tetap sama sesudah cutover:
  ```sh
  for u in https://space.tiyoouw.app/api/health https://news.tiyoouw.com/ https://post.tiyoouw.com/; do
    printf '%s ' "$u"; curl -s -o /dev/null -w '%{http_code}\n' --max-time 15 "$u"; done
  ```
  Per 26 Nov: `200`, `404` (halaman akar newsletter memang 404), `307`.
- [ ] Langkah 1 butuh root di `tiyoouw-studio` (sudo owner). Sudo tanpa password untuk `enowx` sudah dicabut.

## 1. Arahkan aturan 404 ke port stabil

Di `tiyoouw-studio`:

```sh
sudo cp /etc/cloudflared/config.yml /etc/cloudflared/config.yml.bak-$(date -u +%Y%m%d-%H%M)
sudoedit /etc/cloudflared/config.yml
```

Dua aturan yang diubah, aturan terakhir (catch-all) tetap 404:

```yaml
  - hostname: space.himasif.id
    service: http://127.0.0.1:8790
  - hostname: origin-space.tiyoouw.my.id
    service: http://127.0.0.1:8790
  - service: http_status:404
```

Periksa lalu restart:

```sh
sudo cloudflared tunnel --config /etc/cloudflared/config.yml ingress validate
sudo cloudflared tunnel --config /etc/cloudflared/config.yml ingress rule https://space.himasif.id/api/health   # harus 127.0.0.1:8790
grep -n 8788 /etc/cloudflared/config.yml && echo "STOP: port instans uji (R-03)"
sudo systemctl restart cloudflared && sleep 5 && systemctl is-active cloudflared
sudo journalctl -u cloudflared -n 20 --no-pager | grep -E 'Registered|ERR'
```

Tunnel tc-vps tidak disentuh. Aturannya untuk kedua hostname masih 404 dan tidak menerima lalu lintas lagi; boleh dihapus kapan saja, **jangan** diarahkan ke 8788.

## 2. Verifikasi TLS dan custom hostname

```sh
dig +short CNAME space.himasif.id                     # origin-space.tiyoouw.my.id.
dig +short CNAME origin-space.tiyoouw.my.id           # (proxied: tampil IP Cloudflare, bukan CNAME)
curl -sv https://space.himasif.id/ -o /dev/null 2>&1 | grep -E 'subject:|issuer:|expire'
```

- Sertifikat `CN=space.himasif.id`, penerbit CA Cloudflare (26 Nov: Google Trust Services WE1).
- Di dasbor Cloudflare, zona `tiyoouw.my.id` → SSL/TLS → Custom Hostnames: `space.himasif.id` **Active**, sertifikat **Active**, fallback origin `origin-space.tiyoouw.my.id` **Active**. Lewat API (kredensial vault `cloudflare-tiyo`):
  ```sh
  enowx creds use cloudflare-tiyo -- sh -c 'curl -s -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
    https://api.cloudflare.com/client/v4/zones/f38db1a67fac14519443e15ee8daa6e9/custom_hostnames?hostname=space.himasif.id' \
    | jq '.result[]|{hostname,status,ssl:.ssl.status,expires:.ssl.certificates[0].expires_on}'
  ```
- Nilai TXT `_acme-challenge.space` dari 25 Nov berlaku sampai sekitar 9 Des. Bila sertifikat belum active atau perlu diterbitkan ulang, minta nilai baru dan berikan ke owner untuk cPanel. Record `@`, `www`, MX dan wildcard `*.himasif.id` di himasif.id tidak diubah.

## 3. Health dan login

```sh
curl -s https://space.himasif.id/api/health | jq '{status,database,version}'   # ok, ok, versi yang terpasang
curl -s -o /dev/null -w '%{http_code}\n' https://space.himasif.id/             # 200, web Space
curl -s -o /dev/null -w '%{http_code}\n' https://origin-space.tiyoouw.my.id/api/health   # 200, instans yang sama
# login dengan akun yang tidak ada: harus 401, bukan 5xx
curl -s -o /dev/null -w '%{http_code}\n' -H 'Content-Type: application/json' \
  -d '{"email":"tidak-ada@himasif.id","password":"salah-salah-salah","client":"web"}' https://space.himasif.id/api/auth/login
journalctl --user -u himasif-space-stable-api --since -10min --no-pager | grep -E '"level":"(ERROR|WARN)"|panic'
```

- Health menyebut versi stabil; bila masih `1.0.0-rc.1`, instans boleh dirutekan tetapi **belum boleh dibuka** untuk pengguna.
- Alamat klien di log harus IP asli (bukan `127.0.0.1`), tanda `CF-Connecting-IP` sampai.
- Instans uji tetap: `curl -s https://space.tiyoouw.app/api/health` → versi uji, bukan versi stabil.
- **Jangan bagikan `https://space.himasif.id/unduh/windows`** sebelum `stable.json` terbit. Selama kanal stable kosong, server mengambil installer kanal rc, dan build rc memakai `space.tiyoouw.app` sebagai alamat bawaan.

## 4. Admin pertama dan organisasi

Syarat: build yang terpasang sudah memuat **F0-16** (seed membuat periode 2026/2027). Build `1.0.0-rc.1` masih membuat periode 2025/2026, jadi bila F0-16 belum terpasang pada 30 Nov, langkah ini ditunda; langkah 1–3 tetap bisa jalan.

1. Siapkan email admin organisasi `@himasif.id` dan kata sandi baru. Simpan di vault enowX (mis. `himasif-space-stable-admin`), jangan ditulis di catatan atau chat.
2. Isi admin hanya selama seed, lalu hapus dari env:
   ```sh
   systemctl --user stop himasif-space-stable-api
   enowx creds use himasif-space-stable-admin -- sh -c '
     SPACE_ADMIN_EMAIL="$USERNAME" SPACE_ADMIN_PASSWORD="$PASSWORD" \
     ~/.local/opt/himasif-space-stable/current/space-server -env-file ~/.config/himasif-space-stable/server.env seed'
   systemctl --user start himasif-space-stable-api
   ```
   Log `seeded` harus menyebut `period_added=true`, `divisions_added`, `admin_added=true`. Seed aman diulang; yang sudah ada tidak dibuat lagi.
3. Periksa DB:
   ```sh
   PGPASSWORD="$(cat ~/.config/himasif-space-stable/pg-owner-password)" /usr/lib/postgresql/16/bin/psql -h 127.0.0.1 -p 55490 -U space_owner -d space \
     -c "select label,starts_on,ends_on,status from periods" -c "select count(*) akun from users"
   ```
   Tepat satu periode `2026/2027` aktif (1 Des 2026 – 30 Nov 2027) dan satu akun. Tidak ada akun demo atau data `[DUMMY]`.
4. Masuk di `https://space.himasif.id` sebagai admin dan ganti kata sandi. Periksa Pengaturan → Organisasi: divisi lengkap, Medhum berupa Space bersama.
5. Email: isi `SPACE_MAIL_DRIVER=smtp`, `SPACE_SMTP_*` dan `SPACE_MAIL_FROM` (alamat `@himasif.id`) di `server.env` dari vault, restart api, lalu `space-server -env-file … mail-test -to <alamat owner>`. Undangan dan pemulihan akun butuh email. Endpoint SNS SES untuk `space.himasif.id` mengikuti [ses-email.md](ses-email.md).
6. Undangan kadiv dan BPH baru **baru dikirim Min 6 Des** setelah 1.0.0 terpasang dan gerbang F1-22 lolos (gladi pulih F0-11, alarm F0-13).

## 5. Rollback

Pilih yang terkecil yang cukup.

- **Jalur web bermasalah (aturan tunnel):** kembalikan kedua aturan ke `http_status:404`, atau pulihkan cadangannya, lalu restart:
  ```sh
  sudo cp /etc/cloudflared/config.yml.bak-<ts> /etc/cloudflared/config.yml && sudo systemctl restart cloudflared
  curl -s -o /dev/null -w '%{http_code}\n' https://space.himasif.id/api/health   # 404
  ```
  Jangan pernah rollback ke instans uji (R-03).
- **Host `tiyoouw-studio` mati atau tunnel-nya rusak:** arahkan CNAME `origin-space.tiyoouw.my.id` (zona `tiyoouw.my.id`, proxied) kembali ke `56423224-b7db-4012-a3e5-47525130025a.cfargotunnel.com`. Tunnel tc-vps masih memuat aturan 404 untuk kedua hostname, jadi hasilnya kembali seperti 26 Nov: 404, bukan instans uji.
- **Versi server bermasalah:** arahkan `current` kembali ke rilis sebelumnya dan restart api (migrasi hanya aditif):
  ```sh
  ln -sfn ~/.local/opt/himasif-space-stable/releases/<VER-LAMA> ~/.local/opt/himasif-space-stable/current
  systemctl --user restart himasif-space-stable-api
  ```
- **Data rusak:** jangan menimpa DB live. Pulihkan ke DB baru dengan `space_ops.py restore` (lihat [recovery.md](recovery.md)), verifikasi, lalu alihkan `SPACE_DATABASE_URL` dengan persetujuan owner.
- **Seed salah periode:** sebelum ada data pengguna, hentikan api, `dropdb`/`createdb` `space` dengan role `space_owner`, start api (migrasi ulang), lalu seed dengan build yang benar.

## 6. Deploy berikutnya ke instans stabil

Sampai F1-14 menambahkan parameter instans ke skrip deploy, deploy stabil dilakukan manual dan **hanya dari rilis kanal stable** (mis. `1.0.0`):

```sh
V=1.0.0-<sha12>; R=~/.local/opt/himasif-space-stable/releases/$V
mkdir -p $R && <salin artefak build: space-server, web/, SHA256SUMS> && (cd $R && sha256sum -c SHA256SUMS)
systemctl --user start himasif-space-stable-backup.service          # backup sebelum deploy
ln -sfn $R ~/.local/opt/himasif-space-stable/current
systemctl --user restart himasif-space-stable-api && sleep 3 && curl -s http://127.0.0.1:8790/api/health
```

Perbarui `release_id` di `~/.config/himasif-space-stable/backup-policy.json`. Verifikasi backup:

```sh
A=$(ls ~/.local/share/himasif-space-stable/backups/*.tar.age | tail -1)
enowx creds use space-backup-age-key -- sh -c "PATH=/usr/lib/postgresql/16/bin:\$PATH python3 \
  ~/.local/opt/himasif-space-stable/operations/space_ops.py --policy ~/.config/himasif-space-stable/backup-policy.json \
  verify --archive $A --identity \"\$KEY_FILE\" --expected-sha256 \$(jq -r .sha256 $A.json)"
```

## To-do yang tersisa (bukan bagian cutover 30 Nov)

- **Backup di luar host** (wajib sebelum DB stabil berisi data nyata, F0-11): tarik harian ke homeserver-girli seperti instans uji, atau restic ke R2 (`r2-tiyoouw` + `restic-space`), dengan retensi 14 harian + 8 mingguan. `prune` belum dijadwalkan.
- Alarm dari luar host (F0-13) untuk `https://space.himasif.id/api/health` dan backup yang tidak datang.
- F1-14: unit dan skrip deploy berparameter instans di repo.
- Hapus aturan 404 `space.himasif.id`/`origin-space.tiyoouw.my.id` di tunnel tc-vps setelah cutover stabil.
