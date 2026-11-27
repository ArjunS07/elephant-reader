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

User state is `inLater` (with `savedAt`) and `readAt`, plus `readProgress`.

- The Feed is a river. Saving an item or marking it read leaves it in the stream.
- Later is `inLater` and no `readAt`. Remove clears `inLater` and does not set `readAt`.
- Mark as read sets `readAt`. The item stays in the river and enters Read.
- Opening an item in Later stores scroll `readProgress` until it is marked read or removed.

## Keys

- `field.data.v1`: sources, items, states, highlights
- `field.prefs`: theme, font, size, leading, column width

Preferences are applied from `index.html` before paint. Reset demo data on Sources restores the seed library and keeps preferences.

## Keys in the reader

`j` / `k` move, `s` saves to Later, `r` marks read, `o` opens the original, `Esc` returns to the list. The same `j` `k` `s` `r` `o` keys work on Feed and Later.
