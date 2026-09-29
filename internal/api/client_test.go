package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backendchallengegolang/internal/config"
)

func testProperties(baseURL string) config.BinanceAPIProperties {
	properties := config.Default().Binance
	properties.BaseURL = baseURL
	properties.MaxMemorySize = config.Megabyte

	return properties
}

func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return NewClient(testProperties(server.URL), server.Client())
}

const exchangeInfoPayload = `{
	"rateLimits": [
		{"rateLimitType":"REQUEST_WEIGHT","interval":"MINUTE","intervalNum":1,"limit":6000}
	],
	"symbols": [
		{"symbol":"BTCUSDT","baseAsset":"BTC","quoteAsset":"USDT","status":"TRADING"}
	]
}`

const orderBookPayload = `{"lastUpdateId":42,"bids":[["1.0","2.0"]],"asks":[["3.0","4.0"]]}`

func TestClient_GetExchangeInfo_OK(t *testing.T) {
	t.Parallel()

	var gotPath string

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(exchangeInfoPayload))
	}))

	info, err := client.GetExchangeInfo(context.Background())
	if err != nil {
		t.Fatalf("GetExchangeInfo returned %v", err)
	}

	if gotPath != "/api/v3/exchangeInfo" {
		t.Errorf("path = %q, want /api/v3/exchangeInfo", gotPath)
	}

	if len(info.Symbols) != 1 || info.Symbols[0].Symbol != "BTCUSDT" {
		t.Errorf("unexpected symbols %+v", info.Symbols)
	}

	if len(info.RateLimits) != 1 || info.RateLimits[0].Limit != 6000 {
		t.Errorf("unexpected rate limits %+v", info.RateLimits)
	}
}

func TestClient_GetRateLimits(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(exchangeInfoPayload))
	}))

	limits, err := client.GetRateLimits(context.Background())
	if err != nil {
		t.Fatalf("GetRateLimits returned %v", err)
	}

	if len(limits) != 1 || limits[0].RateLimitType != "REQUEST_WEIGHT" {
		t.Errorf("unexpected rate limits %+v", limits)
	}
}

func TestClient_GetOrderBook_OK(t *testing.T) {
	t.Parallel()

	var (
		gotSymbol string
		gotLimit  string
	)

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSymbol = r.URL.Query().Get("symbol")
		gotLimit = r.URL.Query().Get("limit")
		_, _ = w.Write([]byte(orderBookPayload))
	}))

	book, err := client.GetOrderBook(context.Background(), "BTCUSDT", 100)
	if err != nil {
		t.Fatalf("GetOrderBook returned %v", err)
	}

	if gotSymbol != "BTCUSDT" || gotLimit != "100" {
		t.Errorf("query = symbol:%q limit:%q, want BTCUSDT and 100", gotSymbol, gotLimit)
	}

	if book.LastUpdateID != 42 {
		t.Errorf("lastUpdateId = %d, want 42", book.LastUpdateID)
	}

	if len(book.Bids) != 1 || len(book.Asks) != 1 {
		t.Errorf("unexpected book %+v", book)
	}
}

func TestClient_GetOrderBook_StatusCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   int
		body     string
		headers  map[string]string
		wantErr  error
		wantText string
	}{
		{
			name:     "too many requests",
			status:   http.StatusTooManyRequests,
			headers:  map[string]string{"Retry-After": "2"},
			wantErr:  ErrRateLimited,
			wantText: "retry after 2s",
		},
		{
			name:     "bad request",
			status:   http.StatusBadRequest,
			wantText: "binance returned status 400",
		},
		{
			name:     "internal server error",
			status:   http.StatusInternalServerError,
			wantText: "binance returned status 500",
		},
		{
			name:     "ip ban",
			status:   http.StatusTeapot,
			wantText: "binance returned status 418",
		},
		{
			name:     "malformed json",
			status:   http.StatusOK,
			body:     `{"lastUpdateId":`,
			wantText: "decode /api/v3/depth response",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				for key, value := range test.headers {
					w.Header().Set(key, value)
				}

				w.WriteHeader(test.status)

				body := test.body
				if body == "" {
					body = `{}`
				}

				_, _ = w.Write([]byte(body))
			}))

			_, err := client.GetOrderBook(context.Background(), "BTCUSDT", 100)
			if err == nil {
				t.Fatalf("GetOrderBook returned nil, want error")
			}

			if test.wantErr != nil && !errors.Is(err, test.wantErr) {
				t.Errorf("errors.Is(%v, %v) = false", err, test.wantErr)
			}

			if !strings.Contains(err.Error(), test.wantText) {
				t.Errorf("error = %q, want it to contain %q", err, test.wantText)
			}
		})
	}
}

func TestClient_GetOrderBook_StatusErrorExposesCode(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))

	_, err := client.GetOrderBook(context.Background(), "BTCUSDT", 100)

	var status *StatusError
	if !errors.As(err, &status) {
		t.Fatalf("errors.As(%v, *StatusError) = false", err)
	}

	if status.Code != http.StatusBadRequest {
		t.Errorf("Code = %d, want 400", status.Code)
	}
}

func TestClient_GetOrderBook_RejectsOversizedBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		padding := strings.Repeat("x", 512)
		_, _ = w.Write([]byte(`{"lastUpdateId":1,"bids":[],"asks":[],"padding":"` + padding + `"}`))
	}))
	t.Cleanup(server.Close)

	properties := testProperties(server.URL)
	properties.MaxMemorySize = config.DataSize(64)

	client := NewClient(properties, server.Client())

	_, err := client.GetOrderBook(context.Background(), "BTCUSDT", 100)
	if err == nil {
		t.Fatal("GetOrderBook returned nil, want error")
	}

	if !strings.Contains(err.Error(), "response exceeds 64 bytes") {
		t.Errorf("error = %q, want it to mention the response size limit", err)
	}
}

func TestClient_GetOrderBook_ContextCancelled(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	client := NewClient(testProperties(server.URL), server.Client())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GetOrderBook(ctx, "BTCUSDT", 100)
	if err == nil {
		t.Fatal("GetOrderBook returned nil, want error")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(%v, context.Canceled) = false", err)
	}
}

func TestClient_GetOrderBook_DeadlineExceeded(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))

	t.Cleanup(func() {
		close(release)
		server.Close()
	})

	client := NewClient(testProperties(server.URL), server.Client())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.GetOrderBook(ctx, "BTCUSDT", 100)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(%v, context.DeadlineExceeded) = false, err = %v", err, err)
	}
}
