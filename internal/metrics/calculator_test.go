package metrics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/shopspring/decimal"

	"backendchallengegolang/internal/api"
	"backendchallengegolang/internal/ratelimit"
)

type stubSource struct {
	mu       sync.Mutex
	calls    map[string]int
	book     func(symbol string) (api.OrderBook, error)
	inflight atomic.Int64
	peak     atomic.Int64
	release  chan struct{}
}

func newStubSource(book func(symbol string) (api.OrderBook, error)) *stubSource {
	return &stubSource{calls: make(map[string]int), book: book}
}

func (s *stubSource) GetOrderBook(
	_ context.Context,
	symbol string,
	_ int,
) (api.OrderBook, error) {
	running := s.inflight.Add(1)
	defer s.inflight.Add(-1)

	s.observePeak(running)

	if s.release != nil {
		select {
		case <-s.release:
		case <-time.After(50 * time.Millisecond):
		}
	}

	s.mu.Lock()
	s.calls[symbol]++
	s.mu.Unlock()

	return s.book(symbol)
}

func (s *stubSource) observePeak(running int64) {
	for {
		peak := s.peak.Load()
		if running <= peak || s.peak.CompareAndSwap(peak, running) {
			return
		}
	}
}

func (s *stubSource) callCount(symbol string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.calls[symbol]
}

func discLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(discard{}, nil))
}

type discard struct{}

func (discard) Write(payload []byte) (int, error) {
	return len(payload), nil
}

func bookFor(ask string, bid string) api.OrderBook {
	return api.OrderBook{
		LastUpdateID: 1,
		Asks:         []api.Order{{Volume: decimal.RequireFromString(ask)}},
		Bids:         []api.Order{{Volume: decimal.RequireFromString(bid)}},
	}
}

func tradingSymbols(count int) []api.Symbol {
	symbols := make([]api.Symbol, 0, count+2)

	for i := range count {
		symbols = append(symbols, api.Symbol{
			Symbol: "SYM" + strconv.Itoa(i),
			Status: api.StatusTrading,
		})
	}

	symbols = append(symbols, api.Symbol{Symbol: "BROKEN", Status: "BREAK"})
	symbols = append(symbols, api.Symbol{Symbol: "", Status: api.StatusTrading})

	return symbols
}

func newCalculator(t *testing.T, source OrderBookSource, options Options) *Calculator {
	t.Helper()

	if options.Workers == 0 {
		options.Workers = 4
	}

	if options.Depth == 0 {
		options.Depth = 100
	}

	return NewCalculator(source, nil, options, discLogger())
}

func TestCalculator_Calculate(t *testing.T) {
	t.Parallel()

	source := newStubSource(func(symbol string) (api.OrderBook, error) {
		if symbol == "SYM2" {
			return bookFor("1", "2"), nil
		}

		return bookFor("1", "3"), nil
	})

	calculator := newCalculator(t, source, Options{Workers: 4})

	result, err := calculator.Calculate(context.Background(), tradingSymbols(5))
	if err != nil {
		t.Fatalf("Calculate returned %v", err)
	}

	if result.Metrics.Symbols != 5 {
		t.Errorf("Symbols = %d, want 5", result.Metrics.Symbols)
	}

	if result.Metrics.Failed != 0 {
		t.Errorf("Failed = %d, want 0", result.Metrics.Failed)
	}

	if len(result.Errors) != 0 {
		t.Errorf("Errors = %v, want none", result.Errors)
	}

	if want := decimal.RequireFromString("5"); !result.Metrics.AskVolume.Equal(want) {
		t.Errorf("AskVolume = %s, want %s", result.Metrics.AskVolume, want)
	}

	if want := decimal.RequireFromString("14"); !result.Metrics.BidVolume.Equal(want) {
		t.Errorf("BidVolume = %s, want %s", result.Metrics.BidVolume, want)
	}

	for _, symbol := range []string{"SYM0", "SYM4"} {
		if got := source.callCount(symbol); got != 1 {
			t.Errorf("GetOrderBook(%s) called %d times, want 1", symbol, got)
		}
	}

	if got := source.callCount("BROKEN"); got != 0 {
		t.Errorf("GetOrderBook(BROKEN) called %d times, want 0", got)
	}
}

func TestCalculator_Calculate_DryRunLimitsSymbols(t *testing.T) {
	t.Parallel()

	source := newStubSource(func(string) (api.OrderBook, error) {
		return bookFor("1", "1"), nil
	})

	calculator := newCalculator(t, source, Options{Workers: 2, DryRun: true})

	result, err := calculator.Calculate(context.Background(), tradingSymbols(50))
	if err != nil {
		t.Fatalf("Calculate returned %v", err)
	}

	if result.Metrics.Symbols != DryRunSymbols {
		t.Errorf("Symbols = %d, want %d", result.Metrics.Symbols, DryRunSymbols)
	}

	if got := source.inflight.Load(); got != 0 {
		t.Errorf("inflight = %d, want 0", got)
	}
}

