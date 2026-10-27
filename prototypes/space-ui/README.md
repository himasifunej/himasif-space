# Space — UI draft 03

Latest gallery: `http://localhost:5173/refined-gallery.html?draft=03`.
WhatsApp settings: `http://localhost:5173/refined.html?view=settings&settings=whatsapp`.
The MyGo preview remains available through `bash run.sh desktop refined`.

The sidebar stays fixed with responsive spacing, its collapse control is below Appearance, and the two duplicate header controls are removed. Scrolling content uses matching custom scrollbars. All select/date/time/color menus, alert confirmations and snackbars are now Space components.

Division logos are fetched from the HIMASIF profile assets (see `frontend/assets/SOURCES.md`). General settings supports an in-memory custom logo. Medhum starts with a paired Humas/Mediatek mark; that combination is an exploration rather than an official new logo.

WhatsApp settings cover connection/QR, account type, message label/style, groups, reminder/SOP rules, custom commands with stickers and delivery history. Humas serves Humas + Medhum; Mediatek serves its own group. Role 45 management and member/read-only states can be reviewed via `role=45` and `role=member`; `division=Humas` shows the Humas sample. These are UI simulations, not server authorization.

Six new self-contained artifacts, screenshots and QA observations are in `artifacts/refined-03`; all six HTML files are downloadable as `frontend/downloads/Space-Draft-03.zip`. Original A–E and draft 02 exports are preserved. The Canvas ID is `a9bd699c07c9`.

The production repository continues to use native MyGo Go components. This isolated HTML/MyGo wrapper is a visual reference. Real Baileys login, group sync, scheduling, backend persistence and delivery are not implemented here. All data edits reset on reload.

---

# Space — Combined UI draft 02

The latest draft combines the user’s choices from A/B/C/E. Open `http://localhost:5173/refined-gallery.html`, or run `bash run.sh desktop refined` for the MyGo launcher. The original A–E remain available below.

The six new portable HTML artifacts in `artifacts/refined-02` show one application: home, board, docked task detail, calendar, appearance, and light mode. Every standalone HTML carries the complete in-memory interactions. Download all six from `frontend/downloads/Space-Draft-02.zip`.

- A workspace sidebar collapses to B’s icon rail, with Ctrl B.
- B’s header has a visible search field and Ctrl K.
- A’s home links directly to the board and E’s calendar.
- Cards drag between columns and reorder within a column; a menu provides a keyboard alternative.
- C’s right inspector takes space from the board, with no task modal or backdrop. Its divider can be dragged.
- A persistent bottom bar shows connection, local pending changes and shortcuts.
- Appearance supports dark/light/system, five accent presets and a custom color, density, sidebar, panel width, motion and Ace visibility.

This is a disposable visual reference, using fixture data and memory only. The Kiro Sesi 1A foundation uses **native MyGo Go components** for desktop and WebAssembly. Its production UI must implement these validated layouts with that component system; the HTML wrapper here is an isolated preview, not a migration of the production app.

Manual observations and platform limits are in `artifacts/refined-02/QA.md`; exports are listed in that folder’s `manifest.json`. No actual WhatsApp messages or cloud writes occur.

---

# Space — UI exploration A–E

Isolated, disposable prototype for choosing the desktop UI of Space HIMASIF. It does not connect to Kiro's backend, WhatsApp, Trello, Sheets, or production data. All edits are in memory and reset on reload.

The desktop launcher uses **MyGo v0.7.1** and embeds the exact HTML/CSS/JavaScript shown by the browser preview. No framework or frontend build pipeline is selected on behalf of Kiro's foundation.

## Run

```sh
bash run.sh preview
```

Open `http://localhost:5173/gallery.html` for the five-option gallery, or `http://localhost:5173/?variant=A` for the interactive preview. Switch with the floating A–E controls or the left/right arrow keys. Arrow keys are left alone while editing text. Each variant has a direct URL via `?variant=A` through `?variant=E`.

For the actual MyGo desktop window on a supported desktop with Go 1.27.3+ and the system webview:

```sh
bash run.sh desktop A
```

On Windows, use `go run . -variant A`. A Windows executable can be cross-compiled with `bash run.sh build-windows`. A successful cross-build verifies compilation; runtime toast/tray support is outside these UI alternatives and must be tested on Windows later.

## Alternatives

| Key | Name | Primary structure |
|---|---|---|
| A | Ruang | enowX-inspired workspace sidebar, division task overview, agenda and Ace |
| B | Orbit | Compact icon rail, workspace tabs and four-column task board |
| C | Alur | Persistent task list and detailed task view side by side |
| D | Fokus | Calm personal day, one priority, quick notes and a contextual timeline |
| E | Ritme | Horizontal navigation, weekly calendar and today's agenda |

All use the same warm charcoal/parchment palette, Indonesian copy, fixture people/tasks, and PRD concepts. The Space mark and Ace illustration here are exploration assets, not final branding.

Try: opening tasks, changing Space, filtering tasks, creating a sample task, notes/templates, meeting attendance, ping simulation, reminder privacy, offline indicator, audio preview, and Ace floating/edge/hidden modes. Side effects remain in memory; success copy labels remote operations as simulation.

To inspect state, use `window.__SPACE_PROTOTYPE_STATE__`; each render also logs the state in the browser console. Prototype controls are confined to this directory, not a production route or app.

The Canvas artifacts, standalone HTML exports, screenshots and QA notes are recorded in `artifacts/manifest.json` when generated. Keep all five alternatives until the user picks a winner or a combination. Do not promote prototype mutations or simulated permission checks to production.

## Exports and QA

The five standalone HTML artifacts are also downloadable from the gallery. A bundled `Space-ABCDE.zip` contains all five. Canvas holds five designs under **Space — Pilihan UI A–E**. See `artifacts/QA.md` for checks performed and Windows runtime limitations.
