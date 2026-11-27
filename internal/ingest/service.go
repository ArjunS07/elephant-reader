package ingest

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	mathrand "math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"elephant/internal/db"
	"elephant/internal/model"
)

const (
	syncWorkers       = 4
	hostSlots         = 2
	maxFeedBytes      = 32 << 20
	syncAttempts      = 3
	syncSourceTimeout = 60 * time.Second
	catchUpStale      = 60 * time.Minute
)

type SyncOptions struct {
	MaxItems     int
	BackfillDays int
	MinItems     int
}

func SyncOptionsFromEnv() SyncOptions {
	opts := SyncOptions{MaxItems: 25, BackfillDays: 60, MinItems: 5}
	if n, ok := envInt("ELEPHANT_MAX_ITEMS_PER_FEED"); ok && n > 0 {
		opts.MaxItems = n
	}
	if n, ok := envInt("ELEPHANT_BACKFILL_DAYS"); ok && n >= 0 {
		opts.BackfillDays = n
	}
	return opts
}

func envInt(key string) (int, bool) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0, false
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return n, true
}

type Service struct {
	Store   *db.Store
	HTTP    *http.Client
	Opts    SyncOptions
	Backoff func(attempt int) time.Duration

	hosts *hostGate
	mu    sync.Mutex
	state syncState
}

func New(store *db.Store) *Service {
	return &Service{
		Store:   store,
		HTTP:    &http.Client{Timeout: 15 * time.Second},
		Opts:    SyncOptionsFromEnv(),
		Backoff: defaultBackoff,
		hosts:   newHostGate(hostSlots),
	}
}

type SyncResult struct {
	Synced      int      `json:"synced"`
	NotModified int      `json:"notModified"`
	Failed      int      `json:"failed"`
	ItemsAdded  int      `json:"itemsAdded"`
	Errors      []string `json:"errors"`
}

type SyncStatus struct {
	Running    bool     `json:"running"`
	StartedAt  *string  `json:"startedAt"`
	FinishedAt *string  `json:"finishedAt"`
	Total      int      `json:"total"`
	Completed  int      `json:"completed"`
	Failed     int      `json:"failed"`
	Errors     []string `json:"errors"`
}

type syncState struct {
	running   bool
	started   time.Time
	finished  time.Time
	total     int
	completed int
	failed    int
	errors    []string
}

func (st syncState) view() SyncStatus {
	out := SyncStatus{
		Running:   st.running,
		Total:     st.total,
		Completed: st.completed,
		Failed:    st.failed,
		Errors:    append([]string{}, st.errors...),
	}
	if out.Errors == nil {
		out.Errors = []string{}
	}
	if !st.started.IsZero() {
		value := st.started.UTC().Format(time.RFC3339Nano)
		out.StartedAt = &value
	}
	if !st.running && !st.finished.IsZero() {
		value := st.finished.UTC().Format(time.RFC3339Nano)
		out.FinishedAt = &value
	}
	return out
}

func (s *Service) Status() SyncStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.view()
}

// StartSync launches one full sync detached from any request context.
// A sync that is already running is left alone and its status is returned.
func (s *Service) StartSync() SyncStatus {
	s.mu.Lock()
	if s.state.running {
		st := s.state.view()
		s.mu.Unlock()
		return st
	}
	s.state = syncState{running: true, started: time.Now().UTC(), errors: []string{}}
	s.mu.Unlock()
	go s.background()
	return s.Status()
}

func (s *Service) background() {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("sync: panic: %v", rec)
		}
		s.mu.Lock()
		s.state.running = false
		s.state.finished = time.Now().UTC()
		s.mu.Unlock()
	}()
	_, _ = s.runSync(context.Background(), "", true)
}

// CatchUp starts a background sync when startup sync is enabled and the newest
// successful fetch is missing or older than an hour.
func (s *Service) CatchUp() {
	if !catchUpEnabled() {
		log.Printf("sync: catch-up disabled")
		return
	}
	newest, err := s.Store.NewestLastFetched()
	if err != nil {
		log.Printf("sync: catch-up skipped: %v", err)
		return
	}
	if newest != nil && time.Since(*newest) < catchUpStale {
		log.Printf("sync: catch-up skipped, last fetch %s", newest.UTC().Format(time.RFC3339))
		return
	}
	log.Printf("sync: starting catch-up")
	s.StartSync()
}

func catchUpEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ELEPHANT_SYNC_ON_START"))) {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func (s *Service) Ingest(ctx context.Context, inputs []model.IngestItem) ([]model.IngestResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(inputs) == 0 {
		return []model.IngestResult{}, nil
	}
	prepared := make([]db.PreparedItem, 0, len(inputs))
	for _, input := range inputs {
		item, err := s.prepare(input)
		if err != nil {
			return nil, err
		}
		prepared = append(prepared, item)
	}
	return s.Store.CommitIngest(prepared)
}

func (s *Service) prepare(input model.IngestItem) (db.PreparedItem, error) {
	input.CanonicalURL = strings.TrimSpace(input.CanonicalURL)
	input.Title = strings.TrimSpace(input.Title)
	if input.CanonicalURL == "" || input.Title == "" {
		return db.PreparedItem{}, fmt.Errorf("title and canonicalUrl are required")
	}
	source, err := s.resolveSource(input.SourceID)
	if err != nil {
		return db.PreparedItem{}, err
	}
	input.SourceID = source.ID
	if strings.TrimSpace(input.ExternalID) == "" {
		input.ExternalID = input.CanonicalURL
	}
	if strings.TrimSpace(input.Author) == "" {
		input.Author = fallbackAuthor(source, input.CanonicalURL)
	}
	published := time.Now().UTC()
	if strings.TrimSpace(input.PublishedAt) != "" {
		parsed, err := ParseTime(input.PublishedAt)
		if err != nil {
			return db.PreparedItem{}, fmt.Errorf("publishedAt: %w", err)
		}
		published = parsed.UTC()
	}
	if !model.ValidOpenMode(input.OpenMode) {
		input.OpenMode = source.DefaultOpenMode
		if !model.ValidOpenMode(input.OpenMode) {
			input.OpenMode = "reader"
		}
		if source.SourceType == "podcast" && input.OpenMode == "deeplink" {
			input.OpenMode = "reader"
		}
	}
	if strings.TrimSpace(input.ActionURL) == "" {
		input.ActionURL = input.CanonicalURL
	}
	meta, err := mergeMetadata(input.Metadata, input.TractionNote, input.DiscussionQuotes)
	if err != nil {
		return db.PreparedItem{}, err
	}
	plain := strings.TrimSpace(input.PlainText)
	if plain == "" {
		plain = PlainFromMarkdown(input.ContentMarkdown)
	}
	loc := time.UTC
	if s.Store != nil && s.Store.Loc != nil {
		loc = s.Store.Loc
	}
	if strings.TrimSpace(input.ID) == "" {
		input.ID = newID("item_")
	}
	return db.PreparedItem{
		Write: db.ItemWrite{
			ID:              strings.TrimSpace(input.ID),
			SourceID:        source.ID,
			ExternalID:      input.ExternalID,
			CanonicalURL:    input.CanonicalURL,
			DiscussionURL:   input.DiscussionURL,
			Title:           input.Title,
			Author:          input.Author,
			PublishedAt:     published.Format(time.RFC3339Nano),
			PublishedDay:    published.In(loc).Format("2006-01-02"),
			OpenMode:        input.OpenMode,
			ActionURL:       input.ActionURL,
			ContentMarkdown: input.ContentMarkdown,
			PlainText:       plain,
			Excerpt:         Excerpt(plain, input.ContentMarkdown, meta, input.Excerpt, input.Title),
			Metadata:        meta,
		},
		Initial: input.InitialState,
	}, nil
}

func (s *Service) ingestOne(input model.IngestItem) (model.IngestResult, error) {
	results, err := s.Ingest(context.Background(), []model.IngestItem{input})
	if err != nil {
		return model.IngestResult{}, err
	}
	return results[0], nil
}

func (s *Service) resolveSource(id string) (model.Source, error) {
	if strings.TrimSpace(id) != "" {
		return s.Store.GetSource(id)
	}
	return s.Store.EnsureManualSource()
}

func (s *Service) SyncSource(ctx context.Context, source model.Source) (int, bool, error) {
	items, fresh, notModified, err := s.fetchSource(ctx, source)
	if err != nil {
		s.recordFailure(source.ID, err)
		return 0, false, err
	}
	now := db.Now()
	status := "ok"
	empty := ""
	patch := db.SourcePatch{
		TouchFetched:   true,
		LastFetchedAt:  &now,
		TouchSyncState: true,
		LastStatus:     &status,
		LastError:      &empty,
	}
	if notModified {
		status = "not_modified"
		if _, err := s.Store.PatchSource(source.ID, patch); err != nil {
			return 0, true, err
		}
		return 0, true, nil
	}
	etag := fresh.ETag
	if etag == "" {
		etag = source.ETag
	}
	modified := fresh.LastModified
	if modified == "" {
		modified = source.LastModified
	}
	patch.TouchValidators = true
	patch.ETag = &etag
	patch.LastModified = &modified
	for i := range items {
		items[i].SourceID = source.ID
	}
	results, err := s.Ingest(ctx, items)
	if err != nil {
		s.recordFailure(source.ID, err)
		return 0, false, err
	}
	if _, err := s.Store.PatchSource(source.ID, patch); err != nil {
		return 0, false, err
	}
	added := 0
	for _, result := range results {
		if result.Created {
			added++
		}
	}
	return added, false, nil
}

