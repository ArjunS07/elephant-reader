package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"elephant/internal/db"
	"elephant/internal/ingest"
	"elephant/internal/model"
)

type Server struct {
	Store      *db.Store
	Ingest     *ingest.Service
	Token      string
	Passphrase string
	CORSOrigin string
	macKey     []byte
	limiter    *rateLimiter
}

// Options configures API authentication and the feed client.
// HTTPClient replaces the SSRF-safe feed client. Tests set it so they can
// fetch from httptest.NewServer, which listens on loopback.
type Options struct {
	Token      string
	Passphrase string
	CORSOrigin string
	HTTPClient *http.Client
	// Service is the feed syncer the handler and startup catch-up share.
	// Nil creates one. HTTPClient, when set, replaces that service's client
	// so tests can fetch loopback URLs the SSRF dialer refuses.
	Service *ingest.Service
}

func New(store *db.Store, token string) http.Handler {
	return NewOptions(store, Options{Token: token})
}

func NewOptions(store *db.Store, opt Options) http.Handler {
	svc := opt.Service
	if svc == nil {
		svc = ingest.New(store)
	}
	if opt.HTTPClient != nil {
		svc.HTTP = opt.HTTPClient
	}
	s := &Server{
		Store:      store,
		Ingest:     svc,
		Token:      opt.Token,
		Passphrase: opt.Passphrase,
		CORSOrigin: strings.TrimSpace(opt.CORSOrigin),
		limiter:    newRateLimiter(sessionAttemptLimit, sessionAttemptWindow),
	}
	if secret := s.loginSecret(); secret != "" {
		s.macKey = sessionMACKey(secret)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", s.health)
	mux.HandleFunc("GET /v1/session", s.getSession)
	mux.HandleFunc("POST /v1/session", s.postSession)
	mux.HandleFunc("DELETE /v1/session", s.deleteSession)
	mux.HandleFunc("POST /v1/ingest", s.ingest)
	mux.HandleFunc("GET /v1/feed", s.feed)
	mux.HandleFunc("GET /v1/calendar", s.calendar)
	mux.HandleFunc("GET /v1/later", s.later)
	mux.HandleFunc("GET /v1/read", s.read)
	mux.HandleFunc("GET /v1/items/{id}", s.item)
	mux.HandleFunc("PATCH /v1/items/{id}/state", s.patchState)
	mux.HandleFunc("POST /v1/items/{id}/highlights", s.addHighlight)
	mux.HandleFunc("PATCH /v1/highlights/{id}", s.patchHighlight)
	mux.HandleFunc("DELETE /v1/highlights/{id}", s.deleteHighlight)
	mux.HandleFunc("GET /v1/sources", s.listSources)
	mux.HandleFunc("POST /v1/sources/import", s.importSources)
	mux.HandleFunc("POST /v1/sources", s.createSource)
	mux.HandleFunc("PATCH /v1/sources/{id}", s.patchSource)
	mux.HandleFunc("DELETE /v1/sources/{id}", s.deleteSource)
	mux.HandleFunc("POST /v1/sources/{id}/sync", s.syncOne)
	mux.HandleFunc("GET /v1/sync/status", s.syncStatus)
	mux.HandleFunc("POST /v1/sync", s.syncAll)
	mux.HandleFunc("POST /v1/inbound/email", s.inbound)
	mux.HandleFunc("GET /v1/preferences", s.getPrefs)
	mux.HandleFunc("PATCH /v1/preferences", s.patchPrefs)
	return s.auth(mux)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) ingest(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	items, prefix, err := decodeIngest(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	results, err := s.Ingest.Ingest(r.Context(), items)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	out := make([]model.Item, 0, len(results))
	for _, result := range results {
		out = append(out, result.Item)
	}
	resp := map[string]any{"items": out}
	if prefix != "" {
		keep := make([]string, 0, len(items))
		for _, item := range items {
			if strings.HasPrefix(item.ExternalID, prefix) {
				keep = append(keep, item.ExternalID)
			}
		}
		deleted, err := s.Store.DeleteByExternalIDPrefixExcept(prefix, keep)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		resp["deleted"] = deleted
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) feed(w http.ResponseWriter, r *http.Request) {
	folder := r.URL.Query().Get("folder")
	sourceID := r.URL.Query().Get("sourceId")
	day := strings.TrimSpace(r.URL.Query().Get("day"))
	counts, err := s.Store.Counts(folder, sourceID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	earliest, latest, days := db.Days(counts)
	if day == "" {
		day = latest
	} else if !validDay(day) {
		writeError(w, http.StatusBadRequest, "day must be YYYY-MM-DD")
		return
	}
	items := []model.ItemRow{}
	if day != "" {
		items, err = s.Store.DayItems(day, folder, sourceID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
	}
	prev, next := db.NeighborDays(days, day)
	var earliestPtr, latestPtr *string
	if earliest != "" {
		earliestPtr = &earliest
		latestPtr = &latest
	}
	if counts == nil {
		counts = map[string]int{}
	}
	writeJSON(w, http.StatusOK, model.Feed{
		Day:      day,
		PrevDay:  prev,
		NextDay:  next,
		Earliest: earliestPtr,
		Latest:   latestPtr,
		Counts:   counts,
		Items:    items,
	})
}

func (s *Server) calendar(w http.ResponseWriter, r *http.Request) {
	counts, err := s.Store.Counts(r.URL.Query().Get("folder"), r.URL.Query().Get("sourceId"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if counts == nil {
		counts = map[string]int{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"counts": counts})
}

func (s *Server) later(w http.ResponseWriter, r *http.Request) {
	items, err := s.Store.Later()
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) read(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := atoiDefault(r.URL.Query().Get("limit"), 50)
	offset := atoiDefault(r.URL.Query().Get("offset"), 0)
	match := ""
	if q != "" {
		match = db.MatchQuery(q)
		if match == "" {
			writeJSON(w, http.StatusOK, map[string]any{"items": []model.ItemRow{}, "limit": limit, "offset": offset})
			return
		}
	}
	var items []model.ItemRow
	var err error
	if match != "" {
		items, err = s.Store.Search(match, limit, offset)
	} else {
		items, err = s.Store.Read("", limit, offset)
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset})
}

func (s *Server) item(w http.ResponseWriter, r *http.Request) {
	item, source, state, highlights, err := s.Store.GetItem(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"item":       item,
		"source":     source,
		"state":      state,
		"highlights": highlights,
	})
}

func (s *Server) patchState(w http.ResponseWriter, r *http.Request) {
	var body map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	patch := db.StatePatch{}
	if raw, ok := body["inLater"]; ok {
		var v bool
		if err := json.Unmarshal(raw, &v); err != nil {
			writeError(w, http.StatusBadRequest, "inLater must be a boolean")
			return
		}
		patch.InLater = &v
	}
	if raw, ok := body["readAt"]; ok {
		patch.ReadAtSet = true
		if string(raw) != "null" {
			var v string
			if err := json.Unmarshal(raw, &v); err != nil {
				writeError(w, http.StatusBadRequest, "readAt must be a string or null")
				return
			}
			patch.ReadAt = &v
		}
	}
	if raw, ok := body["readProgress"]; ok {
		var v float64
		if err := json.Unmarshal(raw, &v); err != nil {
			writeError(w, http.StatusBadRequest, "readProgress must be a number")
			return
		}
		patch.ProgressSet = true
		patch.ReadProgress = v
	}
	state, err := s.Store.PatchState(r.PathValue("id"), patch)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) addHighlight(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID     string `json:"id"`
		Quote  string `json:"quote"`
		Prefix string `json:"prefix"`
		Suffix string `json:"suffix"`
		Note   string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(body.Quote) == "" {
		writeError(w, http.StatusBadRequest, "quote is required")
		return
	}
	if body.ID == "" {
		body.ID = "hl_" + time.Now().UTC().Format("20060102150405.000000000")
	}
	highlight, err := s.Store.AddHighlight(model.Highlight{
		ID:     body.ID,
		ItemID: r.PathValue("id"),
		Quote:  body.Quote,
		Prefix: body.Prefix,
		Suffix: body.Suffix,
		Note:   body.Note,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, highlight)
}

func (s *Server) patchHighlight(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Note string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	highlight, err := s.Store.PatchHighlight(r.PathValue("id"), body.Note)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, highlight)
}

func (s *Server) deleteHighlight(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteHighlight(r.PathValue("id")); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) importSources(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	filename := ""
	var data []byte
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "invalid upload")
			return
		}
		if value := r.FormValue("format"); value != "" {
			format = value
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "file is required")
			return
		}
		defer file.Close()
		filename = header.Filename
		data, err = io.ReadAll(io.LimitReader(file, 8<<20))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid upload")
			return
		}
	} else {
		var err error
		data, err = io.ReadAll(io.LimitReader(r.Body, 8<<20))
		if err != nil || len(bytesTrimSpace(data)) == 0 {
			writeError(w, http.StatusBadRequest, "empty body")
			return
		}
	}
	result, err := ingest.ImportBytes(r.Context(), s.Store, data, ingest.ImportOptions{Filename: filename, Format: format})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func bytesTrimSpace(data []byte) []byte {
	return []byte(strings.TrimSpace(string(data)))
}

func (s *Server) listSources(w http.ResponseWriter, _ *http.Request) {
	sources, err := s.Store.ListSources()
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": sources})
}

func (s *Server) createSource(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID              string         `json:"id"`
		Name            string         `json:"name"`
		SourceType      string         `json:"sourceType"`
		Folder          string         `json:"folder"`
		DefaultOpenMode string         `json:"defaultOpenMode"`
		DefaultQueue    string         `json:"defaultQueue"`
		FeedURL         string         `json:"feedUrl"`
		EmailAlias      string         `json:"emailAlias"`
		Config          map[string]any `json:"config"`
		Enabled         *bool          `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(body.Name) == "" || !model.ValidSourceType(body.SourceType) || strings.TrimSpace(body.Folder) == "" {
		writeError(w, http.StatusBadRequest, "name, sourceType, and folder are required")
		return
	}
	if body.DefaultOpenMode == "" {
		body.DefaultOpenMode = "reader"
	}
	if body.DefaultQueue == "" {
		body.DefaultQueue = "feed"
	}
	if !model.ValidOpenMode(body.DefaultOpenMode) || !model.ValidQueue(body.DefaultQueue) {
		writeError(w, http.StatusBadRequest, "invalid open mode or queue")
		return
	}
	if body.SourceType == "newsletter" && strings.TrimSpace(body.EmailAlias) == "" {
		body.EmailAlias = ingest.NewsletterAlias(body.Name, func(alias string) bool {
			taken, err := s.Store.AliasTaken(alias)
			return err == nil && taken
		})
	}
	if body.ID == "" {
		body.ID = "src_" + strings.ReplaceAll(ingest.Slug(body.Name), "-", "")
		if body.ID == "src_" {
			body.ID = "src_manual"
		}
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	source, err := s.Store.InsertSource(model.Source{
		ID:              body.ID,
		Name:            strings.TrimSpace(body.Name),
		SourceType:      body.SourceType,
		Folder:          body.Folder,
		DefaultOpenMode: body.DefaultOpenMode,
		DefaultQueue:    body.DefaultQueue,
		FeedURL:         body.FeedURL,
		EmailAlias:      body.EmailAlias,
		Config:          body.Config,
		Enabled:         enabled,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, source)
}

func (s *Server) patchSource(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name            *string        `json:"name"`
		Folder          *string        `json:"folder"`
		DefaultOpenMode *string        `json:"defaultOpenMode"`
		DefaultQueue    *string        `json:"defaultQueue"`
		FeedURL         *string        `json:"feedUrl"`
		EmailAlias      *string        `json:"emailAlias"`
		Enabled         *bool          `json:"enabled"`
		Config          map[string]any `json:"config"`
		GenerateAlias   bool           `json:"generateAlias"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.DefaultOpenMode != nil && !model.ValidOpenMode(*body.DefaultOpenMode) {
		writeError(w, http.StatusBadRequest, "invalid open mode")
		return
	}
	if body.DefaultQueue != nil && !model.ValidQueue(*body.DefaultQueue) {
		writeError(w, http.StatusBadRequest, "invalid queue")
		return
	}
	patch := db.SourcePatch{
		Name:            body.Name,
		Folder:          body.Folder,
		DefaultOpenMode: body.DefaultOpenMode,
		DefaultQueue:    body.DefaultQueue,
		FeedURL:         body.FeedURL,
		EmailAlias:      body.EmailAlias,
		Enabled:         body.Enabled,
		Config:          body.Config,
		HasConfig:       body.Config != nil,
	}
	if body.GenerateAlias {
		current, err := s.Store.GetSource(r.PathValue("id"))
		if err != nil {
			writeStoreError(w, err)
			return
		}
		if current.SourceType != "newsletter" {
			writeError(w, http.StatusBadRequest, "alias is for newsletter sources")
			return
		}
		alias := ingest.NewsletterAlias(current.Name, func(alias string) bool {
			if strings.EqualFold(alias, current.EmailAlias) {
				return false
			}
			taken, err := s.Store.AliasTaken(alias)
			return err == nil && taken
		})
		patch.EmailAlias = &alias
	}
	source, err := s.Store.PatchSource(r.PathValue("id"), patch)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, source)
}

