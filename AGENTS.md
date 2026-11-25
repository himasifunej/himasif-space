# AGENTS.md — Space (himasif-space)

Context for AI coding agents and for anyone landing here from a shared link. Read this first; it is enough to understand the project on its own. Human contributors: the same rules, in Indonesian, are in [CONTRIBUTING.md](CONTRIBUTING.md).

## What Space is

**Space — Ruang Kerja HIMASIF** is the workspace app of HIMASIF, the Information Systems student association at Universitas Jember, built for its 2025/2026 board (*HIMASIF ASTRA 25/26*). Its job: after every meeting, each task has a clear expected result, one main owner (*PJ utama*) and a deadline, and people are reminded on their device and in WhatsApp instead of having to open a spreadsheet.

- Each **division** has a *Space* per **period** (2025/2026, 2026/2027, …): task board, agenda, shared notes, meetings, members.
- Cross-division work has shared spaces without copying tasks. **Medhum is not a division**: it is the shared Mediatek × Humas content flow (Humas broadcast → design task for the Mediatek PIC with a Deadline → shared ACC → posting), modelled as a shared Space.
- Each member has **Untukku** ("for me"): one day-by-day timeline of their tasks, meetings and reminders across all Spaces.
- Windows desktop adds tray, notifications, offline cache, auto-update and **Ace Island** (the mascot *Ace* as a small floating island with an inbox).
- A non-AI **WhatsApp bot** per division answers commands (`!tugas`, `!hari`, `!cek`, …), sends scheduled reminders, and records submission evidence. The SOP review menus stay hidden until Litbang joins, but remain ready (PRD EVA-11).

Status: **pilot, pre-1.0** (see `app/mygo.json` for the current version). Target: **1.0.0 stable at `space.himasif.id` by Monday 7 Dec 2026 at the latest**, before the new board starts, with Mediatek, Humas and BPH fully working and the Medhum flow running across divisions (PRD §2.3). Users: HIMASIF board members; UI language: Indonesian.

Instances (ADR 0003): `space.tiyoouw.app` is the **test instance** (pilot DB, demo accounts, rc channel) and stays after 1.0.0; `space.himasif.id` will be the **stable instance** with a new DB that starts empty (period 2026/2027), no demo accounts, and new secrets. Download today: <https://space.tiyoouw.app/unduh/windows>. Web today: <https://space.tiyoouw.app>.

Product requirements: [docs/PRD.md](docs/PRD.md) (requirement IDs like `ACC-10`, `NFR-01`), decisions: [docs/decision-register.md](docs/decision-register.md) (the owner's decisions of 25 Nov 2026 toward 1.0.0 are its last section), technical decisions: [docs/adr/](docs/adr), work toward 1.0.0: [docs/implementation-plan.md](docs/implementation-plan.md#menuju-100).

Decisions that are easy to get wrong (details in the register):
- **Medhum dates and PIC:** the date in a Humas broadcast is the posting date; the design Deadline is posting − 2 days, once. The Mediatek PIC is set by the Mediatek kadiv at the start of the month; a broadcast's `pj med` is matched against it, never used to set it.
- **Shared ACC (Medhum design tasks only):** one ACC from the content's PJ Humas, a Humas manager (kadiv/45) or a Mediatek manager (kadiv/45, task owner); the PIC never ACCs their own work; who, which side and when is recorded. Enforced on the server.
- **Bot number:** the Space bot shares Reo's number (Reo = the owner's Hermes agent) with a group allowlist on both sides; groups move to Space one at a time, never served by both. Tests use the "Test" group.
- **Mobile:** no Flutter; a PWA of the same web build until MyGo supports mobile.
- **Privacy:** BPH monitors may see NIMs of every division they monitor; the shared admin account may read tasks and meetings (ADR 0002).
- **Terms:** "Hasil yang diharapkan", "Reminder", "Deadline", "Command", "Ace (Dynamic Island)", "Notulensi" (the notes) and "notulen" (the note-taker) in new text.