func (s *Service) recordFailure(id string, syncErr error) {
	msg := syncErr.Error()
	status := "error"
	_, _ = s.Store.PatchSource(id, db.SourcePatch{
		TouchSyncState: true,
		LastStatus:     &status,
		LastError:      &msg,
	})
}

func (s *Service) Sync(ctx context.Context, sourceID string) (SyncResult, error) {
	return s.runSync(ctx, sourceID, false)
}

func (s *Service) runSync(ctx context.Context, sourceID string, track bool) (SyncResult, error) {
	started := time.Now()
	result := SyncResult{Errors: []string{}}
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
			if track {
				s.mu.Lock()
				s.state.failed++
				s.state.errors = append(s.state.errors, err.Error())
				s.mu.Unlock()
			}
			return SyncResult{}, err
		}
		for _, source := range all {
			if source.Enabled && fetchable(source) {
				sources = append(sources, source)
			}
		}
	}
	queue := make([]model.Source, 0, len(sources))
	for _, source := range sources {
		if fetchable(source) {
			queue = append(queue, source)
		}
	}
	if track {
		s.mu.Lock()
		s.state.total = len(queue)
		s.mu.Unlock()
	}
	if len(queue) == 0 {
		logSummary(result, time.Since(started))
		return result, nil
	}
	jobs := make(chan model.Source)
	var wg sync.WaitGroup
	workers := syncWorkers
	if workers > len(queue) {
		workers = len(queue)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for source := range jobs {
				jobCtx, cancel := context.WithTimeout(ctx, syncSourceTimeout)
				added, unchanged, err := s.SyncSource(jobCtx, source)
				cancel()
				s.mu.Lock()
				if err != nil {
					result.Failed++
					msg := source.Name + ": " + err.Error()
					result.Errors = append(result.Errors, msg)
					if track {
						s.state.failed++
						s.state.errors = append(s.state.errors, msg)
					}
				} else if unchanged {
					result.NotModified++
					if track {
						s.state.completed++
					}
				} else {
					result.Synced++
					result.ItemsAdded += added
					if track {
						s.state.completed++
					}
				}
				s.mu.Unlock()
			}
		}()
	}
	for _, source := range queue {
		jobs <- source
	}
	close(jobs)
	wg.Wait()
	logSummary(result, time.Since(started))
	return result, nil
}

func logSummary(result SyncResult, elapsed time.Duration) {
	log.Printf("sync: total %d ok %d unchanged %d failed %d added %d (%s)",
		result.Synced+result.NotModified+result.Failed, result.Synced, result.NotModified, result.Failed, result.ItemsAdded,
		elapsed.Round(time.Millisecond))
	for _, item := range result.Errors {
		log.Printf("sync: %s", item)
	}
}

type feedFetch struct {
	ETag         string
	LastModified string
}

