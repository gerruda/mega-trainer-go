package telegramapi

import (
	"net/http"
	"net/url"
	"testing"
)

func TestNewHTTPClientWithoutProxy(t *testing.T) {
	t.Parallel()

	client, err := NewHTTPClient("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("client is nil")
	}
	if client.Timeout <= 0 {
		t.Fatal("client timeout must be positive")
	}
}

func TestNewHTTPClientWithHTTPProxy(t *testing.T) {
	t.Parallel()

	client, err := NewHTTPClient("http://127.0.0.1:12334")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", client.Transport)
	}
	if transport.Proxy == nil {
		t.Fatal("proxy function is nil")
	}

	req := &http.Request{URL: &url.URL{Scheme: "https", Host: "api.telegram.org"}}
	proxyURL, err := transport.Proxy(req)
	if err != nil {
		t.Fatalf("proxy function: %v", err)
	}
	if proxyURL == nil || proxyURL.String() != "http://127.0.0.1:12334" {
		t.Fatalf("proxy = %v, want http://127.0.0.1:12334", proxyURL)
	}
}

func TestNewHTTPClientRejectsUnsupportedProxy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		proxyURL string
	}{
		{name: "socks5 is not supported", proxyURL: "socks5://127.0.0.1:1080"},
		{name: "missing host", proxyURL: "http://"},
		{name: "malformed", proxyURL: "://bad"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := NewHTTPClient(tt.proxyURL); err == nil {
				t.Fatalf("proxy URL %q: expected an error", tt.proxyURL)
			}
		})
	}
}

func TestProxyHostHidesCredentials(t *testing.T) {
	t.Parallel()

	got := ProxyHost("http://user:secret@127.0.0.1:12334")
	if got != "127.0.0.1:12334" {
		t.Fatalf("host = %q, want 127.0.0.1:12334", got)
	}

	if got := ProxyHost("not a url"); got != "" {
		t.Fatalf("host = %q, want an empty string for an invalid URL", got)
	}
}
