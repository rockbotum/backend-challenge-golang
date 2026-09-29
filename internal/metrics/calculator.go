package metrics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"backendchallengegolang/internal/api"
	"backendchallengegolang/internal/ratelimit"
)

const DryRunSymbols = 10

const MaxRecordedErrors = 10

type OrderBookSource interface {
	GetOrderBook(ctx context.Context, symbol string, depth int) (api.OrderBook, error)
}

type Options struct {
	Depth      int
	Workers    int
	DryRun     bool
	MaxRetries int
	Backoff    time.Duration
	Weight     int
}

type Calculator struct {
	source  OrderBookSource
	limiter *ratelimit.Limiter
	options Options
	logger  *slog.Logger
}

type Result struct {
	Metrics Metrics
	Errors  []error
}

func NewCalculator(
	source OrderBookSource,
	limiter *ratelimit.Limiter,
	options Options,
	logger *slog.Logger,
) *Calculator {
	if options.Workers <= 0 {
		options.Workers = 1
	}

	if options.Weight <= 0 {
		options.Weight = ratelimit.DepthWeight(options.Depth)
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &Calculator{
		source:  source,
		limiter: limiter,
		options: options,
		logger:  logger,
	}
}

func (c *Calculator) Calculate(ctx context.Context, symbols []api.Symbol) (Result, error) {
	selected := c.selectSymbols(symbols)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan string)
	results := make(chan outcome)

	var workers sync.WaitGroup

	for range c.options.Workers {
		workers.Add(1)

		go func() {
			defer workers.Done()

			c.run(runCtx, jobs, results)
		}()
	}

	go c.produce(runCtx, jobs, selected)

	go func() {
		workers.Wait()
		close(results)
	}()

	result := Result{}

	for item := range results {
		result.add(item)
	}

	if err := runCtx.Err(); err != nil {
		return result, fmt.Errorf("calculate order book metrics: %w", err)
	}

	return result, nil
}

func (c *Calculator) selectSymbols(symbols []api.Symbol) []api.Symbol {
	selected := make([]api.Symbol, 0, len(symbols))

	for _, symbol := range symbols {
		if symbol.Tradable() {
			selected = append(selected, symbol)
		}
	}

	if c.options.DryRun && len(selected) > DryRunSymbols {
		selected = selected[:DryRunSymbols]
	}

	return selected
}

func (c *Calculator) produce(ctx context.Context, jobs chan<- string, symbols []api.Symbol) {
	defer close(jobs)

	for _, symbol := range symbols {
		select {
		case jobs <- symbol.Symbol:
		case <-ctx.Done():
			return
		}
	}
}

func (c *Calculator) run(ctx context.Context, jobs <-chan string, results chan<- outcome) {
	for {
		select {
		case symbol, ok := <-jobs:
			if !ok {
				return
			}

			item := outcome{symbol: symbol}
			item.book, item.err = c.fetch(ctx, symbol)

			select {
			case results <- item:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *Calculator) fetch(ctx context.Context, symbol string) (api.OrderBook, error) {
	var lastErr error

	for attempt := range c.options.MaxRetries + 1 {
		if err := c.acquire(ctx); err != nil {
			return api.OrderBook{}, err
		}

		book, err := c.source.GetOrderBook(ctx, symbol, c.options.Depth)
		retryAfter, retryable := retryDelay(err)

		switch {
		case err == nil:
			return book, nil
		case retryable:
			lastErr = err
			c.backoff(attempt, retryAfter)
		default:
			return book, err
		}
	}

	return api.OrderBook{}, fmt.Errorf(
		"fetch order book for %s: %w",
		symbol,
		lastErr,
	)
}

func (c *Calculator) acquire(ctx context.Context) error {
	if c.limiter == nil {
		return nil
	}

	return c.limiter.Acquire(ctx, c.options.Weight)
}

func (c *Calculator) backoff(attempt int, retryAfter time.Duration) {
	delay := retryAfter
	if scaled := c.options.Backoff << attempt; scaled > delay {
		delay = scaled
	}

	if c.limiter != nil {
		c.limiter.Backoff(delay)
	}

	c.logger.Warn("order book request throttled", "delay", delay, "attempt", attempt)
}

func retryDelay(err error) (time.Duration, bool) {
	var rateLimited *api.RateLimitError
	if errors.As(err, &rateLimited) {
		return rateLimited.RetryAfter, true
	}

	var status *api.StatusError
	if errors.As(err, &status) && status.Code >= 500 {
		return 0, true
	}

	return 0, false
}

type outcome struct {
	symbol string
	book   api.OrderBook
	err    error
}

func (r *Result) add(item outcome) {
	if item.err != nil {
		r.Metrics.Failed++

		if len(r.Errors) < MaxRecordedErrors {
			r.Errors = append(r.Errors, item.err)
		}

		return
	}

	r.Metrics.Add(item.book)
}
