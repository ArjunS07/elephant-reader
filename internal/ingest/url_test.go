package ingest

import (
	"context"
	"testing"

	"elephant/internal/model"
)

func TestSanitizeURL(t *testing.T) {
	cases := map[string]string{
		"https://example.com/a":     "https://example.com/a",
		"http://example.com":        "http://example.com",
		"  https://example.com/x  ": "https://example.com/x",
		"HTTPS://example.com/y":     "HTTPS://example.com/y",
		"overcast://x-callback-url/add?url=https%3A%2F%2Fexample.com": "overcast://x-callback-url/add?url=https%3A%2F%2Fexample.com",
		"javascript:alert(1)":                      "",
		"JaVaScRiPt:alert(1)":                      "",
		"data:text/html,hi":                        "",
		"vbscript:msgbox":                          "",
		"file:///etc/passwd":                       "",
		"http://example.com/\njavascript:alert(1)": "",
		"":            "",
		"example.com": "",
	}
	for raw, want := range cases {
		if got := SanitizeURL(raw); got != want {
			t.Fatalf("SanitizeURL(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestIngestStripsUnsafeURLs(t *testing.T) {
	store := openImportDB(t)
	svc := New(store)
	if _, err := svc.Ingest(context.Background(), []model.IngestItem{{
		CanonicalURL: "javascript:alert(1)",
		Title:        "Nope",
	}}); err == nil {
		t.Fatal("javascript canonical url was stored")
	}
	results, err := svc.Ingest(context.Background(), []model.IngestItem{{
		CanonicalURL:  "https://example.com/a",
		DiscussionURL: "javascript:alert(1)",
		ActionURL:     "data:text/html,hi",
		Title:         "Kept",
	}})
	if err != nil || len(results) != 1 {
		t.Fatalf("ingest %+v %v", results, err)
	}
	item := results[0].Item
	if item.CanonicalURL != "https://example.com/a" || item.DiscussionURL != "" || item.ActionURL != "https://example.com/a" {
		t.Fatalf("urls canonical %s discussion %s action %s", item.CanonicalURL, item.DiscussionURL, item.ActionURL)
	}
	results, err = svc.Ingest(context.Background(), []model.IngestItem{{
		CanonicalURL: "https://example.com/ep",
		ActionURL:    "overcast://x-callback-url/add?url=https%3A%2F%2Fexample.com%2Ffeed",
		Title:        "Episode",
	}})
	if err != nil || results[0].Item.ActionURL != "overcast://x-callback-url/add?url=https%3A%2F%2Fexample.com%2Ffeed" {
		t.Fatalf("overcast %+v %v", results, err)
	}
}
