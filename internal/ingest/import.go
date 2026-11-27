package ingest

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"elephant/internal/db"
	"elephant/internal/model"
)

type ImportResult struct {
	Added   int `json:"added"`
	Skipped int `json:"skipped"`
	Total   int `json:"total"`
}

type ImportOptions struct {
	Filename string
	Format   string
	Lookup   func(ctx context.Context, title, feedURL string) (string, error)
}

type sourceDraft struct {
	Name            string
	SourceType      string
	Folder          string
	DefaultOpenMode string
	DefaultQueue    string
	FeedURL         string
	Config          map[string]any
	Overcast        bool
}

func ImportBytes(ctx context.Context, store *db.Store, data []byte, opt ImportOptions) (ImportResult, error) {
	format := DetectFormat(opt.Filename, opt.Format, data)
	var drafts []sourceDraft
	var err error
	switch format {
	case "opml":
		drafts, err = ParseOPML(data)
	default:
		drafts, err = ParseYouTubeCSV(data)
	}
	if err != nil {
		return ImportResult{}, err
	}
	if err := enrichOvercast(ctx, drafts, opt.Lookup); err != nil {
		return ImportResult{}, err
	}
	existing, err := store.FeedURLs()
	if err != nil {
		return ImportResult{}, err
	}
	seen := map[string]struct{}{}
	for raw := range existing {
		seen[NormalizeFeedURL(raw)] = struct{}{}
	}
	result := ImportResult{}
	for _, draft := range drafts {
		key := NormalizeFeedURL(draft.FeedURL)
		if key == "" || draft.Name == "" {
			continue
		}
		result.Total++
		if _, ok := seen[key]; ok {
			result.Skipped++
			continue
		}
		seen[key] = struct{}{}
		if draft.Config == nil {
			draft.Config = map[string]any{}
		}
		if _, err := store.InsertSource(model.Source{
			ID:              newID("src_"),
			Name:            draft.Name,
			SourceType:      draft.SourceType,
			Folder:          draft.Folder,
			DefaultOpenMode: draft.DefaultOpenMode,
			DefaultQueue:    draft.DefaultQueue,
			FeedURL:         strings.TrimSpace(draft.FeedURL),
			Config:          draft.Config,
			Enabled:         true,
		}); err != nil {
			return result, err
		}
		result.Added++
	}
	return result, nil
}

func DetectFormat(name, hint string, data []byte) string {
	switch strings.ToLower(strings.TrimSpace(hint)) {
	case "youtube", "csv":
		return "youtube"
	case "opml", "xml":
		return "opml"
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".csv":
		return "youtube"
	case ".opml", ".xml":
		return "opml"
	}
	trimmed := bytes.TrimSpace(data)
	if bytes.HasPrefix(trimmed, []byte("<")) || bytes.HasPrefix(bytes.ToLower(trimmed), []byte("<?xml")) {
		return "opml"
	}
	return "youtube"
}

func ParseYouTubeCSV(data []byte) ([]sourceDraft, error) {
	data = bytes.TrimPrefix(data, []byte("\uFEFF"))
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	idCol, titleCol := 0, 2
	start := 0
	if len(rows) > 0 && looksLikeHeader(rows[0]) {
		idCol, titleCol = headerIndexes(rows[0])
		start = 1
	}
	var out []sourceDraft
	for _, row := range rows[start:] {
		if len(row) == 0 || strings.TrimSpace(strings.Join(row, "")) == "" {
			continue
		}
		channelID := field(row, idCol)
		title := field(row, titleCol)
		if channelID == "" {
			continue
		}
		if title == "" {
			title = channelID
		}
		out = append(out, sourceDraft{
			Name:            title,
			SourceType:      "youtube",
			Folder:          "Video",
			DefaultOpenMode: "external",
			DefaultQueue:    "feed",
			FeedURL:         "https://www.youtube.com/feeds/videos.xml?channel_id=" + url.QueryEscape(channelID),
			Config:          map[string]any{},
		})
	}
	return out, nil
}

