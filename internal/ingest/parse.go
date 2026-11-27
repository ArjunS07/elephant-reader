package ingest

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"elephant/internal/model"
)

type FeedEntry struct {
	ID          string
	Title       string
	Author      string
	Link        string
	Published   time.Time
	HTML        string
	DurationMin int
}

func ParseFeed(data []byte) ([]FeedEntry, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }
	var stack []string
	var in bool
	var depth int
	var cur FeedEntry
	var contentRank int
	var items []FeedEntry
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch el := tok.(type) {
		case xml.StartElement:
			name := strings.ToLower(el.Name.Local)
			if (name == "item" || name == "entry") && !in {
				in = true
				depth = 1
				cur = FeedEntry{}
				contentRank = 0
				stack = append(stack, name)
				continue
			}
			if in {
				depth++
				stack = append(stack, name)
				if name == "link" {
					href := attr(el, "href")
					rel := strings.ToLower(attr(el, "rel"))
					if href != "" && (cur.Link == "" || rel == "alternate") && rel != "enclosure" {
						cur.Link = href
					}
				}
				if name == "duration" {
					if seconds := attr(el, "seconds"); seconds != "" {
						if n, err := strconv.Atoi(seconds); err == nil {
							cur.DurationMin = minutes(n)
						}
					}
				}
			}
		case xml.EndElement:
			name := strings.ToLower(el.Name.Local)
			if in {
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
				depth--
				if depth == 0 && (name == "item" || name == "entry") {
					in = false
					if strings.TrimSpace(cur.Title) != "" || cur.Link != "" {
						items = append(items, cur)
					}
				}
			}
		case xml.CharData:
			if !in || len(stack) == 0 {
				continue
			}
			text := strings.TrimSpace(string(el))
			if text == "" {
				continue
			}
			field := stack[len(stack)-1]
			switch field {
			case "title":
				if cur.Title == "" && !contains(stack, "media") {
					cur.Title = text
				}
			case "creator":
				if cur.Author == "" {
					cur.Author = text
				}
			case "name":
				if cur.Author == "" && contains(stack, "author") {
					cur.Author = text
				}
			case "pubdate", "published", "updated", "date":
				if cur.Published.IsZero() {
					if parsed, err := ParseTime(text); err == nil {
						cur.Published = parsed
					}
				}
			case "guid", "id":
				if cur.ID == "" && !contains(stack, "author") {
					cur.ID = text
					if cur.Link == "" && strings.Contains(text, "://") {
						cur.Link = text
					}
				}
			case "encoded", "content", "description", "summary":
				rank := map[string]int{"encoded": 4, "content": 3, "description": 2, "summary": 1}[field]
				if rank > contentRank {
					cur.HTML = text
					contentRank = rank
				}
			case "duration":
				if cur.DurationMin == 0 {
					cur.DurationMin = parseDuration(text)
				}
			case "link":
				if cur.Link == "" && strings.Contains(text, "://") {
					cur.Link = text
				}
			}
		}
	}
	if items == nil {
		items = []FeedEntry{}
	}
	return items, nil
}

func EntriesToItems(source model.Source, entries []FeedEntry) []model.IngestItem {
	var out []model.IngestItem
	for _, entry := range entries {
		link := strings.TrimSpace(entry.Link)
		if link == "" {
			continue
		}
		external := strings.TrimSpace(entry.ID)
		if external == "" {
			external = link
		}
		markdown, plain := HTMLToText(entry.HTML)
		if plain == "" {
			plain = strings.TrimSpace(entry.Title)
		}
		published := entry.Published
		if published.IsZero() {
			published = time.Now().UTC()
		}
		meta := map[string]any{"kind": "generic"}
		switch source.SourceType {
		case "podcast":
			meta = map[string]any{"kind": "podcast", "showName": source.Name, "durationMin": entry.DurationMin}
		case "youtube":
			channel := entry.Author
			if channel == "" {
				channel = source.Name
			}
			meta = map[string]any{"kind": "youtube", "channelName": channel, "durationMin": entry.DurationMin}
		}
		raw, _ := json.Marshal(meta)
		mode := source.DefaultOpenMode
		if !model.ValidOpenMode(mode) {
			mode = "reader"
		}
		out = append(out, model.IngestItem{
			SourceID:        source.ID,
			ExternalID:      external,
			CanonicalURL:    link,
			Title:           strings.TrimSpace(entry.Title),
			Author:          strings.TrimSpace(entry.Author),
			PublishedAt:     published.UTC().Format(time.RFC3339),
			OpenMode:        mode,
			ActionURL:       link,
			ContentMarkdown: markdown,
			PlainText:       plain,
			Metadata:        raw,
		})
	}
	return out
}

