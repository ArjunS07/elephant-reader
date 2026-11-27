package ingest

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"elephant/internal/db"
	"elephant/internal/model"
)

func TestParseRSSAndAtom(t *testing.T) {
	rss := []byte(`<?xml version="1.0"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>Show</title>
    <item>
      <title>Contact models</title>
      <dc:creator>Latent Space</dc:creator>
      <link>https://www.latent.space/p/contact</link>
      <guid>ls-1</guid>
      <pubDate>Wed, 25 Nov 2026 16:00:00 +0000</pubDate>
      <itunes:duration>48:12</itunes:duration>
      <content:encoded><![CDATA[<p>Touch carries the state vision lost. <a href="https://example.com/paper">Paper</a></p>]]></content:encoded>
    </item>
  </channel>
</rss>`)
	entries, err := ParseFeed(rss)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries %d", len(entries))
	}
	if entries[0].Title != "Contact models" || entries[0].Author != "Latent Space" || entries[0].ID != "ls-1" {
		t.Fatalf("entry %+v", entries[0])
	}
	if entries[0].DurationMin != 48 {
		t.Fatalf("duration %d", entries[0].DurationMin)
	}
	if entries[0].Published.UTC().Format(time.RFC3339) != "2026-11-25T16:00:00Z" {
		t.Fatalf("published %s", entries[0].Published)
	}
	items := EntriesToItems(model.Source{ID: "src_pod", Name: "Latent Space", SourceType: "podcast", DefaultOpenMode: "deeplink"}, entries)
	if len(items) != 1 || items[0].OpenMode != "deeplink" {
		t.Fatalf("items %+v", items)
	}
	var meta struct {
		Kind     string `json:"kind"`
		ShowName string `json:"showName"`
		Duration int    `json:"durationMin"`
	}
	if err := jsonUnmarshal(items[0].Metadata, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Kind != "podcast" || meta.ShowName != "Latent Space" || meta.Duration != 48 {
		t.Fatalf("meta %+v", meta)
	}
	if !strings.Contains(items[0].ContentMarkdown, "[Paper](https://example.com/paper)") {
		t.Fatalf("markdown %q", items[0].ContentMarkdown)
	}

	atom := []byte(`<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns:media="http://search.yahoo.com/mrss/">
  <entry>
    <id>yt:video:abc</id>
    <title>Let's build GPT</title>
    <author><name>Andrej Karpathy</name></author>
    <published>2026-11-26T15:00:00Z</published>
    <link rel="alternate" href="https://www.youtube.com/watch?v=abc"/>
    <media:group>
      <yt:duration seconds="6960"/>
      <media:description>A full walkthrough.</media:description>
    </media:group>
  </entry>
</feed>`)
	entries, err = ParseFeed(atom)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Author != "Andrej Karpathy" || entries[0].Link != "https://www.youtube.com/watch?v=abc" {
		t.Fatalf("atom %+v", entries)
	}
	if entries[0].DurationMin != 116 {
		t.Fatalf("yt duration %d", entries[0].DurationMin)
	}
	videos := EntriesToItems(model.Source{ID: "src_yt", Name: "Andrej Karpathy", SourceType: "youtube", DefaultOpenMode: "external"}, entries)
	if videos[0].ExternalID != "yt:video:abc" || !strings.Contains(videos[0].PlainText, "walkthrough") {
		t.Fatalf("video %+v", videos[0])
	}
}

func TestParseHN(t *testing.T) {
	body := []byte(`{"hits":[
		{"objectID":"1","title":"SQLite WAL","url":"https://example.com/wal","author":"rrau","points":120,"num_comments":10,"created_at":"2026-11-26T16:10:00Z"},
		{"objectID":"2","title":"Low score","url":"https://example.com/low","author":"a","points":3,"num_comments":1,"created_at":"2026-11-26T12:00:00Z"}
	]}`)
	items, err := ParseHN(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("hits %d", len(items))
	}
	if items[0].DiscussionURL != "https://news.ycombinator.com/item?id=1" || items[0].CanonicalURL != "https://example.com/wal" {
		t.Fatalf("%+v", items[0])
	}
	kept := 0
	for _, item := range items {
		var meta struct {
			Points int `json:"points"`
		}
		if err := jsonUnmarshal(item.Metadata, &meta); err != nil {
			t.Fatal(err)
		}
		if meta.Points >= 50 {
			kept++
		}
	}
	if kept != 1 {
		t.Fatalf("kept %d", kept)
	}
}