func TestCalculator_Calculate_DryRunWithoutEnoughSymbols(t *testing.T) {
	t.Parallel()

	source := newStubSource(func(string) (api.OrderBook, error) {
		return bookFor("1", "1"), nil
	})

	calculator := newCalculator(t, source, Options{Workers: 2, DryRun: true})

	result, err := calculator.Calculate(context.Background(), tradingSymbols(3))
	if err != nil {
		t.Fatalf("Calculate returned %v", err)
	}

	if result.Metrics.Symbols != 3 {
		t.Errorf("Symbols = %d, want 3", result.Metrics.Symbols)
	}
}

func TestCalculator_Calculate_LimitsConcurrency(t *testing.T) {
	t.Parallel()

	const workers = 3

	source := newStubSource(func(string) (api.OrderBook, error) {
		return bookFor("1", "1"), nil
	})
	source.release = make(chan struct{})

	calculator := newCalculator(t, source, Options{Workers: workers})

	done := make(chan struct{})

	go func() {
		defer close(done)

		if _, err := calculator.Calculate(context.Background(), tradingSymbols(12)); err != nil {
			t.Errorf("Calculate returned %v", err)
		}
	}()

	waitFor(t, func() bool {
		return source.inflight.Load() == workers
	})

	close(source.release)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Calculate did not finish")
	}

	if peak := source.peak.Load(); peak > workers {
		t.Errorf("peak concurrency = %d, want at most %d", peak, workers)
	}

	if result := source.inflight.Load(); result != 0 {
		t.Errorf("inflight = %d after Calculate returned, want 0", result)
	}
}

func TestCalculator_Calculate_KeepsGoingAfterSymbolErrors(t *testing.T) {
	t.Parallel()

	failure := errors.New("upstream is unavailable")

	source := newStubSource(func(symbol string) (api.OrderBook, error) {
		if symbol == "SYM1" {
			return api.OrderBook{}, failure
		}

		return bookFor("2", "4"), nil
	})

	calculator := newCalculator(t, source, Options{Workers: 2})

	result, err := calculator.Calculate(context.Background(), tradingSymbols(4))
	if err != nil {
		t.Fatalf("Calculate returned %v", err)
	}

	if result.Metrics.Failed != 1 {
		t.Errorf("Failed = %d, want 1", result.Metrics.Failed)
	}

	if result.Metrics.Symbols != 3 {
		t.Errorf("Symbols = %d, want 3", result.Metrics.Symbols)
	}

	if len(result.Errors) != 1 || !errors.Is(result.Errors[0], failure) {
		t.Errorf("Errors = %v, want the single upstream failure", result.Errors)
	}
}

func TestCalculator_Calculate_RetriesRateLimitedSymbols(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int64

	source := newStubSource(func(string) (api.OrderBook, error) {
		if attempts.Add(1) <= 2 {
			return api.OrderBook{}, &api.RateLimitError{StatusCode: 429}
		}

		return bookFor("1", "2"), nil
	})

	calculator := newCalculator(t, source, Options{
		Workers:    1,
		MaxRetries: 3,
		Backoff:    time.Millisecond,
		Weight:     1,
	})

	result, err := calculator.Calculate(context.Background(), tradingSymbols(1))
	if err != nil {
		t.Fatalf("Calculate returned %v", err)
	}

	if result.Metrics.Failed != 0 {
		t.Errorf("Failed = %d, want 0", result.Metrics.Failed)
	}

	if got := attempts.Load(); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
}

func TestCalculator_Calculate_GivesUpAfterMaxRetries(t *testing.T) {
	t.Parallel()

	source := newStubSource(func(string) (api.OrderBook, error) {
		return api.OrderBook{}, &api.RateLimitError{StatusCode: 429}
	})

	calculator := newCalculator(t, source, Options{
		Workers:    1,
		MaxRetries: 2,
		Backoff:    time.Millisecond,
		Weight:     1,
	})

	result, err := calculator.Calculate(context.Background(), tradingSymbols(1))
	if err != nil {
		t.Fatalf("Calculate returned %v", err)
	}

	if result.Metrics.Failed != 1 {
		t.Errorf("Failed = %d, want 1", result.Metrics.Failed)
	}

	if got := source.callCount("SYM0"); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
}

func TestCalculator_Calculate_DoesNotRetryClientErrors(t *testing.T) {
	t.Parallel()

	source := newStubSource(func(string) (api.OrderBook, error) {
		return api.OrderBook{}, &api.StatusError{Code: 400, Status: "400 Bad Request"}
	})

	calculator := newCalculator(t, source, Options{Workers: 1, MaxRetries: 3, Backoff: time.Millisecond})

	if _, err := calculator.Calculate(context.Background(), tradingSymbols(1)); err != nil {
		t.Fatalf("Calculate returned %v", err)
	}

	if got := source.callCount("SYM0"); got != 1 {
		t.Errorf("attempts = %d, want 1", got)
	}
}

