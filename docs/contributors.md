# Format `contributors.json`

[`contributors.json`](../contributors.json) di root repo adalah daftar semua yang ikut membangun Space. Halaman **Kredit** di aplikasi membacanya, jadi formatnya dijaga stabil. Skema mesinnya: [`contributors.schema.json`](contributors.schema.json).

## Menambahkan diri

1. Buat branch `docs/contributor-<id>` dari `main`.
2. Tambahkan satu objek di **akhir** array `contributors`. Jangan mengurutkan ulang atau mengubah entri orang lain.
3. Salin berkas yang sama ke `internal/appui/contributors.json`: `cp contributors.json internal/appui/contributors.json`. Aplikasi menanam salinan itu agar Kredit tetap tampil tanpa koneksi, dan `TestContributorsEmbedded` gagal bila keduanya berbeda.
4. Periksa: `python3 scripts/check_contributors.py` (memeriksa format dan kesamaan salinan).
5. Buka PR berjudul `Kontributor: <nama>`. Sebutkan kontribusimu (PR, desain, uji pilot, dll.) di deskripsi.

Contoh entri:

```json
{
  "id": "aruna",
  "name": "Aruna Wicaksana",
  "role": "Desain antarmuka",
  "kind": ["design", "pilot"],
  "affiliation": "Mediatek HIMASIF 25/26",
  "period": "2025/2026",
  "github": "aruna-w",
  "links": [{ "label": "Portofolio", "url": "https://contoh.id" }]
}
```

## Field

| Field | Wajib | Isi |
|---|---|---|
| `id` | ya | Pengenal tetap, huruf kecil/angka/`-`, maks. 39 karakter, unik. Biasanya username GitHub dalam huruf kecil. **Tidak pernah diubah** setelah masuk. |
| `name` | ya | Nama yang ditampilkan, maks. 80 karakter. |
| `alias` | tidak | Nama panggilan, ditampilkan di samping nama. |
| `role` | ya | Peran dalam satu frasa pendek berbahasa Indonesia, maks. 60 karakter (mis. "Pengembang utama", "Desain antarmuka", "Penguji pilot Humas"). |
| `kind` | ya | Satu atau lebih kategori (lihat di bawah); dipakai aplikasi untuk ikon dan pengelompokan. |
| `affiliation` | tidak | Divisi/organisasi, mis. "Humas HIMASIF 25/26". |
| `period` | tidak | Periode kepengurusan, `YYYY/YYYY`. |
| `github` | tidak | Username GitHub tanpa `@`. Avatar diambil dari sini. |
| `links` | tidak | Paling banyak 4 tautan `{label, url}`; `url` wajib `https://`. |

Kategori `kind`: `lead` (pengembang utama/pemimpin proyek), `code`, `design`, `docs`, `test`, `release`, `ops` (server/deploy), `content` (panduan, copy), `translation`, `idea` (gagasan/kebutuhan), `pilot` (pengguna uji coba yang memberi masukan).

## Janji kestabilan (format 1)

Untuk pembaca seperti halaman Kredit (`internal/appui/kredit.go`, yang membaca salinan tertanam `internal/appui/contributors.json`):

- `format` bernilai `1`. Selama `1`, field yang ada tidak dihapus, tidak diganti nama, dan tidak berubah tipe.
- Field opsional baru boleh ditambahkan. Pembaca **harus mengabaikan** field yang tidak dikenalnya, juga nilai `kind` yang tidak dikenalnya.
- Perubahan yang tidak cocok dengan aturan di atas menaikkan `format` menjadi `2`; pembaca yang hanya mengenal `1` sebaiknya menampilkan daftar dari versi terpasang atau tidak menampilkan apa pun.
- Urutan array adalah urutan tampil. Entri `lead` berada di depan; selebihnya urut waktu bergabung.
- Berkas ini publik. Jangan memasukkan email, nomor HP, atau data pribadi lain.
