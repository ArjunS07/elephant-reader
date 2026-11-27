package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const sessionCookie = "elephant_session"

var (
	sessionTTL           = 30 * 24 * time.Hour
	sessionAttemptLimit  = 10
	sessionAttemptWindow = 5 * time.Minute
)

func (s *Server) loginSecret() string {
	if s.Passphrase != "" {
		return s.Passphrase
	}
	return s.Token
}

func (s *Server) authRequired() bool {
	return s.Token != "" || s.Passphrase != ""
}

func sessionMACKey(secret string) []byte {
	sum := sha256.Sum256([]byte("elephant-session-v1\n" + secret))
	return sum[:]
}

// secretEqual compares two secrets without leaking their contents through an
// early length check. SHA-256 both sides so ConstantTimeCompare always sees
// equal-length inputs.
func secretEqual(got, want string) bool {
	if want == "" {
		return false
	}
	gotSum := sha256.Sum256([]byte(got))
	wantSum := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(gotSum[:], wantSum[:]) == 1
}

func bearerToken(r *http.Request) (string, bool) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	if token == "" || strings.Contains(token, " ") {
		return "", false
	}
	return token, true
}

func (s *Server) authorized(r *http.Request) bool {
	if !s.authRequired() {
		return true
	}
	if token, ok := bearerToken(r); ok && s.Token != "" && secretEqual(token, s.Token) {
		return true
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	return s.validSession(cookie.Value)
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w)
		s.applyCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if publicRoute(r) || s.authorized(r) {
			next.ServeHTTP(w, r)
			return
		}
		writeError(w, http.StatusUnauthorized, "unauthorized")
	})
}

func publicRoute(r *http.Request) bool {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/health":
		return true
	case r.URL.Path == "/v1/session" && (r.Method == http.MethodGet || r.Method == http.MethodPost || r.Method == http.MethodDelete):
		return true
	default:
		return false
	}
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' https: data:; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

func (s *Server) applyCORS(w http.ResponseWriter, r *http.Request) {
	origin := strings.TrimSpace(s.CORSOrigin)
	if origin == "" || origin == "*" || strings.ContainsAny(origin, " \t\r\n") {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Add("Vary", "Origin")
}

func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
	required := s.authRequired()
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": !required || s.authorized(r),
		"authRequired":  required,
	})
}

func (s *Server) postSession(w http.ResponseWriter, r *http.Request) {
	if s.limiter != nil && !s.limiter.allow(clientKey(r)) {
		writeError(w, http.StatusTooManyRequests, "too many attempts")
		return
	}
	if !s.authRequired() {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "authRequired": false})
		return
	}
	var body struct {
		Passphrase string `json:"passphrase"`
		Token      string `json:"token"`
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<16))
	if err := dec.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	candidate := body.Passphrase
	if candidate == "" {
		candidate = body.Token
	}
	if candidate == "" || !secretEqual(candidate, s.loginSecret()) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	s.setSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "authRequired": true})
}

func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request) {
	exp := time.Now().Add(sessionTTL)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    s.sealSession(exp),
		Path:     "/",
		Expires:  exp.UTC(),
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   cookieSecure(r),
		SameSite: http.SameSiteStrictMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cookieSecure(r),
		SameSite: http.SameSiteStrictMode,
	})
}

func (s *Server) sealSession(expiry time.Time) string {
	payload := strconv.FormatInt(expiry.Unix(), 10)
	mac := hmac.New(sha256.New, s.macKey)
	_, _ = mac.Write([]byte(payload))
	return payload + "." + hex.EncodeToString(mac.Sum(nil))
}

func (s *Server) validSession(value string) bool {
	if len(s.macKey) == 0 {
		return false
	}
	payload, sig, ok := strings.Cut(value, ".")
	if !ok || payload == "" || sig == "" {
		return false
	}
	mac := hmac.New(sha256.New, s.macKey)
	_, _ = mac.Write([]byte(payload))
	expected := mac.Sum(nil)
	got, err := hex.DecodeString(sig)
	if err != nil || len(got) != len(expected) {
		got = make([]byte, len(expected))
	}
	if subtle.ConstantTimeCompare(got, expected) != 1 {
		return false
	}
	exp, err := strconv.ParseInt(payload, 10, 64)
	if err != nil {
		return false
	}
	expires := time.Unix(exp, 0)
	now := time.Now()
	if !now.Before(expires) {
		return false
	}
	if expires.After(now.Add(sessionTTL + time.Minute)) {
		return false
	}
	return true
}

func cookieSecure(r *http.Request) bool {
	if r.TLS != nil || forwardedHTTPS(r) {
		return true
	}
	host := requestHost(r)
	if host == "" || localHost(host) {
		return false
	}
	return true
}

func forwardedHTTPS(r *http.Request) bool {
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		return false
	}
	first := strings.TrimSpace(strings.Split(proto, ",")[0])
	return strings.EqualFold(first, "https")
}

func requestHost(r *http.Request) string {
	host := r.Host
	if host == "" {
		host = r.URL.Host
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	return strings.ToLower(strings.TrimSuffix(host, "."))
}

func localHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

// clientKey is the address the login limiter counts. A public peer is used as
// itself so a caller cannot rotate X-Forwarded-For. Fly's proxy address is
// private, and Fly-Client-IP is the client address the proxy sets.
func clientKey(r *http.Request) string {
	remote := stripPort(r.RemoteAddr)
	if ip := net.ParseIP(remote); ip != nil && trustedProxy(ip) {
		if fly := headerIP(r.Header.Get("Fly-Client-IP")); fly != "" {
			return fly
		}
		if xff := lastForwardedIP(r.Header.Get("X-Forwarded-For")); xff != "" {
			return xff
		}
	}
	if remote == "" {
		return "unknown"
	}
	return remote
}

func trustedProxy(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

func stripPort(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return strings.Trim(addr, "[]")
}

func headerIP(value string) string {
	value = strings.TrimSpace(strings.Split(value, ",")[0])
	if net.ParseIP(value) == nil {
		return ""
	}
	return value
}

func lastForwardedIP(value string) string {
	parts := strings.Split(value, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(parts[i])
		if net.ParseIP(candidate) != nil {
			return candidate
		}
	}
	return ""
}

type rateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string][]time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:  limit,
		window: window,
		hits:   map[string][]time.Time{},
	}
}

func (l *rateLimiter) allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-l.window)
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.hits) > 4096 {
		for existing, stamps := range l.hits {
			if len(stamps) == 0 || !stamps[len(stamps)-1].After(cutoff) {
				delete(l.hits, existing)
			}
		}
	}
	stamps := l.hits[key]
	kept := make([]time.Time, 0, len(stamps)+1)
	for _, stamp := range stamps {
		if stamp.After(cutoff) {
			kept = append(kept, stamp)
		}
	}
	if len(kept) >= l.limit {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}
