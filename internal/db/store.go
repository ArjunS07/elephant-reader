package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"elephant/internal/model"
)

var ErrNotFound = errors.New("not found")

// ErrInvalidReplaceScope is returned when replaceScope.externalIdPrefix is
// not exactly "mldigest:YYYY-MM-DD:". That tight form is what stops a
// replace from touching other sources or other days.
var ErrInvalidReplaceScope = errors.New("replaceScope.externalIdPrefix must be mldigest:YYYY-MM-DD:")

const rowSelect = `
SELECT
  i.id, i.source_id, i.external_id, i.canonical_url, i.discussion_url,
  i.title, i.author, i.published_at, i.published_day, i.open_mode, i.action_url,
  i.content_markdown, i.plain_text, i.excerpt, i.metadata_json, i.created_at, i.updated_at,
  src.id, src.name, src.source_type, src.folder, src.default_open_mode, src.default_queue,
  src.feed_url, src.email_alias, src.config_json, src.enabled, src.last_fetched_at, src.etag, src.last_modified, src.last_error, src.last_status, src.created_at, src.updated_at,
  IFNULL(st.in_later, 0), st.saved_at, st.read_at, IFNULL(st.read_progress, 0),
  (SELECT COUNT(*) FROM highlights h WHERE h.item_id = i.id)
FROM items i
JOIN sources src ON src.id = i.source_id
LEFT JOIN user_item_state st ON st.item_id = i.id
`

type filter struct {
	Folder   string
	SourceID string
}

func (s *Store) UpsertSource(src model.Source) (model.Source, error) {
	now := Now()
	if src.CreatedAt == "" {
		src.CreatedAt = now
	}
	src.UpdatedAt = now
	if src.Config == nil {
		src.Config = map[string]any{}
	}
	config, err := json.Marshal(src.Config)
	if err != nil {
		return model.Source{}, err
	}
	enabled := 0
	if src.Enabled {
		enabled = 1
	}
	_, err = s.SQL.Exec(`
INSERT INTO sources (
  id, name, source_type, folder, default_open_mode, default_queue,
  feed_url, email_alias, config_json, enabled, last_fetched_at, etag, last_modified, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  name = excluded.name,
  source_type = excluded.source_type,
  folder = excluded.folder,
  default_open_mode = excluded.default_open_mode,
  default_queue = excluded.default_queue,
  feed_url = excluded.feed_url,
  email_alias = CASE
    WHEN sources.email_alias IS NOT NULL AND sources.email_alias != '' THEN sources.email_alias
    ELSE excluded.email_alias
  END,
  config_json = excluded.config_json,
  updated_at = excluded.updated_at
`, src.ID, src.Name, src.SourceType, src.Folder, src.DefaultOpenMode, src.DefaultQueue,
		NullString(src.FeedURL), NullString(src.EmailAlias), string(config), enabled, nil, NullString(src.ETag), NullString(src.LastModified), src.CreatedAt, src.UpdatedAt)
	if err != nil {
		return model.Source{}, err
	}
	return s.GetSource(src.ID)
}

func (s *Store) InsertSource(src model.Source) (model.Source, error) {
	now := Now()
	if src.CreatedAt == "" {
		src.CreatedAt = now
	}
	src.UpdatedAt = now
	if src.Config == nil {
		src.Config = map[string]any{}
	}
	config, err := json.Marshal(src.Config)
	if err != nil {
		return model.Source{}, err
	}
	enabled := 0
	if src.Enabled {
		enabled = 1
	}
	_, err = s.SQL.Exec(`
INSERT INTO sources (
  id, name, source_type, folder, default_open_mode, default_queue,
  feed_url, email_alias, config_json, enabled, last_fetched_at, etag, last_modified, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?)
`, src.ID, src.Name, src.SourceType, src.Folder, src.DefaultOpenMode, src.DefaultQueue,
		NullString(src.FeedURL), NullString(src.EmailAlias), string(config), enabled, NullString(src.ETag), NullString(src.LastModified), src.CreatedAt, src.UpdatedAt)
	if err != nil {
		return model.Source{}, err
	}
	return s.GetSource(src.ID)
}

func (s *Store) ListSources() ([]model.Source, error) {
	rows, err := s.SQL.Query(`
SELECT id, name, source_type, folder, default_open_mode, default_queue,
  feed_url, email_alias, config_json, enabled, last_fetched_at, etag, last_modified, last_error, last_status, created_at, updated_at
FROM sources
ORDER BY name COLLATE NOCASE, id
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Source
	for rows.Next() {
		src, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	if out == nil {
		out = []model.Source{}
	}
	return out, rows.Err()
}

func (s *Store) GetSource(id string) (model.Source, error) {
	row := s.SQL.QueryRow(`
SELECT id, name, source_type, folder, default_open_mode, default_queue,
  feed_url, email_alias, config_json, enabled, last_fetched_at, etag, last_modified, last_error, last_status, created_at, updated_at
FROM sources WHERE id = ?
`, id)
	src, err := scanSource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Source{}, ErrNotFound
	}
	return src, err
}

func (s *Store) FindSourceByAlias(alias string) (model.Source, error) {
	row := s.SQL.QueryRow(`
SELECT id, name, source_type, folder, default_open_mode, default_queue,
  feed_url, email_alias, config_json, enabled, last_fetched_at, etag, last_modified, last_error, last_status, created_at, updated_at
FROM sources WHERE lower(email_alias) = lower(?)
`, strings.TrimSpace(alias))
	src, err := scanSource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Source{}, ErrNotFound
	}
	return src, err
}

func (s *Store) FeedURLs() (map[string]struct{}, error) {
	rows, err := s.SQL.Query(`SELECT feed_url FROM sources WHERE feed_url IS NOT NULL AND feed_url != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		out[raw] = struct{}{}
	}
	return out, rows.Err()
}