func (s *Server) deleteSource(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteSource(r.PathValue("id")); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) syncOne(w http.ResponseWriter, r *http.Request) {
	result, err := s.Ingest.Sync(r.Context(), r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) syncAll(w http.ResponseWriter, _ *http.Request) {
	// Detach from the request so closing the tab does not abort the job.
	writeJSON(w, http.StatusAccepted, s.Ingest.StartSync())
}

func (s *Server) syncStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.Ingest.Status())
}

func (s *Server) inbound(w http.ResponseWriter, r *http.Request) {
	var body struct {
		To         string `json:"to"`
		From       string `json:"from"`
		Subject    string `json:"subject"`
		HTML       string `json:"html"`
		Text       string `json:"text"`
		ReceivedAt string `json:"receivedAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(body.To) == "" {
		writeError(w, http.StatusBadRequest, "to is required")
		return
	}
	result, err := s.Ingest.InboundEmail(body.To, body.From, body.Subject, body.HTML, body.Text, body.ReceivedAt)
	if err != nil {
		if strings.Contains(err.Error(), "no newsletter source") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": result.Item, "created": result.Created})
}

func (s *Server) getPrefs(w http.ResponseWriter, _ *http.Request) {
	prefs, err := s.Store.GetPreferences()
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, prefs)
}

func (s *Server) patchPrefs(w http.ResponseWriter, r *http.Request) {
	current, err := s.Store.GetPreferences()
	if err != nil {
		writeStoreError(w, err)
		return
	}
	var body struct {
		Theme       *string  `json:"theme"`
		FontFamily  *string  `json:"fontFamily"`
		FontSize    *float64 `json:"fontSize"`
		LineHeight  *float64 `json:"lineHeight"`
		ColumnWidth *string  `json:"columnWidth"`
		Align       *string  `json:"align"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Theme != nil {
		if !model.ValidTheme(*body.Theme) {
			writeError(w, http.StatusBadRequest, "invalid theme")
			return
		}
		current.Theme = *body.Theme
	}
	if body.FontFamily != nil {
		if !model.ValidFont(*body.FontFamily) {
			writeError(w, http.StatusBadRequest, "invalid font")
			return
		}
		current.FontFamily = *body.FontFamily
	}
	if body.FontSize != nil {
		current.FontSize = clamp(*body.FontSize, 15, 20)
	}
	if body.LineHeight != nil {
		current.LineHeight = roundStep(clamp(*body.LineHeight, 1.5, 1.85), 0.05)
	}
	if body.ColumnWidth != nil {
		if !model.ValidWidth(*body.ColumnWidth) {
			writeError(w, http.StatusBadRequest, "invalid width")
			return
		}
		current.ColumnWidth = *body.ColumnWidth
	}
	if body.Align != nil {
		if !model.ValidAlign(*body.Align) {
			writeError(w, http.StatusBadRequest, "invalid align")
			return
		}
		current.Align = *body.Align
	}
	saved, err := s.Store.SavePreferences(current)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func WithStatic(api http.Handler, dir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w)
		if strings.HasPrefix(r.URL.Path, "/v1/") || r.URL.Path == "/v1" {
			api.ServeHTTP(w, r)
			return
		}
		if dir == "" || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
			api.ServeHTTP(w, r)
			return
		}
		clean := filepath.Clean(r.URL.Path)
		if clean == "." || strings.Contains(clean, "..") {
			clean = "/"
		}
		target := filepath.Join(dir, clean)
		info, err := os.Stat(target)
		if err == nil && !info.IsDir() {
			setStaticCache(w, r.URL.Path, target)
			http.ServeFile(w, r, target)
			return
		}
		index := filepath.Join(dir, "index.html")
		if _, err := os.Stat(index); err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, index)
	})
}