func ParseHN(data []byte) ([]model.IngestItem, error) {
	var payload struct {
		Hits []struct {
			ObjectID string `json:"objectID"`
			Title    string `json:"title"`
			URL      string `json:"url"`
			Author   string `json:"author"`
			Points   int    `json:"points"`
			Comments int    `json:"num_comments"`
			Created  string `json:"created_at"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	var out []model.IngestItem
	for _, hit := range payload.Hits {
		if hit.ObjectID == "" || strings.TrimSpace(hit.Title) == "" {
			continue
		}
		discussion := "https://news.ycombinator.com/item?id=" + hit.ObjectID
		canonical := strings.TrimSpace(hit.URL)
		if canonical == "" {
			canonical = discussion
		}
		published := hit.Created
		if parsed, err := ParseTime(published); err == nil {
			published = parsed.UTC().Format(time.RFC3339)
		}
		raw, _ := json.Marshal(map[string]any{
			"kind":         "hn",
			"points":       hit.Points,
			"commentCount": hit.Comments,
		})
		out = append(out, model.IngestItem{
			ExternalID:    hit.ObjectID,
			CanonicalURL:  canonical,
			DiscussionURL: discussion,
			Title:         strings.TrimSpace(hit.Title),
			Author:        hit.Author,
			PublishedAt:   published,
			OpenMode:      "external",
			ActionURL:     canonical,
			PlainText:     strings.TrimSpace(hit.Title),
			Metadata:      raw,
		})
	}
	return out, nil
}

func ParseTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		time.RFC1123Z,
		time.RFC1123,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	var last error
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
		last = err
	}
	return time.Time{}, fmt.Errorf("parse time %q: %w", value, last)
}

func parseDuration(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if n, err := strconv.Atoi(value); err == nil {
		return minutes(n)
	}
	parts := strings.Split(value, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0
	}
	nums := make([]int, len(parts))
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			return 0
		}
		nums[i] = n
	}
	seconds := 0
	if len(nums) == 3 {
		seconds = nums[0]*3600 + nums[1]*60 + nums[2]
	} else {
		seconds = nums[0]*60 + nums[1]
	}
	return minutes(seconds)
}

func minutes(seconds int) int {
	if seconds <= 0 {
		return 0
	}
	mins := seconds / 60
	if seconds%60 >= 30 {
		mins++
	}
	if mins == 0 {
		return 1
	}
	return mins
}

func attr(el xml.StartElement, name string) string {
	for _, a := range el.Attr {
		if strings.EqualFold(a.Name.Local, name) {
			return a.Value
		}
	}
	return ""
}

func contains(stack []string, name string) bool {
	for _, item := range stack {
		if item == name {
			return true
		}
	}
	return false
}

func Emails(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if i := strings.Index(part, "<"); i >= 0 {
			if j := strings.Index(part, ">"); j > i {
				part = part[i+1 : j]
			}
		}
		part = strings.Trim(part, `"' `)
		if strings.Contains(part, "@") {
			out = append(out, strings.ToLower(part))
		}
	}
	return out
}

func Slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func NewsletterAlias(name string, taken func(string) bool) string {
	slug := Slug(name)
	if slug == "" {
		slug = "letter"
	}
	alias := "nl-" + slug + "@elephant.local"
	n := 2
	for taken(alias) {
		alias = fmt.Sprintf("nl-%s-%d@elephant.local", slug, n)
		n++
	}
	return alias
}
