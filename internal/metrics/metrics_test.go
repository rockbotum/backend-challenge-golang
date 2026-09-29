package metrics

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"

	"backendchallengegolang/internal/api"
)

func TestMetrics_Add(t *testing.T) {
	t.Parallel()

	metrics := Metrics{}

	metrics.Add(api.OrderBook{
		Asks: []api.Order{
			{Volume: decimal.RequireFromString("0.3")},
			{Volume: decimal.RequireFromString("0.2")},
		},
		Bids: []api.Order{
			{Volume: decimal.RequireFromString("1.0")},
		},
	})

	metrics.Add(api.OrderBook{
		Asks: []api.Order{{Volume: decimal.RequireFromString("0.5")}},
		Bids: []api.Order{{Volume: decimal.RequireFromString("2.0")}},
	})

	if want := decimal.RequireFromString("1"); !metrics.AskVolume.Equal(want) {
		t.Errorf("AskVolume = %s, want %s", metrics.AskVolume, want)
	}

	if want := decimal.RequireFromString("3"); !metrics.BidVolume.Equal(want) {
		t.Errorf("BidVolume = %s, want %s", metrics.BidVolume, want)
	}

	if metrics.AskCount != 2 || metrics.BidCount != 2 {
		t.Errorf("counts = %d/%d, want 2/2", metrics.AskCount, metrics.BidCount)
	}

	if metrics.Symbols != 2 {
		t.Errorf("Symbols = %d, want 2", metrics.Symbols)
	}
}

func TestMetrics_Add_SkipsEmptySides(t *testing.T) {
	t.Parallel()

	metrics := Metrics{}

	metrics.Add(api.OrderBook{Bids: []api.Order{{Volume: decimal.RequireFromString("1")}}})

	if metrics.AskCount != 0 {
		t.Errorf("AskCount = %d, want 0", metrics.AskCount)
	}

	if !metrics.AskVolume.IsZero() {
		t.Errorf("AskVolume = %s, want 0", metrics.AskVolume)
	}

	if metrics.BidCount != 1 {
		t.Errorf("BidCount = %d, want 1", metrics.BidCount)
	}

	if metrics.Symbols != 1 {
		t.Errorf("Symbols = %d, want 1", metrics.Symbols)
	}
}

func TestMetrics_Add_EmptyBook(t *testing.T) {
	t.Parallel()

	metrics := Metrics{}

	metrics.Add(api.OrderBook{})

	if metrics.AskCount != 0 || metrics.BidCount != 0 {
		t.Errorf("counts = %d/%d, want 0/0", metrics.AskCount, metrics.BidCount)
	}

	if metrics.Symbols != 1 {
		t.Errorf("Symbols = %d, want 1", metrics.Symbols)
	}
}

func TestMetrics_Averages(t *testing.T) {
	t.Parallel()

	metrics := Metrics{
		AskVolume: decimal.RequireFromString("1"),
		BidVolume: decimal.RequireFromString("3"),
		AskCount:  2,
		BidCount:  3,
	}

	averageAsk, err := metrics.AverageAskVolume()
	if err != nil {
		t.Fatalf("AverageAskVolume returned %v", err)
	}

	if want := decimal.RequireFromString("0.5"); !averageAsk.Equal(want) {
		t.Errorf("AverageAskVolume = %s, want %s", averageAsk, want)
	}

	averageBid, err := metrics.AverageBidVolume()
	if err != nil {
		t.Fatalf("AverageBidVolume returned %v", err)
	}

	if want := decimal.NewFromInt(1); !averageBid.Equal(want) {
		t.Errorf("AverageBidVolume = %s, want %s", averageBid, want)
	}
}

func TestMetrics_AveragesWithoutLevels(t *testing.T) {
	t.Parallel()

	var metrics Metrics

	if _, err := metrics.AverageAskVolume(); !errors.Is(err, ErrNoLevels) {
		t.Errorf("AverageAskVolume error = %v, want ErrNoLevels", err)
	}

	if _, err := metrics.AverageBidVolume(); !errors.Is(err, ErrNoLevels) {
		t.Errorf("AverageBidVolume error = %v, want ErrNoLevels", err)
	}
}