func setStaticCache(w http.ResponseWriter, requestPath, target string) {
	if filepath.Base(target) == "index.html" {
		w.Header().Set("Cache-Control", "no-cache")
		return
	}
	if strings.HasPrefix(requestPath, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
}

func decodeIngest(body []byte) ([]model.IngestItem, string, error) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil, "", errors.New("empty body")
	}
	if strings.HasPrefix(trimmed, "[") {
		var items []model.IngestItem
		if err := json.Unmarshal(body, &items); err != nil {
			return nil, "", errors.New("invalid json")
		}
		return items, "", nil
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(body, &probe); err != nil {
		return nil, "", errors.New("invalid json")
	}
	prefix, err := decodeReplaceScope(probe)
	if err != nil {
		return nil, "", err
	}
	if raw, ok := probe["items"]; ok {
		if _, single := probe["canonicalUrl"]; !single {
			var items []model.IngestItem
			if err := json.Unmarshal(raw, &items); err != nil {
				return nil, "", errors.New("invalid json")
			}
			return items, prefix, nil
		}
	}
	if prefix != "" {
		return nil, "", errors.New("replaceScope requires an items array")
	}
	var item model.IngestItem
	if err := json.Unmarshal(body, &item); err != nil {
		return nil, "", errors.New("invalid json")
	}
	return []model.IngestItem{item}, "", nil
}