func TestEmailsAndAlias(t *testing.T) {
	got := Emails(`Digest <NL-Import-AI@elephant.local>, other@example.com`)
	if len(got) != 2 || got[0] != "nl-import-ai@elephant.local" {
		t.Fatalf("%v", got)
	}
	alias := NewsletterAlias("The Batch", func(string) bool { return false })
	if alias != "nl-the-batch@elephant.local" {
		t.Fatalf("%s", alias)
	}
	alias = NewsletterAlias("The Batch", func(v string) bool { return v == "nl-the-batch@elephant.local" })
	if alias != "nl-the-batch-2@elephant.local" {
		t.Fatalf("%s", alias)
	}
}

func TestExcerpt(t *testing.T) {
	text := "Robots that only predict pixels fail at the moment of contact. We train a world model on synchronized vision. The model factors the latent state."
	got := Excerpt("", "", []byte(`{"kind":"paper","abstract":"`+text+`"}`), "")
	if !strings.Contains(got, "contact") || strings.Contains(got, "factors") {
		t.Fatalf("excerpt %q", got)
	}
	if Excerpt("kept", "", nil, "given excerpt") != "given excerpt" {
		t.Fatal("provided excerpt")
	}
}

func jsonUnmarshal(raw []byte, dest any) error {
	return json.Unmarshal(raw, dest)
}

func TestParseTimeRSSVariants(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Fri, 4 Mar 2022 15:00:00 +0000", "2022-03-04T15:00:00Z"},
		{"Fri, 04 Mar 2022 15:00:00 +0000", "2022-03-04T15:00:00Z"},
		{"Mon, 2 Jan 2006 15:04:05 GMT", "2006-01-02T15:04:05Z"},
		{"Mon, 2 Jan 2006 15:04:05 gmt", "2006-01-02T15:04:05Z"},
		{"Mon, 2 Jan 2006 15:04:05 UT", "2006-01-02T15:04:05Z"},
		{"4 Mar 2022 15:00:00 -0800", "2022-03-04T23:00:00Z"},
		{"Fri, 4 Mar 2022 15:00:00 +0000 (UTC)", "2022-03-04T15:00:00Z"},
		{"Fri, 4 Mar 2022 15:00:00 +0000 (GMT)", "2022-03-04T15:00:00Z"},
		{"Fri,  4 Mar 2022   15:00:00  +0000", "2022-03-04T15:00:00Z"},
		{"fri, 4 mar 2022 15:00:00 +0000", "2022-03-04T15:00:00Z"},
		{"Mon, 2 Jan 2006 15:04 -0700", "2006-01-02T22:04:00Z"},
		{"Mon, 02 Jan 2006 15:04 -0700", "2006-01-02T22:04:00Z"},
		{"2 Jan 2006 15:04:05 -0700", "2006-01-02T22:04:05Z"},
		{"2 Jan 2006 15:04:05 MST", "2006-01-02T22:04:05Z"},
		{"02 Jan 2006 15:04:05 -0700", "2006-01-02T22:04:05Z"},
		{"02 Jan 2006 15:04:05 MST", "2006-01-02T22:04:05Z"},
		{"Mon, 2 Jan 2006 15:04:05 MST", "2006-01-02T22:04:05Z"},
		{"02 Jan 06 15:04 -0700", "2006-01-02T22:04:00Z"},
		{"02 Jan 06 15:04 MST", "2006-01-02T22:04:00Z"},
		{"Mon, 2 Jan 06 15:04:05 -0700", "2006-01-02T22:04:05Z"},
		{"Mon, 2 Jan 06 15:04:05 MST", "2006-01-02T22:04:05Z"},
		{"Wed, 25 Nov 2026 16:00:00 +0000", "2026-11-25T16:00:00Z"},
		{"2026-11-26T15:00:00Z", "2026-11-26T15:00:00Z"},
		{"2026-11-26t15:00:00z", "2026-11-26T15:00:00Z"},
		{"2026-11-26", "2026-11-26T00:00:00Z"},
		{"2026-11-26 16:10:00", "2026-11-26T16:10:00Z"},
		{"Thu, 3 Mar 2022 08:00:00 EST", "2022-03-03T13:00:00Z"},
		{"Fri, 4 Mar 2022 01:00:00 PST", "2022-03-04T09:00:00Z"},
	}
	for _, tc := range cases {
		got, err := ParseTime(tc.in)
		if err != nil {
			t.Errorf("ParseTime(%q): %v", tc.in, err)
			continue
		}
		if got.UTC().Format(time.RFC3339) != tc.want {
			t.Errorf("ParseTime(%q) = %s, want %s", tc.in, got.UTC().Format(time.RFC3339), tc.want)
		}
	}
	if _, err := ParseTime("not a date"); err == nil {
		t.Fatal("expected garbage to fail")
	}
}