func (s *Store) AliasTaken(alias string) (bool, error) {
	var n int
	err := s.SQL.QueryRow(`SELECT COUNT(*) FROM sources WHERE lower(email_alias) = lower(?)`, alias).Scan(&n)
	return n > 0, err
}

type SourcePatch struct {
	Name            *string
	Folder          *string
	DefaultOpenMode *string
	DefaultQueue    *string
	FeedURL         *string
	EmailAlias      *string
	Enabled         *bool
	Config          map[string]any
	HasConfig       bool
	LastFetchedAt   *string
	TouchFetched    bool
	ETag            *string
	LastModified    *string
	TouchValidators bool
	LastError       *string
	LastStatus      *string
	TouchSyncState  bool
}

func (s *Store) PatchSource(id string, patch SourcePatch) (model.Source, error) {
	current, err := s.GetSource(id)
	if err != nil {
		return model.Source{}, err
	}
	if patch.Name != nil {
		current.Name = *patch.Name
	}
	if patch.Folder != nil {
		current.Folder = *patch.Folder
	}
	if patch.DefaultOpenMode != nil {
		current.DefaultOpenMode = *patch.DefaultOpenMode
	}
	if patch.DefaultQueue != nil {
		current.DefaultQueue = *patch.DefaultQueue
	}
	if patch.FeedURL != nil {
		current.FeedURL = *patch.FeedURL
	}
	if patch.EmailAlias != nil {
		current.EmailAlias = *patch.EmailAlias
	}
	if patch.Enabled != nil {
		current.Enabled = *patch.Enabled
	}
	if patch.HasConfig {
		current.Config = patch.Config
	}
	if patch.TouchFetched {
		current.LastFetchedAt = patch.LastFetchedAt
	}
	if patch.TouchValidators {
		current.ETag = deref(patch.ETag)
		current.LastModified = deref(patch.LastModified)
	}
	if patch.TouchSyncState {
		current.LastStatus = deref(patch.LastStatus)
		current.LastError = deref(patch.LastError)
	}
	if current.Config == nil {
		current.Config = map[string]any{}
	}
	config, err := json.Marshal(current.Config)
	if err != nil {
		return model.Source{}, err
	}
	enabled := 0
	if current.Enabled {
		enabled = 1
	}
	now := Now()
	_, err = s.SQL.Exec(`
UPDATE sources SET
  name = ?, folder = ?, default_open_mode = ?, default_queue = ?,
  feed_url = ?, email_alias = ?, config_json = ?, enabled = ?,
  last_fetched_at = ?, etag = ?, last_modified = ?, last_error = ?, last_status = ?, updated_at = ?
WHERE id = ?
`, current.Name, current.Folder, current.DefaultOpenMode, current.DefaultQueue,
		NullString(current.FeedURL), NullString(current.EmailAlias), string(config), enabled,
		nullablePtr(current.LastFetchedAt), NullString(current.ETag), NullString(current.LastModified),
		NullString(current.LastError), NullString(current.LastStatus), now, id)
	if err != nil {
		return model.Source{}, err
	}
	return s.GetSource(id)
}

