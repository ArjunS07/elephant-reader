package ingest

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"elephant/internal/db"
	"elephant/internal/model"
)

type Service struct {
	Store *db.Store
	HTTP  *http.Client
}

func New(store *db.Store) *Service {
	return &Service{
		Store: store,
		HTTP:  &http.Client{Timeout: 20 * time.Second},
	}
}

type SyncResult struct {
	Sources  int      `json:"sources"`
	Ingested int      `json:"ingested"`
	Errors   []string `json:"errors"`
}

func (s *Service) Ingest(ctx context.Context, inputs []model.IngestItem) ([]model.IngestResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var out []model.IngestResult
	for _, input := range inputs {
		result, err := s.ingestOne(input)
		if err != nil {
			return nil, err
		}
		out = append(out, result)
	}
	if out == nil {
		out = []model.IngestResult{}
	}
	return out, nil
}

func (s *Service) ingestOne(input model.IngestItem) (model.IngestResult, error) {
	input.CanonicalURL = strings.TrimSpace(input.CanonicalURL)
	input.Title = strings.TrimSpace(input.Title)
	if input.CanonicalURL == "" || input.Title == "" {
		return model.IngestResult{}, fmt.Errorf("title and canonicalUrl are required")
	}
	source, err := s.resolveSource(input.SourceID)
	if err != nil {
		return model.IngestResult{}, err
	}
	input.SourceID = source.ID
	if strings.TrimSpace(input.ExternalID) == "" {
		input.ExternalID = input.CanonicalURL
	}
	if strings.TrimSpace(input.Author) == "" {
		input.Author = hostOf(input.CanonicalURL)
	}
	published := time.Now().UTC()
	if strings.TrimSpace(input.PublishedAt) != "" {
		parsed, err := ParseTime(input.PublishedAt)
		if err != nil {
			return model.IngestResult{}, fmt.Errorf("publishedAt: %w", err)
		}
		published = parsed.UTC()
	}
	if !model.ValidOpenMode(input.OpenMode) {
		input.OpenMode = source.DefaultOpenMode
		if !model.ValidOpenMode(input.OpenMode) {
			input.OpenMode = "reader"
		}
	}
	if strings.TrimSpace(input.ActionURL) == "" {
		input.ActionURL = input.CanonicalURL
	}
	meta, err := mergeMetadata(input.Metadata, input.TractionNote, input.DiscussionQuotes)
	if err != nil {
		return model.IngestResult{}, err
	}
	plain := strings.TrimSpace(input.PlainText)
	if plain == "" {
		plain = PlainFromMarkdown(input.ContentMarkdown)
	}
	excerpt := Excerpt(plain, input.ContentMarkdown, meta, input.Excerpt)
	write := db.ItemWrite{
		ID:              strings.TrimSpace(input.ID),
		SourceID:        source.ID,
		ExternalID:      input.ExternalID,
		CanonicalURL:    input.CanonicalURL,
		DiscussionURL:   input.DiscussionURL,
		Title:           input.Title,
		Author:          input.Author,
		PublishedAt:     published.Format(time.RFC3339Nano),
		PublishedDay:    published.In(s.Store.Loc).Format("2006-01-02"),
		OpenMode:        input.OpenMode,
		ActionURL:       input.ActionURL,
		ContentMarkdown: input.ContentMarkdown,
		PlainText:       plain,
		Excerpt:         excerpt,
		Metadata:        meta,
	}
	existing, findErr := s.Store.FindItemID(source.ID, input.ExternalID, input.CanonicalURL)
	if findErr != nil && findErr != db.ErrNotFound {
		return model.IngestResult{}, findErr
	}
	if existing != "" {
		item, err := s.Store.UpdateItem(existing, write, input.InitialState)
		if err != nil {
			return model.IngestResult{}, err
		}
		return model.IngestResult{Item: item, Created: false}, nil
	}
	if write.ID == "" {
		write.ID = newID("item_")
	}
	item, err := s.Store.InsertItem(write, input.InitialState)
	if err != nil {
		again, findErr := s.Store.FindItemID(source.ID, input.ExternalID, input.CanonicalURL)
		if findErr == nil && again != "" {
			item, err = s.Store.UpdateItem(again, write, input.InitialState)
			if err != nil {
				return model.IngestResult{}, err
			}
			return model.IngestResult{Item: item, Created: false}, nil
		}
		return model.IngestResult{}, err
	}
	return model.IngestResult{Item: item, Created: true}, nil
}

func (s *Service) resolveSource(id string) (model.Source, error) {
	if strings.TrimSpace(id) != "" {
		return s.Store.GetSource(id)
	}
	return s.Store.EnsureManualSource()
}

func (s *Service) SyncSource(ctx context.Context, source model.Source) (int, error) {
	items, err := s.fetchSource(ctx, source)
	if err != nil {
		return 0, err
	}
	for i := range items {
		items[i].SourceID = source.ID
	}
	results, err := s.Ingest(ctx, items)
	if err != nil {
		return 0, err
	}
	now := db.Now()
	if _, err := s.Store.PatchSource(source.ID, db.SourcePatch{TouchFetched: true, LastFetchedAt: &now}); err != nil {
		return len(results), err
	}
	return len(results), nil
}