func (s *Service) fetchSource(ctx context.Context, source model.Source) ([]model.IngestItem, feedFetch, bool, error) {
	switch source.SourceType {
	case "hn":
		min := db.MinPoints(source.Config)
		endpoint := "https://hn.algolia.com/api/v1/search_by_date?tags=story&hitsPerPage=40&numericFilters=" + url.QueryEscape(fmt.Sprintf("points>=%d", min))
		res, fresh, notModified, err := s.open(ctx, endpoint, source.ETag, source.LastModified)
		if err != nil || notModified {
			return nil, fresh, notModified, err
		}
		defer res.Body.Close()
		body, err := io.ReadAll(io.LimitReader(res.Body, maxFeedBytes))
		if err != nil {
			return nil, fresh, false, err
		}
		items, err := ParseHN(body)
		if err != nil {
			return nil, fresh, false, err
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
		if max := s.maxItems(); max > 0 && len(kept) > max {
			kept = kept[:max]
		}
		return kept, fresh, false, nil
	default:
		if strings.TrimSpace(source.FeedURL) == "" {
			if source.SourceType == "rss" || source.SourceType == "podcast" || source.SourceType == "youtube" {
				return nil, feedFetch{}, false, fmt.Errorf("missing feedUrl")
			}
			return []model.IngestItem{}, feedFetch{}, false, nil
		}
		res, fresh, notModified, err := s.open(ctx, source.FeedURL, source.ETag, source.LastModified)
		if err != nil || notModified {
			return nil, fresh, notModified, err
		}
		defer res.Body.Close()
		entries, err := ParseFeedReader(io.LimitReader(res.Body, maxFeedBytes), s.feedLimit(source))
		if err != nil {
			return nil, fresh, false, err
		}
		return EntriesToItems(source, entries), fresh, false, nil
	}
}

func (s *Service) open(ctx context.Context, endpoint, etag, modified string) (*http.Response, feedFetch, bool, error) {
	res, err := s.getResponse(ctx, endpoint, etag, modified)
	if err != nil {
		return nil, feedFetch{}, false, err
	}
	fresh := feedFetch{ETag: res.Header.Get("ETag"), LastModified: res.Header.Get("Last-Modified")}
	if res.StatusCode == http.StatusNotModified {
		res.Body.Close()
		return nil, fresh, true, nil
	}
	return res, fresh, false, nil
}

func (s *Service) feedLimit(source model.Source) FeedLimit {
	first := source.LastFetchedAt == nil || strings.TrimSpace(*source.LastFetchedAt) == ""
	lim := FeedLimit{
		MaxItems:  s.maxItems(),
		MinItems:  s.minItems(),
		FirstSync: first,
	}
	if first {
		days := s.Opts.BackfillDays
		if days <= 0 {
			days = 60
		}
		lim.NotBefore = time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	}
	return lim
}

func (s *Service) maxItems() int {
	if s.Opts.MaxItems > 0 {
		return s.Opts.MaxItems
	}
	return 25
}

func (s *Service) minItems() int {
	if s.Opts.MinItems > 0 {
		return s.Opts.MinItems
	}
	return 5
}

func (s *Service) getResponse(ctx context.Context, endpoint, etag, modified string) (*http.Response, error) {
	release, err := s.gate().acquire(ctx, requestHost(endpoint))
	if err != nil {
		return nil, err
	}
	defer release()
	client := s.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	var last error
	for attempt := 0; attempt < syncAttempts; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(s.backoffDelay(attempt))
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "elephant/1.0")
		if strings.TrimSpace(etag) != "" {
			req.Header.Set("If-None-Match", etag)
		}
		if strings.TrimSpace(modified) != "" {
			req.Header.Set("If-Modified-Since", modified)
		}
		res, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		if res.StatusCode == http.StatusNotModified {
			return res, nil
		}
		if retryableStatus(res.StatusCode) && attempt < syncAttempts-1 {
			_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
			res.Body.Close()
			last = fmt.Errorf("fetch %s: %s", endpoint, res.Status)
			continue
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			res.Body.Close()
			return nil, fmt.Errorf("fetch %s: %s", endpoint, res.Status)
		}
		return res, nil
	}
	if last == nil {
		last = fmt.Errorf("fetch %s: failed", endpoint)
	}
	return nil, last
}

func (s *Service) backoffDelay(attempt int) time.Duration {
	if s.Backoff != nil {
		return s.Backoff(attempt)
	}
	return defaultBackoff(attempt)
}

func defaultBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	span := 150 * time.Millisecond
	jitter := time.Duration(mathrand.Int63n(int64(span)))
	return time.Duration(attempt)*200*time.Millisecond + jitter
}

func retryableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func (s *Service) gate() *hostGate {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hosts == nil {
		s.hosts = newHostGate(hostSlots)
	}
	return s.hosts
}

type hostGate struct {
	mu    sync.Mutex
	n     int
	slots map[string]chan struct{}
}

func newHostGate(n int) *hostGate {
	return &hostGate{n: n, slots: map[string]chan struct{}{}}
}

func (g *hostGate) acquire(ctx context.Context, host string) (func(), error) {
	if g == nil || g.n <= 0 {
		return func() {}, nil
	}
	if host == "" {
		host = "_"
	}
	g.mu.Lock()
	ch, ok := g.slots[host]
	if !ok {
		ch = make(chan struct{}, g.n)
		g.slots[host] = ch
	}
	g.mu.Unlock()
	select {
	case ch <- struct{}{}:
		return func() { <-ch }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func requestHost(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" {
		return strings.ToLower(endpoint)
	}
	return strings.ToLower(parsed.Host)
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
