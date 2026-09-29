package api

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

const StatusTrading = "TRADING"

type ExchangeInfo struct {
	RateLimits []RateLimit `json:"rateLimits"`
	Symbols    []Symbol    `json:"symbols"`
}

type RateLimit struct {
	RateLimitType string `json:"rateLimitType"`
	Interval      string `json:"interval"`
	IntervalNum   int    `json:"intervalNum"`
	Limit         int    `json:"limit"`
}

func (r RateLimit) Window() time.Duration {
	return time.Duration(r.IntervalNum) * parseInterval(r.Interval)
}

type Symbol struct {
	Symbol     string `json:"symbol"`
	BaseAsset  string `json:"baseAsset"`
	QuoteAsset string `json:"quoteAsset"`
	Status     string `json:"status"`
}

func (s Symbol) Tradable() bool {
	return s.Symbol != "" && s.Status == StatusTrading
}

type Order struct {
	Price  decimal.Decimal
	Volume decimal.Decimal
}

func (o *Order) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage

	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("decode order level: %w", err)
	}

	if len(raw) != 2 {
		return fmt.Errorf("invalid order level length %d, want 2", len(raw))
	}

	price, err := decodeDecimal(raw[0])
	if err != nil {
		return fmt.Errorf("decode price: %w", err)
	}

	volume, err := decodeDecimal(raw[1])
	if err != nil {
		return fmt.Errorf("decode volume: %w", err)
	}

	o.Price = price
	o.Volume = volume

	return nil
}

func decodeDecimal(data json.RawMessage) (decimal.Decimal, error) {
	var text string

	if err := json.Unmarshal(data, &text); err != nil {
		return decimal.Decimal{}, fmt.Errorf("value must be a decimal string: %w", err)
	}

	value, err := decimal.NewFromString(text)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("invalid decimal %q: %w", text, err)
	}

	return value, nil
}

type OrderBook struct {
	LastUpdateID int64   `json:"lastUpdateId"`
	Bids         []Order `json:"bids"`
	Asks         []Order `json:"asks"`
}

func SideVolume(orders []Order) (decimal.Decimal, int) {
	var total decimal.Decimal

	for _, order := range orders {
		total = total.Add(order.Volume)
	}

	return total, len(orders)
}

func (b OrderBook) AskVolume() (decimal.Decimal, int) {
	return SideVolume(b.Asks)
}

func (b OrderBook) BidVolume() (decimal.Decimal, int) {
	return SideVolume(b.Bids)
}

func parseInterval(interval string) time.Duration {
	switch interval {
	case "SECOND":
		return time.Second
	case "MINUTE":
		return time.Minute
	case "HOUR":
		return time.Hour
	case "DAY":
		return 24 * time.Hour
	default:
		return 0
	}
}
