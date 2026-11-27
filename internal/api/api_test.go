package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"elephant/internal/db"
	"elephant/internal/model"
)

func newServer(t *testing.T, token string) (*httptest.Server, *db.Store) {
	t.Helper()
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	store, err := db.Open(filepath.Join(t.TempDir(), "elephant.db"), loc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	return httptest.NewServer(New(store, token)), store
}

func do(t *testing.T, server *httptest.Server, method, path, token string, body any) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, server.URL+path, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func decode(t *testing.T, res *http.Response, dest any) {
	t.Helper()
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(dest); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationsIngestAndState(t *testing.T) {
	server, store := newServer(t, "")
	defer server.Close()

	res := do(t, server, http.MethodPost, "/v1/sources", "", map[string]any{
		"id": "src_papers", "name": "ML Digest Papers", "sourceType": "papers", "folder": "Papers",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("source %d", res.StatusCode)
	}
	res.Body.Close()
	res = do(t, server, http.MethodPost, "/v1/sources", "", map[string]any{
		"id": "src_pod", "name": "Latent Space", "sourceType": "podcast", "folder": "Podcasts",
	})
	res.Body.Close()

	body := map[string]any{
		"sourceId": "src_papers", "externalId": "2511.18421",
		"canonicalUrl": "https://arxiv.org/abs/2511.18421",
		"title":        "Tactile World Models",
		"author":       "Voss",
		"publishedAt":  "2026-11-26T07:30:00Z",
		"metadata": map[string]any{
			"kind": "paper", "authors": []string{"Voss"}, "affiliations": []string{"Stanford, Hazy Research"},
			"abstract": "Robots that only predict pixels fail at the moment of contact. We train a world model on synchronized vision.",
		},
		"initialState": map[string]any{"inLater": true},
	}
	res = do(t, server, http.MethodPost, "/v1/ingest", "", body)
	var ingested struct {
		Items []model.Item `json:"items"`
	}
	decode(t, res, &ingested)
	if res.StatusCode != http.StatusOK || len(ingested.Items) != 1 {
		t.Fatalf("ingest status %d items %d", res.StatusCode, len(ingested.Items))
	}
	item := ingested.Items[0]
	if item.PublishedDay != "2026-11-25" {
		t.Fatalf("published day %s", item.PublishedDay)
	}
	if item.Excerpt == "" || len(item.Excerpt) > 180 {
		t.Fatalf("excerpt %q", item.Excerpt)
	}
	_, _, state, _, err := store.GetItem(item.ID)
	if err != nil || !state.InLater || state.ReadAt != nil || state.ReadProgress != 0 {
		t.Fatalf("state %+v %v", state, err)
	}

	body["title"] = "Tactile World Models revised"
	delete(body, "initialState")
	res = do(t, server, http.MethodPost, "/v1/ingest", "", body)
	decode(t, res, &ingested)
	if ingested.Items[0].ID != item.ID || ingested.Items[0].Title != "Tactile World Models revised" {
		t.Fatalf("dedup external %+v", ingested.Items[0])
	}
	_, _, state, _, _ = store.GetItem(item.ID)
	if !state.InLater {
		t.Fatal("reingest clobbered later")
	}

	res = do(t, server, http.MethodPost, "/v1/ingest", "", map[string]any{
		"sourceId": "src_pod", "externalId": "other",
		"canonicalUrl": "https://arxiv.org/abs/2511.18421",
		"title":        "Same URL",
		"publishedAt":  "2026-11-26T18:00:00Z",
	})
	decode(t, res, &ingested)
	if ingested.Items[0].ID != item.ID {
		t.Fatal("canonical url was not deduped")
	}

	res = do(t, server, http.MethodPost, "/v1/ingest", "", map[string]any{
		"canonicalUrl": "https://www.inkandswitch.com/essay/local-first/",
		"title":        "Local-first software",
		"initialState": map[string]any{"readAt": "2026-11-25T18:12:00Z"},
	})
	decode(t, res, &ingested)
	if ingested.Items[0].SourceID == "" {
		t.Fatal("manual source missing")
	}
	_, _, state, _, _ = store.GetItem(ingested.Items[0].ID)
	if state.ReadAt == nil || *state.ReadAt == "" || state.InLater {
		t.Fatalf("extension read %+v", state)
	}

	res = do(t, server, http.MethodPatch, "/v1/items/"+item.ID+"/state", "", map[string]any{"readProgress": 0.35})
	var patched model.UserItemState
	decode(t, res, &patched)
	if patched.ReadProgress != 0.35 || !patched.InLater || patched.ReadAt != nil {
		t.Fatalf("progress %+v", patched)
	}
	res = do(t, server, http.MethodPatch, "/v1/items/"+item.ID+"/state", "", map[string]any{"inLater": false})
	decode(t, res, &patched)
	if patched.InLater || patched.SavedAt != nil || patched.ReadAt != nil || patched.ReadProgress != 0.35 {
		t.Fatalf("remove %+v", patched)
	}
	res = do(t, server, http.MethodPatch, "/v1/items/"+item.ID+"/state", "", map[string]any{"readAt": "2026-11-26T20:00:00Z"})
	decode(t, res, &patched)
	if patched.ReadAt == nil {
		t.Fatal("mark read")
	}
	res = do(t, server, http.MethodGet, "/v1/later", "", nil)
	var later struct {
		Items []model.ItemRow `json:"items"`
	}
	decode(t, res, &later)
	for _, row := range later.Items {
		if row.Item.ID == item.ID {
			t.Fatal("read item stayed in later")
		}
	}
	res = do(t, server, http.MethodGet, "/v1/read", "", nil)
	var read struct {
		Items []model.ItemRow `json:"items"`
	}
	decode(t, res, &read)
	if len(read.Items) < 2 {
		t.Fatalf("read %d", len(read.Items))
	}
}

func TestFeedCalendarAndSearch(t *testing.T) {
	server, _ := newServer(t, "secret")
	defer server.Close()
	health := do(t, server, http.MethodGet, "/v1/health", "", nil)
	if health.StatusCode != http.StatusOK {
		t.Fatalf("health %d", health.StatusCode)
	}
	health.Body.Close()
	denied := do(t, server, http.MethodGet, "/v1/feed", "", nil)
	if denied.StatusCode != http.StatusUnauthorized {
		t.Fatalf("auth %d", denied.StatusCode)
	}
	denied.Body.Close()

	auth := "secret"
	for _, source := range []map[string]any{
		{"id": "src_papers", "name": "Papers", "sourceType": "papers", "folder": "Papers"},
		{"id": "src_pod", "name": "Show", "sourceType": "podcast", "folder": "Podcasts"},
	} {
		res := do(t, server, http.MethodPost, "/v1/sources", auth, source)
		res.Body.Close()
	}
	papers := []map[string]any{
		{"sourceId": "src_papers", "externalId": "a", "canonicalUrl": "https://example.com/a", "title": "Friction cones", "publishedAt": "2026-11-26T18:00:00Z", "plainText": "A friction cone stays inside the planner.", "metadata": map[string]any{"kind": "paper", "affiliations": []string{"Stanford, Hazy Research"}, "discussionQuotes": []any{map[string]any{"speaker": "Lena Voss", "text": "The plan and the safety check cannot disagree."}}}},
		{"sourceId": "src_papers", "externalId": "b", "canonicalUrl": "https://example.com/b", "title": "Second paper", "publishedAt": "2026-11-26T20:00:00Z", "plainText": "Quiet follow up."},
		{"sourceId": "src_pod", "externalId": "c", "canonicalUrl": "https://example.com/c", "title": "Podcast hour", "publishedAt": "2026-11-24T18:00:00Z", "plainText": "A conversation about contact."},
	}
	res := do(t, server, http.MethodPost, "/v1/ingest", auth, map[string]any{"items": papers})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("batch %d", res.StatusCode)
	}
	var ingested struct {
		Items []model.Item `json:"items"`
	}
	decode(t, res, &ingested)

	res = do(t, server, http.MethodGet, "/v1/feed?folder=Papers", auth, nil)
	var feed model.Feed
	decode(t, res, &feed)
	if feed.Day != "2026-11-26" || len(feed.Items) != 2 || feed.Counts["2026-11-26"] != 2 || feed.Counts["2026-11-24"] != 0 {
		t.Fatalf("papers feed %+v", feed)
	}
	res = do(t, server, http.MethodGet, "/v1/feed?day=2026-11-24&folder=Papers", auth, nil)
	decode(t, res, &feed)
	if feed.Day != "2026-11-24" || len(feed.Items) != 0 || feed.NextDay == nil || *feed.NextDay != "2026-11-26" {
		t.Fatalf("empty paper day %+v", feed)
	}
	res = do(t, server, http.MethodGet, "/v1/calendar?folder=Podcasts", auth, nil)
	var calendar struct {
		Counts map[string]int `json:"counts"`
	}
	decode(t, res, &calendar)
	if calendar.Counts["2026-11-24"] != 1 || calendar.Counts["2026-11-26"] != 0 {
		t.Fatalf("podcast calendar %+v", calendar.Counts)
	}
	res = do(t, server, http.MethodGet, "/v1/calendar", auth, nil)
	decode(t, res, &calendar)
	if calendar.Counts["2026-11-26"] != 2 || calendar.Counts["2026-11-24"] != 1 {
		t.Fatalf("all calendar %+v", calendar.Counts)
	}

	paper := ingested.Items[0]
	res = do(t, server, http.MethodPatch, "/v1/items/"+paper.ID+"/state", auth, map[string]any{"readAt": "2026-11-27T01:00:00Z"})
	res.Body.Close()
	res = do(t, server, http.MethodPost, "/v1/items/"+paper.ID+"/highlights", auth, map[string]any{
		"id": "hl1", "quote": "friction cone", "prefix": "A ", "suffix": " stays", "note": "pencil mark in the margin",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("highlight %d", res.StatusCode)
	}
	res.Body.Close()

	res = do(t, server, http.MethodGet, "/v1/read?q=Hazy", auth, nil)
	var read struct {
		Items []model.ItemRow `json:"items"`
	}
	decode(t, res, &read)
	if len(read.Items) != 1 || read.Items[0].Item.ID != paper.ID {
		t.Fatalf("affiliation search %+v", read.Items)
	}
	res = do(t, server, http.MethodGet, "/v1/read?q=disagree", auth, nil)
	decode(t, res, &read)
	if len(read.Items) != 1 {
		t.Fatalf("quote search %d", len(read.Items))
	}
	res = do(t, server, http.MethodGet, "/v1/read?q=pencil", auth, nil)
	decode(t, res, &read)
	if len(read.Items) != 1 || read.Items[0].HighlightCount != 1 {
		t.Fatalf("note search %+v", read.Items)
	}
	res = do(t, server, http.MethodPatch, "/v1/highlights/hl1", auth, map[string]any{"note": "replaced margin note"})
	res.Body.Close()
	res = do(t, server, http.MethodGet, "/v1/read?q=pencil", auth, nil)
	decode(t, res, &read)
	if len(read.Items) != 0 {
		t.Fatal("stale highlight still matched")
	}
	res = do(t, server, http.MethodGet, "/v1/read?q=replaced", auth, nil)
	decode(t, res, &read)
	if len(read.Items) != 1 {
		t.Fatal("updated note missing")
	}
	res = do(t, server, http.MethodDelete, "/v1/highlights/hl1", auth, nil)
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete %d", res.StatusCode)
	}
	res.Body.Close()
	res = do(t, server, http.MethodGet, "/v1/read?q=replaced", auth, nil)
	decode(t, res, &read)
	if len(read.Items) != 0 {
		t.Fatal("deleted highlight still matched")
	}
	res = do(t, server, http.MethodGet, "/v1/items/"+paper.ID, auth, nil)
	var detail struct {
		Highlights []model.Highlight `json:"highlights"`
	}
	decode(t, res, &detail)
	if len(detail.Highlights) != 0 {
		t.Fatal("highlight remained")
	}
}

func TestPreferencesAndInbound(t *testing.T) {
	server, _ := newServer(t, "")
	defer server.Close()
	res := do(t, server, http.MethodGet, "/v1/preferences", "", nil)
	var prefs model.Preferences
	decode(t, res, &prefs)
	if prefs.Theme != "warm" || prefs.Align != "left" || prefs.ColumnWidth != "standard" {
		t.Fatalf("defaults %+v", prefs)
	}
	res = do(t, server, http.MethodPatch, "/v1/preferences", "", map[string]any{"theme": "dark", "columnWidth": "cozy", "align": "center"})
	decode(t, res, &prefs)
	if prefs.Theme != "dark" || prefs.ColumnWidth != "cozy" || prefs.Align != "center" {
		t.Fatalf("patched %+v", prefs)
	}
	res = do(t, server, http.MethodPost, "/v1/sources", "", map[string]any{
		"name": "Import AI", "sourceType": "newsletter", "folder": "Newsletters",
	})
	var source model.Source
	decode(t, res, &source)
	if source.EmailAlias == "" {
		t.Fatal("alias missing")
	}
	res = do(t, server, http.MethodPost, "/v1/inbound/email", "", map[string]any{
		"to":         "Import <" + source.EmailAlias + ">",
		"from":       "editor@importai.example",
		"subject":    "Tactile models",
		"html":       `<article><p>Touch carries the state. <a href="https://example.com/p">Paper</a></p></article>`,
		"receivedAt": "2026-11-25T14:00:00Z",
	})
	var inbound struct {
		Item    model.Item `json:"item"`
		Created bool       `json:"created"`
	}
	decode(t, res, &inbound)
	if !inbound.Created || inbound.Item.SourceID != source.ID || !bytes.Contains([]byte(inbound.Item.ContentMarkdown), []byte("Paper")) {
		t.Fatalf("inbound %+v", inbound.Item)
	}
	res = do(t, server, http.MethodPost, "/v1/inbound/email", "", map[string]any{
		"to": source.EmailAlias, "from": "editor@importai.example", "subject": "Tactile models",
		"text": "Touch carries the state.", "receivedAt": "2026-11-25T14:00:00Z",
	})
	decode(t, res, &inbound)
	if inbound.Created {
		t.Fatal("email was not deduped")
	}
	res = do(t, server, http.MethodPost, "/v1/inbound/email", "", map[string]any{"to": "missing@example.com", "subject": "Nope"})
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown alias %d", res.StatusCode)
	}
	res.Body.Close()
}

func TestSyncRSS(t *testing.T) {
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><item>
			<title>Latency budget</title><link>https://harbor.example/latency</link><guid>lat-1</guid>
			<pubDate>Tue, 24 Nov 2026 10:00:00 +0000</pubDate>
			<description>The gap between intending to read and seeing the sentence.</description>
		</item></channel></rss>`))
	}))
	defer feed.Close()
	server, store := newServer(t, "")
	defer server.Close()
	res := do(t, server, http.MethodPost, "/v1/sources", "", map[string]any{
		"id": "src_harbor", "name": "Harbor", "sourceType": "rss", "folder": "Blogs", "feedUrl": feed.URL,
	})
	res.Body.Close()
	res = do(t, server, http.MethodPost, "/v1/sources/src_harbor/sync", "", nil)
	var synced struct {
		Ingested int      `json:"ingested"`
		Errors   []string `json:"errors"`
	}
	decode(t, res, &synced)
	if res.StatusCode != http.StatusOK || synced.Ingested != 1 || len(synced.Errors) != 0 {
		t.Fatalf("sync %+v status %d", synced, res.StatusCode)
	}
	rows, err := store.DayItems("2026-11-24", "RSS", "")
	if err != nil || len(rows) != 1 || rows[0].Item.Title != "Latency budget" {
		t.Fatalf("synced %+v %v", rows, err)
	}
}
