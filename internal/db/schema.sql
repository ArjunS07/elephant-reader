CREATE TABLE IF NOT EXISTS sources (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  source_type TEXT NOT NULL CHECK (source_type IN ('rss', 'newsletter', 'youtube', 'hn', 'papers', 'podcast', 'snapshot', 'manual')),
  folder TEXT NOT NULL,
  default_open_mode TEXT NOT NULL CHECK (default_open_mode IN ('reader', 'external', 'deeplink')),
  default_queue TEXT NOT NULL CHECK (default_queue IN ('feed', 'later')),
  feed_url TEXT,
  email_alias TEXT,
  config_json TEXT NOT NULL DEFAULT '{}',
  enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
  last_fetched_at TEXT,
  etag TEXT,
  last_modified TEXT,
  last_error TEXT,
  last_status TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS sources_email_alias
  ON sources(email_alias)
  WHERE email_alias IS NOT NULL AND email_alias != '';

CREATE TABLE IF NOT EXISTS items (
  id TEXT PRIMARY KEY,
  source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
  external_id TEXT NOT NULL,
  canonical_url TEXT NOT NULL,
  discussion_url TEXT,
  title TEXT NOT NULL,
  author TEXT NOT NULL DEFAULT '',
  published_at TEXT NOT NULL,
  published_day TEXT NOT NULL,
  open_mode TEXT NOT NULL CHECK (open_mode IN ('reader', 'external', 'deeplink')),
  action_url TEXT NOT NULL,
  content_markdown TEXT,
  plain_text TEXT NOT NULL DEFAULT '',
  excerpt TEXT,
  metadata_json TEXT NOT NULL DEFAULT '{}',
  content_hash TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE (source_id, external_id)
);

-- canonical_url is NOT globally unique: episodes often share a show URL, and
-- the same link can appear in more than one source (HN and a digest).
-- Identity is (source_id, external_id). This index only speeds same-source URL lookups.
CREATE INDEX IF NOT EXISTS items_source_canonical ON items(source_id, canonical_url);
CREATE INDEX IF NOT EXISTS items_published_day ON items(published_day);
CREATE INDEX IF NOT EXISTS items_source_id ON items(source_id);

CREATE TABLE IF NOT EXISTS user_item_state (
  item_id TEXT PRIMARY KEY REFERENCES items(id) ON DELETE CASCADE,
  in_later INTEGER NOT NULL DEFAULT 0 CHECK (in_later IN (0, 1)),
  saved_at TEXT,
  read_at TEXT,
  read_progress REAL NOT NULL DEFAULT 0 CHECK (read_progress >= 0 AND read_progress <= 1),
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS state_later ON user_item_state(in_later, read_at, saved_at);
CREATE INDEX IF NOT EXISTS state_read ON user_item_state(read_at);

CREATE TABLE IF NOT EXISTS highlights (
  id TEXT PRIMARY KEY,
  item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  quote TEXT NOT NULL,
  prefix TEXT NOT NULL DEFAULT '',
  suffix TEXT NOT NULL DEFAULT '',
  note TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS highlights_item ON highlights(item_id);

CREATE TABLE IF NOT EXISTS user_preferences (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  theme TEXT NOT NULL DEFAULT 'warm' CHECK (theme IN ('light', 'warm', 'dark')),
  font_family TEXT NOT NULL DEFAULT 'inter' CHECK (font_family IN ('inter', 'lora', 'newsreader')),
  font_size REAL NOT NULL DEFAULT 18,
  line_height REAL NOT NULL DEFAULT 1.65,
  column_width TEXT NOT NULL DEFAULT 'standard' CHECK (column_width IN ('cozy', 'standard', 'wide')),
  align TEXT NOT NULL DEFAULT 'left' CHECK (align IN ('left', 'center')),
  updated_at TEXT NOT NULL
);

INSERT OR IGNORE INTO user_preferences (
  id, theme, font_family, font_size, line_height, column_width, align, updated_at
) VALUES (
  1, 'warm', 'inter', 18, 1.65, 'standard', 'left', '1970-01-01T00:00:00Z'
);

CREATE TABLE IF NOT EXISTS schema_migrations (
  id TEXT PRIMARY KEY,
  applied_at TEXT NOT NULL
);

CREATE VIRTUAL TABLE IF NOT EXISTS items_fts USING fts5(
  item_id UNINDEXED,
  title,
  author,
  plain_text,
  metadata_text,
  highlights_text,
  tokenize = 'porter unicode61'
);
