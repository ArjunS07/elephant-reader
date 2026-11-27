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

`seed` loads the sample library. It is idempotent: running it again refreshes sample documents and does not reset reading state already stored for those items. `sync` fetches enabled RSS, Atom, podcast, YouTube, and Hacker News sources once.

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
| `ELEPHANT_API_TOKEN` | unset | If set, `/v1/*` requires `Authorization: Bearer <token>` except `GET /v1/health` |
| `ELEPHANT_TIMEZONE` | `America/Los_Angeles` | Timezone for `published_day` |
| `STATIC_DIR` | `dist` | Built frontend. Missing directory still serves the API |
| `VITE_ELEPHANT_API_TOKEN` | unset | Sent by the dev UI when the API token is set |

## Layout

- `cmd/elephant` starts `serve`, `seed`, or `sync`
- `internal/db` opens SQLite in WAL mode, migrates, and keeps the FTS5 index in sync
- `internal/api` is the `/v1` HTTP API and the SPA fallback
- `internal/ingest` upserts items, parses feeds, and accepts newsletter mail
- `internal/model` is the JSON contract the UI mirrors in `src/types.ts`
- `src/api.ts` is the only frontend call into `/v1`
