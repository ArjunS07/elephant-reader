package ingest

import (
	"encoding/json"
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