func (s *Store) DeleteSource(id string) error {
	return s.withTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM items_fts WHERE rowid IN (SELECT rowid FROM items WHERE source_id = ?)`, id); err != nil {
			return err
		}
		res, err := tx.Exec(`DELETE FROM sources WHERE id = ?`, id)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// ValidDigestReplacePrefix reports whether prefix is exactly mldigest:YYYY-MM-DD:.
func ValidDigestReplacePrefix(prefix string) bool {
	const want = len("mldigest:2006-01-02:")
	if len(prefix) != want || !strings.HasPrefix(prefix, "mldigest:") || !strings.HasSuffix(prefix, ":") {
		return false
	}
	day := prefix[len("mldigest:") : want-1]
	_, err := time.Parse("2006-01-02", day)
	return err == nil
}

// DeleteByExternalIDPrefixExcept deletes items whose external_id starts with
// prefix, except those whose external_id is in keep. user_item_state and
// highlights follow via ON DELETE CASCADE. items_fts is a standalone table
// and is cleared by items.rowid first. Prefix must be mldigest:YYYY-MM-DD:.
func (s *Store) DeleteByExternalIDPrefixExcept(prefix string, keep []string) (int, error) {
	if !ValidDigestReplacePrefix(prefix) {
		return 0, ErrInvalidReplaceScope
	}
	kept := make([]string, 0, len(keep))
	seen := map[string]struct{}{}
	for _, id := range keep {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		kept = append(kept, id)
	}
	like := prefix + "%"
	var deleted int64
	err := s.withTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`PRAGMA foreign_keys=ON`); err != nil {
			return err
		}
		args := []any{like}
		notIn := ""
		if len(kept) > 0 {
			placeholders := make([]string, len(kept))
			for i, id := range kept {
				placeholders[i] = "?"
				args = append(args, id)
			}
			notIn = " AND external_id NOT IN (" + strings.Join(placeholders, ",") + ")"
		}
		match := "external_id LIKE ?" + notIn
		if _, err := tx.Exec(`DELETE FROM items_fts WHERE rowid IN (SELECT rowid FROM items WHERE `+match+`)`, args...); err != nil {
			return err
		}
		res, err := tx.Exec(`DELETE FROM items WHERE `+match, args...)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		deleted = n
		return nil
	})
	if err != nil {
		return 0, err
	}
	return int(deleted), nil
}

func (s *Store) EnsureManualSource() (model.Source, error) {
	row := s.SQL.QueryRow(`
SELECT id FROM sources
WHERE source_type = 'manual' AND (id = 'src_chrome' OR lower(name) = 'chrome')
LIMIT 1
`)
	var id string
	err := row.Scan(&id)
	if err == nil {
		return s.GetSource(id)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return model.Source{}, err
	}
	err = s.SQL.QueryRow(`
SELECT id FROM sources WHERE source_type = 'manual' AND default_queue = 'later' LIMIT 1
`).Scan(&id)
	if err == nil {
		return s.GetSource(id)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return model.Source{}, err
	}
	return s.InsertSource(model.Source{
		ID:              "src_chrome",
		Name:            "Chrome",
		SourceType:      "manual",
		Folder:          "Blogs",
		DefaultOpenMode: "external",
		DefaultQueue:    "later",
		Enabled:         true,
		Config:          map[string]any{},
	})
}

func (s *Store) FindItemID(sourceID, externalID, canonical string) (string, error) {
	return findItemID(s.SQL, sourceID, externalID, canonical)
}

type rowQuery interface {
	QueryRow(query string, args ...any) *sql.Row
}

// findItemID matches (source_id, external_id). It falls back to
// (source_id, canonical_url) only inside that source, and only when the
// caller has no distinct external id. A guid that differs from the URL must
// not collapse episodes that share a show URL, and a URL must never match a
// row from a different source.
func findItemID(q rowQuery, sourceID, externalID, canonical string) (string, error) {
	if sourceID != "" && externalID != "" {
		var id string
		err := q.QueryRow(`SELECT id FROM items WHERE source_id = ? AND external_id = ?`, sourceID, externalID).Scan(&id)
		if err == nil {
			return id, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}
	if sourceID != "" && canonical != "" && strings.TrimSpace(externalID) == "" {
		var id string
		err := q.QueryRow(`
SELECT id FROM items
WHERE source_id = ? AND canonical_url = ?
ORDER BY created_at, id
LIMIT 1
`, sourceID, canonical).Scan(&id)
		if err == nil {
			return id, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}
	return "", ErrNotFound
}

type ItemWrite struct {
	ID              string
	SourceID        string
	ExternalID      string
	CanonicalURL    string
	DiscussionURL   string
	Title           string
	Author          string
	PublishedAt     string
	PublishedDay    string
	OpenMode        string
	ActionURL       string
	ContentMarkdown string
	PlainText       string
	Excerpt         string
	Metadata        json.RawMessage
}

type PreparedItem struct {
	Write   ItemWrite
	Initial *model.InitialState
}

type ingestOutcome struct {
	ID      string
	Created bool
}

// CommitIngest writes every item in one transaction. A failure rolls the whole batch back.
func (s *Store) CommitIngest(items []PreparedItem) ([]model.IngestResult, error) {
	if len(items) == 0 {
		return []model.IngestResult{}, nil
	}
	outcomes := make([]ingestOutcome, len(items))
	err := s.withTx(func(tx *sql.Tx) error {
		for i, item := range items {
			id, created, err := upsertTx(tx, item)
			if err != nil {
				return err
			}
			outcomes[i] = ingestOutcome{ID: id, Created: created}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]model.IngestResult, len(outcomes))
	for i, outcome := range outcomes {
		got, _, _, _, err := s.GetItem(outcome.ID)
		if err != nil {
			return nil, err
		}
		out[i] = model.IngestResult{Item: got, Created: outcome.Created}
	}
	return out, nil
}

func (s *Store) InsertItem(item ItemWrite, initial *model.InitialState) (model.Item, error) {
	results, err := s.CommitIngest([]PreparedItem{{Write: item, Initial: initial}})
	if err != nil {
		return model.Item{}, err
	}
	return results[0].Item, nil
}

func (s *Store) UpdateItem(id string, item ItemWrite, initial *model.InitialState) (model.Item, error) {
	item.ID = id
	existing, err := findItemID(s.SQL, item.SourceID, item.ExternalID, "")
	if err != nil && !errors.Is(err, ErrNotFound) {
		return model.Item{}, err
	}
	if existing == "" {
		existing = id
	}
	var created bool
	err = s.withTx(func(tx *sql.Tx) error {
		var err error
		existing, created, err = upsertTx(tx, PreparedItem{Write: item, Initial: initial})
		return err
	})
	if err != nil {
		return model.Item{}, err
	}
	_ = created
	got, _, _, _, err := s.GetItem(existing)
	return got, err
}

func upsertTx(tx *sql.Tx, item PreparedItem) (string, bool, error) {
	write := item.Write
	if strings.TrimSpace(string(write.Metadata)) == "" {
		write.Metadata = json.RawMessage(`{}`)
	}
	hash := contentHash(write)
	now := Now()
	existing, err := findItemID(tx, write.SourceID, write.ExternalID, write.CanonicalURL)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return "", false, err
	}
	if existing == "" && strings.TrimSpace(write.ID) != "" {
		var id string
		err = tx.QueryRow(`SELECT id FROM items WHERE id = ?`, write.ID).Scan(&id)
		if err == nil {
			existing = id
		} else if !errors.Is(err, sql.ErrNoRows) {
			return "", false, err
		}
	}
	if existing != "" {
		unchanged, err := itemUnchanged(tx, existing, write, hash)
		if err != nil {
			return "", false, err
		}
		if unchanged {
			if _, err := tx.Exec(`UPDATE items SET content_hash = ? WHERE id = ? AND (content_hash IS NULL OR content_hash != ?)`, hash, existing, hash); err != nil {
				return "", false, err
			}
			if item.Initial != nil {
				if _, err := tx.Exec(`
INSERT INTO user_item_state (item_id, in_later, saved_at, read_at, read_progress, updated_at)
VALUES (?, 0, NULL, NULL, 0, ?)
ON CONFLICT(item_id) DO NOTHING
`, existing, now); err != nil {
					return "", false, err
				}
				if err := applyInitialTx(tx, existing, item.Initial, now); err != nil {
					return "", false, err
				}
			}
			return existing, false, nil
		}
		if _, err := tx.Exec(`
UPDATE items SET
  discussion_url = ?, title = ?, author = ?, published_at = ?, published_day = ?,
  open_mode = ?, action_url = ?, content_markdown = ?, plain_text = ?, excerpt = ?,
  metadata_json = ?, content_hash = ?, updated_at = ?
WHERE id = ?
`, NullString(write.DiscussionURL), write.Title, write.Author, write.PublishedAt, write.PublishedDay,
			write.OpenMode, write.ActionURL, NullString(write.ContentMarkdown), write.PlainText, NullString(write.Excerpt),
			string(write.Metadata), hash, now, existing); err != nil {
			return "", false, err
		}
		if _, err := tx.Exec(`
INSERT INTO user_item_state (item_id, in_later, saved_at, read_at, read_progress, updated_at)
VALUES (?, 0, NULL, NULL, 0, ?)
ON CONFLICT(item_id) DO NOTHING
`, existing, now); err != nil {
			return "", false, err
		}
		if item.Initial != nil {
			if err := applyInitialTx(tx, existing, item.Initial, now); err != nil {
				return "", false, err
			}
		}
		if err := reindexTx(tx, existing); err != nil {
			return "", false, err
		}
		return existing, false, nil
	}
	id := strings.TrimSpace(write.ID)
	if id == "" {
		return "", false, errors.New("item id is required")
	}
	if _, err := tx.Exec(`
INSERT INTO items (
  id, source_id, external_id, canonical_url, discussion_url, title, author,
  published_at, published_day, open_mode, action_url, content_markdown, plain_text,
  excerpt, metadata_json, content_hash, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, id, write.SourceID, write.ExternalID, write.CanonicalURL, NullString(write.DiscussionURL),
		write.Title, write.Author, write.PublishedAt, write.PublishedDay, write.OpenMode, write.ActionURL,
		NullString(write.ContentMarkdown), write.PlainText, NullString(write.Excerpt), string(write.Metadata), hash, now, now); err != nil {
		return "", false, err
	}
	inLater, savedAt, readAt, progress := stateFromInitial(item.Initial, now)
	if _, err := tx.Exec(`
INSERT INTO user_item_state (item_id, in_later, saved_at, read_at, read_progress, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
`, id, inLater, savedAt, readAt, progress, now); err != nil {
		return "", false, err
	}
	if err := reindexTx(tx, id); err != nil {
		return "", false, err
	}
	return id, true, nil
}