func TestCalculator_Calculate_RetriesServerErrors(t *testing.T) {
	t.Parallel()

	source := newStubSource(func(string) (api.OrderBook, error) {
		return api.OrderBook{}, &api.StatusError{Code: 503, Status: "503 Service Unavailable"}
	})

	calculator := newCalculator(t, source, Options{
		Workers:    1,
		MaxRetries: 2,
		Backoff:    time.Millisecond,
		Weight:     1,
	})

	if _, err := calculator.Calculate(context.Background(), tradingSymbols(1)); err != nil {
		t.Fatalf("Calculate returned %v", err)
	}

	if got := source.callCount("SYM0"); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
}

func TestCalculator_Calculate_RespectsCancelledContext(t *testing.T) {
	t.Parallel()

	source := newStubSource(func(string) (api.OrderBook, error) {
		return bookFor("1", "1"), nil
	})
	source.release = make(chan struct{})

	t.Cleanup(func() { close(source.release) })

	calculator := newCalculator(t, source, Options{Workers: 2})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)

	go func() {
		_, err := calculator.Calculate(ctx, tradingSymbols(20))
		done <- err
	}()

	waitFor(t, func() bool {
		return source.inflight.Load() == 2
	})

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Calculate error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Calculate did not return after the context was cancelled")
	}
}

func TestCalculator_Calculate_DoesNotLeakGoroutines(t *testing.T) {
	t.Parallel()

	before := runtime.NumGoroutine()

	source := newStubSource(func(symbol string) (api.OrderBook, error) {
		if symbol == "SYM3" {
			return api.OrderBook{}, errors.New("boom")
		}

		return bookFor("1", "2"), nil
	})

	calculator := newCalculator(t, source, Options{Workers: 6})

	if _, err := calculator.Calculate(context.Background(), tradingSymbols(30)); err != nil {
		t.Fatalf("Calculate returned %v", err)
	}

	waitFor(t, func() bool {
		return runtime.NumGoroutine() <= before+1
	})
}

func TestCalculator_Calculate_WaitsForTheRateLimiter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const weight = 5

		source := newStubSource(func(string) (api.OrderBook, error) {
			return bookFor("1", "1"), nil
		})

		limiter := ratelimit.New([]ratelimit.Limit{{Weight: weight, Window: time.Minute}}, nil)

		calculator := NewCalculator(source, limiter, Options{
			Depth:      100,
			Workers:    1,
			Weight:     weight,
			Backoff:    time.Second,
			MaxRetries: 0,
		}, discLogger())

		done := make(chan struct{})

		go func() {
			defer close(done)

			if _, err := calculator.Calculate(context.Background(), tradingSymbols(3)); err != nil {
				t.Errorf("Calculate returned %v", err)
			}
		}()

		time.Sleep(time.Minute)
		synctest.Wait()

		select {
		case <-done:
			t.Fatal("Calculate finished inside a single weight window")
		default:
		}

		time.Sleep(time.Minute)
		synctest.Wait()

		select {
		case <-done:
		default:
			t.Fatal("Calculate did not finish after two weight windows")
		}
	})
}

func TestCalculator_Calculate_EmptyInput(t *testing.T) {
	t.Parallel()

	source := newStubSource(func(string) (api.OrderBook, error) {
		return bookFor("1", "1"), nil
	})

	calculator := newCalculator(t, source, Options{})

	result, err := calculator.Calculate(context.Background(), nil)
	if err != nil {
		t.Fatalf("Calculate returned %v", err)
	}

	if result.Metrics.Symbols != 0 {
		t.Errorf("Symbols = %d, want 0", result.Metrics.Symbols)
	}
}

func TestCalculator_NewCalculator_NormalisesOptions(t *testing.T) {
	t.Parallel()

	source := newStubSource(func(string) (api.OrderBook, error) {
		return bookFor("1", "1"), nil
	})

	calculator := NewCalculator(source, nil, Options{}, nil)

	if calculator.options.Workers != 1 {
		t.Errorf("Workers = %d, want 1", calculator.options.Workers)
	}

	if want := ratelimit.DepthWeight(0); calculator.options.Weight != want {
		t.Errorf("Weight = %d, want %d", calculator.options.Weight, want)
	}

	if calculator.logger == nil {
		t.Error("logger is nil, want the default logger")
	}
}

func TestCalculator_Calculate_CapsRecordedErrors(t *testing.T) {
	t.Parallel()

	source := newStubSource(func(string) (api.OrderBook, error) {
		return api.OrderBook{}, fmt.Errorf("symbol failed")
	})

	calculator := newCalculator(t, source, Options{Workers: 4})

	result, err := calculator.Calculate(context.Background(), tradingSymbols(25))
	if err != nil {
		t.Fatalf("Calculate returned %v", err)
	}

	if result.Metrics.Failed != 25 {
		t.Errorf("Failed = %d, want 25", result.Metrics.Failed)
	}

	if len(result.Errors) != MaxRecordedErrors {
		t.Errorf("Errors = %d, want %d", len(result.Errors), MaxRecordedErrors)
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		if condition() {
			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatal("condition was not met in time")
}
