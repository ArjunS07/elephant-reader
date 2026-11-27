# Field

A local prototype of a personal reading client. Sources, items, queue state, and highlights live in the browser so the interface and the data model can be tried before a backend exists.

## Run

```bash
npm install
npm run dev
```

The dev server listens on [http://127.0.0.1:4731](http://127.0.0.1:4731). `/` redirects to `/feed`.

```bash
npm run build
npm run preview
```

Routes are real URLs (`/feed`, `/later`, `/read`, `/sources`, `/item/:id`). Vite runs as an SPA. The preview server rewrites extension-less paths to `index.html`, and `public/_redirects` covers hosts that read that file.

## Model

Types live in `src/types/model.ts`.

Queue (`firehose`, `later`, `dismissed`, `none`) is stored apart from `readAt`.

- Dismiss sets `queue` to `dismissed` and `dismissedAt` to now. `readAt` stays empty, so the item leaves Feed and Later and does not enter Read.
- Mark as read sets `readAt` to now. The item leaves Feed and Later and enters Read.
- Save to Later sets `queue` to `later` and clears `readAt`.
- Opening an item in Later stores scroll `readProgress` and leaves the item in Later until it is marked read or dismissed.

## Keys

- `field.data.v1`: sources, items, states, highlights
- `field.prefs`: theme, font, size, leading, column width

Preferences are applied from `index.html` before paint. Reset demo data on Sources restores the seed library and keeps preferences.

## Keys in the reader

`j` / `k` move, `s` saves to Later, `r` marks read, `d` dismisses, `o` opens the original, `Esc` returns to the list. The same `j` `k` `s` `r` `d` `o` keys work on Feed, and `j` `k` `r` `d` `o` work on Later.