func itemUnchanged(tx *sql.Tx, id string, write ItemWrite, hash string) (bool, error) {
	var title, author, published, day, mode, action, plain string
	var storedHash, discussion, markdown, excerpt, meta sql.NullString
	err := tx.QueryRow(`
SELECT content_hash, title, author, published_at, published_day, open_mode, action_url,
  discussion_url, content_markdown, plain_text, excerpt, metadata_json
FROM items WHERE id = ?
`, id).Scan(&storedHash, &title, &author, &published, &day, &mode, &action, &discussion, &markdown, &plain, &excerpt, &meta)
	if err != nil {
		return false, err
	}
	if storedHash.Valid && storedHash.String != "" && storedHash.String == hash {
		return true, nil
	}
	same := title == write.Title &&
		author == write.Author &&
		published == write.PublishedAt &&
		day == write.PublishedDay &&
		mode == write.OpenMode &&
		action == write.ActionURL &&
		plain == write.PlainText &&
		nullEq(discussion, write.DiscussionURL) &&
		nullEq(markdown, write.ContentMarkdown) &&
		nullEq(excerpt, write.Excerpt) &&
		strings.TrimSpace(meta.String) == strings.TrimSpace(string(write.Metadata))
	return same, nil
}

func contentHash(item ItemWrite) string {
	h := sha256.New()
	write := func(s string) {
		_, _ = h.Write([]byte(s))
		_, _ = h.Write([]byte{0})
	}
	write(item.Title)
	write(item.Author)
	write(item.PublishedAt)
	write(item.PublishedDay)
	write(item.OpenMode)
	write(item.ActionURL)
	write(strings.TrimSpace(item.ContentMarkdown))
	write(item.PlainText)
	write(strings.TrimSpace(item.Excerpt))
	write(strings.TrimSpace(item.DiscussionURL))
	write(strings.TrimSpace(string(item.Metadata)))
	return hex.EncodeToString(h.Sum(nil))
}

