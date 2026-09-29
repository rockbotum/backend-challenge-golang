package api

import (
	"net/http"
	"testing"
	"time"

	"backendchallengegolang/internal/config"
)

func testTimeouts() config.Timeouts {
	return config.Default().Binance.Timeout
}

func TestNewHTTPClient_Defaults(t *testing.T) {
	t.Parallel()

	client, err := NewHTTPClient(testTimeouts(), "")
	if err != nil {
		t.Fatalf("NewHTTPClient returned %v", err)
	}

	if client.Timeout != 30*time.Second {
		t.Errorf("Timeout = %s, want 30s", client.Timeout)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport is %T, want *http.Transport", client.Transport)
	}

	if transport.ResponseHeaderTimeout != 10*time.Second {
		t.Errorf("ResponseHeaderTimeout = %s, want 10s", transport.ResponseHeaderTimeout)
	}

	if transport.TLSHandshakeTimeout != 5*time.Second {
		t.Errorf("TLSHandshakeTimeout = %s, want 5s", transport.TLSHandshakeTimeout)
	}

	if transport.Proxy == nil {
		t.Error("Proxy is nil, want the environment proxy")
	}
}

func TestNewHTTPClient_Socks5(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		proxy   string
		wantErr bool
	}{
		{name: "host and port", proxy: "127.0.0.1:1080"},
		{name: "url form", proxy: "socks5://127.0.0.1:1080"},
		{name: "missing port", proxy: "127.0.0.1", wantErr: true},
		{name: "missing host", proxy: ":1080", wantErr: true},
		{name: "garbage", proxy: "://", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			client, err := NewHTTPClient(testTimeouts(), test.proxy)

			if test.wantErr {
				if err == nil {
					t.Fatalf("NewHTTPClient(%q) returned nil error", test.proxy)
				}

				return
			}

			if err != nil {
				t.Fatalf("NewHTTPClient(%q) returned %v", test.proxy, err)
			}

			if client == nil {
				t.Fatal("NewHTTPClient returned a nil client")
			}
		})
	}
}
