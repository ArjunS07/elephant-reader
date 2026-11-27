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
	return nil
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