func nullEq(stored sql.NullString, value string) bool {
	value = strings.TrimSpace(value)
	got := ""
	if stored.Valid {
		got = strings.TrimSpace(stored.String)
	}
	return got == value
}

func (s *Store) PutState(id string, state model.UserItemState) error {
	now := Now()
	inLater := 0
	if state.InLater {
		inLater = 1
	}
	_, err := s.SQL.Exec(`
INSERT INTO user_item_state (item_id, in_later, saved_at, read_at, read_progress, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(item_id) DO UPDATE SET
  in_later = excluded.in_later,
  saved_at = excluded.saved_at,
  read_at = excluded.read_at,
  read_progress = excluded.read_progress,
  updated_at = excluded.updated_at
`, id, inLater, nullablePtr(state.SavedAt), nullablePtr(state.ReadAt), clampProgress(state.ReadProgress), now)
	return err
}

type StatePatch struct {
	InLater      *bool
	ReadAtSet    bool
	ReadAt       *string
	ProgressSet  bool
	ReadProgress float64
}

func (s *Store) PatchState(id string, patch StatePatch) (model.UserItemState, error) {
	var exists string
	if err := s.SQL.QueryRow(`SELECT id FROM items WHERE id = ?`, id).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.UserItemState{}, ErrNotFound
		}
		return model.UserItemState{}, err
	}
	now := Now()
	err := s.withTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`
INSERT INTO user_item_state (item_id, in_later, saved_at, read_at, read_progress, updated_at)
VALUES (?, 0, NULL, NULL, 0, ?)
ON CONFLICT(item_id) DO NOTHING
`, id, now); err != nil {
			return err
		}
		var inLater int
		var saved, read sql.NullString
		var progress float64
		if err := tx.QueryRow(`SELECT in_later, saved_at, read_at, read_progress FROM user_item_state WHERE item_id = ?`, id).
			Scan(&inLater, &saved, &read, &progress); err != nil {
			return err
		}
		if patch.InLater != nil {
			if *patch.InLater {
				inLater = 1
				if !saved.Valid {
					saved = sql.NullString{String: now, Valid: true}
				}
			} else {
				inLater = 0
				saved = sql.NullString{}
			}
		}
		if patch.ReadAtSet {
			read = nullablePtr(patch.ReadAt)
		}
		if patch.ProgressSet {
			progress = clampProgress(patch.ReadProgress)
		}
		_, err := tx.Exec(`
UPDATE user_item_state SET in_later = ?, saved_at = ?, read_at = ?, read_progress = ?, updated_at = ?
WHERE item_id = ?
`, inLater, saved, read, progress, now, id)
		return err
	})
	if err != nil {
		return model.UserItemState{}, err
	}
	_, _, state, _, err := s.GetItem(id)
	return state, err
}

func (s *Store) GetItem(id string) (model.Item, model.Source, model.UserItemState, []model.Highlight, error) {
	row := s.SQL.QueryRow(rowSelect+` WHERE i.id = ?`, id)
	item, source, state, _, err := scanRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Item{}, model.Source{}, model.UserItemState{}, nil, ErrNotFound
	}
	if err != nil {
		return model.Item{}, model.Source{}, model.UserItemState{}, nil, err
	}
	highlights, err := s.ListHighlights(id)
	if err != nil {
		return model.Item{}, model.Source{}, model.UserItemState{}, nil, err
	}
	return item, source, state, highlights, nil
}

