package ingest

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"regexp"
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
		if IsYouTubeShort(source, link, entry.ID, entry.Title) {
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
		action := link
		if overcast := overcastAction(source); overcast != "" {
			action = overcast
			mode = "deeplink"
		}
		out = append(out, model.IngestItem{
			SourceID:        source.ID,
			ExternalID:      external,
			CanonicalURL:    link,
			Title:           strings.TrimSpace(entry.Title),
			Author:          strings.TrimSpace(entry.Author),
			PublishedAt:     published.UTC().Format(time.RFC3339),
			OpenMode:        mode,
			ActionURL:       action,
			ContentMarkdown: markdown,
			PlainText:       plain,
			Metadata:        raw,
		})
	}
	return out
}

func overcastAction(source model.Source) string {
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

// feedTimeLayouts covers Atom, RSS, RFC 3339, and the RFC 2822/822 variants
// podcast feeds actually emit. Go's RFC1123 requires a zero-padded day, so
// single-digit days ("Fri, 4 Mar 2022") need their own layouts.
var feedTimeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	time.RFC1123Z,
	time.RFC1123,
	time.RFC850,
	time.RFC822Z,
	time.RFC822,
	time.RubyDate,
	time.UnixDate,
	time.ANSIC,
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"Mon, 2 Jan 2006 15:04:05 Z0700",
	"Mon, 2 Jan 2006 15:04:05 -07:00",
	"Mon, 2 Jan 2006 15:04:05 Z07:00",
	"Mon, 2 Jan 2006 15:04:05 MST",
	"Mon, 2 Jan 2006 15:04:05 Z",
	"Mon, 02 Jan 2006 15:04:05 -0700",
	"Mon, 02 Jan 2006 15:04:05 -07:00",
	"Mon, 02 Jan 2006 15:04:05 MST",
	"Monday, 2 Jan 2006 15:04:05 -0700",
	"Monday, 2 Jan 2006 15:04:05 MST",
	"Monday, 02 Jan 2006 15:04:05 -0700",
	"Monday, 02 Jan 2006 15:04:05 MST",
	"Mon, 2 Jan 2006 15:04 -0700",
	"Mon, 2 Jan 2006 15:04 -07:00",
	"Mon, 2 Jan 2006 15:04 MST",
	"Mon, 02 Jan 2006 15:04 -0700",
	"Mon, 02 Jan 2006 15:04 -07:00",
	"Mon, 02 Jan 2006 15:04 MST",
	"Mon, 2 Jan 2006 15:04:05",
	"Mon, 02 Jan 2006 15:04:05",
	"Mon, 2 Jan 2006 15:04",
	"Mon, 02 Jan 2006 15:04",
	"2 Jan 2006 15:04:05 -0700",
	"2 Jan 2006 15:04:05 -07:00",
	"2 Jan 2006 15:04:05 MST",
	"02 Jan 2006 15:04:05 -0700",
	"02 Jan 2006 15:04:05 -07:00",
	"02 Jan 2006 15:04:05 MST",
	"2 Jan 2006 15:04 -0700",
	"2 Jan 2006 15:04 MST",
	"02 Jan 2006 15:04 -0700",
	"02 Jan 2006 15:04 MST",
	"2 Jan 2006 15:04:05",
	"02 Jan 2006 15:04:05",
	"Mon, 2 Jan 06 15:04:05 -0700",
	"Mon, 2 Jan 06 15:04:05 -07:00",
	"Mon, 2 Jan 06 15:04:05 MST",
	"Mon, 02 Jan 06 15:04:05 -0700",
	"Mon, 02 Jan 06 15:04:05 MST",
	"Mon, 2 Jan 06 15:04 -0700",
	"Mon, 2 Jan 06 15:04 MST",
	"Mon, 02 Jan 06 15:04 -0700",
	"Mon, 02 Jan 06 15:04 MST",
	"2 Jan 06 15:04:05 -0700",
	"2 Jan 06 15:04:05 MST",
	"02 Jan 06 15:04:05 -0700",
	"02 Jan 06 15:04:05 MST",
	"2 Jan 06 15:04 -0700",
	"2 Jan 06 15:04 MST",
	"02 Jan 06 15:04 -0700",
	"02 Jan 06 15:04 MST",
	"2006-01-02 15:04:05 -0700",
	"2006-01-02 15:04:05 -07:00",
	"2006-01-02 15:04:05 MST",
	"2006-01-02T15:04:05-0700",
	"2006-01-02T15:04:05-07:00",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02",
}

var (
	parenCommentRE = regexp.MustCompile(`\s*\([^)]*\)`)
	spaceCommaRE   = regexp.MustCompile(`\s+,`)
)

