package ingest

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBlockedAddresses(t *testing.T) {
	blocked := []string{
		"127.0.0.1",
		"127.0.0.2",
		"10.1.2.3",
		"172.16.0.1",
		"172.31.255.255",
		"192.168.1.1",
		"169.254.169.254",
		"0.0.0.0",
		"::1",
		"fc00::1",
		"fd00::1",
		"fdaa:0:1::1",
		"fe80::1",
		"::ffff:127.0.0.1",
		"::ffff:10.0.0.1",
		"::ffff:169.254.169.254",
	}
	for _, raw := range blocked {
		if !blockedIP(net.ParseIP(raw)) {
			t.Fatalf("%s should be blocked", raw)
		}
	}
	allowed := []string{"8.8.8.8", "1.1.1.1", "172.15.5.5", "172.32.0.1", "2001:4860:4860::8888"}
	for _, raw := range allowed {
		if blockedIP(net.ParseIP(raw)) {
			t.Fatalf("%s should be allowed", raw)
		}
	}
}

func TestBlockedHosts(t *testing.T) {
	for _, host := range []string{"localhost", "Foo.INTERNAL", "printer.local", "db.internal.", "metadata.google.internal", ""} {
		if !blockedHost(host) {
			t.Fatalf("%q should be blocked", host)
		}
	}
	for _, host := range []string{"example.com", "local.com", "internal.example.com", "news.ycombinator.com"} {
		if blockedHost(host) {
			t.Fatalf("%q should be allowed", host)
		}
	}
}

func TestSafeClientBlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if _, err := SafeClient().Get(srv.URL); err == nil {
		t.Fatal("safe client reached loopback")
	}
	plain := &http.Client{Timeout: 5 * time.Second}
	res, err := plain.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("plain client %d", res.StatusCode)
	}
}

func TestSafeDialBlocksBeforeConnect(t *testing.T) {
	dialer := &net.Dialer{Timeout: time.Second}
	for _, addr := range []string{"169.254.169.254:80", "metadata.google.internal:80", "127.0.0.1:1", "[fdaa::1]:80"} {
		conn, err := safeDial(context.Background(), dialer, "tcp", addr)
		if err == nil {
			conn.Close()
			t.Fatalf("dialed %s", addr)
		}
	}
}

func TestRedirectsAreRevalidated(t *testing.T) {
	prev, err := http.NewRequest(http.MethodGet, "https://example.com/feed", nil)
	if err != nil {
		t.Fatal(err)
	}
	cases := []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://127.0.0.1/",
		"http://10.1.2.3/admin",
		"http://files.local/feed",
		"http://db.internal/feed",
		"javascript:alert(1)",
		"file:///etc/passwd",
	}
	for _, raw := range cases {
		next, err := http.NewRequest(http.MethodGet, raw, nil)
		if err != nil {
			t.Fatalf("request %s: %v", raw, err)
		}
		if err := safeCheckRedirect(next, []*http.Request{prev}); err == nil {
			t.Fatalf("allowed redirect %s", raw)
		}
	}
	ok, err := http.NewRequest(http.MethodGet, "https://example.com/feed.xml", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := safeCheckRedirect(ok, []*http.Request{prev}); err != nil {
		t.Fatal(err)
	}
	via := make([]*http.Request, 10)
	for i := range via {
		via[i] = prev
	}
	if err := safeCheckRedirect(ok, via); err == nil {
		t.Fatal("redirect limit not enforced")
	}
}