Lead developer and maintainer: Ahimsa Jenar Bramsaifstyo Kusuma (Tiyoouw), [@Tiyouw](https://github.com/Tiyouw), <https://tiyoouw.com>.

## Architecture map

Go monorepo (module `github.com/himasifunej/himasif-space`, Go 1.27) plus one Node service.

```
app/ (MyGo native UI)  ──┬─ Windows desktop (native, no WebView)
  internal/appui         └─ js/wasm web app, served by the server
  internal/client ──HTTPS/JSON (internal/api)──▶ cmd/space-server ──pgx──▶ PostgreSQL
                                                     ▲
                                bridge/ (Node 22, Baileys) ◀──▶ WhatsApp
```

| Path | What it is |
|---|---|
| `cmd/space-server` | The server binary: `serve`, `migrate`, `seed`, `version`, `quarantine-restore`, `activate-restore`, `mail-test` |
| `internal/server` | HTTP API (`net/http` router), auth middleware, card links (`/t/…`, `/r/…`), `/unduh/windows`, serves the web build |
| `internal/store` | PostgreSQL via `pgx/v5`, hand-written SQL; numbered embedded migrations in `internal/store/migrations/NNNN_*.sql` |
| `internal/api` | JSON messages shared by server and clients |
| `internal/authz` | Pure permission logic per role (Kadiv, 45, Pendamping, Anggota, custom) |
| `internal/auth` | Argon2id passwords, bearer session tokens (server stores only SHA-256) |
| `internal/appui` | The whole UI and its state, drawn with MyGo `ui`; testable without a window |
| `internal/workspaceui` | Native task board presentation |
| `internal/client` | API client for desktop and web (builds for js/wasm) |
| `internal/island` | Desktop notification queue and timing for Ace Island |
| `internal/update` | Signed (ed25519) update manifest, feed reading, version ordering |
| `internal/whatsapp` | Authenticated protocol to the bridge; commands and parsing live on the Go side |
| `internal/sop` | Versioned evidence comparison for SOP findings |
| `internal/integrations`, `broadcast`, `emergency`, `filestore`, `mail`, `guides`, `seed`, `config`, `id`, `timeutil` | Source adapters, Humas lists, emergency CSV, private blobs, email (SMTP/SES), in-app guides, seed data, env config, UUIDv7, time rules |
| `app/` | Desktop/web entry point and platform glue (tray, notifications, updater, offline storage, deep links) |
| `bridge/` | Separate WhatsApp transport service (Baileys): sessions, media, receipts. No business logic. |
| `tools/` | `updatesign` (sign/verify manifests), `quicknavgen` (Ctrl K index generator), `genicon` |
| `scripts/release` | `build_release.py`, `publish_update.py`, NSIS installer wrapper |
| `scripts/ops` | Backup/restore, legacy comparison, ops checks |
| `docs/` | PRD, ADRs, operations runbooks, per-stage handoff notes, screenshots |

Platform split in `app/` uses build tags and file suffixes: `*_desktop.go` (`!js`), `*_web.go` (`js`), `*_windows.go` (`windows && !js`), `*_other.go`.

## Build and test

Prerequisites: Go 1.27+, PostgreSQL 16/17, Python 3; Node 22+ and FFmpeg for `bridge/`. Configuration lives **outside** the repo: copy `.env.example` to `~/.config/himasif-space/dev.env` (chmod 600).

```bash
make check            # gofmt check, go vet for linux + windows + js/wasm, go test ./...
make test-db          # also runs server DB tests (SPACE_TEST_DATABASE_URL from the env file)
make seed             # migrate + seed period, divisions, org admin
make web              # build the wasm web app into app/build/web
make serve            # API + web on http://localhost:8787
make desktop-windows  # Windows build (go tool mygo build -platform windows/amd64 ./app)
go tool mygo dev ./app            # run the desktop app (needs a display)
go generate ./internal/appui      # regenerate the Ctrl K index after touching settings pages
python3 scripts/check_contributors.py
cd bridge && npm test
```

CI (`.github/workflows/ci.yml`) runs `make fmt vet`, the contributors check, and `go test -count=1 ./...` against Postgres 16. Database tests skip when `SPACE_TEST_DATABASE_URL` is unset, so `make check` passing locally is not the whole story for `internal/server`/`internal/store` changes.

## Conventions

- **Language.** UI text, bot messages, commit messages, CHANGELOG, README/CONTRIBUTING and most docs are in **Indonesian**. Code comments, identifiers and this file are in English. Match the surrounding comment density and style.
- **Commits:** `Area: what changed` in Indonesian, e.g. `Untukku: tes tidak bergantung pada jam saat dijalankan`. One coherent change per commit, tests and docs included.
- **Branches:** `feat/…`, `fix/…`, `design/…`, `docs/…`, `chore/…`, `release/<version>`. Features merge into a `release/…` branch, which merges into `main`. Until 1.0.0, open draft PRs against `release/1.0.0`.
- **Versions:** `app/mygo.json` `"version"` and `var version` in `app/main.go` must match. Pre-1.0: patch per update, minor for big updates; pre-releases `X.Y.Z-rc.N`. Version numbers during development are free; the first stable release is `1.0.0`, fixes after it are `1.0.x`.
- **Changelogs:** user-visible changes go under the top "Belum dirilis" section of `CHANGELOG.md` (create it above the latest release if it is missing; one line per PR). At release, `internal/appui/changelog.json` gets 3–6 short user-facing notes (≤ 600 chars total, they are signed into the update manifest).
- **Time:** instants are `timestamptz` (UTC); dates without time use `timeutil.Date`; default zone `Asia/Jakarta`. Tests must not depend on the wall clock or the machine's zone.
- **IDs:** UUIDv7 from `internal/id`, never derived from titles, rows or deadlines.
- **Formatting:** `gofmt`; no other linters. Keep dependencies minimal: RAM, CPU and download size are product requirements.

## Do

- Enforce every permission **on the server** (`internal/authz` + handlers). The client only hides what the user cannot do.
- Add a **new** migration file with the next number for any schema change.
- Add or update tests with every behaviour change; UI state in `internal/appui` is testable headless.
- Keep `internal/api` messages backward compatible with released desktop clients; old clients may still be running. Update manifests are read with `DisallowUnknownFields` by older clients, so never add manifest fields.
- Update docs (`docs/operations/*`, `CHANGELOG.md`) in the same commit as the change they describe.
- Read the relevant `docs/handoff/*.md` before changing a feature area; they record why things are the way they are.

## Don't

- Don't commit secrets, env files, tokens, the update signing key, databases, member data, or build outputs (`bin/`, `app/build/`, `dist/`). Never print or log secrets.
- Don't edit a migration that has been released.
- Don't add AI/LLM behaviour to the WhatsApp bot; it is deliberately rule-based.
- Don't put business decisions in `bridge/`; it is transport only.
- Don't publish releases, change the update feed, deploy the server, or change GitHub repository settings unless the maintainer explicitly asks. Releasing needs the signing key and is the maintainer's job.
- Don't push to `main` or merge PRs; open a (draft) PR instead.
- Don't invent product behaviour that isn't in the PRD or decision register; ask.
- Don't add Flutter or another mobile frontend, CMS features, or SOP behaviour that runs without an active SOP policy; these were decided against or deferred.

## How to contribute

1. Branch from `main` with the right prefix.
2. Make the change with tests; run `make check` (and `make test-db` for server/store).
3. Add a line to "Belum dirilis" in `CHANGELOG.md` if users will notice.
4. Open a draft PR to `main` (or the open `release/…` branch): what and why, how it was tested, screenshots for UI changes, migration/server impact.
5. Contributors add themselves to [`contributors.json`](contributors.json) (format: [docs/contributors.md](docs/contributors.md)) and copy it to `internal/appui/contributors.json`, which the app's Kredit page embeds (`TestContributorsEmbedded` checks they match).

Full guide: [CONTRIBUTING.md](CONTRIBUTING.md). Code of conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). License: MIT.
