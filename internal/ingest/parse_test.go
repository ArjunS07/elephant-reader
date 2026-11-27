package ingest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

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
	if len(items) != 1 || items[0].OpenMode != "reader" || items[0].ActionURL != "https://www.latent.space/p/contact" {
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
	got := Excerpt("", "", []byte(`{"kind":"paper","abstract":"`+text+`"}`), "", "")
	if !strings.Contains(got, "contact") || strings.Contains(got, "factors") {
		t.Fatalf("excerpt %q", got)
	}
	if Excerpt("kept", "", nil, "given excerpt", "") != "given excerpt" {
		t.Fatal("provided excerpt")
	}
	if Excerpt("The Episode.", "", nil, "", "The Episode") != "" {
		t.Fatal("excerpt duplicated the title")
	}
	cleaned := Excerpt("Cities and roads. See omnystudio.com/listener for privacy information. https://sponsor.example/deal?utm_source=podcast&utm_medium=rss", "", nil, "", "Cities")
	if strings.Contains(strings.ToLower(cleaned), "omnystudio") || strings.Contains(strings.ToLower(cleaned), "utm_") || strings.Contains(cleaned, "sponsor.example") {
		t.Fatalf("boilerplate excerpt %q", cleaned)
	}
	if !strings.Contains(cleaned, "Cities and roads") {
		t.Fatalf("cleaned %q", cleaned)
	}
}

func TestEnclosureFallbackAndAuthor(t *testing.T) {
	feed := []byte(`<?xml version="1.0"?>
<rss xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <item>
      <title>The one without a link</title>
      <guid isPermaLink="false">art19-episode-locator://abc</guid>
      <itunes:author>The Rest Is Politics</itunes:author>
      <enclosure url="https://traffic.megaphone.fm/EP1.mp3" length="12345" type="audio/mpeg"/>
      <description>Show notes about the week. See omnystudio.com/listener for privacy information.</description>
    </item>
    <item>
      <title>No guid either</title>
      <enclosure url="https://traffic.omny.fm/EP2.mp3" length="50" type="audio/mpeg"/>
    </item>
  </channel>
</rss>`)
	entries, err := ParseFeed(feed)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries %d", len(entries))
	}
	if entries[0].Link != "" || entries[0].ID != "art19-episode-locator://abc" || entries[0].Author != "The Rest Is Politics" {
		t.Fatalf("first %+v", entries[0])
	}
	if entries[0].EnclosureURL != "https://traffic.megaphone.fm/EP1.mp3" || entries[0].EnclosureType != "audio/mpeg" || entries[0].EnclosureLength != 12345 {
		t.Fatalf("enclosure %+v", entries[0])
	}
	source := model.Source{ID: "src_trip", Name: "The Rest Is Politics", SourceType: "podcast", DefaultOpenMode: "deeplink", Config: map[string]any{"overcastUrl": "https://overcast.fm/itunes99"}}
	items := EntriesToItems(source, entries)
	if len(items) != 2 {
		t.Fatalf("items %d", len(items))
	}
	if items[0].ExternalID != "art19-episode-locator://abc" || items[0].CanonicalURL != "https://traffic.megaphone.fm/EP1.mp3" || items[0].ActionURL != items[0].CanonicalURL {
		t.Fatalf("identity %+v", items[0])
	}
	if items[0].OpenMode != "reader" || items[0].Author != "The Rest Is Politics" {
		t.Fatalf("mode %+v", items[0])
	}
	var meta map[string]any
	if err := json.Unmarshal(items[0].Metadata, &meta); err != nil || meta["overcastUrl"] != "https://overcast.fm/itunes99" || meta["enclosureUrl"] != "https://traffic.megaphone.fm/EP1.mp3" {
		t.Fatalf("meta %+v", meta)
	}
	if items[1].ExternalID != "https://traffic.omny.fm/EP2.mp3" || items[1].Author != "The Rest Is Politics" {
		t.Fatalf("cdn author fallback %+v", items[1])
	}
	if strings.Contains(strings.ToLower(items[1].Author), "omny.fm") {
		t.Fatal(items[1].Author)
	}
}

