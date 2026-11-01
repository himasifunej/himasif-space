# Space identity and interface assets

These assets were copied without visual changes from the approved Space UI
prototype, revision 03.1, on 27 October 2026:
`prototypes/space-ui/frontend` in the local UI draft project.

- `space.svg`: Space's prototype brand mark, created for HIMASIF Space. Display
  using `ui.Image` to preserve the warm gold cube and dark rounded square.
- `mediatek.png`: the Mediatek division logo displayed by
  [HIMASIF's profile](https://himasif.id/app/profil), originally fetched from
  <https://v2.himasif.id/storage/25/5.-MEDIATEK.png>.
- `humas.png`: the Humas division logo displayed by the same profile, originally
  fetched from <https://v2.himasif.id/storage/24/4.-HUMAS.png>.
- `psdm.png`: PSDM division logo from <https://v2.himasif.id/storage/20/1.-PSDM.png>.
- `kaderisasi.png`: Kaderisasi division logo from
  <https://v2.himasif.id/storage/22/2.-KADERISASI.png>.
- `litbang.png`: Litbang division logo from
  <https://v2.himasif.id/storage/23/3.-LITBANG.png>.

Division assets are embedded for offline display; their artwork and colors are
preserved. Files are fetched from the profile above at full size and scaled down
by `ui.Image`. Lookups use the slugs written by `internal/seed` (`humas`,
`kader`, `litbang`, `mediatek`, `psdm`); `kaderisasi` is accepted as an alias.
BPH has no published emblem and keeps the neutral mark. Unknown division slugs
deliberately return no logo, and a shared space such as Medhum does not inherit
either division's identity.

`../icons.go` contains the prototype's local interface SVG paths from `app.js`
and `refined.js`. Settings and WhatsApp use the refined paths approved after the
prototype's icon alignment review:

- Settings: [Lucide settings](https://github.com/lucide-icons/lucide/blob/main/icons/settings.svg),
  ISC license, copyright (c) 2026 Lucide Icons and Contributors.
- WhatsApp: [Tabler brand-whatsapp](https://github.com/tabler/tabler-icons/blob/main/icons/outline/brand-whatsapp.svg),
  MIT license, copyright (c) 2020–2026 Paweł Kuna.

Other interface paths are retained from the local Space prototype. Icons use a
24 × 24 viewBox, round stroke caps/joins, and MyGo-supported paths, rectangles,
circles, ellipses, and transforms. They are parsed once and tinted by `ui.Icon`.

## Third-party icon licenses

Upstream license texts are included alongside the assets, fetched on
27 October 2026:

- [Lucide license](LICENSE-lucide.txt), from
  <https://raw.githubusercontent.com/lucide-icons/lucide/main/LICENSE>.
- [Tabler license](LICENSE-tabler.txt), from
  <https://raw.githubusercontent.com/tabler/tabler-icons/main/LICENSE>.
