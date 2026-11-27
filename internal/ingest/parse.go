package ingest

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"elephant/internal/model"
)

type FeedEntry struct {
	ID              string
	Title           string
	Author          string
	Link            string
	EnclosureURL    string
	EnclosureType   string
	EnclosureLength int64
	Published       time.Time
	HTML            string
	DurationMin     int
}

// FeedLimit caps how many entries a parse keeps.
// A zero MaxItems with FirstSync false parses the whole document.
// On a newest-first feed the parser stops reading once the cap is satisfied.
type FeedLimit struct {
	MaxItems  int
	MinItems  int
	NotBefore time.Time
	FirstSync bool
}

func ParseFeed(data []byte) ([]FeedEntry, error) {
	return ParseFeedReader(bytes.NewReader(data), FeedLimit{})
}

func ParseFeedReader(r io.Reader, lim FeedLimit) ([]FeedEntry, error) {
	dec := xml.NewDecoder(r)
	dec.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }
	var stack []string
	var in bool
	var depth int
	var cur FeedEntry
	var contentRank int
	var items []FeedEntry
	var more bool
	ordered := true
	stop := false
	for !stop {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			if len(items) > 0 && truncatedXML(err) {
				break
			}
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
				if name == "enclosure" {
					if cur.EnclosureURL == "" {
						cur.EnclosureURL = strings.TrimSpace(attr(el, "url"))
					}
					if cur.EnclosureType == "" {
						cur.EnclosureType = strings.TrimSpace(attr(el, "type"))
					}
					if cur.EnclosureLength == 0 {
						if n, convErr := strconv.ParseInt(strings.TrimSpace(attr(el, "length")), 10, 64); convErr == nil {
							cur.EnclosureLength = n
						}
					}
				}
				if name == "duration" {
					if seconds := attr(el, "seconds"); seconds != "" {
						if n, convErr := strconv.Atoi(seconds); convErr == nil {
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
					if keepEntry(cur) {
						if ordered && len(items) > 0 && !publishedDescending(items[len(items)-1], cur) {
							ordered = false
						}
						if ordered && stopBefore(lim, len(items), cur) {
							more = true
							stop = true
							break
						}
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
			case "creator", "author":
				if cur.Author == "" {
					cur.Author = text
				}
			case "name":
				if cur.Author == "" && contains(stack, "author") {
					cur.Author = text
				}
			case "pubdate", "published", "updated", "date":
				if cur.Published.IsZero() {
					if parsed, timeErr := ParseTime(text); timeErr == nil {
						cur.Published = parsed
					}
				}
			case "guid", "id":
				if cur.ID == "" && !contains(stack, "author") {
					cur.ID = text
					if cur.Link == "" && isHTTPURL(text) {
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
				if cur.Link == "" && isHTTPURL(text) {
					cur.Link = text
				}
			}
		}
	}
	return finalizeEntries(items, lim, more), nil
}

func keepEntry(cur FeedEntry) bool {
	return strings.TrimSpace(cur.Title) != "" || strings.TrimSpace(cur.Link) != "" || strings.TrimSpace(cur.EnclosureURL) != ""
}

func publishedDescending(prev, next FeedEntry) bool {
	if prev.Published.IsZero() || next.Published.IsZero() {
		return true
	}
	return !next.Published.After(prev.Published)
}

func stopBefore(lim FeedLimit, have int, next FeedEntry) bool {
	if lim.MaxItems <= 0 && !lim.FirstSync {
		return false
	}
	if !lim.FirstSync {
		return lim.MaxItems > 0 && have >= lim.MaxItems
	}
	if lim.MaxItems > 0 && have < lim.MaxItems {
		return false
	}
	if lim.NotBefore.IsZero() {
		return lim.MaxItems > 0 && have >= lim.MaxItems
	}
	if next.Published.IsZero() || !next.Published.Before(lim.NotBefore) {
		return false
	}
	return true
}

func finalizeEntries(items []FeedEntry, lim FeedLimit, large bool) []FeedEntry {
	if items == nil {
		items = []FeedEntry{}
	}
	items = sortNewest(items)
	if !lim.FirstSync {
		if lim.MaxItems > 0 && len(items) > lim.MaxItems {
			return items[:lim.MaxItems]
		}
		return items
	}
	if lim.MaxItems > 0 && len(items) > lim.MaxItems {
		large = true
	}
	if !large {
		return items
	}
	minItems := lim.MinItems
	if minItems <= 0 {
		minItems = 5
	}
	var out []FeedEntry
	for _, entry := range items {
		if len(out) < minItems {
			out = append(out, entry)
			continue
		}
		if lim.NotBefore.IsZero() || entry.Published.IsZero() || !entry.Published.Before(lim.NotBefore) {
			out = append(out, entry)
			continue
		}
		break
	}
	if out == nil {
		out = []FeedEntry{}
	}
	return out
}

func sortNewest(items []FeedEntry) []FeedEntry {
	out := append([]FeedEntry(nil), items...)
	sort.SliceStable(out, func(i, j int) bool {
		return rankTime(out[i].Published).After(rankTime(out[j].Published))
	})
	return out
}

func rankTime(t time.Time) time.Time {
	if t.IsZero() {
		return time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return t
}

func truncatedXML(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var syn *xml.SyntaxError
	if errors.As(err, &syn) {
		return strings.Contains(strings.ToLower(syn.Error()), "eof")
	}
	return false
}

func isHTTPURL(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "http://")
}

func EntriesToItems(source model.Source, entries []FeedEntry) []model.IngestItem {
	var out []model.IngestItem
	for _, entry := range entries {
		link := strings.TrimSpace(entry.Link)
		if link == "" {
			link = strings.TrimSpace(entry.EnclosureURL)
		}
		if link == "" {
			continue
		}
		external := strings.TrimSpace(entry.ID)
		if external == "" {
			if enc := strings.TrimSpace(entry.EnclosureURL); enc != "" {
				external = enc
			} else {
				external = link
			}
		}
		markdown, plain := HTMLToText(entry.HTML)
		if plain == "" {
			plain = strings.TrimSpace(entry.Title)
		}
		published := entry.Published
		if published.IsZero() {
			published = time.Now().UTC()
		}
		author := strings.TrimSpace(entry.Author)
		if author == "" {
			author = fallbackAuthor(source, link)
		}
		meta := map[string]any{"kind": "generic"}
		switch source.SourceType {
		case "podcast":
			meta = map[string]any{"kind": "podcast", "showName": source.Name, "durationMin": entry.DurationMin}
			if oc := overcastURL(source); oc != "" {
				meta["overcastUrl"] = oc
			}
			if entry.EnclosureURL != "" {
				meta["enclosureUrl"] = entry.EnclosureURL
			}
			if entry.EnclosureType != "" {
				meta["enclosureType"] = entry.EnclosureType
			}
			if entry.EnclosureLength > 0 {
				meta["enclosureLength"] = entry.EnclosureLength
			}
		case "youtube":
			channel := author
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
		if source.SourceType == "podcast" && mode == "deeplink" {
			mode = "reader"
		}
		out = append(out, model.IngestItem{
			SourceID:        source.ID,
			ExternalID:      external,
			CanonicalURL:    link,
			Title:           strings.TrimSpace(entry.Title),
			Author:          author,
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

func overcastURL(source model.Source) string {
	if source.SourceType != "podcast" || source.Config == nil {
		return ""
	}
	if raw, ok := source.Config["overcastUrl"].(string); ok && strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	if id, ok := source.Config["overcastId"].(string); ok && strings.TrimSpace(id) != "" && strings.TrimSpace(source.FeedURL) != "" {
		return "overcast://x-callback-url/add?url=" + url.QueryEscape(source.FeedURL)
	}
	return ""
}

func fallbackAuthor(source model.Source, canonical string) string {
	name := strings.TrimSpace(source.Name)
	switch source.SourceType {
	case "podcast", "youtube", "rss":
		if name != "" {
			return name
		}
	}
	host := hostOf(canonical)
	if host == "" || cdnHost(host) {
		return name
	}
	return host
}

func cdnHost(host string) bool {
	host = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(host), "www."))
	if host == "" {
		return false
	}
	markers := []string{
		"omny.fm",
		"omnystudio.com",
		"libsyn.com",
		"spreaker.com",
		"art19.com",
		"art19-episode-locator",
		"megaphone.fm",
		"simplecast.com",
		"podbean.com",
		"anchor.fm",
		"buzzsprout.com",
		"transistor.fm",
		"captivate.fm",
		"fireside.fm",
		"acast.com",
		"podtrac.com",
		"chtbl.com",
		"pippa.io",
	}
	for _, marker := range markers {
		if host == marker || strings.HasSuffix(host, "."+marker) || strings.Contains(host, marker) {
			return true
		}
	}
	return false
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