func (s *Service) Sync(ctx context.Context, sourceID string) (SyncResult, error) {
	var sources []model.Source
	if sourceID != "" {
		source, err := s.Store.GetSource(sourceID)
		if err != nil {
			return SyncResult{}, err
		}
		sources = []model.Source{source}
	} else {
		all, err := s.Store.ListSources()
		if err != nil {
			return SyncResult{}, err
		}
		for _, source := range all {
			if source.Enabled && fetchable(source) {
				sources = append(sources, source)
			}
		}
	}
	result := SyncResult{Sources: len(sources), Errors: []string{}}
	for _, source := range sources {
		if !fetchable(source) {
			continue
		}
		n, err := s.SyncSource(ctx, source)
		if err != nil {
			result.Errors = append(result.Errors, source.Name+": "+err.Error())
			continue
		}
		result.Ingested += n
	}
	return result, nil
}

func (s *Service) fetchSource(ctx context.Context, source model.Source) ([]model.IngestItem, error) {
	switch source.SourceType {
	case "hn":
		min := db.MinPoints(source.Config)
		endpoint := "https://hn.algolia.com/api/v1/search_by_date?tags=story&hitsPerPage=40&numericFilters=" + url.QueryEscape(fmt.Sprintf("points>=%d", min))
		body, err := s.get(ctx, endpoint)
		if err != nil {
			return nil, err
		}
		items, err := ParseHN(body)
		if err != nil {
			return nil, err
		}
		var kept []model.IngestItem
		for _, item := range items {
			var meta struct {
				Points int `json:"points"`
			}
			_ = json.Unmarshal(item.Metadata, &meta)
			if meta.Points >= min {
				kept = append(kept, item)
			}
		}
		return kept, nil
	case "rss", "podcast", "youtube":
		if strings.TrimSpace(source.FeedURL) == "" {
			return nil, fmt.Errorf("missing feedUrl")
		}
		body, err := s.get(ctx, source.FeedURL)
		if err != nil {
			return nil, err
		}
		entries, err := ParseFeed(body)
		if err != nil {
			return nil, err
		}
		return EntriesToItems(source, entries), nil
	default:
		if strings.TrimSpace(source.FeedURL) == "" {
			return []model.IngestItem{}, nil
		}
		body, err := s.get(ctx, source.FeedURL)
		if err != nil {
			return nil, err
		}
		entries, err := ParseFeed(body)
		if err != nil {
			return nil, err
		}
		return EntriesToItems(source, entries), nil
	}
}

func (s *Service) InboundEmail(to, from, subject, htmlBody, text, receivedAt string) (model.IngestResult, error) {
	var source model.Source
	var found bool
	for _, address := range Emails(to) {
		match, err := s.Store.FindSourceByAlias(address)
		if err == nil {
			source = match
			found = true
			break
		}
		if err != db.ErrNotFound {
			return model.IngestResult{}, err
		}
	}
	if !found {
		return model.IngestResult{}, fmt.Errorf("no newsletter source matches %q", to)
	}
	markdown := strings.TrimSpace(text)
	plain := PlainFromMarkdown(markdown)
	if strings.TrimSpace(htmlBody) != "" {
		md, extracted := HTMLToText(htmlBody)
		if strings.TrimSpace(md) != "" {
			markdown = md
			plain = extracted
		}
	}
	if strings.TrimSpace(subject) == "" {
		subject = "(no subject)"
	}
	if plain == "" {
		plain = subject
	}
	when := receivedAt
	if strings.TrimSpace(when) == "" {
		when = time.Now().UTC().Format(time.RFC3339)
	}
	sum := sha256.Sum256([]byte(strings.ToLower(source.EmailAlias) + "\n" + subject + "\n" + when))
	id := hex.EncodeToString(sum[:8])
	author := from
	if emails := Emails(from); len(emails) > 0 {
		author = emails[0]
	}
	if author == "" {
		author = source.Name
	}
	return s.ingestOne(model.IngestItem{
		SourceID:        source.ID,
		ExternalID:      "email-" + id,
		CanonicalURL:    "https://newsletter.local/" + source.ID + "/" + id,
		Title:           subject,
		Author:          author,
		PublishedAt:     when,
		OpenMode:        source.DefaultOpenMode,
		ActionURL:       "https://newsletter.local/" + source.ID + "/" + id,
		ContentMarkdown: markdown,
		PlainText:       plain,
		Metadata:        json.RawMessage(`{"kind":"generic"}`),
	})
}

func (s *Service) get(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "elephant/1.0")
	client := s.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch %s: %s", endpoint, res.Status)
	}
	return body, nil
}

func fetchable(source model.Source) bool {
	switch source.SourceType {
	case "hn":
		return true
	case "rss", "podcast", "youtube":
		return strings.TrimSpace(source.FeedURL) != ""
	default:
		return strings.TrimSpace(source.FeedURL) != ""
	}
}

func mergeMetadata(raw json.RawMessage, traction string, quotes []model.DiscussionQuote) (json.RawMessage, error) {
	meta := map[string]any{}
	if len(bytesTrim(raw)) > 0 {
		if err := json.Unmarshal(raw, &meta); err != nil {
			return nil, fmt.Errorf("metadata: %w", err)
		}
	}
	if _, ok := meta["kind"]; !ok {
		meta["kind"] = "generic"
	}
	if strings.TrimSpace(traction) != "" {
		meta["tractionNote"] = traction
	}
	if len(quotes) > 0 {
		meta["discussionQuotes"] = quotes
	}
	return json.Marshal(meta)
}

func bytesTrim(raw json.RawMessage) []byte {
	return []byte(strings.TrimSpace(string(raw)))
}

func hostOf(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" {
		return raw
	}
	return strings.TrimPrefix(parsed.Hostname(), "www.")
}

func newID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		sum := sha256.Sum256([]byte(prefix + time.Now().UTC().Format(time.RFC3339Nano)))
		return prefix + hex.EncodeToString(sum[:8])
	}
	return prefix + hex.EncodeToString(buf)
}