func ParseOPML(data []byte) ([]sourceDraft, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }
	var headTitle string
	var inTitle bool
	var overcast bool
	type frame struct {
		name string
		feed bool
	}
	var stack []frame
	var outlines []struct {
		name     string
		xmlURL   string
		overcast string
		parent   string
	}
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
			local := strings.ToLower(el.Name.Local)
			if local == "title" && len(stack) == 0 {
				inTitle = true
			}
			if local != "outline" {
				continue
			}
			text := attr(el, "text")
			if text == "" {
				text = attr(el, "title")
			}
			xmlURL := attr(el, "xmlUrl")
			overcastID := attr(el, "overcastId")
			if overcastID != "" {
				overcast = true
			}
			parent := ""
			for i := len(stack) - 1; i >= 0; i-- {
				if !stack[i].feed && stack[i].name != "" {
					parent = stack[i].name
					break
				}
			}
			if xmlURL != "" {
				outlines = append(outlines, struct {
					name     string
					xmlURL   string
					overcast string
					parent   string
				}{text, xmlURL, overcastID, parent})
				stack = append(stack, frame{name: text, feed: true})
			} else {
				stack = append(stack, frame{name: text})
			}
		case xml.EndElement:
			local := strings.ToLower(el.Name.Local)
			if local == "title" {
				inTitle = false
			}
			if local == "outline" && len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if inTitle {
				headTitle += string(el)
			}
		}
	}
	if strings.Contains(strings.ToLower(headTitle), "overcast") {
		overcast = true
	}
	var out []sourceDraft
	for _, outline := range outlines {
		feedURL := strings.TrimSpace(outline.xmlURL)
		if feedURL == "" {
			continue
		}
		name := strings.TrimSpace(outline.name)
		if name == "" {
			name = hostOf(feedURL)
		}
		draft := sourceDraft{
			Name:         name,
			FeedURL:      feedURL,
			DefaultQueue: "feed",
			Config:       map[string]any{},
		}
		if overcast {
			draft.SourceType = "podcast"
			draft.Folder = "Podcasts"
			draft.DefaultOpenMode = "deeplink"
			draft.Overcast = true
			if outline.overcast != "" {
				draft.Config["overcastId"] = outline.overcast
			}
		} else {
			draft.SourceType = "rss"
			draft.DefaultOpenMode = "reader"
			if strings.TrimSpace(outline.parent) != "" {
				draft.Folder = strings.TrimSpace(outline.parent)
			} else {
				draft.Folder = "RSS"
			}
		}
		out = append(out, draft)
	}
	return out, nil
}

func enrichOvercast(ctx context.Context, drafts []sourceDraft, lookup func(context.Context, string, string) (string, error)) error {
	if lookup == nil {
		lookup = LookupItunes
	}
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for i := range drafts {
		if !drafts[i].Overcast {
			continue
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			job, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			itunesID, err := lookup(job, drafts[i].Name, drafts[i].FeedURL)
			mu.Lock()
			defer mu.Unlock()
			if drafts[i].Config == nil {
				drafts[i].Config = map[string]any{}
			}
			if err == nil && strings.TrimSpace(itunesID) != "" {
				id := strings.TrimSpace(itunesID)
				drafts[i].Config["itunesId"] = id
				drafts[i].Config["overcastUrl"] = "https://overcast.fm/itunes" + id
				return
			}
			drafts[i].Config["overcastUrl"] = "overcast://x-callback-url/add?url=" + url.QueryEscape(drafts[i].FeedURL)
		}(i)
	}
	wg.Wait()
	return ctx.Err()
}

func LookupItunes(ctx context.Context, title, feedURL string) (string, error) {
	if strings.TrimSpace(title) == "" {
		return "", fmt.Errorf("missing title")
	}
	endpoint := "https://itunes.apple.com/search?entity=podcast&limit=8&term=" + url.QueryEscape(title)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "elephant/1.0")
	client := &http.Client{Timeout: 2 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("itunes %s", res.Status)
	}
	var payload struct {
		Results []struct {
			CollectionID int64  `json:"collectionId"`
			FeedURL      string `json:"feedUrl"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	want := NormalizeFeedURL(feedURL)
	for _, result := range payload.Results {
		if result.CollectionID == 0 {
			continue
		}
		if NormalizeFeedURL(result.FeedURL) == want {
			return fmt.Sprintf("%d", result.CollectionID), nil
		}
	}
	return "", fmt.Errorf("no itunes match")
}

func NormalizeFeedURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return strings.TrimRight(strings.ToLower(raw), "/")
	}
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	if parsed.Path != "/" {
		parsed.Path = strings.TrimRight(parsed.Path, "/")
	}
	return parsed.String()
}

func looksLikeHeader(row []string) bool {
	for _, cell := range row {
		if strings.Contains(strings.ToLower(cell), "channel") {
			return true
		}
	}
	return false
}

func headerIndexes(row []string) (idCol, titleCol int) {
	idCol, titleCol = 0, 2
	for i, cell := range row {
		name := strings.ToLower(strings.TrimSpace(cell))
		switch {
		case strings.Contains(name, "channel id"):
			idCol = i
		case strings.Contains(name, "title"):
			titleCol = i
		}
	}
	return idCol, titleCol
}

func field(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}
