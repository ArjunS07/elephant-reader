# elephant

A reading client for papers, newsletters, RSS, and the links you meant to finish. The interface is a Vite React app. The API and SQLite database are a Go server.

## Run

Two processes in development:

```bash
go run ./cmd/elephant seed
go run ./cmd/elephant serve
npm install
npm run dev
```

The API listens on `:8080`. Vite listens on `:4731` and proxies `/v1` there. Open `http://127.0.0.1:4731`.

Production is one process. Build the UI, then serve it from the Go binary:

```bash
npm run build
STATIC_DIR=dist go run ./cmd/elephant seed
STATIC_DIR=dist go run ./cmd/elephant serve
```

`seed` loads the sample library. It is idempotent: running it again refreshes sample documents and does not reset reading state already stored for those items. `sync` fetches enabled RSS, Atom, podcast, YouTube, and Hacker News sources with 12 workers. Unchanged feeds return 304 and are not parsed again.

```bash
elephant import subscriptions.opml
elephant import takeout.csv youtube
```

YouTube Takeout CSV rows become Video sources. Overcast OPML rows become podcast sources that open in Overcast. Other OPML outlines become RSS sources. A feed URL that already exists is skipped.

## Deploy on Fly.io

The app is one shared-cpu-1x Machine with 256 MB of RAM and 256 MB of swap in `lax`, a 1 GB volume at `/data`, and machines that stop when idle (`min_machines_running = 0`). `GOMEMLIMIT` is `180MiB` and `kill_timeout` is 15s so a deploy can drain in-flight requests. That stays inside the $5/mo Free allowance. The Fly app name is `elephant-reader-arjun`. Deploy with a single Machine so the SQLite file on `elephant_data` is not split across hosts:

```bash
fly launch --no-deploy
fly secrets set ELEPHANT_API_TOKEN=...
fly secrets set ELEPHANT_PASSPHRASE=...
fly deploy --ha=false
```

`GET /v1/health` is the HTTP check and stays open. The browser unlocks with `POST /v1/session` and then sends the `HttpOnly` session cookie. CLI and server-to-server calls keep using `Authorization: Bearer <ELEPHANT_API_TOKEN>`. The token is not baked into the frontend bundle.

There is no second worker. Seed and feed sync run on the same Machine, against the volume:

```bash
fly ssh console -C "/app/elephant seed"
fly ssh console -C "/app/elephant sync"
```

`sync` is the daily pass for RSS, Atom, podcast, YouTube, and Hacker News. Run it over SSH when you want a refresh. An always-on second process would keep the Machine from sleeping.

```bash
go test ./...
go build ./...
npm run build
```

## Environment

| Variable | Default | |
| --- | --- | --- |
| `PORT` | `8080` | HTTP port |
| `DATABASE_PATH` | `data/elephant.db` | SQLite file |
| `ELEPHANT_API_TOKEN` | unset | Bearer token for CLI and server-to-server calls. Also the browser passphrase when `ELEPHANT_PASSPHRASE` is unset |
| `ELEPHANT_PASSPHRASE` | unset | Browser passphrase checked by `POST /v1/session`. Falls back to `ELEPHANT_API_TOKEN` |
| `ELEPHANT_CORS_ORIGIN` | unset | Optional allowed browser origin. Same-origin requests need nothing, and `*` is ignored |
| `ELEPHANT_TIMEZONE` | `America/Los_Angeles` | Timezone for `published_day` |
| `STATIC_DIR` | `dist` | Built frontend. Missing directory still serves the API |

## Layout

- `cmd/elephant` starts `serve`, `seed`, or `sync`
- `internal/db` opens SQLite in WAL mode, migrates, and keeps the FTS5 index in sync
- `internal/api` is the `/v1` HTTP API and the SPA fallback
- `internal/ingest` upserts items, parses feeds, and accepts newsletter mail
- `internal/model` is the JSON contract the UI mirrors in `src/types.ts`
- `src/api.ts` is the only frontend call into `/v1`