func decodeReplaceScope(probe map[string]json.RawMessage) (string, error) {
	raw, ok := probe["replaceScope"]
	if !ok || strings.TrimSpace(string(raw)) == "" || string(raw) == "null" {
		return "", nil
	}
	var scope struct {
		ExternalIDPrefix string `json:"externalIdPrefix"`
	}
	if err := json.Unmarshal(raw, &scope); err != nil {
		return "", errors.New("invalid replaceScope")
	}
	prefix := strings.TrimSpace(scope.ExternalIDPrefix)
	if !db.ValidDigestReplacePrefix(prefix) {
		return "", db.ErrInvalidReplaceScope
	}
	return prefix, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}

func validDay(day string) bool {
	if len(day) != 10 {
		return false
	}
	_, err := time.Parse("2006-01-02", day)
	return err == nil
}

func atoiDefault(v string, fallback int) int {
	if v == "" {
		return fallback
	}
	n := 0
	sign := 1
	for _, r := range v {
		if r == '-' && n == 0 {
			sign = -1
			continue
		}
		if r < '0' || r > '9' {
			return fallback
		}
		n = n*10 + int(r-'0')
	}
	return n * sign
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func roundStep(v, step float64) float64 {
	if step == 0 {
		return v
	}
	n := float64(int((v/step)+0.5)) * step
	return n
}
