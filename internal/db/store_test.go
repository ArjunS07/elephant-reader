package db

import (
	"database/sql"
	"errors"
	"testing"

	"elephant/internal/model"
)

func TestCanonicalURLIsPerSource(t *testing.T) {
	store := openTestDB(t)
	if _, err := store.SQL.Exec(`CREATE UNIQUE INDEX items_canonical_url ON items(canonical_url)`); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	var name string
	err := store.SQL.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'index' AND name = 'items_canonical_url'`).Scan(&name)
	if err != sql.ErrNoRows {
		t.Fatalf("global unique index: %v %q", err, name)
	}
	rows, err := store.SQL.Query(`PRAGMA index_list(items)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var seq, unique, partial int
		var indexName, origin string
		if err := rows.Scan(&seq, &indexName, &unique, &origin, &partial); err != nil {
			t.Fatal(err)
		}
		if indexName == "items_source_canonical" {
			found = true
			if unique != 0 {
				t.Fatalf("source canonical index unique=%d", unique)
			}
		}
	}
	if !found {
		t.Fatal("missing items_source_canonical")
	}

	for _, id := range []string{"src_hn", "src_digest"} {
		if _, err := store.InsertSource(model.Source{
			ID: id, Name: id, SourceType: "rss", Folder: "RSS", DefaultOpenMode: "reader", DefaultQueue: "feed", Enabled: true,
		}); err != nil {
			t.Fatal(err)
		}
	}
	const link = "https://arxiv.org/abs/2511.18421"
	first, err := store.InsertItem(ItemWrite{
		ID: "item_hn", SourceID: "src_hn", ExternalID: "hn-1", CanonicalURL: link,
		Title: "HN row", Author: "alice", PublishedAt: "2026-11-26T18:00:00Z", PublishedDay: "2026-11-26",
		OpenMode: "external", ActionURL: link, PlainText: "hn walrus", Metadata: []byte(`{"kind":"hn"}`),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.InsertItem(ItemWrite{
		ID: "item_digest", SourceID: "src_digest", ExternalID: "digest-1", CanonicalURL: link,
		Title: "Digest row", Author: "bob", PublishedAt: "2026-11-26T18:00:00Z", PublishedDay: "2026-11-26",
		OpenMode: "reader", ActionURL: link, PlainText: "digest walrus", Metadata: []byte(`{"kind":"generic"}`),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatal("cross-source url collapsed")
	}
	third, err := store.InsertItem(ItemWrite{
		ID: "item_hn_2", SourceID: "src_hn", ExternalID: "hn-2", CanonicalURL: link,
		Title: "Second episode", Author: "alice", PublishedAt: "2026-11-20T18:00:00Z", PublishedDay: "2026-11-20",
		OpenMode: "external", ActionURL: link, PlainText: "episode two", Metadata: []byte(`{"kind":"hn"}`),
	}, nil)
	if err != nil || third.ID == first.ID {
		t.Fatalf("same-source distinct guid collapsed: %+v %v", third, err)
	}

	got, err := store.FindItemID("src_hn", "hn-1", link)
	if err != nil || got != first.ID {
		t.Fatalf("external id lookup %s %v", got, err)
	}
	if _, err := store.FindItemID("src_digest", "hn-1", link); err != ErrNotFound {
		t.Fatalf("cross-source external id: %v", err)
	}
	if _, err := store.FindItemID("src_hn", "missing-guid", link); err != ErrNotFound {
		t.Fatalf("distinct guid fell back to shared url: %v", err)
	}
	byURL, err := store.FindItemID("src_digest", "", link)
	if err != nil || byURL != second.ID {
		t.Fatalf("same-source url fallback %s %v", byURL, err)
	}
	if _, err := store.FindItemID("src_other", "", link); err != ErrNotFound {
		t.Fatalf("url fallback crossed sources: %v", err)
	}

	unread, err := store.Search(MatchQuery("walrus"), 10, 0)
	if err != nil || len(unread) != 2 {
		t.Fatalf("search all %d %v", len(unread), err)
	}
	readOnly, err := store.Read(MatchQuery("walrus"), 10, 0)
	if err != nil || len(readOnly) != 0 {
		t.Fatalf("read archive %d %v", len(readOnly), err)
	}

	updated, err := store.UpdateItem(first.ID, ItemWrite{
		ID: first.ID, SourceID: "src_hn", ExternalID: "hn-1", CanonicalURL: link,
		Title: "HN row revised", Author: "alice", PublishedAt: "2026-11-26T18:00:00Z", PublishedDay: "2026-11-26",
		OpenMode: "external", ActionURL: link, PlainText: "hn otter", Metadata: []byte(`{"kind":"hn"}`),
	}, nil)
	if err != nil || updated.Title != "HN row revised" {
		t.Fatalf("update %+v %v", updated, err)
	}
	hits, err := store.Search(MatchQuery("otter"), 10, 0)
	if err != nil || len(hits) != 1 || hits[0].Item.ID != first.ID {
		t.Fatalf("updated search %+v %v", hits, err)
	}
	still, err := store.Search(MatchQuery("digest"), 10, 0)
	if err != nil || len(still) != 1 || still[0].Item.ID != second.ID {
		t.Fatalf("other fts row %+v %v", still, err)
	}
	gone, err := store.Search(MatchQuery("walrus"), 10, 0)
	if err != nil || len(gone) != 1 || gone[0].Item.ID != second.ID {
		t.Fatalf("stale fts %+v %v", gone, err)
	}
}

func TestDeleteByExternalIDPrefixExcept(t *testing.T) {
	store := openTestDB(t)
	for _, id := range []string{"src_papers", "src_hn"} {
		if _, err := store.InsertSource(model.Source{
			ID: id, Name: id, SourceType: "rss", Folder: "RSS", DefaultOpenMode: "reader", DefaultQueue: "feed", Enabled: true,
		}); err != nil {
			t.Fatal(err)
		}
	}
	keepExt := "mldigest:2026-11-27:papers:finedex"
	orphanExt := "mldigest:2026-11-27:papers:2511.18421"
	otherDay := "mldigest:2026-11-26:papers:other"
	inLater := true
	insert := func(id, source, ext, title, day string, initial *model.InitialState) model.Item {
		t.Helper()
		item, err := store.InsertItem(ItemWrite{
			ID: id, SourceID: source, ExternalID: ext,
			CanonicalURL: "https://example.com/" + id, Title: title,
			PublishedAt: day + "T22:00:00Z", PublishedDay: day,
			OpenMode: "reader", ActionURL: "https://example.com/" + id,
			PlainText: title + " body", Excerpt: title,
		}, initial)
		if err != nil {
			t.Fatal(err)
		}
		return item
	}
	keep := insert("item_keep", "src_papers", keepExt, "FineDex", "2026-11-27", &model.InitialState{InLater: &inLater})
	insert("item_orphan", "src_papers", orphanExt, "FineDex old", "2026-11-27", nil)
	other := insert("item_otherday", "src_papers", otherDay, "Other day", "2026-11-26", nil)
	hn := insert("item_hn", "src_hn", "hn-99", "HN stay", "2026-11-27", nil)

	if _, err := store.DeleteByExternalIDPrefixExcept("mldigest:", []string{keepExt}); !errors.Is(err, ErrInvalidReplaceScope) {
		t.Fatalf("wide prefix: %v", err)
	}
	if _, err := store.DeleteByExternalIDPrefixExcept("mldigest:2026-11-27:%", []string{keepExt}); !errors.Is(err, ErrInvalidReplaceScope) {
		t.Fatalf("wildcard prefix: %v", err)
	}
	n, err := store.DeleteByExternalIDPrefixExcept("mldigest:2026-11-27:", []string{keepExt, keepExt, " "})
	if err != nil || n != 1 {
		t.Fatalf("deleted %d %v", n, err)
	}

	if _, _, _, _, err := store.GetItem("item_orphan"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("orphan still present: %v", err)
	}
	_, _, state, _, err := store.GetItem(keep.ID)
	if err != nil || state.InLater != true {
		t.Fatalf("kept state %+v %v", state, err)
	}
	if _, _, _, _, err := store.GetItem(other.ID); err != nil {
		t.Fatalf("other day gone: %v", err)
	}
	if _, _, _, _, err := store.GetItem(hn.ID); err != nil {
		t.Fatalf("hn gone: %v", err)
	}
	hits, err := store.Search(MatchQuery("old"), 10, 0)
	if err != nil || len(hits) != 0 {
		t.Fatalf("stale fts %+v %v", hits, err)
	}
	hits, err = store.Search(MatchQuery("FineDex"), 10, 0)
	if err != nil || len(hits) != 1 || hits[0].Item.ID != keep.ID {
		t.Fatalf("kept fts %+v %v", hits, err)
	}
}

func TestValidDigestReplacePrefix(t *testing.T) {
	ok := []string{"mldigest:2026-11-27:", "mldigest:2026-01-02:"}
	bad := []string{"", "mldigest:", "mldigest:2026-11-27", "mldigest:2026-11-27:papers:", "src_mldigest:", "mldigest:2026-13-01:", "MLDIGEST:2026-11-27:", "mldigest:2026-11-27:%"}
	for _, p := range ok {
		if !ValidDigestReplacePrefix(p) {
			t.Fatalf("rejected valid %q", p)
		}
	}
	for _, p := range bad {
		if ValidDigestReplacePrefix(p) {
			t.Fatalf("accepted invalid %q", p)
		}
	}
}