func TestSingleDigitPubDateNotStampedToday(t *testing.T) {
	rss := []byte(`<?xml version="1.0"?>
<rss version="2.0"><channel><item>
  <title>Libsyn episode</title>
  <link>https://example.com/ep</link>
  <guid>ep-2022</guid>
  <pubDate>Fri, 4 Mar 2022 15:00:00 +0000 (UTC)</pubDate>
</item></channel></rss>`)
	entries, err := ParseFeed(rss)
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].Published.UTC().Format(time.RFC3339) != "2022-03-04T15:00:00Z" {
		t.Fatalf("published %s", entries[0].Published)
	}
	items := EntriesToItems(model.Source{ID: "src_pod", Name: "Show", SourceType: "podcast", DefaultOpenMode: "deeplink"}, entries)
	if len(items) != 1 || items[0].PublishedAt != "2022-03-04T15:00:00Z" {
		t.Fatalf("stamped %+v", items)
	}
	today := time.Now().UTC().Format("2006-01-02")
	if strings.HasPrefix(items[0].PublishedAt, today) {
		t.Fatalf("fell back to today: %s", items[0].PublishedAt)
	}
}

func TestYouTubeShortsExcluded(t *testing.T) {
	source := model.Source{ID: "src_yt", Name: "Channel", SourceType: "youtube", DefaultOpenMode: "external"}
	entries := []FeedEntry{
		{Title: "A short", Link: "https://www.youtube.com/shorts/aaa", ID: "a"},
		{Title: "Mobile", Link: "https://m.youtube.com/shorts/bbb", ID: "b"},
		{Title: "Cased", Link: "https://www.YouTube.com/SHORTS/ccc", ID: "c"},
		{Title: "Demo #Shorts", Link: "https://www.youtube.com/watch?v=ddd", ID: "d", DurationMin: 1},
		{Title: "Tagged #short", Link: "https://www.youtube.com/watch?v=eee", ID: "e"},
		{Title: "Hash in word #shortest", Link: "https://www.youtube.com/watch?v=hash", ID: "hash"},
		{Title: "One minute robotics demo", Link: "https://www.youtube.com/watch?v=fff", ID: "f", DurationMin: 1},
		{Title: "Full lecture", Link: "https://www.youtube.com/watch?v=ggg", ID: "g", DurationMin: 48},
	}
	items := EntriesToItems(source, entries)
	if len(items) != 3 {
		t.Fatalf("kept %d: %+v", len(items), titles(items))
	}
	got := map[string]bool{}
	for _, item := range items {
		got[item.ExternalID] = true
		if strings.Contains(strings.ToLower(item.CanonicalURL), "/shorts/") {
			t.Fatalf("short slipped through: %s", item.CanonicalURL)
		}
	}
	for _, id := range []string{"hash", "f", "g"} {
		if !got[id] {
			t.Fatalf("missing %s in %+v", id, items)
		}
	}
	if items[1].Title != "One minute robotics demo" {
		t.Fatalf("duration filter dropped a watch video: %+v", items)
	}

	podcast := EntriesToItems(model.Source{ID: "src_pod", Name: "Show", SourceType: "podcast", DefaultOpenMode: "deeplink"}, []FeedEntry{
		{Title: "Episode mentions #shorts", Link: "https://example.com/pod", ID: "pod"},
	})
	if len(podcast) != 1 {
		t.Fatal("podcast title hashtag should be kept")
	}

	store := openParseDB(t)
	if _, err := store.InsertSource(model.Source{
		ID: "src_yt", Name: "Channel", SourceType: "youtube", Folder: "Video",
		DefaultOpenMode: "external", DefaultQueue: "feed", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	svc := New(store)
	results, err := svc.Ingest(context.Background(), []model.IngestItem{
		{SourceID: "src_yt", ExternalID: "a", CanonicalURL: "https://www.youtube.com/shorts/aaa", Title: "A short"},
		{SourceID: "src_yt", ExternalID: "b", CanonicalURL: "https://m.youtube.com/SHORTS/bbb", Title: "Mobile"},
		{SourceID: "src_yt", ExternalID: "c", CanonicalURL: "https://www.youtube.com/watch?v=ccc", Title: "Real talk #shorts"},
		{SourceID: "src_yt", ExternalID: "d", CanonicalURL: "https://www.youtube.com/watch?v=ddd", ActionURL: "https://www.youtube.com/shorts/ddd", Title: "Action is a short"},
		{SourceID: "src_yt", ExternalID: "f", CanonicalURL: "https://www.youtube.com/watch?v=fff", Title: "One minute robotics demo"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Item.ExternalID != "f" || !results[0].Created {
		t.Fatalf("ingest kept %+v", results)
	}
	var n int
	if err := store.SQL.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("rows %d", n)
	}
}

func TestResyncUpdatesPublishedDay(t *testing.T) {
	store := openParseDB(t)
	if _, err := store.InsertSource(model.Source{
		ID: "src_pod", Name: "Latent Space", SourceType: "podcast", Folder: "Podcasts",
		DefaultOpenMode: "deeplink", DefaultQueue: "feed", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	svc := New(store)
	later := true
	first, err := svc.Ingest(context.Background(), []model.IngestItem{{
		SourceID:     "src_pod",
		ExternalID:   "ep-2022",
		CanonicalURL: "https://example.com/ep",
		Title:        "Libsyn episode",
		PublishedAt:  time.Now().UTC().Format(time.RFC3339),
		PlainText:    "original notes",
		InitialState: &model.InitialState{InLater: &later},
	}})
	if err != nil || len(first) != 1 || !first[0].Created {
		t.Fatalf("first %+v %v", first, err)
	}
	if first[0].Item.PublishedDay == "2022-03-04" {
		t.Fatal("setup date already matches the feed")
	}
	parsed, err := ParseTime("Fri, 4 Mar 2022 15:00:00 +0000 (UTC)")
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Ingest(context.Background(), []model.IngestItem{{
		SourceID:     "src_pod",
		ExternalID:   "ep-2022",
		CanonicalURL: "https://example.com/ep",
		Title:        "Libsyn episode retitled",
		PublishedAt:  parsed.UTC().Format(time.RFC3339),
		PlainText:    "corrected notes",
	}})
	if err != nil || len(second) != 1 || second[0].Created || second[0].Item.ID != first[0].Item.ID {
		t.Fatalf("resync %+v %v", second, err)
	}
	if second[0].Item.PublishedDay != "2022-03-04" {
		t.Fatalf("published day %s at %s", second[0].Item.PublishedDay, second[0].Item.PublishedAt)
	}
	if !strings.HasPrefix(second[0].Item.PublishedAt, "2022-03-04T15:00:00") {
		t.Fatalf("published at %s", second[0].Item.PublishedAt)
	}
	var at, day, title string
	if err := store.SQL.QueryRow(`SELECT published_at, published_day, title FROM items WHERE id = ?`, second[0].Item.ID).Scan(&at, &day, &title); err != nil {
		t.Fatal(err)
	}
	if day != "2022-03-04" || !strings.HasPrefix(at, "2022-03-04T15:00:00") || title != "Libsyn episode retitled" {
		t.Fatalf("row at=%s day=%s title=%s", at, day, title)
	}
	var ftsTitle, ftsPlain string
	if err := store.SQL.QueryRow(`SELECT title, plain_text FROM items_fts WHERE item_id = ?`, second[0].Item.ID).Scan(&ftsTitle, &ftsPlain); err != nil {
		t.Fatal(err)
	}
	if ftsTitle != "Libsyn episode retitled" || !strings.Contains(ftsPlain, "corrected notes") {
		t.Fatalf("fts title=%q plain=%q", ftsTitle, ftsPlain)
	}
	_, _, state, _, err := store.GetItem(second[0].Item.ID)
	if err != nil || !state.InLater {
		t.Fatalf("state %+v %v", state, err)
	}
}

func titles(items []model.IngestItem) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.Title
	}
	return out
}

func openParseDB(t *testing.T) *db.Store {
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
	return store
}