// abbrevOffset is the UTC offset, in seconds, of zone abbreviations Go does
// not know when the process location is UTC. RSS feeds name the zone
// ("EST", "PST") instead of sending a numeric offset.
var abbrevOffset = map[string]int{
	"GMT":  0,
	"UTC":  0,
	"WET":  0,
	"EST":  -5 * 3600,
	"EDT":  -4 * 3600,
	"CST":  -6 * 3600,
	"CDT":  -5 * 3600,
	"MST":  -7 * 3600,
	"MDT":  -6 * 3600,
	"PST":  -8 * 3600,
	"PDT":  -7 * 3600,
	"AKST": -9 * 3600,
	"AKDT": -8 * 3600,
	"HST":  -10 * 3600,
	"AST":  -4 * 3600,
	"ADT":  -3 * 3600,
	"NST":  -3*3600 - 30*60,
	"NDT":  -2*3600 - 30*60,
	"CET":  1 * 3600,
	"CEST": 2 * 3600,
	"BST":  1 * 3600,
	"WEST": 1 * 3600,
	"EET":  2 * 3600,
	"EEST": 3 * 3600,
	"JST":  9 * 3600,
	"KST":  9 * 3600,
	"AEST": 10 * 3600,
	"AEDT": 11 * 3600,
	"ACST": 9*3600 + 30*60,
	"ACDT": 10*3600 + 30*60,
	"AWST": 8 * 3600,
	"NZST": 12 * 3600,
	"NZDT": 13 * 3600,
}

func ParseTime(value string) (time.Time, error) {
	original := value
	value = normalizeFeedTime(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("parse time %q: empty", strings.TrimSpace(original))
	}
	var last error
	for _, layout := range feedTimeLayouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return applyZoneAbbrev(parsed), nil
		}
		last = err
	}
	return time.Time{}, fmt.Errorf("parse time %q: %w", strings.TrimSpace(original), last)
}

func normalizeFeedTime(value string) string {
	value = strings.TrimSpace(value)
	value = parenCommentRE.ReplaceAllString(value, " ")
	if i := strings.IndexByte(value, '('); i >= 0 {
		value = value[:i]
	}
	value = spaceCommaRE.ReplaceAllString(value, ",")
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	if zone, ok := canonicalZone(fields[len(fields)-1]); ok {
		fields[len(fields)-1] = zone
	}
	value = strings.Join(fields, " ")
	if len(value) >= 11 && value[10] == 't' && strings.IndexByte(value, ' ') < 0 {
		value = value[:10] + "T" + value[11:]
	}
	if strings.HasSuffix(value, "z") && strings.Contains(value, "T") {
		value = value[:len(value)-1] + "Z"
	}
	return value
}

func canonicalZone(token string) (string, bool) {
	if token == "" {
		return "", false
	}
	switch token[0] {
	case '+', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return "", false
	}
	upper := strings.ToUpper(token)
	for _, r := range upper {
		if r < 'A' || r > 'Z' {
			return "", false
		}
	}
	if len(upper) < 1 || len(upper) > 5 {
		return "", false
	}
	if upper == "UT" {
		return "UTC", true
	}
	return upper, true
}

func applyZoneAbbrev(t time.Time) time.Time {
	name, offset := t.Zone()
	if offset != 0 {
		return t
	}
	want, ok := abbrevOffset[strings.ToUpper(name)]
	if !ok || want == 0 {
		return t
	}
	utc := t.In(time.UTC)
	return time.Date(utc.Year(), utc.Month(), utc.Day(), utc.Hour(), utc.Minute(), utc.Second(), utc.Nanosecond(), time.FixedZone(strings.ToUpper(name), want))
}

// IsYouTubeShort reports whether an item is a YouTube Short.
// A Short is a YouTube URL whose path contains /shorts/, or a YouTube item
// whose title carries a #short or #shorts hashtag. Duration is ignored so a
// one-minute /watch video is kept.
func IsYouTubeShort(source model.Source, canonicalURL, actionURL, title string) bool {
	if youtubeShortsPath(canonicalURL) || youtubeShortsPath(actionURL) {
		return true
	}
	if source.SourceType == "youtube" || youtubePage(canonicalURL) || youtubePage(actionURL) {
		return shortsHashtag(title)
	}
	return false
}

var shortsHashtagRE = regexp.MustCompile(`(?i)(?:^|[^A-Za-z0-9_])#shorts?(?:$|[^A-Za-z0-9_])`)

func shortsHashtag(title string) bool {
	return shortsHashtagRE.MatchString(title)
}

func youtubeShortsPath(raw string) bool {
	if !strings.Contains(strings.ToLower(raw), "/shorts/") {
		return false
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" || !youtubeHost(parsed.Hostname()) {
		return false
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = parsed.Path
	}
	return strings.Contains(strings.ToLower(path), "/shorts/")
}

func youtubePage(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" {
		return false
	}
	return youtubeHost(parsed.Hostname())
}

func youtubeHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	host = strings.TrimPrefix(host, "www.")
	switch host {
	case "youtube.com", "youtu.be", "youtube-nocookie.com":
		return true
	default:
		return strings.HasSuffix(host, ".youtube.com")
	}
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
