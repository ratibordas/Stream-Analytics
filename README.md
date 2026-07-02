# Stream Analytics

Live-stream analytics that answers one question: **which games hold a stable
audience, and where is it worth streaming?**

Neither Twitch nor YouTube exposes viewer *history* — only the current value. So the
backend builds the history itself: a collector polls the platform APIs on a schedule
and writes snapshots to SQLite. Every "over period" metric is computed from those
snapshots, so analysis gets deeper the longer the server runs.

> **Twitch is currently disabled** (see [Re-enabling Twitch](#re-enabling-twitch)).
> The active data source is **YouTube**. Without any API key the server runs in
> **mock mode** with generated data so the UI works out of the box.

Monorepo: `backend/` — Go (HTTP API + collectors + SQLite), `frontend/` — React + Vite + TypeScript.

## Quick start

Requires Go ≥ 1.23 and Node ≥ 18.

```bash
./scripts/setup.sh    # check toolchain, install deps, create .env
./scripts/dev.sh      # backend :8080 + Vite :5173  → open http://localhost:5173
```

Single production binary (Go serves the built frontend):

```bash
./scripts/start.sh    # build + run on http://localhost:8080
```

`make dev` / `make run` do the same.

## Configuration

Copy `.env.example` to `.env` (done by `setup.sh`). Key settings:

| Var | Default | Purpose |
|-----|---------|---------|
| `YOUTUBE_API_KEY` | — | YouTube Data API v3 key. Empty → mock mode. |
| `RAWG_API_KEY` | — | [RAWG](https://rawg.io/apidocs) key — powers the game picker's catalog. |
| `YT_POLL_INTERVAL_SEC` | `14400` | Poll interval (4 h). `search.list` costs 100 of 10 000 daily quota units. |
| `MOCK` | `auto` | `auto` (mock when no key) / `true` / `false`. |
| `PORT` | `8080` | HTTP port. |
| `DB_PATH` | `./data/analytics.db` | SQLite file. |

Get keys: YouTube — [console.cloud.google.com](https://console.cloud.google.com) → create a
project → enable **YouTube Data API v3** → Credentials → API key. RAWG —
[rawg.io/apidocs](https://rawg.io/apidocs) (free, email signup).

Keys can also be set at runtime via the **Settings** tab: they're validated against the
real API, kept only in browser `localStorage` + server memory (never written to disk),
and hot-swap the collectors with no restart.

**Tracked games** are chosen in the **Games** tab: an "Add games" picker searches the RAWG
catalog (multi-select), picked games show as removable chips, and each becomes a YouTube
live-search the collector polls. The tracked list lives in the browser, is pushed to the
server, and hot-rebuilds the collector — there is no `.env` list and no hardcoded seed
(the app starts empty). Searching a game that isn't tracked in the streams table also
offers a one-click "Track & fetch".

## API

```
GET  /api/meta                  # mock flag, snapshot count, time range
POST /api/poll                  # poll all collectors now
GET  /api/keys/status           # which keys are active, mock flag
POST /api/keys                  # {youtube_api_key, wipe_data} — validate + hot-swap
GET  /api/queries               # current tracked games
POST /api/queries               # {queries:[...]} — replace + hot-rebuild collector
GET  /api/games/search          # ?q= — RAWG catalog proxy for the game picker
GET  /api/youtube/streams       # ?game=&streamer=&min_viewers=&max_viewers=
                                #  &from=<unix>&to=<unix>&sort=current|period|trend
                                #  &order=asc|desc&live=0|1&limit=&offset=
GET  /api/youtube/games         # per-game aggregates · ?from=<unix>&to=<unix>
```

`{platform}` is `youtube` (Twitch routes return 404 while disabled).

## Metrics

- **Streams table** — current / average / peak viewers per stream, plus deltas:
  `Δ period` (viewer trend over the selected window), `Δ month` (channel over 30 days),
  `Δ subs/mo`. Changes ≥30% are highlighted.
- **Games** — per-game aggregates: `viewers per channel` (competition), `trend` (second
  half of the period vs the first), and `instability` (coefficient of variation; lower =
  steadier). Best target: high viewers-per-channel, low instability, trend ≥ 0.
- The UI is English/Russian (toggle in Settings) and refreshes every 60 s.

## Platform API limits

- Viewer history is collected by us — depth of analysis = collector uptime.
- YouTube quota is 10 000 units/day and `search.list` costs 100, hence the 4 h interval
  and grouping by search query rather than game.
- Subscriber counts come from YouTube channels; Twitch does not expose them for other
  channels.

## Re-enabling Twitch

The Twitch collector, validation, route, and UI are commented out, not deleted. Search
for `Twitch` / `DISABLED` and uncomment in:

- `backend/cmd/server/main.go` — collector wiring + key validation
- `backend/internal/api/server.go` — `platformOK`
- `frontend/src/App.tsx` — Twitch tab
- `frontend/src/SettingsView.tsx` — Twitch fieldset

Then fill `TWITCH_CLIENT_ID` / `TWITCH_CLIENT_SECRET` in `.env`. The collector code
(`backend/internal/collector/twitch.go`) is intact.

## Project layout

```
backend/
  cmd/server/        entrypoint, collector wiring, key hot-swap
  internal/api/      HTTP handlers + static file serving
  internal/collector mock / twitch / youtube collectors + manager + key validation
  internal/store/    SQLite schema, snapshot writes, aggregate queries
  internal/config/   .env + env parsing
frontend/src/        React SPA (views, filters, i18n)
scripts/             setup.sh, dev.sh, start.sh
```