func TestStreamLimitAndBackfill(t *testing.T) {
	now := time.Now().UTC()
	body := rssFeed(40, func(i int) time.Time {
		return now.Add(-time.Duration(i) * time.Hour)
	})
	tail := strings.Repeat("x", 300<<10)
	body = append(body[:len(body)-len("</channel></rss>")], []byte(`<item><title>TAIL</title><description>`+tail+`</description></item></channel></rss>`)...)
	counted := &countReader{r: bytes.NewReader(body)}
	entries, err := ParseFeedReader(counted, FeedLimit{MaxItems: 25})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 25 || entries[0].Title != "Ep 0" {
		t.Fatalf("capped %d %s", len(entries), titleOf(entries))
	}
	if counted.n >= len(body)/2 {
		t.Fatalf("read %d of %d bytes", counted.n, len(body))
	}

	old := rssFeed(30, func(i int) time.Time { return now.AddDate(-2, 0, -i) })
	lim := FeedLimit{MaxItems: 25, MinItems: 5, FirstSync: true, NotBefore: now.AddDate(0, 0, -60)}
	entries, err = ParseFeed(old)
	if err != nil || len(entries) != 30 {
		t.Fatalf("unlimited %d %v", len(entries), err)
	}
	entries, err = ParseFeedReader(bytes.NewReader(old), lim)
	if err != nil || len(entries) != 5 || entries[0].Title != "Ep 0" {
		t.Fatalf("old large %+v %v", titles(entries), err)
	}
	small := rssFeed(10, func(i int) time.Time { return now.AddDate(-3, 0, -i) })
	entries, err = ParseFeedReader(bytes.NewReader(small), lim)
	if err != nil || len(entries) != 10 {
		t.Fatalf("small old feed %d %v", len(entries), err)
	}
	mixed := rssFeed(38, func(i int) time.Time {
		if i < 8 {
			return now.AddDate(0, 0, -i)
		}
		return now.AddDate(-4, 0, -i)
	})
	entries, err = ParseFeedReader(bytes.NewReader(mixed), lim)
	if err != nil || len(entries) != 8 {
		t.Fatalf("window %+v %v", titles(entries), err)
	}

	oldestFirst := rssFeed(30, func(i int) time.Time { return now.AddDate(0, 0, -(29 - i)) })
	entries, err = ParseFeedReader(bytes.NewReader(oldestFirst), FeedLimit{MaxItems: 25})
	if err != nil || len(entries) != 25 || entries[0].Title != "Ep 29" {
		t.Fatalf("newest of oldest-first %+v %v", titles(entries), err)
	}

	cut := []byte(`<?xml version="1.0"?><rss><channel>
<item><title>One</title><link>https://example.com/1</link></item>
<item><title>Two</title><link>https://example.com/2</link></item>
<item><title>Thr`)
	entries, err = ParseFeed(cut)
	if err != nil || len(entries) != 2 {
		t.Fatalf("truncated %d %v", len(entries), err)
	}
}

func rssFeed(n int, when func(int) time.Time) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><rss version="2.0"><channel>`)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, `<item><title>Ep %d</title><link>https://example.com/%d</link><guid>ep-%d</guid><pubDate>%s</pubDate><description>notes %d</description></item>`,
			i, i, i, when(i).Format(time.RFC1123Z), i)
	}
	b.WriteString(`</channel></rss>`)
	return []byte(b.String())
}

func titles(entries []FeedEntry) []string {
	out := make([]string, len(entries))
	for i, entry := range entries {
		out[i] = entry.Title
	}
	return out
}

func titleOf(entries []FeedEntry) string {
	if len(entries) == 0 {
		return ""
	}
	return entries[0].Title
}

type countReader struct {
	r io.Reader
	n int
}

func (c *countReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += n
	return n, err
}

func jsonUnmarshal(raw []byte, dest any) error {
	return json.Unmarshal(raw, dest)
}
