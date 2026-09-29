package api

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestOrderBook_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	payload := `{
		"lastUpdateId": 123,
		"bids": [["65000.10", "0.25000000"], ["64999.00", "1.5"]],
		"asks": [["65001.20", "0.30000000"]]
	}`

	var book OrderBook

	if err := json.Unmarshal([]byte(payload), &book); err != nil {
		t.Fatalf("Unmarshal returned %v", err)
	}

	if book.LastUpdateID != 123 {
		t.Errorf("lastUpdateId = %d, want 123", book.LastUpdateID)
	}

	if len(book.Bids) != 2 {
		t.Fatalf("bids = %d levels, want 2", len(book.Bids))
	}

	if want := decimal.RequireFromString("65000.10"); !book.Bids[0].Price.Equal(want) {
		t.Errorf("bids[0].price = %s, want %s", book.Bids[0].Price, want)
	}

	if want := decimal.RequireFromString("0.25000000"); !book.Bids[0].Volume.Equal(want) {
		t.Errorf("bids[0].volume = %s, want %s", book.Bids[0].Volume, want)
	}

	if want := decimal.RequireFromString("1.5"); !book.Bids[1].Volume.Equal(want) {
		t.Errorf("bids[1].volume = %s, want %s", book.Bids[1].Volume, want)
	}
}

func TestOrder_UnmarshalJSON_Invalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload string
		want    string
	}{
		{
			name:    "too few elements",
			payload: `["65000.10"]`,
			want:    "invalid order level length 1, want 2",
		},
		{
			name:    "too many elements",
			payload: `["65000.10", "0.25", "extra"]`,
			want:    "invalid order level length 3, want 2",
		},
		{
			name:    "invalid price",
			payload: `["not-a-price", "0.25"]`,
			want:    "invalid decimal \"not-a-price\"",
		},
		{
			name:    "invalid volume",
			payload: `["65000.10", "not-a-volume"]`,
			want:    "invalid decimal \"not-a-volume\"",
		},
		{
			name:    "numeric price instead of string",
			payload: `[65000.10, "0.25"]`,
			want:    "value must be a decimal string",
		},
		{
			name:    "object instead of array",
			payload: `{"price":"1","volume":"2"}`,
			want:    "decode order level",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var order Order

			err := json.Unmarshal([]byte(test.payload), &order)
			if err == nil {
				t.Fatalf("Unmarshal(%s) returned nil, want error", test.payload)
			}

			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("Unmarshal(%s) error = %q, want it to contain %q", test.payload, err, test.want)
			}
		})
	}
}

func TestOrderBook_UnmarshalJSON_EmptySides(t *testing.T) {
	t.Parallel()

	var book OrderBook

	if err := json.Unmarshal([]byte(`{"lastUpdateId":1,"bids":[],"asks":[]}`), &book); err != nil {
		t.Fatalf("Unmarshal returned %v", err)
	}

	askVolume, askLevels := book.AskVolume()
	bidVolume, bidLevels := book.BidVolume()

	if askLevels != 0 || !askVolume.IsZero() {
		t.Errorf("ask volume = %s across %d levels, want 0 across 0", askVolume, askLevels)
	}

	if bidLevels != 0 || !bidVolume.IsZero() {
		t.Errorf("bid volume = %s across %d levels, want 0 across 0", bidVolume, bidLevels)
	}
}

func TestOrderBook_Volume(t *testing.T) {
	t.Parallel()

	book := OrderBook{
		Asks: []Order{
			{Volume: decimal.RequireFromString("0.3")},
			{Volume: decimal.RequireFromString("0.2")},
		},
		Bids: []Order{
			{Volume: decimal.RequireFromString("1")},
		},
	}

	askVolume, askLevels := book.AskVolume()
	if want := decimal.RequireFromString("0.5"); !askVolume.Equal(want) {
		t.Errorf("ask volume = %s, want %s", askVolume, want)
	}

	if askLevels != 2 {
		t.Errorf("ask levels = %d, want 2", askLevels)
	}

	bidVolume, bidLevels := book.BidVolume()
	if want := decimal.RequireFromString("1"); !bidVolume.Equal(want) {
		t.Errorf("bid volume = %s, want %s", bidVolume, want)
	}

	if bidLevels != 1 {
		t.Errorf("bid levels = %d, want 1", bidLevels)
	}
}

func TestExchangeInfo_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	payload := `{
		"rateLimits": [
			{"rateLimitType":"REQUEST_WEIGHT","interval":"MINUTE","intervalNum":1,"limit":6000},
			{"rateLimitType":"ORDERS","interval":"SECOND","intervalNum":10,"limit":50}
		],
		"symbols": [
			{"symbol":"BTCUSDT","baseAsset":"BTC","quoteAsset":"USDT","status":"TRADING"},
			{"symbol":"OLDUSDT","baseAsset":"OLD","quoteAsset":"USDT","status":"BREAK"}
		]
	}`

	var info ExchangeInfo

	if err := json.Unmarshal([]byte(payload), &info); err != nil {
		t.Fatalf("Unmarshal returned %v", err)
	}

	if len(info.RateLimits) != 2 {
		t.Fatalf("rateLimits = %d, want 2", len(info.RateLimits))
	}

	if info.RateLimits[0].RateLimitType != "REQUEST_WEIGHT" {
		t.Errorf("rateLimitType = %q, want REQUEST_WEIGHT", info.RateLimits[0].RateLimitType)
	}

	if info.RateLimits[0].Limit != 6000 || info.RateLimits[0].IntervalNum != 1 {
		t.Errorf("unexpected rate limit %+v", info.RateLimits[0])
	}

	if len(info.Symbols) != 2 {
		t.Fatalf("symbols = %d, want 2", len(info.Symbols))
	}

	if info.Symbols[0].Symbol != "BTCUSDT" || info.Symbols[0].BaseAsset != "BTC" {
		t.Errorf("unexpected symbol %+v", info.Symbols[0])
	}
}

func TestSymbol_Tradable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		symbol Symbol
		want   bool
	}{
		{name: "trading", symbol: Symbol{Symbol: "BTCUSDT", Status: StatusTrading}, want: true},
		{name: "not trading", symbol: Symbol{Symbol: "OLDUSDT", Status: "BREAK"}, want: false},
		{name: "missing name", symbol: Symbol{Status: StatusTrading}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := test.symbol.Tradable(); got != test.want {
				t.Errorf("Tradable() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestRateLimit_Window(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		limit RateLimit
		want  time.Duration
	}{
		{
			name:  "minute",
			limit: RateLimit{Interval: "MINUTE", IntervalNum: 1},
			want:  time.Minute,
		},
		{
			name:  "seconds",
			limit: RateLimit{Interval: "SECOND", IntervalNum: 10},
			want:  10 * time.Second,
		},
		{
			name:  "minutes",
			limit: RateLimit{Interval: "MINUTE", IntervalNum: 5},
			want:  5 * time.Minute,
		},
		{
			name:  "unknown interval",
			limit: RateLimit{Interval: "FORTNIGHT", IntervalNum: 1},
			want:  0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := test.limit.Window(); got != test.want {
				t.Errorf("Window() = %s, want %s", got, test.want)
			}
		})
	}
}
