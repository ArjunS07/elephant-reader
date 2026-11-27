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
	if _, err := s.SQL.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`); err != nil {
		return err
	}
	body, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return err
	}
	if _, err := s.SQL.Exec(string(body)); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if err := s.addColumn("sources", "etag", "TEXT"); err != nil {
		return err
	}
	if err := s.addColumn("sources", "last_modified", "TEXT"); err != nil {
		return err
	}
	return s.purgeYouTubeShorts()
}

// purgeYouTubeShorts deletes items whose canonical or action URL is a
// YouTube Short. items_fts is a standalone table, so those rows are removed
// explicitly. user_item_state and highlights follow items via ON DELETE CASCADE.
// The statement matches nothing on a clean database and is safe to repeat.
func (s *Store) purgeYouTubeShorts() error {
	if _, err := s.SQL.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		return err
	}
	const match = `canonical_url LIKE '%/shorts/%' OR action_url LIKE '%/shorts/%'`
	return s.withTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM items_fts WHERE item_id IN (SELECT id FROM items WHERE ` + match + `)`); err != nil {
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
