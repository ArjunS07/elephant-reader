package db

import (
	"database/sql"
	"embed"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

type Store struct {
	SQL *sql.DB
	Loc *time.Location
}

func Open(path string, loc *time.Location) (*Store, error) {
	if loc == nil {
		loc = time.UTC
	}
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=journal_mode(WAL)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	store := &Store{SQL: sqlDB, Loc: loc}
	if err := store.Migrate(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.SQL.Close()
}

func (s *Store) Migrate() error {
	if _, err := s.SQL.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA synchronous=NORMAL;`); err != nil {
		return err
	}
	body, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return err
	}
	if _, err := s.SQL.Exec(string(body)); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	for _, col := range []struct {
		table, column, decl string
	}{
		{"sources", "etag", "TEXT"},
		{"sources", "last_modified", "TEXT"},
		{"sources", "last_error", "TEXT"},
		{"sources", "last_status", "TEXT"},
		{"items", "content_hash", "TEXT"},
	} {
		if err := s.addColumn(col.table, col.column, col.decl); err != nil {
			return err
		}
	}
	// Existing databases created a global UNIQUE index on items.canonical_url.
	// Drop it on every startup; the statement is a no-op once the index is gone.
	if _, err := s.SQL.Exec(`DROP INDEX IF EXISTS items_canonical_url`); err != nil {
		return err
	}
	if _, err := s.SQL.Exec(`CREATE INDEX IF NOT EXISTS items_source_canonical ON items(source_id, canonical_url)`); err != nil {
		return err
	}
	if err := s.rebuildFTSOnce(); err != nil {
		return err
	}
	return s.purgeYouTubeShorts()
}

// rebuildFTSOnce rewrites items_fts so each row's FTS rowid matches items.rowid.
// Deletes can then target the rowid directly instead of scanning the unindexed item_id column.
func (s *Store) rebuildFTSOnce() error {
	const id = "fts_rowid_v1"
	var n int
	if err := s.SQL.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE id = ?`, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	if err := s.withTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM items_fts`); err != nil {
			return err
		}
		rows, err := tx.Query(`SELECT id FROM items`)
		if err != nil {
			return err
		}
		var ids []string
		for rows.Next() {
			var itemID string
			if err := rows.Scan(&itemID); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, itemID)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, itemID := range ids {
			if err := reindexTx(tx, itemID); err != nil {
				return err
			}
		}
		_, err = tx.Exec(`INSERT INTO schema_migrations (id, applied_at) VALUES (?, ?)`, id, Now())
		return err
	}); err != nil {
		return fmt.Errorf("rebuild fts: %w", err)
	}
	return nil
}

// purgeYouTubeShorts deletes items whose canonical or action URL is a
// YouTube Short. items_fts is a standalone table, so those rows are removed
// by items.rowid. user_item_state and highlights follow items via ON DELETE CASCADE.
// The statement matches nothing on a clean database and is safe to repeat.
func (s *Store) purgeYouTubeShorts() error {
	if _, err := s.SQL.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		return err
	}
	const match = `canonical_url LIKE '%/shorts/%' OR action_url LIKE '%/shorts/%'`
	return s.withTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM items_fts WHERE rowid IN (SELECT rowid FROM items WHERE ` + match + `)`); err != nil {
			return fmt.Errorf("purge youtube shorts fts: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM items WHERE ` + match); err != nil {
			return fmt.Errorf("purge youtube shorts: %w", err)
		}
		return nil
	})
}

func (s *Store) addColumn(table, column, decl string) error {
	rows, err := s.SQL.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			return err
		}
		if strings.EqualFold(name, column) {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.SQL.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + decl)
	return err
}

func (s *Store) withTx(fn func(*sql.Tx) error) error {
	tx, err := s.SQL.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func Now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func NullString(v string) sql.NullString {
	v = strings.TrimSpace(v)
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}

func StringOf(v sql.NullString) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func StringPtr(v sql.NullString) *string {
	if !v.Valid || strings.TrimSpace(v.String) == "" {
		return nil
	}
	s := v.String
	return &s
}
