package db

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"elephant/internal/model"
)

func TestUpdateItemRewritesPublishedDay(t *testing.T) {
	store := openTestDB(t)
	if _, err := store.InsertSource(model.Source{
		ID: "src_pod", Name: "Latent Space", SourceType: "podcast", Folder: "Podcasts",
		DefaultOpenMode: "deeplink", DefaultQueue: "feed", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	inserted, err := store.InsertItem(ItemWrite{
		ID: "item_ep", SourceID: "src_pod", ExternalID: "ep-2022",
		CanonicalURL: "https://example.com/ep", Title: "Libsyn episode", Author: "Show",
		PublishedAt: "2026-11-27T18:00:00Z", PublishedDay: "2026-11-27",
		OpenMode: "deeplink", ActionURL: "https://example.com/ep",
		PlainText: "original notes", Metadata: json.RawMessage(`{"kind":"podcast"}`),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := store.UpdateItem(inserted.ID, ItemWrite{
		SourceID: "src_pod", ExternalID: "ep-2022",
		CanonicalURL: "https://example.com/ep", Title: "Libsyn episode retitled", Author: "Show",
		PublishedAt: "2022-03-04T15:00:00Z", PublishedDay: "2022-03-04",
		OpenMode: "deeplink", ActionURL: "https://example.com/ep",
		PlainText: "corrected notes", Metadata: json.RawMessage(`{"kind":"podcast"}`),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if updated.PublishedDay != "2022-03-04" || !strings.HasPrefix(updated.PublishedAt, "2022-03-04T15:00:00") {
		t.Fatalf("item %+v", updated)
	}
	var at, day string
	if err := store.SQL.QueryRow(`SELECT published_at, published_day FROM items WHERE id = ?`, inserted.ID).Scan(&at, &day); err != nil {
		t.Fatal(err)
	}
	if at != "2022-03-04T15:00:00Z" || day != "2022-03-04" {
		t.Fatalf("stored at=%s day=%s", at, day)
	}
	var ftsTitle, ftsPlain string
	if err := store.SQL.QueryRow(`SELECT title, plain_text FROM items_fts WHERE item_id = ?`, inserted.ID).Scan(&ftsTitle, &ftsPlain); err != nil {
		t.Fatal(err)
	}
	if ftsTitle != "Libsyn episode retitled" || ftsPlain != "corrected notes" {
		t.Fatalf("fts title=%q plain=%q", ftsTitle, ftsPlain)
	}
}

func TestMigratePurgesYouTubeShorts(t *testing.T) {
	store := openTestDB(t)
	if _, err := store.InsertSource(model.Source{
		ID: "src_yt", Name: "Channel", SourceType: "youtube", Folder: "Video",
		DefaultOpenMode: "external", DefaultQueue: "feed", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	meta := json.RawMessage(`{"kind":"youtube"}`)
	shortItem, err := store.InsertItem(ItemWrite{
		ID: "item_short", SourceID: "src_yt", ExternalID: "short",
		CanonicalURL: "https://www.youtube.com/shorts/aaa", Title: "A short",
		PublishedAt: "2022-03-04T15:00:00Z", PublishedDay: "2022-03-04",
		OpenMode: "external", ActionURL: "https://www.youtube.com/shorts/aaa",
		PlainText: "short text", Metadata: meta,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddHighlight(model.Highlight{ID: "hl_short", ItemID: shortItem.ID, Quote: "clip"}); err != nil {
		t.Fatal(err)
	}
	actionShort, err := store.InsertItem(ItemWrite{
		ID: "item_action", SourceID: "src_yt", ExternalID: "action",
		CanonicalURL: "https://www.youtube.com/watch?v=action", Title: "Hidden short",
		PublishedAt: "2022-03-04T15:00:00Z", PublishedDay: "2022-03-04",
		OpenMode: "external", ActionURL: "https://m.youtube.com/Shorts/action",
		PlainText: "action text", Metadata: meta,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	kept, err := store.InsertItem(ItemWrite{
		ID: "item_keep", SourceID: "src_yt", ExternalID: "keep",
		CanonicalURL: "https://www.youtube.com/watch?v=fff", Title: "One minute robotics demo",
		PublishedAt: "2022-03-04T15:00:00Z", PublishedDay: "2022-03-04",
		OpenMode: "external", ActionURL: "https://www.youtube.com/watch?v=fff",
		PlainText: "robotics demo", Metadata: meta,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddHighlight(model.Highlight{ID: "hl_keep", ItemID: kept.ID, Quote: "keep me"}); err != nil {
		t.Fatal(err)
	}

	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := store.GetItem(shortItem.ID); err != ErrNotFound {
		t.Fatalf("short item err %v", err)
	}
	if _, _, _, _, err := store.GetItem(actionShort.ID); err != ErrNotFound {
		t.Fatalf("action short err %v", err)
	}
	got, _, _, highlights, err := store.GetItem(kept.ID)
	if err != nil || got.Title != "One minute robotics demo" || len(highlights) != 1 {
		t.Fatalf("kept %+v highlights %d err %v", got, len(highlights), err)
	}
	if n := countQuery(t, store, `SELECT COUNT(*) FROM items`); n != 1 {
		t.Fatalf("items %d", n)
	}
	if n := countQuery(t, store, `SELECT COUNT(*) FROM user_item_state`); n != 1 {
		t.Fatalf("state %d", n)
	}
	if n := countQuery(t, store, `SELECT COUNT(*) FROM highlights`); n != 1 {
		t.Fatalf("highlights %d", n)
	}
	if n := countQuery(t, store, `SELECT COUNT(*) FROM items_fts`); n != 1 {
		t.Fatalf("fts %d", n)
	}
	if n := countQuery(t, store, `SELECT COUNT(*) FROM items_fts WHERE item_id = ?`, shortItem.ID); n != 0 {
		t.Fatalf("short fts %d", n)
	}
	var ftsTitle string
	if err := store.SQL.QueryRow(`SELECT title FROM items_fts WHERE item_id = ?`, kept.ID).Scan(&ftsTitle); err != nil {
		t.Fatal(err)
	}
	if ftsTitle != "One minute robotics demo" {
		t.Fatalf("fts %q", ftsTitle)
	}
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	if n := countQuery(t, store, `SELECT COUNT(*) FROM items`); n != 1 {
		t.Fatalf("second migrate items %d", n)
	}
}

func countQuery(t *testing.T, store *Store, query string, args ...any) int {
	t.Helper()
	var n int
	if err := store.SQL.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func openTestDB(t *testing.T) *Store {
	t.Helper()
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	store, err := Open(filepath.Join(t.TempDir(), "elephant.db"), loc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}
