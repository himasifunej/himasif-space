# Draft 02 — manual verification

This is an isolated, disposable visual prototype, not the production native MyGo UI. Data and edits live in memory; reload resets them.

- Desktop board shrinks when the inspector opens. No task dialog, scrim or backdrop is created; the board remains usable.
- Native drag events moved a card across columns and reordered it before another card in the same column. Six tasks remained, with no duplication. A card's menu also offers status moves.
- Ctrl B collapsed the sidebar; Ctrl K focused the visible search; search returned the matching task and Enter opened it. Ctrl comma opened appearance.
- Light theme, preset and custom accents, and compact density updated both state and rendering. System theme follows the OS media preference.
- Simulated offline mode showed the offline banner, connection state, and local pending-sync label. Actual navigator online/offline events use the same render path.
- Creating a sample task and adding its comment updated the shared in-memory task collection and docked inspector.
- No page-level horizontal overflow at widths 1440, 1100, 780 and 390. Search remained visible; the footer stayed at the viewport bottom. Kanban and calendar scrolling stay inside their own containers.
- Six fresh desktop states were captured at 1440×1000 logical pixels, DPR 2; dark detail, calendar, appearance and light detail were visually inspected. Light-theme secondary copy was darkened for legibility.
- Full HTML exports are self-contained: CSS, JS, inline SVG and fixtures. They have no backend or CDN requirement. The gallery provides HTML and ZIP downloads.

The enowX Browser was used during initial interaction checks. When its app connection stopped responding, final verification and PNG capture used a temporary rootless headless Chromium on this host. No system packages or production services were changed. Canvas snapshot HTML is prepared locally; publishing Canvas frames depends on the enowX app connection.

Windows: the updated MyGo launcher is cross-compiled for amd64. Successful compilation does not verify Windows runtime, notification, tray, or sound behavior. Those require later Windows testing. The production Kiro foundation renders native MyGo components; these prototype HTML views are reference material for its implementation.

Raw manual observations: `qa-observations.json`. Screen/export inventory: `manifest.json`.
