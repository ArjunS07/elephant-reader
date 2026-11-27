package ingest

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"elephant/internal/db"
)

func TestYouTubeCSV(t *testing.T) {
	csv := "Channel ID,Channel URL,Channel title\nUCabc,https://www.youtube.com/channel/UCabc,Example Channel\nUCdef,https://www.youtube.com/channel/UCdef,Second\n"
	drafts, err := ParseYouTubeCSV([]byte(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(drafts) != 2 || drafts[0].FeedURL != "https://www.youtube.com/feeds/videos.xml?channel_id=UCabc" {
		t.Fatalf("%+v", drafts)
	}
	if drafts[0].SourceType != "youtube" || drafts[0].Folder != "Video" || drafts[0].DefaultOpenMode != "external" {
		t.Fatalf("%+v", drafts[0])
	}
}

func TestOPMLAndOvercastAction(t *testing.T) {
	standard := []byte(`<?xml version="1.0"?><opml version="1.0"><head><title>Feeds</title></head><body>
		<outline text="Blogs"><outline text="Harbor" xmlUrl="https://harbor.example/feed.xml"/></outline>
		<outline text="Loose" xmlUrl="https://loose.example/rss"/>
	</body></opml>`)
	drafts, err := ParseOPML(standard)
	if err != nil {
		t.Fatal(err)
	}
	if len(drafts) != 2 || drafts[0].Folder != "Blogs" || drafts[0].SourceType != "rss" || drafts[0].DefaultOpenMode != "reader" {
		t.Fatalf("standard %+v", drafts)
	}
	if drafts[1].Folder != "RSS" {
		t.Fatalf("loose folder %s", drafts[1].Folder)
	}

	overcast := []byte(`<?xml version="1.0"?><opml version="1.0"><head><title>Overcast Podcast Subscriptions</title></head><body>
		<outline text="Latent Space" type="rss" xmlUrl="https://www.latent.space/feed" overcastId="1844"/>
	</body></opml>`)
	store := openImportDB(t)
	result, err := ImportBytes(context.Background(), store, overcast, ImportOptions{
		Filename: "subs.opml",
		Lookup: func(context.Context, string, string) (string, error) {
			return "4242", nil
		},
	})
	if err != nil || result.Added != 1 {
		t.Fatalf("%+v %v", result, err)
	}
	sources, err := store.ListSources()
	if err != nil || len(sources) != 1 {
		t.Fatal(err)
	}
	source := sources[0]
	if source.SourceType != "podcast" || source.Config["overcastId"] != "1844" || source.Config["overcastUrl"] != "https://overcast.fm/itunes4242" {
		t.Fatalf("%+v", source)
	}
	items := EntriesToItems(source, []FeedEntry{{Title: "Episode", Link: "https://www.latent.space/p/ep", ID: "ep"}})
	if items[0].ActionURL != "https://overcast.fm/itunes4242" || items[0].OpenMode != "deeplink" {
		t.Fatalf("%+v", items[0])
	}

	fallback := source
	fallback.Config = map[string]any{"overcastId": "1844"}
	items = EntriesToItems(fallback, []FeedEntry{{Title: "Episode", Link: "https://www.latent.space/p/ep", ID: "ep"}})
	if !strings.HasPrefix(items[0].ActionURL, "overcast://x-callback-url/add?url=") || !strings.Contains(items[0].ActionURL, "latent.space") {
		t.Fatalf("fallback %s", items[0].ActionURL)
	}
}

func TestImportDedup(t *testing.T) {
	store := openImportDB(t)
	body := []byte("Channel ID,Channel URL,Channel title\nUCabc,https://www.youtube.com/channel/UCabc,Example\nUCabc,https://www.youtube.com/channel/UCabc,Example again\n")
	first, err := ImportBytes(context.Background(), store, body, ImportOptions{Filename: "takeout.csv"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Added != 1 || first.Skipped != 1 || first.Total != 2 {
		t.Fatalf("first %+v", first)
	}
	second, err := ImportBytes(context.Background(), store, body, ImportOptions{Filename: "takeout.csv"})
	if err != nil {
		t.Fatal(err)
	}
	if second.Added != 0 || second.Skipped != 2 {
		t.Fatalf("second %+v", second)
	}
	sources, err := store.ListSources()
	if err != nil || len(sources) != 1 || sources[0].Name != "Example" {
		t.Fatalf("%+v %v", sources, err)
	}
}

func openImportDB(t *testing.T) *db.Store {
	t.Helper()
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	store, err := db.Open(filepath.Join(t.TempDir(), "import.db"), loc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}
