package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"elephant/internal/db"
	"elephant/internal/model"
)

func TestSyncCapsRetriesAndHash(t *testing.T) {
	var hits atomic.Int32
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		switch r.URL.Path {
		case "/retry":
			if n < 3 {
				http.Error(w, "busy", http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write(rssItem("Retried", "https://example.com/retry", "retry-1", time.Now().UTC(), "finally"))
		case "/always":
			http.Error(w, "nope", http.StatusBadGateway)
		case "/change":
			word := "alpha"
			if n > 1 {
				word = "beta"
			}
			_, _ = w.Write(rssItem("Changing", "https://example.com/change", "change-1", time.Now().UTC(), word))
		default:
			_, _ = w.Write(rssFeed(30, func(i int) time.Time {
				return time.Now().UTC().Add(-time.Duration(i) * 24 * time.Hour)
			}))
		}
	}))
	defer feed.Close()

	store := openSvcDB(t)
	svc := New(store)
	svc.Backoff = func(int) time.Duration { return 0 }
	svc.Opts = SyncOptions{MaxItems: 25, BackfillDays: 60, MinItems: 5}
	if _, err := store.InsertSource(model.Source{
		ID: "src_big", Name: "Big", SourceType: "rss", Folder: "RSS", DefaultOpenMode: "reader", DefaultQueue: "feed",
		FeedURL: feed.URL + "/big", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	added, _, err := svc.SyncSource(context.Background(), mustSource(t, store, "src_big"))
	if err != nil || added != 30 {
		t.Fatalf("first sync added %d %v", added, err)
	}
	var n int
	if err := store.SQL.QueryRow(`SELECT COUNT(*) FROM items WHERE source_id = 'src_big'`).Scan(&n); err != nil || n != 30 {
		t.Fatalf("kept %d %v", n, err)
	}
	before := itemUpdatedAt(t, store, "src_big")
	added, _, err = svc.SyncSource(context.Background(), mustSource(t, store, "src_big"))
	if err != nil || added != 0 {
		t.Fatalf("second sync added %d %v", added, err)
	}
	if err := store.SQL.QueryRow(`SELECT COUNT(*) FROM items WHERE source_id = 'src_big'`).Scan(&n); err != nil || n != 30 {
		t.Fatalf("second sync deleted history %d %v", n, err)
	}
	if after := itemUpdatedAt(t, store, "src_big"); after != before {
		t.Fatalf("unchanged item rewritten %s -> %s", before, after)
	}

	hits.Store(0)
	if _, err := store.InsertSource(model.Source{
		ID: "src_retry", Name: "Retry", SourceType: "podcast", Folder: "Podcasts", DefaultOpenMode: "deeplink", DefaultQueue: "feed",
		FeedURL: feed.URL + "/retry", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	added, _, err = svc.SyncSource(context.Background(), mustSource(t, store, "src_retry"))
	if err != nil || added != 1 || hits.Load() != 3 {
		t.Fatalf("retry added %d hits %d %v", added, hits.Load(), err)
	}
	var author, mode string
	if err := store.SQL.QueryRow(`SELECT author, open_mode FROM items WHERE source_id = 'src_retry'`).Scan(&author, &mode); err != nil || author != "Retry" || mode != "reader" {
		t.Fatalf("podcast author %q mode %q %v", author, mode, err)
	}
	src := mustSource(t, store, "src_retry")
	if src.LastStatus != "ok" || src.LastError != "" {
		t.Fatalf("status %+v", src)
	}

	hits.Store(0)
	if _, err := store.InsertSource(model.Source{
		ID: "src_down", Name: "Down", SourceType: "rss", Folder: "RSS", DefaultOpenMode: "reader", DefaultQueue: "feed",
		FeedURL: feed.URL + "/always", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.SyncSource(context.Background(), mustSource(t, store, "src_down")); err == nil {
		t.Fatal("expected failure")
	}
	if hits.Load() != 3 {
		t.Fatalf("retries %d", hits.Load())
	}
	down := mustSource(t, store, "src_down")
	if down.LastStatus != "error" || down.LastError == "" || down.LastFetchedAt != nil {
		t.Fatalf("error status %+v", down)
	}

	hits.Store(0)
	if _, err := store.InsertSource(model.Source{
		ID: "src_change", Name: "Change", SourceType: "rss", Folder: "RSS", DefaultOpenMode: "reader", DefaultQueue: "feed",
		FeedURL: feed.URL + "/change", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.SyncSource(context.Background(), mustSource(t, store, "src_change")); err != nil {
		t.Fatal(err)
	}
	stamped := itemUpdatedAt(t, store, "src_change")
	if _, _, err := svc.SyncSource(context.Background(), mustSource(t, store, "src_change")); err != nil {
		t.Fatal(err)
	}
	if again := itemUpdatedAt(t, store, "src_change"); again == stamped {
		t.Fatal("changed item was skipped")
	}
	found, err := store.Search(db.MatchQuery("beta"), 10, 0)
	if err != nil || len(found) != 1 {
		t.Fatalf("beta %+v %v", found, err)
	}
}

func TestBatchIngestIsAtomic(t *testing.T) {
	store := openSvcDB(t)
	svc := New(store)
	if _, err := store.InsertSource(model.Source{
		ID: "src", Name: "Src", SourceType: "rss", Folder: "RSS", DefaultOpenMode: "reader", DefaultQueue: "feed", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Ingest(context.Background(), []model.IngestItem{
		{SourceID: "src", ExternalID: "a", CanonicalURL: "https://example.com/a", Title: "Good", PublishedAt: "2026-11-01T00:00:00Z"},
		{SourceID: "src", ExternalID: "b", CanonicalURL: "https://example.com/b", Title: "Bad", PublishedAt: "not-a-date"},
	})
	if err == nil {
		t.Fatal("expected batch error")
	}
	var n int
	if err := store.SQL.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("partial commit %d %v", n, err)
	}
	results, err := svc.Ingest(context.Background(), []model.IngestItem{
		{SourceID: "src", ExternalID: "a", CanonicalURL: "https://example.com/shared", Title: "First", PublishedAt: "2026-11-01T00:00:00Z", PlainText: "first body"},
		{SourceID: "src", ExternalID: "a", CanonicalURL: "https://example.com/shared", Title: "Second", PublishedAt: "2026-11-01T00:00:00Z", PlainText: "second body"},
		{SourceID: "src", ExternalID: "b", CanonicalURL: "https://example.com/shared", Title: "Other guid", PublishedAt: "2026-11-02T00:00:00Z", PlainText: "other body"},
	})
	if err != nil || len(results) != 3 || !results[0].Created || results[1].Created || results[0].Item.ID != results[1].Item.ID || results[2].Item.ID == results[0].Item.ID {
		t.Fatalf("%+v %v", results, err)
	}
	if results[1].Item.Title != "Second" {
		t.Fatalf("title %s", results[1].Item.Title)
	}
	if err := store.SQL.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("rows %d %v", n, err)
	}
}

func TestBackgroundSyncAndCatchUp(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	var hits atomic.Int32
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			close(started)
		}
		<-release
		_, _ = w.Write(rssItem("Held", "https://example.com/held", "held-1", time.Now().UTC(), "held body"))
	}))
	defer feed.Close()
	store := openSvcDB(t)
	svc := New(store)
	svc.Backoff = func(int) time.Duration { return 0 }
	if _, err := store.InsertSource(model.Source{
		ID: "src", Name: "Held", SourceType: "rss", Folder: "RSS", DefaultOpenMode: "reader", DefaultQueue: "feed",
		FeedURL: feed.URL, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	status := svc.StartSync()
	if !status.Running {
		t.Fatalf("not running %+v", status)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("sync did not start")
	}
	again := svc.StartSync()
	if !again.Running {
		t.Fatal("second start lost the job")
	}
	close(release)
	waitStatus(t, svc)
	if hits.Load() != 1 {
		t.Fatalf("sync ran %d times", hits.Load())
	}
	if st := svc.Status(); st.Running || st.Failed != 0 || st.Completed != 1 || st.Total != 1 || st.FinishedAt == nil || len(st.Errors) != 0 {
		t.Fatalf("status %+v", st)
	}

	t.Setenv("ELEPHANT_SYNC_ON_START", "0")
	svc.CatchUp()
	time.Sleep(30 * time.Millisecond)
	if hits.Load() != 1 || svc.Status().Running {
		t.Fatal("disabled catch-up started")
	}

	t.Setenv("ELEPHANT_SYNC_ON_START", "1")
	svc.CatchUp()
	time.Sleep(40 * time.Millisecond)
	if hits.Load() != 1 {
		t.Fatalf("fresh fetch was repeated: %d", hits.Load())
	}

	old := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339Nano)
	if _, err := store.PatchSource("src", db.SourcePatch{TouchFetched: true, LastFetchedAt: &old}); err != nil {
		t.Fatal(err)
	}
	svc.CatchUp()
	waitStatus(t, svc)
	if hits.Load() < 2 {
		t.Fatalf("stale catch-up did not run: %d", hits.Load())
	}
}

func TestSyncOptionsFromEnv(t *testing.T) {
	t.Setenv("ELEPHANT_MAX_ITEMS_PER_FEED", "7")
	t.Setenv("ELEPHANT_BACKFILL_DAYS", "14")
	opts := SyncOptionsFromEnv()
	if opts.MaxItems != 7 || opts.BackfillDays != 14 || opts.MinItems != 5 {
		t.Fatalf("%+v", opts)
	}
	if defaultBackoff(1) <= 0 || !retryableStatus(http.StatusServiceUnavailable) || retryableStatus(http.StatusInternalServerError) {
		t.Fatal("retry policy")
	}
}

func TestOldCatalogKeepsMinimum(t *testing.T) {
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(rssFeed(30, func(i int) time.Time {
			return time.Now().UTC().AddDate(-8, 0, -i)
		}))
	}))
	defer feed.Close()
	store := openSvcDB(t)
	svc := New(store)
	svc.Opts = SyncOptions{MaxItems: 25, BackfillDays: 60, MinItems: 5}
	if _, err := store.InsertSource(model.Source{
		ID: "src_hh", Name: "Hardcore History", SourceType: "podcast", Folder: "Podcasts",
		DefaultOpenMode: "deeplink", DefaultQueue: "feed", FeedURL: feed.URL, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.SyncSource(context.Background(), mustSource(t, store, "src_hh")); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := store.SQL.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&n); err != nil || n != 5 {
		t.Fatalf("kept %d %v", n, err)
	}
}

func rssItem(title, link, guid string, when time.Time, description string) []byte {
	return []byte(`<?xml version="1.0"?><rss version="2.0"><channel><item><title>` + title + `</title><link>` + link + `</link><guid>` + guid + `</guid><pubDate>` + when.UTC().Format(time.RFC3339) + `</pubDate><description>` + description + `</description></item></channel></rss>`)
}

func mustSource(t *testing.T, store *db.Store, id string) model.Source {
	t.Helper()
	source, err := store.GetSource(id)
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func itemUpdatedAt(t *testing.T, store *db.Store, sourceID string) string {
	t.Helper()
	var stamp string
	if err := store.SQL.QueryRow(`SELECT updated_at FROM items WHERE source_id = ? ORDER BY updated_at DESC LIMIT 1`, sourceID).Scan(&stamp); err != nil {
		t.Fatal(err)
	}
	return stamp
}

func waitStatus(t *testing.T, svc *Service) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !svc.Status().Running {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("still running %+v", svc.Status())
}

func openSvcDB(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.Open(filepath.Join(t.TempDir(), "svc.db"), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}
