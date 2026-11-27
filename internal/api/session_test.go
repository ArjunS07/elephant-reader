package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"elephant/internal/db"
)

func newHandler(t *testing.T, opt Options) (http.Handler, *db.Store) {
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
	if opt.HTTPClient == nil {
		opt.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}
	return NewOptions(store, opt), store
}

func sessionRequest(handler http.Handler, method, target, host string, body string, headers map[string]string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, target, reader)
	if host != "" {
		req.Host = host
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestSessionCookieAndBearer(t *testing.T) {
	handler, _ := newHandler(t, Options{Token: "api-token", Passphrase: "correct horse"})

	open := sessionRequest(handler, http.MethodGet, "/v1/session", "127.0.0.1:8080", "", nil)
	if open.Code != http.StatusOK {
		t.Fatalf("session status %d", open.Code)
	}
	var status struct {
		Authenticated bool `json:"authenticated"`
		AuthRequired  bool `json:"authRequired"`
	}
	if err := json.Unmarshal(open.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Authenticated || !status.AuthRequired {
		t.Fatalf("status %+v", status)
	}
	if open.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("wildcard or unexpected CORS header")
	}
	if !strings.Contains(open.Header().Get("Content-Security-Policy"), "default-src 'self'") {
		t.Fatalf("csp %s", open.Header().Get("Content-Security-Policy"))
	}
	if open.Header().Get("X-Frame-Options") != "DENY" || open.Header().Get("X-Content-Type-Options") != "nosniff" || open.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("security headers %+v", open.Header())
	}

	denied := sessionRequest(handler, http.MethodGet, "/v1/feed", "127.0.0.1:8080", "", nil)
	if denied.Code != http.StatusUnauthorized {
		t.Fatalf("feed %d", denied.Code)
	}
	wrongBearer := sessionRequest(handler, http.MethodGet, "/v1/feed", "127.0.0.1:8080", "", map[string]string{
		"Authorization": "Bearer correct horse",
	})
	if wrongBearer.Code != http.StatusUnauthorized {
		t.Fatalf("passphrase bearer %d", wrongBearer.Code)
	}
	bearer := sessionRequest(handler, http.MethodGet, "/v1/feed", "127.0.0.1:8080", "", map[string]string{
		"Authorization": "Bearer api-token",
	})
	if bearer.Code != http.StatusOK {
		t.Fatalf("bearer %d", bearer.Code)
	}

	bad := sessionRequest(handler, http.MethodPost, "/v1/session", "127.0.0.1:8080", `{"passphrase":"nope"}`, nil)
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("bad passphrase %d", bad.Code)
	}
	tokenAsPass := sessionRequest(handler, http.MethodPost, "/v1/session", "127.0.0.1:8080", `{"token":"api-token"}`, nil)
	if tokenAsPass.Code != http.StatusUnauthorized {
		t.Fatalf("api token is not the passphrase %d", tokenAsPass.Code)
	}

	login := sessionRequest(handler, http.MethodPost, "/v1/session", "127.0.0.1:8080", `{"passphrase":"correct horse"}`, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("login %d %s", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != sessionCookie || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Secure || cookie.MaxAge != int(sessionTTL.Seconds()) {
		t.Fatalf("cookie %+v", cookie)
	}

	authed := httptest.NewRequest(http.MethodGet, "/v1/feed", nil)
	authed.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, authed)
	if rec.Code != http.StatusOK {
		t.Fatalf("cookie feed %d", rec.Code)
	}

	tampered := *cookie
	tampered.Value = tampered.Value[:len(tampered.Value)-1] + "0"
	if strings.HasSuffix(cookie.Value, "0") {
		tampered.Value = tampered.Value[:len(tampered.Value)-1] + "1"
	}
	badCookie := httptest.NewRequest(http.MethodGet, "/v1/feed", nil)
	badCookie.AddCookie(&tampered)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, badCookie)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("tampered cookie %d", rec.Code)
	}

	logout := httptest.NewRequest(http.MethodDelete, "/v1/session", nil)
	logout.AddCookie(cookie)
	logout.Host = "127.0.0.1:8080"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, logout)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout %d", rec.Code)
	}
	cleared := rec.Result().Cookies()
	if len(cleared) != 1 || cleared[0].MaxAge >= 0 {
		t.Fatalf("clear cookie %+v", cleared)
	}
}

func TestSessionCookieSecureFlag(t *testing.T) {
	handler, _ := newHandler(t, Options{Passphrase: "phrase"})
	login := func(host, proto string) *http.Cookie {
		t.Helper()
		rec := sessionRequest(handler, http.MethodPost, "/v1/session", host, `{"token":"phrase"}`, map[string]string{
			"X-Forwarded-Proto": proto,
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("login %s %d", host, rec.Code)
		}
		cookies := rec.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("cookies %d", len(cookies))
		}
		return cookies[0]
	}
	if login("elephant.example", "").Secure != true {
		t.Fatal("non-localhost cookie was not Secure")
	}
	if login("localhost:8080", "").Secure {
		t.Fatal("localhost cookie was Secure")
	}
	if !login("localhost", "https").Secure {
		t.Fatal("forwarded https cookie was not Secure")
	}
}

