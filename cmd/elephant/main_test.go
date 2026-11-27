package main

import (
	"net/http"
	"testing"
	"time"
)

func TestHTTPServerTimeouts(t *testing.T) {
	server := newHTTPServer(":8080", http.NewServeMux())
	if server.ReadHeaderTimeout != 10*time.Second || server.IdleTimeout != 60*time.Second || server.WriteTimeout != 60*time.Second {
		t.Fatalf("timeouts header %s idle %s write %s", server.ReadHeaderTimeout, server.IdleTimeout, server.WriteTimeout)
	}
}