func (s *Store) ListHighlights(itemID string) ([]model.Highlight, error) {
	rows, err := s.SQL.Query(`
SELECT id, item_id, quote, prefix, suffix, note, created_at, updated_at
FROM highlights WHERE item_id = ? ORDER BY created_at, id
`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Highlight
	for rows.Next() {
		var h model.Highlight
		if err := rows.Scan(&h.ID, &h.ItemID, &h.Quote, &h.Prefix, &h.Suffix, &h.Note, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	if out == nil {
		out = []model.Highlight{}
	}
	return out, rows.Err()
}

func (s *Store) AddHighlight(h model.Highlight) (model.Highlight, error) {
	now := Now()
	if h.CreatedAt == "" {
		h.CreatedAt = now
	}
	h.UpdatedAt = now
	err := s.withTx(func(tx *sql.Tx) error {
		var item string
		if err := tx.QueryRow(`SELECT id FROM items WHERE id = ?`, h.ItemID).Scan(&item); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if _, err := tx.Exec(`
INSERT INTO highlights (id, item_id, quote, prefix, suffix, note, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`, h.ID, h.ItemID, h.Quote, h.Prefix, h.Suffix, h.Note, h.CreatedAt, h.UpdatedAt); err != nil {
			return err
		}
		return reindexTx(tx, h.ItemID)
	})
	if err != nil {
		return model.Highlight{}, err
	}
	return s.GetHighlight(h.ID)
}

func (s *Store) GetHighlight(id string) (model.Highlight, error) {
	var h model.Highlight
	err := s.SQL.QueryRow(`
SELECT id, item_id, quote, prefix, suffix, note, created_at, updated_at FROM highlights WHERE id = ?
`, id).Scan(&h.ID, &h.ItemID, &h.Quote, &h.Prefix, &h.Suffix, &h.Note, &h.CreatedAt, &h.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Highlight{}, ErrNotFound
	}
	return h, err
}

func (s *Store) PatchHighlight(id, note string) (model.Highlight, error) {
	now := Now()
	var itemID string
	err := s.withTx(func(tx *sql.Tx) error {
		if err := tx.QueryRow(`SELECT item_id FROM highlights WHERE id = ?`, id).Scan(&itemID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if _, err := tx.Exec(`UPDATE highlights SET note = ?, updated_at = ? WHERE id = ?`, note, now, id); err != nil {
			return err
		}
		return reindexTx(tx, itemID)
	})
	if err != nil {
		return model.Highlight{}, err
	}
	return s.GetHighlight(id)
}

func (s *Store) DeleteHighlight(id string) error {
	return s.withTx(func(tx *sql.Tx) error {
		var itemID string
		if err := tx.QueryRow(`SELECT item_id FROM highlights WHERE id = ?`, id).Scan(&itemID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if _, err := tx.Exec(`DELETE FROM highlights WHERE id = ?`, id); err != nil {
			return err
		}
		return reindexTx(tx, itemID)
	})
}

func (s *Store) GetPreferences() (model.Preferences, error) {
	var p model.Preferences
	err := s.SQL.QueryRow(`
SELECT theme, font_family, font_size, line_height, column_width, align, updated_at
FROM user_preferences WHERE id = 1
`).Scan(&p.Theme, &p.FontFamily, &p.FontSize, &p.LineHeight, &p.ColumnWidth, &p.Align, &p.UpdatedAt)
	return p, err
}

func (s *Store) SavePreferences(p model.Preferences) (model.Preferences, error) {
	p.UpdatedAt = Now()
	_, err := s.SQL.Exec(`
UPDATE user_preferences SET
  theme = ?, font_family = ?, font_size = ?, line_height = ?, column_width = ?, align = ?, updated_at = ?
WHERE id = 1
`, p.Theme, p.FontFamily, p.FontSize, p.LineHeight, p.ColumnWidth, p.Align, p.UpdatedAt)
	if err != nil {
		return model.Preferences{}, err
	}
	return s.GetPreferences()
}

func (s *Store) Counts(folder, sourceID string) (map[string]int, error) {
	where, args := filterSQL(folder, sourceID)
	rows, err := s.SQL.Query(`
SELECT i.published_day, COUNT(*)
FROM items i
JOIN sources src ON src.id = i.source_id
`+where+`
GROUP BY i.published_day
ORDER BY i.published_day
`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var day string
		var n int
		if err := rows.Scan(&day, &n); err != nil {
			return nil, err
		}
		out[day] = n
	}
	return out, rows.Err()
}

func (s *Store) DayItems(day, folder, sourceID string) ([]model.ItemRow, error) {
	where, args := filterSQL(folder, sourceID)
	args = append(args, day)
	rows, err := s.SQL.Query(rowSelect+where+` AND i.published_day = ? ORDER BY i.published_at DESC, i.id DESC`, args...)
	if err != nil {
		return nil, err
	}
	return collectRows(rows)
}

func (s *Store) Later() ([]model.ItemRow, error) {
	rows, err := s.SQL.Query(rowSelect + `
WHERE src.enabled = 1 AND st.in_later = 1 AND st.read_at IS NULL
ORDER BY st.saved_at DESC, i.id DESC
`)
	if err != nil {
		return nil, err
	}
	return collectRows(rows)
}

// Search returns every matching item, read or unread.
func (s *Store) Search(match string, limit, offset int) ([]model.ItemRow, error) {
	limit, offset = clampPage(limit, offset)
	query := rowSelect + ` WHERE 1 = 1`
	var args []any
	if match != "" {
		query += ` AND i.id IN (SELECT item_id FROM items_fts WHERE items_fts MATCH ?)`
		args = append(args, match)
	}
	query += ` ORDER BY i.published_at DESC, i.id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.SQL.Query(query, args...)
	if err != nil {
		return nil, err
	}
	return collectRows(rows)
}

func (s *Store) Read(match string, limit, offset int) ([]model.ItemRow, error) {
	limit, offset = clampPage(limit, offset)
	query := rowSelect + ` WHERE st.read_at IS NOT NULL`
	var args []any
	if match != "" {
		query += ` AND i.id IN (SELECT item_id FROM items_fts WHERE items_fts MATCH ?)`
		args = append(args, match)
	}
	query += ` ORDER BY st.read_at DESC, i.id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.SQL.Query(query, args...)
	if err != nil {
		return nil, err
	}
	return collectRows(rows)
}

func clampPage(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func (s *Store) NewestLastFetched() (*time.Time, error) {
	var raw sql.NullString
	err := s.SQL.QueryRow(`
SELECT MAX(last_fetched_at) FROM sources
WHERE last_fetched_at IS NOT NULL AND last_fetched_at != ''
`).Scan(&raw)
	if err != nil || !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return nil, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw.String)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, raw.String)
		if err != nil {
			return nil, err
		}
	}
	return &parsed, nil
}

func filterSQL(folder, sourceID string) (string, []any) {
	parts := []string{"WHERE src.enabled = 1"}
	var args []any
	if sourceID != "" {
		parts = append(parts, "src.id = ?")
		args = append(args, sourceID)
	}
	switch folder {
	case "", "All":
	case "RSS":
		parts = append(parts, "src.source_type = 'rss'")
	default:
		parts = append(parts, "src.folder = ?")
		args = append(args, folder)
	}
	return strings.Join(parts, " AND ") + " ", args
}

func collectRows(rows *sql.Rows) ([]model.ItemRow, error) {
	defer rows.Close()
	var out []model.ItemRow
	for rows.Next() {
		item, source, state, count, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, model.ItemRow{Item: item, Source: source, State: state, HighlightCount: count})
	}
	if out == nil {
		out = []model.ItemRow{}
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSource(sc scanner) (model.Source, error) {
	var src model.Source
	var feed, alias, config, fetched, etag, modified, lastError, lastStatus sql.NullString
	var enabled int
	err := sc.Scan(
		&src.ID, &src.Name, &src.SourceType, &src.Folder, &src.DefaultOpenMode, &src.DefaultQueue,
		&feed, &alias, &config, &enabled, &fetched, &etag, &modified, &lastError, &lastStatus, &src.CreatedAt, &src.UpdatedAt,
	)
	if err != nil {
		return model.Source{}, err
	}
	src.FeedURL = StringOf(feed)
	src.EmailAlias = StringOf(alias)
	src.Enabled = enabled == 1
	src.LastFetchedAt = StringPtr(fetched)
	src.ETag = StringOf(etag)
	src.LastModified = StringOf(modified)
	src.LastError = StringOf(lastError)
	src.LastStatus = StringOf(lastStatus)
	src.Config = map[string]any{}
	if config.Valid && config.String != "" {
		_ = json.Unmarshal([]byte(config.String), &src.Config)
	}
	return src, nil
}

func scanRow(sc scanner) (model.Item, model.Source, model.UserItemState, int, error) {
	var item model.Item
	var discussion, markdown, excerpt, meta sql.NullString
	var source model.Source
	var feed, alias, config, fetched, etag, modified, lastError, lastStatus sql.NullString
	var enabled, inLater int
	var saved, read sql.NullString
	var progress float64
	var count int
	err := sc.Scan(
		&item.ID, &item.SourceID, &item.ExternalID, &item.CanonicalURL, &discussion,
		&item.Title, &item.Author, &item.PublishedAt, &item.PublishedDay, &item.OpenMode, &item.ActionURL,
		&markdown, &item.PlainText, &excerpt, &meta, &item.CreatedAt, &item.UpdatedAt,
		&source.ID, &source.Name, &source.SourceType, &source.Folder, &source.DefaultOpenMode, &source.DefaultQueue,
		&feed, &alias, &config, &enabled, &fetched, &etag, &modified, &lastError, &lastStatus, &source.CreatedAt, &source.UpdatedAt,
		&inLater, &saved, &read, &progress, &count,
	)
	if err != nil {
		return model.Item{}, model.Source{}, model.UserItemState{}, 0, err
	}
	item.DiscussionURL = StringOf(discussion)
	item.ContentMarkdown = StringOf(markdown)
	item.Excerpt = StringOf(excerpt)
	if meta.Valid && meta.String != "" {
		item.Metadata = json.RawMessage(meta.String)
	} else {
		item.Metadata = json.RawMessage(`{"kind":"generic"}`)
	}
	decorateItem(&item)
	source.FeedURL = StringOf(feed)
	source.EmailAlias = StringOf(alias)
	source.Enabled = enabled == 1
	source.LastFetchedAt = StringPtr(fetched)
	source.ETag = StringOf(etag)
	source.LastModified = StringOf(modified)
	source.LastError = StringOf(lastError)
	source.LastStatus = StringOf(lastStatus)
	source.Config = map[string]any{}
	if config.Valid && config.String != "" {
		_ = json.Unmarshal([]byte(config.String), &source.Config)
	}
	state := model.UserItemState{
		InLater:      inLater == 1,
		SavedAt:      StringPtr(saved),
		ReadAt:       StringPtr(read),
		ReadProgress: progress,
	}
	return item, source, state, count, nil
}

func decorateItem(item *model.Item) {
	var extra struct {
		TractionNote     string                  `json:"tractionNote"`
		DiscussionQuotes []model.DiscussionQuote `json:"discussionQuotes"`
	}
	_ = json.Unmarshal(item.Metadata, &extra)
	if item.TractionNote == "" {
		item.TractionNote = extra.TractionNote
	}
	if len(item.DiscussionQuotes) == 0 && len(extra.DiscussionQuotes) > 0 {
		item.DiscussionQuotes = extra.DiscussionQuotes
	}
}

func stateFromInitial(initial *model.InitialState, now string) (int, sql.NullString, sql.NullString, float64) {
	if initial == nil {
		return 0, sql.NullString{}, sql.NullString{}, 0
	}
	inLater := 0
	var saved sql.NullString
	if initial.InLater != nil && *initial.InLater {
		inLater = 1
		saved = sql.NullString{String: now, Valid: true}
	}
	return inLater, saved, nullablePtr(initial.ReadAt), 0
}

func applyInitialTx(tx *sql.Tx, id string, initial *model.InitialState, now string) error {
	var inLater int
	var saved, read sql.NullString
	var progress float64
	if err := tx.QueryRow(`SELECT in_later, saved_at, read_at, read_progress FROM user_item_state WHERE item_id = ?`, id).
		Scan(&inLater, &saved, &read, &progress); err != nil {
		return err
	}
	if initial.InLater != nil {
		if *initial.InLater {
			inLater = 1
			if !saved.Valid {
				saved = sql.NullString{String: now, Valid: true}
			}
		} else {
			inLater = 0
			saved = sql.NullString{}
		}
	}
	if initial.ReadAt != nil {
		read = sql.NullString{String: *initial.ReadAt, Valid: *initial.ReadAt != ""}
	}
	_, err := tx.Exec(`
UPDATE user_item_state SET in_later = ?, saved_at = ?, read_at = ?, updated_at = ? WHERE item_id = ?
`, inLater, saved, read, now, id)
	return err
}

func reindexTx(tx *sql.Tx, itemID string) error {
	var rowid int64
	var title, author, plain, meta string
	if err := tx.QueryRow(`SELECT rowid, title, author, plain_text, metadata_json FROM items WHERE id = ?`, itemID).
		Scan(&rowid, &title, &author, &plain, &meta); err != nil {
		return err
	}
	rows, err := tx.Query(`SELECT quote, note FROM highlights WHERE item_id = ?`, itemID)
	if err != nil {
		return err
	}
	var notes []string
	for rows.Next() {
		var quote, note string
		if err := rows.Scan(&quote, &note); err != nil {
			rows.Close()
			return err
		}
		notes = append(notes, quote, note)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	// item_id is an UNINDEXED FTS column, so deleting by it scans the whole table.
	// The FTS rowid is items.rowid, which is a direct lookup.
	if _, err := tx.Exec(`DELETE FROM items_fts WHERE rowid = ?`, rowid); err != nil {
		return err
	}
	_, err = tx.Exec(`
INSERT INTO items_fts (rowid, item_id, title, author, plain_text, metadata_text, highlights_text)
VALUES (?, ?, ?, ?, ?, ?, ?)
`, rowid, itemID, title, author, plain, metadataText(json.RawMessage(meta)), strings.Join(notes, "\n"))
	return err
}

func metadataText(raw json.RawMessage) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return ""
	}
	var b strings.Builder
	walkText(v, &b)
	return b.String()
}

func walkText(v any, b *strings.Builder) {
	switch t := v.(type) {
	case string:
		if strings.TrimSpace(t) != "" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(t)
		}
	case []any:
		for _, item := range t {
			walkText(item, b)
		}
	case map[string]any:
		for _, item := range t {
			walkText(item, b)
		}
	}
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func nullablePtr(v *string) sql.NullString {
	if v == nil || strings.TrimSpace(*v) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *v, Valid: true}
}

func clampProgress(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func MatchQuery(q string) string {
	fields := strings.Fields(q)
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.Trim(field, `"`)
		if field == "" {
			continue
		}
		parts = append(parts, `"`+strings.ReplaceAll(field, `"`, `""`)+`"`)
	}
	return strings.Join(parts, " AND ")
}

func Days(counts map[string]int) (earliest, latest string, days []string) {
	for day := range counts {
		days = append(days, day)
	}
	if len(days) == 0 {
		return "", "", nil
	}
	for i := 1; i < len(days); i++ {
		j := i
		for j > 0 && days[j] < days[j-1] {
			days[j], days[j-1] = days[j-1], days[j]
			j--
		}
	}
	return days[0], days[len(days)-1], days
}

func NeighborDays(days []string, day string) (prev, next *string) {
	for _, candidate := range days {
		if candidate < day {
			c := candidate
			prev = &c
		} else if candidate > day && next == nil {
			c := candidate
			next = &c
		}
	}
	return prev, next
}

func MinPoints(config map[string]any) int {
	if config == nil {
		return 50
	}
	switch v := config["minPoints"].(type) {
	case float64:
		if v > 0 {
			return int(v)
		}
	case int:
		if v > 0 {
			return v
		}
	case json.Number:
		n, _ := v.Int64()
		if n > 0 {
			return int(n)
		}
	}
	return 50
}