func TestPassphraseFallsBackToToken(t *testing.T) {
	handler, _ := newHandler(t, Options{Token: "api-token"})
	missing := sessionRequest(handler, http.MethodPost, "/v1/session", "127.0.0.1", `{"passphrase":"other"}`, nil)
	if missing.Code != http.StatusUnauthorized {
		t.Fatalf("wrong %d", missing.Code)
	}
	ok := sessionRequest(handler, http.MethodPost, "/v1/session", "127.0.0.1", `{"token":"api-token"}`, nil)
	if ok.Code != http.StatusOK {
		t.Fatalf("fallback %d %s", ok.Code, ok.Body.String())
	}
}

func TestDevModeStaysOpen(t *testing.T) {
	handler, _ := newHandler(t, Options{})
	rec := sessionRequest(handler, http.MethodGet, "/v1/session", "127.0.0.1", "", nil)
	var status struct {
		Authenticated bool `json:"authenticated"`
		AuthRequired  bool `json:"authRequired"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if !status.Authenticated || status.AuthRequired {
		t.Fatalf("status %+v", status)
	}
	feed := sessionRequest(handler, http.MethodGet, "/v1/feed", "127.0.0.1", "", nil)
	if feed.Code != http.StatusOK {
		t.Fatalf("feed %d", feed.Code)
	}
}

func TestSessionRateLimit(t *testing.T) {
	handler, _ := newHandler(t, Options{Passphrase: "phrase"})
	for i := 0; i < sessionAttemptLimit; i++ {
		rec := sessionRequest(handler, http.MethodPost, "/v1/session", "203.0.113.8", `{"passphrase":"nope"}`, map[string]string{
			"X-Forwarded-For": "198.51.100." + string(rune('1'+i)),
		})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status %d", i, rec.Code)
		}
	}
	blocked := sessionRequest(handler, http.MethodPost, "/v1/session", "203.0.113.8", `{"passphrase":"phrase"}`, nil)
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("limit %d", blocked.Code)
	}

	proxied, _ := newHandler(t, Options{Passphrase: "phrase"})
	for i := 0; i < sessionAttemptLimit; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/session", strings.NewReader(`{"passphrase":"nope"}`))
		req.RemoteAddr = "10.0.0.4:443"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Fly-Client-IP", "203.0.113.10")
		rec := httptest.NewRecorder()
		proxied.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("proxy attempt %d status %d", i, rec.Code)
		}
	}
	other := httptest.NewRequest(http.MethodPost, "/v1/session", strings.NewReader(`{"passphrase":"phrase"}`))
	other.RemoteAddr = "10.0.0.4:443"
	other.Header.Set("Content-Type", "application/json")
	other.Header.Set("Fly-Client-IP", "203.0.113.11")
	rec := httptest.NewRecorder()
	proxied.ServeHTTP(rec, other)
	if rec.Code != http.StatusOK {
		t.Fatalf("other client %d %s", rec.Code, rec.Body.String())
	}
}

func TestExpiredSessionCookie(t *testing.T) {
	old := sessionTTL
	sessionTTL = -time.Minute
	t.Cleanup(func() { sessionTTL = old })
	handler, _ := newHandler(t, Options{Passphrase: "phrase"})
	login := sessionRequest(handler, http.MethodPost, "/v1/session", "127.0.0.1", `{"passphrase":"phrase"}`, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("login %d", login.Code)
	}
	cookie := login.Result().Cookies()[0]
	req := httptest.NewRequest(http.MethodGet, "/v1/feed", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expired %d", rec.Code)
	}
}

func TestCORSOrigin(t *testing.T) {
	handler, _ := newHandler(t, Options{CORSOrigin: "https://reader.example"})
	rec := sessionRequest(handler, http.MethodGet, "/v1/health", "127.0.0.1", "", nil)
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://reader.example" {
		t.Fatalf("origin %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("credentials missing")
	}
	star, _ := newHandler(t, Options{CORSOrigin: "*"})
	rec = sessionRequest(star, http.MethodOptions, "/v1/feed", "127.0.0.1", "", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("options %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("wildcard origin was reflected")
	}
}

func TestStaticCacheAndSecurityHeaders(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>elephant</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "index-abc123.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler, _ := newHandler(t, Options{})
	static := WithStatic(handler, dir)

	index := httptest.NewRecorder()
	static.ServeHTTP(index, httptest.NewRequest(http.MethodGet, "/", nil))
	if index.Code != http.StatusOK || index.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("index %d %s", index.Code, index.Header().Get("Cache-Control"))
	}
	if index.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("index missing security header")
	}

	asset := httptest.NewRecorder()
	static.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/assets/index-abc123.js", nil))
	if asset.Code != http.StatusOK || asset.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("asset %d %s body %s", asset.Code, asset.Header().Get("Cache-Control"), asset.Body.String())
	}

	fallback := httptest.NewRecorder()
	static.ServeHTTP(fallback, httptest.NewRequest(http.MethodGet, "/feed", nil))
	if fallback.Code != http.StatusOK || fallback.Header().Get("Cache-Control") != "no-cache" || !strings.Contains(fallback.Body.String(), "elephant") {
		t.Fatalf("fallback %d %s", fallback.Code, fallback.Header().Get("Cache-Control"))
	}
}
